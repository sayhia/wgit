package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Regression for "how do I delete one snapshot": the purge path only prunes
// by retention count, so a single unwanted snapshot had no deletion path.
// Deleting must drop both the manifest entry and the pinning ref.
func TestDeleteFlightSnapshotRemovesOneEntry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-flight-delete-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, out)
		}
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")
	os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("hello\n"), 0644)
	runGit("add", "a.txt")
	runGit("commit", "-m", "initial")

	svc := GitService{}
	first, err := svc.CreateFlightRecorderSnapshot(tempDir, "manual", "first")
	if err != nil {
		t.Fatalf("CreateFlightRecorderSnapshot failed: %v", err)
	}
	second, err := svc.CreateFlightRecorderSnapshot(tempDir, "manual", "second")
	if err != nil {
		t.Fatalf("CreateFlightRecorderSnapshot failed: %v", err)
	}

	if err := svc.DeleteFlightSnapshot(tempDir, first.ID); err != nil {
		t.Fatalf("DeleteFlightSnapshot failed: %v", err)
	}

	remaining, err := svc.ListFlightSnapshots(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].ID != second.ID {
		t.Fatalf("expected only the second snapshot to remain, got %+v", remaining)
	}

	// The pinning ref must be gone too; a leftover ref only pins objects, but
	// the delete contract says the snapshot disappears.
	refCmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/wgit/snapshots/")
	refCmd.Dir = tempDir
	refOut, _ := refCmd.Output()
	refs := strings.Split(strings.TrimSpace(string(refOut)), "\n")
	if len(refs) != 1 || refs[0] == "" {
		t.Fatalf("expected exactly one snapshot ref after delete, got %q", string(refOut))
	}
}
