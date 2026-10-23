package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetRebaseProgressInLinkedWorktree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	root := t.TempDir()
	repo := filepath.Join(root, "main")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run(repo, "init", "-q", "-b", "main")
	run(repo, "config", "user.name", "Tester")
	run(repo, "config", "user.email", "tester@example.com")
	write(filepath.Join(repo, "shared.txt"), "base\n")
	run(repo, "add", "shared.txt")
	run(repo, "commit", "-qm", "base")

	run(repo, "checkout", "-q", "-b", "feature")
	write(filepath.Join(repo, "shared.txt"), "feature\n")
	run(repo, "commit", "-qam", "feature change")
	run(repo, "checkout", "-q", "main")
	write(filepath.Join(repo, "shared.txt"), "main\n")
	run(repo, "commit", "-qam", "main change")

	wt := filepath.Join(root, "wt")
	run(repo, "worktree", "add", "-q", wt, "feature")
	if out, err := ExecGit(wt, "rebase", "main"); err == nil {
		t.Fatalf("expected rebase conflict, got success: %s", out)
	}

	svc := GitService{}
	progress, err := svc.GetRebaseProgress(wt)
	if err != nil {
		t.Fatalf("GetRebaseProgress failed: %v", err)
	}
	if !progress.IsRebasing || progress.HeadName != "feature" || !progress.HasConflicts {
		t.Fatalf("unexpected rebase progress: %+v", progress)
	}
	if len(progress.ConflictedFiles) == 0 {
		t.Fatalf("expected conflicted files: %+v", progress)
	}
	if progress.CurrentStep != 1 || progress.TotalSteps != 1 {
		t.Fatalf("unexpected rebase step counters: %+v", progress)
	}

	if _, err := svc.AbortConflictOperation(wt); err != nil {
		t.Fatalf("AbortConflictOperation failed: %v", err)
	}
	progress, err = svc.GetRebaseProgress(wt)
	if err != nil {
		t.Fatalf("GetRebaseProgress after abort failed: %v", err)
	}
	if progress.IsRebasing || progress.HasConflicts {
		t.Fatalf("rebase should be cleared after abort: %+v", progress)
	}
}
