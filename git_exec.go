package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// GitCommandLog tracks git command executions for the activity drawer.
type GitCommandLog struct {
	ID         int64  `json:"id"`
	Source     string `json:"source,omitempty"` // git | ai
	Kind       string `json:"kind,omitempty"`   // AI task kind, e.g. commit/summary/conflict
	Mode       string `json:"mode,omitempty"`   // AI transport: api | cli
	Model      string `json:"model,omitempty"`  // AI model identifier
	Command    string `json:"command"`
	Dir        string `json:"dir"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
	TokensIn   int    `json:"tokensIn,omitempty"`
	TokensOut  int    `json:"tokensOut,omitempty"`
	// UsageKnown is false when the transport (local CLI, some providers) does
	// not report token usage, so the UI can distinguish "0" from "unknown".
	UsageKnown bool   `json:"usageKnown"`
	Time       string `json:"time"`
	Success    bool   `json:"success"`
	Cancelled  bool   `json:"cancelled,omitempty"`
}

var (
	activityLogsLock sync.Mutex
	activityLogs     []GitCommandLog
	logSeq           int64
)

// Patterns for standalone secrets that are not embedded in a URL but can still
// be echoed by git (hooks, credential helpers, remote errors). They are
// compiled once so recordGitCommand stays cheap.
var (
	gitHubTokenPattern    = regexp.MustCompile(`\b(gh[pousr]_)[A-Za-z0-9]{20,}`)
	gitLabTokenPattern    = regexp.MustCompile(`\b(glpat-)[A-Za-z0-9_-]{20,}`)
	awsAccessKeyPattern   = regexp.MustCompile(`\b(AKIA)[A-Z0-9]{16}\b`)
	keyValueSecretPattern = regexp.MustCompile(`(?i)\b(token|password|secret|api_key|apikey)=([^\s"'&]+)`)
)

// redactLogSecrets scrubs credentials from text that is about to be stored in
// the in-memory activity log. It reuses maskCredentialURL for credentials
// embedded in URLs, then masks bare provider tokens and key=value secrets that
// git may echo without a URL wrapper.
func redactLogSecrets(s string) string {
	if s == "" {
		return s
	}
	s = maskCredentialURL(s)
	s = gitHubTokenPattern.ReplaceAllString(s, "${1}***")
	s = gitLabTokenPattern.ReplaceAllString(s, "${1}***")
	s = awsAccessKeyPattern.ReplaceAllString(s, "${1}***")
	s = keyValueSecretPattern.ReplaceAllString(s, "${1}=***")
	return s
}

func recordGitCommand(dir string, args []string, outStr string, errStr string, duration time.Duration, success bool) {
	activityLogsLock.Lock()
	defer activityLogsLock.Unlock()

	logSeq++
	// Redact credentials before the entry is stored: userinfo and query-string
	// secrets in URLs plus bare provider tokens or key=value secrets echoed by
	// git must never reach the activity drawer.
	command := redactLogSecrets("git " + strings.Join(args, " "))
	truncatedOut := redactLogSecrets(outStr)
	if len(truncatedOut) > 4000 {
		truncatedOut = truncatedOut[:4000] + "... (truncated)"
	}

	entry := GitCommandLog{
		ID:         logSeq,
		Source:     "git",
		Command:    command,
		Dir:        dir,
		Output:     truncatedOut,
		Error:      redactLogSecrets(errStr),
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

// recordStateChange logs a non-git state mutation (AI settings, feature
// flags, asset states) into the same in-memory activity feed, so state the
// user may want to revisit is auditable from the activity drawer. It shares
// the lock, sequence and cap with recordGitCommand, and applies the same
// redaction to both op and detail.
func recordStateChange(op string, detail string, success bool) {
	activityLogsLock.Lock()
	defer activityLogsLock.Unlock()

	logSeq++
	entry := GitCommandLog{
		ID:      logSeq,
		Source:  "app",
		Command: redactLogSecrets(op),
		Output:  redactLogSecrets(detail),
		Time:    time.Now().Format("15:04:05"),
		Success: success,
	}

	activityLogs = append(activityLogs, entry)
	if len(activityLogs) > 200 {
		activityLogs = activityLogs[len(activityLogs)-200:]
	}
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

// DefaultGitBatchSize is the safe threshold of arguments passed to a single git invocation
// to prevent exceeding operating system ARG_MAX limits.
const DefaultGitBatchSize = 500

// ChunkSlice splits a slice into smaller chunks of at most chunkSize elements.
func ChunkSlice[T any](items []T, chunkSize int) [][]T {
	if chunkSize <= 0 {
		chunkSize = DefaultGitBatchSize
	}
	if len(items) == 0 {
		return nil
	}
	var chunks [][]T
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// ExecGitBatched executes a git command across batches of item arguments (such as paths or hashes)
// to guarantee safety against OS argument list limits (ARG_MAX).
func ExecGitBatched(repoPath string, baseArgs []string, batchItems []string, batchSize int) ([]string, error) {
	if len(batchItems) == 0 {
		return nil, nil
	}
	chunks := ChunkSlice(batchItems, batchSize)
	results := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		args := append(append([]string{}, baseArgs...), chunk...)
		out, err := ExecGit(repoPath, args...)
		if err != nil {
			return results, err
		}
		results = append(results, out)
	}
	return results, nil
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
		// The error surfaces in the UI/RPC response, so scrub credentials the
		// same way the activity log does before echoing stderr back.
		redactedErrStr := redactLogSecrets(errStr)
		if ctx.Err() == context.DeadlineExceeded {
			if redactedErrStr != "" {
				return outStr, fmt.Errorf("%s (git command timed out after %s)", redactedErrStr, timeout)
			}
			return outStr, fmt.Errorf("git command timed out after %s", timeout)
		}
		if redactedErrStr != "" {
			return outStr, fmt.Errorf("%s (%w)", redactedErrStr, err)
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
