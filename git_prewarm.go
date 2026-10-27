package main

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type RepoCachePrewarmResult struct {
	Success           bool     `json:"success"`
	RepoPath          string   `json:"repoPath"`
	BranchCount       int      `json:"branchCount"`
	CurrentBranch     string   `json:"currentBranch"`
	IsClean           bool     `json:"isClean"`
	DurationMs        int64    `json:"durationMs"`
	PrewarmedEntities []string `json:"prewarmedEntities"`
}

func (s GitService) PrewarmRepoCache(repoPath string) (*RepoCachePrewarmResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	start := time.Now()
	var (
		wg            sync.WaitGroup
		branchCount   int
		currentBranch string
		isClean       bool
		entities      []string
		mu            sync.Mutex
	)

	addEntity := func(name string) {
		mu.Lock()
		entities = append(entities, name)
		mu.Unlock()
	}

	// 1. Current branch & branches list
	wg.Add(1)
	go func() {
		defer wg.Done()
		if out, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			mu.Lock()
			currentBranch = strings.TrimSpace(out)
			mu.Unlock()
		}
		if out, err := ExecGit(repoPath, "branch", "--format=%(refname:short)"); err == nil {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			count := 0
			for _, l := range lines {
				if strings.TrimSpace(l) != "" {
					count++
				}
			}
			mu.Lock()
			branchCount = count
			mu.Unlock()
		}
		addEntity("branches")
	}()

	// 2. Status check
	wg.Add(1)
	go func() {
		defer wg.Done()
		if out, err := ExecGit(repoPath, "status", "--porcelain"); err == nil {
			clean := len(strings.TrimSpace(out)) == 0
			mu.Lock()
			isClean = clean
			mu.Unlock()
		}
		addEntity("status")
	}()

	// 3. Remotes
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = ExecGit(repoPath, "remote", "-v")
		addEntity("remotes")
	}()

	// 4. Tags
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = ExecGit(repoPath, "tag", "-l")
		addEntity("tags")
	}()

	wg.Wait()

	return &RepoCachePrewarmResult{
		Success:           true,
		RepoPath:          repoPath,
		BranchCount:       branchCount,
		CurrentBranch:     currentBranch,
		IsClean:           isClean,
		DurationMs:        time.Since(start).Milliseconds(),
		PrewarmedEntities: entities,
	}, nil
}
