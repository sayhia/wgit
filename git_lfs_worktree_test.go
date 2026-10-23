package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLfsCacheUsesCommonDirInLinkedWorktree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	root := t.TempDir()
	repo := filepath.Join(root, "main")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runIn := func(dir string, args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}

	runIn(repo, "init", "-q", "-b", "main")
	runIn(repo, "config", "user.email", "tester@example.com")
	runIn(repo, "config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	runIn(repo, "add", "base.txt")
	runIn(repo, "commit", "-qm", "base")

	wt := filepath.Join(root, "wt")
	runIn(repo, "worktree", "add", "-q", "-b", "feature/lfs", wt)

	oid := strings.Repeat("ab", 32)
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:" + oid + "\nsize 7\n"
	if err := os.WriteFile(filepath.Join(wt, "asset.bin"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	runIn(wt, "add", "asset.bin")
	runIn(wt, "commit", "-qm", "add lfs pointer")

	objectPath := filepath.Join(repo, ".git", "lfs", "objects", oid[:2], oid[2:4], oid)
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	stats, err := svc.InspectLfsCacheStats(wt)
	if err != nil {
		t.Fatalf("InspectLfsCacheStats failed: %v", err)
	}
	if stats.FileCount != 1 || stats.TotalSizeBytes != 7 {
		t.Fatalf("worktree cache should resolve to the main repo: %+v", stats)
	}
	usage, err := svc.GetLfsDiskUsage(wt)
	if err != nil || usage == "0 B" {
		t.Fatalf("GetLfsDiskUsage(worktree) = %q err=%v", usage, err)
	}

	pre, err := svc.VerifyPrePushLfs(wt, "", "")
	if err != nil {
		t.Fatalf("VerifyPrePushLfs failed: %v", err)
	}
	if pre.TotalLfsPointers != 1 || pre.MissingLfsCount != 0 || !pre.IsSafe {
		t.Fatalf("cached object in main repo should satisfy worktree preflight: %+v", pre)
	}

	maintenance, err := svc.GetMaintenanceStats(wt)
	if err != nil || maintenance.GitDirSize == "" || maintenance.GitDirSize == "0 KB" {
		t.Fatalf("worktree maintenance stats should use the common git dir: %+v err=%v", maintenance, err)
	}
	disk, err := svc.GetRepositoryDiskBreakdown(wt)
	if err != nil || disk.GitInternalSize == 0 || disk.LfsStoreSize != 7 {
		t.Fatalf("worktree disk breakdown should include common git/lfs storage: %+v err=%v", disk, err)
	}

	if err := os.Remove(objectPath); err != nil {
		t.Fatal(err)
	}
	pre, err = svc.VerifyPrePushLfs(wt, "", "")
	if err != nil {
		t.Fatalf("VerifyPrePushLfs after delete failed: %v", err)
	}
	if pre.MissingLfsCount != 1 || pre.IsSafe {
		t.Fatalf("deleted object should be reported missing: %+v", pre)
	}
	if len(pre.MissingLfsFiles) != 1 || pre.MissingLfsFiles[0] != "asset.bin" {
		t.Fatalf("missing files = %+v", pre.MissingLfsFiles)
	}
}
