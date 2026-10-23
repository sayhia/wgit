package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
