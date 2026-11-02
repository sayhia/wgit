package main

import (
	"path/filepath"
	"testing"
)

// addRecentRepoTestRepo builds a throwaway repository with one commit.
func addRecentRepoTestRepo(t *testing.T) string {
	t.Helper()
	return initClosedLoopRepo(t)
}

// AddReposToRecent writes the real config file, so every test restores the
// developer's list afterwards — TestMain only stubs recordRecentRepo, which
// this path deliberately does not use.
func withRestoredConfig(t *testing.T) {
	t.Helper()
	original := LoadConfig()
	t.Cleanup(func() { _ = SaveConfig(original) })
}

func TestAddReposToRecentRecordsBatch(t *testing.T) {
	withRestoredConfig(t)
	svc := GitService{}

	a := addRecentRepoTestRepo(t)
	b := addRecentRepoTestRepo(t)

	list, err := svc.AddReposToRecent([]string{a, b})
	if err != nil {
		t.Fatalf("AddReposToRecent failed: %v", err)
	}

	found := map[string]bool{}
	for _, p := range list {
		found[filepath.Clean(p)] = true
	}
	if !found[filepath.Clean(a)] || !found[filepath.Clean(b)] {
		t.Fatalf("both repos should be recorded, got %v", list)
	}
}

func TestAddReposToRecentLeavesLastRepoAlone(t *testing.T) {
	withRestoredConfig(t)
	cfg := LoadConfig()
	cfg.LastRepo = "/some/previously/opened/repo"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	repo := addRecentRepoTestRepo(t)
	if _, err := (GitService{}).AddReposToRecent([]string{repo}); err != nil {
		t.Fatalf("failed: %v", err)
	}

	// Adding from the scanner must not pretend the user opened the repository.
	if got := LoadConfig().LastRepo; got != "/some/previously/opened/repo" {
		t.Fatalf("LastRepo changed to %q", got)
	}
}

func TestAddReposToRecentSkipsInvalidPaths(t *testing.T) {
	withRestoredConfig(t)
	svc := GitService{}
	repo := addRecentRepoTestRepo(t)

	list, err := svc.AddReposToRecent([]string{"/does/not/exist", repo, ""})
	if err != nil {
		t.Fatalf("one stale path must not fail the batch: %v", err)
	}
	if len(list) == 0 || filepath.Clean(list[0]) != filepath.Clean(repo) {
		t.Fatalf("valid repo should lead the list, got %v", list)
	}
}

func TestAddReposToRecentRejectsEmptyBatch(t *testing.T) {
	withRestoredConfig(t)
	if _, err := (GitService{}).AddReposToRecent(nil); err == nil {
		t.Fatal("expected an error for an empty batch")
	}
	if _, err := (GitService{}).AddReposToRecent([]string{"", "  "}); err == nil {
		t.Fatal("expected an error when every path is blank")
	}
}

func TestAddReposToRecentDeduplicates(t *testing.T) {
	withRestoredConfig(t)
	repo := addRecentRepoTestRepo(t)

	list, err := (GitService{}).AddReposToRecent([]string{repo, repo, repo + "/"})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	seen := 0
	for _, p := range list {
		if filepath.Clean(p) == filepath.Clean(repo) {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("expected the repo once, found %d times in %v", seen, list)
	}
}
