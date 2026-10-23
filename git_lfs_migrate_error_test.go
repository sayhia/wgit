package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewLfsMigrationPropagatesFailureWithOutput(t *testing.T) {
	binDir := t.TempDir()
	fakeLfs := filepath.Join(binDir, "git-lfs")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"version\" ]; then echo 'git-lfs/3.0.0'; exit 0; fi\n" +
		"echo 'migrate: scanning history'\n" +
		"echo 'fatal: invalid pattern' >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(fakeLfs, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := t.TempDir()
	if _, err := (GitService{}).InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if _, err := (GitService{}).PreviewLfsMigration(repo, []string{"*.psd"}, true, ""); err == nil || !strings.Contains(err.Error(), "预览失败") {
		t.Fatalf("failed migrate info must not be parsed as a preview: %v", err)
	}
}
