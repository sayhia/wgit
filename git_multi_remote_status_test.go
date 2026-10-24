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

func TestGetMultiRemoteStatusReportsAheadBehindFailure(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: init", time.Now(), "Alice")
	runGitTest(t, repo, nil, "remote", "add", "origin", "https://example.invalid/repo.git")
	runGitTest(t, repo, nil, "update-ref", "refs/remotes/origin/main", "HEAD")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "rev-list" ]; then
  echo "simulated rev-list failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	res, err := (GitService{}).GetMultiRemoteStatus(repo)
	if err != nil || res == nil || len(res.Remotes) != 1 {
		t.Fatalf("GetMultiRemoteStatus failed: %+v err=%v", res, err)
	}
	if !strings.Contains(res.Remotes[0].StatusWarning, "读取领先/落后失败") {
		t.Fatalf("ahead/behind read failure must be surfaced, got: %+v", res.Remotes[0])
	}
}
