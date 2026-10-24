package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetLfsLockActivityWarnsWhenStatusFails(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "lfs" ] && [ "$2" = "locks" ]; then
  echo '[{"id":"1","path":"Assets/hero.png","owner":{"name":"Bob"},"locked_at":"2026-10-25T00:00:00Z"}]'
  exit 0
fi
if [ "$1" = "status" ]; then
  echo "simulated status failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	res, err := (GitService{}).GetLfsLockActivity(repo)
	if err != nil || res == nil {
		t.Fatalf("GetLfsLockActivity failed: %+v err=%v", res, err)
	}
	if res.TotalLocks != 1 {
		t.Fatalf("lock list should be preserved, got %+v", res)
	}
	if !strings.Contains(res.Message, "未执行锁冲突检查") {
		t.Fatalf("status read failure must be disclosed, got: %q", res.Message)
	}
}
