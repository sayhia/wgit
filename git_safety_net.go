package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SafetyNetSnapshot represents a temporary backup snapshot of discarded uncommitted files.
type SafetyNetSnapshot struct {
	ID        string   `json:"id"`
	Timestamp int64    `json:"timestamp"`
	Paths     []string `json:"paths"`
	Count     int      `json:"count"`
}

type safetyNetManifest struct {
	ID        string   `json:"id"`
	Timestamp int64    `json:"timestamp"`
	Files     []string `json:"files"`
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err == nil {
		_ = os.Chmod(dst, info.Mode())
	}
	return nil
}

// DiscardWithSafetyNet backs up uncommitted files into a transient recycle bin before discarding them.
func (a *GitService) DiscardWithSafetyNet(repoPath string, paths []string) (*SafetyNetSnapshot, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	snapshotID := fmt.Sprintf("snap_%d", time.Now().UnixNano())
	recycleBinDir := filepath.Join(repoPath, ".git", "wgit_recycle_bin", snapshotID)
	if err := os.MkdirAll(recycleBinDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create recycle bin directory: %w", err)
	}

	// 1. If paths is empty, find all modified, staged and untracked files
	filesToBackup := paths
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if len(filesToBackup) == 0 {
		cmdStatus := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z")
		cmdStatus.Dir = repoPath
		out, err := cmdStatus.Output()
		if err == nil {
			entries := strings.Split(string(out), "\x00")
			for _, entry := range entries {
				if len(entry) > 3 {
					relPath := entry[3:]
					filesToBackup = append(filesToBackup, relPath)
				}
			}
		}
	}

	// 2. Backup existing files into recycle bin
	var backedUpFiles []string
	for _, rel := range filesToBackup {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		srcPath := filepath.Join(repoPath, filepath.FromSlash(rel))
		dstPath := filepath.Join(recycleBinDir, "files", filepath.FromSlash(rel))

		info, err := os.Stat(srcPath)
		if err == nil && !info.IsDir() {
			if err := copyFile(srcPath, dstPath); err == nil {
				backedUpFiles = append(backedUpFiles, rel)
			}
		}
	}

	// 3. Write manifest
	manifest := safetyNetManifest{
		ID:        snapshotID,
		Timestamp: time.Now().Unix(),
		Files:     backedUpFiles,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(recycleBinDir, "manifest.json"), manifestBytes, 0644)

	// 4. Perform actual discard separating tracked and untracked files
	if len(paths) == 0 {
		cmdRestore := exec.CommandContext(ctx, "git", "restore", "--staged", "--worktree", ".")
		cmdRestore.Dir = repoPath
		_ = cmdRestore.Run()

		cmdClean := exec.CommandContext(ctx, "git", "clean", "-fd")
		cmdClean.Dir = repoPath
		_ = cmdClean.Run()
	} else {
		var trackedPaths []string
		var untrackedPaths []string

		for _, rel := range paths {
			checkCmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", rel)
			checkCmd.Dir = repoPath
			if err := checkCmd.Run(); err == nil {
				trackedPaths = append(trackedPaths, rel)
			} else {
				untrackedPaths = append(untrackedPaths, rel)
			}
		}

		if len(trackedPaths) > 0 {
			restoreArgs := append([]string{"restore", "--staged", "--worktree", "--"}, trackedPaths...)
			cmdRestore := exec.CommandContext(ctx, "git", restoreArgs...)
			cmdRestore.Dir = repoPath
			_ = cmdRestore.Run()
		}

		for _, untracked := range untrackedPaths {
			target := filepath.Join(repoPath, filepath.FromSlash(untracked))
			_ = os.RemoveAll(target)
		}
	}

	return &SafetyNetSnapshot{
		ID:        snapshotID,
		Timestamp: manifest.Timestamp,
		Paths:     backedUpFiles,
		Count:     len(backedUpFiles),
	}, nil
}

// RestoreFromSafetyNet restores backed up files from the specified snapshot.
func (a *GitService) RestoreFromSafetyNet(repoPath string, snapshotID string) error {
	if repoPath == "" {
		return fmt.Errorf("repository path is required")
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return fmt.Errorf("snapshot ID is required")
	}

	recycleBinDir := filepath.Join(repoPath, ".git", "wgit_recycle_bin", snapshotID)
	manifestPath := filepath.Join(recycleBinDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("safety net snapshot %q not found or corrupted: %w", snapshotID, err)
	}

	var manifest safetyNetManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse safety net manifest: %w", err)
	}

	filesBase := filepath.Join(recycleBinDir, "files")
	for _, rel := range manifest.Files {
		src := filepath.Join(filesBase, filepath.FromSlash(rel))
		dst := filepath.Join(repoPath, filepath.FromSlash(rel))
		if info, err := os.Stat(src); err == nil && !info.IsDir() {
			_ = copyFile(src, dst)
		}
	}

	return nil
}
