package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPartialCloneAndHydration(t *testing.T) {
	dir := newNotesRepo(t)
	svc := GitService{}

	// 1. Regular repo should have IsPartialClone == false
	status, err := svc.GetPartialCloneStatus(dir)
	if err != nil {
		t.Fatalf("GetPartialCloneStatus failed: %v", err)
	}
	if status.IsPartialClone {
		t.Errorf("expected regular repo to not be partial clone")
	}

	// 2. Configure promisor remote
	runGitTest(t, dir, nil, "config", "remote.origin.promisor", "true")
	runGitTest(t, dir, nil, "config", "remote.origin.partialclonefilter", "blob:none")

	status, err = svc.GetPartialCloneStatus(dir)
	if err != nil {
		t.Fatalf("GetPartialCloneStatus after config failed: %v", err)
	}
	if !status.IsPartialClone {
		t.Errorf("expected repo to be detected as partial clone")
	}
	if status.PromisorRemote != "origin" {
		t.Errorf("expected PromisorRemote='origin', got '%s'", status.PromisorRemote)
	}
	if status.FilterSpec != "blob:none" {
		t.Errorf("expected FilterSpec='blob:none', got '%s'", status.FilterSpec)
	}

	// 3. Test HydrateBlobs
	testFile := filepath.Join(dir, "asset.bin")
	_ = os.WriteFile(testFile, []byte("large asset content"), 0644)
	runGitTest(t, dir, nil, "add", "asset.bin")
	runGitTest(t, dir, nil, "commit", "-m", "add asset")

	hydrateRes, err := svc.HydrateBlobs(dir, []string{"asset.bin"})
	if err != nil {
		t.Fatalf("HydrateBlobs failed: %v", err)
	}
	if !hydrateRes.Success {
		t.Errorf("expected HydrateBlobs to succeed")
	}
	if len(hydrateRes.HydratedPaths) != 1 || hydrateRes.HydratedPaths[0] != "asset.bin" {
		t.Errorf("expected HydratedPaths=['asset.bin'], got %v", hydrateRes.HydratedPaths)
	}
}
