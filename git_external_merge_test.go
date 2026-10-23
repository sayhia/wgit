package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenExternalMergeUsesCustomCommand(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	writeCommit := func(content, message string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "conflict.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, "conflict.txt"); err != nil {
			t.Fatal(err)
		}
		if err := svc.Commit(repo, message, false); err != nil {
			t.Fatal(err)
		}
	}
	writeCommit("base\n", "chore: base")
	baseBranch, err := ExecGit(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	baseBranch = strings.TrimSpace(baseBranch)
	if _, err := ExecGit(repo, "checkout", "-b", "side"); err != nil {
		t.Fatal(err)
	}
	writeCommit("theirs\n", "feat: side")
	if _, err := ExecGit(repo, "checkout", baseBranch); err != nil {
		t.Fatal(err)
	}
	writeCommit("ours\n", "feat: ours")
	if _, err := ExecGit(repo, "merge", "side"); err == nil {
		t.Fatal("expected merge conflict")
	}

	outDir := t.TempDir()
	captured := map[string]string{
		"theirs": filepath.Join(outDir, "theirs.txt"),
		"ours":   filepath.Join(outDir, "ours.txt"),
		"base":   filepath.Join(outDir, "base.txt"),
	}
	script := filepath.Join(outDir, "merge.sh")
	scriptBody := fmt.Sprintf("#!/bin/sh\ncp \"$1\" %q\ncp \"$2\" %q\ncp \"$3\" %q\nprintf 'resolved\\n' > \"$4\"\n",
		captured["theirs"], captured["ours"], captured["base"])
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatal(err)
	}
	customCmd := fmt.Sprintf("sh %q {theirs} {ours} {base} {result}", script)
	if err := svc.SetExternalToolConfig(repo, "", "custom", "", customCmd, false); err != nil {
		t.Fatalf("SetExternalToolConfig failed: %v", err)
	}
	if err := svc.OpenExternalMerge(repo, "conflict.txt", ""); err != nil {
		t.Fatalf("OpenExternalMerge(custom) failed: %v", err)
	}

	resultPath := filepath.Join(repo, "conflict.txt")
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, readErr := os.ReadFile(resultPath)
		if readErr == nil && string(data) == "resolved\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("custom merge command did not write the result: %q err=%v", data, readErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for stage, want := range map[string]string{"theirs": "theirs\n", "ours": "ours\n", "base": "base\n"} {
		data, err := os.ReadFile(captured[stage])
		if err != nil || string(data) != want {
			t.Fatalf("custom merge %s input = %q (err=%v), want %q", stage, data, err, want)
		}
	}

	if err := svc.SetExternalToolConfig(repo, "", "custom", "", "", false); err != nil {
		t.Fatal(err)
	}
	if err := svc.OpenExternalMerge(repo, "conflict.txt", ""); err == nil || !strings.Contains(err.Error(), "自定义合并命令为空") {
		t.Fatalf("empty custom merge command must be rejected: %v", err)
	}
}
