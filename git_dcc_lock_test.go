package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectDCCLockActivityReportsUnknownLockState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	lockPath := filepath.Join(repo, "~$Hero.psd")
	if err := os.WriteFile(lockPath, []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := (GitService{}).DetectDCCLockActivity(repo)
	if err != nil {
		t.Fatalf("DetectDCCLockActivity failed: %v", err)
	}
	if !strings.Contains(res.Summary, "LFS 锁查询失败") || !strings.Contains(res.Message, "锁状态未知") {
		t.Fatalf("lock query failure must be disclosed: %+v", res)
	}
	if res.UnprotectedCount != 0 {
		t.Fatalf("unknown lock state must not be counted as unprotected: %+v", res)
	}
	found := false
	for _, item := range res.Items {
		if item.LockFilePath == "~$Hero.psd" {
			found = true
			if item.LfsLockStatus != "UNKNOWN" {
				t.Fatalf("lock status = %q, want UNKNOWN: %+v", item.LfsLockStatus, item)
			}
			if item.CollisionRisk {
				t.Fatalf("unknown lock state must not be a collision: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("DCC temp file was not detected: %+v", res.Items)
	}
}
