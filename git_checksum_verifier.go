package main

import (
	"fmt"
	"os"
	"strings"
)

// ChecksumVerifyResult reports whether a distribution artifact's SHA-256 matches an expected digest.
type ChecksumVerifyResult struct {
	FilePath     string `json:"filePath"`
	ComputedHash string `json:"computedHash"`
	ExpectedHash string `json:"expectedHash"`
	Matches      bool   `json:"matches"`
	SizeBytes    int64  `json:"sizeBytes"`
	Message      string `json:"message"`
}

// VerifyArtifactChecksum computes the SHA-256 digest of a local artifact file and compares it against expectedHash.
func (a *GitService) VerifyArtifactChecksum(filePath string, expectedHash string) (*ChecksumVerifyResult, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("artifact file not found: %w", err)
	}

	computed, err := computeFileSha256(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute SHA-256: %w", err)
	}

	expectedClean := strings.ToLower(strings.TrimSpace(expectedHash))
	matches := (expectedClean != "" && expectedClean == strings.ToLower(computed))

	msg := "校验和完全匹配，文件完整性已验证"
	if expectedClean == "" {
		msg = "已成功计算文件 SHA-256 哈希值"
	} else if !matches {
		msg = "校验和不匹配，文件可能存在损坏或篡改"
	}

	return &ChecksumVerifyResult{
		FilePath:     filePath,
		ComputedHash: computed,
		ExpectedHash: expectedHash,
		Matches:      matches,
		SizeBytes:    fi.Size(),
		Message:      msg,
	}, nil
}
