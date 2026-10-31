package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// swallowTestRepo builds a temp repository with two commits so HEAD~1..HEAD and
// HEAD~1 are both addressable for the rebase kanban and external tool tests.
func swallowTestRepo(t *testing.T) string {
	t.Helper()
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "b.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "feat: second", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	return repo
}

// TestGetExternalToolConfigSurfacesBrokenRepo pins the propagated read failure
// for a non-repository (GetExternalToolConfig config-read guard) while keeping
// the benign "all tool keys unset" path on a healthy repository error-free.
func TestGetExternalToolConfigSurfacesBrokenRepo(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

	cfg, err := svc.GetExternalToolConfig(t.TempDir())
	if err == nil {
		t.Fatalf("broken repository must surface a read error instead of an empty config, got %+v", cfg)
	}
	if !strings.Contains(err.Error(), "read external tool config") {
		t.Fatalf("expected wrapped read external tool config error, got %v", err)
	}

	// A healthy repo with every tool key unset must keep succeeding: the
	// per-key swallows only mask the benign "key not configured" git exit.
	repo := swallowTestRepo(t)
	cfg, err = svc.GetExternalToolConfig(repo)
	if err != nil || cfg == nil {
		t.Fatalf("healthy repo with unset tool keys must not error, got cfg=%+v err=%v", cfg, err)
	}
}

// TestOpenExternalDiffUntrackedFileKeepsEmptyLeftSide keeps the preserved
// best-effort behavior in OpenExternalDiff: an untracked file has no index or
// HEAD version, so both `git show` lookups may fail and the diff must still
// launch with an empty left side instead of reporting an error.
func TestOpenExternalDiffUntrackedFileKeepsEmptyLeftSide(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := swallowTestRepo(t)

	outDir := t.TempDir()
	outLeft := filepath.Join(outDir, "untracked_left.txt")
	outRight := filepath.Join(outDir, "untracked_right.txt")
	script := filepath.Join(outDir, "untracked_diff.sh")
	body := fmt.Sprintf("#!/bin/sh\ncp \"$1\" %q\ncp \"$2\" %q\n", outLeft, outRight)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	customCmd := fmt.Sprintf("sh %q {left} {right}", script)
	if err := svc.SetExternalToolConfig(repo, "custom", "", customCmd, "", false); err != nil {
		t.Fatalf("SetExternalToolConfig failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenExternalDiff(repo, "new.txt", false, ""); err != nil {
		t.Fatalf("OpenExternalDiff on untracked file must not fail: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		left, leftErr := os.ReadFile(outLeft)
		right, rightErr := os.ReadFile(outRight)
		if leftErr == nil && rightErr == nil {
			if len(left) != 0 {
				t.Fatalf("untracked file must diff against an empty left side, got %q", left)
			}
			if string(right) != "new content\n" {
				t.Fatalf("right side must be the worktree content, got %q", right)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("custom diff sides = %q / %q (err=%v/%v), want \"\" / \"new content\\n\"",
				left, right, leftErr, rightErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestPreviewRebaseTopologySurfacesUnreadableCommit corrupts the tree object of
// the head commit: `git log` still lists it, but the per-commit `git show
// --name-only` read fails and must propagate instead of silently degrading the
// commit into "no files / low conflict risk".
func TestPreviewRebaseTopologySurfacesUnreadableCommit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := swallowTestRepo(t)

	treeHash, err := ExecGit(repo, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatalf("rev-parse HEAD^{tree} failed: %v", err)
	}
	treeHash = strings.TrimSpace(treeHash)
	objPath := filepath.Join(repo, ".git", "objects", treeHash[:2], treeHash[2:])
	// Git stores loose objects read-only, so make the file writable before
	// replacing its zlib payload with garbage.
	if err := os.Chmod(objPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objPath, []byte("corrupted object"), 0o644); err != nil {
		t.Fatal(err)
	}

	preview, err := svc.PreviewRebaseTopology(repo, "HEAD~1", "HEAD")
	if err == nil {
		t.Fatalf("unreadable commit must surface from the preview, got %+v", preview)
	}
	if !strings.Contains(err.Error(), "read changed files of commit") {
		t.Fatalf("expected wrapped per-commit read error, got %v", err)
	}
}

// TestSplitCommitSurgicallySurfacesStageFailure covers the kanban plan
// "save" path: a failing `git add` for a partition file must stop the split
// with the real cause instead of silently committing an empty partition.
func TestSplitCommitSurgicallySurfacesStageFailure(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := swallowTestRepo(t)

	commitHash, err := ExecGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD failed: %v", err)
	}
	res, err := svc.SplitCommitSurgically(repo, strings.TrimSpace(commitHash), []CommitPartition{
		{Subject: "feat: part 1", Files: []string{"missing.txt"}},
		{Subject: "feat: part 2", Files: []string{"b.txt"}},
	})
	if err != nil {
		t.Fatalf("stage failure must follow the result-object convention, got err=%v", err)
	}
	if res.Success {
		t.Fatalf("failed staging must not report success: %+v", res)
	}
	if !strings.Contains(res.Message, "Failed to stage file missing.txt for partition 1") {
		t.Fatalf("stage failure must carry the real cause, got %q", res.Message)
	}
}
