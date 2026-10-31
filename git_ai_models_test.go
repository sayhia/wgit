package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAIModelsSortsDeduplicatesAndUsesSameOriginKey(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"data":[{"id":"z-model"},{"id":"a-model"},{"id":"z-model"}]}`)
	}))
	t.Cleanup(srv.Close)

	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: srv.URL + "/v1", Model: "m", APIKey: aiTestKey,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	result := svc.ListAIModels(srv.URL+"/v1", "", "openai", "")
	if !result.OK {
		t.Fatalf("result = %#v", result)
	}
	if strings.Join(result.Models, ",") != "a-model,z-model" {
		t.Fatalf("models = %#v", result.Models)
	}
	if authHeader != "Bearer "+aiTestKey {
		t.Fatalf("auth header = %q, want the same-origin stored key", authHeader)
	}
}

func TestListAIModelsDoesNotSendStoredKeyToAnotherOrigin(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	originA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"a"}]}`)
	}))
	t.Cleanup(originA.Close)

	var authHeader = "unset"
	originB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"data":[{"id":"b"}]}`)
	}))
	t.Cleanup(originB.Close)

	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: originA.URL, Model: "m", APIKey: aiTestKey,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	result := svc.ListAIModels(originB.URL, "", "openai", "")
	if !result.OK || strings.Join(result.Models, ",") != "b" {
		t.Fatalf("result = %#v", result)
	}
	if authHeader != "" {
		t.Fatalf("stored key leaked to a different origin: %q", authHeader)
	}
}

func TestListAIModelsAcceptsExplicitKeyAndNativeOllamaShape(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"models":[{"name":"llama3.2"},{"id":"qwen2.5"}]}`)
	}))
	t.Cleanup(srv.Close)

	result := (AiService{}).ListAIModels(srv.URL, "sk-new-1234567890", "openai", "")
	if !result.OK || strings.Join(result.Models, ",") != "llama3.2,qwen2.5" {
		t.Fatalf("result = %#v", result)
	}
	if authHeader != "Bearer sk-new-1234567890" {
		t.Fatalf("auth header = %q", authHeader)
	}
}

func TestListAIModelsHandlesMissingListAndInvalidEndpoint(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"flags":{"aiCommitMsg":true},"schemaVersion":2}`)
	resetAISessionKeyForTest(t)

	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	result := (AiService{}).ListAIModels(srv.URL, "", "openai", "")
	if !result.OK || len(result.Models) != 0 || !strings.Contains(result.Message, "未提供") {
		t.Fatalf("404 result = %#v", result)
	}

	bad := (AiService{}).ListAIModels("localhost:11434/v1", "", "openai", "")
	if bad.OK || !strings.Contains(bad.Message, "http://") {
		t.Fatalf("invalid endpoint result = %#v", bad)
	}
	userinfo := (AiService{}).ListAIModels("http://user:secret@127.0.0.1/v1", "", "openai", "")
	if userinfo.OK || !strings.Contains(userinfo.Message, "用户名或密码") {
		t.Fatalf("userinfo result = %#v", userinfo)
	}
}
