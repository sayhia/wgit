package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkedWorktreeConflictStateAndHooks(t *testing.T) {
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
	writeFile := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}

	run(repo, "init", "-q", "-b", "main")
	run(repo, "config", "user.email", "tester@example.com")
	run(repo, "config", "user.name", "Tester")
	writeFile(filepath.Join(repo, "shared.txt"), "base\n", 0o644)
	run(repo, "add", "shared.txt")
	run(repo, "commit", "-qm", "base")

	run(repo, "checkout", "-q", "-b", "feature")
	writeFile(filepath.Join(repo, "shared.txt"), "feature\n", 0o644)
	run(repo, "commit", "-qam", "feature change")
	run(repo, "checkout", "-q", "main")
	writeFile(filepath.Join(repo, "shared.txt"), "main\n", 0o644)
	run(repo, "commit", "-qam", "main change")

	wt := filepath.Join(root, "wt")
	run(repo, "worktree", "add", "-q", wt, "feature")
	if out, err := ExecGit(wt, "merge", "main"); err == nil {
		t.Fatalf("expected merge conflict, got success: %s", out)
	}

	svc := GitService{}
	status, err := svc.GetRepoStatus(wt)
	if err != nil {
		t.Fatalf("GetRepoStatus failed: %v", err)
	}
	if !status.IsMerging || len(status.ConflictedFiles) == 0 {
		t.Fatalf("worktree merge state not detected: %+v", status)
	}

	if out, err := svc.AbortConflictOperation(wt); err != nil {
		t.Fatalf("AbortConflictOperation failed in worktree: %v (%s)", err, out)
	}
	status, err = svc.GetRepoStatus(wt)
	if err != nil {
		t.Fatalf("GetRepoStatus after abort failed: %v", err)
	}
	if status.IsMerging || len(status.ConflictedFiles) != 0 {
		t.Fatalf("merge state should be cleared: %+v", status)
	}

	hookPath := filepath.Join(repo, ".git", "hooks", "pre-commit")
	writeFile(hookPath, "#!/bin/sh\nexit 0\n", 0o755)
	hooks, err := svc.GetGitHooks(wt)
	if err != nil {
		t.Fatalf("GetGitHooks failed: %v", err)
	}
	var found bool
	for _, hook := range hooks {
		if hook.Name == "pre-commit" {
			found = hook.Installed && hook.Enabled
		}
	}
	if !found {
		t.Fatalf("common-dir pre-commit hook not reported for worktree: %+v", hooks)
	}

	if err := svc.ToggleGitHook(wt, "pre-push", true, "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatalf("ToggleGitHook failed in worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "pre-push")); err != nil {
		t.Fatalf("pre-push hook should be written to the common hooks dir: %v", err)
	}
}
