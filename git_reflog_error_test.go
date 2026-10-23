package main

import (
	"testing"
)

func TestGetReflogRejectsNonRepoAndHandlesEmptyRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.GetReflog(t.TempDir(), 10); err == nil {
		t.Fatal("expected error when reading reflog outside a Git repository")
	}

	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	items, err := svc.GetReflog(repo, 10)
	if err != nil {
		t.Fatalf("empty repository should return an empty reflog, not an error: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("empty repository reflog must be a non-nil empty slice: %+v", items)
	}
}
