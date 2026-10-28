package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceCleanerInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-cleaner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Commit 1 on main
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\n"), 0644)
	runGit("add", "main.go")
	runGit("commit", "-m", "init")

	// Create some junk files
	dsStore := filepath.Join(tempDir, ".DS_Store")
	_ = os.WriteFile(dsStore, []byte("junk ds store content"), 0644)

	appLog := filepath.Join(tempDir, "debug.log")
	_ = os.WriteFile(appLog, []byte("error log line 1\nline 2"), 0644)

	untrackedSrc := filepath.Join(tempDir, "untracked.go")
	_ = os.WriteFile(untrackedSrc, []byte("package main\nfunc Untracked() {}\n"), 0644)

	// 1. Test ScanWorkspaceJunk
	rep, err := svc.ScanWorkspaceJunk(tempDir, true)
	if err != nil {
		t.Fatalf("ScanWorkspaceJunk failed: %v", err)
	}
	if rep.TotalFiles < 3 {
		t.Errorf("expected at least 3 junk files, got %d", rep.TotalFiles)
	}
	if len(rep.RecommendedGitIgnore) == 0 {
		t.Errorf("expected recommended gitignore patterns, got none")
	}

	// 2. Test AppendRecommendedGitIgnore
	ok, err := svc.AppendRecommendedGitIgnore(tempDir, rep.RecommendedGitIgnore)
	if err != nil || !ok {
		t.Fatalf("AppendRecommendedGitIgnore failed: %v", err)
	}

	gitIgnoreContent, _ := os.ReadFile(filepath.Join(tempDir, ".gitignore"))
	if !strings.Contains(string(gitIgnoreContent), ".DS_Store") {
		t.Errorf("expected .gitignore to contain .DS_Store, got: %s", string(gitIgnoreContent))
	}

	// 3. Test ExecuteSafeClean (dryRun = true)
	dryRes, err := svc.ExecuteSafeClean(tempDir, []string{"osArtifacts"}, true)
	if err != nil {
		t.Fatalf("ExecuteSafeClean dryRun failed: %v", err)
	}
	if dryRes.CleanedFilesCount != 1 {
		t.Errorf("expected 1 file in dry run, got %d", dryRes.CleanedFilesCount)
	}
	if _, err := os.Stat(dsStore); err != nil {
		t.Errorf(".DS_Store should still exist after dry run")
	}

	// 4. Test ExecuteSafeClean (dryRun = false)
	realRes, err := svc.ExecuteSafeClean(tempDir, []string{"osArtifacts"}, false)
	if err != nil {
		t.Fatalf("ExecuteSafeClean real failed: %v", err)
	}
	if realRes.CleanedFilesCount != 1 {
		t.Errorf("expected 1 file cleaned, got %d", realRes.CleanedFilesCount)
	}
	if _, err := os.Stat(dsStore); !os.IsNotExist(err) {
		t.Errorf(".DS_Store should have been deleted")
	}
}
