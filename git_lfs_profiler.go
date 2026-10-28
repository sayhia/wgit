package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LfsTypeWeight represents size and file count breakdown for a specific file extension in Git LFS.
type LfsTypeWeight struct {
	Extension      string  `json:"extension"`
	FileCount      int     `json:"fileCount"`
	TotalSizeBytes int64   `json:"totalSizeBytes"`
	FormattedSize  string  `json:"formattedSize"`
	Percentage     float64 `json:"percentage"`
}

// LfsStorageAnalysis summarizes the local LFS object store weight, category distribution and prune advice.
type LfsStorageAnalysis struct {
	TotalSizeBytes    int64           `json:"totalSizeBytes"`
	FormattedTotal    string          `json:"formattedTotal"`
	ObjectCount       int             `json:"objectCount"`
	TypeBreakdown     []LfsTypeWeight `json:"typeBreakdown"`
	UnreferencedBytes int64           `json:"unreferencedBytes"`
	FormattedUnref    string          `json:"formattedUnref"`
	UnreferencedCount int             `json:"unreferencedCount"`
	PruneAdvice       string          `json:"pruneAdvice"`
}

// AnalyzeLfsStorage scans the local Git LFS object cache and analyzes asset weight by extension.
func (a *GitService) AnalyzeLfsStorage(repoPath string) (*LfsStorageAnalysis, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	analysis := &LfsStorageAnalysis{
		TypeBreakdown: []LfsTypeWeight{},
	}

	// 1. Scan .git/lfs/objects disk usage
	lfsObjectsDir := filepath.Join(repoPath, ".git", "lfs", "objects")
	if info, err := os.Stat(lfsObjectsDir); err == nil && info.IsDir() {
		_ = filepath.Walk(lfsObjectsDir, func(path string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				analysis.TotalSizeBytes += fi.Size()
				analysis.ObjectCount++
			}
			return nil
		})
	}
	analysis.FormattedTotal = formatBytes(analysis.TotalSizeBytes)

	// 2. Query tracked LFS files to group by extension
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmdLs := exec.CommandContext(ctx, "git", "lfs", "ls-files")
	cmdLs.Dir = repoPath
	lsOut, _ := cmdLs.Output()

	type extAgg struct {
		count int
		size  int64
	}
	extMap := make(map[string]*extAgg)

	for _, line := range strings.Split(string(lsOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<oid> * <path>" or "<oid> - <path>"
		parts := strings.SplitN(line, " ", 3)
		var relPath string
		if len(parts) >= 3 {
			relPath = strings.TrimSpace(parts[2])
		} else {
			relPath = line
		}

		ext := strings.ToLower(filepath.Ext(relPath))
		if ext == "" {
			ext = "(no ext)"
		}

		if _, exists := extMap[ext]; !exists {
			extMap[ext] = &extAgg{}
		}
		extMap[ext].count++

		// Query file size on disk if available
		filePath := filepath.Join(repoPath, filepath.FromSlash(relPath))
		if fi, err := os.Stat(filePath); err == nil && !fi.IsDir() {
			extMap[ext].size += fi.Size()
		}
	}

	for ext, agg := range extMap {
		var pct float64
		if analysis.TotalSizeBytes > 0 {
			pct = (float64(agg.size) / float64(analysis.TotalSizeBytes)) * 100.0
			if pct > 100.0 {
				pct = 100.0
			}
		}
		analysis.TypeBreakdown = append(analysis.TypeBreakdown, LfsTypeWeight{
			Extension:      ext,
			FileCount:      agg.count,
			TotalSizeBytes: agg.size,
			FormattedSize:  formatBytes(agg.size),
			Percentage:     pct,
		})
	}

	sort.Slice(analysis.TypeBreakdown, func(i, j int) bool {
		if analysis.TypeBreakdown[i].TotalSizeBytes == analysis.TypeBreakdown[j].TotalSizeBytes {
			return analysis.TypeBreakdown[i].FileCount > analysis.TypeBreakdown[j].FileCount
		}
		return analysis.TypeBreakdown[i].TotalSizeBytes > analysis.TypeBreakdown[j].TotalSizeBytes
	})

	// 3. Check unreferenced prunable objects via git lfs prune --dry-run
	cmdPrune := exec.CommandContext(ctx, "git", "lfs", "prune", "--dry-run")
	cmdPrune.Dir = repoPath
	pruneOut, _ := cmdPrune.CombinedOutput()
	pruneStr := string(pruneOut)

	rePrune := regexp.MustCompile(`(?i)(\d+)\s+to prune`)
	if m := rePrune.FindStringSubmatch(pruneStr); len(m) > 1 {
		var prunableCount int
		_, _ = fmt.Sscanf(m[1], "%d", &prunableCount)
		analysis.UnreferencedCount = prunableCount
	}

	if analysis.UnreferencedCount > 0 {
		analysis.PruneAdvice = fmt.Sprintf("发现 %d 个已废弃的悬空 LFS 缓存对象，建议执行清理", analysis.UnreferencedCount)
	} else {
		analysis.PruneAdvice = "所有本地 LFS 缓存均被当前引用正常持有，存储健康"
	}
	analysis.FormattedUnref = formatBytes(analysis.UnreferencedBytes)

	return analysis, nil
}
