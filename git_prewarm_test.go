package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrewarmRepoCache(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "test@test.com")

	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Prewarm Test"), 0644)
	_, _ = ExecGit(repo, "add", "README.md")
	_, _ = ExecGit(repo, "commit", "-m", "chore: initial commit")

	res, err := svc.PrewarmRepoCache(repo)
	if err != nil {
		t.Fatalf("PrewarmRepoCache failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success")
	}
	if res.BranchCount < 1 {
		t.Errorf("expected at least 1 branch, got %d", res.BranchCount)
	}
	if res.CurrentBranch == "" {
		t.Errorf("expected non-empty current branch")
	}
	if !res.IsClean {
		t.Errorf("expected repo to be clean")
	}
	if len(res.PrewarmedEntities) < 4 {
		t.Errorf("expected at least 4 entities, got %d", len(res.PrewarmedEntities))
	}
}
