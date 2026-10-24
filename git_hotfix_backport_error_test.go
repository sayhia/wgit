package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTraceHotfixBackportStatusReportsReadFailures(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Tester")
	runGitTest(t, repo, nil, "config", "user.email", "tester@example.com")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "base")
	runGitTest(t, repo, nil, "checkout", "-q", "-b", "hotfix/v1.0.1")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "fix: hotfix A")
	runGitTest(t, repo, nil, "checkout", "-q", "main")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "feat: main work")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	for _, failCmd := range []string{"cherry", "log"} {
		t.Run(failCmd, func(t *testing.T) {
			binDir := t.TempDir()
			script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = %q ]; then
  echo "simulated %s failure" >&2
  exit 128
fi
exec %q "$@"
`, failCmd, failCmd, realGit)
			if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			_, err := (GitService{}).TraceHotfixBackportStatus(repo, "hotfix/v1.0.1", "main")
			if err == nil {
				t.Fatalf("%s failure must return an error instead of a clean backport verdict", failCmd)
			}
		})
	}
}
