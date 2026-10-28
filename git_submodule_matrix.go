package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SubmoduleMatrixItem details the status, pointer synchronization, and purity of a Git submodule.
type SubmoduleMatrixItem struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	URL            string `json:"url"`
	HeadCommit     string `json:"headCommit"`
	LockedCommit   string `json:"lockedCommit"`
	IsDetached     bool   `json:"isDetached"`
	IsClean        bool   `json:"isClean"`
	IsSynchronized bool   `json:"isSynchronized"`
	Depth          int    `json:"depth"`
}

// SubmoduleSyncResult reports the outcome of recursively synchronizing all repository submodules.
type SubmoduleSyncResult struct {
	SyncedCount int      `json:"syncedCount"`
	Errors      []string `json:"errors"`
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
}

// GetSubmoduleMatrix audits all submodules recursively, checking pointer alignment and working tree purity.
func (a *GitService) GetSubmoduleMatrix(repoPath string) ([]SubmoduleMatrixItem, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "submodule", "status", "--recursive")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// If git submodule status fails because no submodules or .gitmodules absent, return empty slice
		return []SubmoduleMatrixItem{}, nil
	}

	items := []SubmoduleMatrixItem{}
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Git submodule status format: [ -+U]<40-hex-sha> <path> [(<describe>)]
		var statusPrefix byte
		cleanLine := line
		if line[0] == '-' || line[0] == '+' || line[0] == 'U' || line[0] == ' ' {
			statusPrefix = line[0]
			cleanLine = strings.TrimSpace(line[1:])
		}

		fields := strings.Fields(cleanLine)
		if len(fields) < 2 {
			continue
		}

		lockedCommit := fields[0]
		subPath := fields[1]

		depth := strings.Count(filepath.ToSlash(subPath), "/")
		name := filepath.Base(subPath)

		item := SubmoduleMatrixItem{
			Name:           name,
			Path:           subPath,
			LockedCommit:   lockedCommit,
			IsSynchronized: (statusPrefix != '+' && statusPrefix != '-'),
			Depth:          depth,
		}

		// Query submodule actual HEAD and clean status if directory exists
		fullSubPath := filepath.Join(repoPath, subPath)
		cmdHead := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
		cmdHead.Dir = fullSubPath
		if hOut, err := cmdHead.Output(); err == nil {
			item.HeadCommit = strings.TrimSpace(string(hOut))
		} else {
			item.HeadCommit = lockedCommit
		}

		// Check detached HEAD
		cmdSym := exec.CommandContext(ctx, "git", "symbolic-ref", "-q", "HEAD")
		cmdSym.Dir = fullSubPath
		item.IsDetached = (cmdSym.Run() != nil)

		// Check submodule clean status
		cmdStatus := exec.CommandContext(ctx, "git", "status", "--porcelain")
		cmdStatus.Dir = fullSubPath
		if sOut, err := cmdStatus.Output(); err == nil {
			item.IsClean = (len(bytes.TrimSpace(sOut)) == 0)
		} else {
			item.IsClean = true
		}

		// Query URL from .gitmodules
		cmdUrl := exec.CommandContext(ctx, "git", "config", "-f", ".gitmodules", "--get", fmt.Sprintf("submodule.%s.url", name))
		cmdUrl.Dir = repoPath
		if uOut, err := cmdUrl.Output(); err == nil {
			item.URL = strings.TrimSpace(string(uOut))
		}

		items = append(items, item)
	}

	return items, nil
}

// SyncSubmodulesRecursive updates, registers, and syncs all submodules recursively to parent-locked commits.
func (a *GitService) SyncSubmodulesRecursive(repoPath string) (*SubmoduleSyncResult, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()

	res := &SubmoduleSyncResult{
		Errors: []string{},
	}

	if err != nil {
		res.Success = false
		res.Errors = append(res.Errors, strings.TrimSpace(string(out)))
		res.Message = "子模块递归同步过程中出现错误"
		return res, nil
	}

	// Count submodules
	matrix, _ := a.GetSubmoduleMatrix(repoPath)
	res.SyncedCount = len(matrix)
	res.Success = true
	res.Message = fmt.Sprintf("成功递归同步 %d 个子模块至基准锁定版本", res.SyncedCount)

	return res, nil
}
