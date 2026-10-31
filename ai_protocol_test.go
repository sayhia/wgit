package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func saveProtocolConfig(t *testing.T, base, model, protocol, key string, headers map[string]string) {
	t.Helper()
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true,"aiSummary":true,"aiConflictBatch":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	if err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: base, Model: model, TimeoutSeconds: 10,
		Protocol: protocol, APIKey: key, ExtraHeaders: headers,
	}); err != nil {
		t.Fatalf("save protocol config: %v", err)
	}
}

func TestAnthropicProtocolRequestAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-ant-test" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q", got)
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		if !strings.Contains(string(body), `"model":"claude-test"`) || !strings.Contains(string(body), `"messages"`) {
			t.Errorf("body = %s", body)
		}
		fmt.Fprint(w, `{"content":[{"type":"text","text":"anthropic-ok"}]}`)
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL+"/v1", "claude-test", "anthropic", "sk-ant-test", nil)

	out, err := (AiService{}).completeAPI(context.Background(), "hello")
	if err != nil || out != "anthropic-ok" {
		t.Fatalf("completeAPI = %q err=%v", out, err)
	}
}

func TestGeminiProtocolRequestAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "sk-gemini-test" {
			t.Errorf("x-goog-api-key = %q", got)
		}
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"gemini-ok"}]}}]}`)
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL+"/v1beta", "gemini-test", "gemini", "sk-gemini-test", nil)

	out, err := (AiService{}).completeAPI(context.Background(), "hello")
	if err != nil || out != "gemini-ok" {
		t.Fatalf("completeAPI = %q err=%v", out, err)
	}
}

func TestAzureProtocolRequestAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/deployments/deploy-test/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("api-version") != "2024-10-21" {
			t.Errorf("api-version = %q", r.URL.Query().Get("api-version"))
		}
		if got := r.Header.Get("api-key"); got != "azure-key" {
			t.Errorf("api-key = %q", got)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"azure-ok"}}]}`)
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL, "deploy-test", "azure", "azure-key", nil)

	out, err := (AiService{}).completeAPI(context.Background(), "hello")
	if err != nil || out != "azure-ok" {
		t.Fatalf("completeAPI = %q err=%v", out, err)
	}
}

func TestAnthropicStreamingProtocol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"claude-\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"stream\"}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL+"/v1", "claude-test", "anthropic", "sk-ant-test", nil)

	var deltas []string
	out, err := (AiService{}).completeAPIStream(context.Background(), "hello", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil || out != "claude-stream" || strings.Join(deltas, "") != "claude-stream" {
		t.Fatalf("out=%q deltas=%#v err=%v", out, deltas, err)
	}
}

func TestExtraHeadersAndReservedHeaderValidation(t *testing.T) {
	var gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTitle = r.Header.Get("X-Title")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL, "m", "openai", "", map[string]string{"X-Title": "wgit"})
	if _, err := (AiService{}).completeAPI(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if gotTitle != "wgit" {
		t.Fatalf("X-Title = %q", gotTitle)
	}

	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	err := (AiService{}).SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: srv.URL, Model: "m",
		ExtraHeaders: map[string]string{"Authorization": "nope"},
	})
	if err == nil || !strings.Contains(err.Error(), "系统管理") {
		t.Fatalf("reserved header error = %v", err)
	}
}

func TestCLIEnvironmentIsolation(t *testing.T) {
	// The completion funnel records a durable usage entry; keep it out of the
	// real user config directory.
	t.Setenv("WGIT_CONFIG_DIR", t.TempDir())
	t.Setenv("WGIT_TEST_SECRET", "host-secret-value")
	command := writeFakeAICLI(t, `printf '%s' "$WGIT_TEST_SECRET"`)

	inheritCfg := AIConfig{CLICommand: command, CLIInput: "stdin", TimeoutSeconds: 5, CLIEnvMode: "inherit"}
	out, err := (AiService{}).completeCLI(context.Background(), inheritCfg, "", "hello")
	if err != nil || out != "host-secret-value" {
		t.Fatalf("inherit env out=%q err=%v", out, err)
	}

	minimalCfg := inheritCfg
	minimalCfg.CLIEnvMode = "minimal"
	if out, err := (AiService{}).completeCLI(context.Background(), minimalCfg, "", "hello"); err == nil || out != "" {
		t.Fatalf("minimal env leaked host secret: out=%q err=%v", out, err)
	}
}

func TestListAIModelsProtocols(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	geminiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" {
			t.Errorf("gemini path = %q", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "g-key" {
			t.Errorf("gemini key header missing")
		}
		fmt.Fprint(w, `{"models":[{"name":"models/gemini-2.0"},{"name":"models/gemini-1.5"}]}`)
	}))
	t.Cleanup(geminiSrv.Close)
	res := (AiService{}).ListAIModels(geminiSrv.URL+"/v1beta", "g-key", "gemini", "")
	if !res.OK || strings.Join(res.Models, ",") != "gemini-1.5,gemini-2.0" {
		t.Fatalf("gemini models = %#v", res)
	}

	anthropicSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("x-api-key") != "a-key" {
			t.Errorf("anthropic request path=%q key=%q", r.URL.Path, r.Header.Get("x-api-key"))
		}
		fmt.Fprint(w, `{"data":[{"id":"claude-3"}]}`)
	}))
	t.Cleanup(anthropicSrv.Close)
	res = (AiService{}).ListAIModels(anthropicSrv.URL+"/v1", "a-key", "anthropic", "")
	if !res.OK || strings.Join(res.Models, ",") != "claude-3" {
		t.Fatalf("anthropic models = %#v", res)
	}
}

func TestGeminiStreamingProtocol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"gemini-\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"stream\"}]}}]}\n\n")
	}))
	t.Cleanup(srv.Close)
	saveProtocolConfig(t, srv.URL+"/v1beta", "gemini-test", "gemini", "sk-gemini-test", nil)

	var deltas []string
	out, err := (AiService{}).completeAPIStream(context.Background(), "hello", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil || out != "gemini-stream" || strings.Join(deltas, "") != "gemini-stream" {
		t.Fatalf("out=%q deltas=%#v err=%v", out, deltas, err)
	}
}
