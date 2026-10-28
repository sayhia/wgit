package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestApplyStashFileInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-stash-partial-*")
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

	fA := filepath.Join(tempDir, "fileA.txt")
	fB := filepath.Join(tempDir, "fileB.txt")
	_ = os.WriteFile(fA, []byte("original A\n"), 0644)
	_ = os.WriteFile(fB, []byte("original B\n"), 0644)
	runGit("add", ".")
	runGit("commit", "-m", "init")

	// Change both files and stash
	_ = os.WriteFile(fA, []byte("stashed A\n"), 0644)
	_ = os.WriteFile(fB, []byte("stashed B\n"), 0644)
	runGit("stash", "push", "-m", "stash both")

	// Verify working tree is clean
	contentA, _ := os.ReadFile(fA)
	if string(contentA) != "original A\n" {
		t.Fatalf("expected original A after stash")
	}

	service := &GitService{}
	res, err := service.ApplyStashFile(tempDir, 0, "fileA.txt")
	if err != nil {
		t.Fatalf("ApplyStashFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success = true")
	}

	// Verify fileA was restored from stash, but fileB remains clean
	newA, _ := os.ReadFile(fA)
	if string(newA) != "stashed A\n" {
		t.Errorf("expected 'stashed A', got: %s", string(newA))
	}
	newB, _ := os.ReadFile(fB)
	if string(newB) != "original B\n" {
		t.Errorf("expected 'original B', got: %s", string(newB))
	}
}
