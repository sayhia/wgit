package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// walkTestSkipForRoot skips permission-based fixtures, which have no effect
// when the test process runs as root.
func walkTestSkipForRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-denied fixtures are ineffective when running as root")
	}
}

// walkTestBlockPath creates dir holding one file and strips all permissions so
// filepath.Walk reports it as unreadable for non-root users.
func walkTestBlockPath(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blocked.bin"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755)
	})
}

func TestGetMaintenanceStatsReportsUnreadableGitPaths(t *testing.T) {
	walkTestSkipForRoot(t)
	svc := GitService{}

	cases := []struct {
		name      string
		blockWalk bool
		wantNote  bool
	}{
		{name: "clean repo reports no unreadable paths"},
		{name: "unreadable .git path is surfaced", blockWalk: true, wantNote: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGitTest(t, repo, nil, "init", "-b", "main")
			writeAndCommit(t, repo, "base.txt", "base\n", "chore: base", time.Now(), "Tester")
			if tc.blockWalk {
				walkTestBlockPath(t, filepath.Join(repo, ".git", "walk-blocked"))
			}

			stats, err := svc.GetMaintenanceStats(repo)
			if err != nil {
				t.Fatalf("GetMaintenanceStats failed: %v", err)
			}
			hasNote := strings.Contains(stats.Summary, "路径读取失败") &&
				strings.Contains(strings.Join(stats.Warnings, " "), "路径读取失败")
			if hasNote != tc.wantNote {
				t.Fatalf("Summary=%q Warnings=%v, want unreadable note=%v", stats.Summary, stats.Warnings, tc.wantNote)
			}
			if tc.wantNote && !strings.Contains(stats.Summary, "统计体积可能少算") {
				t.Fatalf("summary must warn that sizes may be understated: %q", stats.Summary)
			}
		})
	}
}

func TestRunMaintenanceKeepsUnreadablePathWarnings(t *testing.T) {
	walkTestSkipForRoot(t)
	svc := GitService{}
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	writeAndCommit(t, repo, "base.txt", "base\n", "chore: base", time.Now(), "Tester")
	walkTestBlockPath(t, filepath.Join(repo, ".git", "walk-blocked"))

	stats, err := svc.RunMaintenance(repo)
	if err != nil {
		t.Fatalf("RunMaintenance failed: %v", err)
	}
	if !strings.Contains(strings.Join(stats.Warnings, " "), "路径读取失败") {
		t.Fatalf("RunMaintenance must keep the walk warnings from GetMaintenanceStats: %+v", stats.Warnings)
	}
	// The caveat is a measurement note, not a failed maintenance step.
	if !stats.Success {
		t.Fatalf("measurement caveats must not fail the maintenance run: %+v", stats)
	}
}

func TestGetRepositoryDiskBreakdownReportsUnreadablePaths(t *testing.T) {
	walkTestSkipForRoot(t)
	svc := GitService{}

	cases := []struct {
		name      string
		blockWalk bool
		wantNote  bool
	}{
		{name: "clean repo has no unreadable note"},
		{name: "unreadable working tree path is surfaced", blockWalk: true, wantNote: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			runGitTest(t, repo, nil, "init", "-b", "main")
			writeAndCommit(t, repo, "Assets/model.fbx", "mesh\n", "chore: asset", time.Now(), "Tester")
			if tc.blockWalk {
				walkTestBlockPath(t, filepath.Join(repo, "BlockedFolder"))
			}

			result, err := svc.GetRepositoryDiskBreakdown(repo)
			if err != nil {
				t.Fatalf("GetRepositoryDiskBreakdown failed: %v", err)
			}
			joined := strings.Join(result.Recommendations, " ")
			if hasNote := strings.Contains(joined, "路径读取失败"); hasNote != tc.wantNote {
				t.Fatalf("Recommendations=%v, want unreadable note=%v", result.Recommendations, tc.wantNote)
			}
			if tc.wantNote && strings.Contains(joined, "存储健康") {
				t.Fatalf("healthy-storage default must not appear next to the unreadable note: %q", joined)
			}
		})
	}
}

func TestGetSubmodulesURLResolution(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	cases := []struct {
		name           string
		breakURLConfig bool
	}{
		{name: "readable config keeps the real url"},
		{name: "unresolvable url is marked unreadable", breakURLConfig: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bareDir := filepath.Join(t.TempDir(), "origin.git")
			submoduleTestGit(t, t.TempDir(), "init", "--bare", bareDir)
			submoduleTestSeedRemote(t, bareDir)

			repo := t.TempDir()
			submoduleTestGit(t, repo, "init")
			submoduleTestGit(t, repo, "submodule", "add", bareDir, "sub1")
			submoduleTestGit(t, repo, "commit", "-m", "add submodule")

			if tc.breakURLConfig {
				// Leave no readable URL anywhere: drop the url from
				// .gitmodules (keep the path so `git submodule status` still
				// resolves the entry) and from the local config. This is the
				// situation the config lookups used to paper over silently.
				if err := os.WriteFile(filepath.Join(repo, ".gitmodules"), []byte("[submodule \"sub1\"]\n\tpath = sub1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				submoduleTestGit(t, repo, "config", "--unset", "submodule.sub1.url")
			}

			wantURL := bareDir
			if tc.breakURLConfig {
				wantURL = "(unreadable)"
			}

			list, err := GitService{}.GetSubmodules(repo)
			if err != nil {
				t.Fatalf("GetSubmodules failed: %v", err)
			}
			if len(list) != 1 {
				t.Fatalf("expected 1 submodule, got %+v", list)
			}
			if list[0].URL != wantURL {
				t.Fatalf("expected URL %q, got %q", wantURL, list[0].URL)
			}
		})
	}
}
