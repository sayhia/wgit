package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStudioCIUsesRealGitHubActionsData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses POSIX gh wrapper")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(repo, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
		return out
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "Tester")
	run("config", "user.email", "tester@example.com")
	run("commit", "-q", "--allow-empty", "-m", "initial")
	run("remote", "add", "origin", "https://github.com/acme/game.git")
	head := strings.TrimSpace(run("rev-parse", "HEAD"))

	runsJSON, err := json.Marshal([]map[string]any{
		{
			"databaseId": 101, "name": "CI", "displayTitle": "fix assets", "headBranch": "main",
			"headSha": head, "status": "completed", "conclusion": "success",
			"createdAt": timeString(-3), "updatedAt": timeString(-1), "workflowName": "CI", "number": 12,
		},
		{
			"databaseId": 100, "name": "CI", "displayTitle": "previous", "headBranch": "main",
			"headSha": head, "status": "completed", "conclusion": "failure",
			"createdAt": timeString(-120), "updatedAt": timeString(-118), "workflowName": "CI", "number": 11,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	jobsJSON := `{"jobs":[{"name":"build","status":"completed","conclusion":"success","startedAt":"2026-10-22T10:00:00Z","completedAt":"2026-10-22T10:03:00Z"},{"name":"deploy","status":"completed","conclusion":"failure","startedAt":"2026-10-22T10:03:00Z","completedAt":"2026-10-22T10:04:00Z"}]}`

	work := t.TempDir()
	runsPath := filepath.Join(work, "runs.json")
	jobsPath := filepath.Join(work, "jobs.json")
	logPath := filepath.Join(work, "gh.log")
	if err := os.WriteFile(runsPath, runsJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobsPath, []byte(jobsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(binDir, "git")); err != nil {
		t.Skipf("cannot create git symlink: %v", err)
	}
	ghScript := "#!/bin/sh\n" +
		"printf 'CALL %s\\n' \"$*\" >> \"$FAKE_GH_LOG\"\n" +
		"if [ \"$1\" = \"run\" ] && [ \"$2\" = \"list\" ]; then /bin/cat \"$FAKE_GH_RUNS\"; exit 0; fi\n" +
		"if [ \"$1\" = \"run\" ] && [ \"$2\" = \"view\" ]; then /bin/cat \"$FAKE_GH_JOBS\"; exit 0; fi\n" +
		"if [ \"$1\" = \"workflow\" ] && [ \"$2\" = \"run\" ]; then\n" +
		"  printf '%s\\n' \"$*\" >> \"$FAKE_GH_LOG\"\n" +
		"  printf 'Created workflow_dispatch event\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"unexpected gh args: $*\" >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(ghScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_GH_RUNS", runsPath)
	t.Setenv("FAKE_GH_JOBS", jobsPath)
	t.Setenv("FAKE_GH_LOG", logPath)
	t.Setenv("PATH", binDir)

	svc := GitService{}
	ci, err := svc.GetStudioCIPipelines(repo)
	if err != nil {
		t.Fatalf("GetStudioCIPipelines failed: %v", err)
	}
	if ci.Provider != "GitHub Actions 流水线" || ci.LatestRun == nil || len(ci.HistoryRuns) != 2 {
		if log, errLog := os.ReadFile(logPath); errLog == nil {
			t.Logf("gh log: %s", log)
		}
		t.Logf("runsJSON length=%d path=%s", len(runsJSON), runsPath)
		t.Fatalf("unexpected CI result: %+v", ci)
	}
	if ci.LatestRun.Status != "SUCCESS" || ci.LatestRun.Author != "Tester" || ci.LatestRun.ShortHash != head[:7] {
		t.Fatalf("unexpected latest run: %+v", ci.LatestRun)
	}
	if len(ci.LatestRun.Stages) != 2 || ci.LatestRun.Stages[0].Status != "SUCCESS" || ci.LatestRun.Stages[1].Status != "FAILED" {
		t.Fatalf("unexpected stages: %+v", ci.LatestRun.Stages)
	}
	if !strings.Contains(ci.LatestRun.Stages[0].Duration, "3 分") {
		t.Fatalf("stage duration should come from gh jobs: %+v", ci.LatestRun.Stages[0])
	}
	if ci.HistoryRuns[1].Status != "FAILED" || strings.Contains(ci.Message, "演示") {
		t.Fatalf("history/message not based on real runs: %+v", ci)
	}

	res, err := svc.TriggerStudioCIPipeline(repo, "CI")
	if err != nil {
		t.Fatalf("TriggerStudioCIPipeline failed: %v", err)
	}
	if !res.Success || !strings.Contains(res.Message, "Created workflow_dispatch event") {
		t.Fatalf("unexpected trigger result: %+v", res)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read gh log: %v", err)
	}
	line := string(log)
	for _, want := range []string{"workflow run CI", "--ref main", "-R acme/game"} {
		if !strings.Contains(line, want) {
			t.Fatalf("gh trigger args missing %q: %q", want, line)
		}
	}
}

func timeString(minutesAgo int) string {
	return time.Now().Add(time.Duration(minutesAgo) * time.Minute).UTC().Format(time.RFC3339)
}

// commitAsAuthor creates an empty commit with a distinct author so batch author
// resolution can be pinned name by name.
func commitAsAuthor(t *testing.T, dir, name, message string) string {
	t.Helper()
	email := strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "@studio.dev"
	env := []string{
		"GIT_AUTHOR_NAME=" + name,
		"GIT_AUTHOR_EMAIL=" + email,
		"GIT_COMMITTER_NAME=" + name,
		"GIT_COMMITTER_EMAIL=" + email,
	}
	runGitTest(t, dir, env, "commit", "--allow-empty", "-m", message)
	return strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))
}

// TestGitHubRunAuthorsBatchResolvesInOrder pins the batch contract: one
// `git show` resolves all distinct SHAs, records map back to the requesting
// runs in order, duplicate SHAs share one lookup and empty SHAs stay empty.
func TestGitHubRunAuthorsBatchResolvesInOrder(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Tester")
	runGitTest(t, repo, nil, "config", "user.email", "tester@example.com")
	runGitTest(t, repo, nil, "commit", "--allow-empty", "-m", "root")
	shaAlice := commitAsAuthor(t, repo, "Alice Chen", "feat: alice")
	shaBob := commitAsAuthor(t, repo, "Bob Wu", "feat: bob")
	shaCarol := commitAsAuthor(t, repo, "Carol Li", "feat: carol")

	runs := []ghRunSummary{
		{HeadSha: shaBob},
		{HeadSha: shaAlice},
		{HeadSha: shaCarol},
		{HeadSha: shaBob}, // duplicate SHA: one lookup, two runs
		{HeadSha: ""},     // no SHA: skipped, author stays empty
	}

	authors := gitHubRunAuthors(repo, runs)
	want := []string{"Bob Wu", "Alice Chen", "Carol Li", "Bob Wu", ""}
	for i, w := range want {
		if authors[i] != w {
			t.Fatalf("run %d author: got %q, want %q", i, authors[i], w)
		}
	}
}

// TestGitHubRunAuthorsFallsBackOnUnresolvableSHA keeps the serial error
// semantics: when one SHA cannot be resolved the whole batch probe fails, and
// the per-SHA fallback must still resolve every good run instead of dropping
// all authors.
func TestGitHubRunAuthorsFallsBackOnUnresolvableSHA(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Tester")
	runGitTest(t, repo, nil, "config", "user.email", "tester@example.com")
	runGitTest(t, repo, nil, "commit", "--allow-empty", "-m", "root")
	shaAlice := commitAsAuthor(t, repo, "Alice Chen", "feat: alice")
	shaCarol := commitAsAuthor(t, repo, "Carol Li", "feat: carol")
	badSha := strings.Repeat("d", 40) // well-formed but nonexistent

	runs := []ghRunSummary{
		{HeadSha: shaAlice},
		{HeadSha: badSha},
		{HeadSha: shaCarol},
	}

	authors := gitHubRunAuthors(repo, runs)
	if authors[0] != "Alice Chen" || authors[2] != "Carol Li" {
		t.Fatalf("good runs must keep their authors despite the bad SHA: %+v", authors)
	}
	if authors[1] != "" {
		t.Fatalf("unresolvable SHA must yield an empty author, got %q", authors[1])
	}
}

func TestStudioCIReportsUnsupportedProvider(t *testing.T) {
	repo := t.TempDir()
	svc := GitService{}
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}

	ci, err := svc.GetStudioCIPipelines(repo)
	if err != nil {
		t.Fatalf("GetStudioCIPipelines failed: %v", err)
	}
	if ci.Provider != "未检测到 CI 配置" || ci.LatestRun != nil || len(ci.HistoryRuns) != 0 {
		t.Fatalf("repo without CI should not report sample runs: %+v", ci)
	}
	if !strings.Contains(ci.Message, "未检测到 CI 配置") {
		t.Fatalf("unexpected message: %q", ci.Message)
	}
	if _, err := svc.TriggerStudioCIPipeline(repo, "CI"); err == nil {
		t.Fatal("triggering without a GitHub remote must fail")
	}
}
