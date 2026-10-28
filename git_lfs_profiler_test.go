package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAnalyzeLfsStorageInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-lfs-prof-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")
	runCmd("lfs", "install", "--local")
	runCmd("lfs", "track", "*.bin")
	runCmd("add", ".gitattributes")
	runCmd("commit", "-m", "init lfs track")

	// Create binary asset
	binFile := filepath.Join(tempDir, "sample.bin")
	if err := os.WriteFile(binFile, []byte("large binary asset simulation data\n"), 0644); err != nil {
		t.Fatalf("failed to write bin file: %v", err)
	}
	runCmd("add", "sample.bin")
	runCmd("commit", "-m", "add lfs binary asset")

	svc := &GitService{}
	analysis, err := svc.AnalyzeLfsStorage(tempDir)
	if err != nil {
		t.Fatalf("AnalyzeLfsStorage failed: %v", err)
	}

	if analysis.ObjectCount == 0 {
		t.Fatalf("expected objectCount > 0, got 0")
	}
	if len(analysis.TypeBreakdown) == 0 {
		t.Fatalf("expected at least 1 extension breakdown, got none")
	}

	foundBin := false
	for _, tw := range analysis.TypeBreakdown {
		if tw.Extension == ".bin" {
			foundBin = true
			if tw.FileCount < 1 {
				t.Fatalf("expected fileCount >= 1 for .bin, got: %d", tw.FileCount)
			}
		}
	}
	if !foundBin {
		t.Fatalf("expected .bin extension in typeBreakdown, got: %+v", analysis.TypeBreakdown)
	}
	t.Logf("LFS Analysis successful: ObjectCount=%d TotalSize=%s PruneAdvice=%s", analysis.ObjectCount, analysis.FormattedTotal, analysis.PruneAdvice)
}
