package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepoHealthCheck(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", time.Now(), "Alice")

	svc := GitService{}
	report, err := svc.RunRepoHealthCheck(dir)
	if err != nil {
		t.Fatalf("RunRepoHealthCheck failed: %v", err)
	}
	if len(report.Items) != 6 {
		t.Fatalf("expected 6 dimensions, got %d", len(report.Items))
	}
	if report.Score < 0 || report.Score > 100 {
		t.Fatalf("score out of range: %d", report.Score)
	}
	if report.Grade == "" || report.Summary == "" || report.RepoName == "" {
		t.Fatalf("missing report metadata: %+v", report)
	}
	if report.PassCount+report.WarnCount+report.FailCount != 6 {
		t.Fatalf("unexpected counters: pass=%d warn=%d fail=%d", report.PassCount, report.WarnCount, report.FailCount)
	}

	byKey := map[string]HealthDimension{}
	for _, item := range report.Items {
		byKey[item.Key] = item
		if item.Action == "" {
			t.Fatalf("dimension %s has no action hook", item.Key)
		}
		// Every dimension must be localizable by the UI: a title key, a detail
		// key, and the params that detail key interpolates.
		if item.TitleKey == "" || item.DetailKey == "" {
			t.Fatalf("dimension %s is missing i18n keys: %+v", item.Key, item)
		}
		if item.TitleKey != "health.dim."+item.Key+".title" {
			t.Fatalf("unexpected title key %q for %s", item.TitleKey, item.Key)
		}
	}
	if report.SummaryKey == "" || report.SummaryParams["score"] == "" {
		t.Fatalf("report summary is missing i18n keys: %q %+v", report.SummaryKey, report.SummaryParams)
	}
	// The identity detail interpolates the configured author.
	if byKey["identity"].Params["name"] != "Alice" {
		t.Fatalf("identity detail is missing its name param: %+v", byKey["identity"].Params)
	}
	identity, ok := byKey["identity"]
	if !ok || identity.Status != "pass" {
		t.Fatalf("expected identity to pass, got %+v", identity)
	}
	objects, ok := byKey["objects"]
	if !ok || objects.Status == "fail" {
		t.Fatalf("expected object database to be healthy, got %+v", objects)
	}
	if byKey["signing"].Status != "warn" {
		t.Fatalf("expected signing to warn when disabled, got %+v", byKey["signing"])
	}

	// An 11MB file outside LFS must lower the large-file dimension.
	assets := filepath.Join(dir, "Assets")
	if err := os.MkdirAll(assets, 0755); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(assets, "HugeAsset.bin")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(11 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	report2, err := svc.RunRepoHealthCheck(dir)
	if err != nil {
		t.Fatalf("second health check failed: %v", err)
	}
	for _, item := range report2.Items {
		if item.Key == "largefiles" {
			if item.Status == "pass" {
				t.Fatalf("expected large file warning, got %+v", item)
			}
			t.Logf("largefiles: %s | %s", item.Status, item.Detail)
		}
	}
	if report2.Score > report.Score {
		t.Fatalf("expected score to drop after adding a big file (%d -> %d)", report.Score, report2.Score)
	}
	t.Logf("health: %d (%s) %s", report.Score, report.Grade, report.Summary)
}
