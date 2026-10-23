package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GitCommandLog tracks git command executions for the activity drawer.
type GitCommandLog struct {
	ID         int64  `json:"id"`
	Command    string `json:"command"`
	Dir        string `json:"dir"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
	Time       string `json:"time"`
	Success    bool   `json:"success"`
}

var (
	activityLogsLock sync.Mutex
	activityLogs     []GitCommandLog
	logSeq           int64
)

func recordGitCommand(dir string, args []string, outStr string, errStr string, duration time.Duration, success bool) {
	activityLogsLock.Lock()
	defer activityLogsLock.Unlock()

	logSeq++
	truncatedOut := outStr
	if len(truncatedOut) > 4000 {
		truncatedOut = truncatedOut[:4000] + "... (truncated)"
	}

	entry := GitCommandLog{
		ID:         logSeq,
		Command:    "git " + strings.Join(args, " "),
		Dir:        dir,
		Output:     truncatedOut,
		Error:      errStr,
		DurationMs: duration.Milliseconds(),
		Time:       time.Now().Format("15:04:05"),
		Success:    success,
	}

	activityLogs = append(activityLogs, entry)
	if len(activityLogs) > 200 {
		activityLogs = activityLogs[len(activityLogs)-200:]
	}
}

// GetRecentActivityLogs returns in-memory recorded git command execution logs.
func GetRecentActivityLogs() []GitCommandLog {
	activityLogsLock.Lock()
	defer activityLogsLock.Unlock()
	res := make([]GitCommandLog, len(activityLogs))
	copy(res, activityLogs)
	return res
}

// ClearRecentActivityLogs clears the recorded logs.
func ClearRecentActivityLogs() {
	activityLogsLock.Lock()
	defer activityLogsLock.Unlock()
	activityLogs = nil
}

// ExecGit runs a git command in the specified directory.
func ExecGit(repoPath string, args ...string) (string, error) {
	return execGitTimeout(repoPath, "", true, 60*time.Second, args...)
}

// ExecGitWithTimeout runs a git command with an explicit overall deadline.
func ExecGitWithTimeout(repoPath string, timeout time.Duration, args ...string) (string, error) {
	return execGitTimeout(repoPath, "", true, timeout, args...)
}

// ExecGitRaw runs a git command and keeps the stdout untouched.
// Porcelain formats such as `git status --porcelain` start with a significant
// leading space, which TrimSpace would destroy (misreading the first entry).
func ExecGitRaw(repoPath string, args ...string) (string, error) {
	return execGitTimeout(repoPath, "", false, 60*time.Second, args...)
}

// ExecGitWithStdin runs a git command in the specified directory with optional stdin data.
func ExecGitWithStdin(repoPath string, stdinData string, args ...string) (string, error) {
	return execGitTimeout(repoPath, stdinData, true, 60*time.Second, args...)
}

func execGitTimeout(repoPath string, stdinData string, trimOutput bool, timeout time.Duration, args ...string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath

	// Ensure git uses standard UTF-8 and no paging
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"LC_ALL=en_US.UTF-8",
	)

	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)
	outStr := stdout.String()
	if trimOutput {
		outStr = strings.TrimSpace(outStr)
	}
	errStr := strings.TrimSpace(stderr.String())

	success := err == nil
	recordGitCommand(repoPath, args, strings.TrimSpace(outStr), errStr, duration, success)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			if errStr != "" {
				return outStr, fmt.Errorf("%s (git command timed out after %s)", errStr, timeout)
			}
			return outStr, fmt.Errorf("git command timed out after %s", timeout)
		}
		if errStr != "" {
			return outStr, fmt.Errorf("%s (%w)", errStr, err)
		}
		return outStr, err
	}

	return outStr, nil
}

// IsGitRepo checks if a folder is a valid git repository.
func IsGitRepo(path string) bool {
	if path == "" {
		return false
	}
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err == nil {
		return info.IsDir() || !info.IsDir() // submodule or worktree may have .git file
	}
	// Try rev-parse as fallback
	_, err = ExecGit(path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// gitStateDir returns the per-worktree git directory that holds MERGE_HEAD,
// rebase state, and other in-progress operation markers. A linked worktree's
// own .git is a file pointing at this directory.
func gitStateDir(repoPath string) string {
	return resolveGitDir(repoPath, "--git-dir")
}

// gitCommonDir returns the shared git directory. Hooks, config, LFS objects,
// and repository-level state live here even when running from a linked worktree.
func gitCommonDir(repoPath string) string {
	return resolveGitDir(repoPath, "--git-common-dir")
}

func resolveGitDir(repoPath string, flag string) string {
	out, err := ExecGit(repoPath, "rev-parse", flag)
	if err == nil {
		p := strings.TrimSpace(out)
		if p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(repoPath, p)
			}
			return filepath.Clean(p)
		}
	}
	return filepath.Join(repoPath, ".git")
}

// gitPath resolves a git-managed path such as hooks, honoring core.hooksPath
// and worktree-specific locations.
func gitPath(repoPath string, name string) string {
	if out, err := ExecGit(repoPath, "rev-parse", "--git-path", name); err == nil {
		p := strings.TrimSpace(out)
		if p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(repoPath, p)
			}
			return filepath.Clean(p)
		}
	}
	return filepath.Join(gitCommonDir(repoPath), name)
}
