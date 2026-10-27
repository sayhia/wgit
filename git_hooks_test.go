package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := ExecGit(dir, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := ExecGit(dir, "config", "user.name", "Tester"); err != nil {
		t.Fatalf("config user.name: %v", err)
	}
	if _, err := ExecGit(dir, "config", "user.email", "tester@example.com"); err != nil {
		t.Fatalf("config user.email: %v", err)
	}
	return dir
}

func TestGitHookDetectFramework(t *testing.T) {
	svc := GitService{}

	t.Run("NativeRepo", func(t *testing.T) {
		repo := initTestRepo(t)
		info, err := svc.DetectHooksFramework(repo)
		if err != nil {
			t.Fatalf("DetectHooksFramework: %v", err)
		}
		if info.Type != "native" {
			t.Errorf("expected type 'native', got %q", info.Type)
		}
		if info.ConfigFile != "" {
			t.Errorf("expected empty configFile, got %q", info.ConfigFile)
		}
		if !strings.HasSuffix(filepath.ToSlash(info.HooksPath), ".git/hooks") {
			t.Errorf("unexpected hooksPath: %q", info.HooksPath)
		}
	})

	t.Run("LefthookConfig", func(t *testing.T) {
		repo := initTestRepo(t)
		cfgPath := filepath.Join(repo, "lefthook.yml")
		if err := os.WriteFile(cfgPath, []byte("pre-commit:\n  commands:\n    lint:\n      run: echo ok\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := svc.DetectHooksFramework(repo)
		if err != nil {
			t.Fatalf("DetectHooksFramework: %v", err)
		}
		if info.Type != "lefthook" {
			t.Errorf("expected type 'lefthook', got %q", info.Type)
		}
		if info.ConfigFile != cfgPath {
			t.Errorf("expected configFile %q, got %q", cfgPath, info.ConfigFile)
		}
	})

	t.Run("PreCommitConfig", func(t *testing.T) {
		repo := initTestRepo(t)
		cfgPath := filepath.Join(repo, ".pre-commit-config.yaml")
		if err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		info, err := svc.DetectHooksFramework(repo)
		if err != nil {
			t.Fatalf("DetectHooksFramework: %v", err)
		}
		if info.Type != "pre-commit" {
			t.Errorf("expected type 'pre-commit', got %q", info.Type)
		}
		if info.ConfigFile != cfgPath {
			t.Errorf("expected configFile %q, got %q", cfgPath, info.ConfigFile)
		}
	})

	t.Run("HuskyDir", func(t *testing.T) {
		repo := initTestRepo(t)
		huskyDir := filepath.Join(repo, ".husky")
		if err := os.MkdirAll(huskyDir, 0o755); err != nil {
			t.Fatal(err)
		}
		info, err := svc.DetectHooksFramework(repo)
		if err != nil {
			t.Fatalf("DetectHooksFramework: %v", err)
		}
		if info.Type != "husky" {
			t.Errorf("expected type 'husky', got %q", info.Type)
		}
		if info.ConfigFile != huskyDir {
			t.Errorf("expected configFile %q, got %q", huskyDir, info.ConfigFile)
		}
	})

	t.Run("CustomHooksPath", func(t *testing.T) {
		repo := initTestRepo(t)
		if _, err := ExecGit(repo, "config", "core.hooksPath", ".custom_hooks"); err != nil {
			t.Fatal(err)
		}
		info, err := svc.DetectHooksFramework(repo)
		if err != nil {
			t.Fatalf("DetectHooksFramework: %v", err)
		}
		if info.Type != "custom" {
			t.Errorf("expected type 'custom', got %q", info.Type)
		}
		if info.ConfigFile != ".custom_hooks" {
			t.Errorf("expected configFile '.custom_hooks', got %q", info.ConfigFile)
		}
		if !strings.HasSuffix(filepath.ToSlash(info.HooksPath), ".custom_hooks") {
			t.Errorf("expected hooksPath to end in .custom_hooks, got %q", info.HooksPath)
		}
	})
}

func TestGitHookInstallSampleAndRun(t *testing.T) {
	svc := GitService{}

	t.Run("InstallWithExistingSample", func(t *testing.T) {
		repo := initTestRepo(t)
		hooksDir := filepath.Join(repo, ".git", "hooks")
		_ = os.MkdirAll(hooksDir, 0o755)

		samplePath := filepath.Join(hooksDir, "pre-commit.sample")
		sampleContent := "#!/bin/sh\necho 'hello from sample'\nexit 0\n"
		if err := os.WriteFile(samplePath, []byte(sampleContent), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := svc.InstallSampleHook(repo, "pre-commit"); err != nil {
			t.Fatalf("InstallSampleHook failed: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "pre-commit")
		data, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatalf("failed to read installed hook: %v", err)
		}
		if string(data) != sampleContent {
			t.Errorf("installed hook content mismatch: got %q, want %q", string(data), sampleContent)
		}
		if runtime.GOOS != "windows" {
			info, _ := os.Stat(hookPath)
			if info.Mode()&0111 == 0 {
				t.Errorf("expected executable hook permissions, got %v", info.Mode())
			}
		}

		// Run the installed sample hook
		res, err := svc.RunGitHook(repo, "pre-commit", nil)
		if err != nil {
			t.Fatalf("RunGitHook failed: %v", err)
		}
		if !res.Success || res.ExitCode != 0 {
			t.Fatalf("RunGitHook expected success, got exitCode=%d success=%v", res.ExitCode, res.Success)
		}
		combined := res.Stdout + res.Stderr
		if !strings.Contains(combined, "hello from sample") {
			t.Errorf("expected output to contain 'hello from sample', got stdout=%q, stderr=%q", res.Stdout, res.Stderr)
		}
	})

	t.Run("InstallWithoutSampleFallback", func(t *testing.T) {
		repo := initTestRepo(t)
		hooksDir := filepath.Join(repo, ".git", "hooks")
		_ = os.MkdirAll(hooksDir, 0o755)

		// Ensure no sample file exists for custom-check
		_ = os.Remove(filepath.Join(hooksDir, "custom-check.sample"))

		if err := svc.InstallSampleHook(repo, "custom-check"); err != nil {
			t.Fatalf("InstallSampleHook failed: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "custom-check")
		data, err := os.ReadFile(hookPath)
		if err != nil {
			t.Fatalf("failed to read installed hook: %v", err)
		}
		if !strings.HasPrefix(string(data), "#!/bin/sh") {
			t.Errorf("expected shebang in default hook, got %q", string(data))
		}
	})

	t.Run("RunGitHookWithArgumentsAndFailureCode", func(t *testing.T) {
		repo := initTestRepo(t)
		hooksDir := filepath.Join(repo, ".git", "hooks")
		_ = os.MkdirAll(hooksDir, 0o755)

		hookPath := filepath.Join(hooksDir, "pre-push")
		script := "#!/bin/sh\necho \"args: $@\"\necho \"error detail\" >&2\nexit 7\n"
		if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}

		res, err := svc.RunGitHook(repo, "pre-push", []string{"origin", "main"})
		if err != nil {
			t.Fatalf("RunGitHook returned unexpected error: %v", err)
		}
		if res.Success {
			t.Errorf("expected failure, got success")
		}
		if res.ExitCode != 7 {
			t.Errorf("expected exitCode 7, got %d", res.ExitCode)
		}
		combined := res.Stdout + res.Stderr
		if !strings.Contains(combined, "origin main") {
			t.Errorf("expected output to contain 'origin main', got: %q", combined)
		}
		if !strings.Contains(res.Stderr, "error detail") {
			t.Errorf("expected stderr to contain 'error detail', got: %q", res.Stderr)
		}
	})

	t.Run("RunGitHookMissing", func(t *testing.T) {
		repo := initTestRepo(t)
		res, err := svc.RunGitHook(repo, "non-existent-hook", nil)
		if err == nil {
			t.Fatalf("expected error for missing hook, got res=%+v", res)
		}
	})
}

func TestHookNameRejectsPathTraversal(t *testing.T) {
	svc := GitService{}

	t.Run("ValidatorRules", func(t *testing.T) {
		valid := []string{"pre-commit", "my-custom-hook", "post_checkout", "hook123"}
		for _, name := range valid {
			if err := validHookName(name); err != nil {
				t.Errorf("expected %q to be valid, got %v", name, err)
			}
		}

		invalid := []string{
			"", "   ", "../../evil", "/tmp/evil", `..\evil`, `..\..\evil`,
			"a/b", `a\b`, "-x", "--upload-pack=evil", "..", ".",
			"pre commit", "pre\tcommit", "pre\ncommit", "pre.commit", "evil\x00",
		}
		for _, name := range invalid {
			if err := validHookName(name); err == nil {
				t.Errorf("expected %q to be rejected", name)
			}
		}
	})

	t.Run("RunGitHookRejectsTraversal", func(t *testing.T) {
		repo := initTestRepo(t)
		hooksDir := filepath.Join(repo, ".git", "hooks")

		cases := []struct {
			name   string
			canary string
		}{
			{"../../evil", filepath.Join(repo, "evil")},
			{`..\..\evil`, filepath.Join(repo, "evil")},
			{"/tmp/evil", filepath.Join(hooksDir, "tmp", "evil")},
			{"", ""},
			{"   ", ""},
		}
		for _, tc := range cases {
			res, err := svc.RunGitHook(repo, tc.name, nil)
			if err == nil {
				t.Fatalf("RunGitHook(%q): expected name validation error, got %+v", tc.name, res)
			}
			if !strings.Contains(err.Error(), "非法的钩子名称") {
				t.Errorf("RunGitHook(%q): expected name validation error, got %v", tc.name, err)
			}
			if tc.canary != "" {
				if _, statErr := os.Stat(tc.canary); !os.IsNotExist(statErr) {
					t.Errorf("RunGitHook(%q): canary %q exists or cannot be confirmed absent (err=%v)", tc.name, tc.canary, statErr)
				}
			}
		}
	})

	t.Run("InstallSampleHookRejectsTraversal", func(t *testing.T) {
		repo := initTestRepo(t)
		hooksDir := filepath.Join(repo, ".git", "hooks")

		cases := []struct {
			name   string
			canary string
		}{
			{"../../evil", filepath.Join(repo, "evil")},
			{`..\..\evil`, filepath.Join(repo, "evil")},
			{"/tmp/evil", filepath.Join(hooksDir, "tmp", "evil")},
			{"", ""},
			{"   ", ""},
		}
		for _, tc := range cases {
			err := svc.InstallSampleHook(repo, tc.name)
			if err == nil {
				t.Fatalf("InstallSampleHook(%q): expected name validation error, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "非法的钩子名称") {
				t.Errorf("InstallSampleHook(%q): expected name validation error, got %v", tc.name, err)
			}
			if tc.canary != "" {
				if _, statErr := os.Stat(tc.canary); !os.IsNotExist(statErr) {
					t.Errorf("InstallSampleHook(%q): canary %q exists or cannot be confirmed absent (err=%v)", tc.name, tc.canary, statErr)
				}
			}
		}

		entries, err := os.ReadDir(hooksDir)
		if err != nil {
			t.Fatalf("read hooks dir: %v", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				t.Errorf("unexpected directory %q created inside hooks dir", entry.Name())
			}
		}
	})

	t.Run("AcceptsCustomHookName", func(t *testing.T) {
		repo := initTestRepo(t)
		installed := filepath.Join(repo, ".git", "hooks", "my-custom-hook")

		if err := svc.InstallSampleHook(repo, "my-custom-hook"); err != nil {
			t.Fatalf("InstallSampleHook with valid custom name failed: %v", err)
		}
		if _, err := os.Stat(installed); err != nil {
			t.Fatalf("expected custom hook at %q: %v", installed, err)
		}

		res, err := svc.RunGitHook(repo, "my-custom-hook", nil)
		if err != nil {
			if strings.Contains(err.Error(), "非法的钩子名称") {
				t.Fatalf("valid hook name rejected by validator: %v", err)
			}
			// Environment-dependent "not found or failed to execute" is acceptable here;
			// the point is that the name itself passes validation.
			t.Logf("RunGitHook returned environment error for valid hook name (accepted): %v", err)
			return
		}
		if res == nil {
			t.Fatal("RunGitHook returned nil result without error")
		}
	})
}

func TestGitHookCommitAdvancedNoVerify(t *testing.T) {
	svc := GitService{}
	repo := initTestRepo(t)

	// Create and stage a test file
	filePath := filepath.Join(repo, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("initial content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "add", "tracked.txt"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	// Install a pre-commit hook that rejects every commit
	hooksDir := filepath.Join(repo, ".git", "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	failingHook := "#!/bin/sh\necho 'intercepted by pre-commit' >&2\nexit 1\n"
	if err := os.WriteFile(hookPath, []byte(failingHook), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. Commit with noVerify = false should fail
	err := svc.CommitAdvanced(repo, "normal commit", false, false)
	if err == nil {
		t.Fatal("expected commit to fail due to pre-commit hook, but succeeded")
	}

	// 2. Commit with noVerify = true should bypass the pre-commit hook
	err = svc.CommitAdvanced(repo, "bypassed commit", false, true)
	if err != nil {
		t.Fatalf("expected commit to succeed with --no-verify, but got error: %v", err)
	}

	// Verify commit log
	logOut, err := ExecGit(repo, "log", "-1", "--pretty=%s")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(logOut) != "bypassed commit" {
		t.Errorf("expected commit message 'bypassed commit', got %q", strings.TrimSpace(logOut))
	}

	// 3. Amend with noVerify = true
	if err := os.WriteFile(filePath, []byte("updated content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecGit(repo, "add", "tracked.txt"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	err = svc.CommitAdvanced(repo, "amended bypassed commit", true, true)
	if err != nil {
		t.Fatalf("expected amend to succeed with --no-verify, but got: %v", err)
	}

	logOut, err = ExecGit(repo, "log", "-1", "--pretty=%s")
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if strings.TrimSpace(logOut) != "amended bypassed commit" {
		t.Errorf("expected amended commit message 'amended bypassed commit', got %q", strings.TrimSpace(logOut))
	}
}
