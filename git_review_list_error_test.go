package main

import "testing"

func TestListChangeReviewsRejectsInvalidRepoAndHandlesFirstRun(t *testing.T) {
	svc := GitService{}
	if _, err := svc.ListChangeReviews(t.TempDir()); err == nil {
		t.Fatal("expected error when listing review sessions outside a Git repository")
	}

	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	sessions, err := svc.ListChangeReviews(repo)
	if err != nil {
		t.Fatalf("first-run review list failed: %v", err)
	}
	if sessions == nil || len(sessions) != 0 {
		t.Fatalf("first run should return a non-nil empty list: %+v", sessions)
	}
}
