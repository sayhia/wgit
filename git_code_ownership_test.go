package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodeOwnershipAndTimeline(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	_, _ = ExecGit(repo, "config", "user.name", "Alice Dev")
	_, _ = ExecGit(repo, "config", "user.email", "alice@example.com")

	targetFile := filepath.Join(repo, "engine.ts")
	_ = os.WriteFile(targetFile, []byte("console.log('init');\n"), 0644)
	_, _ = ExecGit(repo, "add", "engine.ts")
	_, _ = ExecGit(repo, "commit", "-m", "feat: initial engine")

	// Commit 2: Alice adds more lines
	_ = os.WriteFile(targetFile, []byte("console.log('init');\nconsole.log('step 1');\nconsole.log('step 2');\n"), 0644)
	_, _ = ExecGit(repo, "add", "engine.ts")
	_, _ = ExecGit(repo, "commit", "-m", "feat: expand engine")

	// Commit 3: Bob edits
	_, _ = ExecGit(repo, "config", "user.name", "Bob Tester")
	_, _ = ExecGit(repo, "config", "user.email", "bob@example.com")
	_ = os.WriteFile(targetFile, []byte("console.log('init');\nconsole.log('step 1');\nconsole.log('step 2');\nconsole.log('bob');\n"), 0644)
	_, _ = ExecGit(repo, "add", "engine.ts")
	_, _ = ExecGit(repo, "commit", "-m", "fix: bob tweak")

	// 1. Test GetCodeOwnership
	rep, err := svc.GetCodeOwnership(repo, "engine.ts")
	if err != nil {
		t.Fatalf("GetCodeOwnership failed: %v", err)
	}
	if len(rep.Owners) < 2 {
		t.Errorf("expected at least 2 owners, got %d", len(rep.Owners))
	}
	if rep.TotalCommits != 3 {
		t.Errorf("expected 3 commits, got %d", rep.TotalCommits)
	}
	if rep.BusFactor < 1 {
		t.Errorf("expected busFactor >= 1, got %d", rep.BusFactor)
	}

	// 2. Test GetFileCommitTimeline
	timeline, err := svc.GetFileCommitTimeline(repo, "engine.ts", 10)
	if err != nil {
		t.Fatalf("GetFileCommitTimeline failed: %v", err)
	}
	if len(timeline) != 3 {
		t.Fatalf("expected 3 timeline entries, got %d", len(timeline))
	}
	// Chronological order: first entry should be initial commit
	if timeline[0].Subject != "feat: initial engine" {
		t.Errorf("expected oldest commit first, got %s", timeline[0].Subject)
	}
	if timeline[2].Subject != "fix: bob tweak" {
		t.Errorf("expected newest commit last, got %s", timeline[2].Subject)
	}
}
