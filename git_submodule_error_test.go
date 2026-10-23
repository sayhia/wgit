package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSubmodulesRejectsInvalidRepoAndReturnsEmptySlice(t *testing.T) {
	svc := GitService{}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitmodules"), []byte("[submodule \"x\"]\n\tpath = x\n\turl = ./x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetSubmodules(dir); err == nil {
		t.Fatal("expected error when reading submodules outside a Git repository")
	}

	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	list, err := svc.GetSubmodules(repo)
	if err != nil {
		t.Fatalf("GetSubmodules in a valid repository failed: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("expected a non-nil empty submodule list, got %+v", list)
	}
}
