package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomActionsActivityLogsAndSigningStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := svc.ClearActivityLogs(); err != nil {
		t.Fatalf("ClearActivityLogs failed: %v", err)
	}
	if logs := svc.GetActivityLogs(); len(logs) != 0 {
		t.Fatalf("expected no activity logs, got %+v", logs)
	}
	if _, err := ExecGit(repo, "status", "--short"); err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	logs := svc.GetActivityLogs()
	if len(logs) == 0 || !strings.Contains(logs[len(logs)-1].Command, "status") {
		t.Fatalf("expected recorded status command, got %+v", logs)
	}

	actions := []CustomActionConfig{
		{ID: "act_echo", Name: "Echo", Command: "printf", Args: "custom-ok", WorkingDir: "", Icon: "E"},
		{ID: "act_pwd", Name: "PWD", Command: "pwd", Args: "", WorkingDir: "sub", Icon: "P"},
		{ID: "act_fail", Name: "Fail", Command: "false", Args: "", WorkingDir: "", Icon: "F"},
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveCustomActions(repo, actions); err != nil {
		t.Fatalf("SaveCustomActions failed: %v", err)
	}
	loaded, err := svc.GetCustomActions(repo)
	if err != nil || len(loaded) != len(actions) || loaded[0].ID != "act_echo" {
		t.Fatalf("GetCustomActions failed: %+v err=%v", loaded, err)
	}
	echoRes, err := svc.ExecuteCustomAction(repo, "act_echo")
	if err != nil || echoRes == nil || !echoRes.Success || echoRes.ExitCode != 0 || strings.TrimSpace(echoRes.Output) != "custom-ok" {
		t.Fatalf("ExecuteCustomAction echo failed: %+v err=%v", echoRes, err)
	}
	pwdRes, err := svc.ExecuteCustomAction(repo, "act_pwd")
	if err != nil || pwdRes == nil || !pwdRes.Success {
		t.Fatalf("ExecuteCustomAction pwd failed: %+v err=%v", pwdRes, err)
	}
	if filepath.Base(strings.TrimSpace(pwdRes.Output)) != "sub" {
		t.Fatalf("working directory not respected: %q", pwdRes.Output)
	}
	failRes, err := svc.ExecuteCustomAction(repo, "act_fail")
	if err != nil || failRes == nil || failRes.Success || failRes.ExitCode == 0 {
		t.Fatalf("expected failing action result, got %+v err=%v", failRes, err)
	}
	if _, err := svc.ExecuteCustomAction(repo, "missing-action"); err == nil {
		t.Fatal("expected error for unknown custom action")
	}

	actionPath := filepath.Join(repo, ".git", "wgit-actions.json")
	if err := os.WriteFile(actionPath, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	defaults, err := svc.GetCustomActions(repo)
	if err != nil || len(defaults) != 5 {
		t.Fatalf("expected default actions after corrupt config, got %d err=%v", len(defaults), err)
	}

	if err := svc.SetCustomConfigKey(repo, "gpg.format", "ssh", false); err != nil {
		t.Fatalf("set gpg.format failed: %v", err)
	}
	if err := svc.SetCustomConfigKey(repo, "user.signingkey", "ssh-ed25519 AAAATEST", false); err != nil {
		t.Fatalf("set signingkey failed: %v", err)
	}
	signing, err := svc.GetCommitSigningStatus(repo)
	if err != nil || signing == nil || signing.GpgSignEnabled || signing.GpgFormat != "ssh" || signing.SigningKey == "" {
		t.Fatalf("GetCommitSigningStatus failed: %+v err=%v", signing, err)
	}
	if signing.RecentCommits == nil || signing.AvailableSSHKeys == nil || signing.TotalChecked != 1 || signing.RecentCommits[0].SignatureStatus != "N" {
		t.Fatalf("unexpected signing audit: %+v", signing)
	}
	if signing.ComplianceRate != "0.0%" {
		t.Fatalf("unexpected compliance rate: %q", signing.ComplianceRate)
	}
}
