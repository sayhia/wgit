package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClearGitProxyPropagatesFailureAndClearsBothScopes(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	const proxy = "http://127.0.0.1:7890"

	if err := svc.SetGitProxy(repo, proxy, false); err != nil {
		t.Fatalf("SetGitProxy(local) failed: %v", err)
	}
	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if err := svc.ClearGitProxy(repo, false); err == nil {
		t.Fatal("failed unset must not be reported as cleared")
	}
	if err := os.Chmod(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := svc.GetGitProxy(repo)
	if err != nil || cfg.HttpProxy != proxy || cfg.HttpsProxy != proxy {
		t.Fatalf("proxy should remain after failed clear: %+v err=%v", cfg, err)
	}
	if err := svc.ClearGitProxy(repo, false); err != nil {
		t.Fatalf("ClearGitProxy(local) failed: %v", err)
	}
	cfg, _ = svc.GetGitProxy(repo)
	if cfg.HttpProxy != "" || cfg.HttpsProxy != "" {
		t.Fatalf("local proxy was not cleared: %+v", cfg)
	}

	if err := svc.SetGitProxy(repo, proxy, true); err != nil {
		t.Fatalf("SetGitProxy(global) failed: %v", err)
	}
	if err := svc.ClearGitProxy(repo, true); err != nil {
		t.Fatalf("ClearGitProxy(global) failed: %v", err)
	}
	out, _ := ExecGit(repo, "config", "--global", "--get-all", "http.proxy")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("global proxy was not cleared: %q", out)
	}
}
