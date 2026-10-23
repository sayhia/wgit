package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseLfsPruneSummary(t *testing.T) {
	cases := []struct {
		name  string
		out   string
		files int
		size  string
	}{
		{
			name:  "plural dry run",
			out:   "4 local objects, 2 retained, done.\n2 files would be pruned (48 B), done.",
			files: 2,
			size:  "48 B",
		},
		{
			name:  "singular dry run",
			out:   "3 local objects, 2 retained, done.\n1 file would be pruned (25 B), done.",
			files: 1,
			size:  "25 B",
		},
		{
			name:  "legacy summary",
			out:   "Pruning 3 files (1.2 MB).",
			files: 3,
			size:  "1.2 MB",
		},
		{
			name:  "nothing pruned",
			out:   "2 local objects, 2 retained, done.",
			files: 0,
			size:  "0 B",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, size := parseLfsPruneSummary(tc.out)
			if files != tc.files || size != tc.size {
				t.Fatalf("got %d files / %q, want %d / %q", files, size, tc.files, tc.size)
			}
		})
	}
}

func TestLfsPruneReportsReclaimedObjects(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
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
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "Tester")
	run("lfs", "install", "--local")
	run("lfs", "track", "*.bin")
	write("keep.bin", "keep-payload")
	run("add", ".gitattributes", "keep.bin")
	run("commit", "-qm", "keep lfs asset")

	run("checkout", "-q", "-b", "orphan")
	write("drop.bin", "drop-payload")
	run("add", "drop.bin")
	run("commit", "-qm", "orphan lfs asset")
	run("checkout", "-q", "main")
	run("branch", "-D", "orphan")
	run("reflog", "expire", "--expire=now", "--all")
	run("gc", "--prune=now")

	svc := GitService{}
	dry, err := svc.LfsPrune(repo, true, false)
	if err != nil {
		t.Fatalf("LfsPrune dry-run failed: %v", err)
	}
	if !dry.Success || dry.PrunedFiles != 1 || dry.ReclaimedStr == "0 B" {
		t.Fatalf("dry-run should report one reclaimable object: %+v", dry)
	}

	actual, err := svc.LfsPrune(repo, false, false)
	if err != nil {
		t.Fatalf("LfsPrune failed: %v", err)
	}
	if !actual.Success || actual.PrunedFiles != 1 || actual.ReclaimedStr == "0 B" {
		t.Fatalf("actual prune should report one reclaimed object: %+v", actual)
	}

	stats, err := svc.InspectLfsCacheStats(repo)
	if err != nil {
		t.Fatalf("InspectLfsCacheStats failed: %v", err)
	}
	if stats.FileCount != 1 {
		t.Fatalf("only the checked-out object should remain, got %+v", stats)
	}
}
