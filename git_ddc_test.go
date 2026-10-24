package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManageStudioDDCReportsUnreadableCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	svc := GitService{}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	cacheDir := filepath.Join(repo, "DerivedDataCache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.bin"), []byte("cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cacheDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	res, err := svc.ManageStudioDDC(repo, "STATUS")
	if err != nil {
		t.Fatalf("ManageStudioDDC failed: %v", err)
	}
	if !strings.Contains(res.Summary, "无法读取") || !strings.Contains(res.Summary, "统计可能不完整") {
		t.Fatalf("unreadable cache must be disclosed: %q", res.Summary)
	}
	if _, err := svc.ManageStudioDDC(repo, "BOGUS"); err == nil {
		t.Fatal("unknown DDC actions must fail instead of silently listing")
	}
}
