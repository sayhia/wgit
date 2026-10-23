package main

import "testing"

func TestScanHistoryLargeBlobsRejectsNonRepoAndHandlesEmptyRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.ScanHistoryLargeBlobs(t.TempDir(), 5); err == nil {
		t.Fatal("expected error when scanning history outside a Git repository")
	}

	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	blobs, err := svc.ScanHistoryLargeBlobs(repo, 5)
	if err != nil {
		t.Fatalf("empty repository scan should succeed: %v", err)
	}
	if blobs == nil || len(blobs) != 0 {
		t.Fatalf("empty repository should return a non-nil empty slice: %+v", blobs)
	}
}
