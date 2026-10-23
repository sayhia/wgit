package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectAssetReferencesRejectsInvalidRepo(t *testing.T) {
	svc := GitService{}
	if _, err := svc.InspectAssetReferences(t.TempDir(), "Assets/Hero.png"); err == nil {
		t.Fatal("a non-repository directory must not be reported as safe to delete")
	}
}

func TestInspectAssetReferencesMarksIncompleteWhenFileUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based fixture is meaningless as root")
	}
	dir := initClosedLoopRepo(t)
	writeScanCandidate(t, dir, "Assets/Tex/Hero.png")
	broken := filepath.Join(dir, "Assets", "Scenes", "Broken.unity")
	if err := os.MkdirAll(filepath.Dir(broken), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("Scene: Hero\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(broken, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(broken, 0644) })

	svc := GitService{}
	res, err := svc.InspectAssetReferences(dir, "Assets/Tex/Hero.png")
	if err != nil {
		t.Fatalf("InspectAssetReferences failed: %v", err)
	}
	if res.AnalysisComplete {
		t.Fatal("an unreadable candidate must make the scan incomplete")
	}
	if res.SafeToDelete {
		t.Fatal("must not claim safe to delete when the scan could not read every candidate")
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected the unreadable file to be reported")
	}
}

func TestInspectAssetReferencesMarksIncompleteWhenCandidateOversized(t *testing.T) {
	dir := initClosedLoopRepo(t)
	writeScanCandidate(t, dir, "Assets/Tex/Hero.png")
	big := filepath.Join(dir, "Assets", "Big.prefab")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2*1024*1024 + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	res, err := svc.InspectAssetReferences(dir, "Assets/Tex/Hero.png")
	if err != nil {
		t.Fatalf("InspectAssetReferences failed: %v", err)
	}
	if res.AnalysisComplete {
		t.Fatal("skipping an oversized candidate must make the scan incomplete")
	}
	if res.SafeToDelete {
		t.Fatal("must not claim safe to delete when a candidate was skipped")
	}
}

func TestAnalyzeAssetImpactDoesNotClaimSafeWhenScanIncomplete(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based fixture is meaningless as root")
	}
	dir := initClosedLoopRepo(t)
	writeScanCandidate(t, dir, "Assets/Tex/Orphan.png")
	broken := filepath.Join(dir, "Assets", "Broken.mat")
	if err := os.WriteFile(broken, []byte("Material: Orphan\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(broken, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(broken, 0644) })

	svc := GitService{}
	res, err := svc.AnalyzeAssetImpact(dir, "Assets/Tex/Orphan.png", 2)
	if err != nil {
		t.Fatalf("AnalyzeAssetImpact failed: %v", err)
	}
	if res.AnalysisComplete {
		t.Fatal("an unreadable candidate must make the impact analysis incomplete")
	}
	if res.SafeToDelete {
		t.Fatal("must not report an unread asset as safe to delete")
	}
	if res.RiskLevel != "unknown" {
		t.Fatalf("expected unknown risk for an incomplete scan, got %q", res.RiskLevel)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected the incomplete scan to carry warnings")
	}
}

func TestAssetInspectorsRejectPathOutsideRepo(t *testing.T) {
	dir := initClosedLoopRepo(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.png")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	svc := GitService{}
	if _, err := svc.InspectAssetMetadata(dir, "../outside.png"); err == nil {
		t.Fatal("metadata inspector must reject a parent-directory path")
	}
	if _, err := svc.InspectAssetReferences(dir, "../outside.png"); err == nil {
		t.Fatal("reference inspector must reject a parent-directory path")
	}
	if _, err := svc.InspectAudioFile(dir, "../outside.png"); err == nil {
		t.Fatal("audio inspector must reject a parent-directory path")
	}
	if _, err := svc.GetAssetDiff(dir, "../outside.png", "HEAD", "WORKTREE"); err == nil {
		t.Fatal("asset diff must reject a parent-directory path")
	}
}

func writeScanCandidate(t *testing.T, dir string, rel string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("PNGDATA"), 0644); err != nil {
		t.Fatal(err)
	}
}
