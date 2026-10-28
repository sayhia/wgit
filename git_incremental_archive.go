package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportArchiveResult holds metadata about the created incremental archive.
type ExportArchiveResult struct {
	Success        bool     `json:"success"`
	OutputPath     string   `json:"outputPath"`
	FileCount      int      `json:"fileCount"`
	TotalSizeBytes int64    `json:"totalSizeBytes"`
	FormattedSize  string   `json:"formattedSize"`
	Files          []string `json:"files"`
}

// ExportChangedFilesArchive creates a zip archive of all modified and added files between fromRef and toRef.
func (GitService) ExportChangedFilesArchive(repoPath string, fromRef string, toRef string, format string, outputPath string) (*ExportArchiveResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path cannot be empty")
	}

	from := strings.TrimSpace(fromRef)
	to := strings.TrimSpace(toRef)
	if to == "" {
		to = "HEAD"
	}
	if from == "" {
		from = to + "^"
	}

	rangeArg := fmt.Sprintf("%s..%s", from, to)
	// Filter for Added, Copied, Modified, Renamed (skip Deleted)
	out, err := ExecGit(repoPath, "diff", "--name-only", "--diff-filter=ACMR", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("failed to query changed files: %w", err)
	}

	rawFiles := strings.Split(out, "\n")
	var changedFiles []string
	for _, f := range rawFiles {
		f = strings.TrimSpace(f)
		if f != "" {
			changedFiles = append(changedFiles, f)
		}
	}

	if len(changedFiles) == 0 {
		return nil, errors.New("所选提交区间内无新增或修改的文件可供归档")
	}

	destPath := strings.TrimSpace(outputPath)
	if destPath == "" {
		fileName := fmt.Sprintf("changed-files-%s-%s-%d.zip", sanitizeRefForFilename(from), sanitizeRefForFilename(to), time.Now().Unix())
		destPath = filepath.Join(os.TempDir(), fileName)
	} else if !filepath.IsAbs(destPath) {
		destPath = filepath.Join(repoPath, destPath)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("创建导出目录失败: %w", err)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("创建归档文件失败: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)

	var totalBytes int64
	for _, relPath := range changedFiles {
		content, showErr := ExecGitRaw(repoPath, "show", fmt.Sprintf("%s:%s", to, relPath))
		if showErr != nil {
			diskPath := filepath.Join(repoPath, relPath)
			diskBytes, readErr := os.ReadFile(diskPath)
			if readErr != nil {
				continue
			}
			content = string(diskBytes)
		}

		w, err := zipWriter.Create(relPath)
		if err != nil {
			continue
		}
		bytesWritten, err := io.WriteString(w, content)
		if err == nil {
			totalBytes += int64(bytesWritten)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("关闭压缩文件失败: %w", err)
	}

	fileInfo, err := os.Stat(destPath)
	var finalSize int64
	if err == nil {
		finalSize = fileInfo.Size()
	} else {
		finalSize = totalBytes
	}

	return &ExportArchiveResult{
		Success:        true,
		OutputPath:     destPath,
		FileCount:      len(changedFiles),
		TotalSizeBytes: finalSize,
		FormattedSize:  formatBytes(finalSize),
		Files:          changedFiles,
	}, nil
}
