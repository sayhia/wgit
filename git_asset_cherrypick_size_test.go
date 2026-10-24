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

func TestCherryPickAssetBundleReportsSizeReadFailure(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "Assets/Hero.mat", "material v1\n", "chore: base", time.Now(), "Alice")

	runGitTest(t, repo, nil, "checkout", "-b", "art")
	writeAndCommit(t, repo, "Assets/Hero.mat", "material v2\n", "feat: art", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "cat-file" ]; then
  echo "simulated cat-file failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = (GitService{}).CherryPickAssetBundle(repo, "art", []string{"Assets/Hero.mat"}, false, true)
	if err == nil || !strings.Contains(err.Error(), "读取资产") {
		t.Fatalf("size read failure must return an explicit error, got: %v", err)
	}
}
