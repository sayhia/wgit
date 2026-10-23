package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenExternalDiffRejectsInvalidRepoAndStagedReadFailure(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), ".gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}

	err := svc.OpenExternalDiff(t.TempDir(), "a.txt", false, "")
	if err == nil || !strings.Contains(err.Error(), "不是有效的 Git 仓库") {
		t.Fatalf("invalid repository must be rejected before launching a tool: %v", err)
	}

	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("head\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	runDiff := func(staged bool, wantLeft, wantRight string) {
		t.Helper()
		label := "unstaged"
		if staged {
			label = "staged"
		}
		outLeft := filepath.Join(outDir, label+"_left.txt")
		outRight := filepath.Join(outDir, label+"_right.txt")
		script := filepath.Join(outDir, label+"_diff.sh")
		body := fmt.Sprintf("#!/bin/sh\ncp \"$1\" %q\ncp \"$2\" %q\n", outLeft, outRight)
		if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		customCmd := fmt.Sprintf("sh %q {left} {right}", script)
		if err := svc.SetExternalToolConfig(repo, "custom", "", customCmd, "", false); err != nil {
			t.Fatal(err)
		}
		if err := svc.OpenExternalDiff(repo, "a.txt", staged, ""); err != nil {
			t.Fatalf("OpenExternalDiff(staged=%v) failed: %v", staged, err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			left, leftErr := os.ReadFile(outLeft)
			right, rightErr := os.ReadFile(outRight)
			if leftErr == nil && rightErr == nil && string(left) == wantLeft && string(right) == wantRight {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("custom diff %s sides = %q / %q (err=%v/%v), want %q / %q",
					label, left, right, leftErr, rightErr, wantLeft, wantRight)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	runDiff(true, "head\n", "staged\n")
	runDiff(false, "staged\n", "work\n")

	indexPath := filepath.Join(repo, ".git", "index")
	if err := os.Chmod(indexPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(indexPath, 0o644) })
	err = svc.OpenExternalDiff(repo, "a.txt", true, "")
	if err == nil || !strings.Contains(err.Error(), "读取暂存区文件内容失败") {
		t.Fatalf("unreadable staged content must surface an error: %v", err)
	}
	if err := os.Chmod(indexPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetExternalToolConfig(repo, "custom", "", "", "", false); err != nil {
		t.Fatal(err)
	}
	err = svc.OpenExternalDiff(repo, "a.txt", false, "")
	if err == nil || !strings.Contains(err.Error(), "自定义比对命令为空") {
		t.Fatalf("empty custom diff command must be rejected: %v", err)
	}
}
