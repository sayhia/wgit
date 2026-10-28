package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyArtifactChecksum(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-checksum-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := []byte("distribution package payload 1234567890")
	f := filepath.Join(tempDir, "release-pkg.tar.gz")
	if err := os.WriteFile(f, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	h := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(h[:])

	service := &GitService{}
	res, err := service.VerifyArtifactChecksum(f, expectedHex)
	if err != nil {
		t.Fatalf("VerifyArtifactChecksum failed: %v", err)
	}

	if !res.Matches {
		t.Errorf("expected Matches = true, got false")
	}
	if res.ComputedHash != expectedHex {
		t.Errorf("expected %s, got %s", expectedHex, res.ComputedHash)
	}

	// Test mismatch
	resMismatch, err := service.VerifyArtifactChecksum(f, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("VerifyArtifactChecksum failed on mismatch test: %v", err)
	}
	if resMismatch.Matches {
		t.Errorf("expected Matches = false on wrong hash")
	}
}
