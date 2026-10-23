package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureCommitSigningPropagatesWriteFailures(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

	if _, err := svc.ConfigureCommitSigning(t.TempDir(), true, "key", "ssh"); err == nil {
		t.Fatal("expected error outside a Git repository")
	}

	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if _, err := svc.ConfigureCommitSigning(repo, true, "~/.ssh/id_ed25519.pub", "ssh"); err == nil {
		t.Fatal("config write failure must not be reported as signing enabled")
	}
	if err := os.Chmod(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := svc.ConfigureCommitSigning(repo, true, "~/.ssh/id_ed25519.pub", "ssh")
	if err != nil || res == nil || !res.Success {
		t.Fatalf("ConfigureCommitSigning failed: %+v err=%v", res, err)
	}
	for key, want := range map[string]string{
		"gpg.format":      "ssh",
		"user.signingkey": "~/.ssh/id_ed25519.pub",
		"commit.gpgsign":  "true",
	} {
		got, err := ExecGit(repo, "config", "--get", key)
		if err != nil || strings.TrimSpace(got) != want {
			t.Fatalf("config %s = %q (err=%v), want %q", key, strings.TrimSpace(got), err, want)
		}
	}
}
