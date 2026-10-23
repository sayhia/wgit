package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunMaintenanceReportsRemotePruneFailure(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	writeAndCommit(t, repo, "base.txt", "base\n", "chore: base", time.Now(), "Tester")
	if _, err := ExecGit(repo, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git")); err != nil {
		t.Fatalf("add broken remote: %v", err)
	}

	stats, err := svc.RunMaintenance(repo)
	if err != nil {
		t.Fatalf("RunMaintenance should report step failures in its result: %v", err)
	}
	if stats.Success || stats.RemotePruneStatus != "FAILED" {
		t.Fatalf("broken remote must mark remote prune as failed: %+v", stats)
	}
	if len(stats.Warnings) == 0 || !strings.Contains(strings.Join(stats.Warnings, " "), "远端引用清理失败") {
		t.Fatalf("remote prune failure must be surfaced: %+v", stats.Warnings)
	}
	if stats.GcStatus != "OK" || stats.PruneStatus != "OK" {
		t.Fatalf("local maintenance steps should still run: %+v", stats)
	}
	if !strings.Contains(stats.Summary, "失败") {
		t.Fatalf("summary should state that some steps failed: %q", stats.Summary)
	}
}

func TestRunMaintenanceRejectsInvalidRepo(t *testing.T) {
	svc := GitService{}
	dir := t.TempDir()
	if _, err := svc.RunMaintenance(dir); err == nil {
		t.Fatal("expected error for a directory that is not a Git repository")
	}
	if _, err := svc.GetMaintenanceStats(dir); err == nil {
		t.Fatal("expected error for maintenance stats on a non-repository")
	}
	if _, err := svc.RunMaintenance(""); err == nil {
		t.Fatal("expected error for empty repository path")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("maintenance must not create repository state: %v", err)
	}
}
