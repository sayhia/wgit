package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzePackageSizeBudgetReportsUnreadablePaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "hero.png"), []byte(strings.Repeat("x", 2048)), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(repo, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "asset.bin"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	res, err := (GitService{}).AnalyzePackageSizeBudget(repo, "IOS")
	if err != nil {
		t.Fatalf("AnalyzePackageSizeBudget failed: %v", err)
	}
	if !strings.Contains(res.Summary, "无法读取") || !strings.Contains(res.Summary, "统计可能不完整") {
		t.Fatalf("unreadable paths must be disclosed in the summary: %q", res.Summary)
	}
}

func TestScanLargeFilesReturnsEmptyArray(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	result, err := (GitService{}).ScanLargeFiles(repo, 100)
	if err != nil {
		t.Fatalf("ScanLargeFiles failed: %v", err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("expected an empty non-nil result, got %+v", result)
	}
	raw, err := json.Marshal(result)
	if err != nil || string(raw) != "[]" {
		t.Fatalf("empty scan must serialize as [], got %s err=%v", raw, err)
	}
}
