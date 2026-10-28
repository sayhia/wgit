package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestGitAliasesInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-aliases-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v, out: %s", err, string(out))
	}

	svc := GitService{}

	// 1. Set local alias
	err = svc.SetGitAlias(tempDir, "local", "st", "status -s")
	if err != nil {
		t.Fatalf("SetGitAlias failed: %v", err)
	}

	err = svc.SetGitAlias(tempDir, "local", "co", "checkout")
	if err != nil {
		t.Fatalf("SetGitAlias failed: %v", err)
	}

	// 2. Read aliases
	aliases, err := svc.GetGitAliases(tempDir)
	if err != nil {
		t.Fatalf("GetGitAliases failed: %v", err)
	}

	var foundSt, foundCo bool
	for _, a := range aliases {
		if a.Scope == "local" && a.Key == "st" && a.Command == "status -s" {
			foundSt = true
		}
		if a.Scope == "local" && a.Key == "co" && a.Command == "checkout" {
			foundCo = true
		}
	}

	if !foundSt || !foundCo {
		t.Fatalf("expected to find local aliases 'st' and 'co', got: %+v", aliases)
	}

	// 3. Validation errors
	if err := svc.SetGitAlias(tempDir, "local", "bad key!", "log"); err == nil {
		t.Fatal("expected error for invalid alias key with special characters")
	}
	if err := svc.SetGitAlias(tempDir, "local", "empty", "   "); err == nil {
		t.Fatal("expected error for empty command")
	}

	// 4. Delete alias
	err = svc.DeleteGitAlias(tempDir, "local", "st")
	if err != nil {
		t.Fatalf("DeleteGitAlias failed: %v", err)
	}

	aliasesAfter, err := svc.GetGitAliases(tempDir)
	if err != nil {
		t.Fatalf("GetGitAliases after delete failed: %v", err)
	}
	for _, a := range aliasesAfter {
		if a.Scope == "local" && a.Key == "st" {
			t.Fatal("alias 'st' was not deleted")
		}
	}
}
