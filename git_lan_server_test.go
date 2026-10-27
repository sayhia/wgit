package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestLanReviewServerLifecycle(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	// 1. Initial status
	initial, err := svc.GetLanReviewServerStatus()
	if err != nil {
		t.Fatalf("GetLanReviewServerStatus failed: %v", err)
	}
	if initial == nil {
		t.Fatal("expected non-nil LanServerInfo")
	}

	// 2. Start server on high port to prevent conflict
	testPort := 18991
	info, err := svc.StartLanReviewServer(repo, testPort)
	if err != nil {
		t.Fatalf("StartLanReviewServer failed: %v", err)
	}
	if !info.Running || info.Port != testPort {
		t.Fatalf("unexpected info after start: %+v", info)
	}

	// Let listener bind
	time.Sleep(50 * time.Millisecond)

	// 3. HTTP GET /api/status
	resp, err := http.Get(info.URL + "/api/status")
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var statusMap map[string]any
		if err := json.Unmarshal(body, &statusMap); err == nil {
			if statusMap["status"] != "running" {
				t.Errorf("expected status 'running', got %v", statusMap["status"])
			}
		}
	}

	// 4. Stop server
	stopped, err := svc.StopLanReviewServer()
	if err != nil {
		t.Fatalf("StopLanReviewServer failed: %v", err)
	}
	if stopped.Running {
		t.Errorf("expected Running to be false after StopLanReviewServer, got true")
	}
}
