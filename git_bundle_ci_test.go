package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitBundleAndCiStatus(t *testing.T) {
	dir := newNotesRepo(t)
	svc := GitService{}

	// Create initial commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Hello Bundle\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	runGitTest(t, dir, nil, "add", "README.md")
	runGitTest(t, dir, nil, "commit", "-m", "Initial commit for bundle")

	headHash := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	// 1. Test CreateGitBundleAdvanced
	bundleOut := filepath.Join(t.TempDir(), "test_repo.bundle")
	res, err := svc.CreateGitBundleAdvanced(dir, "HEAD", bundleOut)
	if err != nil {
		t.Fatalf("CreateGitBundleAdvanced failed: %v", err)
	}

	if res.OutputPath != bundleOut {
		t.Errorf("expected OutputPath=%s, got %s", bundleOut, res.OutputPath)
	}
	if res.SizeBytes <= 0 {
		t.Errorf("expected bundle size > 0, got %d", res.SizeBytes)
	}
	if !strings.Contains(res.CloneHelp, "git clone") {
		t.Errorf("expected CloneHelp to contain 'git clone', got %s", res.CloneHelp)
	}

	// 2. Test VerifyGitBundle
	verifyMsg, err := svc.VerifyGitBundle(dir, bundleOut)
	if err != nil {
		t.Fatalf("VerifyGitBundle failed: %v", err)
	}
	if !strings.Contains(verifyMsg, "is okay") && !strings.Contains(verifyMsg, "HEAD") {
		t.Logf("Verify output: %s", verifyMsg)
	}

	// 3. Test GetCommitCIStatus on commit without CI notes
	ciStatus, err := svc.GetCommitCIStatus(dir, headHash)
	if err != nil {
		t.Fatalf("GetCommitCIStatus failed: %v", err)
	}
	if ciStatus.Status != "none" {
		t.Errorf("expected status 'none', got '%s'", ciStatus.Status)
	}

	// 4. Attach JSON CI note to refs/notes/ci
	jsonNote := `{"status":"success","pipeline":"Production Build","duration":"1m 45s","url":"https://ci.dev/build/123"}`
	runGitTest(t, dir, nil, "notes", "--ref=ci", "add", "-m", jsonNote, headHash)

	ciStatus, err = svc.GetCommitCIStatus(dir, headHash)
	if err != nil {
		t.Fatalf("GetCommitCIStatus with JSON note failed: %v", err)
	}
	if ciStatus.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", ciStatus.Status)
	}
	if ciStatus.Pipeline != "Production Build" {
		t.Errorf("expected pipeline 'Production Build', got '%s'", ciStatus.Pipeline)
	}
	if ciStatus.Duration != "1m 45s" {
		t.Errorf("expected duration '1m 45s', got '%s'", ciStatus.Duration)
	}

	// 5. Test plaintext CI note fallback
	runGitTest(t, dir, nil, "notes", "--ref=ci", "remove", headHash)
	runGitTest(t, dir, nil, "notes", "add", "-f", "-m", "CI: FAILURE in 3m 20s", headHash)

	ciStatus, err = svc.GetCommitCIStatus(dir, headHash)
	if err != nil {
		t.Fatalf("GetCommitCIStatus with plaintext note failed: %v", err)
	}
	if ciStatus.Status != "failure" {
		t.Errorf("expected status 'failure', got '%s'", ciStatus.Status)
	}
	if ciStatus.Duration != "3m 20s" {
		t.Errorf("expected duration '3m 20s', got '%s'", ciStatus.Duration)
	}
}
