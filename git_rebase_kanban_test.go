package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebaseKanbanAndSurgicalSplit(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	_, _ = ExecGit(repo, "config", "user.name", "Rebase Surgeon")
	_, _ = ExecGit(repo, "config", "user.email", "surgeon@example.com")

	// Commit 1 (base): initial
	_ = os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0644)
	_, _ = ExecGit(repo, "add", "base.txt")
	_, _ = ExecGit(repo, "commit", "-m", "chore: initial commit")

	// Branch feature
	_, _ = ExecGit(repo, "checkout", "-b", "feature/surgeon")

	// Commit 2: fileA
	_ = os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("content A"), 0644)
	_, _ = ExecGit(repo, "add", "fileA.txt")
	_, _ = ExecGit(repo, "commit", "-m", "feat: file A")

	// Commit 3: multi-file commit to be split (fileB and fileC)
	_ = os.WriteFile(filepath.Join(repo, "fileB.txt"), []byte("content B"), 0644)
	_ = os.WriteFile(filepath.Join(repo, "fileC.txt"), []byte("content C"), 0644)
	_, _ = ExecGit(repo, "add", ".")
	_, _ = ExecGit(repo, "commit", "-m", "feat: big batch B and C")

	commit3Hash, _ := ExecGit(repo, "rev-parse", "HEAD")
	commit3Hash = strings.TrimSpace(commit3Hash)

	// 1. Test PreviewRebaseTopology
	preview, err := svc.PreviewRebaseTopology(repo, "main", "feature/surgeon")
	if err != nil {
		// Try master if main doesn't exist
		preview, err = svc.PreviewRebaseTopology(repo, "HEAD~2", "HEAD")
		if err != nil {
			t.Fatalf("PreviewRebaseTopology failed: %v", err)
		}
	}

	if preview.TotalCommits < 2 {
		t.Errorf("expected at least 2 commits in preview, got %d", preview.TotalCommits)
	}

	// 2. Test SplitCommitSurgically on commit3
	partitions := []CommitPartition{
		{
			Subject: "feat: partition 1 - file B only",
			Files:   []string{"fileB.txt"},
		},
		{
			Subject: "feat: partition 2 - file C only",
			Files:   []string{"fileC.txt"},
		},
	}

	splitRes, err := svc.SplitCommitSurgically(repo, commit3Hash, partitions)
	if err != nil {
		t.Fatalf("SplitCommitSurgically failed: %v", err)
	}

	if !splitRes.Success {
		t.Fatalf("expected split success: %s", splitRes.Message)
	}
	if len(splitRes.GeneratedCommits) != 2 {
		t.Fatalf("expected 2 generated commits, got %d", len(splitRes.GeneratedCommits))
	}

	// Verify git log has both new commit subjects
	logOut, _ := ExecGit(repo, "log", "-n", "2", "--oneline")
	if !strings.Contains(logOut, "file B only") || !strings.Contains(logOut, "file C only") {
		t.Errorf("expected split commit subjects in git log, got:\n%s", logOut)
	}
}
