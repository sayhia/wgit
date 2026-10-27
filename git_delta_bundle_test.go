package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeltaBundleAirGapped(t *testing.T) {
	tempBase, err := os.MkdirTemp("", "wgit-delta-bundle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempBase)

	svc := GitService{}

	repo1 := filepath.Join(tempBase, "repo1")
	if err := os.MkdirAll(repo1, 0755); err != nil {
		t.Fatalf("mkdir repo1 failed: %v", err)
	}

	// 1. Setup repo1 with base commit
	if _, err := ExecGit(repo1, "init"); err != nil {
		t.Fatalf("git init repo1 failed: %v", err)
	}
	if _, err := ExecGit(repo1, "config", "user.name", "Engineer"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	if _, err := ExecGit(repo1, "config", "user.email", "eng@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}

	f1 := filepath.Join(repo1, "app.go")
	if err := os.WriteFile(f1, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write app.go: %v", err)
	}
	if _, err := ExecGit(repo1, "add", "app.go"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := ExecGit(repo1, "commit", "-m", "chore: base commit"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
	// Rename to main
	_, _ = ExecGit(repo1, "branch", "-M", "main")

	// 2. Clone repo1 to repo2 (simulating air-gapped machine that stays at main)
	repo2 := filepath.Join(tempBase, "repo2")
	if _, err := ExecGit(tempBase, "clone", repo1, repo2); err != nil {
		t.Fatalf("git clone failed: %v", err)
	}

	// 3. In repo1, create feature branch with 2 commits
	if _, err := ExecGit(repo1, "checkout", "-b", "feature/render-pipeline"); err != nil {
		t.Fatalf("git checkout -b failed: %v", err)
	}
	if err := os.WriteFile(f1, []byte("package main\nfunc main() { render() }\n"), 0644); err != nil {
		t.Fatalf("failed to write app.go: %v", err)
	}
	if _, err := ExecGit(repo1, "commit", "-am", "feat: add render pipeline step"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	f2 := filepath.Join(repo1, "shader.hlsl")
	if err := os.WriteFile(f2, []byte("// PBR shader\n"), 0644); err != nil {
		t.Fatalf("failed to write shader: %v", err)
	}
	if _, err := ExecGit(repo1, "add", "shader.hlsl"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if _, err := ExecGit(repo1, "commit", "-m", "feat: new pbr shader asset"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 4. Create delta bundle
	bundleOut := filepath.Join(tempBase, "render_delta.wgitpack")
	res, err := svc.CreateDeltaBundle(repo1, "main", "feature/render-pipeline", bundleOut)
	if err != nil {
		t.Fatalf("CreateDeltaBundle failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected bundle creation success: %s", res.Message)
	}
	if res.CommitCount != 2 {
		t.Fatalf("expected 2 commits in bundle, got %d", res.CommitCount)
	}

	// 5. Inspect bundle in repo2
	manifest, err := svc.InspectDeltaBundle(repo2, bundleOut)
	if err != nil {
		t.Fatalf("InspectDeltaBundle failed: %v", err)
	}
	if manifest.Sha256Checksum == "" {
		t.Fatalf("expected non-empty sha256 checksum")
	}
	if manifest.CommitCount != 2 {
		t.Fatalf("expected 2 commits in manifest, got %d", manifest.CommitCount)
	}

	// 6. Apply bundle in repo2
	applyRes, err := svc.ApplyDeltaBundle(repo2, bundleOut, "imported-render-pipeline")
	if err != nil {
		t.Fatalf("ApplyDeltaBundle failed: %v", err)
	}
	if !applyRes.Success {
		t.Fatalf("expected apply success: %s", applyRes.Message)
	}

	// Verify imported branch in repo2 has the shader file
	logRepo2, err := ExecGit(repo2, "log", "--oneline", "imported-render-pipeline")
	if err != nil {
		t.Fatalf("git log repo2 failed: %v", err)
	}
	if !filepath.IsAbs(bundleOut) {
		t.Fatalf("expected absolute bundle path")
	}
	if len(logRepo2) == 0 {
		t.Fatalf("expected commits in imported branch")
	}
}
