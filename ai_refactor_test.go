package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func writeFakeAICLI(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-ai")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func enableAIFlags(t *testing.T) {
	t.Helper()
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true,"aiSummary":true,"aiConflictBatch":true},"schemaVersion":2}`)
}

func TestCLIModeRunsLocalCommandWithStdin(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	command := writeFakeAICLI(t, `bytes=$(wc -c | tr -d ' ')
echo "feat: stdin-$bytes"`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "cli", CLICommand: command, CLIInput: "stdin", TimeoutSeconds: 10,
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
	repo := tempGitRepoForAI(t)
	out, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n")
	if err != nil {
		t.Fatalf("GenerateCommitMessage: %v", err)
	}
	if !strings.HasPrefix(out, "feat: stdin-") {
		t.Fatalf("output = %q, want the command to receive the prompt on stdin", out)
	}
}

func TestCLIModeSupportsArgTransport(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	command := writeFakeAICLI(t, `echo "fix: arg-$1"`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "cli", CLICommand: command, CLIInput: "arg", TimeoutSeconds: 10,
	}); err != nil {
		t.Fatal(err)
	}
	repo := tempGitRepoForAI(t)
	out, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "fix: arg-") {
		t.Fatalf("output = %q, want the prompt appended as an argument", out)
	}
}

func TestCLIModeReportsStderrAndExitCode(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	command := writeFakeAICLI(t, `echo "model exploded" >&2
exit 2`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{Mode: "cli", CLICommand: command, TimeoutSeconds: 10}); err != nil {
		t.Fatal(err)
	}
	repo := tempGitRepoForAI(t)
	_, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n")
	if err == nil || !strings.Contains(err.Error(), "model exploded") {
		t.Fatalf("err = %v, want the CLI stderr surfaced", err)
	}
}

func TestCLIModeTimesOut(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	command := writeFakeAICLI(t, `sleep 5
echo "feat: too late"`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{Mode: "cli", CLICommand: command, TimeoutSeconds: 1}); err != nil {
		t.Fatal(err)
	}
	repo := tempGitRepoForAI(t)
	started := time.Now()
	_, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n")
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("timeout took %s, want the 1s budget enforced", elapsed)
	}
}

func TestAPIKeySurvivesASessionReset(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)

	var mu sync.Mutex
	var authorization string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorization = r.Header.Get("Authorization")
		mu.Unlock()
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"feat: after restart"}}]}`))
	}))
	defer srv.Close()

	svc := AiService{}
	if err := svc.SaveAIConfig(srv.URL, "test-model", 5, aiTestKey); err != nil {
		t.Fatal(err)
	}
	// Simulate a restart: the in-process cache is gone, the secret store is not.
	setAISessionKey("")
	if cfg := svc.GetAIConfig(); !cfg.KeyAvailable {
		t.Fatalf("KeyAvailable = false after the session reset; config = %#v", cfg)
	}
	repo := tempGitRepoForAI(t)
	if _, err := svc.GenerateCommitMessage(repo, "diff --git a/x b/x\n+line\n"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if authorization != "Bearer "+aiTestKey {
		t.Fatalf("Authorization = %q, want the recovered key", authorization)
	}
}

func TestClearAIKeyRemovesTheStoredSecret(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	svc := AiService{}
	if err := svc.SaveAIConfig("http://127.0.0.1:1/v1", "m", 5, aiTestKey); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearAIKey(); err != nil {
		t.Fatal(err)
	}
	if cfg := svc.GetAIConfig(); cfg.HasAPIKey || cfg.KeyAvailable || cfg.KeyFingerprint != "" {
		t.Fatalf("config still carries a key after ClearAIKey: %#v", cfg)
	}
	if key := getAISessionKey(); key != "" {
		t.Fatalf("session key = %q after ClearAIKey", key)
	}
}

func TestDetectAICLIsReportsAvailableCommand(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "claude")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '2.1.0 (Claude Code)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	options := (AiService{}).DetectAICLIs()
	var claude *AICLIOption
	for i := range options {
		if options[i].ID == "claude" {
			claude = &options[i]
		}
	}
	if claude == nil || !claude.Available || !strings.Contains(claude.Version, "2.1.0") {
		t.Fatalf("claude detection = %#v, want available with its version", claude)
	}
}

func TestSaveAISettingsValidatesMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetAISessionKeyForTest(t)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: ""}); err == nil {
		t.Fatal("API mode without an endpoint was accepted")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: "http://127.0.0.1:1/v1"}); err == nil {
		t.Fatal("API mode without a model was accepted")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: "127.0.0.1:11434/v1", Model: "m"}); err == nil {
		t.Fatal("API mode without an http(s) scheme was accepted")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: "http://user:pass@127.0.0.1/v1", Model: "m"}); err == nil {
		t.Fatal("API endpoint credentials were persisted instead of using the key store")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: "http://127.0.0.1:1/v1", Model: "m", APIKey: "a\nb"}); err == nil {
		t.Fatal("an API key containing a newline was accepted")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "cli"}); err == nil {
		t.Fatal("CLI mode without a command was accepted")
	}
	if err := svc.SaveAISettings(AISaveInput{Mode: "nope", BaseURL: "http://x"}); err == nil {
		t.Fatal("an unknown mode was accepted")
	}
}

func TestTestAIConnectionResultReportsCLI(t *testing.T) {
	// This test persists a CLI config; keep it out of the real user config.
	t.Setenv("HOME", t.TempDir())
	command := writeFakeAICLI(t, `echo "fake 1.0"`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{Mode: "cli", CLICommand: command}); err != nil {
		t.Fatal(err)
	}
	result := svc.TestAIConnectionResult(context.Background())
	if !result.OK || result.Mode != "cli" || !strings.Contains(result.Detail, "fake 1.0") {
		t.Fatalf("result = %#v", result)
	}
}

// TestGeminiPresetStaysHeadless guards the preset against reintroducing "-p":
// Gemini CLI reads piped stdin only when no -p flag is present, while a bare
// "-p" waits for an argv prompt and would hang until the timeout.
func TestGeminiPresetStaysHeadless(t *testing.T) {
	for _, preset := range aiCLIPresets {
		if preset.ID != "gemini" {
			continue
		}
		if preset.Input != "stdin" || len(preset.Args) != 0 {
			t.Fatalf("gemini preset = %#v, want stdin with no args", preset)
		}
		return
	}
	t.Fatal("gemini preset is missing")
}

func TestNormalizeAPIOrigin(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://Example.COM/v1", "http://example.com:80"},
		{"http://example.com:80/v1", "http://example.com:80"},
		{"https://example.com/v1", "https://example.com:443"},
		{"https://example.com:8443/v1", "https://example.com:8443"},
		{"http://[::1]:11434/v1", "http://[::1]:11434"},
		{"", ""},
		{"localhost:11434/v1", ""},
	}
	for _, tc := range cases {
		if got := normalizeAPIOrigin(tc.in); got != tc.want {
			t.Errorf("normalizeAPIOrigin(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSaveAISettingsBindsKeyToEndpointOrigin(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	svc := AiService{}
	const originA = "http://127.0.0.1:18080"
	save := func(base, key string) error {
		return svc.SaveAISettings(AISaveInput{
			Mode: "api", BaseURL: base, Model: "m", TimeoutSeconds: 30, APIKey: key,
		})
	}

	if err := save(originA+"/v1", aiTestKey); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	// Path-only changes keep the key without friction.
	if err := save(originA+"/v2", ""); err != nil {
		t.Fatalf("path-only change was rejected: %v", err)
	}
	if got := getAISessionKey(); got != aiTestKey {
		t.Fatalf("key after path change = %q", got)
	}

	// Per-origin keys: changing the host keeps the old key for its origin but
	// does not send it to the new host.
	if err := save("http://127.0.0.1:18081/v1", ""); err != nil {
		t.Fatalf("origin change was rejected: %v", err)
	}
	if got := getAISessionKey(); got != "" {
		t.Fatalf("new origin inherited the old key: %q", got)
	}
	if got, _ := getAIStoredKeyForOrigin(originA, false); got != aiTestKey {
		t.Fatalf("old origin key was lost: %q", got)
	}
	// Switching back with no new key retrieves the key scoped to that origin.
	if err := save(originA+"/v1", ""); err != nil {
		t.Fatalf("switch back failed: %v", err)
	}
	if got := getAISessionKey(); got != aiTestKey {
		t.Fatalf("switch-back key = %q", got)
	}

	// Scheme, port and host variants are all distinct origins.
	for _, base := range []string{
		"https://127.0.0.1:18080/v1",
		"http://127.0.0.1:18082/v1",
		"http://localhost:18080/v1",
	} {
		if err := save(base, ""); err != nil {
			t.Fatalf("origin change %q was rejected: %v", base, err)
		}
		if got := getAISessionKey(); got != "" {
			t.Fatalf("origin %q inherited a key: %q", base, got)
		}
	}
	// CLI mode may edit the base without touching the keyring.
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "cli", BaseURL: "http://127.0.0.1:18081/v1", CLICommand: "echo",
	}); err != nil {
		t.Fatalf("CLI-mode origin change: %v", err)
	}

	// A new key is scoped to its own origin.
	const newKey = "sk-new-key-abcdef1234567890"
	if err := save("http://127.0.0.1:18081/v1", newKey); err != nil {
		t.Fatalf("new key move was rejected: %v", err)
	}
	if got := getAISessionKey(); got != newKey {
		t.Fatalf("new key = %q", got)
	}
	if got, _ := getAIStoredKeyForOrigin(originA, false); got != aiTestKey {
		t.Fatalf("origin A key was overwritten: %q", got)
	}

	// Clearing only removes the current origin's key.
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "http://127.0.0.1:18081/v1", Model: "m", ClearAPIKey: true,
	}); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	if got := getAISessionKey(); got != "" {
		t.Fatalf("key after clear = %q", got)
	}
	if got, _ := getAIStoredKeyForOrigin(originA, false); got != aiTestKey {
		t.Fatalf("clear removed another origin's key: %q", got)
	}
	if got := LoadConfig().AI.APIKey; got != "" {
		t.Fatalf("fingerprint after clear = %q", got)
	}
}

type delayedSecretStore struct {
	mu      sync.Mutex
	key     string
	started chan struct{}
	release chan struct{}
}

func (s *delayedSecretStore) Get() (string, error) {
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key, nil
}

func (s *delayedSecretStore) Set(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	return nil
}

func (s *delayedSecretStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = ""
	return nil
}

func TestSessionKeyClearWinsOverInFlightLoad(t *testing.T) {
	previous := aiSecrets
	store := &delayedSecretStore{key: "old-key", started: make(chan struct{}, 1), release: make(chan struct{})}
	aiSecrets = store
	t.Cleanup(func() { aiSecrets = previous })
	resetAISessionKeyForTest(t)

	done := make(chan string, 1)
	go func() { done <- getAISessionKey() }()
	<-store.started
	setAISessionKey("")
	_ = store.Delete()
	close(store.release)
	if got := <-done; got != "" {
		t.Fatalf("in-flight load returned %q after clear, want empty", got)
	}
	if got := getAISessionKey(); got != "" {
		t.Fatalf("cleared key was re-cached: %q", got)
	}
}

type countingSecretStore struct {
	mu   sync.Mutex
	key  string
	gets int
}

func (s *countingSecretStore) Get() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	return s.key, nil
}
func (s *countingSecretStore) Set(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	return nil
}
func (s *countingSecretStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = ""
	return nil
}
func (s *countingSecretStore) getCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gets
}

func TestGetAIConfigUsesWarmSessionCache(t *testing.T) {
	previous := aiSecrets
	store := &countingSecretStore{}
	aiSecrets = store
	t.Cleanup(func() { aiSecrets = previous })
	setAISessionKey("cached-key")
	t.Cleanup(func() { setAISessionKey("") })

	_ = AiService{}.GetAIConfig()
	if got := store.getCount(); got != 0 {
		t.Fatalf("GetAIConfig performed %d secret-store reads with a warm cache", got)
	}
}

func TestAIDeepTestPerformsRealGeneration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"OK"}}]}`)
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)

	result := (AiService{}).TestAIDeepResult(context.Background())
	if !result.OK || !strings.Contains(result.Detail, "OK") {
		t.Fatalf("deep result = %#v", result)
	}
}

func TestAIDeepTestCLIAndFailure(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	command := writeFakeAICLI(t, `echo "OK"`)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{Mode: "cli", CLICommand: command, TimeoutSeconds: 5}); err != nil {
		t.Fatal(err)
	}
	result := svc.TestAIDeepResult(context.Background())
	if !result.OK || !strings.Contains(result.Detail, "OK") {
		t.Fatalf("CLI deep result = %#v", result)
	}

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(failSrv.Close)
	if err := svc.SaveAISettings(AISaveInput{Mode: "api", BaseURL: failSrv.URL, Model: "m", TimeoutSeconds: 5}); err != nil {
		t.Fatal(err)
	}
	failResult := svc.TestAIDeepResult(context.Background())
	if failResult.OK || !strings.Contains(failResult.Detail, "502") {
		t.Fatalf("failure deep result = %#v", failResult)
	}
}
