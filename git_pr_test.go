package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func initPrRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "remote", "add", "origin", remoteURL)
	return dir
}

// clearPrTokenEnv blanks every token source so a developer machine's real
// credentials never leak into assertions.
func clearPrTokenEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"WGIT_PR_TOKEN", "GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "GL_TOKEN"} {
		t.Setenv(name, "")
	}
	t.Setenv("WGIT_PR_PROVIDER", "")
}

func TestPrSummaryLabelsNeverNull(t *testing.T) {
	cases := map[string]PrSummary{
		"github": mapGhSummary(ghPR{}),
		"gitlab": mapGlSummary(glMR{}),
	}
	for name, summary := range cases {
		raw, err := json.Marshal(summary)
		if err != nil {
			t.Fatalf("%s marshal: %v", name, err)
		}
		if strings.Contains(string(raw), `"labels":null`) {
			t.Fatalf("%s labels must serialize as [], got %s", name, raw)
		}
	}
}

func TestParsePrRemoteURL(t *testing.T) {
	cases := []struct {
		name     string
		remote   string
		host     string
		path     string
		provider string
	}{
		{"github https", "https://github.com/acme/demo.git", "github.com", "acme/demo", "github"},
		{"github ssh scp", "git@github.com:acme/demo.git", "github.com", "acme/demo", "github"},
		{"github ssh url", "ssh://git@github.com/acme/demo.git", "github.com", "acme/demo", "github"},
		{"gitlab subgroup", "https://gitlab.com/studio/assets/textures.git", "gitlab.com", "studio/assets/textures", "gitlab"},
		{"gitlab ssh", "git@gitlab.com:studio/assets.git", "gitlab.com", "studio/assets", "gitlab"},
		{"gitee unsupported", "https://gitee.com/acme/demo.git", "gitee.com", "acme/demo", "gitee"},
		{"unknown host", "https://git.acme.internal/tools/wgit.git", "git.acme.internal", "tools/wgit", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearPrTokenEnv(t)
			remote := parsePrRemoteURL(tc.remote)
			if remote.Host != tc.host || remote.Path != tc.path || remote.Provider != tc.provider {
				t.Fatalf("got %+v, want host=%s path=%s provider=%s", remote, tc.host, tc.path, tc.provider)
			}
			if remote.WebBaseURL != "https://"+tc.host+"/"+tc.path {
				t.Fatalf("unexpected web base: %s", remote.WebBaseURL)
			}
		})
	}
}

func TestPrWebURL(t *testing.T) {
	gh := &PrProviderInfo{Provider: "github", WebBaseURL: "https://github.com/acme/demo"}
	if got := prWebURL(gh, 7); got != "https://github.com/acme/demo/pull/7" {
		t.Fatalf("unexpected github PR url: %q", got)
	}
	gl := &PrProviderInfo{Provider: "gitlab", WebBaseURL: "https://gitlab.com/studio/assets"}
	if got := prWebURL(gl, 3); got != "https://gitlab.com/studio/assets/-/merge_requests/3" {
		t.Fatalf("unexpected gitlab MR url: %q", got)
	}
	if got := prWebURL(nil, 1); got != "" {
		t.Fatalf("nil provider must yield an empty url, got %q", got)
	}
	if got := prWebURL(gh, 0); got != "" {
		t.Fatalf("invalid number must yield an empty url, got %q", got)
	}
}

func TestGetPullRequestProviderMasksRemoteCredentials(t *testing.T) {
	clearPrTokenEnv(t)
	dir := initPrRepo(t, "https://ghp_supersecret@github.com/acme/demo.git")

	info, err := (GitService{}).GetPullRequestProvider(dir)
	if err != nil {
		t.Fatalf("GetPullRequestProvider failed: %v", err)
	}
	if strings.Contains(info.RemoteURL, "supersecret") {
		t.Fatalf("remote URL leaked the token: %q", info.RemoteURL)
	}
	if !strings.Contains(info.RemoteURL, "***@") {
		t.Fatalf("expected masked userinfo, got %q", info.RemoteURL)
	}
}

func TestGetPullRequestProviderDetectsTokenSources(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "env-token")
	dir := initPrRepo(t, "git@github.com:acme/demo.git")

	info, err := (GitService{}).GetPullRequestProvider(dir)
	if err != nil {
		t.Fatalf("GetPullRequestProvider failed: %v", err)
	}
	if !info.Supported || info.Provider != "github" {
		t.Fatalf("expected supported github, got %+v", info)
	}
	if info.Owner != "acme" || info.Repo != "demo" {
		t.Fatalf("unexpected owner/repo: %s/%s", info.Owner, info.Repo)
	}
	if !info.HasToken || info.TokenSource != "环境变量 GITHUB_TOKEN" {
		t.Fatalf("unexpected token state: %+v", info)
	}
	if strings.Contains(info.TokenSource, "env-token") {
		t.Fatal("token value must never be echoed back to the frontend")
	}
	if info.ApiBaseURL != "https://api.github.com" {
		t.Fatalf("unexpected api base: %s", info.ApiBaseURL)
	}
}

func TestGetPullRequestProviderReportsUnsupportedAndMissingRemote(t *testing.T) {
	clearPrTokenEnv(t)
	dir := initPrRepo(t, "https://gitee.com/acme/demo.git")
	info, err := (GitService{}).GetPullRequestProvider(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Supported || info.Provider != "gitee" || info.Message == "" {
		t.Fatalf("expected unsupported gitee with a message, got %+v", info)
	}

	empty := t.TempDir()
	runGitTest(t, empty, nil, "init", "-b", "main")
	info, err = (GitService{}).GetPullRequestProvider(empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Supported || info.Message == "" {
		t.Fatalf("expected a no-remote message, got %+v", info)
	}
}

func TestListPullRequestsMapsGithubPayload(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if r.URL.Query().Get("state") != "open" {
			t.Errorf("expected state=open, got %q", r.URL.Query().Get("state"))
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %q", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[
			{"number":7,"title":"feat: water shader","state":"open","draft":false,
			 "user":{"login":"alice"},"head":{"ref":"feat/water","sha":"aaa"},
			 "base":{"ref":"main","sha":"bbb"},"created_at":"2026-10-20T10:00:00Z",
			 "updated_at":"2026-10-21T10:00:00Z","comments":2,
			 "html_url":"https://github.com/acme/demo/pull/7",
			 "labels":[{"name":"asset"}],"mergeable":true,"mergeable_state":"clean"}
		]`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	res, err := (GitService{}).ListPullRequests(dir, "open", 10, 1)
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPath != "/repos/acme/demo/pulls" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	item := res.Items[0]
	if item.Number != 7 || item.Author != "alice" || item.SourceBranch != "feat/water" {
		t.Fatalf("unexpected mapping: %+v", item)
	}
	if len(item.Labels) != 1 || item.Labels[0] != "asset" {
		t.Fatalf("labels not mapped: %+v", item.Labels)
	}
	if !item.Mergeable || item.MergeableState != "clean" {
		t.Fatalf("mergeability not mapped: %+v", item)
	}
	if res.Total != 1 || res.FetchedAt == "" {
		t.Fatalf("unexpected envelope: %+v", res)
	}
}

func TestListPullRequestsFiltersMergedOnGithub(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "closed" {
			t.Errorf("merged filter must query closed, got %q", r.URL.Query().Get("state"))
		}
		io.WriteString(w, `[
			{"number":1,"title":"merged one","state":"closed","merged":true,"user":{"login":"a"}},
			{"number":2,"title":"abandoned","state":"closed","merged":false,"user":{"login":"b"}}
		]`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	res, err := (GitService{}).ListPullRequests(dir, "merged", 10, 1)
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Number != 1 || res.Items[0].State != "merged" {
		t.Fatalf("expected only the merged PR, got %+v", res.Items)
	}
}

func TestGetPullRequestDetailGithub(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls/7":
			io.WriteString(w, `{"number":7,"title":"feat","state":"open","body":"desc","merged":false,
				"additions":12,"deletions":3,"changed_files":2,"commits":3,
				"user":{"login":"alice"},"head":{"ref":"feat/water","sha":"deadbeefcafe"},
				"base":{"ref":"main","sha":"bbb"},"html_url":"https://github.com/acme/demo/pull/7",
				"mergeable":true,"mergeable_state":"clean"}`)
		case "/repos/acme/demo/pulls/7/files":
			io.WriteString(w, `[{"filename":"Assets/Water.mat","status":"modified","additions":10,
				"deletions":2,"changes":12,"patch":"@@ -1 +1 @@\n-old\n+new"}]`)
		case "/repos/acme/demo/pulls/7/comments":
			io.WriteString(w, `[{"id":101,"user":{"login":"bob"},"body":"nit","path":"Assets/Water.mat",
				"line":12,"side":"RIGHT","created_at":"2026-10-21T10:00:00Z"}]`)
		case "/repos/acme/demo/issues/7/comments":
			io.WriteString(w, `[{"id":201,"user":{"login":"carol"},"body":"looks fine"}]`)
		case "/repos/acme/demo/commits/deadbeefcafe/check-runs":
			io.WriteString(w, `{"check_runs":[{"name":"build","status":"completed","conclusion":"success","html_url":"https://ci/1"}]}`)
		case "/repos/acme/demo/commits/deadbeefcafe/status":
			io.WriteString(w, `{"state":"pending","statuses":[{"context":"license","state":"failure","target_url":"https://ci/2","description":"missing"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	detail, err := (GitService{}).GetPullRequestDetail(dir, 7)
	if err != nil {
		t.Fatalf("GetPullRequestDetail failed: %v", err)
	}
	if detail.Body != "desc" || detail.CommitCount != 3 || detail.HeadSHA != "deadbeefcafe" {
		t.Fatalf("unexpected header mapping: %+v", detail)
	}
	if len(detail.Files) != 1 || detail.Files[0].Additions != 10 || detail.Files[0].Binary {
		t.Fatalf("unexpected files: %+v", detail.Files)
	}
	if len(detail.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %+v", detail.Comments)
	}
	if detail.Comments[0].Kind != "review" || detail.Comments[0].Line != 12 || detail.Comments[0].Path != "Assets/Water.mat" {
		t.Fatalf("review comment not mapped: %+v", detail.Comments[0])
	}
	if detail.Comments[1].Kind != "issue" || detail.Comments[1].Author != "carol" {
		t.Fatalf("issue comment not mapped: %+v", detail.Comments[1])
	}
	if len(detail.Checks) != 2 || detail.Summary.ChecksState != "failure" {
		t.Fatalf("checks not aggregated: %+v state=%s", detail.Checks, detail.Summary.ChecksState)
	}
}

func TestGetPullRequestDetailReportsCheckFailures(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls/7":
			io.WriteString(w, `{"number":7,"title":"feat","state":"open",
				"head":{"ref":"feat/water","sha":"deadbeefcafe"},"base":{"ref":"main","sha":"bbb"}}`)
		case "/repos/acme/demo/pulls/7/files":
			io.WriteString(w, `[]`)
		case "/repos/acme/demo/pulls/7/comments", "/repos/acme/demo/issues/7/comments":
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"message":"comments unavailable"}`)
		case "/repos/acme/demo/commits/deadbeefcafe/check-runs", "/repos/acme/demo/commits/deadbeefcafe/status":
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, `{"message":"ci unavailable"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	detail, err := (GitService{}).GetPullRequestDetail(dir, 7)
	if err != nil {
		t.Fatalf("CI lookup failure must not discard the whole detail: %v", err)
	}
	if !strings.Contains(detail.Message, "检查运行读取失败") || !strings.Contains(detail.Message, "提交状态读取失败") {
		t.Fatalf("check failures must be reported, got %q", detail.Message)
	}
	if !strings.Contains(detail.Message, "行内评论读取失败") || !strings.Contains(detail.Message, "讨论读取失败") {
		t.Fatalf("comment failures must be reported, got %q", detail.Message)
	}
}

func TestGetPullRequestDetailWarnsWhenCommentsAreCapped(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls/7":
			io.WriteString(w, `{"number":7,"title":"feat","state":"open",
				"head":{"ref":"feat","sha":"deadbeefcafe"},"base":{"ref":"main","sha":"bbb"}}`)
		case "/repos/acme/demo/pulls/7/files", "/repos/acme/demo/issues/7/comments":
			io.WriteString(w, `[]`)
		case "/repos/acme/demo/pulls/7/comments":
			var b strings.Builder
			b.WriteString("[")
			for i := 0; i < 100; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(`{"id":1,"user":{"login":"u"},"body":"b","path":"a","line":1,"side":"RIGHT"}`)
			}
			b.WriteString("]")
			io.WriteString(w, b.String())
		case "/repos/acme/demo/commits/deadbeefcafe/check-runs":
			io.WriteString(w, `{"check_runs":[]}`)
		case "/repos/acme/demo/commits/deadbeefcafe/status":
			io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	detail, err := (GitService{}).GetPullRequestDetail(dir, 7)
	if err != nil {
		t.Fatalf("GetPullRequestDetail failed: %v", err)
	}
	if !strings.Contains(detail.Message, "仅显示前 100 条") {
		t.Fatalf("a full comment page must be reported as truncated, got %q", detail.Message)
	}
}

func TestGetPullRequestDetailMapsGithubErrors(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "expired")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"Bad credentials"}`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	_, err := (GitService{}).GetPullRequestDetail(dir, 7)
	if err == nil || !strings.Contains(err.Error(), "令牌无效") {
		t.Fatalf("expected a readable token error, got %v", err)
	}
}

func TestCreatePullRequestCommentInlineAndGeneral(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	var commentBody map[string]any
	var generalBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls/7":
			io.WriteString(w, `{"number":7,"head":{"sha":"deadbeefcafe"},"html_url":"https://github.com/acme/demo/pull/7"}`)
		case "/repos/acme/demo/pulls/7/comments":
			json.NewDecoder(r.Body).Decode(&commentBody)
			io.WriteString(w, `{"id":1}`)
		case "/repos/acme/demo/issues/7/comments":
			json.NewDecoder(r.Body).Decode(&generalBody)
			io.WriteString(w, `{"id":2}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	svc := GitService{}

	if _, err := svc.CreatePullRequestComment(dir, 7, "行内意见", "Assets/Water.mat", 12, "new"); err != nil {
		t.Fatalf("inline comment failed: %v", err)
	}
	if commentBody["commit_id"] != "deadbeefcafe" || commentBody["path"] != "Assets/Water.mat" {
		t.Fatalf("inline payload missing anchors: %+v", commentBody)
	}
	if commentBody["side"] != "right" || commentBody["line"].(float64) != 12 {
		t.Fatalf("inline payload side/line wrong: %+v", commentBody)
	}

	if result, err := svc.CreatePullRequestComment(dir, 7, "总体意见", "", 0, ""); err != nil {
		t.Fatalf("general comment failed: %v", err)
	} else if result.WebURL != "https://github.com/acme/demo/pull/7" {
		t.Fatalf("general comment should point at the PR, got %q", result.WebURL)
	}
	if generalBody["body"] != "总体意见" {
		t.Fatalf("general payload wrong: %+v", generalBody)
	}

	if _, err := svc.CreatePullRequestComment(dir, 7, "   ", "", 0, ""); err == nil {
		t.Fatal("expected an error for an empty comment")
	}
	if _, err := svc.CreatePullRequestComment(dir, 7, "x", "Assets/Water.mat", 0, "new"); err == nil {
		t.Fatal("file path without a line must not silently become a general comment")
	}
	if _, err := svc.CreatePullRequestComment(dir, 7, "x", "", 5, "new"); err == nil {
		t.Fatal("a line without a file path must be rejected")
	}
	if _, err := svc.CreatePullRequestComment(dir, 7, "x", "Assets/Water.mat", -1, "new"); err == nil {
		t.Fatal("negative line numbers must be rejected")
	}
}

func TestSubmitPullRequestReviewAndMerge(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	var reviewEvent string
	var mergeMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls/7/reviews":
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			reviewEvent, _ = payload["event"].(string)
			io.WriteString(w, `{"id":9}`)
		case "/repos/acme/demo/pulls/7/merge":
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			mergeMethod, _ = payload["merge_method"].(string)
			io.WriteString(w, `{"merged":true,"sha":"0123456789abcdef","message":"ok"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	svc := GitService{}

	result, err := svc.SubmitPullRequestReview(dir, 7, "approve", "")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if reviewEvent != "APPROVE" || !result.Success {
		t.Fatalf("unexpected review mapping: event=%s result=%+v", reviewEvent, result)
	}
	if _, err := svc.SubmitPullRequestReview(dir, 7, "request_changes", ""); err == nil {
		t.Fatal("request_changes without a body must be rejected locally")
	}
	if _, err := svc.SubmitPullRequestReview(dir, 7, "nonsense", "x"); err == nil {
		t.Fatal("unknown review events must be rejected")
	}

	merged, err := svc.MergePullRequest(dir, 7, "squash")
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if mergeMethod != "squash" || !merged.Success || merged.NewState != "merged" {
		t.Fatalf("unexpected merge result: method=%s result=%+v", mergeMethod, merged)
	}
	if !strings.Contains(merged.Message, "01234567") {
		t.Fatalf("merge message should carry the merge commit: %s", merged.Message)
	}
	if merged.WebURL != "https://github.com/acme/demo/pull/7" {
		t.Fatalf("merge result should point at the PR, got %q", merged.WebURL)
	}
}

func TestMergePullRequestReportsBlockedMerge(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"merged":false,"message":"Pull Request is not mergeable"}`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	result, err := (GitService{}).MergePullRequest(dir, 7, "merge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success || result.NewState != "open" || !strings.Contains(result.Message, "not mergeable") {
		t.Fatalf("blocked merge should surface the platform reason, got %+v", result)
	}
}

func TestGitlabListAndDetail(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "gl-token")
	var gotPrivateToken, gotProject string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrivateToken = r.Header.Get("PRIVATE-TOKEN")
		// GitLab project paths are percent-encoded as a single segment; the
		// server-side URL.Path is decoded, so compare the escaped form.
		switch r.URL.EscapedPath() {
		case "/projects/studio%2Fassets%2Ftextures/merge_requests":
			gotProject = r.URL.EscapedPath()
			if r.URL.Query().Get("state") != "opened" {
				t.Errorf("expected opened state, got %q", r.URL.Query().Get("state"))
			}
			io.WriteString(w, `[{"iid":3,"title":"Draft: reimport","state":"opened","draft":true,
				"author":{"username":"dave"},"source_branch":"feat","target_branch":"main",
				"user_notes_count":1,"web_url":"https://gitlab.com/studio/assets/textures/-/merge_requests/3",
				"labels":["asset"],"has_conflicts":false,"merge_status":"can_be_merged","changes_count":"4"}]`)
		case "/projects/studio%2Fassets%2Ftextures/merge_requests/3":
			io.WriteString(w, `{"iid":3,"title":"Draft: reimport","state":"opened","description":"why",
				"author":{"username":"dave"},"source_branch":"feat","target_branch":"main",
				"web_url":"https://gitlab.com/x/-/merge_requests/3","changes_count":"1",
				"diff_refs":{"base_sha":"base","head_sha":"head","start_sha":"start"}}`)
		case "/projects/studio%2Fassets%2Ftextures/merge_requests/3/changes":
			if r.URL.Query().Get("per_page") != "100" {
				t.Errorf("expected per_page=100 for changes, got %q", r.URL.Query().Get("per_page"))
			}
			io.WriteString(w, `{"changes":[{"old_path":"a.png","new_path":"a.png",
				"diff":"@@ -1 +1 @@\n-x\n+y","new_file":false,"renamed_file":false,"deleted_file":false}],
				"overflow":true}`)
		case "/projects/studio%2Fassets%2Ftextures/merge_requests/3/notes":
			io.WriteString(w, `[{"id":1,"body":"系统消息","system":true,"author":{"username":"gitlab"}},
				{"id":2,"body":"这里要改","system":false,"author":{"username":"eve"},
				 "position":{"new_path":"a.png","old_path":"a.png","new_line":5}}]`)
		case "/projects/studio%2Fassets%2Ftextures/merge_requests/3/pipelines":
			io.WriteString(w, `[{"id":9,"status":"failed","web_url":"https://ci/9","ref":"feat"}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets/textures.git")
	svc := GitService{}

	list, err := svc.ListPullRequests(dir, "open", 20, 1)
	if err != nil {
		t.Fatalf("gitlab list failed: %v", err)
	}
	if gotPrivateToken != "gl-token" {
		t.Fatalf("expected PRIVATE-TOKEN auth, got %q", gotPrivateToken)
	}
	if !strings.Contains(gotProject, "projects/") {
		t.Fatalf("project path not encoded: %s", gotProject)
	}
	if len(list.Items) != 1 || list.Items[0].Number != 3 || !list.Items[0].IsDraft {
		t.Fatalf("unexpected gitlab list: %+v", list.Items)
	}
	if list.Items[0].State != "open" || list.Items[0].ChangedFiles != 4 {
		t.Fatalf("gitlab state/changes mapping wrong: %+v", list.Items[0])
	}

	detail, err := svc.GetPullRequestDetail(dir, 3)
	if err != nil {
		t.Fatalf("gitlab detail failed: %v", err)
	}
	if detail.Body != "why" || detail.HeadSHA != "head" {
		t.Fatalf("unexpected gitlab detail header: %+v", detail)
	}
	if len(detail.Files) != 1 || detail.Files[0].Additions != 1 || detail.Files[0].Deletions != 1 {
		t.Fatalf("gitlab diff not mapped: %+v", detail.Files)
	}
	if !detail.FilesTruncated {
		t.Fatal("GitLab overflow flag must mark the file list as truncated")
	}
	if len(detail.Comments) != 1 || detail.Comments[0].Kind != "review" || detail.Comments[0].Line != 5 {
		t.Fatalf("gitlab notes should drop system entries: %+v", detail.Comments)
	}
	if len(detail.Checks) != 1 || detail.Summary.ChecksState != "failure" {
		t.Fatalf("gitlab pipelines not mapped: %+v", detail.Checks)
	}
}

func TestGitlabDetailWarnsWhenPipelinesAreCapped(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "gl-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/projects/studio%2Fassets/merge_requests/3":
			io.WriteString(w, `{"iid":3,"title":"MR","state":"opened","changes_count":"1",
				"diff_refs":{"base_sha":"base","head_sha":"head","start_sha":"start"}}`)
		case "/projects/studio%2Fassets/merge_requests/3/changes":
			io.WriteString(w, `{"changes":[]}`)
		case "/projects/studio%2Fassets/merge_requests/3/notes":
			io.WriteString(w, `[]`)
		case "/projects/studio%2Fassets/merge_requests/3/pipelines":
			var b strings.Builder
			b.WriteString("[")
			for i := 0; i < 20; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				b.WriteString(`{"id":1,"status":"success"}`)
			}
			b.WriteString("]")
			io.WriteString(w, b.String())
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	detail, err := (GitService{}).GetPullRequestDetail(dir, 3)
	if err != nil {
		t.Fatalf("GetPullRequestDetail failed: %v", err)
	}
	if !strings.Contains(detail.Message, "流水线超过单页上限") {
		t.Fatalf("capped pipelines must be reported, got %q", detail.Message)
	}
}

func TestGitlabInlineCommentUsesRenamedOldPath(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "gl-token")
	var position map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.EscapedPath() == "/projects/studio%2Fassets/merge_requests/3":
			io.WriteString(w, `{"iid":3,"web_url":"https://gitlab.com/studio/assets/-/merge_requests/3",
				"diff_refs":{"base_sha":"base","head_sha":"head","start_sha":"start"}}`)
		case r.Method == "GET" && r.URL.EscapedPath() == "/projects/studio%2Fassets/merge_requests/3/changes":
			io.WriteString(w, `{"changes":[{"old_path":"Assets/Old.mat","new_path":"Assets/New.mat"}]}`)
		case r.Method == "POST" && r.URL.EscapedPath() == "/projects/studio%2Fassets/merge_requests/3/discussions":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			position, _ = payload["position"].(map[string]any)
			io.WriteString(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	if _, err := (GitService{}).CreatePullRequestComment(dir, 3, "rename ok", "Assets/New.mat", 5, "new"); err != nil {
		t.Fatalf("CreatePullRequestComment failed: %v", err)
	}
	if position["old_path"] != "Assets/Old.mat" || position["new_path"] != "Assets/New.mat" {
		t.Fatalf("renamed file must use old/new paths, got %+v", position)
	}
}

func TestGitlabMergeRejectsRebase(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "gl-token")
	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	_, err := (GitService{}).MergePullRequest(dir, 3, "rebase")
	if err == nil || !strings.Contains(err.Error(), "rebase") {
		t.Fatalf("expected a rebase rejection, got %v", err)
	}
}

func TestDescribePrErrorRateLimit(t *testing.T) {
	err := describePrError(&prHTTPError{Status: 429, Body: `{"message":"API rate limit exceeded"}`})
	if !strings.Contains(err.Error(), "速率限制") {
		t.Fatalf("429 must map to the rate-limit message, got %v", err)
	}
	err = describePrError(&prHTTPError{Status: 403, Body: `{"message":"secondary rate limit"}`})
	if !strings.Contains(err.Error(), "速率限制") {
		t.Fatalf("403 secondary rate limit must map to the rate-limit message, got %v", err)
	}
}

func TestPrTruncationKeepsValidUtf8(t *testing.T) {
	snippet := prSnippet(strings.Repeat("中", 300))
	if !utf8.ValidString(snippet) || !strings.HasSuffix(snippet, "…") {
		t.Fatalf("snippet truncation split a rune: %q", snippet)
	}
	cut, truncated := truncatePrPatch(strings.Repeat("中", 200000))
	if !truncated || !utf8.ValidString(cut) {
		t.Fatalf("patch truncation split a rune (truncated=%v)", truncated)
	}
}

func TestGetPullRequestProviderRejectsIncompleteRemote(t *testing.T) {
	clearPrTokenEnv(t)
	for _, remote := range []string{"https://github.com/", "acme/demo"} {
		dir := initPrRepo(t, remote)
		info, err := (GitService{}).GetPullRequestProvider(dir)
		if err != nil {
			t.Fatalf("GetPullRequestProvider(%q) failed: %v", remote, err)
		}
		if info.Supported {
			t.Fatalf("incomplete remote %q must not be reported as supported: %+v", remote, info)
		}
		if !strings.Contains(info.Message, "origin") {
			t.Fatalf("expected an origin-address hint for %q, got %q", remote, info.Message)
		}
	}
}

func TestGetPullRequestProviderReportsVaultProblems(t *testing.T) {
	clearPrTokenEnv(t)
	dir := initPrRepo(t, "git@github.com:acme/demo.git")
	vaultPath := filepath.Join(gitCommonDir(dir), "wgit_vault.json")

	if err := os.WriteFile(vaultPath, []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := (GitService{}).GetPullRequestProvider(dir)
	if err != nil {
		t.Fatalf("GetPullRequestProvider failed: %v", err)
	}
	if !strings.Contains(info.Message, "损坏") {
		t.Fatalf("a corrupt vault must be reported, got %q", info.Message)
	}

	bogus := `[{"secretType":"pr-token","identifier":"github.com","ciphertext":"AAAA","nonce":"AAAA"}]`
	if err := os.WriteFile(vaultPath, []byte(bogus), 0600); err != nil {
		t.Fatal(err)
	}
	info, err = (GitService{}).GetPullRequestProvider(dir)
	if err != nil {
		t.Fatalf("GetPullRequestProvider failed: %v", err)
	}
	if !strings.Contains(info.Message, "无法解密") {
		t.Fatalf("an undecryptable vault record must be reported, got %q", info.Message)
	}
}

func TestSaveAndClearPullRequestToken(t *testing.T) {
	clearPrTokenEnv(t)
	var userAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		userAuth = r.Header.Get("PRIVATE-TOKEN")
		io.WriteString(w, `{"username":"dave"}`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	svc := GitService{}

	info, err := svc.SavePullRequestToken(dir, "  secret-token  ")
	if err != nil {
		t.Fatalf("SavePullRequestToken failed: %v", err)
	}
	if userAuth != "secret-token" {
		t.Fatalf("token was not used for verification: %q", userAuth)
	}
	if !info.HasToken || info.Account != "dave" {
		t.Fatalf("unexpected provider info after save: %+v", info)
	}
	if info.TokenSource != "本地保险库" {
		t.Fatalf("expected vault as token source, got %q", info.TokenSource)
	}

	vaultFile := filepath.Join(gitCommonDir(dir), "wgit_vault.json")
	raw, err := os.ReadFile(vaultFile)
	if err != nil {
		t.Fatalf("vault file missing: %v", err)
	}
	if strings.Contains(string(raw), "secret-token") {
		t.Fatal("token must be encrypted at rest")
	}
	decoded, ok := readVaultToken(dir, "gitlab.com")
	if !ok || decoded != "secret-token" {
		t.Fatalf("vault round-trip failed: %q ok=%v", decoded, ok)
	}
	if mode, err := os.Stat(vaultFile); err == nil && mode.Mode().Perm() != 0600 {
		t.Fatalf("vault permissions should be 0600, got %o", mode.Mode().Perm())
	}

	cleared, err := svc.ClearPullRequestToken(dir)
	if err != nil {
		t.Fatalf("ClearPullRequestToken failed: %v", err)
	}
	if cleared.HasToken {
		t.Fatalf("token should be gone: %+v", cleared)
	}
	if _, ok := readVaultToken(dir, "gitlab.com"); ok {
		t.Fatal("vault record still readable after clear")
	}

	// Clearing again must not pretend another record was removed.
	again, err := svc.ClearPullRequestToken(dir)
	if err != nil {
		t.Fatalf("second ClearPullRequestToken failed: %v", err)
	}
	if !strings.Contains(again.Message, "没有该主机") {
		t.Fatalf("empty clear should be explicit, got %q", again.Message)
	}
}

func TestSavePullRequestTokenRejectsInvalidToken(t *testing.T) {
	clearPrTokenEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"401 Unauthorized"}`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	_, err := (GitService{}).SavePullRequestToken(dir, "bad")
	if err == nil || !strings.Contains(err.Error(), "令牌校验失败") {
		t.Fatalf("expected verification failure, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(gitCommonDir(dir), "wgit_vault.json")); statErr == nil {
		t.Fatal("an invalid token must not be persisted")
	}
}

func TestPullRequestOpsRequireToken(t *testing.T) {
	clearPrTokenEnv(t)
	// A reserved host keeps the credential helpers from finding a real token
	// on the developer machine, so the "no token" path stays deterministic.
	dir := initPrRepo(t, "https://gitlab.invalid/studio/assets.git")
	if _, err := (GitService{}).ListPullRequests(dir, "open", 10, 1); err != nil {
		t.Fatalf("listing without a token should degrade to a message, got %v", err)
	}
	res, err := (GitService{}).ListPullRequests(dir, "open", 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != 0 || res.Message == "" {
		t.Fatalf("expected an actionable message, got %+v", res)
	}
	if _, err := (GitService{}).GetPullRequestDetail(dir, 1); err == nil {
		t.Fatal("detail without a token must fail")
	}
}

func TestListPullRequestsForwardsPage(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	var gotPage, gotLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPage = r.URL.Query().Get("page")
		gotLimit = r.URL.Query().Get("per_page")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	res, err := (GitService{}).ListPullRequests(dir, "open", 25, 2)
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if gotPage != "2" || gotLimit != "25" {
		t.Fatalf("pagination not forwarded: page=%q per_page=%q", gotPage, gotLimit)
	}
	if res.Total != 0 {
		t.Fatalf("expected an empty page, got %+v", res.Items)
	}
}

// A PR without labels used to marshal as `"labels": null`; the list template
// reads `labels.length`, so the whole PR view crashed right after a token was
// saved and the first real page arrived. Keep the wire shape an array.
func TestListPullRequestsNormalizesMissingLabels(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"number":1,"title":"no labels","state":"open","user":{"login":"alice"},"head":{"ref":"feat"},"base":{"ref":"main"}}]`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "https://github.com/acme/demo.git")
	res, err := (GitService{}).ListPullRequests(dir, "open", 10, 1)
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Labels == nil {
		t.Fatal("labels must be an empty slice, not nil")
	}
	raw, err := json.Marshal(res.Items[0])
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(raw), `"labels":[]`) {
		t.Fatalf("labels must marshal as [], got %s", raw)
	}
}

func TestListPullRequestsNormalizesNullGitlabLabels(t *testing.T) {
	clearPrTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "gl-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"iid":4,"title":"no labels","state":"opened","author":{"username":"dave"},"labels":null}]`)
	}))
	defer server.Close()
	t.Setenv("WGIT_PR_API_BASE", server.URL)

	dir := initPrRepo(t, "git@gitlab.com:studio/assets.git")
	res, err := (GitService{}).ListPullRequests(dir, "open", 10, 1)
	if err != nil {
		t.Fatalf("ListPullRequests failed: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Labels == nil {
		t.Fatalf("gitlab labels must be an empty slice: %+v", res.Items)
	}
}
