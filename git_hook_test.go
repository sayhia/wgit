package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHooksInstalledAndToggle(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	byName := func() map[string]GitHookInfo {
		t.Helper()
		hooks, err := svc.GetGitHooks(dir)
		if err != nil {
			t.Fatalf("GetGitHooks: %v", err)
		}
		out := map[string]GitHookInfo{}
		for _, h := range hooks {
			out[h.Name] = h
		}
		return out
	}

	if got := byName()["pre-commit"]; !got.Installed || !got.Enabled {
		t.Fatalf("pre-commit = %+v, want installed+enabled", got)
	}
	if got := byName()["commit-msg"]; got.Installed {
		t.Fatalf("commit-msg should not be installed: %+v", got)
	}

	if err := svc.ToggleGitHook(dir, "pre-commit", false, ""); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if got := byName()["pre-commit"]; !got.Installed || got.Enabled {
		t.Fatalf("after disable = %+v, want installed+disabled", got)
	}
	if _, err := os.Stat(hookPath + ".disabled"); err != nil {
		t.Errorf("expected .disabled file: %v", err)
	}

	if err := svc.ToggleGitHook(dir, "pre-commit", true, ""); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if got := byName()["pre-commit"]; !got.Installed || !got.Enabled {
		t.Fatalf("after enable = %+v, want installed+enabled", got)
	}
}

func TestToggleGitHookReportsRenameFailure(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	hooksDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	disabled := filepath.Join(hooksDir, "pre-commit.disabled")
	if err := os.WriteFile(disabled, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A non-empty directory at the active path makes the rename fail.
	if err := os.MkdirAll(filepath.Join(hooksDir, "pre-commit", "block"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := (GitService{}).ToggleGitHook(dir, "pre-commit", true, "")
	if err == nil || !strings.Contains(err.Error(), "启用钩子失败") {
		t.Fatalf("rename failure must be reported, got %v", err)
	}
	if _, statErr := os.Stat(disabled); statErr != nil {
		t.Fatalf("failed enable must keep the disabled hook: %v", statErr)
	}
}
