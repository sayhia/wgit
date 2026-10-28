package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BloatObjectItem struct {
	Hash         string `json:"hash"`
	Path         string `json:"path"`
	SizeBytes    int64  `json:"sizeBytes"`
	SizeHuman    string `json:"sizeHuman"`
	IsGhostBlob  bool   `json:"isGhostBlob"` // Deleted in HEAD, only exists in history
	CommitHash   string `json:"commitHash"`
	CommitMsg    string `json:"commitMsg"`
	Author       string `json:"author"`
	LastModified string `json:"lastModified"`
}

type RepoBloatReport struct {
	TotalBlobsScanned int64             `json:"totalBlobsScanned"`
	TotalGhostBytes   int64             `json:"totalGhostBytes"`
	GhostBytesHuman   string            `json:"ghostBytesHuman"`
	TopBloatObjects   []BloatObjectItem `json:"topBloatObjects"`
	Summary           string            `json:"summary"`
}

type SlimmingPlanResult struct {
	TargetPaths             []string `json:"targetPaths"`
	EstimatedReclaimedBytes int64    `json:"estimatedReclaimedBytes"`
	EstimatedReclaimedHuman string   `json:"estimatedReclaimedHuman"`
	FilterRepoCommand       string   `json:"filterRepoCommand"`
	BfgCommand              string   `json:"bfgCommand"`
	GitGcCommand            string   `json:"gitGcCommand"`
	SafetyNotice            string   `json:"safetyNotice"`
}

func execGitBloat(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// InspectRepoBloat scans git object database for the largest historical blobs and identifies ghost objects.
func (GitService) InspectRepoBloat(repoPath string, limit int) (*RepoBloatReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// 1. Get objects with rev-list
	revCmd := exec.Command("git", "rev-list", "--objects", "--all")
	revCmd.Dir = repoPath
	revOut, err := revCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe rev-list: %w", err)
	}

	catCmd := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize) %(rest)")
	catCmd.Dir = repoPath
	catCmd.Stdin = revOut

	if err := revCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start rev-list: %w", err)
	}
	catOutput, err := catCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to pipe cat-file: %w", err)
	}
	if err := catCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cat-file: %w", err)
	}

	type rawBlob struct {
		hash string
		size int64
		path string
	}

	var blobs []rawBlob
	var totalScanned int64 = 0

	scanner := bufio.NewScanner(catOutput)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 4)
		if len(parts) >= 3 && parts[1] == "blob" {
			totalScanned++
			size, _ := strconv.ParseInt(parts[2], 10, 64)
			path := ""
			if len(parts) >= 4 {
				path = strings.TrimSpace(parts[3])
			}
			if size > 10*1024 || totalScanned <= 100 { // Track objects > 10KB or first 100
				blobs = append(blobs, rawBlob{
					hash: parts[0],
					size: size,
					path: path,
				})
			}
		}
	}
	_ = revCmd.Wait()
	_ = catCmd.Wait()

	// Sort descending by size
	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].size > blobs[j].size
	})

	// Get HEAD tree object hashes to identify ghost blobs
	headTreeOut, _ := execGitBloat(repoPath, "ls-tree", "-r", "HEAD")
	headHashes := make(map[string]bool)
	for _, l := range strings.Split(headTreeOut, "\n") {
		fields := strings.Fields(l)
		if len(fields) >= 3 {
			headHashes[fields[2]] = true
		}
	}

	if len(blobs) > limit {
		blobs = blobs[:limit]
	}

	var topItems []BloatObjectItem
	var totalGhostBytes int64 = 0

	for _, b := range blobs {
		isGhost := !headHashes[b.hash]
		if isGhost {
			totalGhostBytes += b.size
		}

		item := BloatObjectItem{
			Hash:        b.hash,
			Path:        b.path,
			SizeBytes:   b.size,
			SizeHuman:   formatBytes(b.size),
			IsGhostBlob: isGhost,
		}

		// Find commit info if path is present
		if b.path != "" {
			logOut, _ := execGitBloat(repoPath, "log", "-1", "--format=%H|%s|%an|%ad", "--date=relative", "--", b.path)
			parts := strings.Split(strings.TrimSpace(logOut), "|")
			if len(parts) >= 4 {
				item.CommitHash = parts[0]
				item.CommitMsg = parts[1]
				item.Author = parts[2]
				item.LastModified = parts[3]
			}
		}

		topItems = append(topItems, item)
	}

	summary := fmt.Sprintf("Scanned %d objects. Identified %d top bloat objects, reclaiming up to %s of ghost storage.",
		totalScanned, len(topItems), formatBytes(totalGhostBytes))

	return &RepoBloatReport{
		TotalBlobsScanned: totalScanned,
		TotalGhostBytes:   totalGhostBytes,
		GhostBytesHuman:   formatBytes(totalGhostBytes),
		TopBloatObjects:   topItems,
		Summary:           summary,
	}, nil
}

// GenerateSlimmingPlan generates commands and instructions to purge specified bloat files from history.
func (GitService) GenerateSlimmingPlan(repoPath string, targetPaths []string) (*SlimmingPlanResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if len(targetPaths) == 0 {
		return nil, errors.New("targetPaths cannot be empty")
	}

	var cleanPaths []string
	var totalBytes int64 = 0

	for _, p := range targetPaths {
		p = strings.TrimSpace(p)
		if p != "" {
			cleanPaths = append(cleanPaths, p)
		}
	}

	// Estimate size
	for _, p := range cleanPaths {
		sizeOut, _ := execGitBloat(repoPath, "log", "--all", "--format=", "--stat", "--", p)
		if len(sizeOut) > 0 {
			totalBytes += 1024 * 1024 // Estimate minimum 1MB per bloat path
		}
	}

	pathArgs := strings.Join(cleanPaths, " ")
	filterRepoCmd := fmt.Sprintf("git-filter-repo --invert-paths $(printf ' --path %%s' %s)", pathArgs)
	bfgCmd := fmt.Sprintf("bfg --delete-files '{%s}'", strings.Join(cleanPaths, ","))
	gcCmd := "git reflog expire --expire=now --all && git gc --prune=now --aggressive"

	safetyNotice := "Before running history rewriting commands, create a complete backup copy of your repository folder. History rewriting modifies all commit hashes and requires force-push (git push --force-with-lease) to remotes."

	return &SlimmingPlanResult{
		TargetPaths:             cleanPaths,
		EstimatedReclaimedBytes: totalBytes,
		EstimatedReclaimedHuman: formatBytes(totalBytes),
		FilterRepoCommand:       filterRepoCmd,
		BfgCommand:              bfgCmd,
		GitGcCommand:            gcCmd,
		SafetyNotice:            safetyNotice,
	}, nil
}
