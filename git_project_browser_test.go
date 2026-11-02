package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A linked worktree's .git is a file, not a directory. Before this test the
// scanner only treated a .git *directory* as a repository marker, so worktrees
// were invisible to "scan local projects" — the very thing a worktree user
// would scan for.
func TestScanLocalProjectsFindsWorktrees(t *testing.T) {
	svc := GitService{}
	repo := initClosedLoopRepo(t)
	if !strings.HasSuffix(repo, filepath.Base(repo)) {
		t.Fatalf("unexpected repo path: %s", repo)
	}

	wtPath := filepath.Join(filepath.Dir(repo), "wt-sibling")
	out, err := exec.Command("git", "-C", repo, "worktree", "add", "-b", "scan-wt", wtPath).CombinedOutput()
	if err != nil {
		t.Fatalf("worktree add failed: %v\n%s", err, out)
	}

	repos, err := svc.ScanLocalProjects(filepath.Dir(repo))
	if err != nil {
		t.Fatalf("ScanLocalProjects failed: %v", err)
	}

	found := map[string]bool{}
	for _, r := range repos {
		found[filepath.Clean(r)] = true
	}
	if !found[filepath.Clean(repo)] {
		t.Fatalf("main repo missing from results: %v", repos)
	}
	if !found[filepath.Clean(wtPath)] {
		t.Fatalf("linked worktree missing from results (is .git-as-file handled?): %v", repos)
	}
}

// Scanning a nonexistent root should return an explicit error, not an empty
// list that looks like "no projects here".
func TestScanLocalProjectsRejectsMissingRoot(t *testing.T) {
	svc := GitService{}
	_, err := svc.ScanLocalProjects("/nonexistent/wgit-scan-root-xyz")
	if err == nil {
		t.Fatal("expected an error for a missing scan root")
	}
	if !strings.Contains(err.Error(), "扫描根目录不存在") {
		t.Fatalf("unexpected error wording: %v", err)
	}
}
