package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// validHookName rejects hook names that could escape the hooks directory or be
// interpreted as command-line options. Only plain names made of letters, digits,
// '-' and '_' are accepted, so path separators, "..", absolute paths, leading
// dashes, dots, whitespace and control characters are all refused.
func validHookName(name string) error {
	if name == "" {
		return fmt.Errorf("非法的钩子名称: %s", name)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("非法的钩子名称: %s", name)
	}
	if filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("非法的钩子名称: %s", name)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("非法的钩子名称: %s", name)
		}
	}
	return nil
}

// GitHookRunResult represents the execution result of a git hook.
type GitHookRunResult struct {
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"durationMs"`
	Success    bool   `json:"success"`
}

// GitHooksFrameworkInfo contains information about the detected git hooks management framework.
type GitHooksFrameworkInfo struct {
	Type       string `json:"type"`
	ConfigFile string `json:"configFile"`
	HooksPath  string `json:"hooksPath"`
}

// DetectHooksFramework checks whether a repository uses a hook framework like
// husky, lefthook, pre-commit, custom core.hooksPath, or native git hooks.
func (GitService) DetectHooksFramework(repoPath string) (*GitHooksFrameworkInfo, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}

	hooksPath := gitPath(repoPath, "hooks")

	// 1. Check for Lefthook configuration
	lefthookConfigs := []string{"lefthook.yml", "lefthook.yaml", ".lefthook.yml", ".lefthook.yaml"}
	for _, conf := range lefthookConfigs {
		confPath := filepath.Join(repoPath, conf)
		if fi, err := os.Stat(confPath); err == nil && !fi.IsDir() {
			return &GitHooksFrameworkInfo{
				Type:       "lefthook",
				ConfigFile: confPath,
				HooksPath:  hooksPath,
			}, nil
		}
	}

	// 2. Check for pre-commit configuration
	preCommitConfig := filepath.Join(repoPath, ".pre-commit-config.yaml")
	if fi, err := os.Stat(preCommitConfig); err == nil && !fi.IsDir() {
		return &GitHooksFrameworkInfo{
			Type:       "pre-commit",
			ConfigFile: preCommitConfig,
			HooksPath:  hooksPath,
		}, nil
	}

	// 3. Check for Husky directory (.husky)
	huskyDir := filepath.Join(repoPath, ".husky")
	if fi, err := os.Stat(huskyDir); err == nil && fi.IsDir() {
		return &GitHooksFrameworkInfo{
			Type:       "husky",
			ConfigFile: huskyDir,
			HooksPath:  hooksPath,
		}, nil
	}

	// 4. Check custom core.hooksPath from git config
	if customHooksPath, err := ExecGit(repoPath, "config", "--get", "core.hooksPath"); err == nil {
		custom := strings.TrimSpace(customHooksPath)
		if custom != "" {
			if strings.Contains(custom, ".husky") {
				return &GitHooksFrameworkInfo{
					Type:       "husky",
					ConfigFile: filepath.Join(repoPath, custom),
					HooksPath:  hooksPath,
				}, nil
			}
			return &GitHooksFrameworkInfo{
				Type:       "custom",
				ConfigFile: custom,
				HooksPath:  hooksPath,
			}, nil
		}
	}

	// 5. Default to native git hooks
	return &GitHooksFrameworkInfo{
		Type:       "native",
		ConfigFile: "",
		HooksPath:  hooksPath,
	}, nil
}

// RunGitHook executes the specified hook, capturing its execution duration,
// stdout, stderr, and exit code. It first attempts to execute via `git hook run <hookName> [-- args...]`,
// falling back to directly executing the hook script if needed.
func (GitService) RunGitHook(repoPath string, hookName string, args []string) (*GitHookRunResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	hookName = strings.TrimSpace(hookName)
	if err := validHookName(hookName); err != nil {
		return nil, err
	}

	hooksPath := gitPath(repoPath, "hooks")
	hookScriptPath := filepath.Join(hooksPath, hookName)

	startTime := time.Now()

	// 1. Attempt via `git hook run <hookName> [-- args...]`
	gitArgs := []string{"hook", "run", "--allow-unknown-hook-name", hookName}
	if len(args) > 0 {
		gitArgs = append(gitArgs, "--")
		gitArgs = append(gitArgs, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"LC_ALL=en_US.UTF-8",
	)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	duration := time.Since(startTime)

	if err == nil {
		outStr := stdoutBuf.String()
		errStr := stderrBuf.String()
		// git hook run merges hook stdout into stderr; if stdout is empty, mirror it
		if outStr == "" && errStr != "" {
			outStr = errStr
		}
		return &GitHookRunResult{
			ExitCode:   0,
			Stdout:     outStr,
			Stderr:     errStr,
			DurationMs: duration.Milliseconds(),
			Success:    true,
		}, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode := exitErr.ExitCode()
		errStr := stderrBuf.String()
		gitFailed := strings.Contains(errStr, "cannot find a hook named") ||
			strings.Contains(errStr, "is not a git command") ||
			strings.Contains(errStr, "unknown command") ||
			strings.Contains(errStr, "unknown option")

		if !gitFailed {
			// Hook was executed and failed with exitCode
			outStr := stdoutBuf.String()
			if outStr == "" && errStr != "" {
				outStr = errStr
			}
			return &GitHookRunResult{
				ExitCode:   exitCode,
				Stdout:     outStr,
				Stderr:     errStr,
				DurationMs: duration.Milliseconds(),
				Success:    false,
			}, nil
		}
	}

	// 2. Fallback to direct execution of the hook script
	if fi, statErr := os.Stat(hookScriptPath); statErr == nil && !fi.IsDir() {
		startTime = time.Now()
		directCtx, directCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer directCancel()

		var directCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			directCmd = exec.CommandContext(directCtx, "sh", append([]string{hookScriptPath}, args...)...)
		} else {
			if fi.Mode()&0111 == 0 {
				_ = os.Chmod(hookScriptPath, 0755)
			}
			directCmd = exec.CommandContext(directCtx, hookScriptPath, args...)
		}
		directCmd.Dir = repoPath
		directCmd.Env = append(os.Environ(),
			"GIT_DIR="+gitStateDir(repoPath),
			"GIT_WORK_TREE="+repoPath,
			"GIT_TERMINAL_PROMPT=0",
			"GIT_PAGER=cat",
			"LC_ALL=en_US.UTF-8",
		)

		var dStdout, dStderr bytes.Buffer
		directCmd.Stdout = &dStdout
		directCmd.Stderr = &dStderr

		dErr := directCmd.Run()
		dDuration := time.Since(startTime)

		dExitCode := 0
		if dErr != nil {
			var dExitErr *exec.ExitError
			if errors.As(dErr, &dExitErr) {
				dExitCode = dExitErr.ExitCode()
			} else {
				return nil, dErr
			}
		}

		return &GitHookRunResult{
			ExitCode:   dExitCode,
			Stdout:     dStdout.String(),
			Stderr:     dStderr.String(),
			DurationMs: dDuration.Milliseconds(),
			Success:    dExitCode == 0,
		}, nil
	}

	return nil, fmt.Errorf("hook %q not found or failed to execute: %v", hookName, err)
}

// InstallSampleHook installs a hook script from its .sample file if available,
// or creates a standard shell script template with 0755 permissions.
func (GitService) InstallSampleHook(repoPath string, hookName string) error {
	if strings.TrimSpace(repoPath) == "" {
		return errors.New("repository path is required")
	}
	hookName = strings.TrimSpace(hookName)
	if err := validHookName(hookName); err != nil {
		return err
	}

	hooksDir := gitPath(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	targetPath := filepath.Join(hooksDir, hookName)
	samplePath := filepath.Join(hooksDir, hookName+".sample")

	// Check sample in hooksDir, or fallback to common hooks dir
	if _, err := os.Stat(samplePath); err != nil {
		commonSample := filepath.Join(gitCommonDir(repoPath), "hooks", hookName+".sample")
		if _, errCommon := os.Stat(commonSample); errCommon == nil {
			samplePath = commonSample
		}
	}

	var scriptContent []byte
	if sampleData, err := os.ReadFile(samplePath); err == nil {
		scriptContent = sampleData
	} else {
		scriptContent = []byte("#!/bin/sh\nexit 0\n")
	}

	if err := os.WriteFile(targetPath, scriptContent, 0755); err != nil {
		return fmt.Errorf("failed to write hook script: %w", err)
	}
	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to make hook executable: %w", err)
	}

	// Remove disabled marker if one was present
	_ = os.Remove(targetPath + ".disabled")

	return nil
}

// CommitAdvanced creates a new commit with optional --amend and --no-verify flags.
func (GitService) CommitAdvanced(repoPath string, message string, amend bool, noVerify bool) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("commit message cannot be empty")
	}
	args := []string{"commit", "-m", message}
	if amend {
		args = append(args, "--amend")
	}
	if noVerify {
		args = append(args, "--no-verify")
	}
	_, err := ExecGit(repoPath, args...)
	return err
}
