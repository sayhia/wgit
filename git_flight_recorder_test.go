package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlightRecorderSnapshots(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-flight-recorder-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	// Initialize git repo
	if _, err := ExecGit(tempDir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := ExecGit(tempDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	if _, err := ExecGit(tempDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}

	// Create initial commit
	file1 := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("initial version 1\n"), 0644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if _, err := ExecGit(tempDir, "add", "file1.txt"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := ExecGit(tempDir, "commit", "-m", "chore: initial commit"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Modify file1 and add untracked file2
	if err := os.WriteFile(file1, []byte("modified version 1 - critical work in progress\n"), 0644); err != nil {
		t.Fatalf("failed to update file1: %v", err)
	}
	file2 := filepath.Join(tempDir, "untracked.asset")
	if err := os.WriteFile(file2, []byte("brand new game asset untracked\n"), 0644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	// 1. Take Flight Recorder Snapshot
	snap, err := svc.CreateFlightRecorderSnapshot(tempDir, "pre-reset", "Emergency backup before reset")
	if err != nil {
		t.Fatalf("CreateFlightRecorderSnapshot failed: %v", err)
	}
	if snap.ID == "" {
		t.Fatalf("expected non-empty snapshot ID")
	}
	if snap.FileCount != 2 {
		t.Fatalf("expected 2 files in snapshot, got %d", snap.FileCount)
	}

	// 2. List snapshots
	list, err := svc.ListFlightSnapshots(tempDir)
	if err != nil {
		t.Fatalf("ListFlightSnapshots failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(list))
	}
	if list[0].ID != snap.ID {
		t.Fatalf("expected ID %s, got %s", snap.ID, list[0].ID)
	}

	// 3. Simulate disaster: hard reset and clean away untracked files
	if _, err := ExecGit(tempDir, "reset", "--hard", "HEAD"); err != nil {
		t.Fatalf("git reset --hard failed: %v", err)
	}
	if _, err := ExecGit(tempDir, "clean", "-fd"); err != nil {
		t.Fatalf("git clean -fd failed: %v", err)
	}

	// Verify files were clobbered
	data1, _ := os.ReadFile(file1)
	if string(data1) != "initial version 1\n" {
		t.Fatalf("expected file1 to be reset, got: %s", string(data1))
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Fatalf("expected untracked file2 to be wiped")
	}

	// 4. Restore using Flight Recorder!
	restoreRes, err := svc.RestoreFlightSnapshot(tempDir, snap.ID)
	if err != nil {
		t.Fatalf("RestoreFlightSnapshot failed: %v", err)
	}
	if !restoreRes.Success {
		t.Fatalf("RestoreFlightSnapshot returned unsuccessful: %s", restoreRes.Message)
	}

	// Verify files are restored
	restoredData1, err := os.ReadFile(file1)
	if err != nil || string(restoredData1) != "modified version 1 - critical work in progress\n" {
		t.Fatalf("file1 not restored properly: %s", string(restoredData1))
	}
	restoredData2, err := os.ReadFile(file2)
	if err != nil || string(restoredData2) != "brand new game asset untracked\n" {
		t.Fatalf("file2 not restored properly: %s", string(restoredData2))
	}

	// 5. Test snapshot purging
	for i := 0; i < 5; i++ {
		_, _ = svc.CreateFlightRecorderSnapshot(tempDir, "manual", "dummy")
	}
	purged, err := svc.PurgeFlightSnapshots(tempDir, 3)
	if err != nil {
		t.Fatalf("PurgeFlightSnapshots failed: %v", err)
	}
	if purged != 3 {
		t.Fatalf("expected 3 purged snapshots, got %d", purged)
	}
	remainingList, _ := svc.ListFlightSnapshots(tempDir)
	if len(remainingList) != 3 {
		t.Fatalf("expected 3 remaining snapshots, got %d", len(remainingList))
	}
}
