package main

import (
	"os"
	"testing"
)

func TestGitSyncOpsStructures(t *testing.T) {
	pullOpts := PullAdvancedOptions{
		Remote:     "origin",
		Branch:     "main",
		Strategy:   "rebase",
		Autostash:  true,
		Prune:      true,
		Submodules: true,
	}
	if pullOpts.Strategy != "rebase" || !pullOpts.Autostash {
		t.Errorf("PullAdvancedOptions failed to retain values: %+v", pullOpts)
	}

	pushOpts := PushAdvancedOptions{
		Remote:       "origin",
		Branch:       "feature",
		RemoteBranch: "feature-remote",
		ForceMode:    "lease",
		SetUpstream:  true,
		FollowTags:   true,
		SkipCI:       true,
	}
	if pushOpts.ForceMode != "lease" || !pushOpts.SetUpstream {
		t.Errorf("PushAdvancedOptions failed to retain values: %+v", pushOpts)
	}

	preview := MergePreviewResult{
		CanMergeCleanly: true,
		HasConflicts:    false,
		Conflicts:       []string{},
		ChangedFiles:    []string{"file1.txt", "file2.txt"},
		AheadCount:      2,
		BehindCount:     1,
		BaseCommit:      "abc1234",
		TargetCommit:    "def5678",
	}
	if !preview.CanMergeCleanly || preview.AheadCount != 2 {
		t.Errorf("MergePreviewResult unexpected: %+v", preview)
	}
}

func TestConflictOperationsNoOp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wgit-sync-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	svc := GitService{}
	_, err = svc.AbortConflictOperation(tmpDir)
	if err == nil {
		t.Error("Expected error from AbortConflictOperation on clean/empty dir, got nil")
	}

	_, err = svc.ContinueConflictOperation(tmpDir)
	if err == nil {
		t.Error("Expected error from ContinueConflictOperation on clean/empty dir, got nil")
	}

	_, err = svc.SkipConflictOperation(tmpDir)
	if err == nil {
		t.Error("Expected error from SkipConflictOperation on clean/empty dir, got nil")
	}
}
