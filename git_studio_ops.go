package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Studio 协作、CI 触发、团队活动、自定义流水线与凭据/密钥巡检。

// GetActivityLogs returns recent executed git command history
func (GitService) GetActivityLogs() []GitCommandLog {
	return GetRecentActivityLogs()
}

// ClearActivityLogs clears the command history
func (GitService) ClearActivityLogs() error {
	ClearRecentActivityLogs()
	return nil
}

var gitAuthSSHTimeout = 8 * time.Second

// DiagnoseGitAuth checks SSH keys and remote connectivity
func (GitService) DiagnoseGitAuth(repoPath string, host string) (*AuthDiagnosticResult, error) {
	if host == "" {
		host = "github.com"
	}
	res := &AuthDiagnosticResult{Host: host}

	// Find ~/.ssh keys
	homeDir, err := os.UserHomeDir()
	if err == nil {
		sshDir := filepath.Join(homeDir, ".ssh")
		entries, err := os.ReadDir(sshDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".pub") {
					res.SSHKeysFound = append(res.SSHKeysFound, entry.Name())
				}
			}
		}
	}

	// Credential helper. `git config` exits 1 when the key is unset — the
	// ordinary "no helper configured" state the diagnostic reports as empty,
	// not an error. The SSH verdict below is derived from the ssh probe alone,
	// so this informational read stays best-effort and cannot fake a pass.
	helper, _ := ExecGit(repoPath, "config", "credential.helper")
	res.CredentialHelper = strings.TrimSpace(helper)

	// Quick SSH test with 3s connect timeout plus an overall command deadline.
	ctx, cancel := context.WithTimeout(context.Background(), gitAuthSSHTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=3", fmt.Sprintf("git@%s", host))
	// ssh 可能是脚本包装：deadline 到点只杀直接子进程，孙进程仍持有
	// stdout/stderr，CombinedOutput 会一直等到管道 EOF。WaitDelay 兜底，
	// 保证诊断在 deadline 之后立即返回。
	cmd.WaitDelay = 250 * time.Millisecond
	out, _ := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		outStr = "SSH 诊断超时，未能完成连接测试。"
	}
	res.SSHAuthOutput = outStr
	// Typical success: "Hi username! You've successfully authenticated..."
	if strings.Contains(outStr, "successfully authenticated") || strings.Contains(outStr, "Welcome to") {
		res.SSHAuthSuccess = true
	}

	return res, nil
}

// ScanStagedSecrets scans staged changes for potential sensitive credentials, private keys, and tokens
func (GitService) ScanStagedSecrets(repoPath string) ([]SecretFinding, error) {
	findings := make([]SecretFinding, 0)

	// 1. Check staged file paths for high-risk sensitive file types
	namesOut, err := ExecGit(repoPath, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("读取暂存文件列表失败（无法完成密钥扫描）: %w", err)
	}
	if namesOut != "" {
		for _, rawPath := range strings.Split(namesOut, "\n") {
			path := strings.TrimSpace(rawPath)
			if path == "" {
				continue
			}
			base := strings.ToLower(filepath.Base(path))
			ext := strings.ToLower(filepath.Ext(path))

			// Check private key or keystore files
			if ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" || ext == ".keystore" || ext == ".jks" ||
				base == "id_rsa" || base == "id_ed25519" || base == "id_ecdsa" || base == "id_dsa" {
				findings = append(findings, SecretFinding{
					FilePath:        path,
					LineNumber:      1,
					RuleName:        "敏感证书/私钥文件 (Private Key / Keystore File)",
					Severity:        "CRITICAL",
					RedactedSnippet: fmt.Sprintf("[Staged sensitive file: %s]", base),
				})
			} else if base == ".env" || base == ".env.local" || base == ".env.production" || base == ".env.staging" || base == ".env.development" {
				findings = append(findings, SecretFinding{
					FilePath:        path,
					LineNumber:      1,
					RuleName:        "敏感环境变量文件 (Environment File)",
					Severity:        "WARNING",
					RedactedSnippet: fmt.Sprintf("[Staged environment config: %s]", base),
				})
			}
		}
	}

	// 2. Scan diff for secret patterns in added lines
	diffOut, err := ExecGit(repoPath, "diff", "--cached", "-U0")
	if err != nil {
		return nil, fmt.Errorf("读取暂存差异失败（无法完成密钥扫描）: %w", err)
	}
	if diffOut == "" {
		return findings, nil
	}

	patterns := []struct {
		name     string
		severity string
		regex    *regexp.Regexp
	}{
		{
			name:     "私有密钥 (Private Key Header)",
			severity: "CRITICAL",
			regex:    regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
		},
		{
			name:     "腾讯云 SecretId (Tencent Cloud)",
			severity: "CRITICAL",
			regex:    regexp.MustCompile(`AKID[a-zA-Z0-9]{32}`),
		},
		{
			name:     "AWS Access Key ID",
			severity: "CRITICAL",
			regex:    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		},
		{
			name:     "GitHub 访问令牌 (GitHub PAT)",
			severity: "CRITICAL",
			regex:    regexp.MustCompile(`(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9_]{82})`),
		},
		{
			name:     "OpenAI / AI 服务密钥",
			severity: "HIGH",
			regex:    regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
		},
		{
			name:     "Webhook 凭据 (Slack/Discord)",
			severity: "HIGH",
			regex:    regexp.MustCompile(`(https:\/\/hooks\.slack\.com\/services\/[A-Za-z0-9+\/]{30,}|https:\/\/discord\.com\/api\/webhooks\/[0-9]+\/[A-Za-z0-9_\-]+)`),
		},
		{
			name:     "硬编码 API/Secret 凭据",
			severity: "HIGH",
			regex:    regexp.MustCompile(`(?i)(api[_-]?secret|jwt[_-]?secret|db[_-]?password|secret[_-]?key)\s*[:=]\s*['"][a-zA-Z0-9_!@#$%^&*\-]{10,}['"]`),
		},
	}

	currentFile := ""
	currentLine := 0
	hunkRegex := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

	lines := strings.Split(diffOut, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			currentLine = 0
			continue
		}
		if strings.HasPrefix(line, "+++ /dev/null") {
			currentFile = ""
			continue
		}

		if m := hunkRegex.FindStringSubmatch(line); len(m) > 1 {
			start, _ := strconv.Atoi(m[1])
			currentLine = start - 1
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			currentLine++
			addedContent := strings.TrimPrefix(line, "+")

			for _, p := range patterns {
				if loc := p.regex.FindStringIndex(addedContent); loc != nil {
					redacted := redactSecretText(addedContent, loc[0], loc[1])
					findings = append(findings, SecretFinding{
						FilePath:        currentFile,
						LineNumber:      currentLine,
						RuleName:        p.name,
						Severity:        p.severity,
						RedactedSnippet: redacted,
					})
					break
				}
			}
		} else if strings.HasPrefix(line, " ") {
			currentLine++
		}
	}

	return findings, nil
}

// GenerateStudioChangelog generates structured Markdown release notes between two refs
func (GitService) GenerateStudioChangelog(repoPath string, fromRef string, toRef string) (*StudioChangelogResult, error) {
	if toRef == "" {
		toRef = "HEAD"
	}

	if fromRef == "" {
		prevTag, err := ExecGit(repoPath, "describe", "--tags", "--abbrev=0", toRef+"^")
		if err == nil && strings.TrimSpace(prevTag) != "" {
			fromRef = strings.TrimSpace(prevTag)
		} else {
			fromRef = toRef + "~20"
			if _, errRev := ExecGit(repoPath, "rev-parse", fromRef); errRev != nil {
				rootCommit, rootErr := ExecGit(repoPath, "rev-list", "--max-parents=0", "HEAD")
				if rootErr != nil {
					return nil, fmt.Errorf("无法确定变更日志起始提交: %v", rootErr)
				}
				fromRef = strings.TrimSpace(rootCommit)
				if fromRef == "" {
					return nil, errors.New("无法确定变更日志起始提交")
				}
			}
		}
	}

	revRange := fromRef + ".." + toRef
	args := []string{"log", "--format=%H%x00%h%x00%an%x00%ae%x00%ad%x00%cr%x00%s", revRange}
	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("无法获取提交范围 %s 的历史: %w", revRange, err)
	}

	var allCommits []CommitItem
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		pts := strings.Split(line, "\x00")
		if len(pts) >= 7 {
			allCommits = append(allCommits, CommitItem{
				Hash:         pts[0],
				ShortHash:    pts[1],
				AuthorName:   pts[2],
				AuthorEmail:  pts[3],
				Date:         pts[4],
				RelativeDate: pts[5],
				Subject:      pts[6],
			})
		}
	}

	catFeat := StudioChangelogCategory{Category: "游戏玩法与系统逻辑 (Features)", Icon: "🎮", Commits: []CommitItem{}}
	catArt := StudioChangelogCategory{Category: "美术资产与贴图材质 (Art Assets)", Icon: "🎨", Commits: []CommitItem{}}
	catLevel := StudioChangelogCategory{Category: "关卡场景与配置布设 (Level & Scene)", Icon: "🗺️", Commits: []CommitItem{}}
	catAudio := StudioChangelogCategory{Category: "音频音效与音画联动 (Audio & SFX)", Icon: "🔊", Commits: []CommitItem{}}
	catPerf := StudioChangelogCategory{Category: "性能优化与运行时调优 (Performance)", Icon: "⚡", Commits: []CommitItem{}}
	catFix := StudioChangelogCategory{Category: "缺陷修复 (Bugfixes)", Icon: "🐛", Commits: []CommitItem{}}
	catEngine := StudioChangelogCategory{Category: "引擎底层与管线构建 (Engine & Build)", Icon: "⚙️", Commits: []CommitItem{}}
	catMisc := StudioChangelogCategory{Category: "日常维护与其他改动 (Chore & Misc)", Icon: "📝", Commits: []CommitItem{}}

	for _, c := range allCommits {
		sub := strings.ToLower(c.Subject)
		switch {
		case strings.HasPrefix(sub, "feat") || strings.Contains(sub, "[feat"):
			catFeat.Commits = append(catFeat.Commits, c)
		case strings.HasPrefix(sub, "art") || strings.Contains(sub, "[art") || strings.Contains(c.Subject, "🎨"):
			catArt.Commits = append(catArt.Commits, c)
		case strings.HasPrefix(sub, "level") || strings.Contains(sub, "[level") || strings.Contains(c.Subject, "🗺️"):
			catLevel.Commits = append(catLevel.Commits, c)
		case strings.HasPrefix(sub, "audio") || strings.Contains(sub, "[audio") || strings.Contains(sub, "sound"):
			catAudio.Commits = append(catAudio.Commits, c)
		case strings.HasPrefix(sub, "perf") || strings.Contains(sub, "[perf") || strings.Contains(sub, "opt"):
			catPerf.Commits = append(catPerf.Commits, c)
		case strings.HasPrefix(sub, "fix") || strings.Contains(sub, "[fix") || strings.Contains(sub, "bug"):
			catFix.Commits = append(catFix.Commits, c)
		case strings.HasPrefix(sub, "engine") || strings.Contains(sub, "[engine") || strings.HasPrefix(sub, "build") || strings.HasPrefix(sub, "ci"):
			catEngine.Commits = append(catEngine.Commits, c)
		default:
			catMisc.Commits = append(catMisc.Commits, c)
		}
	}

	var activeCategories []StudioChangelogCategory
	for _, cat := range []StudioChangelogCategory{catFeat, catArt, catLevel, catAudio, catPerf, catFix, catEngine, catMisc} {
		if len(cat.Commits) > 0 {
			activeCategories = append(activeCategories, cat)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 游戏研发发版变更与里程碑日志 (%s → %s)\n\n", fromRef, toRef))
	sb.WriteString(fmt.Sprintf("> 仓库路径: `%s` | 生成时间: %s | 包含提交总数: %d\n\n", repoPath, time.Now().Format("2006-01-02 15:04:05"), len(allCommits)))

	for _, cat := range activeCategories {
		sb.WriteString(fmt.Sprintf("### %s %s (%d)\n\n", cat.Icon, cat.Category, len(cat.Commits)))
		for _, c := range cat.Commits {
			sb.WriteString(fmt.Sprintf("- [`%s`] %s — *@%s*\n", c.ShortHash, c.Subject, c.AuthorName))
		}
		sb.WriteString("\n")
	}

	return &StudioChangelogResult{
		FromRef:     fromRef,
		ToRef:       toRef,
		TotalCount:  len(allCommits),
		Categories:  activeCategories,
		MarkdownDoc: sb.String(),
	}, nil
}

// GetCustomActions returns configured or studio default custom pipeline scripts
func (GitService) GetCustomActions(repoPath string) ([]CustomActionConfig, error) {
	configPath := filepath.Join(gitCommonDir(repoPath), "wgit-actions.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var list []CustomActionConfig
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			return list, nil
		}
	}
	// Fallback to legacy ugit-actions.json
	legacyPath := filepath.Join(gitCommonDir(repoPath), "ugit-actions.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		var list []CustomActionConfig
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			return list, nil
		}
	}
	return defaultCustomActions(), nil
}

// SaveCustomActions writes custom actions list to .git/wgit-actions.json
func (GitService) SaveCustomActions(repoPath string, actions []CustomActionConfig) error {
	configPath := filepath.Join(gitCommonDir(repoPath), "wgit-actions.json")
	data, err := json.MarshalIndent(actions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// GetStudioCIPipelines retrieves studio CI/CD build runs and stage progress for the current branch
func (GitService) GetStudioCIPipelines(repoPath string) (*StudioCIResult, error) {
	// Same best-effort branch probe as the web-URL panels: on an unborn or
	// unreadable HEAD the rev-parse exits non-zero but git still echoes the
	// literal "HEAD" on stdout, which ExecGit preserves next to the error, so
	// the panel carries that label instead of failing. When HEAD cannot be
	// resolved, the provider/slug detection in the same pass degrades too
	// (no readable remote → the friendly "未检测到 CI 配置" message), and if a
	// readable GitHub remote somehow coexisted with a broken HEAD, the gh
	// query itself fails against ref "HEAD" and surfaces via res.Message —
	// another branch's runs are never presented as current.
	currentBranch, _ := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	currentBranch = strings.TrimSpace(currentBranch)
	if currentBranch == "" {
		currentBranch = "main"
	}

	provider, slug := studioCIDescriptor(repoPath)
	res := &StudioCIResult{
		Provider:     provider,
		ActiveBranch: currentBranch,
		HistoryRuns:  []CIPipelineRun{},
	}

	if provider != "GitHub Actions 流水线" || slug == "" {
		switch {
		case provider == "GitHub Actions 流水线":
			res.Message = "检测到 GitHub Actions 工作流配置，但当前仓库没有可读取的 GitHub 远程仓库。"
		case provider == "未检测到 CI 配置":
			res.Message = "未检测到 CI 配置；当前仅支持读取 GitHub Actions 的运行状态。"
		default:
			res.Message = fmt.Sprintf("已检测到 %s 配置；当前仅支持读取 GitHub Actions 的运行状态。", provider)
		}
		return res, nil
	}

	listArgs := []string{
		"run", "list", "-R", slug, "--branch", currentBranch, "--limit", "10",
		"--json", "databaseId,name,displayTitle,headBranch,headSha,status,conclusion,createdAt,updatedAt,workflowName,number",
	}
	raw, err := runGhCommand(repoPath, listArgs...)
	if err != nil {
		res.Message = fmt.Sprintf("读取 GitHub Actions 运行记录失败：%s", err.Error())
		return res, nil
	}

	var runs []ghRunSummary
	if err := json.Unmarshal([]byte(raw), &runs); err != nil {
		res.Message = fmt.Sprintf("解析 GitHub Actions 运行记录失败：%v", err)
		return res, nil
	}
	if len(runs) == 0 {
		res.Message = "GitHub Actions 中暂无可读取的运行记录。"
		return res, nil
	}

	now := time.Now()
	authors := gitHubRunAuthors(repoPath, runs)
	for idx, run := range runs {
		item := gitHubRunToPipeline(run, now, authors[idx])
		item.Branch = run.HeadBranch
		if item.Branch == "" {
			item.Branch = currentBranch
		}
		if idx == 0 {
			item.Stages = gitHubRunStages(repoPath, slug, run)
		}
		res.HistoryRuns = append(res.HistoryRuns, item)
	}
	res.LatestRun = &res.HistoryRuns[0]
	res.Message = fmt.Sprintf("GitHub Actions 最近 %d 次运行，最新状态：%s。", len(runs), res.LatestRun.Status)
	return res, nil
}

type ghRunSummary struct {
	DatabaseID   int64     `json:"databaseId"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"displayTitle"`
	HeadBranch   string    `json:"headBranch"`
	HeadSha      string    `json:"headSha"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	WorkflowName string    `json:"workflowName"`
	Number       int       `json:"number"`
}

type ghJobSummary struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
}

func studioCIDescriptor(repoPath string) (string, string) {
	remote := defaultPushRemote(repoPath)
	if remote != "" {
		if raw, err := ExecGit(repoPath, "remote", "get-url", remote); err == nil {
			info := ParseRemoteWebURL(raw)
			if info.Provider == "github" {
				slug := strings.Trim(strings.TrimPrefix(info.WebBaseURL, "https://github.com/"), "/")
				return "GitHub Actions 流水线", slug
			}
		}
	}

	switch {
	case dirExists(filepath.Join(repoPath, ".github", "workflows")):
		return "GitHub Actions 流水线", ""
	case fileExists(filepath.Join(repoPath, ".gitlab-ci.yml")):
		return "GitLab CI", ""
	case fileExists(filepath.Join(repoPath, ".coding-ci.yml")):
		return "腾讯工蜂 CODING CI", ""
	case fileExists(filepath.Join(repoPath, "Jenkinsfile")):
		return "Jenkins 自动化构建", ""
	default:
		return "未检测到 CI 配置", ""
	}
}

func dirExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func runGhCommand(repoPath string, args ...string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("未安装 GitHub CLI (gh)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gh 命令执行超时")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if idx := strings.IndexByte(detail, '\n'); idx >= 0 {
			detail = detail[:idx]
		}
		return strings.TrimSpace(stdout.String()), fmt.Errorf("%s", detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// gitHubRunAuthors resolves the commit author for every run's HeadSha.
// All distinct SHAs are resolved with a single `git show -s` invocation
// (records come back in argument order) instead of one `git log -1` process
// per run. If the batch probe fails — including when a single SHA cannot be
// resolved, because git then exits non-zero as a whole — the lookup degrades
// to one probe per remaining SHA, so every resolvable run keeps its author
// exactly as the previous serial per-run behavior did.
func gitHubRunAuthors(repoPath string, runs []ghRunSummary) []string {
	authors := make([]string, len(runs))
	shas := make([]string, 0, len(runs))
	shaIndexes := make(map[string][]int, len(runs))
	for i, run := range runs {
		sha := strings.TrimSpace(run.HeadSha)
		if sha == "" {
			continue
		}
		if _, dup := shaIndexes[sha]; !dup {
			shas = append(shas, sha)
		}
		shaIndexes[sha] = append(shaIndexes[sha], i)
	}
	if len(shas) == 0 {
		return authors
	}

	resolved := make(map[string]string, len(shas))
	args := append([]string{"show", "-s", "--format=%H%x00%an"}, shas...)
	if out, err := ExecGit(repoPath, args...); err == nil {
		lines := strings.Split(out, "\n")
		for i, sha := range shas {
			if i >= len(lines) {
				break
			}
			parts := strings.SplitN(lines[i], "\x00", 2)
			if len(parts) == 2 {
				resolved[sha] = strings.TrimSpace(parts[1])
			}
		}
	}
	if len(resolved) < len(shas) {
		for _, sha := range shas {
			if _, ok := resolved[sha]; ok {
				continue
			}
			if out, logErr := ExecGit(repoPath, "log", "-1", "--format=%an", sha); logErr == nil {
				resolved[sha] = strings.TrimSpace(out)
			}
		}
	}
	for sha, indexes := range shaIndexes {
		for _, i := range indexes {
			authors[i] = resolved[sha]
		}
	}
	return authors
}

func gitHubRunToPipeline(run ghRunSummary, now time.Time, author string) CIPipelineRun {
	name := run.WorkflowName
	if name == "" {
		name = run.Name
	}
	message := run.DisplayTitle
	if message == "" {
		message = run.Name
	}
	shortHash := run.HeadSha
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	status := gitHubStatus(run.Status, run.Conclusion)
	return CIPipelineRun{
		ID:          fmt.Sprintf("%s #%d", name, run.Number),
		Name:        name,
		CommitHash:  run.HeadSha,
		ShortHash:   shortHash,
		Author:      author,
		Message:     message,
		Status:      status,
		TriggerTime: gitHubRelativeTime(run.CreatedAt, now),
		Duration:    gitHubDuration(run.CreatedAt, run.UpdatedAt, status == "RUNNING" || status == "PENDING"),
		Stages:      []CIPipelineStage{},
	}
}

func gitHubRunStages(repoPath string, slug string, run ghRunSummary) []CIPipelineStage {
	stages := []CIPipelineStage{}
	raw, err := runGhCommand(repoPath, "run", "view", strconv.FormatInt(run.DatabaseID, 10), "-R", slug, "--json", "jobs")
	if err != nil {
		return stages
	}
	var payload struct {
		Jobs []ghJobSummary `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return stages
	}
	for _, job := range payload.Jobs {
		status := gitHubStatus(job.Status, job.Conclusion)
		stages = append(stages, CIPipelineStage{
			Name:     job.Name,
			Status:   status,
			Duration: gitHubDuration(job.StartedAt, job.CompletedAt, status == "RUNNING" || status == "PENDING"),
		})
	}
	return stages
}

func gitHubStatus(status string, conclusion string) string {
	switch strings.ToLower(conclusion) {
	case "success":
		return "SUCCESS"
	case "failure", "timed_out", "cancelled", "action_required", "startup_failure":
		return "FAILED"
	case "skipped", "neutral":
		return "SKIPPED"
	}
	switch strings.ToLower(status) {
	case "in_progress":
		return "RUNNING"
	case "queued", "waiting", "requested", "pending":
		return "PENDING"
	case "completed":
		return "PENDING"
	default:
		return "PENDING"
	}
}

func gitHubDuration(start, end time.Time, running bool) string {
	if start.IsZero() {
		return ""
	}
	if running || end.IsZero() || end.Before(start) {
		end = time.Now()
	}
	seconds := int(end.Sub(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	switch {
	case seconds < 60:
		return fmt.Sprintf("%d 秒", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%d 分 %02d 秒", seconds/60, seconds%60)
	default:
		return fmt.Sprintf("%d 小时 %02d 分", seconds/3600, (seconds%3600)/60)
	}
}

func gitHubRelativeTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := now.Sub(t)
	switch {
	case diff < time.Minute:
		return "刚刚"
	case diff < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(diff.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(diff.Hours()/24))
	}
}

// TriggerStudioCIPipeline triggers a GitHub Actions workflow for the current branch
func (GitService) TriggerStudioCIPipeline(repoPath string, pipelineName string) (*StudioCIActionResult, error) {
	// Unborn/unreadable HEAD exits non-zero, but git still echoes the literal
	// "HEAD" on stdout (a non-git directory yields an empty branch). Either
	// way the trigger is refused downstream: the empty check below catches the
	// empty case, the provider gate rejects a repo without a readable GitHub
	// remote, and gh itself fails on an unresolvable --ref — the fail-safe
	// direction (no workflow is ever fired against a guessed branch), so keep
	// the read best-effort.
	currentBranch, _ := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	currentBranch = strings.TrimSpace(currentBranch)
	if currentBranch == "" {
		return nil, fmt.Errorf("当前仓库没有可触发的分支")
	}

	provider, slug := studioCIDescriptor(repoPath)
	if provider != "GitHub Actions 流水线" || slug == "" {
		return nil, fmt.Errorf("当前仓库未配置可触发的 GitHub Actions 远程")
	}
	if strings.TrimSpace(pipelineName) == "" {
		return nil, fmt.Errorf("请先指定要触发的 GitHub Actions 工作流")
	}

	out, err := runGhCommand(repoPath, "workflow", "run", pipelineName, "--ref", currentBranch, "-R", slug)
	if err != nil {
		return nil, fmt.Errorf("触发 GitHub Actions 失败: %s", err.Error())
	}
	message := fmt.Sprintf("已触发 GitHub Actions 工作流 [%s]，目标分支: %s。", pipelineName, currentBranch)
	if out != "" {
		message += " " + out
	}
	return &StudioCIActionResult{Success: true, Message: message}, nil
}
