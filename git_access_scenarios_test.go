package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file complements the unit-level gate tests in git_access_test.go with
// scenario coverage: real temporary repositories driven through the same RPCs
// the frontend calls, proving the G1+F2 role gates never reject a legitimate
// workflow. The unit tests pin "viewer is denied"; these pin "operator and
// admin still work end to end".

// scenarioAccessConfig plants an isolated HOME whose config.json carries the
// given access role plus one non-default feature flag, so both the role gates
// and a read RPC (GetAppFlags) have deterministic content to return.
func scenarioAccessConfig(t *testing.T, role string) {
	t.Helper()
	writeFeatureFlagsConfigFile(t, `{"theme":"dark","access":{"role":"`+role+`"},"flags":{"reviewFlow":true},"schemaVersion":2}`)
}

// newAccessScenarioRepo builds a real throwaway repository with one commit on
// the default branch, following the fixture pattern of
// git_core_workflow_test.go (InitRepo + explicit identity + a staged commit).
func newAccessScenarioRepo(t *testing.T, svc GitService) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "access-scenario")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	if _, err := ExecGit(repo, "config", "user.name", "Scenario Tester"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if _, err := ExecGit(repo, "config", "user.email", "scenario@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}
	if err := svc.StageFile(repo, "hello.txt"); err != nil {
		t.Fatalf("StageFile: %v", err)
	}
	if err := svc.Commit(repo, "feat: scenario base", false); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return repo
}

// assertBusinessFailure pins the distinction this file exists for: the call got
// past the role gate and failed for an ordinary reason, so the error must not
// be the forbidden wire message.
func assertBusinessFailure(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s unexpectedly succeeded, want a business failure", label)
	}
	if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
		t.Fatalf("%s failed with a forbidden gate error %q, want an ordinary business failure (the gate must let this role through)", label, err)
	}
}

// TestAccessScenarioViewerReadOnlyRPCs drives a server-mode viewer through the
// read-only surfaces on a real repository. Every call must succeed with a
// non-degenerate result; the final gated call proves the mode really is on by
// rejecting a mutating RPC with the exact viewer forbidden message.
func TestAccessScenarioViewerReadOnlyRPCs(t *testing.T) {
	setServerMode(t, true)
	scenarioAccessConfig(t, "viewer")
	setRoleEnv(t, "", false)

	svc := GitService{}
	repo := newAccessScenarioRepo(t, svc)

	// 1. GetRepoStatus: a committed repo is clean with a resolvable HEAD.
	status, err := svc.GetRepoStatus(repo)
	if err != nil {
		t.Fatalf("viewer GetRepoStatus: %v", err)
	}
	if !status.IsClean || status.CurrentBranch == "" || status.HeadCommit == "" {
		t.Fatalf("viewer GetRepoStatus = %+v, want a clean status with a branch and a HEAD commit", status)
	}

	// 2. GetCommits: the history view shows the seeded commit intact.
	commits, err := svc.GetCommits(repo, 10, "")
	if err != nil {
		t.Fatalf("viewer GetCommits: %v", err)
	}
	if len(commits) != 1 || commits[0].Subject != "feat: scenario base" || commits[0].Hash == "" {
		t.Fatalf("viewer GetCommits = %+v, want exactly the seeded commit with subject and hash", commits)
	}

	// 3. GetBranches: the default branch is listed as the current one.
	branches, err := svc.GetBranches(repo)
	if err != nil {
		t.Fatalf("viewer GetBranches: %v", err)
	}
	var current []BranchInfo
	for _, b := range branches {
		if b.IsCurrent {
			current = append(current, b)
		}
	}
	if len(current) != 1 || current[0].Name == "" || current[0].CommitHash == "" {
		t.Fatalf("viewer GetBranches = %+v, want exactly one current branch with a name and hash", branches)
	}
	if current[0].Name != status.CurrentBranch {
		t.Errorf("current branch %q disagrees with GetRepoStatus %q", current[0].Name, status.CurrentBranch)
	}

	// 4. GetRecentActivityLogs: the reads above were recorded for the drawer.
	ClearRecentActivityLogs()
	if _, err := svc.GetRepoStatus(repo); err != nil {
		t.Fatalf("viewer GetRepoStatus after log clear: %v", err)
	}
	logs := GetRecentActivityLogs()
	if len(logs) == 0 {
		t.Fatalf("viewer GetRecentActivityLogs is empty after a read, want the git invocations recorded")
	}
	foundStatus := false
	for _, l := range logs {
		if strings.Contains(l.Command, "status") && l.Dir == repo && l.Success {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Fatalf("activity logs %v do not contain the successful status call for %s", logs, repo)
	}

	// 5. GetAppFlags: the configured feature flags surface unchanged.
	flags := svc.GetAppFlags()
	if !flags.ReviewFlow {
		t.Errorf("viewer GetAppFlags = %+v, want the planted reviewFlow=true to come back", flags)
	}

	// 6. GetGitProxy: a valid empty proxy config (the throwaway HOME has no
	// global gitconfig), not an error and not a fabricated value.
	proxy, err := svc.GetGitProxy(repo)
	if err != nil {
		t.Fatalf("viewer GetGitProxy: %v", err)
	}
	if proxy == nil || proxy.HttpProxy != "" || proxy.HttpsProxy != "" || proxy.IsGlobal {
		t.Fatalf("viewer GetGitProxy = %+v, want an empty non-global proxy config", proxy)
	}

	// 7. GetStashes: an empty stash list reads back as empty, not as an error.
	stashes, err := svc.GetStashes(repo)
	if err != nil {
		t.Fatalf("viewer GetStashes: %v", err)
	}
	if len(stashes) != 0 {
		t.Fatalf("viewer GetStashes = %+v, want an empty list", stashes)
	}

	// Contrast: the same process really is gating — a mutating RPC from the
	// same viewer gets the exact forbidden message, so the read successes above
	// mean the gate is correct, not switched off.
	if err := svc.DiscardFile(repo, "hello.txt"); err == nil || err.Error() != "forbidden: requires operator role (current: viewer)" {
		t.Errorf("viewer DiscardFile error = %v, want the exact operator forbidden message", err)
	}
}

// TestAccessScenarioOperatorFullWorkflow runs the day-to-day operator chain on
// a real repository: mutate → discard, stash cycles, and tag lifecycle. Gated
// calls must succeed; their failures must be ordinary git business errors —
// never the forbidden message. A final viewer-contrast call pins both halves.
func TestAccessScenarioOperatorFullWorkflow(t *testing.T) {
	setServerMode(t, true)
	scenarioAccessConfig(t, "operator")
	setRoleEnv(t, "", false)

	svc := GitService{}
	repo := newAccessScenarioRepo(t, svc)
	helloPath := filepath.Join(repo, "hello.txt")

	// DiscardFile (operator-gated) really discards: the working tree change is
	// reverted to the committed content.
	if err := os.WriteFile(helloPath, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modify hello.txt: %v", err)
	}
	if err := svc.DiscardFile(repo, "hello.txt"); err != nil {
		t.Fatalf("operator DiscardFile: %v", err)
	}
	if got, err := os.ReadFile(helloPath); err != nil || string(got) != "hello\n" {
		t.Fatalf("after DiscardFile hello.txt = %q (err %v), want the committed \"hello\\n\"", got, err)
	}

	// StashPop against an empty stash: the gate lets the operator through and
	// git answers with an ordinary stash error — the exact "gate open, business
	// failed" shape the denial tests cannot express.
	assertBusinessFailure(t, "operator StashPop on an empty stash", svc.StashPop(repo, 0))

	// A real stash cycle through the gate: save → list → pop → content back.
	if err := os.WriteFile(helloPath, []byte("stashed change\n"), 0o644); err != nil {
		t.Fatalf("modify hello.txt for stash: %v", err)
	}
	if err := svc.StashSaveAdvanced(repo, "wip scenario", false, false, false); err != nil {
		t.Fatalf("operator StashSaveAdvanced: %v", err)
	}
	stashes, err := svc.GetStashes(repo)
	if err != nil || len(stashes) != 1 {
		t.Fatalf("operator GetStashes after save = %v (err %v), want exactly one stash", stashes, err)
	}
	if err := svc.StashPop(repo, 0); err != nil {
		t.Fatalf("operator StashPop: %v", err)
	}
	if got, err := os.ReadFile(helloPath); err != nil || string(got) != "stashed change\n" {
		t.Fatalf("after StashPop hello.txt = %q (err %v), want the stashed change back", got, err)
	}
	if stashes, err := svc.GetStashes(repo); err != nil || len(stashes) != 0 {
		t.Fatalf("operator GetStashes after pop = %v (err %v), want the stash consumed", stashes, err)
	}

	// DeleteTag on a tag that does not exist: past the gate, git's own
	// "not found" business error comes back.
	err = svc.DeleteTag(repo, "no-such-tag")
	assertBusinessFailure(t, "operator DeleteTag on a missing tag", err)
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("operator DeleteTag missing-tag error = %v, want git's not-found wording", err)
	}

	// ...and the same gated RPC succeeds once the tag exists.
	if err := svc.CreateTag(repo, "v1.0", "", ""); err != nil {
		t.Fatalf("operator CreateTag: %v", err)
	}
	if err := svc.DeleteTag(repo, "v1.0"); err != nil {
		t.Fatalf("operator DeleteTag: %v", err)
	}
	if tags, err := svc.GetTags(repo); err != nil || len(tags) != 0 {
		t.Fatalf("operator GetTags after delete = %v (err %v), want the tag gone", tags, err)
	}

	// Contrast: flipping WGIT_ROLE to viewer in this same process turns the very
	// same DiscardFile call into the exact forbidden message. Gate denial and
	// business failure are observably different outcomes.
	t.Setenv("WGIT_ROLE", "viewer")
	if err := svc.DiscardFile(repo, "hello.txt"); err == nil || err.Error() != "forbidden: requires operator role (current: viewer)" {
		t.Errorf("viewer-override DiscardFile error = %v, want the exact operator forbidden message", err)
	}
}

// TestAccessScenarioAdminMaintenanceRPCs runs the two admin-gated maintenance
// RPCs against a real repository. Both must get past the gate and do real
// work: staged-secret remediation unstages and gitignores, object-store
// optimization runs its tasks successfully.
func TestAccessScenarioAdminMaintenanceRPCs(t *testing.T) {
	setServerMode(t, true)
	scenarioAccessConfig(t, "admin")
	setRoleEnv(t, "", false)

	svc := GitService{}
	repo := newAccessScenarioRepo(t, svc)

	// RemediateStagedSecrets (admin-gated): a staged credentials file is pulled
	// back out of the index and added to .gitignore.
	secretPath := filepath.Join(repo, "credentials.txt")
	if err := os.WriteFile(secretPath, []byte("token=supersecret-value\n"), 0o644); err != nil {
		t.Fatalf("write credentials.txt: %v", err)
	}
	if err := svc.StageFile(repo, "credentials.txt"); err != nil {
		t.Fatalf("StageFile credentials.txt: %v", err)
	}
	before, err := svc.GetRepoStatus(repo)
	if err != nil || len(before.StagedFiles) != 1 {
		t.Fatalf("pre-remediation status = %+v (err %v), want credentials.txt staged", before, err)
	}
	res, err := svc.RemediateStagedSecrets(repo, []string{"credentials.txt"}, true, false)
	if err != nil {
		t.Fatalf("admin RemediateStagedSecrets: %v", err)
	}
	if res == nil || !res.Success || res.Unstaged != 1 || len(res.PatternsAdded) != 1 || res.PatternsAdded[0] != "credentials.txt" {
		t.Fatalf("admin RemediateStagedSecrets result = %+v, want success with the file unstaged and gitignored", res)
	}
	after, err := svc.GetRepoStatus(repo)
	if err != nil || len(after.StagedFiles) != 0 {
		t.Fatalf("post-remediation status = %+v (err %v), want an empty index", after, err)
	}
	ignored, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil || !strings.Contains(string(ignored), "credentials.txt") {
		t.Fatalf(".gitignore = %q (err %v), want the credentials.txt pattern", ignored, err)
	}

	// OptimizeObjectStore (admin-gated): the requested tasks really run.
	res2, err := svc.OptimizeObjectStore(repo, []string{"commit-graph", "pack-refs"})
	if err != nil {
		t.Fatalf("admin OptimizeObjectStore: %v", err)
	}
	if res2 == nil || !res2.Success || len(res2.Results) != 2 {
		t.Fatalf("admin OptimizeObjectStore result = %+v, want success with both tasks executed", res2)
	}
	for _, r := range res2.Results {
		if r.Skipped || !r.Success {
			t.Errorf("optimize task %q = skipped=%v success=%v (%s), want it to have run cleanly", r.Task, r.Skipped, r.Success, r.Output)
		}
	}
}

// TestAccessScenarioRoleSwitchHygiene checks in-process role switching: with a
// server-mode operator config, flipping WGIT_ROLE and re-reading the config
// changes effectiveRole() and therefore the behaviour of one gated RPC
// (DeleteTag with empty arguments, whose post-gate validation error is
// "tagName 不能为空").
func TestAccessScenarioRoleSwitchHygiene(t *testing.T) {
	setServerMode(t, true)
	scenarioAccessConfig(t, "operator")
	setRoleEnv(t, "", false)
	svc := GitService{}

	if got := effectiveRole(); got != RoleOperator {
		t.Fatalf("effectiveRole() = %v, want RoleOperator from the config file", got)
	}
	if err := svc.DeleteTag("", ""); err == nil || err.Error() != "tagName 不能为空" {
		t.Errorf("operator DeleteTag(\"\",\"\") error = %v, want the post-gate validation error", err)
	}

	// Downgrade via the environment: the next LoadConfig inside effectiveRole
	// picks WGIT_ROLE up and the gate slams shut with the exact message.
	t.Setenv("WGIT_ROLE", "viewer")
	if got := LoadConfig().Access.Role; got != "viewer" {
		t.Errorf("LoadConfig().Access.Role under WGIT_ROLE=viewer = %q, want viewer", got)
	}
	if got := effectiveRole(); got != RoleViewer {
		t.Errorf("effectiveRole() under WGIT_ROLE=viewer = %v, want RoleViewer", got)
	}
	if err := svc.DeleteTag("", ""); err == nil || err.Error() != "forbidden: requires operator role (current: viewer)" {
		t.Errorf("viewer DeleteTag(\"\",\"\") error = %v, want the exact operator forbidden message", err)
	}

	// Upgrade again in the same process: admin passes the operator gate and
	// lands back in ordinary argument validation.
	t.Setenv("WGIT_ROLE", "admin")
	if got := effectiveRole(); got != RoleAdmin {
		t.Errorf("effectiveRole() under WGIT_ROLE=admin = %v, want RoleAdmin", got)
	}
	if err := svc.DeleteTag("", ""); err == nil || err.Error() != "tagName 不能为空" {
		t.Errorf("admin DeleteTag(\"\",\"\") error = %v, want the post-gate validation error", err)
	}
}
