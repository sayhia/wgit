package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DeltaBundleManifest struct {
	BundleVersion  string   `json:"bundleVersion"`
	BaseRef        string   `json:"baseRef"`
	TargetRef      string   `json:"targetRef"`
	CommitCount    int      `json:"commitCount"`
	Commits        []string `json:"commits"`
	HasLfs         bool     `json:"hasLfs"`
	LfsFiles       []string `json:"lfsFiles"`
	Sha256Checksum string   `json:"sha256Checksum"`
	CreatedAt      string   `json:"createdAt"`
	SizeBytes      int64    `json:"sizeBytes"`
}

type DeltaBundleResult struct {
	Success     bool   `json:"success"`
	BundlePath  string `json:"bundlePath"`
	TotalSize   int64  `json:"totalSize"`
	CommitCount int    `json:"commitCount"`
	Message     string `json:"message"`
}

type DeltaApplyResult struct {
	Success        bool   `json:"success"`
	AppliedCommits int    `json:"appliedCommits"`
	TargetBranch   string `json:"targetBranch"`
	Message        string `json:"message"`
}

func computeFileSha256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sanitizeRefForFilename(ref string) string {
	r := strings.ReplaceAll(ref, "/", "_")
	r = strings.ReplaceAll(r, "\\", "_")
	r = strings.ReplaceAll(r, "..", "_")
	r = strings.ReplaceAll(r, "^", "_")
	r = strings.ReplaceAll(r, "~", "_")
	if len(r) > 20 {
		return r[:20]
	}
	return r
}

// CreateDeltaBundle packages commits between baseRef and targetRef into an air-gapped delta pack.
func (GitService) CreateDeltaBundle(repoPath string, baseRef string, targetRef string, outputPath string) (*DeltaBundleResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	baseRef = strings.TrimSpace(baseRef)
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" {
		targetRef = "HEAD"
	}
	if baseRef == "" {
		baseRef = "HEAD~1"
	}

	rangeArg := fmt.Sprintf("%s..%s", baseRef, targetRef)
	logOut, err := ExecGit(repoPath, "log", "--oneline", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate commit range %s: %w", rangeArg, err)
	}

	var commitLines []string
	for _, l := range strings.Split(strings.TrimSpace(logOut), "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			commitLines = append(commitLines, trimmed)
		}
	}

	if len(commitLines) == 0 {
		return &DeltaBundleResult{
			Success:     false,
			BundlePath:  "",
			TotalSize:   0,
			CommitCount: 0,
			Message:     fmt.Sprintf("No commits found in range %s", rangeArg),
		}, nil
	}

	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		safeBase := sanitizeRefForFilename(baseRef)
		safeTarget := sanitizeRefForFilename(targetRef)
		outputPath = filepath.Join(repoPath, fmt.Sprintf("delta_%s_to_%s_%d.wgitpack", safeBase, safeTarget, time.Now().Unix()))
	} else if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(repoPath, outputPath)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, err
	}

	// Create git bundle
	_, err = ExecGit(repoPath, "bundle", "create", outputPath, rangeArg)
	if err != nil {
		return nil, fmt.Errorf("git bundle create failed: %w", err)
	}

	// Calculate checksum and file size
	checksum, _ := computeFileSha256(outputPath)
	fi, _ := os.Stat(outputPath)
	var sizeBytes int64
	if fi != nil {
		sizeBytes = fi.Size()
	}

	manifest := DeltaBundleManifest{
		BundleVersion:  "1.0",
		BaseRef:        baseRef,
		TargetRef:      targetRef,
		CommitCount:    len(commitLines),
		Commits:        commitLines,
		HasLfs:         false,
		LfsFiles:       []string{},
		Sha256Checksum: checksum,
		CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		SizeBytes:      sizeBytes,
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(outputPath+".manifest.json", manifestData, 0644)

	return &DeltaBundleResult{
		Success:     true,
		BundlePath:  outputPath,
		TotalSize:   sizeBytes,
		CommitCount: len(commitLines),
		Message:     fmt.Sprintf("Created delta pack (%d commits, %d bytes)", len(commitLines), sizeBytes),
	}, nil
}

// InspectDeltaBundle verifies and inspects an offline delta bundle file.
func (GitService) InspectDeltaBundle(repoPath string, bundlePath string) (*DeltaBundleManifest, error) {
	if bundlePath == "" {
		return nil, errors.New("bundlePath cannot be empty")
	}

	if _, err := os.Stat(bundlePath); err != nil {
		return nil, fmt.Errorf("bundle file not found: %w", err)
	}

	checksum, err := computeFileSha256(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	fi, err := os.Stat(bundlePath)
	if err != nil {
		return nil, err
	}

	// Try reading companion manifest if available
	manifestPath := bundlePath + ".manifest.json"
	if mData, err := os.ReadFile(manifestPath); err == nil {
		var m DeltaBundleManifest
		if err := json.Unmarshal(mData, &m); err == nil {
			m.Sha256Checksum = checksum
			m.SizeBytes = fi.Size()
			return &m, nil
		}
	}

	// If no manifest file, inspect using git bundle commands
	headsOut, _ := ExecGit(repoPath, "bundle", "list-heads", bundlePath)
	var heads []string
	for _, l := range strings.Split(strings.TrimSpace(headsOut), "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			heads = append(heads, trimmed)
		}
	}

	return &DeltaBundleManifest{
		BundleVersion:  "1.0",
		BaseRef:        "auto",
		TargetRef:      strings.Join(heads, ", "),
		CommitCount:    len(heads),
		Commits:        heads,
		HasLfs:         false,
		LfsFiles:       []string{},
		Sha256Checksum: checksum,
		CreatedAt:      fi.ModTime().Format("2006-01-02 15:04:05"),
		SizeBytes:      fi.Size(),
	}, nil
}

// ApplyDeltaBundle verifies and fetches/applies the offline bundle into the destination repository.
func (GitService) ApplyDeltaBundle(repoPath string, bundlePath string, targetBranch string) (*DeltaApplyResult, error) {
	if repoPath == "" || bundlePath == "" {
		return nil, errors.New("repoPath and bundlePath are required")
	}

	if _, err := os.Stat(bundlePath); err != nil {
		return nil, fmt.Errorf("bundle file not found: %w", err)
	}

	// 1. Verify bundle
	_, err := ExecGit(repoPath, "bundle", "verify", bundlePath)
	if err != nil {
		return &DeltaApplyResult{
			Success:        false,
			AppliedCommits: 0,
			TargetBranch:   targetBranch,
			Message:        fmt.Sprintf("Bundle verification failed: %v", err),
		}, err
	}

	// 2. Resolve heads in bundle
	headsOut, err := ExecGit(repoPath, "bundle", "list-heads", bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list heads from bundle: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(headsOut), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, errors.New("bundle contains no ref heads")
	}

	// Example output: "14a33d25b... refs/heads/feature" or "14a33d25b... HEAD"
	firstHeadParts := strings.Fields(lines[0])
	var sourceRef string
	if len(firstHeadParts) >= 2 {
		sourceRef = firstHeadParts[1]
	} else {
		sourceRef = "HEAD"
	}

	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		if strings.HasPrefix(sourceRef, "refs/heads/") {
			targetBranch = strings.TrimPrefix(sourceRef, "refs/heads/")
		} else {
			targetBranch = fmt.Sprintf("delta_sync_%d", time.Now().Unix())
		}
	}

	// 3. Fetch from bundle into target branch
	fetchRefSpec := fmt.Sprintf("%s:%s", sourceRef, targetBranch)
	_, err = ExecGit(repoPath, "fetch", bundlePath, fetchRefSpec)
	if err != nil {
		// Fallback to fetching HEAD if named ref fails
		_, err = ExecGit(repoPath, "fetch", bundlePath, fmt.Sprintf("HEAD:%s", targetBranch))
		if err != nil {
			return &DeltaApplyResult{
				Success:        false,
				AppliedCommits: 0,
				TargetBranch:   targetBranch,
				Message:        fmt.Sprintf("Failed to fetch commits from bundle: %v", err),
			}, err
		}
	}

	return &DeltaApplyResult{
		Success:        true,
		AppliedCommits: len(lines),
		TargetBranch:   targetBranch,
		Message:        fmt.Sprintf("Successfully applied delta pack to branch '%s'", targetBranch),
	}, nil
}
