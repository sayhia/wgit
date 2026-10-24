package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectAssetMetadataReportsLfsPointerFailure(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-q", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.png filter=lfs diff=lfs merge=lfs -text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Assets", "hero.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git: %v", err)
	}
	binDir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "lfs" ] && [ "$2" = "pointer" ]; then
  echo "simulated lfs pointer failure" >&2
  exit 128
fi
exec %q "$@"
`, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	meta, err := (GitService{}).InspectAssetMetadata(repo, "Assets/hero.png")
	if err != nil || meta == nil {
		t.Fatalf("InspectAssetMetadata failed: %+v err=%v", meta, err)
	}
	if !meta.IsLFS {
		t.Fatalf("expected the asset to be detected as LFS: %+v", meta)
	}
	if !strings.Contains(meta.LFSPointerError, "读取 LFS 指针失败") {
		t.Fatalf("pointer read failure must be surfaced, got: %+v", meta)
	}
}
