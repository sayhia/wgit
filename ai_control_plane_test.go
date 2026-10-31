package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func controlPlaneConfig(t *testing.T, baseURL, model string) {
	t.Helper()
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true,"aiSummary":true,"aiConflictBatch":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	if err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: baseURL, Model: model, TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
}

func TestAIControlPlaneDurableUsageAndBudget(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1000000,"completion_tokens":500000}}`)
	}))
	t.Cleanup(srv.Close)
	controlPlaneConfig(t, srv.URL, "priced-model")

	if err := (AiService{}).SaveAIAdvancedConfig(AIAdvancedConfig{
		TaskModels:       map[string]string{"commit": "commit-model"},
		RepoOverrides:    map[string]AIRepoOverride{},
		InputUSDPerMTok:  1,
		OutputUSDPerMTok: 2,
		MonthlyBudgetUSD: 1.5,
	}); err != nil {
		t.Fatalf("SaveAIAdvancedConfig: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := (AiService{}).completeAPI(context.Background(), "hello"); err != nil {
			t.Fatalf("completeAPI: %v", err)
		}
	}

	logs, err := (AiService{}).GetAICallLogs(10)
	if err != nil {
		t.Fatalf("GetAICallLogs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("durable logs = %d, want 2", len(logs))
	}
	if logs[0].TokensIn != 1_000_000 || logs[0].TokensOut != 500_000 || !logs[0].UsageKnown {
		t.Fatalf("unexpected log usage: %+v", logs[0])
	}
	if logs[0].Model != "priced-model" {
		t.Fatalf("model = %q", logs[0].Model)
	}
	if logs[0].ID <= logs[1].ID {
		t.Fatalf("logs are not newest-first: %+v", logs)
	}
	if diff := logs[0].EstimatedCost - 2; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("estimated cost = %v, want 2", logs[0].EstimatedCost)
	}

	summary, err := (AiService{}).GetAIUsageSummary(1)
	if err != nil {
		t.Fatalf("GetAIUsageSummary: %v", err)
	}
	if summary.Calls != 2 || summary.Success != 2 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.TokensIn != 2_000_000 || summary.TokensOut != 1_000_000 {
		t.Fatalf("summary tokens = %+v", summary)
	}
	if diff := summary.EstimatedCost - 4; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("estimated cost = %v, want 4", summary.EstimatedCost)
	}
	if !summary.OverBudget || summary.BudgetPercent <= 100 {
		t.Fatalf("budget state = over:%v percent:%v", summary.OverBudget, summary.BudgetPercent)
	}
	if len(summary.ByKind) != 1 || summary.ByKind[0].Key != "complete" {
		t.Fatalf("byKind = %+v", summary.ByKind)
	}
	if len(summary.ByModel) != 1 || summary.ByModel[0].Key != "priced-model" {
		t.Fatalf("byModel = %+v", summary.ByModel)
	}

	exportPath, err := (AiService{}).ExportAICallLogs("csv")
	if err != nil {
		t.Fatalf("ExportAICallLogs: %v", err)
	}
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if !strings.Contains(string(data), "priced-model") || !strings.Contains(string(data), "tokens_in") {
		t.Fatalf("unexpected csv: %s", data)
	}

	if err := (AiService{}).ClearAICallLogs(); err != nil {
		t.Fatalf("ClearAICallLogs: %v", err)
	}
	summary, err = (AiService{}).GetAIUsageSummary(1)
	if err != nil {
		t.Fatalf("summary after clear: %v", err)
	}
	if summary.Calls != 0 {
		t.Fatalf("calls after clear = %d", summary.Calls)
	}
}

func TestAITaskModelsAndRepositoryOverrides(t *testing.T) {
	repo := initTestRepo(t)
	other := initTestRepo(t)
	controlPlaneConfig(t, "http://127.0.0.1:9/v1", "base-model")
	includeBody := true
	if err := (AiService{}).SaveAIAdvancedConfig(AIAdvancedConfig{
		TaskModels: map[string]string{"commit": "commit-model", "summary": "global-summary"},
		RepoOverrides: map[string]AIRepoOverride{
			filepath.Clean(repo): {
				Model:              "repo-model",
				CommitLanguage:     "zh",
				CommitIncludeBody:  &includeBody,
				CommitInstructions: "use team scopes",
				TaskModels:         map[string]string{"summary": "repo-summary"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveAIAdvancedConfig: %v", err)
	}
	cfg := LoadConfig().AI

	if got := aiResolvedAIConfig(cfg, repo, "commit").Model; got != "repo-model" {
		t.Fatalf("repo commit model = %q", got)
	}
	if got := aiResolvedAIConfig(cfg, repo, "summary").Model; got != "repo-summary" {
		t.Fatalf("repo summary model = %q", got)
	}
	if got := aiResolvedAIConfig(cfg, repo, "explain").Model; got != "repo-model" {
		t.Fatalf("repo explain model = %q", got)
	}
	if got := aiResolvedAIConfig(cfg, other, "commit").Model; got != "commit-model" {
		t.Fatalf("global commit model = %q", got)
	}
	if got := aiResolvedAIConfig(cfg, other, "summary").Model; got != "global-summary" {
		t.Fatalf("global summary model = %q", got)
	}
	if got := aiResolvedAIConfig(cfg, other, "chat").Model; got != "base-model" {
		t.Fatalf("base model = %q", got)
	}

	opts := commitMessageOptionsFor(repo)
	if opts.Language != "zh" || !opts.IncludeBody || opts.Instructions != "use team scopes" {
		t.Fatalf("repo options = %+v", opts)
	}
	if opts := commitMessageOptionsFor(other); opts.Language != "auto" || opts.IncludeBody {
		t.Fatalf("global options = %+v", opts)
	}

	advanced := (AiService{}).GetAIAdvancedConfig()
	if advanced.TaskModels["commit"] != "commit-model" || advanced.RepoOverrides[filepath.Clean(repo)].Model != "repo-model" {
		t.Fatalf("advanced roundtrip = %+v", advanced)
	}

	// A model-only override must inherit the global language/body instead of
	// silently resetting them.
	if err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "http://127.0.0.1:9/v1", Model: "base-model", TimeoutSeconds: 10,
		CommitLanguage: "zh", CommitIncludeBody: true,
	}); err != nil {
		t.Fatalf("SaveAISettings with preferences: %v", err)
	}
	sparse := AIRepoOverride{Model: "repo-only"}
	if err := (AiService{}).SaveAIAdvancedConfig(AIAdvancedConfig{
		TaskModels:    map[string]string{},
		RepoOverrides: map[string]AIRepoOverride{filepath.Clean(repo): sparse},
	}); err != nil {
		t.Fatalf("SaveAIAdvancedConfig sparse: %v", err)
	}
	sparseCfg := aiResolvedAIConfig(LoadConfig().AI, repo, "commit")
	if sparseCfg.Model != "repo-only" || sparseCfg.CommitLanguage != "zh" || !sparseCfg.CommitIncludeBody {
		t.Fatalf("sparse override did not inherit globals: %+v", sparseCfg)
	}
}

func TestAIMultiOriginKeyInventory(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	originA := "https://a.example.com/v1"
	originB := "https://b.example.com/v1"
	if err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: originA, Model: "model-a", TimeoutSeconds: 10, APIKey: "key-a",
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	if err := (AiService{}).SaveAIKeyForOrigin(originB, "key-b"); err != nil {
		t.Fatalf("SaveAIKeyForOrigin: %v", err)
	}
	keys, err := (AiService{}).ListAIKeys()
	if err != nil {
		t.Fatalf("ListAIKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("keys = %+v", keys)
	}
	current := normalizeAPIOrigin(originA)
	foundCurrent, foundOther := false, false
	for _, entry := range keys {
		if entry.Origin == current {
			foundCurrent = entry.Current && entry.Available && entry.Fingerprint == aiKeyFingerprint("key-a")
		}
		if entry.Origin == normalizeAPIOrigin(originB) {
			foundOther = !entry.Current && entry.Available && entry.Fingerprint == aiKeyFingerprint("key-b")
		}
	}
	if !foundCurrent || !foundOther {
		t.Fatalf("unexpected inventory: %+v", keys)
	}

	if err := (AiService{}).DeleteAIKeyForOrigin(originB); err != nil {
		t.Fatalf("DeleteAIKeyForOrigin: %v", err)
	}
	keys, _ = (AiService{}).ListAIKeys()
	if len(keys) != 1 {
		t.Fatalf("keys after delete = %+v", keys)
	}
	if err := (AiService{}).DeleteAIKeyForOrigin(originA); err != nil {
		t.Fatalf("Delete current: %v", err)
	}
	if cfg := LoadConfig().AI; cfg.APIKey != "" {
		t.Fatalf("fingerprint after deleting current key = %q", cfg.APIKey)
	}
}

func TestAIProviderRetryAndFallback(t *testing.T) {
	t.Run("Retry", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"error":{"message":"temporary"}}`)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"content":"retry-ok"}}]}`)
		}))
		t.Cleanup(srv.Close)
		controlPlaneConfig(t, srv.URL, "retry-model")
		if err := (AiService{}).SaveAIAdvancedConfig(AIAdvancedConfig{
			TaskModels: map[string]string{}, RepoOverrides: map[string]AIRepoOverride{},
			RetryCount: 1,
		}); err != nil {
			t.Fatalf("advanced: %v", err)
		}
		out, err := (AiService{}).completeAPI(context.Background(), "hello")
		if err != nil || out != "retry-ok" {
			t.Fatalf("completeAPI = %q err=%v", out, err)
		}
		if calls.Load() != 2 {
			t.Fatalf("calls = %d, want 2", calls.Load())
		}
	})

	t.Run("FallbackProvider", func(t *testing.T) {
		var primary atomic.Int32
		primarySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			primary.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":{"message":"down"}}`)
		}))
		t.Cleanup(primarySrv.Close)
		fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"fallback-ok"}}]}`)
		}))
		t.Cleanup(fallbackSrv.Close)

		controlPlaneConfig(t, primarySrv.URL, "primary-model")
		if err := (AiService{}).SaveAIAdvancedConfig(AIAdvancedConfig{
			TaskModels:       map[string]string{},
			RepoOverrides:    map[string]AIRepoOverride{},
			RetryCount:       0,
			FallbackBaseURL:  fallbackSrv.URL,
			FallbackModel:    "fallback-model",
			FallbackProtocol: "openai",
		}); err != nil {
			t.Fatalf("advanced: %v", err)
		}
		out, err := (AiService{}).completeAPI(context.Background(), "hello")
		if err != nil || out != "fallback-ok" {
			t.Fatalf("completeAPI = %q err=%v", out, err)
		}
		logs, err := (AiService{}).GetAICallLogs(1)
		if err != nil || len(logs) != 1 {
			t.Fatalf("logs = %+v err=%v", logs, err)
		}
		if logs[0].Model != "fallback-model" || !logs[0].Success {
			t.Fatalf("fallback audit entry = %+v", logs[0])
		}
	})
}

func TestAIStreamReasoningAndToolNotices(t *testing.T) {
	content, reasoning, providerErr := parseAIStreamChunkParts(
		`{"choices":[{"delta":{"reasoning_content":"think","content":"answer"}}]}`, "openai")
	if providerErr != "" || content != "answer" || reasoning != "think" {
		t.Fatalf("openai parts = %q %q %q", content, reasoning, providerErr)
	}
	content, reasoning, _ = parseAIStreamChunkParts(
		`{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm"}}`, "anthropic")
	if content != "" || reasoning != "hmm" {
		t.Fatalf("anthropic parts = %q %q", content, reasoning)
	}
	content, reasoning, _ = parseAIStreamChunkParts(
		`{"candidates":[{"content":{"parts":[{"text":"why","thought":true},{"text":"ok"},{"functionCall":{"name":"shell"}}]}}]}`, "gemini")
	if content != "ok" || !strings.Contains(reasoning, "why") || !strings.Contains(reasoning, "shell") {
		t.Fatalf("gemini parts = %q %q", content, reasoning)
	}
	content, reasoning, _ = parseAIStreamChunkParts(
		`{"choices":[{"delta":{"content":"","tool_calls":[{"function":{"name":"run"}}]}}]}`, "openai")
	if content != "" || !strings.Contains(reasoning, "run") {
		t.Fatalf("tool notice = %q %q", content, reasoning)
	}

	body := strings.NewReader("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"r\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"c\"}}]}\n\ndata: [DONE]\n\n")
	var thoughts []string
	text, _, emitted, err := readAIStreamBodyReasoning(body, "text/event-stream", "openai",
		func(string) error { return nil }, func(delta string) error {
			thoughts = append(thoughts, delta)
			return nil
		})
	if err != nil || text != "c" || !emitted || strings.Join(thoughts, "") != "r" {
		t.Fatalf("stream = %q thoughts=%v emitted=%v err=%v", text, thoughts, emitted, err)
	}
}

func TestAIReadOnlyAssistants(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Demo\nA sample project."), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if _, err := ExecGit(repo, "add", "README.md", "main.go"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := ExecGit(repo, "commit", "-qm", "feat: initial demo"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	hash, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	hash = strings.TrimSpace(hash)

	chat, err := buildChatPrompt(repo, "main.go", "+func main() {}", "diff", "What changed?", []AIChatTurn{
		{Role: "user", Content: "Context?"},
		{Role: "assistant", Content: "A Go entry point."},
	}, false)
	if err != nil {
		t.Fatalf("diff chat prompt: %v", err)
	}
	for _, want := range []string{"What changed?", "main.go", "+func main() {}", "Conversation so far"} {
		if !strings.Contains(chat, want) {
			t.Fatalf("diff chat prompt missing %q:\n%s", want, chat)
		}
	}

	repoChat, err := buildChatPrompt(repo, "", "", "repo", "What is this project?", nil, false)
	if err != nil {
		t.Fatalf("repo chat prompt: %v", err)
	}
	for _, want := range []string{"Tracked files", "README.md", "feat: initial demo", "A sample project."} {
		if !strings.Contains(repoChat, want) {
			t.Fatalf("repo chat prompt missing %q:\n%s", want, repoChat)
		}
	}

	review, err := buildReviewPrompt(repo, hash, "", "")
	if err != nil {
		t.Fatalf("review prompt: %v", err)
	}
	for _, want := range []string{"## Verdict", "## Findings", "feat: initial demo", "package main"} {
		if !strings.Contains(review, want) {
			t.Fatalf("review prompt missing %q", want)
		}
	}

	release, err := buildReleaseNotesPrompt(repo, "", "", 5)
	if err != nil {
		t.Fatalf("release prompt: %v", err)
	}
	if !strings.Contains(release, "feat: initial demo") || !strings.Contains(release, "## Highlights") {
		t.Fatalf("release prompt missing commit/contract:\n%s", release)
	}

	for _, bad := range []string{"--output=/tmp/x", "a b", "a:b", "a..b", "a^b"} {
		if err := validateGitRef(bad); err == nil {
			t.Fatalf("validateGitRef(%q) = nil, want error", bad)
		}
	}
	if err := validateGitRef("HEAD~2"); err != nil {
		t.Fatalf("HEAD~2 rejected: %v", err)
	}
}

func TestAIUsageLogBoundaryStaysClean(t *testing.T) {
	// Sanity check that the durable record never stores prompt text or keys.
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	now := time.Now()
	recordAICallLog(AICallLog{
		ID: now.UnixNano(), At: now.Unix(), Time: now.Format(time.RFC3339),
		Kind: "commit", Operation: "ai.stream.commit", Mode: "api", Model: "m",
		Status: "status=ok", Success: true, TokensIn: 3, TokensOut: 2, UsageKnown: true,
	})
	data, err := os.ReadFile(aiCallLogsPath())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(data), "prompt") || strings.Contains(string(data), "sk-") {
		t.Fatalf("durable log leaked payload: %s", data)
	}
}

func TestAIUsageSummaryByRepo(t *testing.T) {
	controlPlaneConfig(t, "https://api.example.com/v1", "gpt-4.1-mini")
	now := time.Now()
	entries := []AICallLog{
		{At: now.Unix(), Time: now.Format(time.RFC3339), Kind: "commit", Model: "gpt-4.1-mini", RepoPath: "/repo/a", Success: true, TokensIn: 100, TokensOut: 50, EstimatedCost: 0.1, CostKnown: true},
		{At: now.Unix(), Time: now.Format(time.RFC3339), Kind: "summary", Model: "gpt-4.1-mini", RepoPath: "/repo/a", Success: true, TokensIn: 200, TokensOut: 80, EstimatedCost: 0.2, CostKnown: true},
		{At: now.Unix(), Time: now.Format(time.RFC3339), Kind: "chat", Model: "unknown-model", RepoPath: "/repo/b", Success: false, TokensIn: 10, TokensOut: 0, EstimatedCost: 0, CostKnown: false},
		{At: now.Unix(), Time: now.Format(time.RFC3339), Kind: "complete", Model: "gpt-4.1-mini", Success: true},
	}
	for _, entry := range entries {
		if err := appendAICallLog(entry); err != nil {
			t.Fatalf("appendAICallLog: %v", err)
		}
	}

	summary, err := (AiService{}).GetAIUsageSummary(1)
	if err != nil {
		t.Fatalf("GetAIUsageSummary: %v", err)
	}
	if len(summary.ByRepo) != 3 {
		t.Fatalf("byRepo = %+v, want 3 buckets", summary.ByRepo)
	}
	byKey := map[string]AIUsageBucket{}
	for _, bucket := range summary.ByRepo {
		byKey[bucket.Key] = bucket
	}
	a := byKey["/repo/a"]
	if a.Calls != 2 || a.Success != 2 || a.TokensIn != 300 || a.TokensOut != 130 {
		t.Fatalf("repo a bucket = %+v", a)
	}
	if diff := a.EstimatedCost - 0.3; diff > 0.0001 || diff < -0.0001 {
		t.Fatalf("repo a cost = %v", a.EstimatedCost)
	}
	b := byKey["/repo/b"]
	if b.Calls != 1 || b.Failed != 1 || b.CostKnownCalls != 0 {
		t.Fatalf("repo b bucket = %+v", b)
	}
	if global := byKey[""]; global.Calls != 1 || global.Success != 1 {
		t.Fatalf("global bucket = %+v", global)
	}
}
