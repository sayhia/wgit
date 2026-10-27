package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitBundleResult captures the output path, size, and unpack instructions of a git bundle.
type GitBundleResult struct {
	OutputPath string `json:"outputPath"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizeHuman  string `json:"sizeHuman"`
	RevSpec    string `json:"revSpec"`
	VerifyHelp string `json:"verifyHelp"`
	CloneHelp  string `json:"cloneHelp"`
	FetchHelp  string `json:"fetchHelp"`
}

// CreateGitBundleAdvanced exports an offline Git bundle file (.bundle) with detailed metrics and unpacking instructions.
func (GitService) CreateGitBundleAdvanced(repoPath string, revSpec string, outputPath string) (*GitBundleResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	revSpec = strings.TrimSpace(revSpec)
	if revSpec == "" {
		revSpec = "HEAD"
	}

	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		repoName := filepath.Base(repoPath)
		cleanRev := strings.ReplaceAll(strings.ReplaceAll(revSpec, "/", "_"), " ", "_")
		if cleanRev == "--all" {
			cleanRev = "all"
		}
		filename := fmt.Sprintf("%s_%s_%s.bundle", repoName, cleanRev, time.Now().Format("20060102_150405"))
		outputPath = filepath.Join(filepath.Dir(repoPath), filename)
	}

	// Ensure target directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for bundle: %w", err)
	}

	// Execute git bundle create
	args := []string{"bundle", "create", outputPath}
	if revSpec == "--all" {
		args = append(args, "--all")
	} else {
		parts := strings.Fields(revSpec)
		args = append(args, parts...)
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("git bundle create failed: %v (%s)", err, strings.TrimSpace(out))
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("bundle file was created but cannot be read: %w", err)
	}

	sizeHuman := formatBundleSize(info.Size())
	baseName := filepath.Base(outputPath)

	return &GitBundleResult{
		OutputPath: outputPath,
		SizeBytes:  info.Size(),
		SizeHuman:  sizeHuman,
		RevSpec:    revSpec,
		VerifyHelp: fmt.Sprintf("git bundle verify %s", outputPath),
		CloneHelp:  fmt.Sprintf("git clone %s %s_offline", outputPath, strings.TrimSuffix(baseName, ".bundle")),
		FetchHelp:  fmt.Sprintf("git fetch %s %s:local-branch", outputPath, revSpec),
	}, nil
}

func formatBundleSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
