package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runReleasePlanGit runs a real git command with fully isolated config: the
// global/system config is ignored and identity/signing are supplied per command,
// so the test never depends on the machine's ~/.gitconfig.
func runReleasePlanGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=Release Plan Test",
		"-c", "user.email=release-plan@wgit.test",
		"-c", "commit.gpgsign=false",
		"-c", "tag.gpgsign=false",
		"-c", "init.defaultBranch=main",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// initReleasePlanRepo creates a repository on main with a single commit.
func initReleasePlanRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runReleasePlanGit(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("release plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runReleasePlanGit(t, dir, "add", "-A")
	runReleasePlanGit(t, dir, "commit", "-m", "feat: initial release plan subject")
	return dir
}

// findCheck returns the first check with the given key, or nil.
func findCheck(plan *ReleasePlan, key string) *ReleaseCheck {
	for i := range plan.Checks {
		if plan.Checks[i].Key == key {
			return &plan.Checks[i]
		}
	}
	return nil
}

func TestGetReleasePlanDetachedHead(t *testing.T) {
	repo := initReleasePlanRepo(t)
	runReleasePlanGit(t, repo, "checkout", "--detach")

	plan, err := (GitService{}).GetReleasePlan(repo)
	if err != nil {
		t.Fatalf("GetReleasePlan on detached HEAD failed: %v", err)
	}
	if plan.Branch != "HEAD (detached)" {
		t.Fatalf("expected branch %q, got %q", "HEAD (detached)", plan.Branch)
	}
	check := findCheck(plan, "branch")
	if check == nil {
		t.Fatalf("branch check missing: %+v", plan.Checks)
	}
	if check.Status != "warn" {
		t.Fatalf("detached HEAD must warn (not fail), got %+v", check)
	}
	if check.DetailKey != "release.check.branch.detail.other" || check.Blocking {
		t.Fatalf("detached HEAD check must stay non-blocking 'other', got %+v", check)
	}
	if check.Params["branch"] != "HEAD (detached)" {
		t.Fatalf("detached HEAD detail should name the detached state, got %+v", check.Params)
	}
	if !plan.Ready {
		t.Fatalf("detached HEAD must not block the release: %+v", plan.Checks)
	}
}

func TestGetReleasePlanUnreadableBranch(t *testing.T) {
	// A plain temp dir is not a git repository, so reading HEAD fails. That must
	// surface as UNKNOWN/fail instead of being reported as a detached HEAD.
	dir := t.TempDir()

	plan, err := (GitService{}).GetReleasePlan(dir)
	if err != nil {
		t.Fatalf("GetReleasePlan should still return a plan on a read failure: %v", err)
	}
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	if plan.Branch != "UNKNOWN" {
		t.Fatalf("expected branch %q for an unreadable HEAD, got %q", "UNKNOWN", plan.Branch)
	}
	check := findCheck(plan, "branch")
	if check == nil {
		t.Fatalf("branch check missing: %+v", plan.Checks)
	}
	if check.Status != "fail" || !check.Blocking {
		t.Fatalf("unreadable branch must block the release, got %+v", check)
	}
	if check.DetailKey != "release.check.branch.detail.unknown" {
		t.Fatalf("unexpected detail key: %+v", check)
	}
	if strings.TrimSpace(check.Params["error"]) == "" {
		t.Fatalf("expected the read error to be carried in params: %+v", check)
	}
	if strings.TrimSpace(check.Detail) == "" || !strings.Contains(check.Detail, check.Params["error"]) {
		t.Fatalf("detail should render the read error, got %q", check.Detail)
	}
	if plan.Ready {
		t.Fatalf("an unreadable branch must not produce a ready plan: %+v", plan.Checks)
	}
}

func TestGetReleasePlanNormalBranch(t *testing.T) {
	repo := initReleasePlanRepo(t)

	plan, err := (GitService{}).GetReleasePlan(repo)
	if err != nil {
		t.Fatalf("GetReleasePlan failed: %v", err)
	}
	if plan.Branch != "main" {
		t.Fatalf("expected branch %q, got %q", "main", plan.Branch)
	}
	check := findCheck(plan, "branch")
	if check == nil {
		t.Fatalf("branch check missing: %+v", plan.Checks)
	}
	if check.Status != "pass" || check.DetailKey != "release.check.branch.detail.ok" {
		t.Fatalf("main must be a passing release line, got %+v", check)
	}
	if check.Params["branch"] != "main" {
		t.Fatalf("unexpected branch param: %+v", check.Params)
	}
}
