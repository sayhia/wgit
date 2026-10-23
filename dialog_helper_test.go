package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestChooseFolderFallbackSyntax(t *testing.T) {
	// Verify runtime OS check does not panic
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("Skipping on non-standard OS")
	}

	// Verify filepath clean logic works as expected
	sample := filepath.Clean("/Users/sunmking/workplace/wgit/")
	if filepath.Base(sample) != "wgit" {
		t.Errorf("Unexpected base: %s", filepath.Base(sample))
	}
}
