package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportAndImportReviewNotesInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-notes-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	f := filepath.Join(tempDir, "file.txt")
	_ = os.WriteFile(f, []byte("content\n"), 0644)
	runCmd("add", ".")
	runCmd("commit", "-m", "initial commit")
	hash := runCmd("rev-parse", "HEAD")

	// Add note to hash
	runCmd("notes", "add", "-m", "LGTM, clean code review feedback", hash)

	svc := &GitService{}
	exportPath := filepath.Join(tempDir, "notes_bundle.json")
	if err := svc.ExportReviewNotes(tempDir, exportPath); err != nil {
		t.Fatalf("ExportReviewNotes failed: %v", err)
	}

	// Remove note
	runCmd("notes", "remove", hash)

	// Import note
	res, err := svc.ImportReviewNotes(tempDir, exportPath)
	if err != nil {
		t.Fatalf("ImportReviewNotes failed: %v", err)
	}
	if res.TotalImported != 1 {
		t.Fatalf("expected 1 note imported, got: %d", res.TotalImported)
	}

	// Verify note was restored
	noteContent := runCmd("notes", "show", hash)
	if !strings.Contains(noteContent, "LGTM") {
		t.Fatalf("expected note to contain LGTM, got: %s", noteContent)
	}
}
