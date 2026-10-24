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

func TestGetCommitTopologyStatsReportsReadFailures(t *testing.T) {
	if _, err := (GitService{}).GetCommitTopologyStats(t.TempDir()); err == nil {
		t.Fatal("non-git directory must return an error instead of zero stats")
	}

	emptyRepo := t.TempDir()
	runGitTest(t, emptyRepo, nil, "init", "-q", "-b", "main")
	empty, err := (GitService{}).GetCommitTopologyStats(emptyRepo)
	if err != nil || empty == nil || empty.TotalCommits != 0 {
		t.Fatalf("empty repo should return zero stats without error: %+v err=%v", empty, err)
	}

	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "a.txt", "a\n", "chore: init", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--merges" ]; then
    echo "simulated merge count failure" >&2
    exit 128
  fi
done
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = (GitService{}).GetCommitTopologyStats(repo)
	if err == nil || !strings.Contains(err.Error(), "合并提交数") {
		t.Fatalf("merge count failure must return an explicit error, got: %v", err)
	}
}
