package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBundleAuditInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-bundle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	f1 := filepath.Join(tempDir, "file1.txt")
	os.WriteFile(f1, []byte("release bundle line 1\n"), 0644)
	runGit("add", "file1.txt")
	runGit("commit", "-m", "initial release commit")

	bundleFile := filepath.Join(tempDir, "repo.bundle")
	runGit("bundle", "create", bundleFile, "HEAD")

	svc := GitService{}

	// Test AuditAirGappedBundle
	audit, err := svc.AuditAirGappedBundle(tempDir, bundleFile)
	if err != nil {
		t.Fatalf("AuditAirGappedBundle failed: %v", err)
	}
	if audit == nil || !audit.IsValid {
		t.Fatalf("Expected valid bundle audit, got %+v", audit)
	}

	// Test GenerateReleaseChecksumManifest
	manifest, err := svc.GenerateReleaseChecksumManifest(tempDir, "HEAD")
	if err != nil {
		t.Fatalf("GenerateReleaseChecksumManifest failed: %v", err)
	}
	if manifest == nil || manifest.TotalFiles == 0 || manifest.ManifestSha256 == "" {
		t.Fatalf("Expected manifest entries, got %+v", manifest)
	}
}
