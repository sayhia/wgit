package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateReleaseArtifactsInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-test-rel-art-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runCmd := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runCmd("init", "-b", "main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	f := filepath.Join(tempDir, "app.go")
	_ = os.WriteFile(f, []byte("package main\n\nfunc main() {}\n"), 0644)
	runCmd("add", ".")
	runCmd("commit", "-m", "release v1.0.0 commit")
	runCmd("tag", "-a", "v1.0.0", "-m", "Release v1.0.0")

	svc := &GitService{}
	res, err := svc.CreateReleaseArtifacts(tempDir, "v1.0.0", true)
	if err != nil {
		t.Fatalf("CreateReleaseArtifacts failed: %v", err)
	}

	if _, err := os.Stat(res.ArchiveTarGz); err != nil {
		t.Fatalf("expected tar.gz archive to exist: %v", err)
	}
	if _, err := os.Stat(res.ArchiveZip); err != nil {
		t.Fatalf("expected zip archive to exist: %v", err)
	}
	if res.ChecksumFile == "" {
		t.Fatalf("expected checksum file, got empty")
	}
	if _, err := os.Stat(res.ChecksumFile); err != nil {
		t.Fatalf("expected checksum file to exist: %v", err)
	}
	t.Logf("Release artifacts successfully built: tar.gz=%s zip=%s sha256=%s", res.ArchiveTarGz, res.ArchiveZip, res.Sha256Digest)
}
