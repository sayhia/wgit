package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStashSandboxInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-stash-*")
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

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	f1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(f1, []byte("line1\nline2\n"), 0644)
	runGit("add", "file1.txt")
	runGit("commit", "-m", "initial commit")

	// Create stash 0
	os.WriteFile(f1, []byte("line1\nline2-stash0\n"), 0644)
	runGit("stash", "push", "-m", "stash zero")

	// Create stash 1
	os.WriteFile(f1, []byte("line1\nline2-stash1\n"), 0644)
	runGit("stash", "push", "-m", "stash one")

	svc := GitService{}

	// Test CompareStashes
	report, err := svc.CompareStashes(tempDir, "stash@{0}", "stash@{1}")
	if err != nil {
		t.Fatalf("CompareStashes failed: %v", err)
	}
	if report == nil || report.TotalFiles == 0 {
		t.Fatalf("Expected report files > 0, got %+v", report)
	}

	// Test ApplyStashHunk (file restore from stash)
	ok, err := svc.ApplyStashHunk(tempDir, 0, "file1.txt", 0)
	if err != nil || !ok {
		t.Fatalf("ApplyStashHunk failed: %v", err)
	}

	// Test CreateMicroSnapshot
	snap, err := svc.CreateMicroSnapshot(tempDir, []string{})
	if err != nil || snap == nil {
		t.Fatalf("CreateMicroSnapshot failed: %v", err)
	}
	if snap.SnapshotRef == "" {
		t.Fatalf("Expected snapshot ref, got empty")
	}
}
