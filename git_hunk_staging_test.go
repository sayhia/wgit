package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageAndDiscardPatchHunkInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-hunk-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
		return string(out)
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	testFile := filepath.Join(tempDir, "code.txt")
	_ = os.WriteFile(testFile, []byte("line 1\nline 2\nline 3\n"), 0644)
	runCmd("add", ".")
	runCmd("commit", "-m", "initial commit")

	// Modify line 2
	_ = os.WriteFile(testFile, []byte("line 1\nline 2 modified\nline 3\n"), 0644)
	diffOut := runCmd("diff", "code.txt")

	svc := &GitService{}

	// 1. Stage the hunk
	if err := svc.StagePatchHunk(tempDir, diffOut, false); err != nil {
		t.Fatalf("StagePatchHunk (stage) failed: %v", err)
	}

	statusAfterStage := runCmd("status", "--porcelain")
	if !strings.HasPrefix(statusAfterStage, "M ") {
		t.Fatalf("expected file to be staged (M ), got: %q", statusAfterStage)
	}

	// 2. Unstage the hunk
	if err := svc.StagePatchHunk(tempDir, diffOut, true); err != nil {
		t.Fatalf("StagePatchHunk (unstage) failed: %v", err)
	}

	statusAfterUnstage := runCmd("status", "--porcelain")
	if !strings.HasPrefix(statusAfterUnstage, " M") {
		t.Fatalf("expected file to be unstaged ( M), got: %q", statusAfterUnstage)
	}

	// 3. Discard the hunk from worktree
	if err := svc.DiscardPatchHunk(tempDir, diffOut); err != nil {
		t.Fatalf("DiscardPatchHunk failed: %v", err)
	}

	statusAfterDiscard := runCmd("status", "--porcelain")
	if strings.TrimSpace(statusAfterDiscard) != "" {
		t.Fatalf("expected clean status after discard, got: %q", statusAfterDiscard)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "line 1\nline 2\nline 3\n" {
		t.Fatalf("expected original content restored, got: %s", string(content))
	}
}
