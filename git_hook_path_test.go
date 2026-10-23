package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitHooksRespectCustomHooksPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "core.hooksPath", ".husky")
	if err := os.MkdirAll(filepath.Join(repo, ".husky"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".husky", "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	hooks, err := svc.GetGitHooks(repo)
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
		t.Fatalf("custom hooksPath pre-commit hook not reported: %+v", hooks)
	}

	if err := svc.ToggleGitHook(repo, "pre-push", true, "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatalf("ToggleGitHook failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".husky", "pre-push")); err != nil {
		t.Fatalf("pre-push should be written under core.hooksPath: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".git", "hooks", "pre-push")); err == nil {
		t.Fatal("pre-push should not be written to the default hooks dir")
	}
}
