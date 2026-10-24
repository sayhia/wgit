package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectCredentialVaultReportsBrokenVault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "wgit_vault.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	res, err := (GitService{}).InspectCredentialVault(repo)
	if err != nil {
		t.Fatalf("InspectCredentialVault failed: %v", err)
	}
	if !strings.Contains(res.Message, "损坏") || !strings.Contains(res.Message, "未纳入") {
		t.Fatalf("broken vault must be reported, got %q", res.Message)
	}
}

func TestCredentialVaultMasksTokenStyleURLs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "remote.origin.url", "https://ghp_supersecret@github.com/acme/private.git")
	run("config", "remote.leak.url", "https://github.com/acme/repo.git?access_token=querysecret")

	res, err := (GitService{}).InspectCredentialVault(repo)
	if err != nil {
		t.Fatalf("InspectCredentialVault failed: %v", err)
	}
	if res.CriticalExposures < 2 {
		t.Fatalf("expected both credential URLs to be flagged: %+v", res)
	}
	foundUserInfo, foundQuery := false, false
	for _, item := range res.Items {
		if !strings.Contains(item.MaskedValue, "supersecret") && !strings.Contains(item.MaskedValue, "querysecret") {
			if strings.Contains(item.MaskedValue, "***") {
				foundUserInfo = foundUserInfo || strings.Contains(item.MaskedValue, "***@")
				foundQuery = foundQuery || strings.Contains(item.MaskedValue, "access_token=***")
			}
			continue
		}
		t.Fatalf("secret leaked in masked value: %+v", item)
	}
	if !foundUserInfo || !foundQuery {
		t.Fatalf("expected userinfo and query secrets to be masked: %+v", res.Items)
	}
	if strings.Contains(res.Summary+res.Message, "supersecret") || strings.Contains(res.Summary+res.Message, "querysecret") {
		t.Fatalf("secret leaked in summary: %q %q", res.Summary, res.Message)
	}
}
