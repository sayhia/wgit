package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateReleaseChangelog(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	// Configure git user
	_, _ = ExecGit(repo, "config", "user.name", "WGit Test")
	_, _ = ExecGit(repo, "config", "user.email", "test@wgit.internal")

	// Commit 1: feat
	f1 := filepath.Join(repo, "f1.txt")
	_ = os.WriteFile(f1, []byte("feature 1"), 0644)
	_, _ = ExecGit(repo, "add", ".")
	_, _ = ExecGit(repo, "commit", "-m", "feat(auth): add OAuth2 provider support")

	// Tag v1.0.0
	_, _ = ExecGit(repo, "tag", "v1.0.0")

	// Commit 2: fix
	f2 := filepath.Join(repo, "f2.txt")
	_ = os.WriteFile(f2, []byte("fix 1"), 0644)
	_, _ = ExecGit(repo, "add", ".")
	_, _ = ExecGit(repo, "commit", "-m", "fix(core): prevent race condition on shutdown")

	// Commit 3: breaking feat!
	f3 := filepath.Join(repo, "f3.txt")
	_ = os.WriteFile(f3, []byte("breaking 1"), 0644)
	_, _ = ExecGit(repo, "add", ".")
	_, _ = ExecGit(repo, "commit", "-m", "feat(api)!: migrate payload format to v2")

	// Generate changelog from v1.0.0 to HEAD
	res, err := svc.GenerateReleaseChangelog(repo, "v1.0.0", "HEAD")
	if err != nil {
		t.Fatalf("GenerateReleaseChangelog failed: %v", err)
	}

	if res.TotalCommits != 2 {
		t.Errorf("expected 2 commits in range, got %d", res.TotalCommits)
	}

	if len(res.BreakingChanges) != 1 {
		t.Errorf("expected 1 breaking change, got %d", len(res.BreakingChanges))
	}

	if len(res.Categories["fix"]) != 1 {
		t.Errorf("expected 1 fix, got %d", len(res.Categories["fix"]))
	}

	if !strings.Contains(res.Markdown, "Breaking Changes") {
		t.Errorf("expected Markdown to contain Breaking Changes header")
	}

	if !strings.Contains(res.Markdown, "migrate payload format to v2") {
		t.Errorf("expected Markdown to contain commit subject")
	}

	if len(res.Contributors) == 0 || res.Contributors[0] != "WGit Test" {
		t.Errorf("expected contributor 'WGit Test', got %+v", res.Contributors)
	}
}
