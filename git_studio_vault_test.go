package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreCredentialSecretSecuresVaultAndRejectsCorruption(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	vault := filepath.Join(repo, ".git", "wgit_vault.json")
	if err := os.WriteFile(vault, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	res, err := svc.StoreCredentialSecret(repo, "GIT_TOKEN", "token-1", "supersecret123", "LOCAL_REPO")
	if err != nil {
		t.Fatalf("StoreCredentialSecret failed: %v", err)
	}
	info, err := os.Stat(vault)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("vault permissions must be tightened to 0600, got %o", info.Mode().Perm())
	}
	data, err := os.ReadFile(vault)
	if err != nil || strings.Contains(string(data), "supersecret123") {
		t.Fatalf("vault must store ciphertext only: %q err=%v", data, err)
	}
	if strings.Contains(res.MaskedValue, "supersecret123") {
		t.Fatalf("masked result leaked the secret: %+v", res)
	}

	if err := os.WriteFile(vault, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StoreCredentialSecret(repo, "GIT_TOKEN", "token-2", "anothersecret", "LOCAL_REPO"); err == nil {
		t.Fatal("corrupt vault must not be silently overwritten")
	}
	after, err := os.ReadFile(vault)
	if err != nil || string(after) != "not-json" {
		t.Fatalf("corrupt vault should stay untouched: %q err=%v", after, err)
	}
}
