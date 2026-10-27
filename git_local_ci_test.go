package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalCiSandboxAndWebhook(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	_, _ = ExecGit(repo, "config", "user.name", "CI Bot")
	_, _ = ExecGit(repo, "config", "user.email", "ci@example.com")

	_ = os.WriteFile(filepath.Join(repo, "test.sh"), []byte("#!/bin/sh\necho 'ci test success'\n"), 0755)
	_, _ = ExecGit(repo, "add", "test.sh")
	_, _ = ExecGit(repo, "commit", "-m", "chore: setup ci script")

	// 1. Run local CI sandbox with multiple steps
	commands := []string{
		"git status",
		"sh test.sh",
	}
	ciRes, err := svc.RunLocalCiSandbox(repo, commands, 15)
	if err != nil {
		t.Fatalf("RunLocalCiSandbox failed: %v", err)
	}
	if !ciRes.Success {
		t.Fatalf("expected CI to succeed: %s", ciRes.Summary)
	}
	if len(ciRes.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(ciRes.Steps))
	}
	if !ciRes.Steps[1].Success {
		t.Errorf("step 2 should have succeeded, output: %s", ciRes.Steps[1].Output)
	}

	// 2. Dispatch Team Webhook to mock server
	var receivedBody string
	var receivedSig string
	var receivedEvent string

	secret := "super-team-secret"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		receivedSig = r.Header.Get("X-Hub-Signature-256")
		receivedEvent = r.Header.Get("X-WGit-Event")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"delivered"}`))
	}))
	defer ts.Close()

	payload := `{"build":"success","duration":120}`
	whRes, err := svc.DispatchTeamWebhook(ts.URL, "ci.completed", payload, secret)
	if err != nil {
		t.Fatalf("DispatchTeamWebhook failed: %v", err)
	}
	if !whRes.Success {
		t.Fatalf("expected webhook dispatch success, got code %d", whRes.StatusCode)
	}
	if whRes.StatusCode != 200 {
		t.Errorf("expected 200, got %d", whRes.StatusCode)
	}
	if receivedEvent != "ci.completed" {
		t.Errorf("expected event ci.completed, got %s", receivedEvent)
	}
	if receivedBody != payload {
		t.Errorf("expected body %s, got %s", payload, receivedBody)
	}

	// Validate HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if receivedSig != expectedSig {
		t.Errorf("expected HMAC signature %s, got %s", expectedSig, receivedSig)
	}
}
