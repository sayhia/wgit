package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunRepoHealthCheckUnverifiedWhenGitDirProbeFails(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, repo, "src/Unverified.cs", "class Unverified {}\n", "feat: base", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$#" -eq 2 ] && [ "$1" = "rev-parse" ] && [ "$2" = "--git-dir" ]; then
  echo "simulated rev-parse --git-dir failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	report, err := (GitService{}).RunRepoHealthCheck(repo)
	if err != nil {
		t.Fatalf("health check must still return a report: %v", err)
	}
	if len(report.Items) != 6 {
		t.Fatalf("expected 6 dimensions, got %d", len(report.Items))
	}

	var workspace *HealthDimension
	for i := range report.Items {
		if report.Items[i].Key == "workspace" {
			workspace = &report.Items[i]
			break
		}
	}
	if workspace == nil {
		t.Fatal("workspace dimension is missing")
	}
	if workspace.DetailKey != "health.workspace.detail.unverified" {
		t.Fatalf("git-dir probe failure must be disclosed, got detailKey %q", workspace.DetailKey)
	}
	if workspace.Status == "pass" {
		t.Fatalf("unverified workspace must not pass, got %+v", *workspace)
	}
	if workspace.Score >= 100 {
		t.Fatalf("unverified workspace score must be reduced, got %d", workspace.Score)
	}
	if workspace.Params["error"] == "" {
		t.Fatalf("unverified detail is missing its error param: %+v", workspace.Params)
	}
	if report.Summary == "" || report.Grade == "" || report.Score < 0 || report.Score > 100 {
		t.Fatalf("report metadata is incomplete: %+v", report)
	}
	t.Logf("workspace: %s (%d) %s", workspace.Status, workspace.Score, workspace.Detail)
}
