package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func configureAPITestEndpoint(t *testing.T, baseURL string) {
	t.Helper()
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true,"aiSummary":true,"aiConflictBatch":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	if err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: baseURL, Model: "test-model", TimeoutSeconds: 5,
	}); err != nil {
		t.Fatalf("SaveAISettings: %v", err)
	}
}

func TestCompleteAPIStreamParsesSSEChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("test response writer is not a flusher")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"feat: \"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, ": keepalive\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"streaming\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)

	var deltas []string
	text, usage, err := (AiService{}).completeAPIStreamUsage(context.Background(), "prompt", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("completeAPIStream: %v", err)
	}
	if text != "feat: streaming" {
		t.Fatalf("text = %q", text)
	}
	if strings.Join(deltas, "|") != "feat: |streaming" {
		t.Fatalf("deltas = %#v", deltas)
	}
	if usage.In != 3 || usage.Out != 2 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestCompleteAPIStreamFallsBackToJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"fix: non-stream fallback"}}]}`)
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)

	var deltas []string
	text, err := (AiService{}).completeAPIStream(context.Background(), "prompt", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("completeAPIStream: %v", err)
	}
	if text != "fix: non-stream fallback" {
		t.Fatalf("text = %q", text)
	}
	if len(deltas) != 1 || deltas[0] != text {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestCompleteAPIStreamCancelsInFlightRequest(t *testing.T) {
	firstSent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"first\"}}]}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		close(firstSent)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var deltas []string
	deltaSeen := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := (AiService{}).completeAPIStream(ctx, "prompt", func(delta string) error {
			deltas = append(deltas, delta)
			select {
			case deltaSeen <- struct{}{}:
			default:
			}
			return nil
		})
		done <- err
	}()
	<-firstSent
	<-deltaSeen
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled API stream did not return")
	}
	if strings.Join(deltas, "") != "first" {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestCompleteCLIStreamEmitsStandardOutputIncrementally(t *testing.T) {
	command := writeFakeAICLI(t, `printf 'feat: '
sleep 0.05
printf 'cli\n'`)
	cfg := AIConfig{CLICommand: command, CLIInput: "stdin", Model: "m", TimeoutSeconds: 5}
	var deltas []string
	text, err := (AiService{}).completeCLIStream(context.Background(), cfg, "", "prompt", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("completeCLIStream: %v", err)
	}
	if text != "feat: cli" {
		t.Fatalf("text = %q", text)
	}
	if len(deltas) < 2 || !strings.Contains(strings.Join(deltas, ""), "feat: cli") {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestCompleteCLIStreamCancelsChildProcess(t *testing.T) {
	command := writeFakeAICLI(t, `printf 'first\n'
sleep 5
printf 'late\n'`)
	cfg := AIConfig{CLICommand: command, CLIInput: "stdin", Model: "m", TimeoutSeconds: 10}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var deltas []string
	started := time.Now()
	_, err := (AiService{}).completeCLIStream(ctx, cfg, "", "prompt", func(delta string) error {
		deltas = append(deltas, delta)
		cancel()
		return nil
	})
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cancelled CLI stream took %s, want under 3s", elapsed)
	}
	if strings.Join(deltas, "") != "first\n" {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestStartAIStreamValidatesRequestBeforeStarting(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	svc := AiService{}
	if err := svc.StartAIStream(AIStreamInput{RequestID: "bad id with spaces", Kind: "commit"}); err == nil {
		t.Fatal("invalid request ID was accepted")
	}
	repo := tempGitRepoForAI(t)
	err := svc.StartAIStream(AIStreamInput{RequestID: "stream-off", Kind: "commit", RepoPath: repo, Diff: "diff"})
	if err == nil || !strings.Contains(err.Error(), "feature disabled") {
		t.Fatalf("flag-off stream error = %v", err)
	}
	if err := svc.StartAIStream(AIStreamInput{RequestID: "stream-unknown", Kind: "unknown"}); err == nil {
		t.Fatal("unknown stream kind was accepted")
	}
}

func TestBuildExplainDiffPrompt(t *testing.T) {
	prompt, err := buildExplainDiffPrompt("src/app.go", "diff --git a/app.go b/app.go\n+fmt.Println(1)")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"File: src/app.go", "diff --git", "--- diff end ---"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
	if _, err := buildExplainDiffPrompt("", "  "); err == nil {
		t.Fatal("empty diff was accepted")
	}
}

func TestCommitMessagePromptOptionsAndPostProcessing(t *testing.T) {
	repo := tempGitRepoForAI(t)
	if _, err := ExecGit(repo, "commit", "--allow-empty", "-m", "feat(ui): existing style"); err != nil {
		t.Fatal(err)
	}
	prompt, err := buildCommitMessagePrompt(repo, "diff --git a/x b/x\n+line", commitMessageOptions{Language: "zh", IncludeBody: true, Instructions: "use our scope list"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Write the subject and body in Chinese", "concise body", "existing style", "use our scope list"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
	combined, err := postProcessCommitMessageDetailed("feat(core): subject\n\nbody line one\nbody line two")
	if err != nil {
		t.Fatal(err)
	}
	if combined != "feat(core): subject\n\nbody line one\nbody line two" {
		t.Fatalf("combined = %q", combined)
	}
	subjectOnly, err := postProcessCommitMessageDetailed("fix(ui): only subject\n")
	if err != nil || subjectOnly != "fix(ui): only subject" {
		t.Fatalf("subjectOnly = %q err=%v", subjectOnly, err)
	}
}

func TestAIAuditRecordsMetadata(t *testing.T) {
	ClearRecentActivityLogs()
	defer ClearRecentActivityLogs()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)
	if _, err := (AiService{}).completeAPI(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	logs := GetRecentActivityLogs()
	if len(logs) == 0 {
		t.Fatal("missing AI log")
	}
	entry := logs[len(logs)-1]
	if entry.Source != "ai" || entry.Mode != "api" || entry.Model != "test-model" || entry.Kind != "complete" {
		t.Fatalf("AI log metadata = %+v", entry)
	}
}

func TestPreviewAIRequestRedactsSecrets(t *testing.T) {
	enableAIFlags(t)
	resetAISessionKeyForTest(t)
	repo := tempGitRepoForAI(t)
	preview, err := (AiService{}).PreviewAIRequest(AIStreamInput{
		Kind: "commit", RepoPath: repo, Diff: "diff --git a/x b/x\n+key=sk-live-abcdef1234567890",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(preview, "sk-live-abcdef1234567890") {
		t.Fatalf("preview leaked the key: %s", preview)
	}
	if !strings.Contains(preview, "staged diff begin") {
		t.Fatalf("preview is not the commit prompt: %s", preview)
	}
}

func TestCompleteAPIStreamTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	configureAPITestEndpoint(t, srv.URL)
	// The saved timeout is 5s; a parent context makes the test fast and
	// exercises the same deadline path.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := (AiService{}).completeAPIStream(ctx, "hello", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("err = %v, want timeout", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
}
