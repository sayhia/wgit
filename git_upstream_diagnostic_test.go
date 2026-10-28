package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetGoneLocalBranchesInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-upstream-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	remoteRepo := filepath.Join(tempDir, "remote.git")
	localRepo := filepath.Join(tempDir, "local")

	// 1. Setup bare remote
	runCmd := func(dir string, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd %s %v failed: %v, out: %s", name, args, err, string(out))
		}
	}

	if err := os.MkdirAll(remoteRepo, 0755); err != nil {
		t.Fatal(err)
	}
	runCmd(remoteRepo, "git", "init", "--bare")

	// 2. Clone local
	runCmd(tempDir, "git", "clone", remoteRepo, localRepo)
	runCmd(localRepo, "git", "config", "user.name", "TestUser")
	runCmd(localRepo, "git", "config", "user.email", "test@example.com")

	// Initial commit on main
	runCmd(localRepo, "git", "checkout", "-b", "main")
	testFile := filepath.Join(localRepo, "file.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0644)
	runCmd(localRepo, "git", "add", "file.txt")
	runCmd(localRepo, "git", "commit", "-m", "init commit")
	runCmd(localRepo, "git", "push", "-u", "origin", "main")

	// Create and push feature branch
	runCmd(localRepo, "git", "checkout", "-b", "feature/to-be-deleted")
	_ = os.WriteFile(testFile, []byte("feature update"), 0644)
	runCmd(localRepo, "git", "commit", "-am", "feature commit")
	runCmd(localRepo, "git", "push", "-u", "origin", "feature/to-be-deleted")

	// Switch back to main
	runCmd(localRepo, "git", "checkout", "main")

	svc := GitService{}

	// Initially, no branches are gone
	gone, err := svc.GetGoneLocalBranches(localRepo)
	if err != nil {
		t.Fatalf("GetGoneLocalBranches failed: %v", err)
	}
	if len(gone) != 0 {
		t.Fatalf("expected 0 gone branches, got %d", len(gone))
	}

	// Delete branch on remote
	runCmd(localRepo, "git", "push", "origin", "--delete", "feature/to-be-deleted")
	// Prune remote tracking branches in local
	runCmd(localRepo, "git", "fetch", "--prune")

	// Now feature/to-be-deleted should be detected as gone!
	gone, err = svc.GetGoneLocalBranches(localRepo)
	if err != nil {
		t.Fatalf("GetGoneLocalBranches after prune failed: %v", err)
	}
	if len(gone) != 1 {
		t.Fatalf("expected 1 gone branch, got %d", len(gone))
	}
	if gone[0].Name != "feature/to-be-deleted" {
		t.Fatalf("expected branch feature/to-be-deleted, got %s", gone[0].Name)
	}
	if gone[0].LastCommitSubject != "feature commit" {
		t.Fatalf("expected commit subject 'feature commit', got %s", gone[0].LastCommitSubject)
	}
}
