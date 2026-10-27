package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// initToggleHookRepo creates a throwaway repository and returns its hooks dir.
func initToggleHookRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return repo, filepath.Join(repo, ".git", "hooks")
}

// TestToggleGitHookRejectsInvalidNames guards against path traversal and
// absolute hook names escaping .git/hooks through ToggleGitHook.
func TestToggleGitHookRejectsInvalidNames(t *testing.T) {
	repo, hooksDir := initToggleHookRepo(t)

	// Canary paths a traversal or absolute name could otherwise reach.
	canaries := []string{
		filepath.Join(repo, "evil"),           // ../../evil resolved from .git/hooks
		filepath.Join(hooksDir, "..", "evil"), // one level up: .git/evil
		filepath.Join(hooksDir, `..\evil`),    // literal backslash file name
		filepath.Join(hooksDir, `..\evil.disabled`),
		filepath.Join(hooksDir, "   "),
		filepath.Join(repo, "evil.disabled"),
	}
	const absoluteCanary = "/tmp/evil"
	absoluteCanaryExisted := false
	if _, err := os.Stat(absoluteCanary); err == nil {
		absoluteCanaryExisted = true
	}

	names := []string{"", "   ", "../../evil", "/tmp/evil", `..\evil`, "pre-receive"}
	svc := GitService{}
	for _, name := range names {
		for _, enable := range []bool{true, false} {
			content := ""
			if enable {
				content = "#!/bin/sh\nexit 0\n"
			}
			err := svc.ToggleGitHook(repo, name, enable, content)
			if err == nil || !strings.Contains(err.Error(), "不支持的钩子名称") {
				t.Fatalf("ToggleGitHook(%q, enable=%v) = %v, want 不支持的钩子名称 error", name, enable, err)
			}
		}
	}

	for _, p := range canaries {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("canary %q was created for a rejected hook name (stat err = %v)", p, err)
		}
	}
	if !absoluteCanaryExisted {
		if _, err := os.Stat(absoluteCanary); !os.IsNotExist(err) {
			t.Fatalf("absolute canary %q was created (stat err = %v)", absoluteCanary, err)
		}
	}
}

// TestToggleGitHookValidHookStillToggles keeps the whitelisted enable/disable
// behavior intact after routing through resolveHookPaths.
func TestToggleGitHookValidHookStillToggles(t *testing.T) {
	repo, hooksDir := initToggleHookRepo(t)
	active := filepath.Join(hooksDir, "pre-commit")
	disabled := active + ".disabled"
	svc := GitService{}

	if err := svc.ToggleGitHook(repo, "pre-commit", true, "exit 0\n"); err != nil {
		t.Fatalf("enable with content: %v", err)
	}
	data, err := os.ReadFile(active)
	if err != nil {
		t.Fatalf("read active hook: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh\n") {
		t.Fatalf("content missing shebang prefix: %q", string(data))
	}
	info, err := os.Stat(active)
	if err != nil {
		t.Fatalf("stat active hook: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("active hook mode = %v, want 0755", info.Mode().Perm())
	}
	if _, err := os.Stat(disabled); !os.IsNotExist(err) {
		t.Fatalf("disabled copy must not exist while enabled, stat err = %v", err)
	}

	if err := svc.ToggleGitHook(repo, "pre-commit", false, ""); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("active hook must be gone after disable, stat err = %v", err)
	}
	if info, err := os.Stat(disabled); err != nil {
		t.Fatalf("disabled hook missing after disable: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("disabled hook lost its exec bit: %v", info.Mode().Perm())
	}
	// Disabling an already-disabled hook stays a no-op success.
	if err := svc.ToggleGitHook(repo, "pre-commit", false, ""); err != nil {
		t.Fatalf("second disable: %v", err)
	}

	if err := svc.ToggleGitHook(repo, "pre-commit", true, ""); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if info, err := os.Stat(active); err != nil {
		t.Fatalf("active hook missing after enable: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("re-enabled hook mode = %v, want 0755", info.Mode().Perm())
	}
	if _, err := os.Stat(disabled); !os.IsNotExist(err) {
		t.Fatalf("disabled copy must be gone after enable, stat err = %v", err)
	}
	if data, err := os.ReadFile(active); err != nil {
		t.Fatalf("read restored hook: %v", err)
	} else if !strings.HasPrefix(string(data), "#!/bin/sh\n") {
		t.Fatalf("restored hook lost its content: %q", string(data))
	}
}

// TestToggleGitHookEnableMissingHookIsClearError covers enabling a hook that
// exists neither as an active nor as a .disabled file without supplying a
// script: it must report a readable error instead of a raw chmod failure.
func TestToggleGitHookEnableMissingHookIsClearError(t *testing.T) {
	repo, hooksDir := initToggleHookRepo(t)
	active := filepath.Join(hooksDir, "pre-commit")
	disabled := active + ".disabled"
	// git init may pre-seed the hooks dir; make sure the target is truly absent.
	for _, p := range []string{active, disabled} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove %q: %v", p, err)
		}
	}

	err := (GitService{}).ToggleGitHook(repo, "pre-commit", true, "")
	if err == nil || !strings.Contains(err.Error(), "钩子不存在，无法启用") {
		t.Fatalf("enable without hook files = %v, want 钩子不存在，无法启用 error", err)
	}
	if _, statErr := os.Stat(active); !os.IsNotExist(statErr) {
		t.Fatalf("failed enable must not create %q (stat err = %v)", active, statErr)
	}
	if _, statErr := os.Stat(disabled); !os.IsNotExist(statErr) {
		t.Fatalf("failed enable must not create %q (stat err = %v)", disabled, statErr)
	}

	// Supplying script content is still allowed to create a brand-new hook.
	if err := (GitService{}).ToggleGitHook(repo, "pre-commit", true, "exit 0"); err != nil {
		t.Fatalf("enable with content must create the hook: %v", err)
	}
	data, err := os.ReadFile(active)
	if err != nil {
		t.Fatalf("hook not created: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/sh\n") {
		t.Fatalf("hook missing shebang prefix: %q", string(data))
	}
	info, err := os.Stat(active)
	if err != nil {
		t.Fatalf("stat created hook: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Fatalf("created hook mode = %v, want 0755", info.Mode().Perm())
	}
}
