package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBlastRadiusInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-radius-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Commit 1 on main
	_ = os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content 1\n"), 0644)
	runGit("add", "file1.txt")
	runGit("commit", "-m", "commit 1")

	// Commit 2 on main
	_ = os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("content 2\n"), 0644)
	runGit("add", "file2.txt")
	runGit("commit", "-m", "commit 2")

	// Add dirty uncommitted change
	_ = os.WriteFile(filepath.Join(tempDir, "file3.txt"), []byte("uncommitted\n"), 0644)

	// 1. Test reset-hard blast radius
	repReset, err := svc.CalculateBlastRadius(tempDir, "reset-hard", "HEAD~1")
	if err != nil {
		t.Fatalf("CalculateBlastRadius reset-hard failed: %v", err)
	}
	if repReset.UncommittedFilesCount != 1 {
		t.Errorf("expected 1 uncommitted file, got %d", repReset.UncommittedFilesCount)
	}
	if repReset.DanglingCommitsCount != 1 {
		t.Errorf("expected 1 dangling commit, got %d", repReset.DanglingCommitsCount)
	}
	if repReset.RiskLevel != "critical" {
		t.Errorf("expected critical risk level, got %s", repReset.RiskLevel)
	}

	// Clean dirty change
	_ = os.Remove(filepath.Join(tempDir, "file3.txt"))

	// Create unmerged feature branch
	runGit("checkout", "-b", "feature-unmerged")
	_ = os.WriteFile(filepath.Join(tempDir, "feat.txt"), []byte("feat\n"), 0644)
	runGit("add", "feat.txt")
	runGit("commit", "-m", "unmerged feat commit")

	runGit("checkout", "main")

	// 2. Test branch-delete blast radius for unmerged branch
	repBranch, err := svc.CalculateBlastRadius(tempDir, "branch-delete", "feature-unmerged")
	if err != nil {
		t.Fatalf("CalculateBlastRadius branch-delete failed: %v", err)
	}
	if repBranch.RiskLevel != "high" {
		t.Errorf("expected high risk level for unmerged branch delete, got %s", repBranch.RiskLevel)
	}
	if repBranch.DanglingCommitsCount != 1 {
		t.Errorf("expected 1 dangling commit, got %d", repBranch.DanglingCommitsCount)
	}
}
