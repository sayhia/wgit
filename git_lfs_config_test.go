package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLfsConfigAndInspectionWithoutBinary(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "config", "user.name", "Tester")
	_, _ = ExecGit(repo, "config", "user.email", "tester@example.com")

	tuning, err := svc.GetLfsTuningConfig(repo)
	if err != nil || tuning.ConcurrentTransfers != 3 || tuning.PostBufferMB != 1 || tuning.DialTimeoutSec != 30 {
		t.Fatalf("GetLfsTuningConfig defaults failed: %v, cfg=%+v", err, tuning)
	}
	highSpeed, err := svc.ApplyLfsTuningOptimization(repo, "studio_high_speed")
	if err != nil || highSpeed == nil || highSpeed.Config.ConcurrentTransfers != 16 || highSpeed.Config.PostBufferMB != 512 || !highSpeed.Config.IsOptimized {
		t.Fatalf("ApplyLfsTuningOptimization(high_speed) failed: %v, res=%+v", err, highSpeed)
	}
	standard, err := svc.ApplyLfsTuningOptimization(repo, "standard")
	if err != nil || standard.Config.ConcurrentTransfers != 8 || standard.Config.PostBufferMB != 256 {
		t.Fatalf("ApplyLfsTuningOptimization(standard) failed: %v, res=%+v", err, standard)
	}
	reset, err := svc.ApplyLfsTuningOptimization(repo, "reset_default")
	if err != nil || reset.Config.ConcurrentTransfers != 3 || reset.Config.PostBufferMB != 1 || reset.Config.IsOptimized {
		t.Fatalf("ApplyLfsTuningOptimization(reset_default) failed: %v, res=%+v", err, reset)
	}

	if err := svc.SetLfsFilterConfig(repo, []string{"Assets/**", "*.psd"}, []string{"Temp/**"}); err != nil {
		t.Fatalf("SetLfsFilterConfig failed: %v", err)
	}
	filters, err := svc.GetLfsFilterConfig(repo)
	if err != nil || len(filters.FetchInclude) != 2 || len(filters.FetchExclude) != 1 || filters.FetchInclude[0] != "Assets/**" || filters.FetchExclude[0] != "Temp/**" {
		t.Fatalf("GetLfsFilterConfig failed: %v, filters=%+v", err, filters)
	}
	if err := svc.SetLfsFilterConfig(repo, nil, nil); err != nil {
		t.Fatalf("clearing LFS filters failed: %v", err)
	}
	filters, _ = svc.GetLfsFilterConfig(repo)
	if len(filters.FetchInclude) != 0 || len(filters.FetchExclude) != 0 {
		t.Fatalf("LFS filters were not cleared: %+v", filters)
	}

	emptyStats, err := svc.InspectLfsCacheStats(repo)
	if err != nil || emptyStats.FileCount != 0 || emptyStats.TotalSizeBytes != 0 {
		t.Fatalf("InspectLfsCacheStats(empty) failed: %v, stats=%+v", err, emptyStats)
	}
	cacheFile := filepath.Join(repo, ".git", "lfs", "objects", "aa", "bb", "object")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	stats, err := svc.InspectLfsCacheStats(repo)
	if err != nil || stats.FileCount != 1 || stats.TotalSizeBytes != 1024 || stats.CachePath == "" {
		t.Fatalf("InspectLfsCacheStats failed: %v, stats=%+v", err, stats)
	}
	if usage, err := svc.GetLfsDiskUsage(repo); err != nil || usage == "0 B" {
		t.Fatalf("GetLfsDiskUsage failed: %v, usage=%q", err, usage)
	}

	readme := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readme, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "add", "README.md")
	_, _ = ExecGit(repo, "commit", "-m", "docs: v1")
	if err := os.WriteFile(readme, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "add", "README.md")
	_, _ = ExecGit(repo, "commit", "-m", "docs: v2")
	history, err := svc.InspectLfsFileHistory(repo, "README.md")
	if err != nil || len(history) != 2 || history[0].ShortHash == "" {
		t.Fatalf("InspectLfsFileHistory failed: %v, history=%+v", err, history)
	}

	large := filepath.Join(repo, "large.bin")
	if err := os.WriteFile(large, make([]byte, 2*1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = ExecGit(repo, "add", "large.bin")
	largeFiles, err := svc.CheckStagedNonLfsLargeFiles(repo, 1)
	if err != nil || len(largeFiles) != 1 || largeFiles[0].Path != "large.bin" {
		t.Fatalf("CheckStagedNonLfsLargeFiles failed: %v, files=%+v", err, largeFiles)
	}

	report, err := svc.ExportLfsLocksReport(repo, "markdown")
	if err != nil || !strings.Contains(report, "Git LFS") {
		t.Fatalf("ExportLfsLocksReport(markdown) failed: %v, report=%q", err, report)
	}
	csv, err := svc.ExportLfsLocksReport(repo, "csv")
	if err != nil || !strings.Contains(csv, "Path,Owner,ID") {
		t.Fatalf("ExportLfsLocksReport(csv) failed: %v, csv=%q", err, csv)
	}
}

func TestApplyLfsTuningRejectsInvalidRepoAndWriteFailure(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	if _, err := svc.ApplyLfsTuningOptimization(t.TempDir(), "standard"); err == nil {
		t.Fatal("expected error for a directory that is not a Git repository")
	}

	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApplyLfsTuningOptimization(repo, "unsupported"); err == nil {
		t.Fatal("expected error for an unsupported preset")
	}
	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if _, err := svc.ApplyLfsTuningOptimization(repo, "standard"); err == nil {
		t.Fatal("config write failure must not be reported as a successful optimization")
	}
}

func TestSetLfsFilterConfigPropagatesUnsetFailure(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	repo := t.TempDir()
	if _, err := ExecGit(repo, "init"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetLfsFilterConfig(repo, []string{"Assets/**"}, []string{"Temp/**"}); err != nil {
		t.Fatalf("SetLfsFilterConfig failed: %v", err)
	}
	gitDir := filepath.Join(repo, ".git")
	if err := os.Chmod(gitDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gitDir, 0o755) })
	if err := svc.SetLfsFilterConfig(repo, nil, nil); err == nil {
		t.Fatal("failed unset must not be reported as cleared")
	}
	if err := os.Chmod(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filters, err := svc.GetLfsFilterConfig(repo)
	if err != nil || len(filters.FetchInclude) != 1 || len(filters.FetchExclude) != 1 {
		t.Fatalf("filters should remain after failed clear: %+v err=%v", filters, err)
	}
	if err := svc.SetLfsFilterConfig(repo, nil, nil); err != nil {
		t.Fatalf("clearing filters after restoring permissions failed: %v", err)
	}
	filters, _ = svc.GetLfsFilterConfig(repo)
	if len(filters.FetchInclude) != 0 || len(filters.FetchExclude) != 0 {
		t.Fatalf("filters were not cleared: %+v", filters)
	}
}

func TestCheckStagedNonLfsLargeFilesRejectsInvalidRepo(t *testing.T) {
	if _, err := (GitService{}).CheckStagedNonLfsLargeFiles(t.TempDir(), 1); err == nil {
		t.Fatal("expected error when scanning staged files outside a Git repository")
	}
}
