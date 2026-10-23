package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManageStudioDDCUsesLocalCache(t *testing.T) {
	repo := t.TempDir()
	cacheDir := filepath.Join(repo, "DerivedDataCache", "bucket")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(cacheDir, "fresh.bin")
	stale := filepath.Join(cacheDir, "stale.bin")
	if err := os.WriteFile(fresh, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, make([]byte, 50), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	res, err := svc.ManageStudioDDC(repo, "status")
	if err != nil {
		t.Fatalf("ManageStudioDDC failed: %v", err)
	}
	if res.TotalCacheSizeBytes != 150 || res.StaleBlobsCount != 1 || res.CacheHitRatio != 0 || res.IsSharedConnected {
		t.Fatalf("unexpected DDC stats: %+v", res)
	}
	if !strings.Contains(res.Summary, "本地 DDC 缓存") {
		t.Fatalf("unexpected summary: %q", res.Summary)
	}

	res, err = svc.ManageStudioDDC(repo, "PRUNE_STALE")
	if err != nil {
		t.Fatalf("ManageStudioDDC prune failed: %v", err)
	}
	if res.StaleBlobsCount != 0 || res.TotalCacheSizeBytes != 100 || !strings.Contains(res.Summary, "已删除 1 个") {
		t.Fatalf("unexpected prune result: %+v", res)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale cache should be removed: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh cache should remain: %v", err)
	}

	empty, err := svc.ManageStudioDDC(t.TempDir(), "status")
	if err != nil {
		t.Fatalf("ManageStudioDDC on empty repo failed: %v", err)
	}
	if empty.TotalCacheSizeBytes != 0 || !strings.Contains(empty.Summary, "未检测到") {
		t.Fatalf("unexpected empty DDC result: %+v", empty)
	}
}
