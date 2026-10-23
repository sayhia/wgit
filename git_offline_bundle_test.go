package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateOfflinePatchBundleCreatesVerifiedBundle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "base.txt")
	run("commit", "-qm", "base")
	base := strings.TrimSpace(run("rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:" + strings.Repeat("a", 64) + "\nsize 123\n"
	if err := os.WriteFile(filepath.Join(repo, "asset.bin"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "feature.txt", "asset.bin")
	run("commit", "-qm", "feature with lfs pointer")

	outPath := filepath.Join(t.TempDir(), "offline.bundle")
	res, err := (GitService{}).GenerateOfflinePatchBundle(repo, base, "HEAD", outPath)
	if err != nil {
		t.Fatalf("GenerateOfflinePatchBundle failed: %v", err)
	}
	if !res.Success || res.CommitsCount != 1 || res.LfsBlobsCount != 1 {
		t.Fatalf("unexpected bundle result: %+v", res)
	}
	data, err := os.ReadFile(outPath)
	if err != nil || len(data) == 0 || res.TotalSizeBytes != int64(len(data)) {
		t.Fatalf("bundle file missing or wrong size: %+v err=%v", res, err)
	}
	wantSHA := fmt.Sprintf("%x", sha256.Sum256(data))
	if res.Sha256Checksum != wantSHA {
		t.Fatalf("checksum mismatch: got %s want %s", res.Sha256Checksum, wantSHA)
	}
	if !strings.Contains(strings.Join(res.ManifestEntries, "\n"), "LFS: asset.bin") {
		t.Fatalf("manifest should list LFS pointer: %+v", res.ManifestEntries)
	}
	if out, err := ExecGit(repo, "bundle", "verify", outPath); err != nil {
		t.Fatalf("git bundle verify failed: %v (%s)", err, out)
	}

	if _, err := (GitService{}).GenerateOfflinePatchBundle(repo, "missing-ref", "HEAD", outPath); err == nil {
		t.Fatal("invalid base ref must fail")
	}
}

func TestCollectPatchLfsPointersRejectsInvalidRefs(t *testing.T) {
	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if _, _, err := collectPatchLfsPointers(repo, "missing-ref", "HEAD"); err == nil {
		t.Fatal("invalid refs must surface an error instead of reporting zero LFS pointers")
	}
}
