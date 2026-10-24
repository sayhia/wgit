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

func TestVerifyPrePushLfsReportsDiffTreeFailure(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "art/big.bin", "binary payload", "feat: asset", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "diff-tree" ]; then
  echo "simulated diff-tree failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = (GitService{}).VerifyPrePushLfs(repo, "", "")
	if err == nil || !strings.Contains(err.Error(), "读取待推送") {
		t.Fatalf("diff-tree failure must return an explicit error instead of a safe verdict, got: %v", err)
	}
}
