package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSquashSelectedCommitsInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-squash-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoPath := filepath.Join(tempDir, "repo")
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
	runGit(repoPath, "config", "user.name", "SquashTester")
	runGit(repoPath, "config", "user.email", "squash@test.com")

	// Commit 1: base
	_ = os.WriteFile(filepath.Join(repoPath, "f0.txt"), []byte("base\n"), 0o644)
	runGit(repoPath, "add", "f0.txt")
	runGit(repoPath, "commit", "-m", "feat: base commit")

	// Commit 2
	_ = os.WriteFile(filepath.Join(repoPath, "f1.txt"), []byte("step 1\n"), 0o644)
	runGit(repoPath, "add", "f1.txt")
	runGit(repoPath, "commit", "-m", "wip: step 1")
	c2Sha, _ := ExecGit(repoPath, "rev-parse", "HEAD")

	// Commit 3
	_ = os.WriteFile(filepath.Join(repoPath, "f2.txt"), []byte("step 2\n"), 0o644)
	runGit(repoPath, "add", "f2.txt")
	runGit(repoPath, "commit", "-m", "wip: step 2")
	c3Sha, _ := ExecGit(repoPath, "rev-parse", "HEAD")

	svc := GitService{}

	// Test validation: fewer than 2 commits
	if _, err := svc.SquashSelectedCommits(repoPath, []string{strings.TrimSpace(c3Sha)}, "msg"); err == nil {
		t.Fatalf("expected error for fewer than 2 commits")
	}

	// Test validation: empty message
	if _, err := svc.SquashSelectedCommits(repoPath, []string{strings.TrimSpace(c3Sha), strings.TrimSpace(c2Sha)}, "   "); err == nil {
		t.Fatalf("expected error for empty message")
	}

	// Test successful squash of top 2 commits
	res, err := svc.SquashSelectedCommits(repoPath, []string{strings.TrimSpace(c3Sha), strings.TrimSpace(c2Sha)}, "feat: squashed step 1 and 2")
	if err != nil {
		t.Fatalf("SquashSelectedCommits failed: %v", err)
	}
	if !res.Success || res.Count != 2 || res.NewHash == "" {
		t.Fatalf("unexpected squash result: %+v", res)
	}

	// Verify commit log: should have exactly 2 commits now (base + squashed)
	logOut, _ := ExecGit(repoPath, "log", "--oneline")
	lines := strings.Split(strings.TrimSpace(logOut), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 commits in log, got %d:\n%s", len(lines), logOut)
	}
	if !strings.Contains(lines[0], "feat: squashed step 1 and 2") {
		t.Errorf("expected top commit to contain squash message, got: %s", lines[0])
	}
}
