package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageSelectedPatchInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-line-stage-*")
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

	f := filepath.Join(tempDir, "file.txt")
	_ = os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0644)
	runGit("add", "file.txt")
	runGit("commit", "-m", "init")

	// Modify file in worktree
	_ = os.WriteFile(f, []byte("line1\nline2_modified\nline3_new\n"), 0644)

	patch := "--- a/file.txt\n+++ b/file.txt\n@@ -2,1 +2,1 @@\n-line2\n+line2_modified\n"

	service := &GitService{}
	res, err := service.StageSelectedPatch(tempDir, "file.txt", patch, false)
	if err != nil {
		t.Fatalf("StageSelectedPatch failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected Success = true")
	}

	// Verify that staged diff contains line2_modified
	cmdDiff := exec.Command("git", "diff", "--cached")
	cmdDiff.Dir = tempDir
	diffOut, _ := cmdDiff.Output()
	if !strings.Contains(string(diffOut), "+line2_modified") {
		t.Errorf("expected staged diff to contain '+line2_modified', got: %s", string(diffOut))
	}
}
