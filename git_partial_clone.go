package main

import (
	"errors"
	"fmt"
	"strings"
)

// PartialCloneStatus represents the partial clone state and promisor filter of a repository.
type PartialCloneStatus struct {
	IsPartialClone bool   `json:"isPartialClone"`
	PromisorRemote string `json:"promisorRemote"`
	FilterSpec     string `json:"filterSpec"`
	MissingBlobs   int    `json:"missingBlobs"`
	Summary        string `json:"summary"`
}

// HydrateBlobsResult represents the outcome of on-demand blob fetching.
type HydrateBlobsResult struct {
	Success       bool     `json:"success"`
	HydratedPaths []string `json:"hydratedPaths"`
	Message       string   `json:"message"`
}

// GetPartialCloneStatus detects whether the repository was cloned with partial clone filters (e.g. --filter=blob:none).
func (GitService) GetPartialCloneStatus(repoPath string) (*PartialCloneStatus, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}

	res := &PartialCloneStatus{
		IsPartialClone: false,
		PromisorRemote: "",
		FilterSpec:     "",
		MissingBlobs:   0,
		Summary:        "标准完整克隆（非 Partial Clone）",
	}

	// 1. Check promisor remote config: git config --get-regexp remote\..*\.promisor
	out, err := ExecGit(repoPath, "config", "--get-regexp", `remote\..*\.promisor`)
	if err == nil && strings.TrimSpace(out) != "" {
		res.IsPartialClone = true
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "true" {
				keyParts := strings.Split(parts[0], ".")
				if len(keyParts) >= 2 {
					res.PromisorRemote = keyParts[1]
					break
				}
			}
		}
	}

	// 2. Check filter spec: git config --get-regexp remote\..*\.partialclonefilter
	outFilter, err := ExecGit(repoPath, "config", "--get-regexp", `remote\..*\.partialclonefilter`)
	if err == nil && strings.TrimSpace(outFilter) != "" {
		res.IsPartialClone = true
		for _, line := range strings.Split(strings.TrimSpace(outFilter), "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				res.FilterSpec = parts[1]
				break
			}
		}
	}

	if res.IsPartialClone {
		if res.PromisorRemote == "" {
			res.PromisorRemote = "origin"
		}
		if res.FilterSpec == "" {
			res.FilterSpec = "blob:none"
		}
		res.Summary = fmt.Sprintf("Partial Clone 活跃中 (远程: %s, 规则: %s)", res.PromisorRemote, res.FilterSpec)
	}

	return res, nil
}

// HydrateBlobs triggers on-demand materialization of blobs for specified file paths from the promisor remote.
func (GitService) HydrateBlobs(repoPath string, paths []string) (*HydrateBlobsResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}

	if len(paths) == 0 {
		return &HydrateBlobsResult{
			Success:       true,
			HydratedPaths: []string{},
			Message:       "没有指定需要拉取的文件路径",
		}, nil
	}

	hydrated := []string{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := ExecGit(repoPath, "checkout", "--", p); err == nil {
			hydrated = append(hydrated, p)
		}
	}

	return &HydrateBlobsResult{
		Success:       true,
		HydratedPaths: hydrated,
		Message:       fmt.Sprintf("已成功从远程拉取并物化 %d 个文件对象", len(hydrated)),
	}, nil
}
