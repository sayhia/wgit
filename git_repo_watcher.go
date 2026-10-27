package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	repoWatcherMu     sync.Mutex
	activeRepoWatcher *fsnotify.Watcher
	activeWatcherStop chan struct{}
	activeWatchPath   string
)

// StartRepoWatcher starts a reactive file watcher on the current repository, emitting
// "repo:status-changed" Wails events when working tree or .git index/HEAD changes.
func (GitService) StartRepoWatcher(repoPath string) (bool, error) {
	cleanPath := filepath.Clean(repoPath)
	if cleanPath == "" {
		return false, fmt.Errorf("empty repo path")
	}

	repoWatcherMu.Lock()
	defer repoWatcherMu.Unlock()

	// If already watching this exact path, keep the watcher running
	if activeRepoWatcher != nil && activeWatchPath == cleanPath {
		return true, nil
	}

	// Stop previous watcher if running
	if activeRepoWatcher != nil {
		if activeWatcherStop != nil {
			close(activeWatcherStop)
			activeWatcherStop = nil
		}
		_ = activeRepoWatcher.Close()
		activeRepoWatcher = nil
		activeWatchPath = ""
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return false, fmt.Errorf("failed to create fs watcher: %w", err)
	}

	// Watch root repository directory
	if err := watcher.Add(cleanPath); err != nil {
		_ = watcher.Close()
		return false, fmt.Errorf("failed to watch repo root %s: %w", cleanPath, err)
	}

	// Watch .git directory if present
	gitDir := filepath.Join(cleanPath, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		_ = watcher.Add(gitDir)
		headFile := filepath.Join(gitDir, "HEAD")
		if _, err := os.Stat(headFile); err == nil {
			_ = watcher.Add(headFile)
		}
		refsDir := filepath.Join(gitDir, "refs")
		if _, err := os.Stat(refsDir); err == nil {
			_ = watcher.Add(refsDir)
		}
	}

	// Watch first-level non-ignored source subdirectories
	entries, _ := os.ReadDir(cleanPath)
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "dist" || name == "bin" {
				continue
			}
			subDir := filepath.Join(cleanPath, name)
			_ = watcher.Add(subDir)
		}
	}

	stopCh := make(chan struct{})
	activeRepoWatcher = watcher
	activeWatcherStop = stopCh
	activeWatchPath = cleanPath

	go func(targetPath string, w *fsnotify.Watcher, stop <-chan struct{}) {
		var debounceTimer *time.Timer
		var timerMu sync.Mutex

		for {
			select {
			case <-stop:
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				// Filter noisy or internal git cache files
				name := event.Name
				if strings.Contains(name, ".git/objects") ||
					strings.Contains(name, ".git/logs") ||
					strings.Contains(name, "node_modules") ||
					strings.HasSuffix(name, ".tmp") {
					continue
				}

				timerMu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
					if mainWindowRef != nil {
						mainWindowRef.EmitEvent("repo:status-changed", targetPath)
					}
				})
				timerMu.Unlock()

			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}(cleanPath, watcher, stopCh)

	return true, nil
}

// StopRepoWatcher stops any active background repository watcher.
func (GitService) StopRepoWatcher() bool {
	repoWatcherMu.Lock()
	defer repoWatcherMu.Unlock()

	if activeRepoWatcher == nil {
		return false
	}

	if activeWatcherStop != nil {
		close(activeWatcherStop)
		activeWatcherStop = nil
	}
	_ = activeRepoWatcher.Close()
	activeRepoWatcher = nil
	activeWatchPath = ""
	return true
}

// CreatePartialStash pushes a stash containing only the specified file paths.
func (GitService) CreatePartialStash(repoPath string, message string, filePaths []string) (string, error) {
	if len(filePaths) == 0 {
		return "", fmt.Errorf("no files selected for partial stash")
	}

	args := []string{"stash", "push"}
	if strings.TrimSpace(message) != "" {
		args = append(args, "-m", message)
	}
	args = append(args, "--")
	for _, fp := range filePaths {
		clean, err := repoRelativePathFromInput(repoPath, fp)
		if err == nil && clean != "" {
			args = append(args, clean)
		}
	}

	return ExecGit(repoPath, args...)
}
