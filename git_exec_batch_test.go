package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkSlice(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	chunks := ChunkSlice(items, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || chunks[0][0] != "a" || chunks[0][1] != "b" {
		t.Errorf("chunk 0 invalid: %v", chunks[0])
	}
	if len(chunks[1]) != 2 || chunks[1][0] != "c" || chunks[1][1] != "d" {
		t.Errorf("chunk 1 invalid: %v", chunks[1])
	}
	if len(chunks[2]) != 1 || chunks[2][0] != "e" {
		t.Errorf("chunk 2 invalid: %v", chunks[2])
	}

	// Empty
	if c := ChunkSlice([]string{}, 10); c != nil {
		t.Errorf("expected nil for empty slice, got %v", c)
	}

	// Default chunk size
	big := make([]int, 1200)
	bigChunks := ChunkSlice(big, 0)
	if len(bigChunks) != 3 { // 500, 500, 200
		t.Errorf("expected 3 chunks with default batch size 500, got %d", len(bigChunks))
	}
}

func TestExecGitBatched(t *testing.T) {
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(dir, "config", "user.name", "tester")
	_, _ = ExecGit(dir, "config", "user.email", "test@test.com")

	// Create 15 small files
	var files []string
	for i := 1; i <= 15; i++ {
		fname := fmt.Sprintf("file_%02d.txt", i)
		full := filepath.Join(dir, fname)
		if err := os.WriteFile(full, []byte(fmt.Sprintf("hello %d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, fname)
	}

	// Batch add with chunk size 5 (3 batches)
	outs, err := ExecGitBatched(dir, []string{"add"}, files, 5)
	if err != nil {
		t.Fatalf("ExecGitBatched failed: %v", err)
	}
	if len(outs) != 3 {
		t.Errorf("expected 3 batch results, got %d", len(outs))
	}

	// Verify all 15 are staged
	status, err := ExecGit(dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !containsSubstr(status, f) {
			t.Errorf("file %s not staged in status: %s", f, status)
		}
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || filepath.Base(s) == sub || stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
