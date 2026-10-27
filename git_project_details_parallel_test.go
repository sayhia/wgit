package main

import (
	"os"
	"path/filepath"
	"testing"
)

// seedRepo initialises a git repository at dir with one commit so that branch,
// last-commit and dirty-state detection all have real data to read.
func seedRepo(t *testing.T, svc GitService, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if _, err := svc.InitRepo(dir); err != nil {
		t.Fatalf("InitRepo %s: %v", dir, err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v in %s: %v (%s)", args, dir, err, out)
		}
	}
	file := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(file, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	if out, err := ExecGit(dir, "add", "-A"); err != nil {
		t.Fatalf("git add in %s: %v (%s)", dir, err, out)
	}
	if out, err := ExecGit(dir, "commit", "-m", "chore: seed"); err != nil {
		t.Fatalf("git commit in %s: %v (%s)", dir, err, out)
	}
}

// TestGetProjectsDetailsPreservesInputOrder guards the parallel project scan:
// results must stay in the exact order the caller supplied even though the
// per-project git work now runs concurrently, and each item must keep its
// pre-parallelisation semantics (valid repos carry a branch, plain dirs are
// invalid, current-path marking is per item).
func TestGetProjectsDetailsPreservesInputOrder(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()

	repoDirs := make([]string, 0, 3)
	for _, name := range []string{"beta-repo", "alpha-repo", "gamma-repo"} {
		dir := filepath.Join(parent, name)
		seedRepo(t, svc, dir)
		repoDirs = append(repoDirs, dir)
	}

	plainDirs := make([]string, 0, 2)
	for _, name := range []string{"notes-plain", "scratch-plain"} {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		plainDirs = append(plainDirs, dir)
	}

	// Deliberately non-alphabetical and interleaved, so a regression that sorts
	// by path (or appends in completion order) cannot pass by accident.
	input := []string{repoDirs[0], plainDirs[0], repoDirs[2], repoDirs[1], plainDirs[1]}
	active := repoDirs[2]

	details, err := svc.GetProjectsDetails(input, active)
	if err != nil {
		t.Fatalf("GetProjectsDetails failed: %v", err)
	}
	if len(details) != len(input) {
		t.Fatalf("expected %d details, got %d: %+v", len(input), len(details), details)
	}

	for i, want := range input {
		if got := details[i].Path; got != filepath.Clean(want) {
			t.Fatalf("detail[%d].Path = %q, want %q (order must match input)", i, got, filepath.Clean(want))
		}
		if got, wantName := details[i].Name, filepath.Base(want); got != wantName {
			t.Fatalf("detail[%d].Name = %q, want %q", i, got, wantName)
		}
		if got, wantCurrent := details[i].IsCurrent, want == active; got != wantCurrent {
			t.Fatalf("detail[%d] (%s).IsCurrent = %v, want %v", i, want, got, wantCurrent)
		}
	}

	repos := make(map[string]bool, len(repoDirs))
	for _, dir := range repoDirs {
		repos[filepath.Clean(dir)] = true
	}

	for i, item := range details {
		if repos[filepath.Clean(input[i])] {
			if !item.IsValid {
				t.Fatalf("detail[%d] (%s) must be a valid repo: %+v", i, input[i], item)
			}
			if item.CurrentBranch == "" {
				t.Fatalf("detail[%d] (%s) must report a branch: %+v", i, input[i], item)
			}
			if item.LastCommitHash == "" || item.LastCommitSubject == "" {
				t.Fatalf("detail[%d] (%s) must report the last commit: %+v", i, input[i], item)
			}
			if item.IsDirty || item.DirtyCount != 0 || item.StatusWarning != "" {
				t.Fatalf("detail[%d] (%s) must be a clean repo: %+v", i, input[i], item)
			}
			if item.AheadCount != 0 || item.BehindCount != 0 {
				t.Fatalf("detail[%d] (%s) has no upstream, ahead/behind must stay zero: %+v", i, input[i], item)
			}
			if item.EngineType == "" {
				t.Fatalf("detail[%d] (%s) must keep engine detection result: %+v", i, input[i], item)
			}
			continue
		}

		if item.IsValid {
			t.Fatalf("detail[%d] (%s) is a plain directory and must be invalid: %+v", i, input[i], item)
		}
		if item.CurrentBranch != "" || item.LastCommitHash != "" || item.StatusWarning != "" {
			t.Fatalf("detail[%d] (%s) plain directory must not carry repo fields: %+v", i, input[i], item)
		}
		if item.EngineType == "" {
			t.Fatalf("detail[%d] (%s) must keep engine detection result: %+v", i, input[i], item)
		}
	}
}

// TestGetProjectsDetailsSkipsBlankPathsInOrder pins the pre-filter semantics:
// blank entries never produce a result slot, and the surviving items keep the
// caller's relative order.
func TestGetProjectsDetailsSkipsBlankPathsInOrder(t *testing.T) {
	svc := GitService{}
	parent := t.TempDir()

	dirs := make([]string, 0, 3)
	for _, name := range []string{"second", "first", "third"} {
		dir := filepath.Join(parent, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		dirs = append(dirs, dir)
	}

	input := []string{"", dirs[0], "   ", dirs[1], dirs[2]}
	details, err := svc.GetProjectsDetails(input, "")
	if err != nil {
		t.Fatalf("GetProjectsDetails failed: %v", err)
	}
	if len(details) != 3 {
		t.Fatalf("blank paths must be skipped, expected 3 details, got %d: %+v", len(details), details)
	}
	for i, want := range dirs {
		if got := details[i].Path; got != want {
			t.Fatalf("detail[%d].Path = %q, want %q", i, got, want)
		}
	}
}
