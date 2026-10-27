package main

import (
	"testing"
	"time"
)

// TestRepoHealthCheckParallelDimensions verifies that running the independent
// diagnostics concurrently still yields the documented, deterministic ordering
// and self-consistent aggregation.
func TestRepoHealthCheckParallelDimensions(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")
	writeAndCommit(t, dir, "src/Parallel.cs", "class Parallel {}\n", "feat: parallel health", time.Now(), "Alice")

	svc := GitService{}
	wantOrder := []string{"identity", "objects", "ignore", "largefiles", "signing", "workspace"}

	report, err := svc.RunRepoHealthCheck(dir)
	if err != nil {
		t.Fatalf("RunRepoHealthCheck failed: %v", err)
	}
	checkHealthReportShape(t, report, wantOrder)

	// The concurrent run must be deterministic: a second pass over the same repo
	// yields the exact same dimension order and per-dimension identity.
	report2, err := svc.RunRepoHealthCheck(dir)
	if err != nil {
		t.Fatalf("second RunRepoHealthCheck failed: %v", err)
	}
	checkHealthReportShape(t, report2, wantOrder)
	for i := range report.Items {
		if report.Items[i].Key != report2.Items[i].Key {
			t.Fatalf("dimension order is not deterministic at %d: %q vs %q", i, report.Items[i].Key, report2.Items[i].Key)
		}
		if report.Items[i].Status != report2.Items[i].Status || report.Items[i].Score != report2.Items[i].Score {
			t.Fatalf("dimension %s changed between runs: %+v vs %+v", report.Items[i].Key, report.Items[i], report2.Items[i])
		}
	}
	if report.Score != report2.Score || report.Summary != report2.Summary {
		t.Fatalf("aggregation is not deterministic: score %d/%d summary %q/%q",
			report.Score, report2.Score, report.Summary, report2.Summary)
	}

	t.Logf("health(parallel): %d (%s) %s", report.Score, report.Grade, report.Summary)
}

// checkHealthReportShape asserts the invariants the report contract relies on:
// one item per documented dimension in order, sane counters/score and a summary.
func checkHealthReportShape(t *testing.T, report *RepoHealthReport, wantOrder []string) {
	t.Helper()
	if report == nil {
		t.Fatal("health report is nil")
	}
	if report.Items == nil {
		t.Fatal("health report items must be non-nil")
	}
	if len(report.Items) != len(wantOrder) {
		t.Fatalf("expected %d dimensions, got %d", len(wantOrder), len(report.Items))
	}
	seen := make(map[string]int, len(report.Items))
	for i, item := range report.Items {
		if item.Key != wantOrder[i] {
			t.Fatalf("dimension %d out of order: expected %q, got %q", i, wantOrder[i], item.Key)
		}
		seen[item.Key]++
	}
	for _, key := range wantOrder {
		if seen[key] != 1 {
			t.Fatalf("dimension %q must appear exactly once, got %d", key, seen[key])
		}
	}
	if got := report.PassCount + report.WarnCount + report.FailCount; got != len(report.Items) {
		t.Fatalf("counters are inconsistent: pass=%d warn=%d fail=%d, items=%d",
			report.PassCount, report.WarnCount, report.FailCount, len(report.Items))
	}
	if report.Score < 0 || report.Score > 100 {
		t.Fatalf("score out of range: %d", report.Score)
	}
	if report.Summary == "" {
		t.Fatal("summary must not be empty")
	}
	if report.SummaryKey == "" {
		t.Fatal("summary key must be set")
	}
	for _, item := range report.Items {
		if item.Status != "pass" && item.Status != "warn" && item.Status != "fail" {
			t.Fatalf("dimension %s has an unknown status %q", item.Key, item.Status)
		}
		if item.Score < 0 || item.Score > 100 {
			t.Fatalf("dimension %s score out of range: %d", item.Key, item.Score)
		}
	}
}
