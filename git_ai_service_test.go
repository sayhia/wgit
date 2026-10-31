package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// Never touch the real login Keychain or the developer's real recent-repo
	// config from tests.
	aiSecrets = &memoryAISecretStore{}
	recordRecentRepo = func(string) {}
	os.Exit(m.Run())
}

const (
	aiTestKey       = "sk-live-secret-key-abcdef1234567890"
	aiTestURLSecret = "supersecrettoken"
	aiTestGHToken   = "ghp_a1B2c3D4e5a1B2c3D4e5a1B2c3D4e5"
)

// resetAISessionKeyForTest wipes the in-process plaintext key before and after
// the test so tests never leak state into each other.
func resetAISessionKeyForTest(t *testing.T) {
	t.Helper()
	clear := func() {
		aiSessionKeyMu.Lock()
		aiSessionKey = ""
		aiSessionKeyMu.Unlock()
		if store, ok := aiSecrets.(*memoryAISecretStore); ok {
			_ = store.Delete()
		}
	}
	clear()
	t.Cleanup(clear)
}

// TestGetAIConfigNeverReturnsPlaintextKey ensures the sanitized view carries
// the fingerprint and presence flag but never the key itself.
func TestGetAIConfigNeverReturnsPlaintextKey(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	svc := AiService{}
	if err := svc.SaveAIConfig("http://localhost:11434/v1/", "llama3", 30, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	got := svc.GetAIConfig()

	serialized, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized config: %v", err)
	}
	if strings.Contains(string(serialized), aiTestKey) {
		t.Fatalf("sanitized config leaked the plaintext key: %s", serialized)
	}
	if !got.HasAPIKey {
		t.Errorf("HasAPIKey = false, want true after saving a key")
	}
	if !got.KeyAvailable {
		t.Errorf("KeyAvailable = false, want true in the saving session")
	}
	if got.KeyFingerprint != aiKeyFingerprint(aiTestKey) {
		t.Errorf("KeyFingerprint = %q, want %q", got.KeyFingerprint, aiKeyFingerprint(aiTestKey))
	}
	if got.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", got.BaseURL)
	}
	if got.Notice != "" {
		t.Errorf("Notice = %q, want empty while the session key is available", got.Notice)
	}

	// The persisted file must hold only the fingerprint, never the key.
	if cfg := LoadConfig().AI; strings.Contains(cfg.APIKey, "live-secret") {
		t.Fatalf("config.json persisted plaintext key material: %q", cfg.APIKey)
	}
}

// TestSaveAIConfigEmptyKeyKeepsExistingKey ensures an empty apiKey means
// "keep the current key": the fingerprint and the in-memory key survive.
func TestSaveAIConfigEmptyKeyKeepsExistingKey(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	svc := AiService{}
	if err := svc.SaveAIConfig("http://localhost:11434/v1", "llama3", 30, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	fingerprint := LoadConfig().AI.APIKey
	if fingerprint == "" || fingerprint == aiTestKey {
		t.Fatalf("persisted APIKey = %q, want the fingerprint only", fingerprint)
	}

	if err := svc.SaveAIConfig("http://localhost:11434/v1", "model-b", 45, ""); err != nil {
		t.Fatalf("SaveAIConfig with empty key failed: %v", err)
	}

	cfg := LoadConfig().AI
	if cfg.APIKey != fingerprint {
		t.Errorf("empty apiKey cleared the persisted fingerprint: %q -> %q", fingerprint, cfg.APIKey)
	}
	if cfg.Model != "model-b" || cfg.TimeoutSeconds != 45 {
		t.Errorf("non-key fields not saved: model=%q timeout=%d", cfg.Model, cfg.TimeoutSeconds)
	}

	got := svc.GetAIConfig()
	if !got.HasAPIKey || !got.KeyAvailable {
		t.Errorf("HasAPIKey/KeyAvailable = %v/%v, want true/true after empty-key save", got.HasAPIKey, got.KeyAvailable)
	}
	if got.KeyFingerprint != fingerprint {
		t.Errorf("KeyFingerprint = %q, want %q", got.KeyFingerprint, fingerprint)
	}
}

// TestCompleteRedactsPromptAndSendsOpenAIFormat verifies the outbound chat
// request shape (POST /chat/completions, Bearer key, max_tokens) and that the
// prompt is scrubbed before leaving the process.
func TestCompleteRedactsPromptAndSendsOpenAIFormat(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	var mu sync.Mutex
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  merged answer  "}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 30, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	prompt := "review https://user:" + aiTestURLSecret + "@host/repo and " + aiTestGHToken + " please"
	out, err := svc.completeAPI(context.Background(), prompt)
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if out != "merged answer" {
		t.Errorf("complete = %q, want trimmed content %q", out, "merged answer")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/chat/completions" {
		t.Errorf("request path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer "+aiTestKey {
		t.Errorf("Authorization = %q, want the session key as Bearer token", gotAuth)
	}

	var sent aiChatRequest
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("outbound body is not the expected JSON shape: %v (%s)", err, gotBody)
	}
	if sent.Model != "test-model" {
		t.Errorf("outbound model = %q, want %q", sent.Model, "test-model")
	}
	if sent.MaxTokens != aiMaxTokens {
		t.Errorf("outbound max_tokens = %d, want %d", sent.MaxTokens, aiMaxTokens)
	}
	if len(sent.Messages) != 1 || sent.Messages[0].Role != "user" {
		t.Fatalf("outbound messages = %+v, want a single user message", sent.Messages)
	}

	content := sent.Messages[0].Content
	if strings.Contains(content, aiTestURLSecret) || strings.Contains(gotBody, aiTestURLSecret) {
		t.Errorf("outbound body leaked the URL credential secret: %s", gotBody)
	}
	if !strings.Contains(content, "https://***@host/repo") {
		t.Errorf("URL userinfo credential was not masked: %q", content)
	}
	if strings.Contains(content, aiTestGHToken) {
		t.Errorf("outbound body leaked the provider token: %q", content)
	}
	if !strings.Contains(content, "ghp_***") {
		t.Errorf("provider token was not masked: %q", content)
	}
}

// TestCompleteTimeout ensures a slow endpoint surfaces a timeout error.
func TestAICompleteTimeout(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"late"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 1, ""); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	out, err := svc.completeAPI(context.Background(), "hello")
	if err == nil {
		t.Fatalf("complete succeeded, want a timeout error")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("error = %v, want a timeout mention", err)
	}
	if out != "" {
		t.Errorf("complete returned content %q alongside an error", out)
	}
}

// TestCompleteNon200Error ensures a server failure becomes a contextual,
// redacted error instead of empty output.
func TestAICompleteNon200Error(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "https://user:"+aiTestURLSecret+"@host exploded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, ""); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	out, err := svc.completeAPI(context.Background(), "hello")
	if err == nil {
		t.Fatalf("complete succeeded, want an HTTP 500 error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error = %v, want an HTTP 500 mention", err)
	}
	if strings.Contains(err.Error(), aiTestURLSecret) {
		t.Errorf("error leaked the URL credential secret: %v", err)
	}
	if out != "" {
		t.Errorf("complete returned content %q alongside an error", out)
	}
}

// TestCompleteRecordsOplogAudit verifies each complete call appends one
// activity-log entry in the recordGitCommand pattern, carrying only the
// operation name, elapsed time and result status — never prompt or key text.
func TestAICompleteRecordsOplogAudit(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)
	ClearRecentActivityLogs()
	defer ClearRecentActivityLogs()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	prompt := "audit check https://user:" + aiTestURLSecret + "@host/repo"
	if _, err := svc.completeAPI(context.Background(), prompt); err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	logs := GetRecentActivityLogs()
	if len(logs) == 0 {
		t.Fatalf("no activity log entries recorded")
	}
	entry := logs[len(logs)-1]
	if entry.Command != "ai.complete" {
		t.Errorf("entry.Command = %q, want ai.complete", entry.Command)
	}
	if !entry.Success {
		t.Errorf("entry.Success = false, want true for a successful call: %+v", entry)
	}
	if entry.Output != "status=ok" {
		t.Errorf("entry.Output = %q, want only the result status", entry.Output)
	}
	joined := entry.Command + "\n" + entry.Output
	for _, leaked := range []string{prompt, aiTestURLSecret, aiTestKey, "audit check"} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("oplog entry leaked %q: %+v", leaked, entry)
		}
	}

	// A failed call must also land in the log, with the failure status.
	ClearRecentActivityLogs()
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer failSrv.Close()
	if err := svc.SaveAIConfig(failSrv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	if _, err := svc.completeAPI(context.Background(), "failing prompt"); err == nil {
		t.Fatalf("complete against a 502 endpoint succeeded, want an error")
	}
	logs = GetRecentActivityLogs()
	if len(logs) == 0 {
		t.Fatalf("failed call recorded no activity log entry")
	}
	failed := logs[len(logs)-1]
	if failed.Command != "ai.complete" || failed.Success {
		t.Errorf("failed entry = %+v, want command ai.complete with Success=false", failed)
	}
	if failed.Output != "status=error" {
		t.Errorf("failed entry.Output = %q, want only the result status", failed.Output)
	}
}

// =========================================================================
// F1: GenerateCommitMessage（AI 提交信息生成）测试
// =========================================================================

// tempGitRepoForAI creates a throwaway real git repository so the repoPath
// guard inside GenerateCommitMessage accepts it.
func tempGitRepoForAI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init"); err != nil {
		t.Fatalf("git init %s: %v", dir, err)
	}
	return dir
}

// TestGenerateCommitMessageFlagDisabled ensures the aiCommitMsg rollout gate
// answers a clear wire-English "feature disabled" error and never spends an
// outbound request while the user switched the flag off.
func TestGenerateCommitMessageFlagDisabled(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"feat: should never happen"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	repo := tempGitRepoForAI(t)
	out, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n")
	if err == nil {
		t.Fatalf("GenerateCommitMessage succeeded with the flag off, want a feature-disabled error")
	}
	if !strings.Contains(err.Error(), "feature disabled") {
		t.Errorf("error = %v, want an English \"feature disabled\" mention", err)
	}
	if out != "" {
		t.Errorf("GenerateCommitMessage returned %q alongside an error", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("flag-off call hit the AI endpoint %d time(s), want 0", calls)
	}
}

// TestGenerateCommitMessageGuards ensures an invalid repoPath and an empty
// staged diff are rejected defensively before any outbound call happens.
func TestGenerateCommitMessageGuards(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"feat: x"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	if _, err := svc.GenerateCommitMessage(t.TempDir(), "some diff"); err == nil {
		t.Fatalf("GenerateCommitMessage on a non-repository path succeeded, want an error")
	} else if !strings.Contains(err.Error(), "Git 仓库") {
		t.Errorf("error = %v, want an invalid-repository mention", err)
	}

	repo := tempGitRepoForAI(t)
	for name, diff := range map[string]string{"empty": "", "whitespace": "  \n\t"} {
		if _, err := svc.GenerateCommitMessage(repo, diff); err == nil {
			t.Fatalf("GenerateCommitMessage with a %s diff succeeded, want an error", name)
		} else if !strings.Contains(err.Error(), "diff 为空") {
			t.Errorf("empty-diff case %q error = %v, want an empty-diff mention", name, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("defensive guard hit the AI endpoint %d time(s), want 0", calls)
	}
}

// TestGenerateCommitMessageOversizedDiffTruncated feeds a staged diff larger
// than the 200 KB cap and verifies the outbound prompt is truncated (the tail
// marker is gone) with an explicit truncation note, while the extracted
// subject still comes back.
func TestGenerateCommitMessageOversizedDiffTruncated(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	const tailMarker = "TAIL_MARKER_SHOULD_BE_TRUNCATED_AWAY"
	var mu sync.Mutex
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"feat(diff): 截断超大差异"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	// Chinese lines keep the truncation on a real multi-byte rune boundary.
	line := "+修改内容：中文行确保 UTF-8 边界安全\n"
	diff := strings.Repeat(line, aiCommitMsgMaxDiffBytes/len(line)+2)
	diff += tailMarker + "\n"
	if len(diff) <= aiCommitMsgMaxDiffBytes {
		t.Fatalf("test diff is only %d bytes, want > %d", len(diff), aiCommitMsgMaxDiffBytes)
	}

	out, err := svc.GenerateCommitMessage(tempGitRepoForAI(t), diff)
	if err != nil {
		t.Fatalf("GenerateCommitMessage failed: %v", err)
	}
	if out != "feat(diff): 截断超大差异" {
		t.Errorf("subject = %q, want the single-line extracted content", out)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotBody, "[diff truncated to first 200 KB]") {
		t.Errorf("outbound prompt lacks the truncation note: %s", gotBody)
	}
	if strings.Contains(gotBody, tailMarker) {
		t.Errorf("outbound prompt still carries the oversized tail past the 200 KB cap")
	}
	if !strings.Contains(gotBody, "conventional-commit") {
		t.Errorf("outbound prompt lacks the fixed commit-message template: %s", gotBody)
	}
}

// TestGenerateCommitMessagePostProcess verifies the result pipeline: code
// fences are stripped, the first non-empty line becomes the subject, an empty
// usable result errors, and outbound prompts stay redacted end to end.
func TestGenerateCommitMessagePostProcess(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var mu sync.Mutex
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fence := "```"
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + fence + `text\n\n  feat(ai): 生成提交信息\n\n` + fence + `"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	// The embedded diff keeps a secret; redactAIPayload inside complete() must
	// scrub it before the prompt leaves the process.
	secretDiff := "diff --git a/auth.go b/auth.go\ntoken = \"" + aiTestGHToken + "\"\n"
	out, err := svc.GenerateCommitMessage(tempGitRepoForAI(t), secretDiff)
	if err != nil {
		t.Fatalf("GenerateCommitMessage failed: %v", err)
	}
	if out != "feat(ai): 生成提交信息" {
		t.Errorf("subject = %q, want the first non-fence non-empty line", out)
	}

	mu.Lock()
	if strings.Contains(gotBody, aiTestGHToken) {
		t.Errorf("outbound prompt leaked the provider token: %s", gotBody)
	}
	mu.Unlock()

	// Post-processing edge cases, exercised directly.
	if _, err := postProcessCommitMessage("```\n```\n"); err == nil {
		t.Errorf("fence-only completion post-processed without error, want an empty-result error")
	}
	if got, err := postProcessCommitMessage("```sh\n  feat: spaced  \n```"); err != nil || got != "feat: spaced" {
		t.Errorf("postProcessCommitMessage = (%q, %v), want (feat: spaced, nil)", got, err)
	}
	if got, err := postProcessCommitMessage("中文主题也可以"); err != nil || got != "中文主题也可以" {
		t.Errorf("postProcessCommitMessage = (%q, %v), want the Chinese subject unchanged", got, err)
	}
}

// TestTruncateUTF8Safe pins the rune-boundary behaviour used by diff
// truncation: multi-byte characters are never split in half.
func TestTruncateUTF8Safe(t *testing.T) {
	const s = "中文中文" // 4 runes × 3 bytes
	if got := truncateUTF8Safe(s, len(s)); got != s {
		t.Errorf("truncateUTF8Safe at the full length = %q, want unchanged", got)
	}
	if got := truncateUTF8Safe(s, 5); got != s[:3] {
		t.Errorf("truncateUTF8Safe(s, 5) = %q, want the first rune only %q", got, s[:3])
	}
	if got := truncateUTF8Safe("abc", 10); got != "abc" {
		t.Errorf("truncateUTF8Safe below the length = %q, want unchanged", got)
	}
}

// TestTestAIConnection verifies the 10s probe: unconfigured endpoint errors,
// a healthy /models endpoint succeeds, and failures carry the HTTP status.
func TestTestAIConnection(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
	resetAISessionKeyForTest(t)

	svc := AiService{}
	if err := svc.TestAIConnection(context.Background()); err == nil {
		t.Fatalf("TestAIConnection with no BaseURL succeeded, want an unconfigured error")
	}

	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
			return
		}
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	if err := svc.SaveAIConfig(srv.URL, "llama3", 0, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	if err := svc.TestAIConnection(context.Background()); err != nil {
		t.Fatalf("TestAIConnection against a healthy endpoint failed: %v", err)
	}
	if gotPath != "/models" {
		t.Errorf("probe path = %q, want /models", gotPath)
	}
	if gotAuth != "Bearer "+aiTestKey {
		t.Errorf("probe Authorization = %q, want the session key as Bearer token", gotAuth)
	}

	if err := svc.SaveAIConfig(srv.URL+"/forbidden", "llama3", 0, ""); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	if err := svc.TestAIConnection(context.Background()); err == nil {
		t.Fatalf("TestAIConnection against a 403 endpoint succeeded, want an error")
	} else if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("error = %v, want an HTTP 403 mention", err)
	}
}

// =========================================================================
// F2: SummarizeCommit（AI 变更摘要）测试
// =========================================================================

// summarizeTestRepo creates a real git repository containing exactly one
// commit, so SummarizeCommit has a truthful context to summarize.
func summarizeTestRepo(t *testing.T) string {
	t.Helper()
	repo := tempGitRepoForAI(t)
	if _, err := ExecGit(repo, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("config user.email: %v", err)
	}
	if _, err := ExecGit(repo, "config", "user.name", "Test"); err != nil {
		t.Fatalf("config user.name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if _, err := ExecGit(repo, "add", "a.txt"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := ExecGit(repo, "commit", "-m", "feat: add a.txt"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return repo
}

// TestSummarizeCommitFlagDisabled mirrors the F1 gate test: a dark aiSummary
// flag answers the wire-English "feature disabled" error and never spends an
// outbound request.
func TestSummarizeCommitFlagDisabled(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"should never happen"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	repo := summarizeTestRepo(t)
	hashOut, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	out, err := svc.SummarizeCommit(repo, strings.TrimSpace(hashOut))
	if err == nil {
		t.Fatalf("SummarizeCommit succeeded with the flag off, want a feature-disabled error")
	}
	if !strings.Contains(err.Error(), "feature disabled") {
		t.Errorf("error = %v, want an English \"feature disabled\" mention", err)
	}
	if out != "" {
		t.Errorf("SummarizeCommit returned %q alongside an error", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("flag-off call hit the AI endpoint %d time(s), want 0", calls)
	}
}

// TestSummarizeCommitProducesThreeLineSummary covers the happy path: the
// commit subject reaches the prompt, the completion's code fences are
// stripped, and the summary is capped at three lines.
func TestSummarizeCommitProducesThreeLineSummary(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiSummary":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var mu sync.Mutex
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"content": "```text\n第一行：新增 a.txt。\n第二行：内容为 hello。\n第三行：为什么。\n第四行：应被截去。\n```"},
		}}}
		data, _ := json.Marshal(payload)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	repo := summarizeTestRepo(t)
	hashOut, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	summary, err := svc.SummarizeCommit(repo, strings.TrimSpace(hashOut))
	if err != nil {
		t.Fatalf("SummarizeCommit failed: %v", err)
	}
	lines := strings.Split(summary, "\n")
	if len(lines) != 3 {
		t.Errorf("summary has %d line(s), want exactly 3: %q", len(lines), summary)
	}
	if strings.Contains(summary, "```") || strings.Contains(summary, "第四行") {
		t.Errorf("summary leaked fences or the truncated line: %q", summary)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotBody, "feat: add a.txt") {
		t.Errorf("outbound prompt missing the commit subject; body=%q", gotBody)
	}
	if !strings.Contains(gotBody, "commit context begin") {
		t.Errorf("outbound prompt missing the context end-marker framing; body=%q", gotBody)
	}
}

// TestSummarizeCommitGuards ensures an empty hash and an unknown hash are
// rejected before/outside any outbound AI call.
func TestSummarizeCommitGuards(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiSummary":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}
	repo := summarizeTestRepo(t)

	if _, err := svc.SummarizeCommit(repo, "  "); err == nil {
		t.Fatalf("empty hash accepted, want an error")
	} else if !strings.Contains(err.Error(), "commit hash 为空") {
		t.Errorf("empty-hash error = %v, want the defensive message", err)
	}
	if _, err := svc.SummarizeCommit(repo, "deadbeef"); err == nil {
		t.Fatalf("unknown hash accepted, want an error")
	} else if !strings.Contains(err.Error(), "读取提交失败") {
		t.Errorf("unknown-hash error = %v, want the wrapped read failure", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("guard-rejected calls hit the AI endpoint %d time(s), want 0", calls)
	}
}

// =========================================================================
// F5 第一步：GenerateConflictAdvice（冲突建议后端化）测试
// =========================================================================

// TestGenerateConflictAdviceFlagDisabled mirrors the F1/F2 gate tests: a dark
// aiConflictBatch flag answers the wire-English "feature disabled" error and
// never spends an outbound request.
func TestGenerateConflictAdviceFlagDisabled(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"should never happen"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	out, err := svc.GenerateConflictAdvice("ours code", "theirs code", "base code")
	if err == nil {
		t.Fatalf("GenerateConflictAdvice succeeded with the flag off, want a feature-disabled error")
	}
	if !strings.Contains(err.Error(), "feature disabled (aiConflictBatch flag is off)") {
		t.Errorf(`error = %v, want the wire-English "feature disabled (aiConflictBatch flag is off)"`, err)
	}
	if out != "" {
		t.Errorf("GenerateConflictAdvice returned %q alongside an error", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("flag-off call hit the AI endpoint %d time(s), want 0", calls)
	}
}

// TestGenerateConflictAdviceGuards ensures empty ours/theirs content is
// rejected defensively before any outbound call happens; base stays optional.
func TestGenerateConflictAdviceGuards(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiConflictBatch":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"merged"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	cases := map[string][2]string{
		"empty ours":        {"", "theirs"},
		"whitespace ours":   {"  \n\t", "theirs"},
		"empty theirs":      {"ours", ""},
		"whitespace theirs": {"ours", " \n"},
		"both empty":        {"", ""},
	}
	for name, sides := range cases {
		if _, err := svc.GenerateConflictAdvice(sides[0], sides[1], ""); err == nil {
			t.Fatalf("GenerateConflictAdvice with %s succeeded, want an error", name)
		} else if !strings.Contains(err.Error(), "冲突内容为空") {
			t.Errorf("empty-content case %q error = %v, want an empty-content mention", name, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("guard-rejected calls hit the AI endpoint %d time(s), want 0", calls)
	}
}

// TestGenerateConflictAdviceOversizedTruncated feeds three-way content larger
// than the 120 KB cap and verifies the outbound prompt is truncated with an
// explicit note while the raw suggestion still comes back unparsed.
func TestGenerateConflictAdviceOversizedTruncated(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiConflictBatch":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	const tailMarker = "TAIL_MARKER_SHOULD_BE_TRUNCATED_AWAY"
	var mu sync.Mutex
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"  建议的合并结果\n风险说明  "}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	// Chinese lines keep the truncation on a real multi-byte rune boundary;
	// BASE is embedded last so tail truncation sacrifices it first.
	line := "冲突行内容：中文确保 UTF-8 边界安全\n"
	sides := strings.Repeat(line, aiConflictAdviceMaxBytes/len(line)/2)
	ours := sides + "OURS_TAIL\n"
	theirs := sides + "THEIRS_TAIL\n"
	base := sides + "BASE_TAIL\n" + tailMarker + "\n"
	if len(ours)+len(theirs)+len(base) <= aiConflictAdviceMaxBytes {
		t.Fatalf("test content is only %d bytes, want > %d",
			len(ours)+len(theirs)+len(base), aiConflictAdviceMaxBytes)
	}

	out, err := svc.GenerateConflictAdvice(ours, theirs, base)
	if err != nil {
		t.Fatalf("GenerateConflictAdvice failed: %v", err)
	}
	// Raw text is returned unparsed (only complete()'s whitespace trim).
	if out != "建议的合并结果\n风险说明" {
		t.Errorf("advice = %q, want the raw trimmed completion", out)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotBody, "[content truncated to first 120 KB]") {
		t.Errorf("outbound prompt lacks the truncation note: %s", gotBody)
	}
	if strings.Contains(gotBody, tailMarker) {
		t.Errorf("outbound prompt still carries the oversized tail past the 120 KB cap")
	}
	if !strings.Contains(gotBody, "conflict resolution advisor") {
		t.Errorf("outbound prompt lacks the fixed advice template: %s", gotBody)
	}
}

// TestGenerateConflictAdvicePromptAndRedaction verifies through a fake
// endpoint that ours/theirs/base all reach the prompt, an absent base omits
// the BASE section, and conflict text carrying secrets is redacted by
// complete()'s redactAIPayload before leaving the process.
func TestGenerateConflictAdvicePromptAndRedaction(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiConflictBatch":true},"schemaVersion":1}`)
	resetAISessionKeyForTest(t)

	var mu sync.Mutex
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		gotBody = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"merged result"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	// Ours carries a provider token; redactAIPayload inside complete() must
	// scrub it before the prompt leaves the process.
	ours := "token = \"" + aiTestGHToken + "\"\nOURS_MARKER"
	theirs := "THEIRS_MARKER"
	base := "BASE_MARKER"

	if _, err := svc.GenerateConflictAdvice(ours, theirs, base); err != nil {
		t.Fatalf("GenerateConflictAdvice failed: %v", err)
	}
	mu.Lock()
	if !strings.Contains(gotBody, "OURS_MARKER") {
		t.Errorf("outbound prompt missing ours content: %s", gotBody)
	}
	if !strings.Contains(gotBody, "THEIRS_MARKER") {
		t.Errorf("outbound prompt missing theirs content: %s", gotBody)
	}
	if !strings.Contains(gotBody, "BASE_MARKER") {
		t.Errorf("outbound prompt missing base content: %s", gotBody)
	}
	if strings.Contains(gotBody, aiTestGHToken) {
		t.Errorf("outbound prompt leaked the provider token: %s", gotBody)
	}
	if !strings.Contains(gotBody, "ghp_***") {
		t.Errorf("provider token was not masked: %s", gotBody)
	}
	if !strings.Contains(gotBody, "conflict content end") {
		t.Errorf("outbound prompt missing the closing content marker: %s", gotBody)
	}
	mu.Unlock()

	// A 2-way conflict (no base) must not fabricate a BASE section.
	if _, err := svc.GenerateConflictAdvice(ours, theirs, "  "); err != nil {
		t.Fatalf("GenerateConflictAdvice without base failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(gotBody, "BASE_MARKER") {
		t.Errorf("empty base still embedded a BASE section: %s", gotBody)
	}
}
