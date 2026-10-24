package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGetReleasePlanBlocksWhenStatusUnreadable(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "a.txt", "a\n", "feat: release me", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
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

	plan, err := (GitService{}).GetReleasePlan(repo)
	if err != nil || plan == nil {
		t.Fatalf("GetReleasePlan failed: %+v err=%v", plan, err)
	}
	var workspace *ReleaseCheck
	for i := range plan.Checks {
		if plan.Checks[i].Key == "workspace" {
			workspace = &plan.Checks[i]
			break
		}
	}
	if workspace == nil {
		t.Fatalf("workspace check missing: %+v", plan.Checks)
	}
	if workspace.Status != "fail" || !workspace.Blocking {
		t.Fatalf("unreadable status must block the release, got: %+v", workspace)
	}
}
