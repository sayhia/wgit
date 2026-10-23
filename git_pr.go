package main

// In-app PR / MR review (C2).
//
// Design notes:
//   · No third-party SDK: GitHub REST v3 and GitLab v4 are plain JSON over
//     HTTP, and the app already ships net/http. Keeping the dependency surface
//     at zero matters for a local-first tool.
//   · Tokens never leave the backend. The frontend only ever sees
//     `hasToken` + `tokenSource`, and stored tokens are encrypted with the same
//     AES-256-GCM vault used by the studio credential store.
//   · Every network call is bounded (25s) and body reads are capped, so a
//     hostile or broken proxy cannot wedge the UI or exhaust memory.
//   · `WGIT_PR_API_BASE` / `WGIT_PR_PROVIDER` let tests (and self-hosted
//     instances that do not contain "gitlab" in their hostname) override
//     detection without touching production defaults.

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	prVaultSecretType = "pr-token"
	prTokenEnvVar     = "WGIT_PR_TOKEN"
	prMaxBodyBytes    = 8 << 20
	prMaxPatchBytes   = 200 << 10
)

// ---------------------------------------------------------------- types ----

type PrProviderInfo struct {
	Supported   bool   `json:"supported"`
	Provider    string `json:"provider"` // github | gitlab | gitee | bitbucket | gitea | unknown
	Host        string `json:"host"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	ProjectPath string `json:"projectPath"`
	RemoteURL   string `json:"remoteUrl"`
	WebBaseURL  string `json:"webBaseUrl"`
	ApiBaseURL  string `json:"apiBaseUrl"`
	HasToken    bool   `json:"hasToken"`
	TokenSource string `json:"tokenSource"`
	Account     string `json:"account"`
	TokenHint   string `json:"tokenHint"`
	Message     string `json:"message"`
}

type PrSummary struct {
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	State          string   `json:"state"` // open | closed | merged
	IsDraft        bool     `json:"isDraft"`
	Author         string   `json:"author"`
	SourceBranch   string   `json:"sourceBranch"`
	TargetBranch   string   `json:"targetBranch"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	CommentCount   int      `json:"commentCount"`
	Additions      int      `json:"additions"`
	Deletions      int      `json:"deletions"`
	ChangedFiles   int      `json:"changedFiles"`
	WebURL         string   `json:"webUrl"`
	Labels         []string `json:"labels"`
	ChecksState    string   `json:"checksState"`
	Mergeable      bool     `json:"mergeable"`
	MergeableState string   `json:"mergeableState"`
}

type PrFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath"`
	Status    string `json:"status"` // added | modified | removed | renamed | copied | changed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
	Truncated bool   `json:"truncated"`
	Binary    bool   `json:"binary"`
}

type PrComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side"`
	CreatedAt string `json:"createdAt"`
	Kind      string `json:"kind"` // review (line-anchored) | issue (general)
	ReplyTo   string `json:"replyTo"`
}

type PrCheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
	Detail     string `json:"detail"`
}

type PrDetail struct {
	Summary        PrSummary   `json:"summary"`
	Body           string      `json:"body"`
	Files          []PrFile    `json:"files"`
	Comments       []PrComment `json:"comments"`
	Checks         []PrCheck   `json:"checks"`
	CommitCount    int         `json:"commitCount"`
	BaseSHA        string      `json:"baseSha"`
	HeadSHA        string      `json:"headSha"`
	StartSHA       string      `json:"startSha"`
	FilesTruncated bool        `json:"filesTruncated"`
	Message        string      `json:"message"`
}

type PrListResult struct {
	Provider   string      `json:"provider"`
	Items      []PrSummary `json:"items"`
	Total      int         `json:"total"`
	HasMore    bool        `json:"hasMore"`
	FetchedAt  string      `json:"fetchedAt"`
	WebBaseURL string      `json:"webBaseUrl"`
	Message    string      `json:"message"`
}

type PrActionResult struct {
	Success  bool   `json:"success"`
	Action   string `json:"action"`
	Message  string `json:"message"`
	WebURL   string `json:"webUrl"`
	NewState string `json:"newState"`
}

// ------------------------------------------------------- remote parsing ----

type prRemote struct {
	Host       string
	Path       string
	Provider   string
	WebBaseURL string
}

func parsePrRemoteURL(remoteURL string) prRemote {
	raw := strings.TrimSpace(remoteURL)
	var host, path string

	switch {
	case strings.HasPrefix(raw, "git@"):
		rest := strings.TrimPrefix(raw, "git@")
		if i := strings.Index(rest, ":"); i > 0 {
			host = rest[:i]
			path = rest[i+1:]
		}
	case strings.Contains(raw, "://"):
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			host = u.Host
			path = u.Path
		}
	default:
		// scp-like "host:owner/repo.git" without a user prefix
		if i := strings.Index(raw, ":"); i > 0 {
			host = raw[:i]
			path = raw[i+1:]
		}
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")

	provider := prProviderForHost(host)
	webBase := ""
	if host != "" && path != "" {
		webBase = fmt.Sprintf("https://%s/%s", host, path)
	}
	return prRemote{Host: host, Path: path, Provider: provider, WebBaseURL: webBase}
}

func prProviderForHost(host string) string {
	if override := strings.ToLower(strings.TrimSpace(os.Getenv("WGIT_PR_PROVIDER"))); override != "" {
		return override
	}
	lower := strings.ToLower(host)
	switch {
	case strings.Contains(lower, "github"):
		return "github"
	case strings.Contains(lower, "gitlab"):
		return "gitlab"
	case strings.Contains(lower, "gitee"):
		return "gitee"
	case strings.Contains(lower, "bitbucket"):
		return "bitbucket"
	case strings.Contains(lower, "gitea"), strings.Contains(lower, "forgejo"):
		return "gitea"
	default:
		return "unknown"
	}
}

func prAPIBase(provider, host string) string {
	if override := strings.TrimSpace(os.Getenv("WGIT_PR_API_BASE")); override != "" {
		return strings.TrimRight(override, "/")
	}
	switch provider {
	case "github":
		if strings.EqualFold(host, "github.com") {
			return "https://api.github.com"
		}
		return "https://" + host + "/api/v3"
	case "gitlab":
		return "https://" + host + "/api/v4"
	}
	return ""
}

// --------------------------------------------------------- token lookup ----

type prContext struct {
	Info  *PrProviderInfo
	Token string
	API   *prAPI
}

func buildPrProviderInfo(repoPath string) (*PrProviderInfo, string) {
	info := &PrProviderInfo{Provider: "unknown"}
	remoteURL := prOriginURL(repoPath)
	info.RemoteURL = remoteURL
	if remoteURL == "" {
		info.Message = "当前仓库没有配置远端，无法定位 PR/MR"
		return info, ""
	}

	remote := parsePrRemoteURL(remoteURL)
	info.Host = remote.Host
	info.ProjectPath = remote.Path
	info.Provider = remote.Provider
	info.WebBaseURL = remote.WebBaseURL
	if parts := strings.Split(remote.Path, "/"); len(parts) >= 2 {
		info.Owner = strings.Join(parts[:len(parts)-1], "/")
		info.Repo = parts[len(parts)-1]
	}

	switch remote.Provider {
	case "github", "gitlab":
		info.Supported = true
	default:
		info.Message = "仅内置 GitHub / GitLab 的 PR 接口；自建实例可设置 WGIT_PR_PROVIDER 与 WGIT_PR_API_BASE 环境变量后重试"
		return info, ""
	}

	info.ApiBaseURL = prAPIBase(remote.Provider, remote.Host)
	token, source := resolvePrToken(repoPath, remote.Provider, remote.Host)
	info.HasToken = token != ""
	info.TokenSource = source
	info.TokenHint = prTokenHint(remote.Provider)
	if !info.HasToken {
		info.Message = "未找到访问令牌：可设置环境变量 " + strings.Join(prTokenEnvNames(remote.Provider), " / ") + "，或在下方保存令牌"
	}
	return info, token
}

func prOriginURL(repoPath string) string {
	if out, err := ExecGit(repoPath, "remote", "get-url", "origin"); err == nil {
		if url := strings.TrimSpace(out); url != "" {
			return url
		}
	}
	out, err := ExecGit(repoPath, "remote")
	if err != nil {
		return ""
	}
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if urlOut, err := ExecGit(repoPath, "remote", "get-url", name); err == nil {
			if url := strings.TrimSpace(urlOut); url != "" {
				return url
			}
		}
	}
	return ""
}

func prTokenEnvNames(provider string) []string {
	switch provider {
	case "github":
		return []string{"GITHUB_TOKEN", "GH_TOKEN", prTokenEnvVar}
	case "gitlab":
		return []string{"GITLAB_TOKEN", "GL_TOKEN", prTokenEnvVar}
	default:
		return []string{prTokenEnvVar}
	}
}

func prTokenHint(provider string) string {
	switch provider {
	case "github":
		return "GitHub 令牌需要 repo 权限（公开仓库只读可用 public_repo）"
	case "gitlab":
		return "GitLab 令牌需要 api 或 read_api 权限"
	}
	return ""
}

func resolvePrToken(repoPath, provider, host string) (string, string) {
	for _, name := range prTokenEnvNames(provider) {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value, "环境变量 " + name
		}
	}
	if token, ok := readVaultToken(repoPath, host); ok {
		return token, "本地保险库"
	}
	if provider == "github" {
		if token := ghCliToken(host); token != "" {
			return token, "gh CLI"
		}
	}
	if token := gitCredentialToken(repoPath, host); token != "" {
		return token, "git credential"
	}
	return "", ""
}

func readVaultToken(repoPath, host string) (string, bool) {
	data, err := os.ReadFile(getVaultFilePath(repoPath))
	if err != nil {
		return "", false
	}
	var records []EncryptedVaultRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return "", false
	}
	key := deriveVaultKey(repoPath)
	for _, record := range records {
		if record.SecretType != prVaultSecretType || record.Identifier != host {
			continue
		}
		plain, err := decryptVaultValue(key, record.Ciphertext, record.Nonce)
		if err != nil || strings.TrimSpace(plain) == "" {
			continue
		}
		return plain, true
	}
	return "", false
}

func decryptVaultValue(key []byte, cipherB64, nonceB64 string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func ghCliToken(host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func gitCredentialToken(repoPath, host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "credential", "fill")
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=" + host + "\n\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return ""
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "password=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "password="))
		}
	}
	return ""
}

func loadPrContext(repoPath string) (*prContext, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	info, token := buildPrProviderInfo(repoPath)
	if !info.Supported {
		return &prContext{Info: info}, nil
	}
	if token == "" {
		return &prContext{Info: info}, nil
	}
	return &prContext{Info: info, Token: token, API: newPrAPI(info.Provider, info.Host, token)}, nil
}

// ------------------------------------------------------------ http layer ----

type prHTTPError struct {
	Status int
	Body   string
	Path   string
}

func (e *prHTTPError) Error() string {
	return fmt.Sprintf("HTTP %d on %s: %s", e.Status, e.Path, prSnippet(e.Body))
}

type prAPI struct {
	provider string
	baseURL  string
	token    string
	client   *http.Client
}

func newPrAPI(provider, host, token string) *prAPI {
	return &prAPI{
		provider: provider,
		baseURL:  prAPIBase(provider, host),
		token:    token,
		client:   &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *prAPI) do(method, path string, query url.Values, payload any, out any) error {
	if c.baseURL == "" {
		return errors.New("该托管平台暂不支持 API 访问")
	}
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "wgit/1.0")
	if c.provider == "gitlab" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
		req.Header.Set("Accept", "application/json")
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("网络请求失败：%w", err)
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, prMaxBodyBytes))
	if readErr != nil {
		return fmt.Errorf("读取 %s 响应失败：%w", path, readErr)
	}

	if resp.StatusCode >= 400 {
		return &prHTTPError{Status: resp.StatusCode, Body: string(data), Path: path}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("解析 %s 响应失败：%w", path, err)
		}
	}
	return nil
}

func describePrError(err error) error {
	var httpErr *prHTTPError
	if errors.As(err, &httpErr) {
		lower := strings.ToLower(httpErr.Body)
		switch {
		case httpErr.Status == 401:
			return errors.New("访问令牌无效或已过期，请重新设置")
		case httpErr.Status == 403 && strings.Contains(lower, "rate limit"):
			return errors.New("托管平台 API 速率限制已用尽，请稍后重试或更换令牌")
		case httpErr.Status == 403:
			return errors.New("令牌权限不足（GitHub 需 repo，GitLab 需 api / read_api）")
		case httpErr.Status == 404:
			return errors.New("未找到仓库或 PR/MR：请确认令牌拥有该仓库权限")
		case httpErr.Status == 409, httpErr.Status == 422:
			return fmt.Errorf("平台拒绝该操作：%s", prSnippet(httpErr.Body))
		default:
			return fmt.Errorf("平台返回 %d：%s", httpErr.Status, prSnippet(httpErr.Body))
		}
	}
	return err
}

func prSnippet(raw string) string {
	text := strings.TrimSpace(raw)
	var parsed map[string]any
	if json.Unmarshal([]byte(text), &parsed) == nil {
		for _, key := range []string{"message", "error", "errors"} {
			if value, ok := parsed[key]; ok {
				if s, ok := value.(string); ok && s != "" {
					text = s
					break
				}
			}
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 240 {
		text = text[:240] + "…"
	}
	if text == "" {
		text = "无错误详情"
	}
	return text
}

// --------------------------------------------------------------- GitHub ----

type ghUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type ghPR struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	State          string    `json:"state"`
	Draft          bool      `json:"draft"`
	Body           string    `json:"body"`
	Merged         bool      `json:"merged"`
	Mergeable      *bool     `json:"mergeable"`
	MergeableState string    `json:"mergeable_state"`
	User           ghUser    `json:"user"`
	Head           ghRef     `json:"head"`
	Base           ghRef     `json:"base"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
	Comments       int       `json:"comments"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ChangedFiles   int       `json:"changed_files"`
	Commits        int       `json:"commits"`
	HTMLURL        string    `json:"html_url"`
	Labels         []ghLabel `json:"labels"`
}

type ghFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	Patch            string `json:"patch"`
}

type ghComment struct {
	ID           int64  `json:"id"`
	User         ghUser `json:"user"`
	Body         string `json:"body"`
	Path         string `json:"path"`
	Side         string `json:"side"`
	Line         int    `json:"line"`
	OriginalLine int    `json:"original_line"`
	CreatedAt    string `json:"created_at"`
	InReplyTo    *int64 `json:"in_reply_to_id"`
}

type ghCheckRuns struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HTMLURL    string `json:"html_url"`
		Output     struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
		} `json:"output"`
	} `json:"check_runs"`
}

type ghCombinedStatus struct {
	State    string `json:"state"`
	Statuses []struct {
		Context     string `json:"context"`
		State       string `json:"state"`
		TargetURL   string `json:"target_url"`
		Description string `json:"description"`
	} `json:"statuses"`
}

func ghPrState(pr ghPR) string {
	if pr.Merged {
		return "merged"
	}
	if pr.State == "" {
		return "open"
	}
	return pr.State
}

func mapGhSummary(pr ghPR) PrSummary {
	summary := PrSummary{
		Number:         pr.Number,
		Title:          pr.Title,
		State:          ghPrState(pr),
		IsDraft:        pr.Draft,
		Author:         pr.User.Login,
		SourceBranch:   pr.Head.Ref,
		TargetBranch:   pr.Base.Ref,
		CreatedAt:      pr.CreatedAt,
		UpdatedAt:      pr.UpdatedAt,
		CommentCount:   pr.Comments,
		Additions:      pr.Additions,
		Deletions:      pr.Deletions,
		ChangedFiles:   pr.ChangedFiles,
		WebURL:         pr.HTMLURL,
		MergeableState: pr.MergeableState,
	}
	for _, label := range pr.Labels {
		if label.Name != "" {
			summary.Labels = append(summary.Labels, label.Name)
		}
	}
	if pr.Mergeable != nil {
		summary.Mergeable = *pr.Mergeable
	}
	if summary.MergeableState == "" {
		summary.MergeableState = "unknown"
	}
	return summary
}

func ghRepoPath(info *PrProviderInfo) string {
	return fmt.Sprintf("/repos/%s/%s", url.PathEscape(info.Owner), url.PathEscape(info.Repo))
}

func ghCheckState(status string, conclusion string) string {
	switch strings.ToLower(status) {
	case "queued", "in_progress", "waiting", "pending", "requested":
		return "pending"
	}
	switch strings.ToLower(conclusion) {
	case "success":
		return "success"
	case "failure", "timed_out", "cancelled", "action_required", "startup_failure":
		return "failure"
	case "neutral", "skipped", "stale":
		return "neutral"
	}
	return ""
}

func prWorstState(states ...string) string {
	rank := map[string]int{"": 0, "neutral": 1, "success": 2, "pending": 3, "failure": 4}
	worst := ""
	for _, state := range states {
		if rank[state] > rank[worst] {
			worst = state
		}
	}
	return worst
}

// --------------------------------------------------------------- GitLab ----

type glMR struct {
	IID         int    `json:"iid"`
	Title       string `json:"title"`
	State       string `json:"state"`
	Draft       bool   `json:"draft"`
	Description string `json:"description"`
	Author      struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	SourceBranch        string   `json:"source_branch"`
	TargetBranch        string   `json:"target_branch"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	UserNotesCount      int      `json:"user_notes_count"`
	WebURL              string   `json:"web_url"`
	Labels              []string `json:"labels"`
	HasConflicts        bool     `json:"has_conflicts"`
	MergeStatus         string   `json:"merge_status"`
	DetailedMergeStatus string   `json:"detailed_merge_status"`
	ChangesCount        string   `json:"changes_count"`
	SHA                 string   `json:"sha"`
	DiffRefs            struct {
		BaseSHA  string `json:"base_sha"`
		HeadSHA  string `json:"head_sha"`
		StartSHA string `json:"start_sha"`
	} `json:"diff_refs"`
}

func glMrState(mr glMR) string {
	switch strings.ToLower(mr.State) {
	case "merged":
		return "merged"
	case "closed", "locked":
		return "closed"
	default:
		return "open"
	}
}

func glChangeCount(mr glMR) int {
	if n, err := strconv.Atoi(strings.TrimSpace(mr.ChangesCount)); err == nil {
		return n
	}
	return 0
}

func mapGlSummary(mr glMR) PrSummary {
	mergeable := !mr.HasConflicts && strings.ToLower(mr.MergeStatus) != "cannot_be_merged"
	state := "unknown"
	switch {
	case mr.HasConflicts:
		state = "conflicting"
	case strings.EqualFold(mr.MergeStatus, "can_be_merged"):
		state = "mergeable"
	}
	summary := PrSummary{
		Number:         mr.IID,
		Title:          mr.Title,
		State:          glMrState(mr),
		IsDraft:        mr.Draft || strings.HasPrefix(strings.ToLower(mr.Title), "draft:"),
		Author:         mr.Author.Username,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		CreatedAt:      mr.CreatedAt,
		UpdatedAt:      mr.UpdatedAt,
		CommentCount:   mr.UserNotesCount,
		ChangedFiles:   glChangeCount(mr),
		WebURL:         mr.WebURL,
		Labels:         mr.Labels,
		Mergeable:      mergeable,
		MergeableState: state,
	}
	return summary
}

func glProjectPath(info *PrProviderInfo) string {
	return "/projects/" + url.PathEscape(info.ProjectPath)
}

// ---------------------------------------------------------------- RPCs ----

// GetPullRequestProvider reports which hosting provider the repo points at and
// whether a usable token is already available (never returns the token).
func (GitService) GetPullRequestProvider(repoPath string) (*PrProviderInfo, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	info, _ := buildPrProviderInfo(repoPath)
	return info, nil
}

// SavePullRequestToken verifies the token against the provider, then stores it
// encrypted in .git/wgit_vault.json.
func (s GitService) SavePullRequestToken(repoPath, token string) (*PrProviderInfo, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("令牌不能为空")
	}
	info, _ := buildPrProviderInfo(repoPath)
	if !info.Supported {
		if info.Message != "" {
			return nil, errors.New(info.Message)
		}
		return nil, errors.New("当前仓库的远端不受支持")
	}

	account, verifyErr := newPrAPI(info.Provider, info.Host, token).verifyAccount()
	if verifyErr != nil {
		var httpErr *prHTTPError
		if errors.As(verifyErr, &httpErr) && (httpErr.Status == 401 || httpErr.Status == 403) {
			return nil, fmt.Errorf("令牌校验失败：%s", describePrError(verifyErr))
		}
	}

	if _, err := s.StoreCredentialSecret(repoPath, prVaultSecretType, info.Host, token, "PR_API"); err != nil {
		return nil, err
	}

	updated, _ := buildPrProviderInfo(repoPath)
	if updated == nil {
		updated = info
	}
	updated.Account = account
	updated.HasToken = true
	if updated.TokenSource == "" {
		updated.TokenSource = "本地保险库"
	}
	if verifyErr != nil {
		updated.Message = "令牌已加密保存，但本次在线校验失败（" + verifyErr.Error() + "），请检查网络后重试"
	} else {
		updated.Message = "令牌已通过在线校验并加密保存到本地保险库"
		if account != "" {
			updated.Message += "，当前账号：" + account
		}
	}
	return updated, nil
}

// ClearPullRequestToken removes stored tokens for this repo's host.
func (s GitService) ClearPullRequestToken(repoPath string) (*PrProviderInfo, error) {
	info, _ := buildPrProviderInfo(repoPath)
	if info.Host == "" {
		return info, errors.New("当前仓库未配置可识别的远端")
	}

	vaultFile := getVaultFilePath(repoPath)
	data, err := os.ReadFile(vaultFile)
	if err != nil {
		if os.IsNotExist(err) {
			return info, nil
		}
		return nil, err
	}
	var records []EncryptedVaultRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("解析本地凭据库失败，已中止写入以保护现有记录: %w", err)
	}

	kept := make([]EncryptedVaultRecord, 0, len(records))
	removed := 0
	for _, record := range records {
		if record.SecretType == prVaultSecretType && record.Identifier == info.Host {
			removed++
			continue
		}
		kept = append(kept, record)
	}
	if removed == 0 {
		return info, nil
	}

	out, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(vaultFile, out, 0600); err != nil {
		return nil, err
	}
	if err := os.Chmod(vaultFile, 0600); err != nil {
		return nil, fmt.Errorf("令牌记录已移除，但收紧凭据库权限失败: %w", err)
	}

	updated, _ := buildPrProviderInfo(repoPath)
	if updated != nil {
		updated.Message = fmt.Sprintf("已从本地保险库移除 %d 条令牌记录", removed)
		return updated, nil
	}
	return info, nil
}

func (c *prAPI) verifyAccount() (string, error) {
	var user struct {
		Login    string `json:"login"`
		Username string `json:"username"`
	}
	if err := c.do("GET", "/user", nil, nil, &user); err != nil {
		return "", err
	}
	if user.Login != "" {
		return user.Login, nil
	}
	return user.Username, nil
}

// ListPullRequests lists open / closed / merged / all PRs.
func (GitService) ListPullRequests(repoPath string, state string, limit int) (*PrListResult, error) {
	ctx, err := loadPrContext(repoPath)
	if err != nil {
		return nil, err
	}
	result := &PrListResult{
		Provider:   ctx.Info.Provider,
		Items:      []PrSummary{},
		FetchedAt:  time.Now().Format(time.RFC3339),
		WebBaseURL: ctx.Info.WebBaseURL,
	}
	if !ctx.Info.Supported {
		result.Message = ctx.Info.Message
		return result, nil
	}
	if ctx.API == nil {
		result.Message = ctx.Info.Message
		if result.Message == "" {
			result.Message = "未配置访问令牌"
		}
		return result, nil
	}

	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "open"
	}

	switch ctx.Info.Provider {
	case "github":
		ghState := state
		if ghState == "merged" {
			ghState = "closed"
		}
		query := url.Values{
			"state":     {ghState},
			"per_page":  {strconv.Itoa(limit)},
			"sort":      {"updated"},
			"direction": {"desc"},
		}
		var pulls []ghPR
		if err := ctx.API.do("GET", ghRepoPath(ctx.Info)+"/pulls", query, nil, &pulls); err != nil {
			return nil, describePrError(err)
		}
		for _, pull := range pulls {
			if state == "merged" && !pull.Merged {
				continue
			}
			result.Items = append(result.Items, mapGhSummary(pull))
		}
		result.HasMore = len(pulls) == limit
	case "gitlab":
		glState := state
		if glState == "open" {
			glState = "opened"
		}
		query := url.Values{
			"state":    {glState},
			"per_page": {strconv.Itoa(limit)},
			"order_by": {"updated_at"},
			"sort":     {"desc"},
		}
		var mrs []glMR
		if err := ctx.API.do("GET", glProjectPath(ctx.Info)+"/merge_requests", query, nil, &mrs); err != nil {
			return nil, describePrError(err)
		}
		for _, mr := range mrs {
			result.Items = append(result.Items, mapGlSummary(mr))
		}
		result.HasMore = len(mrs) == limit
	}

	result.Total = len(result.Items)
	if result.Total == 0 {
		result.Message = "没有匹配的 PR/MR"
	}
	return result, nil
}

func requirePrContext(repoPath string) (*prContext, error) {
	ctx, err := loadPrContext(repoPath)
	if err != nil {
		return nil, err
	}
	if !ctx.Info.Supported {
		if ctx.Info.Message != "" {
			return nil, errors.New(ctx.Info.Message)
		}
		return nil, errors.New("当前仓库的远端托管平台不受支持")
	}
	if ctx.API == nil {
		if ctx.Info.Message != "" {
			return nil, errors.New(ctx.Info.Message)
		}
		return nil, errors.New("未配置访问令牌")
	}
	return ctx, nil
}

func truncatePrPatch(patch string) (string, bool) {
	if len(patch) <= prMaxPatchBytes {
		return patch, false
	}
	return patch[:prMaxPatchBytes], true
}

func countPatchLines(patch string) (int, int) {
	adds, dels := 0, 0
	for _, line := range strings.Split(patch, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			adds++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			dels++
		}
	}
	return adds, dels
}

func prStateFromStatus(raw string) string {
	switch strings.ToLower(raw) {
	case "success":
		return "success"
	case "pending", "running", "queued", "in_progress", "waiting", "created", "preparing", "manual", "scheduled":
		return "pending"
	case "failure", "failed", "error", "cancelled", "canceled", "timed_out", "action_required":
		return "failure"
	case "neutral", "skipped", "stale":
		return "neutral"
	}
	return ""
}

func int64PtrString(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

// GetPullRequestDetail loads the PR, its file patches, discussion and checks.
func (GitService) GetPullRequestDetail(repoPath string, number int) (*PrDetail, error) {
	if number <= 0 {
		return nil, errors.New("PR/MR 编号必须大于 0")
	}
	ctx, err := requirePrContext(repoPath)
	if err != nil {
		return nil, err
	}

	detail := &PrDetail{Files: []PrFile{}, Comments: []PrComment{}, Checks: []PrCheck{}}
	switch ctx.Info.Provider {
	case "github":
		err = fillGithubDetail(ctx, number, detail)
	case "gitlab":
		err = fillGitlabDetail(ctx, number, detail)
	default:
		return nil, errors.New("当前托管平台暂不支持 PR 详情")
	}
	if err != nil {
		return nil, describePrError(err)
	}
	return detail, nil
}

func fillGithubDetail(ctx *prContext, number int, detail *PrDetail) error {
	base := fmt.Sprintf("%s/pulls/%d", ghRepoPath(ctx.Info), number)

	var pull ghPR
	if err := ctx.API.do("GET", base, nil, nil, &pull); err != nil {
		return err
	}
	detail.Summary = mapGhSummary(pull)
	detail.Body = pull.Body
	detail.CommitCount = pull.Commits
	detail.BaseSHA = pull.Base.SHA
	detail.HeadSHA = pull.Head.SHA
	detail.StartSHA = pull.Base.SHA

	var files []ghFile
	if err := ctx.API.do("GET", base+"/files", url.Values{"per_page": {"100"}}, nil, &files); err != nil {
		return err
	}
	for _, file := range files {
		patch, truncated := truncatePrPatch(file.Patch)
		detail.Files = append(detail.Files, PrFile{
			Path:      file.Filename,
			OldPath:   file.PreviousFilename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Patch:     patch,
			Truncated: truncated,
			Binary:    file.Patch == "" && file.Additions == 0 && file.Deletions == 0,
		})
	}
	detail.FilesTruncated = len(files) == 100 && pull.ChangedFiles > len(files)

	var reviewComments []ghComment
	if err := ctx.API.do("GET", base+"/comments", url.Values{"per_page": {"100"}}, nil, &reviewComments); err != nil {
		return err
	}
	for _, comment := range reviewComments {
		line := comment.Line
		if line == 0 {
			line = comment.OriginalLine
		}
		detail.Comments = append(detail.Comments, PrComment{
			ID:        strconv.FormatInt(comment.ID, 10),
			Author:    comment.User.Login,
			Body:      comment.Body,
			Path:      comment.Path,
			Line:      line,
			Side:      comment.Side,
			CreatedAt: comment.CreatedAt,
			Kind:      "review",
			ReplyTo:   int64PtrString(comment.InReplyTo),
		})
	}

	var issueComments []ghComment
	if err := ctx.API.do("GET", fmt.Sprintf("%s/issues/%d/comments", ghRepoPath(ctx.Info), number), url.Values{"per_page": {"100"}}, nil, &issueComments); err != nil {
		return err
	}
	for _, comment := range issueComments {
		detail.Comments = append(detail.Comments, PrComment{
			ID:        strconv.FormatInt(comment.ID, 10),
			Author:    comment.User.Login,
			Body:      comment.Body,
			CreatedAt: comment.CreatedAt,
			Kind:      "issue",
		})
	}

	if pull.Head.SHA != "" {
		states := []string{}
		var runs ghCheckRuns
		checksPath := fmt.Sprintf("%s/commits/%s/check-runs", ghRepoPath(ctx.Info), url.PathEscape(pull.Head.SHA))
		if err := ctx.API.do("GET", checksPath, url.Values{"per_page": {"50"}}, nil, &runs); err == nil {
			for _, run := range runs.CheckRuns {
				states = append(states, ghCheckState(run.Status, run.Conclusion))
				detail.Checks = append(detail.Checks, PrCheck{
					Name:       run.Name,
					Status:     run.Status,
					Conclusion: run.Conclusion,
					URL:        run.HTMLURL,
					Detail:     run.Output.Title,
				})
			}
		}

		var combined ghCombinedStatus
		statusPath := fmt.Sprintf("%s/commits/%s/status", ghRepoPath(ctx.Info), url.PathEscape(pull.Head.SHA))
		if err := ctx.API.do("GET", statusPath, nil, nil, &combined); err == nil {
			for _, status := range combined.Statuses {
				states = append(states, prStateFromStatus(status.State))
				detail.Checks = append(detail.Checks, PrCheck{
					Name:       status.Context,
					Status:     status.State,
					Conclusion: status.State,
					URL:        status.TargetURL,
					Detail:     status.Description,
				})
			}
			if len(combined.Statuses) == 0 && combined.State != "" {
				states = append(states, prStateFromStatus(combined.State))
			}
		}
		detail.Summary.ChecksState = prWorstState(states...)
	}

	detail.Message = fmt.Sprintf("已加载 %d 个文件变更、%d 条讨论、%d 项检查",
		len(detail.Files), len(detail.Comments), len(detail.Checks))
	return nil
}

type glNote struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	System    bool   `json:"system"`
	CreatedAt string `json:"created_at"`
	Author    struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	Position *struct {
		NewPath string `json:"new_path"`
		OldPath string `json:"old_path"`
		NewLine int    `json:"new_line"`
		OldLine int    `json:"old_line"`
	} `json:"position"`
}

type glPipeline struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
	Ref    string `json:"ref"`
}

func fillGitlabDetail(ctx *prContext, number int, detail *PrDetail) error {
	base := fmt.Sprintf("%s/merge_requests/%d", glProjectPath(ctx.Info), number)

	var mr glMR
	if err := ctx.API.do("GET", base, nil, nil, &mr); err != nil {
		return err
	}
	detail.Summary = mapGlSummary(mr)
	detail.Body = mr.Description
	detail.BaseSHA = mr.DiffRefs.BaseSHA
	detail.HeadSHA = mr.DiffRefs.HeadSHA
	detail.StartSHA = mr.DiffRefs.StartSHA

	var changes struct {
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := ctx.API.do("GET", base+"/changes", nil, nil, &changes); err != nil {
		return err
	}
	for _, change := range changes.Changes {
		status := "modified"
		switch {
		case change.NewFile:
			status = "added"
		case change.DeletedFile:
			status = "removed"
		case change.RenamedFile:
			status = "renamed"
		}
		adds, dels := countPatchLines(change.Diff)
		patch, truncated := truncatePrPatch(change.Diff)
		detail.Files = append(detail.Files, PrFile{
			Path:      change.NewPath,
			OldPath:   change.OldPath,
			Status:    status,
			Additions: adds,
			Deletions: dels,
			Patch:     patch,
			Truncated: truncated,
			Binary:    change.Diff == "",
		})
	}

	var notes []glNote
	if err := ctx.API.do("GET", base+"/notes", url.Values{"per_page": {"100"}, "sort": {"asc"}}, nil, &notes); err != nil {
		return err
	}
	for _, note := range notes {
		if note.System {
			continue
		}
		comment := PrComment{
			ID:        strconv.FormatInt(note.ID, 10),
			Author:    note.Author.Username,
			Body:      note.Body,
			CreatedAt: note.CreatedAt,
			Kind:      "issue",
		}
		if note.Position != nil {
			comment.Kind = "review"
			comment.Path = note.Position.NewPath
			if comment.Path == "" {
				comment.Path = note.Position.OldPath
			}
			comment.Line = note.Position.NewLine
			if comment.Line == 0 {
				comment.Line = note.Position.OldLine
				comment.Side = "old"
			} else {
				comment.Side = "new"
			}
		}
		detail.Comments = append(detail.Comments, comment)
	}

	var pipelines []glPipeline
	if err := ctx.API.do("GET", base+"/pipelines", url.Values{"per_page": {"20"}}, nil, &pipelines); err == nil {
		states := []string{}
		for _, pipeline := range pipelines {
			states = append(states, prStateFromStatus(pipeline.Status))
			detail.Checks = append(detail.Checks, PrCheck{
				Name:       fmt.Sprintf("Pipeline #%d", pipeline.ID),
				Status:     pipeline.Status,
				Conclusion: pipeline.Status,
				URL:        pipeline.WebURL,
				Detail:     pipeline.Ref,
			})
		}
		detail.Summary.ChecksState = prWorstState(states...)
	}

	detail.Message = fmt.Sprintf("已加载 %d 个文件变更、%d 条讨论、%d 条流水线",
		len(detail.Files), len(detail.Comments), len(detail.Checks))
	return nil
}

// CreatePullRequestComment posts a general comment, or a line-anchored review
// comment when filePath + line are supplied.
func (GitService) CreatePullRequestComment(repoPath string, number int, body, filePath string, line int, side string) (*PrActionResult, error) {
	if number <= 0 {
		return nil, errors.New("PR/MR 编号必须大于 0")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("评论内容不能为空")
	}
	filePath = strings.TrimSpace(filePath)
	if line < 0 {
		line = 0
	}

	ctx, err := requirePrContext(repoPath)
	if err != nil {
		return nil, err
	}

	switch ctx.Info.Provider {
	case "github":
		base := fmt.Sprintf("%s/pulls/%d", ghRepoPath(ctx.Info), number)
		if filePath != "" && line > 0 {
			var pull ghPR
			if err := ctx.API.do("GET", base, nil, nil, &pull); err != nil {
				return nil, describePrError(err)
			}
			ghSide := "right"
			if strings.EqualFold(side, "old") || strings.EqualFold(side, "left") {
				ghSide = "left"
			}
			payload := map[string]any{
				"body":      body,
				"commit_id": pull.Head.SHA,
				"path":      filePath,
				"line":      line,
				"side":      ghSide,
			}
			if err := ctx.API.do("POST", base+"/comments", nil, payload, nil); err != nil {
				return nil, describePrError(err)
			}
			return &PrActionResult{
				Success: true,
				Action:  "comment",
				Message: fmt.Sprintf("已在 %s:%d 发表行内评论", filePath, line),
				WebURL:  pull.HTMLURL,
			}, nil
		}
		payload := map[string]any{"body": body}
		if err := ctx.API.do("POST", fmt.Sprintf("%s/issues/%d/comments", ghRepoPath(ctx.Info), number), nil, payload, nil); err != nil {
			return nil, describePrError(err)
		}
		return &PrActionResult{Success: true, Action: "comment", Message: "评论已发布", WebURL: ctx.Info.WebBaseURL}, nil

	case "gitlab":
		base := fmt.Sprintf("%s/merge_requests/%d", glProjectPath(ctx.Info), number)
		if filePath != "" && line > 0 {
			var mr glMR
			if err := ctx.API.do("GET", base, nil, nil, &mr); err != nil {
				return nil, describePrError(err)
			}
			position := map[string]any{
				"position_type": "text",
				"base_sha":      mr.DiffRefs.BaseSHA,
				"head_sha":      mr.DiffRefs.HeadSHA,
				"start_sha":     mr.DiffRefs.StartSHA,
				"new_path":      filePath,
				"old_path":      filePath,
			}
			if strings.EqualFold(side, "old") || strings.EqualFold(side, "left") {
				position["old_line"] = line
			} else {
				position["new_line"] = line
			}
			payload := map[string]any{"body": body, "position": position}
			if err := ctx.API.do("POST", base+"/discussions", nil, payload, nil); err != nil {
				return nil, describePrError(err)
			}
			return &PrActionResult{
				Success: true,
				Action:  "comment",
				Message: fmt.Sprintf("已在 %s:%d 发表行内评论", filePath, line),
				WebURL:  mr.WebURL,
			}, nil
		}
		payload := map[string]any{"body": body}
		if err := ctx.API.do("POST", base+"/notes", nil, payload, nil); err != nil {
			return nil, describePrError(err)
		}
		return &PrActionResult{Success: true, Action: "comment", Message: "评论已发布", WebURL: ctx.Info.WebBaseURL}, nil
	}

	return nil, errors.New("当前托管平台暂不支持评论")
}

// SubmitPullRequestReview records a verdict: approve / request_changes / comment.
func (GitService) SubmitPullRequestReview(repoPath string, number int, event string, body string) (*PrActionResult, error) {
	if number <= 0 {
		return nil, errors.New("PR/MR 编号必须大于 0")
	}
	event = strings.ToLower(strings.TrimSpace(event))
	switch event {
	case "approve", "request_changes", "comment":
	default:
		return nil, errors.New("评审动作仅支持 approve / request_changes / comment")
	}
	body = strings.TrimSpace(body)
	if event == "request_changes" && body == "" {
		return nil, errors.New("请求修改需要填写说明，便于作者知道要改什么")
	}

	ctx, err := requirePrContext(repoPath)
	if err != nil {
		return nil, err
	}

	switch ctx.Info.Provider {
	case "github":
		ghEvent := map[string]string{
			"approve":         "APPROVE",
			"request_changes": "REQUEST_CHANGES",
			"comment":         "COMMENT",
		}[event]
		payload := map[string]any{"event": ghEvent}
		if body != "" {
			payload["body"] = body
		}
		path := fmt.Sprintf("%s/pulls/%d/reviews", ghRepoPath(ctx.Info), number)
		if err := ctx.API.do("POST", path, nil, payload, nil); err != nil {
			return nil, describePrError(err)
		}
		label := map[string]string{"approve": "已批准", "request_changes": "已请求修改", "comment": "已留下评审意见"}[event]
		return &PrActionResult{Success: true, Action: event, Message: label, WebURL: ctx.Info.WebBaseURL}, nil

	case "gitlab":
		base := fmt.Sprintf("%s/merge_requests/%d", glProjectPath(ctx.Info), number)
		if event == "approve" {
			// GitLab's approve endpoint accepts an optional sha; a free-form
			// comment must go through notes, so keep this call payload-free.
			if err := ctx.API.do("POST", base+"/approve", nil, map[string]any{}, nil); err != nil {
				return nil, describePrError(err)
			}
			return &PrActionResult{Success: true, Action: event, Message: "已批准该 MR", WebURL: ctx.Info.WebBaseURL}, nil
		}
		prefix := "💬 "
		message := "已留下评审意见"
		if event == "request_changes" {
			prefix = "🔴 请求修改："
			message = "GitLab 没有原生 Request changes，已作为讨论记录（作者会收到通知）"
		}
		payload := map[string]any{"body": prefix + body}
		if err := ctx.API.do("POST", base+"/notes", nil, payload, nil); err != nil {
			return nil, describePrError(err)
		}
		return &PrActionResult{Success: true, Action: event, Message: message, WebURL: ctx.Info.WebBaseURL}, nil
	}

	return nil, errors.New("当前托管平台暂不支持评审动作")
}

// MergePullRequest merges with merge / squash / rebase semantics.
func (GitService) MergePullRequest(repoPath string, number int, method string) (*PrActionResult, error) {
	if number <= 0 {
		return nil, errors.New("PR/MR 编号必须大于 0")
	}
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		method = "merge"
	}
	switch method {
	case "merge", "squash", "rebase":
	default:
		return nil, errors.New("合并方式仅支持 merge / squash / rebase")
	}

	ctx, err := requirePrContext(repoPath)
	if err != nil {
		return nil, err
	}

	switch ctx.Info.Provider {
	case "github":
		payload := map[string]any{"merge_method": method}
		path := fmt.Sprintf("%s/pulls/%d/merge", ghRepoPath(ctx.Info), number)
		var response struct {
			Merged  bool   `json:"merged"`
			Message string `json:"message"`
			SHA     string `json:"sha"`
		}
		if err := ctx.API.do("PUT", path, nil, payload, &response); err != nil {
			return nil, describePrError(err)
		}
		if !response.Merged {
			message := response.Message
			if message == "" {
				message = "平台未合并该 PR，请检查分支保护规则或冲突状态"
			}
			return &PrActionResult{Success: false, Action: "merge", Message: message, NewState: "open"}, nil
		}
		return &PrActionResult{
			Success:  true,
			Action:   "merge",
			Message:  fmt.Sprintf("已按 %s 方式合并，合并提交 %s", method, prShortHash(response.SHA)),
			NewState: "merged",
			WebURL:   ctx.Info.WebBaseURL,
		}, nil

	case "gitlab":
		if method == "rebase" {
			return nil, errors.New("GitLab 合并接口不支持 rebase，请改用 merge 或 squash")
		}
		payload := map[string]any{"squash": method == "squash"}
		path := fmt.Sprintf("%s/merge_requests/%d/merge", glProjectPath(ctx.Info), number)
		var response struct {
			State   string `json:"state"`
			Message string `json:"message"`
		}
		if err := ctx.API.do("PUT", path, nil, payload, &response); err != nil {
			return nil, describePrError(err)
		}
		state := response.State
		if state == "" {
			state = "merged"
		}
		return &PrActionResult{
			Success:  strings.EqualFold(state, "merged"),
			Action:   "merge",
			Message:  fmt.Sprintf("MR 状态：%s", state),
			NewState: state,
			WebURL:   ctx.Info.WebBaseURL,
		}, nil
	}

	return nil, errors.New("当前托管平台暂不支持合并")
}

func prShortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	if hash == "" {
		return "(未知)"
	}
	return hash
}
