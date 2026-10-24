package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGetCommitDetailReportsReadFailures(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "a.txt", "hello\n", "chore: init", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	for _, failArg := range []string{"--numstat", "--patch"} {
		t.Run(failArg, func(t *testing.T) {
			binDir := t.TempDir()
			script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = %q ]; then
    echo "simulated show failure" >&2
    exit 128
  fi
done
exec %q "$@"
`, failArg, realGit)
			if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			if _, err := (GitService{}).GetCommitDetail(repo, "HEAD"); err == nil {
				t.Fatalf("%s read failure must return an error instead of an empty detail", failArg)
			}
		})
	}
}

func TestGetRepoOverviewRejectsNonRepo(t *testing.T) {
	if _, err := (GitService{}).GetRepoOverview(t.TempDir()); err == nil {
		t.Fatal("non-git directory must return an error instead of zero stats")
	}
}
