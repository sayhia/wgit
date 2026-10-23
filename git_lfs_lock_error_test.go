package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetLfsLocksPropagatesQueryFailure(t *testing.T) {
	binDir := t.TempDir()
	fakeLfs := filepath.Join(binDir, "git-lfs")
	if err := os.WriteFile(fakeLfs, []byte("#!/bin/sh\necho 'fatal: could not read from remote repository' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if _, err := (GitService{}).GetLfsLocks(repo); err == nil || !strings.Contains(err.Error(), "读取 Git LFS 锁失败") {
		t.Fatalf("lock query failure must be surfaced: %v", err)
	}
}

func TestIsGitLfsUnavailable(t *testing.T) {
	if !isGitLfsUnavailable(errors.New("git: 'lfs' is not a git command. See 'git --help'.")) {
		t.Fatal("missing git-lfs should be treated as a degraded mode")
	}
	if isGitLfsUnavailable(errors.New("fatal: unable to access remote: 403 Forbidden")) {
		t.Fatal("network/auth failures must not be treated as missing git-lfs")
	}
}
