package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type WorkspaceJunkCategory struct {
	CategoryID  string   `json:"categoryId"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FileCount   int      `json:"fileCount"`
	SizeBytes   int64    `json:"sizeBytes"`
	Files       []string `json:"files"`
}

type WorkspaceJunkReport struct {
	TotalJunkSize        int64                   `json:"totalJunkSize"`
	TotalFiles           int                     `json:"totalFiles"`
	Categories           []WorkspaceJunkCategory `json:"categories"`
	RecommendedGitIgnore []string                `json:"recommendedGitIgnore"`
	Summary              string                  `json:"summary"`
}

type SafeCleanResult struct {
	CleanedFilesCount int      `json:"cleanedFilesCount"`
	ReclaimedBytes    int64    `json:"reclaimedBytes"`
	RemovedFiles      []string `json:"removedFiles"`
	Summary           string   `json:"summary"`
}

func execGitCleaner(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func getPathSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi != nil && !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func categorizeJunkPath(relPath string) string {
	lower := strings.ToLower(relPath)
	base := strings.ToLower(filepath.Base(relPath))

	// OS artifacts
	if base == ".ds_store" || base == "thumbs.db" || base == "desktop.ini" || strings.HasSuffix(lower, ".directory") {
		return "osArtifacts"
	}

	// Logs and temp
	if strings.HasSuffix(lower, ".log") || strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".swp") || strings.HasSuffix(lower, ".bak") || strings.HasSuffix(lower, ".pid") {
		return "logAndTemp"
	}

	// Build artifacts
	if strings.Contains(lower, "node_modules") || strings.HasPrefix(lower, "dist/") || strings.HasPrefix(lower, "build/") ||
		strings.HasPrefix(lower, "bin/") || strings.HasPrefix(lower, "obj/") || strings.HasPrefix(lower, "target/") ||
		strings.HasPrefix(lower, "out/") || strings.HasSuffix(lower, ".pyc") || strings.Contains(lower, "__pycache__") {
		return "buildArtifacts"
	}

	return "untrackedFiles"
}

// ScanWorkspaceJunk scans for untracked and ignored files, grouping them into hygiene categories.
func (GitService) ScanWorkspaceJunk(repoPath string, scanIgnored bool) (*WorkspaceJunkReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	args := []string{"status", "--porcelain", "-uall"}
	if scanIgnored {
		args = append(args, "--ignored")
	}

	out, err := execGitCleaner(repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace junk: %v (%s)", err, strings.TrimSpace(out))
	}

	catMap := map[string]*WorkspaceJunkCategory{
		"osArtifacts": {
			CategoryID:  "osArtifacts",
			Title:       "OS 垃圾文件 (OS Artifacts)",
			Description: ".DS_Store, Thumbs.db 等系统自动生成的非代码临时文件",
			Files:       []string{},
		},
		"logAndTemp": {
			CategoryID:  "logAndTemp",
			Title:       "日志与临时文件 (Logs & Temp)",
			Description: "*.log, *.tmp, *.swp 等诊断与编辑器临时快照",
			Files:       []string{},
		},
		"buildArtifacts": {
			CategoryID:  "buildArtifacts",
			Title:       "编译与依赖产物 (Build & Dependencies)",
			Description: "dist/, build/, bin/, obj/, target/ 等可重现编译输出",
			Files:       []string{},
		},
		"untrackedFiles": {
			CategoryID:  "untrackedFiles",
			Title:       "未跟踪源码与资源 (Untracked Files)",
			Description: "尚未纳入版本控制的普通新增文件",
			Files:       []string{},
		},
	}

	recommendationSet := make(map[string]bool)
	var totalSize int64
	var totalFiles int

	lines := strings.Split(out, "\n")
	seenDirs := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 4 {
			continue
		}
		status := trimmed[:2]
		if status != "??" && status != "!!" {
			continue
		}

		relPath := strings.TrimSpace(trimmed[2:])
		fullPath := filepath.Join(repoPath, relPath)

		// Check if inside already handled directory
		isSubdir := false
		for d := range seenDirs {
			if strings.HasPrefix(relPath, d+"/") {
				isSubdir = true
				break
			}
		}
		if isSubdir {
			continue
		}

		catKey := categorizeJunkPath(relPath)
		cat := catMap[catKey]
		if cat == nil {
			cat = catMap["untrackedFiles"]
		}

		size := getPathSize(fullPath)
		cat.Files = append(cat.Files, relPath)
		cat.FileCount++
		cat.SizeBytes += size

		totalFiles++
		totalSize += size

		// Check ignore suggestions
		lower := strings.ToLower(relPath)
		base := strings.ToLower(filepath.Base(relPath))
		if base == ".ds_store" {
			recommendationSet["**/.DS_Store"] = true
		} else if base == "thumbs.db" {
			recommendationSet["**/Thumbs.db"] = true
		} else if strings.HasSuffix(lower, ".log") {
			recommendationSet["*.log"] = true
		} else if strings.HasSuffix(lower, ".tmp") {
			recommendationSet["*.tmp"] = true
		} else if strings.Contains(lower, "node_modules") {
			recommendationSet["**/node_modules/"] = true
		}
	}

	categories := []WorkspaceJunkCategory{
		*catMap["osArtifacts"],
		*catMap["logAndTemp"],
		*catMap["buildArtifacts"],
		*catMap["untrackedFiles"],
	}

	recommendations := make([]string, 0, len(recommendationSet))
	for rec := range recommendationSet {
		recommendations = append(recommendations, rec)
	}

	summary := fmt.Sprintf("Scanned %d files (%s) across %d categories", totalFiles, formatBytes(totalSize), len(categories))

	return &WorkspaceJunkReport{
		TotalJunkSize:        totalSize,
		TotalFiles:           totalFiles,
		Categories:           categories,
		RecommendedGitIgnore: recommendations,
		Summary:              summary,
	}, nil
}

// ExecuteSafeClean deletes junk files in selected categories.
func (s GitService) ExecuteSafeClean(repoPath string, categories []string, dryRun bool) (*SafeCleanResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	rep, err := s.ScanWorkspaceJunk(repoPath, true)
	if err != nil {
		return nil, err
	}

	catFilter := make(map[string]bool)
	for _, c := range categories {
		catFilter[c] = true
	}

	var removedFiles []string
	var reclaimedBytes int64

	for _, cat := range rep.Categories {
		if !catFilter[cat.CategoryID] {
			continue
		}
		for _, f := range cat.Files {
			fullPath := filepath.Join(repoPath, f)
			size := getPathSize(fullPath)
			if !dryRun {
				_ = os.RemoveAll(fullPath)
			}
			removedFiles = append(removedFiles, f)
			reclaimedBytes += size
		}
	}

	actionStr := "Cleaned"
	if dryRun {
		actionStr = "Simulated clean of"
	}
	summary := fmt.Sprintf("%s %d files, reclaimed %s", actionStr, len(removedFiles), formatBytes(reclaimedBytes))

	return &SafeCleanResult{
		CleanedFilesCount: len(removedFiles),
		ReclaimedBytes:    reclaimedBytes,
		RemovedFiles:      removedFiles,
		Summary:           summary,
	}, nil
}

// AppendRecommendedGitIgnore adds recommended patterns to repository .gitignore without duplication.
func (GitService) AppendRecommendedGitIgnore(repoPath string, rules []string) (bool, error) {
	if repoPath == "" {
		return false, errors.New("repoPath cannot be empty")
	}
	if len(rules) == 0 {
		return true, nil
	}

	ignoreFile := filepath.Join(repoPath, ".gitignore")
	existingContent := ""
	data, err := os.ReadFile(ignoreFile)
	if err == nil {
		existingContent = string(data)
	}

	existingLines := strings.Split(existingContent, "\n")
	seen := make(map[string]bool)
	for _, line := range existingLines {
		seen[strings.TrimSpace(line)] = true
	}

	var toAppend []string
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r != "" && !seen[r] {
			toAppend = append(toAppend, r)
			seen[r] = true
		}
	}

	if len(toAppend) == 0 {
		return true, nil
	}

	newContent := existingContent
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n# Auto-appended by wgit Workspace Hygiene\n"
	newContent += strings.Join(toAppend, "\n") + "\n"

	err = os.WriteFile(ignoreFile, []byte(newContent), 0644)
	return err == nil, err
}
