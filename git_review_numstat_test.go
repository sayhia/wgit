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

func TestChangeReviewReportsDiffStatFailure(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: base", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	runGitTest(t, dir, nil, "checkout", "-b", "feature/numstat")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle { /* changed */ }\n", "feat: change", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "--numstat" ]; then
    echo "simulated numstat failure" >&2
    exit 128
  fi
done
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := (GitService{}).GetChangeReview(dir, base, "feature/numstat"); err == nil {
		t.Fatal("numstat failure must return an error instead of zero line counts")
	}
}
