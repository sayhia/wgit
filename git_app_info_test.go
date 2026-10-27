package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetAppVersion(t *testing.T) {
	svc := GitService{}
	info, err := svc.GetAppVersion()
	if err != nil {
		t.Fatalf("unexpected error getting app version: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil AppVersionInfo")
	}
	if info.Version == "" {
		t.Error("expected non-empty Version")
	}
	if info.Platform != runtime.GOOS {
		t.Errorf("expected platform %s, got %s", runtime.GOOS, info.Platform)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("expected arch %s, got %s", runtime.GOARCH, info.Arch)
	}
	if info.GoVersion == "" {
		t.Error("expected non-empty GoVersion")
	}
}

func TestOpenFileInEditorFallback(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	svc := GitService{}
	// Use an unknown editor to exercise the fallback path
	// It will fall back to OpenFileWithDefaultApp which might return an error or start an opener
	_ = svc.OpenFileInEditor(tempDir, tempFile, 10, "nonexistent-editor-xyz-1234")
}
