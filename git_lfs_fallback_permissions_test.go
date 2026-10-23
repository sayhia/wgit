package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLfsTrackedPatternsFallbackAndLockablePermissions(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	attrs := "*.uasset filter=lfs diff=lfs merge=lfs -text\n*.fbx filter=lfs diff=lfs merge=lfs -text\n"
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte(attrs), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"art/hero.uasset", "art/hero.fbx", "notes.txt"} {
		full := filepath.Join(repo, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("data\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	patterns, err := svc.GetLfsTrackedPatterns(repo)
	if err != nil {
		t.Fatalf("GetLfsTrackedPatterns failed: %v", err)
	}
	found := map[string]string{}
	for _, pattern := range patterns {
		found[pattern.Pattern] = pattern.Source
	}
	if found["*.uasset"] != ".gitattributes" || found["*.fbx"] != ".gitattributes" {
		t.Fatalf("expected .gitattributes fallback patterns, got %+v", patterns)
	}

	state, err := svc.SyncLockableFilePermissions(repo)
	if err != nil || state == nil || state.TotalLockableFiles != 2 || state.WritableCount != 2 || state.ReadOnlyCount != 0 || state.IsProtectionActive {
		t.Fatalf("inactive protection sync failed: %+v err=%v", state, err)
	}
	if state.ProtectedFiles == nil {
		t.Fatalf("protected files slice must not be nil: %+v", state)
	}
	for _, file := range []string{"art/hero.uasset", "art/hero.fbx"} {
		info, err := os.Stat(filepath.Join(repo, filepath.FromSlash(file)))
		if err != nil {
			t.Fatalf("stat %s failed: %v", file, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s should be writable while protection is off, got %o", file, info.Mode().Perm())
		}
	}

	state, err = svc.ToggleLockableReadOnlyProtection(repo, true)
	if err != nil || state == nil || !state.IsProtectionActive || state.TotalLockableFiles != 2 || state.ReadOnlyCount != 2 || state.WritableCount != 0 {
		t.Fatalf("enable protection failed: %+v err=%v", state, err)
	}
	for _, file := range []string{"art/hero.uasset", "art/hero.fbx"} {
		info, err := os.Stat(filepath.Join(repo, filepath.FromSlash(file)))
		if err != nil {
			t.Fatalf("stat %s failed: %v", file, err)
		}
		if info.Mode().Perm() != 0o444 {
			t.Fatalf("%s should be read-only under protection, got %o", file, info.Mode().Perm())
		}
		if !strings.Contains(strings.Join(state.ProtectedFiles, ","), file) {
			t.Fatalf("expected %s in protected files, got %+v", file, state.ProtectedFiles)
		}
	}
	noteInfo, _ := os.Stat(filepath.Join(repo, "notes.txt"))
	if noteInfo.Mode().Perm() != 0o644 {
		t.Fatalf("non-lockable file permissions changed: %o", noteInfo.Mode().Perm())
	}

	state, err = svc.ToggleLockableReadOnlyProtection(repo, false)
	if err != nil || state == nil || state.IsProtectionActive || state.WritableCount != 2 {
		t.Fatalf("disable protection failed: %+v err=%v", state, err)
	}
	for _, file := range []string{"art/hero.uasset", "art/hero.fbx"} {
		info, _ := os.Stat(filepath.Join(repo, filepath.FromSlash(file)))
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s should be writable after disabling protection, got %o", file, info.Mode().Perm())
		}
	}
}
