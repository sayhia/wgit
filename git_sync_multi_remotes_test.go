package main

import (
	"strings"
	"testing"
)

func TestSyncMultiRemotesPushAllRejectsDetachedHead(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "commit", "-q", "--allow-empty", "-m", "base")
	runGitTest(t, repo, nil, "remote", "add", "origin", "https://example.invalid/repo.git")
	runGitTest(t, repo, nil, "checkout", "-q", "--detach")

	_, err := (GitService{}).SyncMultiRemotes(repo, "push-all")
	if err == nil || !strings.Contains(err.Error(), "分离 HEAD") {
		t.Fatalf("push-all in detached HEAD must fail explicitly, got: %v", err)
	}
}
