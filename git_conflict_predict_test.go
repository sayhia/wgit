package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPredictMergeConflicts(t *testing.T) {
	dir := newNotesRepo(t)
	svc := GitService{}

	// Initial commit on main
	fileA := filepath.Join(dir, "fileA.txt")
	_ = os.WriteFile(fileA, []byte("line 1\nline 2\nline 3\n"), 0644)
	runGitTest(t, dir, nil, "add", "fileA.txt")
	runGitTest(t, dir, nil, "commit", "-m", "initial commit")

	// Create and switch to feature
	runGitTest(t, dir, nil, "checkout", "-b", "feature")
	_ = os.WriteFile(fileA, []byte("line 1\nfeature change\nline 3\n"), 0644)
	runGitTest(t, dir, nil, "add", "fileA.txt")
	runGitTest(t, dir, nil, "commit", "-m", "feature change")

	// Switch back to main and make conflicting change
	runGitTest(t, dir, nil, "checkout", "main")
	_ = os.WriteFile(fileA, []byte("line 1\nmain conflict change\nline 3\n"), 0644)
	runGitTest(t, dir, nil, "add", "fileA.txt")
	runGitTest(t, dir, nil, "commit", "-m", "main change")

	// Predict merge conflicts
	pred, err := svc.PredictMergeConflicts(dir, "feature")
	if err != nil {
		t.Fatalf("PredictMergeConflicts failed: %v", err)
	}

	if pred.RiskScore <= 0 {
		t.Errorf("expected RiskScore > 0, got %d", pred.RiskScore)
	}
	if len(pred.ConflictingFiles) == 0 {
		t.Errorf("expected conflicting files to contain fileA.txt, got %v", pred.ConflictingFiles)
	}
}
