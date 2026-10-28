package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ReleaseArtifactsResult struct {
	TagName      string `json:"tagName"`
	ArchiveTarGz string `json:"archiveTarGz"`
	ArchiveZip   string `json:"archiveZip"`
	ChecksumFile string `json:"checksumFile,omitempty"`
	Sha256Digest string `json:"sha256Digest,omitempty"`
	Success      bool   `json:"success"`
}

// CreateReleaseArtifacts builds tar.gz and zip distribution archives for a release tag with optional SHA256 checksums.
func (a *GitService) CreateReleaseArtifacts(repoPath string, tagName string, generateChecksums bool) (*ReleaseArtifactsResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil, fmt.Errorf("tag name is required")
	}

	distDir := filepath.Join(repoPath, "dist", "releases", tagName)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create release artifacts directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tarPath := filepath.Join(distDir, fmt.Sprintf("%s.tar.gz", tagName))
	zipPath := filepath.Join(distDir, fmt.Sprintf("%s.zip", tagName))

	// 1. git archive tar.gz
	cmdTar := exec.CommandContext(ctx, "git", "archive", "--format=tar.gz", fmt.Sprintf("--prefix=%s/", tagName), "-o", tarPath, tagName)
	cmdTar.Dir = repoPath
	if out, err := cmdTar.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create tar.gz archive: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// 2. git archive zip
	cmdZip := exec.CommandContext(ctx, "git", "archive", "--format=zip", fmt.Sprintf("--prefix=%s/", tagName), "-o", zipPath, tagName)
	cmdZip.Dir = repoPath
	if out, err := cmdZip.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create zip archive: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	result := &ReleaseArtifactsResult{
		TagName:      tagName,
		ArchiveTarGz: tarPath,
		ArchiveZip:   zipPath,
		Success:      true,
	}

	// 3. Optional checksum generation
	if generateChecksums {
		tarHash, _ := computeFileSha256(tarPath)
		zipHash, _ := computeFileSha256(zipPath)

		checksumPath := filepath.Join(distDir, fmt.Sprintf("%s-checksums.sha256", tagName))
		content := fmt.Sprintf("%s  %s.tar.gz\n%s  %s.zip\n", tarHash, tagName, zipHash, tagName)
		_ = os.WriteFile(checksumPath, []byte(content), 0644)

		result.ChecksumFile = checksumPath
		result.Sha256Digest = tarHash
	}

	return result, nil
}
