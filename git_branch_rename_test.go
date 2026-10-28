package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenameBranchInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-branch-rename-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
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
	}

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	f := filepath.Join(tempDir, "sample.txt")
	_ = os.WriteFile(f, []byte("hello\n"), 0644)
	runGit("add", "sample.txt")
	runGit("commit", "-m", "init")
	runGit("branch", "-M", "feature/old-name")

	service := &GitService{}
	res, err := service.RenameBranchWithUpstream(tempDir, "feature/old-name", "feature/new-name", false)
	if err != nil {
		t.Fatalf("RenameBranchWithUpstream failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected Success = true")
	}
	if res.NewName != "feature/new-name" {
		t.Errorf("expected NewName = 'feature/new-name', got %s", res.NewName)
	}

	cmdCur := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdCur.Dir = tempDir
	out, err := cmdCur.Output()
	if err != nil || string(out) != "feature/new-name\n" {
		t.Errorf("expected active branch to be 'feature/new-name', got %s", string(out))
	}
}
