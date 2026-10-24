package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInspectLfsFileHistoryReportsLogFailure(t *testing.T) {
	if _, err := (GitService{}).InspectLfsFileHistory(t.TempDir(), "Assets/hero.png"); err == nil {
		t.Fatal("non-git directory must return an error instead of an empty history")
	}

	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "Assets/hero.png", "png\n", "art: hero", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "log" ]; then
  echo "simulated log failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = (GitService{}).InspectLfsFileHistory(repo, "Assets/hero.png")
	if err == nil || !strings.Contains(err.Error(), "读取 LFS 文件历史失败") {
		t.Fatalf("log failure must return an explicit error, got: %v", err)
	}
}
