package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioRPCsRejectPathTraversal(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := ExecGit(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Tester"},
		{"config", "user.email", "tester@example.com"},
	} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "base"}} {
		if out, err := ExecGit(repo, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	outside := filepath.Join(parent, "secret.txt")
	secret := []byte("outside the repository")
	if err := os.WriteFile(outside, secret, 0o644); err != nil {
		t.Fatal(err)
	}
	outsideImage := filepath.Join(parent, "secret.png")
	if err := os.WriteFile(outsideImage, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := GitService{}

	if _, err := svc.ResolveSceneStreamConflict(repo, "../secret.txt", "SPATIAL_OFFSET", "guid-1"); err == nil || !strings.Contains(err.Error(), "repository") {
		t.Fatalf("scene resolution must reject ../ paths, got %v", err)
	}
	after, err := os.ReadFile(outside)
	if err != nil || string(after) != string(secret) {
		t.Fatalf("outside file must stay untouched: %q err=%v", after, err)
	}

	for name, call := range map[string]func() error{
		"LintModelMesh": func() error {
			_, err := svc.LintModelMesh(repo, "../secret.txt", "")
			return err
		},
		"LintAnimationRig": func() error {
			_, err := svc.LintAnimationRig(repo, "../secret.txt", "")
			return err
		},
		"DiffUnrealAssetNodes": func() error {
			_, err := svc.DiffUnrealAssetNodes(repo, "../secret.txt", "HEAD", "WORKTREE")
			return err
		},
		"InspectShaderVariants": func() error {
			_, err := svc.InspectShaderVariants(repo, "../secret.txt")
			return err
		},
		"AuditSceneStreamMerge": func() error {
			_, err := svc.AuditSceneStreamMerge(repo, "main", "../secret.txt")
			return err
		},
		"LintModelMeshAbsoluteOutside": func() error {
			_, err := svc.LintModelMesh(repo, outside, "")
			return err
		},
		"GetFileRevisionTimeline": func() error {
			_, err := svc.GetFileRevisionTimeline(repo, "../secret.txt")
			return err
		},
		"GetFileRevisionTimelineInvalidRepo": func() error {
			_, err := svc.GetFileRevisionTimeline(t.TempDir(), "a.txt")
			return err
		},
		"RevertFileToRevision": func() error {
			_, err := svc.RevertFileToRevision(repo, "../secret.txt", "HEAD")
			return err
		},
		"GetConflictFileContent": func() error {
			_, err := svc.GetConflictFileContent(repo, "../secret.txt")
			return err
		},
		"GetFileBlame": func() error {
			_, err := svc.GetFileBlame(repo, "../secret.txt")
			return err
		},
		"GetFileContentAtCommit": func() error {
			_, err := svc.GetFileContentAtCommit(repo, "HEAD", "../secret.txt")
			return err
		},
		"GetFileDiffBetween": func() error {
			_, err := svc.GetFileDiffBetween(repo, "HEAD", "HEAD", "../secret.txt", false)
			return err
		},
		"GetSourceFileDiff": func() error {
			_, err := svc.GetSourceFileDiff(repo, "../secret.txt", "working")
			return err
		},
		"GetSourceFileDetail": func() error {
			_, err := svc.GetSourceFileDetail(repo, "../secret.txt", "working")
			return err
		},
		"GetImageBase64": func() error {
			_, err := svc.GetImageBase64(repo, "WORKING", "../secret.png")
			return err
		},
		"ResolveConflictBackup": func() error {
			return svc.ResolveConflict(repo, "../secret.txt", "backup_and_theirs")
		},
		"OpenExternalDiff": func() error {
			return svc.OpenExternalDiff(repo, "../secret.txt", false, "custom")
		},
		"OpenExternalMerge": func() error {
			return svc.OpenExternalMerge(repo, "../secret.txt", "custom")
		},
		"ScanMissingMaterials": func() error {
			_, err := svc.ScanMissingMaterials(repo, "../secret.txt")
			return err
		},
		"AuditLodStreaming": func() error {
			_, err := svc.AuditLodStreaming(repo, "../secret.txt")
			return err
		},
		"AuditPhysicsMeshCollision": func() error {
			_, err := svc.AuditPhysicsMeshCollision(repo, "../secret.txt")
			return err
		},
		"AuditMaterialInstanceOverrides": func() error {
			_, err := svc.AuditMaterialInstanceOverrides(repo, "../secret.txt")
			return err
		},
		"DetectAssetCyclicDependencies": func() error {
			_, err := svc.DetectAssetCyclicDependencies(repo, "../secret.txt")
			return err
		},
		"AuditTextureChannelPacking": func() error {
			_, err := svc.AuditTextureChannelPacking(repo, "../secret.txt")
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatalf("%s must reject paths outside the repository", name)
		}
	}

	// Absolute paths inside the repository remain supported.
	if err := os.WriteFile(filepath.Join(repo, "model.obj"), []byte("v 0 0 0\nf 1 1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LintModelMesh(repo, filepath.Join(repo, "model.obj"), ""); err != nil {
		t.Fatalf("absolute path inside the repository must be accepted: %v", err)
	}
}
