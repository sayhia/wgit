package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrackExtensionsAsLfsStagesPatternsAndLockable(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	if err := (GitService{}).TrackExtensionsAsLfs(repo, []string{".psd", ".bin", " .fbx "}); err != nil {
		t.Fatalf("TrackExtensionsAsLfs failed: %v", err)
	}

	attrs, err := os.ReadFile(filepath.Join(repo, ".gitattributes"))
	if err != nil {
		t.Fatalf("read .gitattributes: %v", err)
	}
	content := string(attrs)
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "*.psd "):
			if !strings.Contains(line, "lockable") {
				t.Fatalf("psd should be lockable: %q", line)
			}
		case strings.HasPrefix(line, "*.fbx "):
			if !strings.Contains(line, "lockable") {
				t.Fatalf("fbx should be lockable: %q", line)
			}
		}
	}
	if !strings.Contains(content, "*.psd ") || !strings.Contains(content, "*.bin ") || !strings.Contains(content, "*.fbx ") {
		t.Fatalf("missing tracked patterns: %q", content)
	}

	staged, err := ExecGit(repo, "diff", "--cached", "--name-only")
	if err != nil || !strings.Contains(staged, ".gitattributes") {
		t.Fatalf("staged files = %q err=%v", staged, err)
	}
}

func TestTrackExtensionsAsLfsReportsGitLfsFailure(t *testing.T) {
	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.keep text\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git not found: %v", err)
	}
	binDir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(binDir, "git")); err != nil {
		t.Skipf("cannot create git symlink: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("GIT_EXEC_PATH", t.TempDir())

	err = (GitService{}).TrackExtensionsAsLfs(repo, []string{".png"})
	if err == nil {
		t.Fatal("expected an error when git-lfs is unavailable")
	}
	attrs, readErr := os.ReadFile(filepath.Join(repo, ".gitattributes"))
	if readErr != nil {
		t.Fatalf("read .gitattributes: %v", readErr)
	}
	if strings.Contains(string(attrs), "*.png") {
		t.Fatalf("failed tracking should not modify .gitattributes: %q", attrs)
	}
}

func TestTrackLfsPatternStagesAttributes(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := t.TempDir()
	if out, err := ExecGit(repo, "init", "-q"); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, out)
	}
	svc := GitService{}
	if err := svc.TrackLfsPattern(repo, "*.bin"); err != nil {
		t.Fatalf("TrackLfsPattern failed: %v", err)
	}
	attrs, err := os.ReadFile(filepath.Join(repo, ".gitattributes"))
	if err != nil || !strings.Contains(string(attrs), "*.bin filter=lfs") {
		t.Fatalf("tracked pattern missing: %q err=%v", attrs, err)
	}
	staged, err := ExecGit(repo, "diff", "--cached", "--name-only")
	if err != nil || !strings.Contains(staged, ".gitattributes") {
		t.Fatalf("track should stage .gitattributes, staged=%q err=%v", staged, err)
	}

	if err := svc.UntrackLfsPattern(repo, "*.bin"); err != nil {
		t.Fatalf("UntrackLfsPattern failed: %v", err)
	}
	attrs, err = os.ReadFile(filepath.Join(repo, ".gitattributes"))
	if err != nil || strings.Contains(string(attrs), "*.bin") {
		t.Fatalf("pattern should be removed after untrack: %q err=%v", attrs, err)
	}
	staged, err = ExecGit(repo, "diff", "--cached", "--name-only")
	if err != nil || !strings.Contains(staged, ".gitattributes") {
		t.Fatalf("untrack should stage .gitattributes, staged=%q err=%v", staged, err)
	}
}
