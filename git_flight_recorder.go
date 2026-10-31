package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FlightSnapshotInfo struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	TriggerAction string `json:"triggerAction"` // "manual", "pre-reset", "pre-checkout", "pre-merge", "pre-rebase"
	FileCount     int    `json:"fileCount"`
	HeadHash      string `json:"headHash"`
	TreeHash      string `json:"treeHash"`
	CommitHash    string `json:"commitHash"`
	Message       string `json:"message"`
	SizeBytes     int64  `json:"sizeBytes"`
}

type SnapshotRestoreResult struct {
	Success       bool   `json:"success"`
	SnapshotID    string `json:"snapshotId"`
	RestoredFiles int    `json:"restoredFiles"`
	Message       string `json:"message"`
}

func execGitFlight(repoPath string, extraEnv []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"LC_ALL=en_US.UTF-8",
	)
	if len(extraEnv) > 0 {
		cmd.Env = append(cmd.Env, extraEnv...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w (%s)", args, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func getFlightStorePath(repoPath string) string {
	return filepath.Join(repoPath, ".git", "wgit", "flight_recorder.json")
}

func readFlightSnapshots(repoPath string) ([]FlightSnapshotInfo, error) {
	storePath := getFlightStorePath(repoPath)
	data, err := os.ReadFile(storePath)
	if os.IsNotExist(err) {
		return []FlightSnapshotInfo{}, nil
	}
	if err != nil {
		return nil, err
	}

	var list []FlightSnapshotInfo
	if err := json.Unmarshal(data, &list); err != nil {
		return []FlightSnapshotInfo{}, nil
	}
	return list, nil
}

func writeFlightSnapshots(repoPath string, list []FlightSnapshotInfo) error {
	storePath := getFlightStorePath(repoPath)
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(storePath, data, 0644)
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// CreateFlightRecorderSnapshot takes a zero-impact instant micro-snapshot of the entire working tree.
func (GitService) CreateFlightRecorderSnapshot(repoPath string, triggerAction string, message string) (*FlightSnapshotInfo, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	triggerAction = strings.TrimSpace(triggerAction)
	if triggerAction == "" {
		triggerAction = "manual"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = fmt.Sprintf("Snapshot [%s] at %s", triggerAction, time.Now().Format("15:04:05"))
	}

	// 1. Prepare temporary isolated index
	gitDir := filepath.Join(repoPath, ".git")
	tempIndex := filepath.Join(gitDir, fmt.Sprintf("flight_rec_%d.tmp.index", time.Now().UnixNano()))
	realIndex := filepath.Join(gitDir, "index")

	if _, err := os.Stat(realIndex); err == nil {
		// Seed the temp index from the real one: the `add -A` below then only
		// re-stages the working tree on top of it. A silently failed copy
		// starts from an empty index, so staged deletions and
		// tracked-but-ignored files would drop out of the snapshot and restore
		// would silently produce content that never matched the staged state —
		// fail the capture instead. (No index yet, e.g. unborn HEAD, is
		// skipped by the Stat guard and stays the legitimate empty-snapshot
		// path.)
		if err := copyFileContents(realIndex, tempIndex); err != nil {
			return nil, fmt.Errorf("failed to prepare flight recorder temp index: %w", err)
		}
	}
	defer os.Remove(tempIndex)

	env := []string{fmt.Sprintf("GIT_INDEX_FILE=%s", tempIndex)}

	// Stage all current working changes into temp index
	if _, err := execGitFlight(repoPath, env, "add", "-A"); err != nil {
		return nil, fmt.Errorf("failed to stage into flight recorder index: %w", err)
	}

	// Write tree from temp index
	treeHash, err := execGitFlight(repoPath, env, "write-tree")
	if err != nil {
		return nil, fmt.Errorf("failed to write-tree: %w", err)
	}

	// Resolve HEAD hash. On an unborn branch (fresh repo) rev-parse exits
	// non-zero yet still echoes the literal "HEAD" on stdout, which ExecGit
	// keeps next to the error — feeding that garbage into `commit-tree -p`
	// below would break the capture with an unrelated "not a valid object
	// name" error, so clear the hash on failure and let the designed
	// no-parent root-commit path run. A genuinely broken repo at worst loses
	// the lineage link (the tree itself stays pinned via the snapshot ref
	// below), while commit-tree failures still propagate.
	headHash, headErr := ExecGit(repoPath, "rev-parse", "HEAD")
	if headErr != nil {
		headHash = ""
	}

	// Create commit object
	var commitHash string
	if headHash != "" {
		commitHash, err = ExecGit(repoPath, "commit-tree", treeHash, "-p", headHash, "-m", message)
	} else {
		commitHash, err = ExecGit(repoPath, "commit-tree", treeHash, "-m", message)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create commit-tree for flight snapshot: %w", err)
	}

	snapshotID := fmt.Sprintf("snap_%d", time.Now().UnixNano())

	// Pin ref to prevent git gc from collecting it. A silently failed pin
	// leaves the snapshot unreferenced: git gc would prune the commit while
	// the manifest keeps advertising it, and restore would only fail weeks
	// later — surface the failure instead.
	refName := fmt.Sprintf("refs/wgit/snapshots/%s", snapshotID)
	if _, err := ExecGit(repoPath, "update-ref", refName, commitHash); err != nil {
		return nil, fmt.Errorf("failed to pin flight snapshot ref %s: %w", refName, err)
	}

	// Count files in tree. FileCount is display metadata for the snapshot list
	// and the restore banner; a failed count must not discard an otherwise
	// valid snapshot, so degrade to 0 instead of failing the capture.
	lsOut, _ := ExecGit(repoPath, "ls-tree", "-r", "--name-only", treeHash)
	fileCount := 0
	if strings.TrimSpace(lsOut) != "" {
		fileCount = len(strings.Split(strings.TrimSpace(lsOut), "\n"))
	}

	info := FlightSnapshotInfo{
		ID:            snapshotID,
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),
		TriggerAction: triggerAction,
		FileCount:     fileCount,
		HeadHash:      headHash,
		TreeHash:      treeHash,
		CommitHash:    commitHash,
		Message:       message,
	}

	// Save into manifest. A failed read must not fall through to an empty
	// list: the append below would then overwrite the manifest and silently
	// drop every earlier snapshot entry (their pinned refs would linger
	// unlisted).
	list, err := readFlightSnapshots(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flight recorder manifest: %w", err)
	}
	list = append([]FlightSnapshotInfo{info}, list...)
	// Same for the write: a snapshot that never lands in the manifest is
	// invisible to the UI and to PurgeFlightSnapshots — report it instead of
	// pretending the capture succeeded.
	if err := writeFlightSnapshots(repoPath, list); err != nil {
		return nil, fmt.Errorf("failed to save flight recorder manifest: %w", err)
	}

	return &info, nil
}

// ListFlightSnapshots returns historical flight recorder snapshots.
func (GitService) ListFlightSnapshots(repoPath string) ([]FlightSnapshotInfo, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	list, err := readFlightSnapshots(repoPath)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// RestoreFlightSnapshot restores the working tree to the state preserved in the snapshot.
func (GitService) RestoreFlightSnapshot(repoPath string, snapshotID string) (*SnapshotRestoreResult, error) {
	if repoPath == "" || snapshotID == "" {
		return nil, errors.New("repoPath and snapshotID are required")
	}

	list, err := readFlightSnapshots(repoPath)
	if err != nil {
		return nil, err
	}

	var target *FlightSnapshotInfo
	for _, item := range list {
		if item.ID == snapshotID {
			target = &item
			break
		}
	}

	if target == nil {
		return nil, fmt.Errorf("snapshot %s not found in flight recorder index", snapshotID)
	}

	// Restore files using git checkout from the snapshot ref
	refName := fmt.Sprintf("refs/wgit/snapshots/%s", snapshotID)
	_, err = ExecGit(repoPath, "checkout", refName, "--", ".")
	if err != nil {
		// Fallback to treeHash if ref lookup fails
		_, err = ExecGit(repoPath, "checkout", target.TreeHash, "--", ".")
		if err != nil {
			return &SnapshotRestoreResult{
				Success:    false,
				SnapshotID: snapshotID,
				Message:    fmt.Sprintf("Failed to checkout files from snapshot: %v", err),
			}, err
		}
	}

	return &SnapshotRestoreResult{
		Success:       true,
		SnapshotID:    snapshotID,
		RestoredFiles: target.FileCount,
		Message:       fmt.Sprintf("Successfully restored %d files from snapshot %s", target.FileCount, snapshotID),
	}, nil
}

// PurgeFlightSnapshots cleans up older snapshots beyond retention limit.
func (GitService) PurgeFlightSnapshots(repoPath string, keepCount int) (int, error) {
	if repoPath == "" {
		return 0, errors.New("repoPath cannot be empty")
	}
	if keepCount <= 0 {
		keepCount = 20
	}

	list, err := readFlightSnapshots(repoPath)
	if err != nil {
		return 0, err
	}

	if len(list) <= keepCount {
		return 0, nil
	}

	// Sort newest first just in case
	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp > list[j].Timestamp
	})

	toKeep := list[:keepCount]
	toDelete := list[keepCount:]

	for _, item := range toDelete {
		refName := fmt.Sprintf("refs/wgit/snapshots/%s", item.ID)
		// Best-effort cleanup: a failed delete leaves the old snapshot pinned,
		// which is gc-safe (just an invisible leftover ref), and the manifest
		// entry is removed below either way — retention still makes progress,
		// so do not fail the purge over a stale ref.
		_, _ = ExecGit(repoPath, "update-ref", "-d", refName)
	}

	if err := writeFlightSnapshots(repoPath, toKeep); err != nil {
		return 0, err
	}

	return len(toDelete), nil
}
