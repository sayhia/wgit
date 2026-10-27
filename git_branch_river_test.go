package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchRiverAndStaleArchiver(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}

	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	// Ensure branch name is main
	_, _ = ExecGit(repo, "checkout", "-b", "main")
	_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("# River Test"), 0644)
	_, _ = ExecGit(repo, "add", "README.md")
	_, _ = ExecGit(repo, "commit", "-m", "chore: initial commit on main")

	// Create feature/alpha branch with 1 commit ahead
	_, _ = ExecGit(repo, "checkout", "-b", "feature/alpha")
	_ = os.WriteFile(filepath.Join(repo, "alpha.txt"), []byte("alpha"), 0644)
	_, _ = ExecGit(repo, "add", "alpha.txt")
	_, _ = ExecGit(repo, "commit", "-m", "feat: alpha work")

	// Create feature/merged branch and merge it to main
	_, _ = ExecGit(repo, "checkout", "main")
	_, _ = ExecGit(repo, "checkout", "-b", "feature/merged")
	_ = os.WriteFile(filepath.Join(repo, "merged.txt"), []byte("merged"), 0644)
	_, _ = ExecGit(repo, "add", "merged.txt")
	_, _ = ExecGit(repo, "commit", "-m", "feat: merged feature")

	_, _ = ExecGit(repo, "checkout", "main")
	_, _ = ExecGit(repo, "merge", "--no-ff", "feature/merged", "-m", "merge: feature/merged into main")

	// 1. Test GetBranchRiverData
	riverData, err := svc.GetBranchRiverData(repo, "main")
	if err != nil {
		t.Fatalf("GetBranchRiverData failed: %v", err)
	}
	if riverData.BaseBranch != "main" {
		t.Errorf("expected base branch main, got %s", riverData.BaseBranch)
	}
	if len(riverData.Nodes) < 3 {
		t.Errorf("expected at least 3 nodes, got %d", len(riverData.Nodes))
	}

	var foundMerged, foundAlpha bool
	for _, node := range riverData.Nodes {
		if node.Name == "feature/merged" {
			foundMerged = true
			if !node.IsMerged {
				t.Errorf("expected feature/merged to be marked as merged")
			}
			if node.Status != "merged" {
				t.Errorf("expected status 'merged', got %s", node.Status)
			}
		}
		if node.Name == "feature/alpha" {
			foundAlpha = true
			if node.AheadCount == 0 {
				t.Errorf("expected feature/alpha ahead count > 0, got %d", node.AheadCount)
			}
		}
	}
	if !foundMerged || !foundAlpha {
		t.Errorf("missing branches in river data: foundMerged=%v, foundAlpha=%v", foundMerged, foundAlpha)
	}

	// 2. Test DetectStaleBranches
	staleReport, err := svc.DetectStaleBranches(repo, 30)
	if err != nil {
		t.Fatalf("DetectStaleBranches failed: %v", err)
	}
	if staleReport.MergedCount < 1 {
		t.Errorf("expected at least 1 merged branch detected, got %d", staleReport.MergedCount)
	}

	// 3. Test ArchiveAndDeleteBranches
	archiveRes, err := svc.ArchiveAndDeleteBranches(repo, []string{"feature/merged"}, "")
	if err != nil {
		t.Fatalf("ArchiveAndDeleteBranches failed: %v", err)
	}
	if !archiveRes.Success {
		t.Fatalf("ArchiveAndDeleteBranches returned false: %s", archiveRes.Message)
	}
	if len(archiveRes.DeletedBranches) != 1 || archiveRes.DeletedBranches[0] != "feature/merged" {
		t.Errorf("expected feature/merged deleted, got %v", archiveRes.DeletedBranches)
	}
	if _, err := os.Stat(archiveRes.BundlePath); os.IsNotExist(err) {
		t.Errorf("bundle file does not exist at %s", archiveRes.BundlePath)
	}

	// Verify branch is gone
	branchesAfter, _ := ExecGit(repo, "branch")
	if strings.Contains(branchesAfter, "feature/merged") {
		t.Errorf("expected feature/merged to be deleted from git branch")
	}
}
