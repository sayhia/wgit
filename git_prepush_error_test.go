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

func TestAuditPrePushChecklistReportsReadFailures(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, dir, "src/a.txt", "hello\n", "feat: base [ISSUE-1]", time.Now(), "Alice")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}

	for _, command := range []string{"log", "diff-tree", "ls-tree", "status"} {
		t.Run(command, func(t *testing.T) {
			binDir := t.TempDir()
			script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = %q ]; then
  echo "simulated %s failure" >&2
  exit 128
fi
exec %q "$@"
`, command, command, realGit)
			if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			_, err := (GitService{}).AuditPrePushChecklist(dir, "main")
			if err == nil {
				t.Fatalf("%s failure must return an error instead of a passing gate", command)
			}
			if !strings.Contains(err.Error(), "失败") {
				t.Fatalf("unexpected error for %s: %v", command, err)
			}
		})
	}
}
