package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineQueueListNeverSerializesNullItems(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	res, err := svc.ManageOfflineSubmitQueue(repo, "list", "", "")
	if err != nil {
		t.Fatalf("ManageOfflineSubmitQueue failed: %v", err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal queue result: %v", err)
	}
	if strings.Contains(string(raw), `"items":null`) {
		t.Fatalf("queue items must not serialize as null: %s", raw)
	}
	if _, err := svc.ManageOfflineSubmitQueue(repo, "NOT_AN_ACTION", "", ""); err == nil {
		t.Fatal("unknown offline queue actions must fail instead of silently listing")
	}
}

func TestOfflineQueueRejectsUnknownOperations(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if _, err := svc.ManageOfflineSubmitQueue(repo, "enqueue", "", `{"operation":"config --global user.name hacked"}`); err == nil {
		t.Fatal("enqueue must reject unknown operations")
	}

	queuePath := getOfflineQueuePath(repo)
	if err := os.MkdirAll(filepath.Dir(queuePath), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"items":[{"id":"q1","operation":"config --global user.name hacked","target":"","status":"queued","retryCount":0,"maxRetries":5,"backoffSecs":5,"createdAt":"2026-10-24 00:00:00"}]}`
	if err := os.WriteFile(queuePath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := svc.ManageOfflineSubmitQueue(repo, "replay", "all", "")
	if err != nil {
		t.Fatalf("replay should report a per-item failure instead of executing it: %v", err)
	}
	if len(res.Items) != 1 || !strings.Contains(res.Items[0].LastError, "不支持的离线操作") {
		t.Fatalf("unknown operation must be rejected on replay: %+v", res.Items)
	}
}

func TestOfflineQueueCorruptionIsNotSilentlyOverwritten(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	qPath := getOfflineQueuePath(repo)
	if err := os.MkdirAll(filepath.Dir(qPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const corrupt = `{"items":[{"id":"keep-me"` // truncated JSON
	if err := os.WriteFile(qPath, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}

	payload := `{"operation":"push","target":"origin","payload":"x"}`
	if _, err := svc.ManageOfflineSubmitQueue(repo, "enqueue", "new-id", payload); err == nil {
		t.Fatal("corrupt queue must fail instead of being overwritten")
	}
	if data, err := os.ReadFile(qPath); err != nil || string(data) != corrupt {
		t.Fatalf("corrupt queue must be preserved for recovery: %q err=%v", data, err)
	}

	if err := os.Remove(qPath); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ManageOfflineSubmitQueue(repo, "enqueue", "new-id", payload); err != nil {
		t.Fatalf("enqueue after removing corrupt file failed: %v", err)
	}
	items, err := loadOfflineQueue(repo)
	if err != nil || len(items) != 1 || items[0].ID != "new-id" {
		t.Fatalf("expected one queued item, got %+v err=%v", items, err)
	}
	if !strings.Contains(items[0].Operation, "push") {
		t.Fatalf("unexpected queued operation: %+v", items[0])
	}
}
