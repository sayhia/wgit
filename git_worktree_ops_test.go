package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeOpsInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-worktree-ops-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "main-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	runGit := func(dir string, args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	runGit(repoPath, "init", "-b", "main")
	runGit(repoPath, "config", "user.name", "Tester")
	runGit(repoPath, "config", "user.email", "tester@test.com")

	testFile := filepath.Join(repoPath, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello worktree\n"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	runGit(repoPath, "add", "hello.txt")
	runGit(repoPath, "commit", "-m", "init commit")

	wtPath := filepath.Join(tempDir, "linked-wt")
	runGit(repoPath, "worktree", "add", "-b", "feature/worker", wtPath)

	svc := GitService{}

	// Test 1: GetWorktreeDiskUsage
	usage, err := svc.GetWorktreeDiskUsage(repoPath, wtPath)
	if err != nil {
		t.Fatalf("GetWorktreeDiskUsage failed: %v", err)
	}
	if usage.SizeBytes <= 0 || usage.FileCount <= 0 {
		t.Errorf("expected positive size and file count, got %+v", usage)
	}
	if usage.FormattedSize == "" {
		t.Errorf("expected formatted size, got empty")
	}

	// Test 2: ValidateWorktreeBranch - branch feature/worker should be unavailable
	val1, err := svc.ValidateWorktreeBranch(repoPath, "feature/worker")
	if err != nil {
		t.Fatalf("ValidateWorktreeBranch failed: %v", err)
	}
	if val1.Available {
		t.Errorf("expected feature/worker to be unavailable, got available")
	}

	// Test 3: ValidateWorktreeBranch - branch feature/new-unopened should be available
	val2, err := svc.ValidateWorktreeBranch(repoPath, "feature/new-unopened")
	if err != nil {
		t.Fatalf("ValidateWorktreeBranch failed: %v", err)
	}
	if !val2.Available {
		t.Errorf("expected feature/new-unopened to be available, got unavailable")
	}
}
