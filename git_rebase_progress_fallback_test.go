package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetRebaseProgressKeepsStoppedShaWhenLogFails(t *testing.T) {
	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}

	stateDir := filepath.Join(repo, ".git", "rebase-merge")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	missingSha := "0123456789abcdef0123456789abcdef01234567"
	for name, content := range map[string]string{
		"head-name":   "refs/heads/feature\n",
		"onto":        "1111111111111111111111111111111111111111\n",
		"msgnum":      "1\n",
		"end":         "3\n",
		"stopped-sha": missingSha + "\n",
	} {
		if err := os.WriteFile(filepath.Join(stateDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	progress, err := (GitService{}).GetRebaseProgress(repo)
	if err != nil || progress == nil || !progress.IsRebasing {
		t.Fatalf("GetRebaseProgress failed: %+v err=%v", progress, err)
	}
	if progress.CurrentCommit != missingSha[:7] {
		t.Fatalf("log failure should fall back to the stopped short sha, got %q", progress.CurrentCommit)
	}
}
