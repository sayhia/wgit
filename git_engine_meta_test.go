package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectAssetMetadataUsesRealLfsRules(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	write(".gitattributes", "*.psd filter=lfs diff=lfs merge=lfs -text\n")
	write("Assets/hero.png", "png")
	write("Assets/hero.psd", "psd")

	pngMeta, err := GitService{}.InspectAssetMetadata(dir, "Assets/hero.png")
	if err != nil {
		t.Fatalf("InspectAssetMetadata png: %v", err)
	}
	if pngMeta.IsLFS {
		t.Error("hero.png should not be LFS without a matching rule")
	}

	psdMeta, err := GitService{}.InspectAssetMetadata(dir, "Assets/hero.psd")
	if err != nil {
		t.Fatalf("InspectAssetMetadata psd: %v", err)
	}
	if !psdMeta.IsLFS {
		t.Error("hero.psd should be LFS via *.psd rule")
	}
}

func TestAutoFixUnityMetaStagesUnpairedAsset(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")
	run("commit", "-q", "--allow-empty", "-m", "base")

	write("Assets/fixme.png", "png")
	write("Assets/fixme.png.meta", "guid: fixme\n")
	// Stage only the meta; the asset is untracked, so the pair is unpaired.
	run("add", "Assets/fixme.png.meta")

	mismatches, err := GitService{}.CheckUnityMetaSync(dir)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync: %v", err)
	}
	if len(mismatches) != 1 || mismatches[0].IssueType != "unpaired_staged" {
		t.Fatalf("expected one unpaired_staged mismatch, got %+v", mismatches)
	}

	fixed, err := GitService{}.AutoFixUnityMeta(dir)
	if err != nil {
		t.Fatalf("AutoFixUnityMeta: %v", err)
	}
	found := false
	for _, path := range fixed {
		if path == "Assets/fixme.png" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fixme.png to be staged, fixed=%v", fixed)
	}

	staged, _ := ExecGit(dir, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "Assets/fixme.png") {
		t.Errorf("asset not staged after fix:\n%s", staged)
	}

	after, err := GitService{}.CheckUnityMetaSync(dir)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync after fix: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected no mismatches after fix, got %+v", after)
	}
}

func TestCheckUnityMetaSyncIgnoresNonAssetPaths(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")
	run("commit", "-q", "--allow-empty", "-m", "base")

	// Non-Assets paths must not be reported as missing .meta companions.
	write(".gitattributes", "*.psd filter=lfs diff=lfs merge=lfs -text\n")
	write("ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 2022.3.10f1\n")
	write("Assets/hero.png", "png")
	write("Assets/orphan.png.meta", "guid: orphan\n")
	write("Assets/paired.png", "png")
	write("Assets/paired.png.meta", "guid: paired\n")

	mismatches, err := GitService{}.CheckUnityMetaSync(dir)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync: %v", err)
	}
	if len(mismatches) != 2 {
		t.Fatalf("got %d mismatches, want 2: %+v", len(mismatches), mismatches)
	}

	byAsset := map[string]string{}
	for _, m := range mismatches {
		byAsset[m.AssetPath] = m.IssueType
	}
	if got := byAsset["Assets/hero.png"]; got != "missing_meta" {
		t.Errorf("hero.png issue = %q, want missing_meta", got)
	}
	if got := byAsset["Assets/orphan.png"]; got != "orphan_meta" {
		t.Errorf("orphan.png issue = %q, want orphan_meta", got)
	}
	if _, ok := byAsset[".gitattributes"]; ok {
		t.Errorf(".gitattributes should not require a .meta: %+v", mismatches)
	}
	if _, ok := byAsset["ProjectSettings/"]; ok {
		t.Errorf("ProjectSettings/ should not require a .meta: %+v", mismatches)
	}
}

func TestCheckUnityMetaSyncReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	mismatches, err := GitService{}.CheckUnityMetaSync(dir)
	if err != nil {
		t.Fatalf("CheckUnityMetaSync: %v", err)
	}
	if mismatches == nil {
		t.Fatal("expected empty slice, got nil")
	}
}

func TestAutoFixUnityMetaPropagatesStageFailure(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")
	run("commit", "-q", "--allow-empty", "-m", "base")
	write("Assets/fixme.png", "png")
	write("Assets/fixme.png.meta", "guid: fixme\n")
	run("add", "Assets/fixme.png.meta")

	gitDir := filepath.Join(dir, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })

	fixed, err := GitService{}.AutoFixUnityMeta(dir)
	if err == nil {
		t.Fatalf("stage failure must be surfaced, fixed=%v", fixed)
	}
	if len(fixed) != 0 {
		t.Fatalf("failed staging must not be reported as fixed: %v", fixed)
	}
}
