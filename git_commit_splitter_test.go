package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitSplitterInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-split-*")
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

	fBase := filepath.Join(tempDir, "base.txt")
	os.WriteFile(fBase, []byte("base content\n"), 0644)
	runGit("add", "base.txt")
	runGit("commit", "-m", "initial base commit")

	// Create a commit with 2 different files (e.g. bugfix and refactor)
	fBug := filepath.Join(tempDir, "bugfix.go")
	os.WriteFile(fBug, []byte("package bugfix\n"), 0644)
	fRefactor := filepath.Join(tempDir, "refactor.go")
	os.WriteFile(fRefactor, []byte("package refactor\n"), 0644)

	runGit("add", "bugfix.go", "refactor.go")
	runGit("commit", "-m", "feat: combined bugfix and refactor")

	svc := GitService{}

	plans := []SplitCommitPlan{
		{
			Message: "fix: isolated bugfix logic",
			Files:   []string{"bugfix.go"},
		},
		{
			Message: "refactor: isolated cleanups",
			Files:   []string{"refactor.go"},
		},
	}

	res, err := svc.DeconstructCommit(tempDir, "HEAD", plans)
	if err != nil {
		t.Fatalf("DeconstructCommit failed: %v", err)
	}

	if !res.Success || res.CreatedCount != 2 {
		t.Fatalf("Expected 2 split commits, got %+v", res)
	}
}
