package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneRepoAdvancedInTempRepo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	svc := GitService{}
	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.InitRepo(source); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(source, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(source, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(source, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := svc.CreateBranch(source, "topic", ""); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	fullTarget := filepath.Join(parent, "clone-full")
	info, err := svc.CloneRepoAdvanced(CloneOptions{URL: source, TargetPath: fullTarget, CloneType: "full"})
	if err != nil || info == nil || info.Name != "clone-full" || info.CurrentBranch == "" {
		t.Fatalf("full clone failed: %+v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(fullTarget, "base.txt")); err != nil {
		t.Fatalf("full clone missing file: %v", err)
	}
	commits, err := svc.GetCommits(fullTarget, 10, "")
	if err != nil || len(commits) != 1 {
		t.Fatalf("full clone commit count failed: %d err=%v", len(commits), err)
	}

	shallowTarget := filepath.Join(parent, "clone-shallow")
	info, err = svc.CloneRepoAdvanced(CloneOptions{
		URL: "file://" + source, TargetPath: shallowTarget, CloneType: "shallow", Depth: 1,
	})
	if err != nil || info == nil {
		t.Fatalf("shallow clone failed: %+v err=%v", info, err)
	}
	shallowCommits, _ := svc.GetCommits(shallowTarget, 10, "")
	if len(shallowCommits) != 1 {
		t.Fatalf("shallow clone should have one commit, got %d", len(shallowCommits))
	}

	branchTarget := filepath.Join(parent, "clone-branch")
	info, err = svc.CloneRepoAdvanced(CloneOptions{
		URL: source, TargetPath: branchTarget, CloneType: "full", Branch: "topic",
	})
	if err != nil || info == nil || info.CurrentBranch != "topic" {
		t.Fatalf("branch clone failed: %+v err=%v", info, err)
	}

	badTarget := filepath.Join(parent, "clone-missing")
	if _, err := svc.CloneRepoAdvanced(CloneOptions{URL: filepath.Join(parent, "missing"), TargetPath: badTarget, CloneType: "full"}); err == nil {
		t.Fatal("expected clone of missing source to fail")
	}
	if _, err := svc.CloneRepoAdvanced(CloneOptions{URL: "", TargetPath: badTarget, CloneType: "full"}); err == nil {
		t.Fatal("expected error for empty clone URL")
	}
	if _, err := svc.CloneRepoAdvanced(CloneOptions{URL: source, TargetPath: "", CloneType: "full"}); err == nil {
		t.Fatal("expected error for empty clone target")
	}
	occupied := filepath.Join(parent, "clone-occupied")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "existing.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CloneRepoAdvanced(CloneOptions{URL: source, TargetPath: occupied, CloneType: "full"}); err == nil {
		t.Fatal("expected clone into a non-empty directory to fail")
	}
}

func TestDiskBreakdownLargeBlobsAndImagePreview(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	bigData := strings.Repeat("0123456789abcdef", 70*1024)
	if err := os.WriteFile(filepath.Join(repo, "big.bin"), []byte(bigData), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "big.bin"); err != nil {
		t.Fatalf("StageFile big failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: big", false); err != nil {
		t.Fatalf("Commit big failed: %v", err)
	}

	pngV1 := []byte("\x89PNG\r\n\x1a\nfirst-version")
	if err := os.WriteFile(filepath.Join(repo, "image.png"), pngV1, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "image.png"); err != nil {
		t.Fatalf("StageFile image failed: %v", err)
	}
	if err := svc.Commit(repo, "feat: image", false); err != nil {
		t.Fatalf("Commit image failed: %v", err)
	}
	imageHash, _ := ExecGit(repo, "rev-parse", "HEAD")
	imageHash = strings.TrimSpace(imageHash)
	pngV2 := []byte("\x89PNG\r\n\x1a\nsecond-version")
	if err := os.WriteFile(filepath.Join(repo, "image.png"), pngV2, 0o644); err != nil {
		t.Fatal(err)
	}

	disk, err := svc.GetRepositoryDiskBreakdown(repo)
	if err != nil || disk == nil || disk.TotalSize <= 0 || disk.GitInternalSize <= 0 || disk.WorkingTreeSize <= 0 {
		t.Fatalf("GetRepositoryDiskBreakdown failed: %+v err=%v", disk, err)
	}
	if disk.TopFolders == nil || disk.AssetCategories == nil || disk.Recommendations == nil {
		t.Fatalf("disk breakdown slices must not be nil: %+v", disk)
	}

	blobs, err := svc.ScanHistoryLargeBlobs(repo, 10)
	if err != nil || len(blobs) == 0 {
		t.Fatalf("ScanHistoryLargeBlobs failed: %+v err=%v", blobs, err)
	}
	var sawBig bool
	for _, blob := range blobs {
		if blob.Path == "big.bin" {
			sawBig = blob.Size >= int64(len(bigData)) && blob.SizeStr != "" && blob.OID != ""
		}
	}
	if !sawBig {
		t.Fatalf("expected big.bin in large blob scan, got %+v", blobs)
	}

	uri, err := svc.GetImageBase64(repo, imageHash, "image.png")
	if err != nil || !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("GetImageBase64 at commit failed: %q err=%v", uri, err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/png;base64,"))
	if err != nil || string(decoded) != string(pngV1) {
		t.Fatalf("commit image content mismatch: %q err=%v", decoded, err)
	}
	uri, err = svc.GetImageBase64(repo, "WORKING", "image.png")
	if err != nil {
		t.Fatalf("GetImageBase64 working failed: %v", err)
	}
	decoded, _ = base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/png;base64,"))
	if string(decoded) != string(pngV2) {
		t.Fatalf("working image content mismatch: %q", decoded)
	}
	if _, err := svc.GetImageBase64(repo, "", "big.bin"); err == nil {
		t.Fatal("expected non-image preview to fail")
	}
	if _, err := svc.GetImageBase64(repo, "", "missing.png"); err == nil {
		t.Fatal("expected missing image preview to fail")
	}
}
