package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FastCommitResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Pushed  bool   `json:"pushed"`
}

type GitService struct{}

// OpenRepo loads repository details and saves to recent list
func (GitService) OpenRepo(path string) (*RepoInfo, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}
	path = filepath.Clean(path)
	if !IsGitRepo(path) {
		return nil, fmt.Errorf("'%s' is not a valid git repository", path)
	}

	// If a subdirectory or .git folder inside a worktree was selected, resolve to the repository root
	if topLevel, err := ExecGit(path, "rev-parse", "--show-toplevel"); err == nil && strings.TrimSpace(topLevel) != "" {
		path = filepath.Clean(strings.TrimSpace(topLevel))
	}

	AddRecentRepo(path)
	return getRepoInfo(path)
}

// OpenRepoDialog pops up the native macOS/system folder picker
func (s GitService) OpenRepoDialog() (*RepoInfo, error) {
	path, err := ChooseFolderDialog("选择 Git 仓库目录")
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	return s.OpenRepo(path)
}

// InitRepo creates a new git repository in the given directory
func (s GitService) InitRepo(path string) (*RepoInfo, error) {
	if path == "" {
		return nil, errors.New("directory path cannot be empty")
	}
	path = filepath.Clean(path)
	_, err := ExecGit(path, "init")
	if err != nil {
		return nil, fmt.Errorf("failed to git init: %w", err)
	}
	return s.OpenRepo(path)
}

// GetRecentRepos returns list of recently opened repos
func (GitService) GetRecentRepos() []string {
	cfg := LoadConfig()
	var valid []string
	for _, p := range cfg.RecentRepos {
		if IsGitRepo(p) {
			valid = append(valid, p)
		}
	}
	return valid
}

// GetRepoStatus fetches uncommitted changes and branch tracking status
func (GitService) GetRepoStatus(repoPath string) (*RepoStatus, error) {
	raw, err := ExecGit(repoPath, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, err
	}
	st := ParseRepoStatus(repoPath, raw)
	headHash, errHead := ExecGit(repoPath, "rev-parse", "-q", "--verify", "HEAD")
	if errHead == nil && strings.TrimSpace(headHash) != "" {
		st.HeadCommit = strings.TrimSpace(headHash)
		st.IsInitialCommit = false
	} else {
		st.HeadCommit = ""
		st.IsInitialCommit = true
		st.TotalCommits = 0
	}
	return &st, nil
}

// GetCommits fetches commit history with DAG graph lane topology
func (GitService) GetCommits(repoPath string, limit int, branch string) ([]CommitItem, error) {
	if limit <= 0 {
		limit = 100
	}
	// %x1e = record separator, %x1f = unit separator
	// %H = hash, %h = short hash, %P = parent hashes, %an = author name, %ae = author email,
	// %ci = commit date ISO, %cr = relative date, %s = subject, %b = body, %D = ref names
	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	args := []string{"log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("-n%d", limit), "--topo-order"}

	if branch != "" && branch != "HEAD" {
		args = append(args, branch)
	} else {
		args = append(args, "--all")
	}

	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		// Empty repo with no commits
		if strings.Contains(err.Error(), "does not have any commits yet") || strings.Contains(raw, "does not have any commits yet") {
			return []CommitItem{}, nil
		}
		return nil, err
	}

	return ParseRawGitLog(raw), nil
}

// GetCommitDetail returns commit metadata, file list, stats, and diff
func (GitService) GetCommitDetail(repoPath string, commitHash string) (*CommitDetail, error) {
	if commitHash == "" {
		return nil, errors.New("commit hash is required")
	}

	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	rawMeta, err := ExecGit(repoPath, "show", fmt.Sprintf("--format=%s", format), "-s", commitHash)
	if err != nil {
		return nil, err
	}

	commits := ParseRawGitLog(rawMeta)
	var commitItem CommitItem
	if len(commits) > 0 {
		commitItem = commits[0]
	}

	// Fetch file stat
	rawStat, err := ExecGit(repoPath, "show", "--numstat", "--pretty=format:", commitHash)
	if err != nil {
		return nil, fmt.Errorf("读取提交文件统计失败: %v", err)
	}
	var files []FileStatus
	totalAdd := 0
	totalDel := 0

	lfsExtensions := getLFSExtensions(repoPath)

	for _, line := range strings.Split(rawStat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pts := strings.Split(line, "\t")
		if len(pts) >= 3 {
			add, _ := strconv.Atoi(pts[0])
			del, _ := strconv.Atoi(pts[1])
			filePath := pts[2]
			totalAdd += add
			totalDel += del

			files = append(files, FileStatus{
				Path:      filePath,
				Additions: add,
				Deletions: del,
				IsLFS:     isPathLFS(filePath, lfsExtensions),
			})
		}
	}

	// Fetch unified diff (large-repo guard: payloads are capped for webview safety)
	rawDiff, err := ExecGit(repoPath, "show", "--patch", "--pretty=format:", commitHash)
	if err != nil {
		return nil, fmt.Errorf("读取提交差异失败: %v", err)
	}
	diffText, diffTruncated := truncateDiffPayload(rawDiff)

	return &CommitDetail{
		Commit:        commitItem,
		Files:         files,
		Additions:     totalAdd,
		Deletions:     totalDel,
		Diff:          diffText,
		DiffTruncated: diffTruncated,
	}, nil
}

// GetWorkingDiffAdvanced returns diff of a file in the working tree with advanced options
func (GitService) GetWorkingDiffAdvanced(repoPath string, filePath string, staged bool, ignoreWhitespace bool) (string, error) {
	var out string
	var err error
	var args []string
	if staged {
		args = []string{"diff", "--cached"}
	} else {
		args = []string{"diff"}
	}
	if ignoreWhitespace {
		args = append(args, "-w")
	}
	args = append(args, "--", filePath)

	out, err = ExecGit(repoPath, args...)
	if !staged {
		// If untracked, git diff prints nothing; show its content as an addition.
		if err == nil && strings.TrimSpace(out) == "" {
			statusOut, stErr := ExecGit(repoPath, "status", "--porcelain", "--", filePath)
			if stErr == nil && strings.HasPrefix(strings.TrimSpace(statusOut), "??") {
				noIndexArgs := []string{"diff"}
				if ignoreWhitespace {
					noIndexArgs = append(noIndexArgs, "-w")
				}
				noIndexArgs = append(noIndexArgs, "--no-index", "/dev/null", filePath)
				out, _ = ExecGit(repoPath, noIndexArgs...)
				capped, _ := truncateDiffPayload(out)
				return capped, nil
			}
		}
	}
	if err != nil {
		return out, err
	}
	capped, _ := truncateDiffPayload(out)
	return capped, nil
}

// GetWorkingDiff returns diff of a file in the working tree
func (s GitService) GetWorkingDiff(repoPath string, filePath string, staged bool) (string, error) {
	return s.GetWorkingDiffAdvanced(repoPath, filePath, staged, false)
}

// StageFile stages a single file
func (GitService) StageFile(repoPath string, filePath string) error {
	_, err := ExecGit(repoPath, "add", "--", filePath)
	return err
}

// UnstageFile unstages a single file
func (GitService) UnstageFile(repoPath string, filePath string) error {
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); err != nil {
		// Before the first commit there is no HEAD for `restore --staged`.
		_, rmErr := ExecGit(repoPath, "rm", "--cached", "--", filePath)
		return rmErr
	}
	_, err := ExecGit(repoPath, "restore", "--staged", "--", filePath)
	return err
}

// StageAll stages all changes
func (GitService) StageAll(repoPath string) error {
	_, err := ExecGit(repoPath, "add", "-A")
	return err
}

// UnstageAll unstages all changes
func (GitService) UnstageAll(repoPath string) error {
	_, err := ExecGit(repoPath, "reset", "HEAD")
	return err
}

// DiscardFile discards changes to a working tree file
func (GitService) DiscardFile(repoPath string, filePath string) error {
	// If untracked, remove file
	statusOut, _ := ExecGit(repoPath, "status", "--porcelain", "--", filePath)
	if strings.HasPrefix(strings.TrimSpace(statusOut), "??") {
		_, err := ExecGit(repoPath, "clean", "-fd", "--", filePath)
		return err
	}
	_, err := ExecGit(repoPath, "restore", "--", filePath)
	return err
}

// Commit creates a new commit
func (GitService) Commit(repoPath string, message string, amend bool) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("commit message cannot be empty")
	}
	args := []string{"commit", "-m", message}
	if amend {
		args = append(args, "--amend")
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// FastCommit: Tencent UGit signature feature!
// Stages all changes, checks if remote conflicts with current files, commits and rebases/pushes cleanly!
func (s GitService) FastCommit(repoPath string, message string) (*FastCommitResult, error) {
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("commit message cannot be empty")
	}

	// 1. Stage all changes
	if err := s.StageAll(repoPath); err != nil {
		return nil, fmt.Errorf("failed to stage changes: %w", err)
	}

	// 2. Commit locally
	if err := s.Commit(repoPath, message, false); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	// 3. Fetch remote
	_, _ = ExecGit(repoPath, "fetch", defaultPushRemote(repoPath))

	// 4. Try rebase pull to avoid merge bubble
	_, err := ExecGit(repoPath, "pull", "--rebase")
	if err != nil {
		// No upstream, missing remote, or a rebase conflict all surface here.
		return &FastCommitResult{
			Success: true,
			Message: fmt.Sprintf("Committed locally, but pull --rebase did not complete: %v. Resolve conflicts or configure an upstream, then push.", err),
			Pushed:  false,
		}, nil
	}

	// 5. Push to current branch
	_, pushErr := ExecGit(repoPath, "push")
	if pushErr != nil {
		return &FastCommitResult{
			Success: true,
			Message: fmt.Sprintf("Committed and rebased, but push failed: %v", pushErr),
			Pushed:  false,
		}, nil
	}

	return &FastCommitResult{
		Success: true,
		Message: "快速提交完成：已提交、变基拉取并推送到远端。",
		Pushed:  true,
	}, nil
}

// GetBranches returns list of local and remote branches
func (GitService) GetBranches(repoPath string) ([]BranchInfo, error) {
	raw, err := ExecGit(repoPath, "branch", "-a", "-vv")
	if err != nil {
		return nil, err
	}

	var branches []BranchInfo
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Detached HEAD is not a branch; git branch prints a pseudo entry such as
		// "* (HEAD detached at v1.0.0)" that must not surface as a local branch.
		if strings.HasPrefix(line, "* (HEAD") || strings.HasPrefix(line, "* (no branch") ||
			strings.HasPrefix(line, "+ (HEAD") || strings.HasPrefix(line, "+ (no branch") {
			continue
		}

		isCurrent := false
		if strings.HasPrefix(line, "* ") {
			isCurrent = true
			line = strings.TrimPrefix(line, "* ")
		} else if strings.HasPrefix(line, "+ ") {
			// "+" marks a branch checked out in another linked worktree.
			line = strings.TrimPrefix(line, "+ ")
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		branchName := parts[0]
		isRemote := strings.HasPrefix(branchName, "remotes/")
		if isRemote {
			branchName = strings.TrimPrefix(branchName, "remotes/")
		}

		// skip HEAD pointer like remotes/origin/HEAD -> origin/main
		if strings.Contains(branchName, "/HEAD") {
			continue
		}

		commitHash := ""
		if len(parts) > 1 {
			commitHash = parts[1]
		}

		upstream := ""
		ahead := 0
		behind := 0

		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			bStart := strings.Index(line, "[")
			bEnd := strings.Index(line, "]")
			bracketContent := line[bStart+1 : bEnd]
			bracketParts := strings.Split(bracketContent, ": ")
			upstream = bracketParts[0]

			if len(bracketParts) > 1 {
				statParts := strings.Split(bracketParts[1], ", ")
				for _, sp := range statParts {
					if strings.HasPrefix(sp, "ahead ") {
						ahead, _ = strconv.Atoi(strings.TrimPrefix(sp, "ahead "))
					} else if strings.HasPrefix(sp, "behind ") {
						behind, _ = strconv.Atoi(strings.TrimPrefix(sp, "behind "))
					}
				}
			}
		}

		branches = append(branches, BranchInfo{
			Name:       branchName,
			IsCurrent:  isCurrent,
			IsRemote:   isRemote,
			CommitHash: commitHash,
			Upstream:   upstream,
			Ahead:      ahead,
			Behind:     behind,
		})
	}

	return branches, nil
}

// CheckoutBranch switches to a branch or creates and switches
func (GitService) CheckoutBranch(repoPath string, branchName string, createNew bool) error {
	if branchName == "" {
		return errors.New("branch name cannot be empty")
	}
	if createNew {
		_, err := ExecGit(repoPath, "checkout", "-b", branchName)
		return err
	}

	// A local branch with the same name takes precedence; plain checkout of a
	// remote-tracking ref (e.g. upstream/topic) would detach HEAD.
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branchName); err == nil {
		_, err = ExecGit(repoPath, "checkout", branchName)
		return err
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", "refs/remotes/"+branchName); err == nil {
		localName := branchName
		remotes, remotesErr := ExecGit(repoPath, "remote")
		if remotesErr == nil {
			best := ""
			for _, remote := range strings.Split(remotes, "\n") {
				remote = strings.TrimSpace(remote)
				if strings.HasPrefix(branchName, remote+"/") && len(remote) > len(best) {
					best = remote
				}
			}
			if best != "" {
				localName = strings.TrimPrefix(branchName, best+"/")
			} else if idx := strings.Index(branchName, "/"); idx >= 0 {
				localName = branchName[idx+1:]
			}
		} else if idx := strings.Index(branchName, "/"); idx >= 0 {
			localName = branchName[idx+1:]
		}
		// Reuse an existing local branch (preserving local commits); otherwise
		// create it with upstream tracking.
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+localName); err == nil {
			_, err = ExecGit(repoPath, "checkout", localName)
			return err
		}
		_, err := ExecGit(repoPath, "checkout", "--track", branchName)
		return err
	}

	_, err := ExecGit(repoPath, "checkout", branchName)
	return err
}

// CreateBranch creates a new branch without checking out
func (GitService) CreateBranch(repoPath string, branchName string, startPoint string) error {
	args := []string{"branch", branchName}
	if startPoint != "" {
		args = append(args, startPoint)
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// DeleteBranch deletes a branch
func (GitService) DeleteBranch(repoPath string, branchName string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := ExecGit(repoPath, "branch", flag, branchName)
	return err
}

// DeleteRemoteBranch deletes a remote branch
func (GitService) DeleteRemoteBranch(repoPath string, remote string, branchName string) error {
	if remote == "" {
		remote = "origin"
	}
	cleanBranch := strings.TrimSpace(branchName)
	cleanBranch = strings.TrimPrefix(cleanBranch, "remotes/")
	cleanBranch = strings.TrimPrefix(cleanBranch, remote+"/")
	_, err := ExecGit(repoPath, "push", remote, "--delete", cleanBranch)
	return err
}

// GetTags returns all tags
func (GitService) GetTags(repoPath string) ([]TagInfo, error) {
	// ExecGitRaw, not ExecGit: a tag with an empty message ends its line with a
	// trailing tab, and trimming the whole stdout would drop that separator and
	// make the last tag look malformed.
	//
	// Two things this format gets right that the old one did not:
	//   - %(objecttype) distinguishes annotated tags ("tag") from lightweight
	//     ones ("commit"). Without it they are indistinguishable, because
	//     %(contents:subject) falls back to the commit subject for lightweight
	//     tags, so every tag looks annotated.
	//   - %(*objectname:short) is the dereferenced commit. For an annotated tag,
	//     plain %(objectname:short) is the *tag object*, whose OID matches
	//     nothing in git log and cannot be used to reset or compare.
	raw, err := ExecGitRaw(repoPath, "tag", "-l",
		"--format=%(refname:short)%09%(objecttype)%09%(*objectname:short)%09%(objectname:short)%09%(contents:subject)")
	if err != nil {
		return nil, err
	}

	var tags []TagInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		pts := strings.Split(line, "\t")
		name := refField(pts, 0)
		if name == "" {
			continue
		}
		// Dereferenced commit for annotated tags, falling back to the ref OID
		// for lightweight ones (where the deref field is empty).
		hash := refField(pts, 2)
		if hash == "" {
			hash = refField(pts, 3)
		}
		tags = append(tags, TagInfo{
			Name:        name,
			IsAnnotated: refField(pts, 1) == "tag",
			CommitHash:  hash,
			Message:     refField(pts, 4),
		})
	}
	return tags, nil
}

func validateTagName(repoPath string, tagName string) (string, error) {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return "", errors.New("tagName 不能为空")
	}
	if _, err := ExecGit(repoPath, "check-ref-format", "refs/tags/"+tagName); err != nil {
		return "", fmt.Errorf("非法的标签名 %q", tagName)
	}
	return tagName, nil
}

// CreateTag creates a new tag
func (GitService) CreateTag(repoPath string, tagName string, commitHash string, message string) error {
	tagName, err := validateTagName(repoPath, tagName)
	if err != nil {
		return err
	}
	args := []string{"tag"}
	if message != "" {
		args = append(args, "-a", "-m", message)
	}
	args = append(args, "--", tagName)
	if commitHash != "" {
		args = append(args, commitHash)
	}
	_, err = ExecGit(repoPath, args...)
	return err
}

// DeleteTag deletes a tag
func (GitService) DeleteTag(repoPath string, tagName string) error {
	tagName, err := validateTagName(repoPath, tagName)
	if err != nil {
		return err
	}
	_, err = ExecGit(repoPath, "tag", "-d", "--", tagName)
	return err
}

// GetStashes returns list of stashes
func (GitService) GetStashes(repoPath string) ([]StashInfo, error) {
	raw, err := ExecGit(repoPath, "stash", "list", "--format=%gd%x09%cr%x09%gs")
	if err != nil {
		return nil, err
	}

	var stashes []StashInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pts := strings.Split(line, "\t")
		if len(pts) >= 3 {
			// pts[0] is e.g. "stash@{0}"
			idxStr := strings.TrimPrefix(pts[0], "stash@{")
			idxStr = strings.TrimSuffix(idxStr, "}")
			idx, _ := strconv.Atoi(idxStr)
			stashes = append(stashes, StashInfo{
				Index:   idx,
				Date:    pts[1],
				Message: pts[2],
			})
		}
	}
	return stashes, nil
}

// GetStashDiff returns the diff of changes saved in a stash
func (GitService) GetStashDiff(repoPath string, index int) (string, error) {
	stashRef := fmt.Sprintf("stash@{%d}", index)
	raw, err := ExecGit(repoPath, "stash", "show", "-p", stashRef)
	if err != nil {
		return raw, err
	}
	capped, _ := truncateDiffPayload(raw)
	return capped, nil
}

// StashApply applies stash at index without removing it from the stash list
func (GitService) StashApply(repoPath string, index int) error {
	stashRef := fmt.Sprintf("stash@{%d}", index)
	_, err := ExecGit(repoPath, "stash", "apply", stashRef)
	return err
}

// StashBranch creates and checks out a new branch from a stash
func (GitService) StashBranch(repoPath string, index int, branchName string) error {
	if branchName == "" {
		return errors.New("branch name cannot be empty")
	}
	stashRef := fmt.Sprintf("stash@{%d}", index)
	_, err := ExecGit(repoPath, "stash", "branch", branchName, stashRef)
	return err
}

// StashSave stashes working directory changes
func (GitService) StashSave(repoPath string, message string, keepIndex bool, includeUntracked bool) error {
	return GitService{}.StashSaveAdvanced(repoPath, message, keepIndex, includeUntracked, false)
}

// StashSaveAdvanced stashes working directory changes with full options including ignored files
func (GitService) StashSaveAdvanced(repoPath string, message string, keepIndex bool, includeUntracked bool, includeAll bool) error {
	args := []string{"stash", "push"}
	if keepIndex {
		args = append(args, "--keep-index")
	}
	if includeAll {
		args = append(args, "-a")
	} else if includeUntracked {
		args = append(args, "-u")
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// StashPop applies and drops stash at index
func (GitService) StashPop(repoPath string, index int) error {
	stashRef := fmt.Sprintf("stash@{%d}", index)
	_, err := ExecGit(repoPath, "stash", "pop", stashRef)
	return err
}

// StashDrop drops stash at index
func (GitService) StashDrop(repoPath string, index int) error {
	stashRef := fmt.Sprintf("stash@{%d}", index)
	_, err := ExecGit(repoPath, "stash", "drop", stashRef)
	return err
}

// Fetch fetches from remote
func (GitService) Fetch(repoPath string, remote string) (string, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	return ExecGit(repoPath, "fetch", "--prune", remote)
}

// Pull pulls from remote branch
func (GitService) Pull(repoPath string, remote string, branch string) (string, error) {
	args := []string{"pull"}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}
	return ExecGit(repoPath, args...)
}

// Push pushes to remote branch
func (GitService) Push(repoPath string, remote string, branch string, force bool) (string, error) {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}
	return ExecGit(repoPath, args...)
}

// OpenInTerminal opens the repo directory in default system Terminal
func (GitService) OpenInTerminal(repoPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Terminal", repoPath)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "cmd.exe", "/k", "cd", repoPath)
	default:
		cmd = exec.Command("x-terminal-emulator", "--working-directory="+repoPath)
	}
	return cmd.Start()
}

// OpenInFinder opens the repo directory in Finder / File Explorer
func (GitService) OpenInFinder(repoPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", repoPath)
	case "windows":
		cmd = exec.Command("explorer", repoPath)
	default:
		cmd = exec.Command("xdg-open", repoPath)
	}
	return cmd.Start()
}

func getRepoInfo(path string) (*RepoInfo, error) {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	name := filepath.Base(path)
	rawStatus, err := ExecGit(path, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, err
	}
	status := ParseRepoStatus(path, rawStatus)

	rawRemotes, _ := ExecGit(path, "remote")
	var remotes []string
	for _, r := range strings.Split(rawRemotes, "\n") {
		r = strings.TrimSpace(r)
		if r != "" {
			remotes = append(remotes, r)
		}
	}

	return &RepoInfo{
		Path:          path,
		Name:          name,
		CurrentBranch: status.CurrentBranch,
		Status:        status,
		Remotes:       remotes,
	}, nil
}

type LargeFileInfo struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SizeHuman string `json:"sizeHuman"`
	IsLFS     bool   `json:"isLfs"`
}

// RevertCommit reverts the changes introduced by a commit
func (GitService) RevertCommit(repoPath string, commitHash string) error {
	if commitHash == "" {
		return errors.New("commit hash is required")
	}
	_, err := ExecGit(repoPath, "revert", "--no-edit", commitHash)
	return err
}

// CherryPick applies the changes from an existing commit to current branch
func (GitService) CherryPick(repoPath string, commitHash string) error {
	if commitHash == "" {
		return errors.New("commit hash is required")
	}
	_, err := ExecGit(repoPath, "cherry-pick", commitHash)
	return err
}

// ResetBranch resets current HEAD to the specified commit (soft, mixed, hard)
func (GitService) ResetBranch(repoPath string, commitHash string, mode string) error {
	if commitHash == "" {
		return errors.New("commit hash is required")
	}
	flag := "--mixed"
	switch strings.ToLower(mode) {
	case "soft":
		flag = "--soft"
	case "hard":
		flag = "--hard"
	case "mixed":
		flag = "--mixed"
	}
	_, err := ExecGit(repoPath, "reset", flag, commitHash)
	return err
}

// DiscardAll discards all unstaged changes and cleans untracked files
func (GitService) DiscardAll(repoPath string) error {
	if repoPath == "" {
		return errors.New("repoPath 不能为空")
	}
	// An empty repository has no HEAD to restore from; untracked files can
	// still be cleaned safely.
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); err == nil {
		if _, err := ExecGit(repoPath, "restore", "--", "."); err != nil {
			return fmt.Errorf("恢复已跟踪文件失败: %w", err)
		}
	}
	if _, err := ExecGit(repoPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("清理未跟踪文件失败: %w", err)
	}
	return nil
}

// RenameBranch renames a local branch
func (GitService) RenameBranch(repoPath string, oldName string, newName string) error {
	if oldName == "" || newName == "" {
		return errors.New("branch names cannot be empty")
	}
	_, err := ExecGit(repoPath, "branch", "-m", oldName, newName)
	return err
}

// ScanLargeFiles scans repository working tree for large assets (Tencent UGit feature)
func (GitService) ScanLargeFiles(repoPath string, minSizeMB int) ([]LargeFileInfo, error) {
	if minSizeMB <= 0 {
		minSizeMB = 5 // default 5MB
	}
	threshold := int64(minSizeMB) * 1024 * 1024

	list := []LargeFileInfo{}
	lfsExtensions := getLFSExtensions(repoPath)

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip .git directory and node_modules
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "build" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Size() >= threshold || isPathLFS(path, lfsExtensions) {
			rel, relErr := filepath.Rel(repoPath, path)
			if relErr == nil {
				list = append(list, LargeFileInfo{
					Path:      rel,
					SizeBytes: info.Size(),
					SizeHuman: formatBytes(info.Size()),
					IsLFS:     isPathLFS(rel, lfsExtensions),
				})
			}
		}
		return nil
	})

	return list, err
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

type GitUserConfig struct {
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	IsGlobal  bool   `json:"isGlobal"`
}

type RemoteDetail struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"` // "fetch" or "push"
}

// AddToGitignore appends a pattern or file path to .gitignore
func (GitService) AddToGitignore(repoPath string, pattern string) error {
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	ignoreFile := filepath.Join(repoPath, ".gitignore")
	var content []byte
	if data, err := os.ReadFile(ignoreFile); err == nil {
		content = data
	}

	str := string(content)
	if !strings.HasSuffix(str, "\n") && len(str) > 0 {
		str += "\n"
	}
	str += pattern + "\n"

	return os.WriteFile(ignoreFile, []byte(str), 0644)
}

// GetGitConfig gets user.name and user.email
func (GitService) GetGitConfig(repoPath string) (*GitUserConfig, error) {
	name, _ := ExecGit(repoPath, "config", "user.name")
	email, _ := ExecGit(repoPath, "config", "user.email")
	return &GitUserConfig{
		UserName:  name,
		UserEmail: email,
	}, nil
}

// SetGitConfig sets user.name and user.email locally or globally
func (GitService) SetGitConfig(repoPath string, name string, email string, global bool) error {
	var argsName, argsEmail []string
	if global {
		argsName = []string{"config", "--global", "user.name", name}
		argsEmail = []string{"config", "--global", "user.email", email}
	} else {
		argsName = []string{"config", "user.name", name}
		argsEmail = []string{"config", "user.email", email}
	}

	if _, err := ExecGit(repoPath, argsName...); err != nil {
		return err
	}
	if _, err := ExecGit(repoPath, argsEmail...); err != nil {
		return err
	}
	return nil
}

// GetRemoteDetails returns list of remotes with URL and type
func (GitService) GetRemoteDetails(repoPath string) ([]RemoteDetail, error) {
	raw, err := ExecGit(repoPath, "remote", "-v")
	if err != nil {
		return nil, err
	}

	var list []RemoteDetail
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: origin	https://github.com/... (fetch)
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			name := fields[0]
			url := fields[1]
			t := strings.Trim(fields[2], "()")
			list = append(list, RemoteDetail{
				Name: name,
				URL:  url,
				Type: t,
			})
		}
	}
	return list, nil
}

// AddRemote adds a new remote
func (GitService) AddRemote(repoPath string, name string, url string) error {
	if name == "" || url == "" {
		return errors.New("remote name and URL are required")
	}
	_, err := ExecGit(repoPath, "remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote
func (GitService) RemoveRemote(repoPath string, name string) error {
	if name == "" {
		return errors.New("remote name is required")
	}
	_, err := ExecGit(repoPath, "remote", "remove", name)
	return err
}

// GetLastCommitMessage returns subject and body of HEAD commit
func (GitService) GetLastCommitMessage(repoPath string) (string, error) {
	return ExecGit(repoPath, "log", "-1", "--pretty=format:%B")
}

type BlameLine struct {
	LineNumber int    `json:"lineNumber"`
	CommitHash string `json:"commitHash"`
	ShortHash  string `json:"shortHash"`
	Author     string `json:"author"`
	Date       string `json:"date"`
	Summary    string `json:"summary"`
	Content    string `json:"content"`
}

// GetFileBlame returns git blame line by line with commit, author and date
func (GitService) GetFileBlame(repoPath string, filePath string) ([]BlameLine, error) {
	if filePath == "" {
		return nil, errors.New("file path is required")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}
	raw, err := ExecGit(repoPath, "blame", "--line-porcelain", "--", cleanPath)
	if err != nil {
		return nil, err
	}

	var result []BlameLine
	lines := strings.Split(raw, "\n")

	var currentHash, currentShort, currentAuthor, currentDate, currentSummary string
	lineNum := 1

	for _, line := range lines {
		if len(line) >= 40 && !strings.Contains(line[:40], " ") && strings.Contains(line, " ") {
			// First line of header: <hash> <orig_line> <final_line>
			pts := strings.Fields(line)
			if len(pts) >= 3 {
				currentHash = pts[0]
				if len(currentHash) >= 7 {
					currentShort = currentHash[:7]
				} else {
					currentShort = currentHash
				}
			}
			continue
		}

		if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			tsStr := strings.TrimPrefix(line, "author-time ")
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err == nil {
				currentDate = time.Unix(ts, 0).Format("2006-01-02")
			}
		} else if strings.HasPrefix(line, "summary ") {
			currentSummary = strings.TrimPrefix(line, "summary ")
		} else if strings.HasPrefix(line, "\t") {
			// Content line
			content := strings.TrimPrefix(line, "\t")
			result = append(result, BlameLine{
				LineNumber: lineNum,
				CommitHash: currentHash,
				ShortHash:  currentShort,
				Author:     currentAuthor,
				Date:       currentDate,
				Summary:    currentSummary,
				Content:    content,
			})
			lineNum++
			// Large-repo guard: generated files can have hundreds of thousands of
			// lines; blame only needs a bounded window for the gutter UI.
			if len(result) >= 30000 {
				break
			}
		}
	}

	return result, nil
}

type ContributorStat struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Commits int    `json:"commits"`
}

type RepoOverviewStats struct {
	TotalCommits  int               `json:"totalCommits"`
	TotalBranches int               `json:"totalBranches"`
	TotalTags     int               `json:"totalTags"`
	Contributors  []ContributorStat `json:"contributors"`
}

// GetRepoOverview gathers high-level stats about the repository
func (s GitService) GetRepoOverview(repoPath string) (*RepoOverviewStats, error) {
	// Total Commits
	countStr, err := ExecGit(repoPath, "rev-list", "--count", "--all")
	if err != nil {
		return nil, fmt.Errorf("读取提交总数失败: %v", err)
	}
	totalCommits, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil {
		return nil, fmt.Errorf("解析提交总数失败: %v", err)
	}

	// Branches & Tags
	branches, err := s.GetBranches(repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取分支统计失败: %v", err)
	}
	tags, err := s.GetTags(repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取标签统计失败: %v", err)
	}

	// Contributors
	shortlog, err := ExecGit(repoPath, "shortlog", "-sne", "--all")
	if err != nil {
		return nil, fmt.Errorf("读取贡献者统计失败: %v", err)
	}
	contributors := []ContributorStat{}
	for _, line := range strings.Split(shortlog, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// format: <count>\t<name> <<email>>
		pts := strings.Split(line, "\t")
		if len(pts) >= 2 {
			count, _ := strconv.Atoi(strings.TrimSpace(pts[0]))
			authorPart := strings.TrimSpace(pts[1])
			name := authorPart
			email := ""
			if strings.Contains(authorPart, " <") && strings.HasSuffix(authorPart, ">") {
				eStart := strings.Index(authorPart, " <")
				name = authorPart[:eStart]
				email = strings.TrimSuffix(authorPart[eStart+2:], ">")
			}
			contributors = append(contributors, ContributorStat{
				Name:    name,
				Email:   email,
				Commits: count,
			})
		}
	}

	return &RepoOverviewStats{
		TotalCommits:  totalCommits,
		TotalBranches: len(branches),
		TotalTags:     len(tags),
		Contributors:  contributors,
	}, nil
}

// StageHunk applies a patch hunk directly to the git staging area (index)
func (GitService) StageHunk(repoPath string, patch string) error {
	if patch == "" {
		return errors.New("patch cannot be empty")
	}
	_, err := ExecGitWithStdin(repoPath, patch, "apply", "--cached", "--unidiff-zero", "-")
	return err
}

// DiscardHunk applies a patch in reverse to discard a hunk in the working tree
func (GitService) DiscardHunk(repoPath string, patch string) error {
	if patch == "" {
		return errors.New("patch cannot be empty")
	}
	_, err := ExecGitWithStdin(repoPath, patch, "apply", "--reverse", "--unidiff-zero", "-")
	return err
}

// GetDiffBetween compares two revisions (commit hashes, branches, or tags)
func (GitService) GetDiffBetween(repoPath string, refA string, refB string) (*DiffComparisonResult, error) {
	if refA == "" || refB == "" {
		return nil, errors.New("both references are required for comparison")
	}
	rangeArg := fmt.Sprintf("%s..%s", refA, refB)
	return buildDiffComparison(repoPath, rangeArg, refA, refB)
}

// GetSubmodules returns all git submodules registered in the repository
func (GitService) GetSubmodules(repoPath string) ([]SubmoduleInfo, error) {
	gitmodulesPath := filepath.Join(repoPath, ".gitmodules")
	if _, err := os.Stat(gitmodulesPath); os.IsNotExist(err) {
		return []SubmoduleInfo{}, nil
	}

	raw, err := ExecGit(repoPath, "submodule", "status")
	if err != nil {
		return nil, fmt.Errorf("读取子模块状态失败: %w", err)
	}

	list := []SubmoduleInfo{}
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// status character: ' ' (synced), '+' (modified commit), '-' (not initialized), 'U' (conflict)
		statusChar := line[0]
		statusStr := "synced"
		switch statusChar {
		case '-':
			statusStr = "uninitialized"
		case '+':
			statusStr = "modified"
		case 'U':
			statusStr = "conflict"
		}

		fields := strings.Fields(line)
		commitHash := ""
		subPath := ""
		branch := ""

		if len(fields) >= 1 {
			commitHash = strings.TrimPrefix(fields[0], "-")
			commitHash = strings.TrimPrefix(commitHash, "+")
			commitHash = strings.TrimPrefix(commitHash, "U")
		}
		if len(fields) >= 2 {
			subPath = fields[1]
		}
		if len(fields) >= 3 {
			branch = strings.Trim(fields[2], "()")
		}

		// Retrieve submodule URL from config
		subURL, _ := ExecGit(repoPath, "config", "-f", ".gitmodules", fmt.Sprintf("submodule.%s.url", subPath))
		if subURL == "" {
			subURL, _ = ExecGit(repoPath, "config", fmt.Sprintf("submodule.%s.url", subPath))
		}

		name := filepath.Base(subPath)
		list = append(list, SubmoduleInfo{
			Name:   name,
			Path:   subPath,
			URL:    strings.TrimSpace(subURL),
			Branch: branch,
			Commit: commitHash,
			Status: statusStr,
		})
	}

	return list, nil
}

// UpdateSubmodules runs git submodule update
func (GitService) UpdateSubmodules(repoPath string, init bool, recursive bool) (string, error) {
	args := []string{"submodule", "update"}
	if init {
		args = append(args, "--init")
	}
	if recursive {
		args = append(args, "--recursive")
	}
	return ExecGit(repoPath, args...)
}

// LockFile locks a file using Git LFS
func (GitService) LockFile(repoPath string, filePath string) error {
	_, err := ExecGit(repoPath, "lfs", "lock", filePath)
	return err
}

// UnlockFile unlocks a file using Git LFS
func (GitService) UnlockFile(repoPath string, filePath string, force bool) error {
	args := []string{"lfs", "unlock"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, filePath)
	_, err := ExecGit(repoPath, args...)
	return err
}

// CleanWorkingTree cleans untracked files with dry-run support
func (GitService) CleanWorkingTree(repoPath string, dryRun bool) ([]string, error) {
	args := []string{"clean", "-d"}
	if dryRun {
		args = append(args, "-n")
	} else {
		args = append(args, "-f")
	}
	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Would remove ") {
			files = append(files, strings.TrimPrefix(line, "Would remove "))
		} else if strings.HasPrefix(line, "Removing ") {
			files = append(files, strings.TrimPrefix(line, "Removing "))
		}
	}
	return files, nil
}

// GetGitHooks inspects active and sample hooks in the repository
func (GitService) GetGitHooks(repoPath string) ([]GitHookInfo, error) {
	hooksDir := gitPath(repoPath, "hooks")
	var hooks []GitHookInfo

	standardHooks := []string{
		"pre-commit", "prepare-commit-msg", "commit-msg", "post-commit",
		"pre-rebase", "post-checkout", "post-merge", "pre-push",
	}

	for _, name := range standardHooks {
		hookPath := filepath.Join(hooksDir, name)
		installed := false
		enabled := false
		if info, err := os.Stat(hookPath); err == nil && !info.IsDir() {
			installed = true
			if runtime.GOOS != "windows" {
				enabled = (info.Mode() & 0111) != 0
			} else {
				enabled = true
			}
		} else if info, err := os.Stat(hookPath + ".disabled"); err == nil && !info.IsDir() {
			installed = true
		}
		hooks = append(hooks, GitHookInfo{
			Name:      name,
			Enabled:   enabled,
			Installed: installed,
			Path:      hookPath,
		})
	}
	return hooks, nil
}

// ExportArchive exports a commit, branch or tag as a zip archive
func (GitService) ExportArchive(repoPath string, ref string, outputPath string) error {
	if ref == "" {
		ref = "HEAD"
	}
	if outputPath == "" {
		return errors.New("output path cannot be empty")
	}
	_, err := ExecGit(repoPath, "archive", "--format=zip", fmt.Sprintf("--output=%s", outputPath), ref)
	return err
}

// SetUpstream configures the upstream tracking branch for a local branch
func (GitService) SetUpstream(repoPath string, branch string, upstream string) error {
	_, err := ExecGit(repoPath, "branch", fmt.Sprintf("--set-upstream-to=%s", upstream), branch)
	return err
}

// UnsetUpstream removes the upstream tracking branch for a local branch
func (GitService) UnsetUpstream(repoPath string, branchName string) error {
	branch := strings.TrimSpace(branchName)
	args := []string{"branch", "--unset-upstream"}
	if branch != "" {
		args = append(args, branch)
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// StageLines stages arbitrary patch lines into the git staging area (index)
func (GitService) StageLines(repoPath string, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("patch cannot be empty")
	}
	_, err := ExecGitWithStdin(repoPath, patch, "apply", "--cached", "--unidiff-zero", "-")
	return err
}

// DiscardLines discards arbitrary patch lines from the working directory
func (GitService) DiscardLines(repoPath string, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return errors.New("patch cannot be empty")
	}
	_, err := ExecGitWithStdin(repoPath, patch, "apply", "--reverse", "--unidiff-zero", "-")
	return err
}

// BisectStatus returns current git bisect state
func (GitService) BisectStatus(repoPath string) (*BisectStatus, error) {
	// Resolve the per-worktree git dir (plain repos: .git, linked worktrees: elsewhere).
	bisectStartPath := filepath.Join(repoPath, ".git", "BISECT_START")
	if out, err := ExecGit(repoPath, "rev-parse", "--git-path", "BISECT_START"); err == nil {
		p := strings.TrimSpace(out)
		if p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(repoPath, p)
			}
			bisectStartPath = p
		}
	}
	if _, err := os.Stat(bisectStartPath); os.IsNotExist(err) {
		return &BisectStatus{IsActive: false, GoodRefs: []string{}}, nil
	}

	// BISECT_START stores the ref the session started from (used by reset); the bad
	// commit lives in refs/bisect/bad and the good candidates in refs/bisect/good-*.
	startRef, _ := os.ReadFile(bisectStartPath)
	badHash, _ := ExecGit(repoPath, "rev-parse", "--short", "refs/bisect/bad")
	badHash = strings.TrimSpace(badHash)
	if badHash == "" {
		// Older/minimal bisect states may only carry BISECT_START.
		badHash = strings.TrimSpace(string(startRef))
	}

	goodOut, _ := ExecGit(repoPath, "for-each-ref", "--format=%(objectname:short)", "refs/bisect/good-*")
	goodRefs := strings.Fields(goodOut)
	if goodRefs == nil {
		goodRefs = []string{}
	}

	logOut, _ := ExecGit(repoPath, "bisect", "log")
	headHash, _ := ExecGit(repoPath, "rev-parse", "--short", "HEAD")

	judgements := 0
	for _, line := range strings.Split(logOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "git bisect good") ||
			strings.HasPrefix(line, "git bisect bad") ||
			strings.HasPrefix(line, "git bisect skip") {
			judgements++
		}
	}
	step := ""
	if judgements > 0 {
		step = fmt.Sprintf("已完成 %d 次 good/bad 判定", judgements)
	}

	var currentSubject, currentAuthor, currentDate string
	if strings.TrimSpace(headHash) != "" {
		if cInfo, err := ExecGit(repoPath, "log", "-1", "--format=%s%x00%an%x00%cr", strings.TrimSpace(headHash)); err == nil {
			parts := strings.Split(strings.TrimSpace(cInfo), "\x00")
			if len(parts) >= 3 {
				currentSubject = parts[0]
				currentAuthor = parts[1]
				currentDate = parts[2]
			}
		}
	}

	return &BisectStatus{
		IsActive:       true,
		CurrentHash:    strings.TrimSpace(headHash),
		CurrentStep:    step,
		BadRef:         badHash,
		GoodRefs:       goodRefs,
		Log:            logOut,
		CurrentSubject: currentSubject,
		CurrentAuthor:  currentAuthor,
		CurrentDate:    currentDate,
	}, nil
}

// BisectStart begins a binary search session
func (s GitService) BisectStart(repoPath string, badRef string, goodRef string) (*BisectStatus, error) {
	if badRef == "" {
		badRef = "HEAD"
	}
	_, _ = ExecGit(repoPath, "bisect", "reset")
	_, err := ExecGit(repoPath, "bisect", "start", badRef)
	if err != nil {
		return nil, err
	}
	if goodRef != "" {
		_, err = ExecGit(repoPath, "bisect", "good", goodRef)
		if err != nil {
			return nil, err
		}
	}
	return s.BisectStatus(repoPath)
}

// BisectGood marks current commit as good
func (s GitService) BisectGood(repoPath string) (*BisectStatus, error) {
	out, err := ExecGit(repoPath, "bisect", "good")
	if err != nil {
		return nil, fmt.Errorf("标记 good 失败：%s %s", strings.TrimSpace(out), err.Error())
	}
	return s.BisectStatus(repoPath)
}

// BisectBad marks current commit as bad
func (s GitService) BisectBad(repoPath string) (*BisectStatus, error) {
	out, err := ExecGit(repoPath, "bisect", "bad")
	if err != nil {
		return nil, fmt.Errorf("标记 bad 失败：%s %s", strings.TrimSpace(out), err.Error())
	}
	return s.BisectStatus(repoPath)
}

// BisectReset resets and terminates the bisect session
func (GitService) BisectReset(repoPath string) error {
	_, err := ExecGit(repoPath, "bisect", "reset")
	return err
}

// BisectRun automates the binary search by executing a script or shell command at each revision
func (s GitService) BisectRun(repoPath string, scriptCmd string) (*BisectRunResult, error) {
	if strings.TrimSpace(scriptCmd) == "" {
		return nil, errors.New("测试脚本命令不能为空")
	}
	status, err := s.BisectStatus(repoPath)
	if err != nil {
		return nil, err
	}
	if !status.IsActive {
		return nil, errors.New("当前没有进行中的 Bisect 会话，请先启动二分会话")
	}

	out, runErr := ExecGitWithTimeout(repoPath, 300*time.Second, "bisect", "run", "sh", "-c", scriptCmd)
	updatedStatus, _ := s.BisectStatus(repoPath)

	res := &BisectRunResult{
		Success: runErr == nil,
		Log:     out,
		Status:  updatedStatus,
	}

	firstBadRe := regexp.MustCompile(`([0-9a-fA-F]{7,40})\s+is the first bad commit`)
	if m := firstBadRe.FindStringSubmatch(out); len(m) > 1 {
		res.FirstBadCommit = m[1]
	} else if updatedStatus != nil {
		re := regexp.MustCompile(`first\s+['"]?bad['"]?\s+commit:\s*\[([0-9a-fA-F]{7,40})\]`)
		if m := re.FindStringSubmatch(updatedStatus.Log); len(m) > 1 {
			res.FirstBadCommit = m[1]
		}
	}

	if runErr != nil {
		res.Message = fmt.Sprintf("Bisect 自动运行中断或失败: %v", runErr)
	} else if res.FirstBadCommit != "" {
		res.Message = fmt.Sprintf("Bisect 自动排查成功定位到坏提交: %s", res.FirstBadCommit)
	} else {
		res.Message = "Bisect 自动测试完成"
	}

	return res, nil
}

// GetReflog returns recent HEAD reflog items
func (GitService) GetReflog(repoPath string, limit int) ([]ReflogItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("读取 reflog 失败: %w", err)
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); err != nil {
		// A repository without commits has no reflog yet; that is an honest
		// empty state, not a read failure.
		return []ReflogItem{}, nil
	}
	raw, err := ExecGit(repoPath, "reflog", fmt.Sprintf("-n%d", limit), "--format=%gD%x1f%h%x1f%gs%x1f%cr")
	if err != nil {
		return nil, fmt.Errorf("读取 reflog 失败: %w", err)
	}

	items := []ReflogItem{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pts := strings.Split(line, "\x1f")
		if len(pts) >= 4 {
			action := ""
			msg := pts[2]
			if colonIdx := strings.Index(msg, ":"); colonIdx > 0 {
				action = strings.TrimSpace(msg[:colonIdx])
			}
			items = append(items, ReflogItem{
				Selector:   pts[0],
				CommitHash: pts[1],
				Action:     action,
				Message:    msg,
				Date:       pts[3],
			})
		}
	}
	return items, nil
}

// CheckoutReflog checks out a historical reflog point
func (GitService) CheckoutReflog(repoPath string, selector string) error {
	if selector == "" {
		return errors.New("reflog selector cannot be empty")
	}
	_, err := ExecGit(repoPath, "checkout", selector)
	return err
}

// CreateBranchFromReflog creates a new branch from a reflog hash
func (GitService) CreateBranchFromReflog(repoPath string, branchName string, commitHash string) error {
	if branchName == "" || commitHash == "" {
		return errors.New("branch name and commit hash are required")
	}
	_, err := ExecGit(repoPath, "branch", branchName, commitHash)
	return err
}

// GetWorktrees returns registered git worktrees
func (GitService) GetWorktrees(repoPath string) ([]WorktreeInfo, error) {
	raw, err := ExecGit(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("读取工作树列表失败: %w", err)
	}

	list := []WorktreeInfo{}
	var current WorktreeInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				list = append(list, current)
				current = WorktreeInfo{}
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Head = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(current.Branch, "refs/heads/")
		} else if line == "bare" {
			current.IsBare = true
		} else if strings.HasPrefix(line, "locked") {
			current.IsLocked = true
			current.LockReason = strings.TrimPrefix(line, "locked ")
		}
	}
	if current.Path != "" {
		list = append(list, current)
	}
	// git worktree list always emits the main worktree first.
	if len(list) > 0 {
		list[0].IsMain = true
	}
	return list, nil
}

// AddWorktree creates a new linked working tree
func (GitService) AddWorktree(repoPath string, path string, branch string, newBranch bool) error {
	if path == "" {
		return errors.New("worktree path cannot be empty")
	}
	args := []string{"worktree", "add"}
	if newBranch && branch != "" {
		args = append(args, "-b", branch, path)
	} else if branch != "" {
		args = append(args, path, branch)
	} else {
		args = append(args, path)
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// RemoveWorktree deletes a linked working tree
func (GitService) RemoveWorktree(repoPath string, path string, force bool) error {
	if path == "" {
		return errors.New("worktree path cannot be empty")
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := ExecGit(repoPath, args...)
	return err
}

// AddSubmodule adds a new git submodule
func (GitService) AddSubmodule(repoPath string, url string, path string) error {
	if url == "" || path == "" {
		return errors.New("url and path cannot be empty")
	}
	_, err := ExecGit(repoPath, "submodule", "add", url, path)
	return err
}

// RemoveSubmodule unbinds and removes a submodule
func (GitService) RemoveSubmodule(repoPath string, path string) error {
	if path == "" {
		return errors.New("submodule path cannot be empty")
	}
	if _, err := ExecGit(repoPath, "submodule", "deinit", "-f", "--", path); err != nil {
		return fmt.Errorf("取消初始化子模块失败: %w", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, ".gitmodules")); err == nil {
		// git rm refuses to touch a .gitmodules with unstaged edits (e.g. branch config).
		if _, err := ExecGit(repoPath, "add", "--", ".gitmodules"); err != nil {
			return err
		}
	}
	_, err := ExecGit(repoPath, "rm", "-f", "--", path)
	if err != nil {
		return err
	}
	if data, readErr := os.ReadFile(filepath.Join(repoPath, ".gitmodules")); readErr == nil && !strings.Contains(string(data), "[submodule") {
		if removeErr := os.Remove(filepath.Join(repoPath, ".gitmodules")); removeErr != nil {
			return fmt.Errorf("子模块已移除，但清理空 .gitmodules 失败: %w", removeErr)
		}
		if _, addErr := ExecGit(repoPath, "add", "-A", "--", ".gitmodules"); addErr != nil {
			return fmt.Errorf("子模块已移除，但暂存 .gitmodules 删除失败: %w", addErr)
		}
	}
	return nil
}

// ResolveConflict chooses ours, theirs, backup_and_theirs, mergetool, or marks conflict resolved
func (GitService) ResolveConflict(repoPath string, filePath string, resolution string) error {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return err
	}
	filePath = cleanPath

	switch resolution {
	case "ours":
		if _, err := ExecGit(repoPath, "checkout", "--ours", "--", filePath); err != nil {
			return err
		}
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	case "theirs":
		if _, err := ExecGit(repoPath, "checkout", "--theirs", "--", filePath); err != nil {
			return err
		}
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	case "backup_and_theirs", "backup_theirs":
		// Tencent UGit safe resolution: Backup local file then checkout theirs
		src := filepath.Join(repoPath, filePath)
		ext := filepath.Ext(filePath)
		base := strings.TrimSuffix(filePath, ext)
		ts := time.Now().Format("20060102_150405")
		backupPath := filepath.Join(repoPath, fmt.Sprintf("%s.backup_%s%s", base, ts, ext))
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("读取本地冲突文件失败，未覆盖: %w", err)
		}
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			return fmt.Errorf("创建本地备份失败，未覆盖: %w", err)
		}
		if _, err := ExecGit(repoPath, "checkout", "--theirs", "--", filePath); err != nil {
			return err
		}
		_, err = ExecGit(repoPath, "add", "--", filePath)
		return err
	case "mergetool":
		_, err := ExecGit(repoPath, "mergetool", "-y", "--", filePath)
		return err
	case "mark", "mark_resolved":
		_, err := ExecGit(repoPath, "add", "--", filePath)
		return err
	default:
		return fmt.Errorf("unknown resolution: %s", resolution)
	}
}

// GetConflictFileContent reads content of conflicted file from disk
func (GitService) GetConflictFileContent(repoPath string, filePath string) (string, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(repoPath, cleanPath))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

type ConflictBlockResolveResult struct {
	Success            bool   `json:"success"`
	RemainingConflicts int    `json:"remainingConflicts"`
	FileAutoStaged     bool   `json:"fileAutoStaged"`
	Message            string `json:"message"`
}

type conflictBlock struct {
	startLine   int
	endLine     int
	oursLines   []string
	theirsLines []string
}

func parseConflictBlocks(lines []string) []conflictBlock {
	var blocks []conflictBlock
	const (
		stateOutside = iota
		stateOurs
		stateBase
		stateTheirs
	)

	state := stateOutside
	currStart := -1
	var currOurs []string
	var currTheirs []string

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<<"):
			state = stateOurs
			currStart = i
			currOurs = nil
			currTheirs = nil
		case strings.HasPrefix(trimmed, "|||||||") && state == stateOurs:
			state = stateBase
		case (trimmed == "=======" || strings.HasPrefix(trimmed, "=======")) && (state == stateOurs || state == stateBase):
			state = stateTheirs
			currTheirs = nil
		case strings.HasPrefix(trimmed, ">>>>>>>") && state == stateTheirs:
			blocks = append(blocks, conflictBlock{
				startLine:   currStart,
				endLine:     i,
				oursLines:   currOurs,
				theirsLines: currTheirs,
			})
			state = stateOutside
			currStart = -1
			currOurs = nil
			currTheirs = nil
		default:
			switch state {
			case stateOurs:
				currOurs = append(currOurs, line)
			case stateBase:
				// Skip diff3 common ancestor lines
			case stateTheirs:
				currTheirs = append(currTheirs, line)
			}
		}
	}

	return blocks
}

// ResolveConflictBlock resolves a specific conflict block in a file by blockIndex
// with choice ("ours", "theirs", "both"), updates the file, and auto-stages if no conflicts remain.
func (GitService) ResolveConflictBlock(repoPath string, filePath string, blockIndex int, choice string) (*ConflictBlockResolveResult, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(repoPath, cleanPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	choiceNorm := strings.ToLower(strings.TrimSpace(choice))
	if choiceNorm != "ours" && choiceNorm != "theirs" && choiceNorm != "both" {
		return nil, fmt.Errorf("invalid choice %q: must be 'ours', 'theirs', or 'both'", choice)
	}

	content := string(data)
	isCRLF := strings.Contains(content, "\r\n")
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	rawLines := strings.Split(normalized, "\n")

	blocks := parseConflictBlocks(rawLines)
	if blockIndex < 0 || blockIndex >= len(blocks) {
		return nil, fmt.Errorf("conflict block index %d out of range (found %d blocks)", blockIndex, len(blocks))
	}

	target := blocks[blockIndex]
	var replacementLines []string
	switch choiceNorm {
	case "ours":
		replacementLines = target.oursLines
	case "theirs":
		replacementLines = target.theirsLines
	case "both":
		replacementLines = append(append([]string{}, target.oursLines...), target.theirsLines...)
	}

	newLines := make([]string, 0, len(rawLines)-(target.endLine-target.startLine+1)+len(replacementLines))
	newLines = append(newLines, rawLines[:target.startLine]...)
	newLines = append(newLines, replacementLines...)
	newLines = append(newLines, rawLines[target.endLine+1:]...)

	newContent := strings.Join(newLines, "\n")
	if isCRLF {
		newContent = strings.ReplaceAll(newContent, "\n", "\r\n")
	}

	info, err := os.Stat(fullPath)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(fullPath, []byte(newContent), perm); err != nil {
		return nil, fmt.Errorf("failed to write updated file %s: %w", cleanPath, err)
	}

	remaining := strings.Count(newContent, "<<<<<<<")
	fileAutoStaged := false
	if remaining == 0 {
		if _, err := ExecGit(repoPath, "add", "--", cleanPath); err != nil {
			return nil, fmt.Errorf("failed to auto-stage file %s: %w", cleanPath, err)
		}
		fileAutoStaged = true
	}

	var msg string
	if fileAutoStaged {
		msg = fmt.Sprintf("Conflict block %d resolved with %s; all conflicts resolved and %s auto-staged", blockIndex, choiceNorm, cleanPath)
	} else {
		msg = fmt.Sprintf("Conflict block %d resolved with %s; %d conflict marker(s) remaining", blockIndex, choiceNorm, remaining)
	}

	return &ConflictBlockResolveResult{
		Success:            true,
		RemainingConflicts: remaining,
		FileAutoStaged:     fileAutoStaged,
		Message:            msg,
	}, nil
}

// GetCommitsFiltered filters commits by branch, author, date range, and keyword
func (GitService) GetCommitsFiltered(repoPath string, branch string, author string, since string, until string, keyword string, skip int, limit int) (*CommitPaginationResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if skip < 0 {
		skip = 0
	}
	if branch != "" && branch != "HEAD" {
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", branch+"^{commit}"); err != nil {
			return nil, fmt.Errorf("分支 %s 不存在或不可解析: %w", branch, err)
		}
	}

	// Scope + filter arguments are shared by the log page query and the total count,
	// so large-project UIs can show "已加载 N / 共 M" for filtered history too.
	scopeAndFilters := []string{}
	if branch != "" && branch != "HEAD" {
		scopeAndFilters = append(scopeAndFilters, branch)
	} else {
		scopeAndFilters = append(scopeAndFilters, "--all")
	}
	if author != "" {
		scopeAndFilters = append(scopeAndFilters, fmt.Sprintf("--author=%s", author))
	}
	if since != "" {
		scopeAndFilters = append(scopeAndFilters, fmt.Sprintf("--since=%s", since))
	}
	if until != "" {
		scopeAndFilters = append(scopeAndFilters, fmt.Sprintf("--until=%s", until))
	}
	if keyword != "" {
		scopeAndFilters = append(scopeAndFilters, fmt.Sprintf("--grep=%s", keyword))
	}

	countArgs := append([]string{"rev-list", "--count"}, scopeAndFilters...)
	totalStr, err := ExecGit(repoPath, countArgs...)
	if err != nil {
		return nil, fmt.Errorf("读取筛选提交总数失败: %v", err)
	}
	total, err := strconv.Atoi(strings.TrimSpace(totalStr))
	if err != nil {
		return nil, fmt.Errorf("解析筛选提交总数失败: %v", err)
	}

	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	args := append([]string{"log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("-n%d", limit), fmt.Sprintf("--skip=%d", skip), "--topo-order"}, scopeAndFilters...)

	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") || strings.Contains(raw, "does not have any commits yet") {
			return &CommitPaginationResult{Commits: []CommitItem{}, TotalCount: 0, HasMore: false}, nil
		}
		return nil, err
	}

	items := ParseRawGitLog(raw)
	hasMore := (skip + len(items)) < total
	if total < skip+len(items) {
		// Defensive: if rev-list could not be counted, never show a bogus total.
		total = skip + len(items)
	}
	return &CommitPaginationResult{
		Commits:    items,
		TotalCount: total,
		HasMore:    hasMore,
	}, nil
}

// ExportPatch exports a commit as a unified .patch file
func (GitService) ExportPatch(repoPath string, commitHash string, outputPath string) error {
	if commitHash == "" || outputPath == "" {
		return errors.New("commitHash and outputPath are required")
	}
	patchContent, err := ExecGit(repoPath, "format-patch", "-1", commitHash, "--stdout")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(patchContent), 0644)
}

// ApplyPatch applies a .patch file onto the working tree
func (GitService) ApplyPatch(repoPath string, patchPath string) error {
	if patchPath == "" {
		return errors.New("patchPath cannot be empty")
	}
	_, err := ExecGit(repoPath, "apply", "--ignore-whitespace", patchPath)
	return err
}

// PushTag pushes a single tag to a remote repository
func (GitService) PushTag(repoPath string, remote string, tagName string) (string, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	tagName, err := validateTagName(repoPath, tagName)
	if err != nil {
		return "", err
	}
	return ExecGit(repoPath, "push", remote, "--", tagName)
}

// DeleteRemoteTag deletes a tag on the remote repository
func (GitService) DeleteRemoteTag(repoPath string, remote string, tagName string) (string, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	tagName, err := validateTagName(repoPath, tagName)
	if err != nil {
		return "", err
	}
	return ExecGit(repoPath, "push", remote, "--delete", "--", tagName)
}

// RunMaintenance performs prune, remote prune, and git gc
func (s GitService) RunMaintenance(repoPath string) (*RepoMaintenanceStats, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	warnings := []string{}
	remoteStatus := "SKIPPED"
	remoteNote := "仓库没有配置远端，已跳过远端引用清理"
	if remotes, err := ExecGit(repoPath, "remote"); err == nil && strings.TrimSpace(remotes) != "" {
		remote := defaultPushRemote(repoPath)
		if _, err := ExecGit(repoPath, "remote", "prune", remote); err != nil {
			remoteStatus = "FAILED"
			remoteNote = remote
			warnings = append(warnings, fmt.Sprintf("远端引用清理失败(%s): %v", remote, err))
		} else {
			remoteStatus = "OK"
			remoteNote = remote
		}
	}

	gcStatus := "OK"
	if _, err := ExecGit(repoPath, "gc", "--auto"); err != nil {
		gcStatus = "FAILED"
		warnings = append(warnings, "git gc --auto 失败: "+err.Error())
	}
	pruneStatus := "OK"
	if _, err := ExecGit(repoPath, "prune"); err != nil {
		pruneStatus = "FAILED"
		warnings = append(warnings, "git prune 失败: "+err.Error())
	}

	stats, err := s.GetMaintenanceStats(repoPath)
	if err != nil {
		return nil, err
	}
	stats.Success = len(warnings) == 0
	stats.RemotePruneStatus = remoteStatus
	stats.RemotePruneNote = remoteNote
	stats.GcStatus = gcStatus
	stats.PruneStatus = pruneStatus
	stats.Warnings = warnings
	if stats.Success {
		if remoteStatus == "SKIPPED" {
			stats.Summary = "本地 GC / Prune 完成；无远端，已跳过远端引用清理"
		} else {
			stats.Summary = fmt.Sprintf("远端引用 (%s)、GC 与 Prune 全部完成", remoteNote)
		}
	} else {
		stats.Summary = fmt.Sprintf("维护执行完成，但有 %d 个步骤失败", len(warnings))
	}
	return stats, nil
}

// GetMaintenanceStats calculates repository .git storage metrics
func (GitService) GetMaintenanceStats(repoPath string) (*RepoMaintenanceStats, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	gitDir := gitCommonDir(repoPath)
	if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %s", repoPath)
	}
	var totalSize int64
	packCount := 0
	looseCount := 0

	_ = filepath.Walk(gitDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
			if strings.HasSuffix(path, ".pack") {
				packCount++
			}
			if strings.Contains(path, filepath.Join(".git", "objects")) && len(filepath.Base(path)) == 38 {
				looseCount++
			}
		}
		return nil
	})

	sizeHuman := ""
	if totalSize < 1024*1024 {
		sizeHuman = fmt.Sprintf("%d KB", totalSize/1024)
	} else {
		sizeHuman = fmt.Sprintf("%.2f MB", float64(totalSize)/(1024*1024))
	}

	return &RepoMaintenanceStats{
		GitDirSize:     sizeHuman,
		PackFilesCount: packCount,
		LooseObjects:   looseCount,
	}, nil
}

// GetRebaseCommits returns commits on current branch not in upstream
func (GitService) GetRebaseCommits(repoPath string, upstream string) ([]CommitItem, error) {
	if upstream == "" {
		upstream = defaultUpstreamRef(repoPath)
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", upstream+"^{commit}"); err != nil {
		return nil, fmt.Errorf("上游引用 %s 不存在或不可解析: %w", upstream, err)
	}
	rangeArg := fmt.Sprintf("%s..HEAD", upstream)
	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	raw, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), rangeArg)
	if err != nil {
		return nil, fmt.Errorf("读取变基提交失败: %w", err)
	}
	return ParseRawGitLog(raw), nil
}

// GetGitVersion returns the current git CLI version and OS info
func (GitService) GetGitVersion() (*GitVersionInfo, error) {
	out, err := exec.Command("git", "--version").Output()
	ver := "git version unknown"
	if err == nil {
		ver = strings.TrimSpace(string(out))
	}
	return &GitVersionInfo{
		Version: ver,
		System:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}, nil
}

// GetWorkingWordDiff returns git diff with word-level changes
func (GitService) GetWorkingWordDiff(repoPath string, filePath string, staged bool) (string, error) {
	if staged {
		return ExecGit(repoPath, "diff", "--cached", "--word-diff=plain", "--", filePath)
	}
	return ExecGit(repoPath, "diff", "--word-diff=plain", "--", filePath)
}

var remoteTestTimeout = 15 * time.Second

// TestRemoteConnection checks connection to remote repository
func (GitService) TestRemoteConnection(repoPath string, remote string) (*RemoteTestResult, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	start := time.Now()
	out, err := ExecGitWithTimeout(repoPath, remoteTestTimeout, "ls-remote", "--heads", remote)
	latency := fmt.Sprintf("%dms", time.Since(start).Milliseconds())
	if err != nil {
		return &RemoteTestResult{
			Remote:  remote,
			Success: false,
			Latency: latency,
			Error:   err.Error(),
		}, nil
	}
	var heads []string
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			heads = append(heads, strings.TrimPrefix(parts[1], "refs/heads/"))
		}
	}
	return &RemoteTestResult{
		Remote:  remote,
		Success: true,
		Latency: latency,
		Heads:   heads,
	}, nil
}

// RenameRemote renames a remote
func (GitService) RenameRemote(repoPath string, oldName string, newName string) error {
	if oldName == "" || newName == "" {
		return errors.New("remote names cannot be empty")
	}
	_, err := ExecGit(repoPath, "remote", "rename", oldName, newName)
	return err
}

// SetRemoteURL updates the fetch/push URL of a remote
func (GitService) SetRemoteURL(repoPath string, name string, newURL string) error {
	if name == "" || newURL == "" {
		return errors.New("remote name and URL cannot be empty")
	}
	_, err := ExecGit(repoPath, "remote", "set-url", name, newURL)
	return err
}

// GetCommitSignatureDetails extracts GPG / SSH signature verification information
func (GitService) GetCommitSignatureDetails(repoPath string, commitHash string) (*SignatureDetails, error) {
	if commitHash == "" {
		commitHash = "HEAD"
	}
	raw, err := ExecGit(repoPath, "log", "-1", "--format=%G?%x1f%GS%x1f%GK%x1f%GF%x1f%ci", commitHash)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(raw), "\x1f")
	status := "N"
	signer := ""
	keyID := ""
	fp := ""
	date := ""
	if len(parts) > 0 {
		status = parts[0]
	}
	if len(parts) > 1 {
		signer = parts[1]
	}
	if len(parts) > 2 {
		keyID = parts[2]
	}
	if len(parts) > 3 {
		fp = parts[3]
	}
	if len(parts) > 4 {
		date = parts[4]
	}
	if status == "N" {
		// %ci is the commit date, not a signature time; do not present it as one.
		date = ""
	}

	verifyOut, _ := ExecGit(repoPath, "verify-commit", "--verbose", commitHash)

	return &SignatureDetails{
		Status:      status,
		Signer:      signer,
		KeyID:       keyID,
		Fingerprint: fp,
		Date:        date,
		Raw:         verifyOut,
	}, nil
}

// GetRepoStorageBreakdown analyzes working copy file sizes grouped by extension
func (GitService) GetRepoStorageBreakdown(repoPath string) ([]StorageBreakdownItem, error) {
	extMap := make(map[string]*StorageBreakdownItem)

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == ".venv" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "无后缀"
		}
		item, exists := extMap[ext]
		if !exists {
			item = &StorageBreakdownItem{Extension: ext}
			extMap[ext] = item
		}
		item.Count++
		item.SizeBytes += info.Size()
		return nil
	})

	var result []StorageBreakdownItem
	for _, it := range extMap {
		if it.SizeBytes < 1024 {
			it.SizeHuman = fmt.Sprintf("%d B", it.SizeBytes)
		} else if it.SizeBytes < 1024*1024 {
			it.SizeHuman = fmt.Sprintf("%.1f KB", float64(it.SizeBytes)/1024)
		} else {
			it.SizeHuman = fmt.Sprintf("%.2f MB", float64(it.SizeBytes)/(1024*1024))
		}
		result = append(result, *it)
	}

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].SizeBytes < result[j].SizeBytes {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if len(result) > 15 {
		result = result[:15]
	}
	return result, nil
}

// GenerateGitignoreTemplate appends template rules to .gitignore
func (GitService) GenerateGitignoreTemplate(repoPath string, templateType string) error {
	var rules string
	switch strings.ToLower(templateType) {
	case "go":
		rules = "\n# Go\nbin/\n*.exe\n*.exe~\n*.dll\n*.so\n*.dylib\n*.test\n*.out\ngo.work\nvendor/\n"
	case "node":
		rules = "\n# Node\nnode_modules/\nnpm-debug.log*\nyarn-debug.log*\nyarn-error.log*\n.pnpm-debug.log*\n.env\n.env.local\ndist/\nbuild/\n"
	case "unity":
		rules = "\n# Unity Game Engine\n/[Ll]ibrary/\n/[Tt]emp/\n/[Oo]bj/\n/[Bb]uild/\n/[Bb]uilds/\n/[Ll]ogs/\n/[Uu]ser[Ss]ettings/\n/[Mm]emoryCaptures/\n*.csproj\n*.unityproj\n*.sln\n"
	case "unreal":
		rules = "\n# Unreal Game Engine\nBinaries/*\nDerivedDataCache/*\nIntermediate/*\nSaved/*\n*.VC.db\n*.opensdf\n*.opendb\n*.sdf\n*.sln\n*.suo\n*.xcodeproj\n*.xcworkspace\n"
	case "python":
		rules = "\n# Python\n__pycache__/\n*.py[cod]\n*$py.class\n*.so\n.env\n.venv\nenv/\nvenv/\nENV/\nbuild/\ndevelop-eggs/\ndist/\n*.egg-info/\n"
	case "macos":
		rules = "\n# macOS System\n.DS_Store\n.AppleDouble\n.LSOverride\n._*\n.Spotlight-V100\n.Trashes\n"
	default:
		return fmt.Errorf("unknown gitignore template: %s", templateType)
	}

	target := filepath.Join(repoPath, ".gitignore")
	f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(rules)
	return err
}

// ToggleGitHook enables, disables or writes a git hook script
func (GitService) ToggleGitHook(repoPath string, hookName string, enable bool, scriptContent string) error {
	hooksDir := gitPath(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("创建 hooks 目录失败: %w", err)
	}
	hookPath := filepath.Join(hooksDir, hookName)
	if !enable {
		disabledPath := hookPath + ".disabled"
		if _, err := os.Stat(hookPath); err == nil {
			return os.Rename(hookPath, disabledPath)
		}
		return nil
	}
	disabledPath := hookPath + ".disabled"
	if _, err := os.Stat(disabledPath); err == nil {
		if err := os.Rename(disabledPath, hookPath); err != nil {
			return fmt.Errorf("启用钩子失败: %w", err)
		}
	}
	if scriptContent != "" {
		if !strings.HasPrefix(scriptContent, "#!") {
			scriptContent = "#!/bin/sh\n" + scriptContent
		}
		err := os.WriteFile(hookPath, []byte(scriptContent), 0755)
		if err != nil {
			return err
		}
	}
	return os.Chmod(hookPath, 0755)
}

// SaveStashWithFiles stashes only specified files
func (GitService) SaveStashWithFiles(repoPath string, message string, files []string) error {
	if len(files) == 0 {
		return errors.New("no files specified for stash")
	}
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, "--")
	args = append(args, files...)
	_, err := ExecGit(repoPath, args...)
	return err
}

// CheckoutCommitDetached detaches HEAD at the specified commit
func (GitService) CheckoutCommitDetached(repoPath string, commitHash string) error {
	if commitHash == "" {
		return errors.New("commitHash cannot be empty")
	}
	_, err := ExecGit(repoPath, "checkout", "--detach", commitHash)
	return err
}

// GetFileContentAtCommit returns the full text of a file at a specific commit revision
func (GitService) GetFileContentAtCommit(repoPath string, commitHash string, filePath string) (string, error) {
	if commitHash == "" {
		commitHash = "HEAD"
	}
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	return ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", commitHash, cleanPath))
}

// InteractiveRebasePlan returns commits between upstream and HEAD as rebase items
func (GitService) InteractiveRebasePlan(repoPath string, upstream string) ([]RebaseTodoItem, error) {
	if upstream == "" {
		upstream = defaultUpstreamRef(repoPath)
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", upstream+"^{commit}"); err != nil {
		return nil, fmt.Errorf("上游引用 %s 不存在或不可解析: %w", upstream, err)
	}
	format := "%H%x1f%h%x1f%s%x1f%an"
	raw, err := ExecGit(repoPath, "log", "--reverse", fmt.Sprintf("--format=%s", format), fmt.Sprintf("%s..HEAD", upstream))
	if err != nil {
		return nil, fmt.Errorf("读取变基提交失败: %w", err)
	}
	todos := []RebaseTodoItem{}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.Split(line, "\x1f")
		if len(parts) >= 4 {
			todos = append(todos, RebaseTodoItem{
				Action:    "pick",
				Hash:      parts[0],
				ShortHash: parts[1],
				Subject:   parts[2],
				Author:    parts[3],
			})
		}
	}
	return todos, nil
}

// InteractiveRebaseExecute performs visual rebase with sequence editor script
func (GitService) InteractiveRebaseExecute(repoPath string, upstream string, todos []RebaseTodoItem) (string, error) {
	if upstream == "" {
		upstream = defaultUpstreamRef(repoPath)
	}
	if len(todos) == 0 {
		return "没有可变基的提交", nil
	}
	var sb strings.Builder
	for _, it := range todos {
		act := it.Action
		if act == "" {
			act = "pick"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s\n", act, it.ShortHash, it.Subject))
	}

	tmpFile, err := os.CreateTemp("", "rebase-todo-*.sh")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	script := fmt.Sprintf("#!/bin/sh\ncat << 'EOF' > \"$1\"\n%sEOF\n", sb.String())
	if err := os.WriteFile(tmpFile.Name(), []byte(script), 0755); err != nil {
		return "", err
	}
	// WriteFile keeps the existing temp-file mode; GIT_SEQUENCE_EDITOR must be executable.
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rebase", "-i", upstream)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SEQUENCE_EDITOR=%s", tmpFile.Name()))
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	recordGitCommand(repoPath, []string{"rebase", "-i", upstream}, outputStr, errStr, 0, err == nil)
	if err != nil {
		return outputStr, fmt.Errorf("变基执行中断: %s", outputStr)
	}
	return outputStr, nil
}

// SquashCommits squashes the last N commits into one with a custom commit message
func (GitService) SquashCommits(repoPath string, commitCount int, message string) (string, error) {
	if commitCount < 2 {
		return "", errors.New("至少需要合并 2 个提交")
	}
	if strings.TrimSpace(message) == "" {
		return "", errors.New("提交说明不能为空")
	}

	// Reset soft HEAD~N
	_, err := ExecGit(repoPath, "reset", "--soft", fmt.Sprintf("HEAD~%d", commitCount))
	if err != nil {
		return "", fmt.Errorf("软重置失败: %w", err)
	}

	out, err := ExecGit(repoPath, "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("合并提交失败: %w", err)
	}
	return out, nil
}

// CompareBranches compares ahead and behind commits between two branches
func (GitService) CompareBranches(repoPath string, baseBranch string, targetBranch string) (*BranchCompareResult, error) {
	if baseBranch == "" {
		baseBranch = "main"
	}
	if targetBranch == "" {
		targetBranch = "HEAD"
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", baseBranch+"^{commit}"); err != nil {
		return nil, fmt.Errorf("基线分支 %s 不存在或不可解析: %w", baseBranch, err)
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", targetBranch+"^{commit}"); err != nil {
		return nil, fmt.Errorf("目标分支 %s 不存在或不可解析: %w", targetBranch, err)
	}

	res := &BranchCompareResult{
		BaseBranch:    baseBranch,
		TargetBranch:  targetBranch,
		AheadCommits:  []CommitItem{},
		BehindCommits: []CommitItem{},
	}

	// Ahead commits: targetBranch has but baseBranch doesn't
	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	rawAhead, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("%s..%s", baseBranch, targetBranch))
	if err != nil {
		return nil, fmt.Errorf("比较分支失败: %w", err)
	}
	if strings.TrimSpace(rawAhead) != "" {
		res.AheadCommits = ParseRawGitLog(rawAhead)
		res.AheadCount = len(res.AheadCommits)
	}

	// Behind commits: baseBranch has but targetBranch doesn't
	rawBehind, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("%s..%s", targetBranch, baseBranch))
	if err != nil {
		return nil, fmt.Errorf("比较分支失败: %w", err)
	}
	if strings.TrimSpace(rawBehind) != "" {
		res.BehindCommits = ParseRawGitLog(rawBehind)
		res.BehindCount = len(res.BehindCommits)
	}

	return res, nil
}

// CreateAnnotatedTag creates an annotated tag with optional GPG signing
func (GitService) CreateAnnotatedTag(repoPath string, name string, message string, sign bool) error {
	if name == "" {
		return errors.New("标签名不能为空")
	}
	args := []string{"tag", "-a", name, "-m", message}
	if sign {
		args = append(args, "-s")
	}
	_, err := ExecGit(repoPath, args...)
	return err
}

// GetTagDetail returns detailed metadata for an annotated or lightweight tag
func (GitService) GetTagDetail(repoPath string, tagName string) (*TagDetail, error) {
	if tagName == "" {
		return nil, errors.New("标签名不能为空")
	}

	detail := &TagDetail{Name: tagName}
	// Annotated tag refs point at a tag object; dereference to the commit the
	// UI labels as the target.
	hash, err := ExecGit(repoPath, "rev-parse", tagName+"^{commit}")
	if err != nil {
		return nil, err
	}
	detail.CommitHash = strings.TrimSpace(hash)

	// Check if annotated tag object
	catOut, err := ExecGit(repoPath, "cat-file", "-p", fmt.Sprintf("refs/tags/%s", tagName))
	if err == nil && strings.Contains(catOut, "tagger ") {
		detail.IsAnnotated = true
		lines := strings.Split(catOut, "\n")
		var msgLines []string
		inMessage := false
		for _, line := range lines {
			if inMessage {
				msgLines = append(msgLines, line)
			} else if strings.HasPrefix(line, "tagger ") {
				taggerLine := strings.TrimPrefix(line, "tagger ")
				// Tagger format: Name <email> 1729118742 +0800
				if idx := strings.Index(taggerLine, "<"); idx != -1 {
					detail.TaggerName = strings.TrimSpace(taggerLine[:idx])
					rest := taggerLine[idx+1:]
					if endIdx := strings.Index(rest, ">"); endIdx != -1 {
						detail.TaggerEmail = rest[:endIdx]
						rawDate := strings.TrimSpace(rest[endIdx+1:])
						if fields := strings.Fields(rawDate); len(fields) > 0 {
							if sec, parseErr := strconv.ParseInt(fields[0], 10, 64); parseErr == nil {
								detail.Date = time.Unix(sec, 0).Format("2006-01-02 15:04:05 -0700")
							} else {
								detail.Date = rawDate
							}
						} else {
							detail.Date = rawDate
						}
					}
				}
			} else if line == "" {
				inMessage = true
			}
		}
		detail.Message = strings.TrimSpace(strings.Join(msgLines, "\n"))
	} else {
		// Lightweight tag: extract commit message & author
		detail.IsAnnotated = false
		showOut, _ := ExecGit(repoPath, "show", "-s", "--format=%an%x1f%ae%x1f%ci%x1f%s", tagName)
		pts := strings.Split(showOut, "\x1f")
		if len(pts) >= 4 {
			detail.TaggerName = pts[0]
			detail.TaggerEmail = pts[1]
			detail.Date = pts[2]
			detail.Message = pts[3]
		}
	}

	return detail, nil
}

// PruneRemote prunes remote tracking branches that no longer exist on remote
func (GitService) PruneRemote(repoPath string, remoteName string) (string, error) {
	if remoteName == "" {
		remoteName = defaultPushRemote(repoPath)
	}
	return ExecGit(repoPath, "remote", "prune", remoteName)
}

// SyncSubmodule synchronizes submodules and updates recursively
func (GitService) SyncSubmodule(repoPath string, submodulePath string) (string, error) {
	args := []string{"submodule", "sync"}
	if submodulePath != "" {
		args = append(args, "--", submodulePath)
	}
	syncOut, err := ExecGit(repoPath, args...)
	if err != nil {
		return syncOut, err
	}

	updateArgs := []string{"submodule", "update", "--init", "--recursive"}
	if submodulePath != "" {
		updateArgs = append(updateArgs, "--", submodulePath)
	}
	updateOut, err := ExecGit(repoPath, updateArgs...)
	if err != nil {
		return syncOut + "\n" + updateOut, err
	}
	return syncOut + "\n" + updateOut, nil
}

// SetSubmoduleBranch updates tracking branch for a submodule
func (GitService) SetSubmoduleBranch(repoPath string, submodulePath string, branch string) error {
	if submodulePath == "" || branch == "" {
		return errors.New("submodule path and branch are required")
	}
	_, err := ExecGit(repoPath, "config", "-f", ".gitmodules", fmt.Sprintf("submodule.%s.branch", submodulePath), branch)
	return err
}

// GetSparseCheckoutFolders returns the active sparse-checkout directories
func (GitService) GetSparseCheckoutFolders(repoPath string) ([]string, error) {
	out, err := ExecGit(repoPath, "sparse-checkout", "list")
	if err != nil {
		return []string{}, nil
	}
	var folders []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			folders = append(folders, line)
		}
	}
	return folders, nil
}

// SetSparseCheckoutFolders initializes and configures sparse checkout paths
func (GitService) SetSparseCheckoutFolders(repoPath string, folders []string) error {
	if len(folders) == 0 {
		_, err := ExecGit(repoPath, "sparse-checkout", "disable")
		return err
	}
	_, err := ExecGit(repoPath, "sparse-checkout", "init", "--cone")
	if err != nil {
		return err
	}
	args := []string{"sparse-checkout", "set"}
	args = append(args, folders...)
	_, err = ExecGit(repoPath, args...)
	return err
}

// LockWorktree locks a linked worktree to prevent accidental removal
func (GitService) LockWorktree(repoPath string, worktreePath string, reason string) error {
	args := []string{"worktree", "lock"}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	args = append(args, worktreePath)
	_, err := ExecGit(repoPath, args...)
	return err
}

// UnlockWorktree unlocks a linked worktree
func (GitService) UnlockWorktree(repoPath string, worktreePath string) error {
	_, err := ExecGit(repoPath, "worktree", "unlock", worktreePath)
	return err
}

// ExportCommitReport exports formatted markdown or HTML report of commits
func (GitService) ExportCommitReport(repoPath string, since string, until string, format string) (string, error) {
	args := []string{"log", "--pretty=format:%h|%an|%ad|%s", "--date=short"}
	if since != "" {
		args = append(args, fmt.Sprintf("--since=%s", since))
	}
	if until != "" {
		args = append(args, fmt.Sprintf("--until=%s", until))
	}

	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	lines := strings.Split(raw, "\n")
	if format == "html" {
		sb.WriteString("<div class=\"wgit-commit-report\">\n<h2>wgit 提交版本报告</h2>\n<table border=\"1\" style=\"border-collapse:collapse;width:100%\">\n<thead><tr><th>Hash</th><th>作者</th><th>日期</th><th>提交说明</th></tr></thead>\n<tbody>\n")
		for _, line := range lines {
			pts := strings.SplitN(line, "|", 4)
			if len(pts) >= 4 {
				sb.WriteString(fmt.Sprintf("<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
					html.EscapeString(pts[0]), html.EscapeString(pts[1]), html.EscapeString(pts[2]), html.EscapeString(pts[3])))
			}
		}
		sb.WriteString("</tbody></table></div>")
	} else {
		// Markdown
		mdCell := func(value string) string {
			return strings.ReplaceAll(value, "|", "\\|")
		}
		sb.WriteString("# wgit 提交发布报告\n\n| Commit | 作者 | 日期 | 说明 |\n|---|---|---|---|\n")
		for _, line := range lines {
			pts := strings.SplitN(line, "|", 4)
			if len(pts) >= 4 {
				sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", mdCell(pts[0]), mdCell(pts[1]), mdCell(pts[2]), mdCell(pts[3])))
			}
		}
	}
	return sb.String(), nil
}

// ApplyFileFromStash extracts a single file from stash without popping
func (GitService) ApplyFileFromStash(repoPath string, stashIndex string, filePath string) error {
	if stashIndex == "" || filePath == "" {
		return errors.New("stash index and file path are required")
	}
	_, err := ExecGit(repoPath, "checkout", stashIndex, "--", filePath)
	return err
}

// BatchStageFiles stages multiple files at once
func (GitService) BatchStageFiles(repoPath string, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}
	args := []string{"add", "-A", "--"}
	args = append(args, filePaths...)
	_, err := ExecGit(repoPath, args...)
	return err
}

// BatchDiscardFiles discards changes for multiple files
func (GitService) BatchDiscardFiles(repoPath string, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}
	var tracked, untracked []string
	for _, filePath := range filePaths {
		if strings.TrimSpace(filePath) == "" {
			continue
		}
		if _, err := ExecGit(repoPath, "ls-files", "--error-unmatch", "--", filePath); err == nil {
			tracked = append(tracked, filePath)
		} else {
			untracked = append(untracked, filePath)
		}
	}
	if len(tracked) > 0 {
		args := append([]string{"restore", "--"}, tracked...)
		if _, err := ExecGit(repoPath, args...); err != nil {
			return err
		}
	}
	if len(untracked) > 0 {
		args := append([]string{"clean", "-fd", "--"}, untracked...)
		if _, err := ExecGit(repoPath, args...); err != nil {
			return err
		}
	}
	return nil
}

// BatchUnstageFiles removes multiple paths from the index in one call.
func (GitService) BatchUnstageFiles(repoPath string, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}
	args := []string{"restore", "--staged", "--"}
	args = append(args, filePaths...)
	_, err := ExecGit(repoPath, args...)
	return err
}

func formatSignedByteSize(sz int64) string {
	if sz > 0 {
		return "+" + formatByteSize(sz)
	}
	return formatByteSize(sz)
}

func formatByteSize(sz int64) string {
	if sz < 0 {
		return "-" + formatByteSize(-sz)
	}
	if sz < 1024 {
		return fmt.Sprintf("%d B", sz)
	} else if sz < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(sz)/1024.0)
	} else if sz < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(sz)/(1024.0*1024.0))
	}
	return fmt.Sprintf("%.2f GB", float64(sz)/(1024.0*1024.0*1024.0))
}

func detectAssetMime(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".fbx":
		return "model/fbx"
	case ".obj":
		return "model/obj"
	case ".blend":
		return "application/x-blender"
	case ".uasset":
		return "application/x-unreal-asset"
	case ".psd":
		return "image/vnd.adobe.photoshop"
	default:
		return "application/octet-stream"
	}
}

// DeleteBatchTags deletes multiple tags locally and optionally on remote
func (GitService) DeleteBatchTags(repoPath string, tags []string, deleteRemote bool, remote string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	deleted := []string{}
	failures := []string{}
	for _, tag := range tags {
		normalized, err := validateTagName(repoPath, tag)
		if err != nil {
			if strings.TrimSpace(tag) != "" {
				failures = append(failures, tag+": "+err.Error())
			}
			continue
		}
		tag = normalized
		if _, err := ExecGit(repoPath, "tag", "-d", "--", tag); err != nil {
			failures = append(failures, tag+": "+err.Error())
			continue
		}
		deleted = append(deleted, tag)
		if deleteRemote {
			if _, err := ExecGit(repoPath, "push", remote, "--delete", "--", tag); err != nil {
				failures = append(failures, tag+" (远端): "+err.Error())
			}
		}
	}
	if len(failures) > 0 {
		return deleted, fmt.Errorf("部分标签未能删除: %s", strings.Join(failures, "; "))
	}
	return deleted, nil
}

// RenameBranchWithRemote renames a local branch and optionally syncs with remote
func (GitService) RenameBranchWithRemote(repoPath string, oldName string, newName string, updateRemote bool) error {
	if oldName == "" || newName == "" {
		return errors.New("oldName and newName are required")
	}
	_, err := ExecGit(repoPath, "branch", "-m", oldName, newName)
	if err != nil {
		return err
	}
	if updateRemote {
		remote, _ := ExecGit(repoPath, "config", "--get", "branch."+newName+".remote")
		remote = strings.TrimSpace(remote)
		if remote == "" || remote == "." {
			remote = ""
			if remotes, err := ExecGit(repoPath, "remote"); err == nil {
				list := strings.Fields(remotes)
				for _, candidate := range list {
					if candidate == "origin" {
						remote = candidate
						break
					}
				}
				if remote == "" && len(list) > 0 {
					remote = list[0]
				}
			}
		}
		if remote == "" {
			return fmt.Errorf("本地分支已重命名为 %s，但未找到可同步的远程", newName)
		}
		if _, err := ExecGit(repoPath, "push", remote, "-u", newName); err != nil {
			return fmt.Errorf("本地分支已重命名，但推送远端新分支 %s 失败: %w", newName, err)
		}
		defaultRef, _ := ExecGit(repoPath, "symbolic-ref", "--short", "refs/remotes/"+remote+"/HEAD")
		defaultBranch := strings.TrimPrefix(strings.TrimSpace(defaultRef), remote+"/")
		if defaultBranch == oldName {
			// Remotes refuse to delete their default branch; keeping it is the only option.
			return nil
		}
		if _, err := ExecGit(repoPath, "push", remote, "--delete", oldName); err != nil {
			return fmt.Errorf("远端正分支已推送，但删除远端旧分支 %s 失败: %w", oldName, err)
		}
	}
	return nil
}

func defaultUpstreamRef(repoPath string) string {
	if upstream, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		if ref := strings.TrimSpace(upstream); ref != "" {
			return ref
		}
	}
	if branch, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		if name := strings.TrimSpace(branch); name != "" && name != "HEAD" {
			return defaultPushRemote(repoPath) + "/" + name
		}
	}
	return "origin/main"
}

func defaultPushRemote(repoPath string) string {
	if upstream, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err == nil {
		upstream = strings.TrimSpace(upstream)
		if idx := strings.Index(upstream, "/"); idx > 0 {
			return upstream[:idx]
		}
	}
	if remotes, err := ExecGit(repoPath, "remote"); err == nil {
		list := strings.Fields(remotes)
		for _, candidate := range list {
			if candidate == "origin" {
				return candidate
			}
		}
		if len(list) > 0 {
			return list[0]
		}
	}
	return "origin"
}

// SearchRepoCode performs fast full-text code & symbol search with git grep
func (GitService) SearchRepoCode(repoPath string, query string, ignoreCase bool, maxResults int) ([]GrepMatchItem, error) {
	if strings.TrimSpace(query) == "" {
		return []GrepMatchItem{}, nil
	}
	if maxResults <= 0 {
		maxResults = 200
	}
	args := []string{"grep", "-n", "-I"}
	if ignoreCase {
		args = append(args, "-i")
	}
	args = append(args, "--", query)

	raw, err := ExecGit(repoPath, args...)
	if err != nil {
		// git grep returns exit 1 if no matches found
		return []GrepMatchItem{}, nil
	}

	var results []GrepMatchItem
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		// Format: path:line:content
		firstColon := strings.Index(l, ":")
		if firstColon == -1 {
			continue
		}
		filePath := l[:firstColon]
		rest := l[firstColon+1:]
		secondColon := strings.Index(rest, ":")
		if secondColon == -1 {
			continue
		}
		lineNumStr := rest[:secondColon]
		content := rest[secondColon+1:]
		lineNum, _ := strconv.Atoi(lineNumStr)

		results = append(results, GrepMatchItem{
			Path:        filePath,
			LineNumber:  lineNum,
			LineContent: strings.TrimSpace(content),
		})
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

// RevertNoCommit reverts a commit's changes into working tree without auto-committing
func (GitService) RevertNoCommit(repoPath string, commitHash string) (string, error) {
	if commitHash == "" {
		return "", errors.New("commitHash is required")
	}
	return ExecGit(repoPath, "revert", "--no-commit", commitHash)
}

// StashStagedOnly stashes only staged changes while preserving working directory
func (GitService) StashStagedOnly(repoPath string, message string) (string, error) {
	args := []string{"stash", "push", "-S"}
	if message != "" {
		args = append(args, "-m", message)
	}
	return ExecGit(repoPath, args...)
}

// CreateGitBundle creates a portable offline git bundle file
func (GitService) CreateGitBundle(repoPath string, outputPath string, refRange string) (string, error) {
	if outputPath == "" {
		return "", errors.New("outputPath is required")
	}
	if refRange == "" {
		refRange = "--all"
	}
	return ExecGit(repoPath, "bundle", "create", outputPath, refRange)
}

// VerifyGitBundle verifies integrity of a git bundle file
func (GitService) VerifyGitBundle(repoPath string, bundlePath string) (string, error) {
	if bundlePath == "" {
		return "", errors.New("bundlePath is required")
	}
	return ExecGit(repoPath, "bundle", "verify", bundlePath)
}

// ApplyPatchContent applies patch from content string
func (GitService) ApplyPatchContent(repoPath string, patchContent string, checkOnly bool) (string, error) {
	if strings.TrimSpace(patchContent) == "" {
		return "", errors.New("patch content cannot be empty")
	}
	args := []string{"apply", "-v"}
	if checkOnly {
		args = append(args, "--check")
	}
	args = append(args, "-")
	return ExecGitWithStdin(repoPath, patchContent, args...)
}

// GetDetailedGitConfig retrieves local and global git configuration key-values
func (GitService) GetDetailedGitConfig(repoPath string) ([]ConfigEntry, error) {
	raw, err := ExecGit(repoPath, "config", "--list", "--show-scope", "--show-origin")
	hasScope := err == nil
	if !hasScope {
		raw, err = ExecGit(repoPath, "config", "--list", "--show-origin")
	}
	if err != nil {
		raw, err = ExecGit(repoPath, "config", "--list")
		if err != nil {
			return []ConfigEntry{}, err
		}
	}

	var entries []ConfigEntry
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		scope := "local"
		configPart := l
		if hasScope {
			if parts := strings.SplitN(l, "\t", 3); len(parts) >= 3 {
				scope = parts[0]
				configPart = parts[2]
			}
		} else if parts := strings.SplitN(l, "\t", 2); len(parts) >= 2 {
			origin := parts[0]
			configPart = parts[1]
			switch {
			case strings.Contains(origin, "/.git/config"):
				scope = "local"
			case strings.Contains(origin, ".gitconfig"), strings.Contains(origin, "/.config/git/config"):
				scope = "global"
			case strings.Contains(origin, "/etc/gitconfig"):
				scope = "system"
			}
		}

		if idx := strings.Index(configPart, "="); idx != -1 {
			k := configPart[:idx]
			v := configPart[idx+1:]
			entries = append(entries, ConfigEntry{
				Key:   strings.TrimSpace(k),
				Value: strings.TrimSpace(v),
				Scope: scope,
			})
		}
	}
	return entries, nil
}

// SetCustomConfigKey updates any git config key
func (GitService) SetCustomConfigKey(repoPath string, key string, value string, isGlobal bool) error {
	if key == "" {
		return errors.New("config key cannot be empty")
	}
	args := []string{"config"}
	if isGlobal {
		args = append(args, "--global")
	}
	args = append(args, key, value)
	_, err := ExecGit(repoPath, args...)
	return err
}

// DiscardUntrackedFile removes an untracked file or folder
func (GitService) DiscardUntrackedFile(repoPath string, filePath string) error {
	if filePath == "" {
		return errors.New("filePath cannot be empty")
	}
	statusOut, _ := ExecGit(repoPath, "status", "--porcelain", "--", filePath)
	if !strings.HasPrefix(strings.TrimSpace(statusOut), "??") {
		return fmt.Errorf("not an untracked path: %s", filePath)
	}
	_, err := ExecGit(repoPath, "clean", "-fd", "--", filePath)
	return err
}

// CherryPickRange cherry-picks a range of commits in chronological order
func (GitService) CherryPickRange(repoPath string, startHash string, endHash string) (string, error) {
	if startHash == "" || endHash == "" {
		return "", errors.New("startHash and endHash are required")
	}
	return ExecGit(repoPath, "cherry-pick", fmt.Sprintf("%s..%s", startHash, endHash))
}

// VerifyRepoHealth runs git fsck to verify repository database health
func (GitService) VerifyRepoHealth(repoPath string) (*GitFsckResult, error) {
	raw, fsckErr := ExecGit(repoPath, "fsck", "--full")
	res := &GitFsckResult{IsClean: true}
	if fsckErr != nil {
		res.IsClean = false
		res.RawErrors = append(res.RawErrors, fsckErr.Error())
	}
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "dangling blob ") {
			res.DanglingBlob++
		} else if strings.HasPrefix(l, "dangling tree ") {
			res.DanglingTree++
		} else if strings.HasPrefix(l, "dangling commit ") {
			res.DanglingCommit++
		} else if strings.HasPrefix(l, "dangling ") {
			// Tags and other dangling object types are not corruption.
			continue
		} else {
			res.IsClean = false
			res.RawErrors = append(res.RawErrors, l)
		}
	}
	return res, nil
}

// VerifyLfsPointers runs `git lfs fsck` to verify LFS pointer and object integrity
func (GitService) VerifyLfsPointers(repoPath string) (*GitFsckResult, error) {
	raw, fsckErr := ExecGit(repoPath, "lfs", "fsck")
	res := &GitFsckResult{IsClean: true}
	if fsckErr != nil {
		res.IsClean = false
		res.RawErrors = append(res.RawErrors, fsckErr.Error())
	}
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		lower := strings.ToLower(l)
		if strings.Contains(lower, "error") || strings.Contains(lower, "corrupt") || strings.Contains(lower, "missing") {
			res.IsClean = false
			res.RawErrors = append(res.RawErrors, l)
		}
	}
	return res, nil
}

// CheckIgnoreRule checks which .gitignore rule is ignoring a path
func (GitService) CheckIgnoreRule(repoPath string, filePath string) (*CheckIgnoreResult, error) {
	if filePath == "" {
		return nil, errors.New("filePath is required")
	}
	raw, err := ExecGit(repoPath, "check-ignore", "-v", "--", filePath)
	if err != nil {
		// check-ignore exits 1 for "not ignored"; other failures (bad repo,
		// broken git) must not be reported as a clean ignore result.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return &CheckIgnoreResult{
				TargetFile: filePath,
				IsIgnored:  false,
			}, nil
		}
		return nil, fmt.Errorf("检查忽略规则失败: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return &CheckIgnoreResult{
			TargetFile: filePath,
			IsIgnored:  false,
		}, nil
	}
	// Output format: .gitignore:12:*.log  my_debug.log
	res := &CheckIgnoreResult{
		TargetFile: filePath,
		IsIgnored:  true,
	}
	raw = strings.TrimSpace(raw)
	pts := strings.Split(raw, "\t")
	if len(pts) >= 2 {
		rulePart := pts[0]
		rpts := strings.Split(rulePart, ":")
		if len(rpts) >= 3 {
			res.SourceFile = rpts[0]
			res.LineNumber, _ = strconv.Atoi(rpts[1])
			res.Pattern = rpts[2]
		}
	}
	return res, nil
}

// GetCommitTopologyStats returns statistics on merge vs direct commits
func (GitService) GetCommitTopologyStats(repoPath string) (*CommitTopologyStats, error) {
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %v", err)
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); err != nil {
		return &CommitTopologyStats{}, nil
	}

	totalStr, err := ExecGit(repoPath, "rev-list", "--count", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("读取提交总数失败: %v", err)
	}
	total, err := strconv.Atoi(strings.TrimSpace(totalStr))
	if err != nil {
		return nil, fmt.Errorf("解析提交总数失败: %v", err)
	}

	mergeStr, err := ExecGit(repoPath, "rev-list", "--merges", "--count", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("读取合并提交数失败: %v", err)
	}
	merges, err := strconv.Atoi(strings.TrimSpace(mergeStr))
	if err != nil {
		return nil, fmt.Errorf("解析合并提交数失败: %v", err)
	}

	direct := total - merges
	ratio := 0.0
	if total > 0 {
		ratio = float64(merges) / float64(total)
	}

	return &CommitTopologyStats{
		TotalCommits:  total,
		MergeCommits:  merges,
		DirectCommits: direct,
		MergeRatio:    ratio,
	}, nil
}

// GetImageBase64 retrieves an image's base64 data URI at a commit or current worktree
func (GitService) GetImageBase64(repoPath string, commitHash string, filePath string) (string, error) {
	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return "", err
	}
	filePath = cleanPath

	ext := strings.ToLower(filepath.Ext(filePath))
	mime := detectAssetMime(ext)
	if !strings.HasPrefix(mime, "image/") {
		return "", errors.New("not an image file")
	}

	var data []byte
	if commitHash == "" || commitHash == "WORKING" {
		fullPath := filepath.Join(repoPath, filePath)
		data, err = os.ReadFile(fullPath)
	} else {
		cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commitHash, filePath))
		cmd.Dir = repoPath
		data, err = cmd.Output()
	}
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

// StashClear drops all stashed changes
func (GitService) StashClear(repoPath string) error {
	_, err := ExecGit(repoPath, "stash", "clear")
	return err
}

// GetGitAttributes reads current .gitattributes content
func (GitService) GetGitAttributes(repoPath string) (string, error) {
	attrPath := filepath.Join(repoPath, ".gitattributes")
	data, err := os.ReadFile(attrPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SaveGitAttributes writes .gitattributes content
func (GitService) SaveGitAttributes(repoPath string, content string) error {
	attrPath := filepath.Join(repoPath, ".gitattributes")
	return os.WriteFile(attrPath, []byte(content), 0644)
}

// UpdateSubmodulesRecursive updates all submodules recursively with init
func (GitService) UpdateSubmodulesRecursive(repoPath string) (string, error) {
	return ExecGit(repoPath, "submodule", "update", "--init", "--recursive")
}

// UpdateSubmoduleRemote fetches and fast-forwards submodule to its tracked remote branch
func (GitService) UpdateSubmoduleRemote(repoPath string, submodulePath string) (string, error) {
	args := []string{"submodule", "update", "--remote"}
	if submodulePath != "" {
		args = append(args, "--", submodulePath)
	}
	return ExecGit(repoPath, args...)
}

// GetGitProxy reads configured http.proxy and https.proxy (local or global)
func (GitService) GetGitProxy(repoPath string) (*GitProxyConfig, error) {
	localHTTP, _ := ExecGit(repoPath, "config", "--local", "--get", "http.proxy")
	localHTTPS, _ := ExecGit(repoPath, "config", "--local", "--get", "https.proxy")
	globalHTTP, _ := ExecGit(repoPath, "config", "--global", "--get", "http.proxy")
	globalHTTPS, _ := ExecGit(repoPath, "config", "--global", "--get", "https.proxy")

	httpProxy, httpsProxy := localHTTP, localHTTPS
	isGlobal := false
	if httpProxy == "" && httpsProxy == "" {
		httpProxy, httpsProxy = globalHTTP, globalHTTPS
		isGlobal = httpProxy != "" || httpsProxy != ""
	} else {
		if httpProxy == "" {
			httpProxy = globalHTTP
		}
		if httpsProxy == "" {
			httpsProxy = globalHTTPS
		}
	}

	return &GitProxyConfig{
		HttpProxy:  strings.TrimSpace(httpProxy),
		HttpsProxy: strings.TrimSpace(httpsProxy),
		IsGlobal:   isGlobal,
	}, nil
}

// SetGitProxy configures http.proxy and https.proxy
func (GitService) SetGitProxy(repoPath string, proxyURL string, isGlobal bool) error {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return errors.New("proxy URL cannot be empty")
	}

	argsHttp := []string{"config"}
	argsHttps := []string{"config"}
	if isGlobal {
		argsHttp = append(argsHttp, "--global")
		argsHttps = append(argsHttps, "--global")
	}
	argsHttp = append(argsHttp, "http.proxy", proxyURL)
	argsHttps = append(argsHttps, "https.proxy", proxyURL)

	if _, err := ExecGit(repoPath, argsHttp...); err != nil {
		return err
	}
	_, err := ExecGit(repoPath, argsHttps...)
	return err
}

func clearGitConfigKey(repoPath string, key string, isGlobal bool) error {
	unsetArgs := []string{"config"}
	getArgs := []string{"config"}
	if isGlobal {
		unsetArgs = append(unsetArgs, "--global")
		getArgs = append(getArgs, "--global")
	}
	if _, err := ExecGit(repoPath, append(unsetArgs, "--unset-all", key)...); err != nil {
		// Unsetting an absent key is fine; a value that remains is not.
		if current, getErr := ExecGit(repoPath, append(getArgs, "--get-all", key)...); getErr == nil && strings.TrimSpace(current) != "" {
			return fmt.Errorf("清除 %s 失败: %w", key, err)
		}
	}
	return nil
}

// ClearGitProxy removes http.proxy and https.proxy
func (GitService) ClearGitProxy(repoPath string, isGlobal bool) error {
	if err := clearGitConfigKey(repoPath, "http.proxy", isGlobal); err != nil {
		return err
	}
	return clearGitConfigKey(repoPath, "https.proxy", isGlobal)
}

// CloneRepoAdvanced clones a repository with optimizations like blobless (--filter=blob:none), shallow depth, and submodule recursion
func (s GitService) CloneRepoAdvanced(opts CloneOptions) (*RepoInfo, error) {
	if opts.URL == "" || opts.TargetPath == "" {
		return nil, errors.New("URL and target directory are required")
	}
	targetPath := filepath.Clean(opts.TargetPath)
	parentDir := filepath.Dir(targetPath)
	repoName := filepath.Base(targetPath)

	args := []string{"clone"}
	if opts.CloneType == "blobless" {
		args = append(args, "--filter=blob:none")
	} else if opts.CloneType == "shallow" {
		depth := opts.Depth
		if depth <= 0 {
			depth = 1
		}
		args = append(args, fmt.Sprintf("--depth=%d", depth))
	}
	if opts.Branch != "" {
		args = append(args, "-b", opts.Branch)
	}
	if opts.RecurseSubmodules {
		args = append(args, "--recurse-submodules")
	}
	args = append(args, opts.URL, repoName)

	if _, err := ExecGit(parentDir, args...); err != nil {
		return nil, fmt.Errorf("failed to clone: %w", err)
	}

	return s.OpenRepo(targetPath)
}

// protectedBranchName reports whether name is a long-lived branch that branch
// cleanup must never list as removable or delete, even when fully merged.
func protectedBranchName(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "refs/heads/")
	name = strings.ToLower(name)
	switch name {
	case "main", "master", "release", "develop", "trunk", "head":
		return true
	}
	for _, prefix := range []string{"release/", "hotfix/", "develop/", "support/"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// GetMergedBranches returns local branches that have been fully merged into targetBranch
func (GitService) GetMergedBranches(repoPath string, targetBranch string) ([]string, error) {
	if targetBranch == "" {
		targetBranch = "HEAD"
	}
	out, err := ExecGit(repoPath, "branch", "--merged", targetBranch)
	if err != nil {
		return []string{}, err
	}

	lines := strings.Split(out, "\n")
	merged := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "*") {
			// Skip current active branch or empty
			continue
		}
		name := strings.TrimPrefix(l, "+ ")
		name = strings.TrimSpace(name)
		if name == "" || protectedBranchName(name) || name == targetBranch {
			continue
		}
		merged = append(merged, name)
	}
	return merged, nil
}

// DeleteMergedBranches batch deletes safe merged branches
func (GitService) DeleteMergedBranches(repoPath string, branches []string) ([]string, error) {
	deleted := []string{}
	for _, b := range branches {
		b = strings.TrimSpace(b)
		if b == "" || protectedBranchName(b) || strings.HasPrefix(b, "-") {
			continue
		}
		if _, err := ExecGit(repoPath, "check-ref-format", "--branch", b); err != nil {
			continue
		}
		if _, err := ExecGit(repoPath, "branch", "-d", b); err == nil {
			deleted = append(deleted, b)
		}
	}
	return deleted, nil
}

func redactSecretText(line string, start int, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(line) {
		end = len(line)
	}
	if start < end {
		redacted := "****"
		if end-start > 8 {
			redacted = line[start:start+4] + "****" + line[end-4:end]
		}
		line = line[:start] + redacted + line[end:]
	}
	line = strings.TrimSpace(line)
	if len(line) > 120 {
		line = line[:120] + "..."
	}
	return line
}

// CherryPickMultiple applies multiple commits in sequence (chronological order)
func (s GitService) CherryPickMultiple(repoPath string, commitHashes []string) (*BatchCherryPickResult, error) {
	if len(commitHashes) == 0 {
		return nil, errors.New("commit hashes list cannot be empty")
	}

	status, err := s.GetRepoStatus(repoPath)
	if err == nil && !status.IsClean {
		return nil, errors.New("working tree is dirty; please commit or stash changes before batch cherry-picking")
	}

	res := &BatchCherryPickResult{
		TotalCount:    len(commitHashes),
		AppliedHashes: []string{},
	}

	for _, hash := range commitHashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}

		out, err := ExecGit(repoPath, "cherry-pick", hash)
		if err != nil {
			curStatus, _ := s.GetRepoStatus(repoPath)
			if len(curStatus.ConflictedFiles) > 0 {
				res.HasConflict = true
			}
			res.FailedCommitHash = hash
			res.ErrorMessage = strings.TrimSpace(out + " " + err.Error())
			return res, nil
		}

		res.SuccessCount++
		res.AppliedHashes = append(res.AppliedHashes, hash)
	}

	return res, nil
}

// GetCommitDetailsBatch returns commit details for a list of commit hashes
func (GitService) GetCommitDetailsBatch(repoPath string, commitHashes []string) ([]CommitItem, error) {
	var items []CommitItem
	for _, hash := range commitHashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		raw, err := ExecGit(repoPath, "show", "-s", "--format=%H%x1f%h%x1f%an%x1f%ae%x1f%ad%x1f%ar%x1f%s%x1f%b", "--date=iso", hash)
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimSpace(raw), "\x1f")
		if len(parts) >= 7 {
			body := ""
			if len(parts) > 7 {
				body = parts[7]
			}
			items = append(items, CommitItem{
				Hash:         parts[0],
				ShortHash:    parts[1],
				AuthorName:   parts[2],
				AuthorEmail:  parts[3],
				Date:         parts[4],
				RelativeDate: parts[5],
				Subject:      parts[6],
				Body:         body,
			})
		}
	}
	return items, nil
}

// ScanHistoryLargeBlobs scans Git object database for the largest historical blobs across all revisions
func (GitService) ScanHistoryLargeBlobs(repoPath string, limit int) ([]HistoryLargeBlob, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	cmd1 := exec.Command("git", "rev-list", "--objects", "--all")
	cmd1.Dir = repoPath

	cmd2 := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize) %(rest)")
	cmd2.Dir = repoPath

	pipe, err := cmd1.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd2.Stdin = pipe

	var outBuf bytes.Buffer
	cmd2.Stdout = &outBuf

	if err := cmd1.Start(); err != nil {
		return nil, err
	}
	if err := cmd2.Start(); err != nil {
		return nil, err
	}

	if err := cmd1.Wait(); err != nil {
		return nil, fmt.Errorf("枚举历史对象失败: %w", err)
	}
	if err := cmd2.Wait(); err != nil {
		return nil, fmt.Errorf("读取历史对象大小失败: %w", err)
	}

	type rawBlob struct {
		oid  string
		size int64
		path string
	}

	var blobs []rawBlob
	scanner := bufio.NewScanner(&outBuf)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 4)
		if len(parts) >= 3 && parts[1] == "blob" {
			size, err := strconv.ParseInt(parts[2], 10, 64)
			if err == nil {
				path := ""
				if len(parts) == 4 {
					path = strings.TrimSpace(parts[3])
				}
				blobs = append(blobs, rawBlob{
					oid:  parts[0],
					size: size,
					path: path,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("解析历史对象列表失败: %w", err)
	}

	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].size > blobs[j].size
	})

	if len(blobs) > limit {
		blobs = blobs[:limit]
	}

	var results = make([]HistoryLargeBlob, 0, len(blobs))
	for _, b := range blobs {
		commitHash := ""
		commitDate := ""
		author := ""
		subject := ""

		if b.path != "" {
			logOut, err := ExecGit(repoPath, "log", "-1", "--format=%h\x1f%ad\x1f%an\x1f%s", "--date=short", "--all", "--", b.path)
			if err == nil && logOut != "" {
				lParts := strings.Split(strings.TrimSpace(logOut), "\x1f")
				if len(lParts) >= 4 {
					commitHash = lParts[0]
					commitDate = lParts[1]
					author = lParts[2]
					subject = lParts[3]
				}
			}
		}

		results = append(results, HistoryLargeBlob{
			OID:        b.oid,
			Size:       b.size,
			SizeStr:    formatByteSize(b.size),
			Path:       b.path,
			CommitHash: commitHash,
			CommitDate: commitDate,
			Author:     author,
			Subject:    subject,
		})
	}

	return results, nil
}

// ScanEmptyDirectories scans for empty folders in the working tree (excluding ignore dirs)
func (GitService) ScanEmptyDirectories(repoPath string) ([]string, error) {
	emptyDirs := []string{}
	ignoredDirs := map[string]bool{
		".git": true, "node_modules": true, "dist": true, "build": true,
		"Library": true, "Temp": true, "Obj": true, "Build": true,
		"Builds": true, "Logs": true, "UserSettings": true, "MemoryCaptures": true,
		"Intermediate": true, "Saved": true, "Binaries": true,
		"DerivedDataCache": true, ".vs": true, ".idea": true,
	}

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		parts := strings.Split(filepath.ToSlash(rel), "/")
		for _, p := range parts {
			if ignoredDirs[p] {
				return filepath.SkipDir
			}
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			emptyDirs = append(emptyDirs, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描空目录失败: %w", err)
	}
	return emptyDirs, nil
}

// GenerateGitkeeps creates .gitkeep in specified directories and stages them
func (GitService) GenerateGitkeeps(repoPath string, directories []string) (int, error) {
	if repoPath == "" {
		return 0, errors.New("repoPath 不能为空")
	}
	count := 0
	failures := []string{}
	for _, dir := range directories {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(dir))
		if filepath.IsAbs(clean) {
			failures = append(failures, dir+": 必须是仓库内相对路径")
			continue
		}
		targetDir := filepath.Join(repoPath, clean)
		relDir, err := filepath.Rel(repoPath, targetDir)
		if err != nil || relDir == ".." || strings.HasPrefix(relDir, ".."+string(filepath.Separator)) {
			failures = append(failures, dir+": 目录越出仓库")
			continue
		}
		info, err := os.Stat(targetDir)
		if err != nil || !info.IsDir() {
			continue
		}
		// A directory symlink can still point outside the repository even when
		// the textual path looks contained.
		repoReal := repoPath
		if resolved, resolveErr := filepath.EvalSymlinks(repoPath); resolveErr == nil {
			repoReal = resolved
		}
		targetReal := targetDir
		if resolved, resolveErr := filepath.EvalSymlinks(targetDir); resolveErr == nil {
			targetReal = resolved
		}
		realRel, relErr := filepath.Rel(repoReal, targetReal)
		if relErr != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
			failures = append(failures, dir+": 目录链接指向仓库外")
			continue
		}
		keepPath := filepath.Join(targetDir, ".gitkeep")
		if err := os.WriteFile(keepPath, []byte("# Keep this directory tracked in git\n"), 0o644); err != nil {
			failures = append(failures, dir+": "+err.Error())
			continue
		}
		relKeep := filepath.ToSlash(filepath.Join(relDir, ".gitkeep"))
		if _, err := ExecGit(repoPath, "add", "--", relKeep); err != nil {
			failures = append(failures, relKeep+": "+err.Error())
			continue
		}
		count++
	}
	if len(failures) > 0 {
		return count, fmt.Errorf("部分 .gitkeep 未能暂存: %s", strings.Join(failures, "; "))
	}
	return count, nil
}

// RevealFileInFinder reveals a file or folder in macOS Finder, Windows Explorer, or Linux File Manager
func (GitService) RevealFileInFinder(repoPath string, filePath string) error {
	target := filePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(repoPath, filePath)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", target)
	case "windows":
		cmd = exec.Command("explorer", "/select,", target)
	default:
		dir := target
		if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
			dir = filepath.Dir(target)
		}
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

// OpenFileWithDefaultApp opens a file using the operating system's default registered application
func (GitService) OpenFileWithDefaultApp(repoPath string, filePath string) error {
	target := filePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(repoPath, filePath)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// GetCommitTemplate fetches configured commit.template or project .gitmessage
func (GitService) GetCommitTemplate(repoPath string) (*CommitTemplateInfo, error) {
	result := &CommitTemplateInfo{}
	out, err := ExecGit(repoPath, "config", "--get", "commit.template")
	configured := strings.TrimSpace(out)
	if err == nil && configured != "" {
		result.ConfiguredPath = configured
		targetPath := configured
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(repoPath, targetPath)
		}
		if data, err := os.ReadFile(targetPath); err == nil {
			result.Content = string(data)
			result.HasTemplate = true
			return result, nil
		}
	}

	// Fallback to repo root .gitmessage or .gitmessage.txt
	for _, candidate := range []string{".gitmessage", ".gitmessage.txt"} {
		p := filepath.Join(repoPath, candidate)
		if data, err := os.ReadFile(p); err == nil {
			result.ConfiguredPath = candidate
			result.Content = string(data)
			result.HasTemplate = true
			return result, nil
		}
	}

	return result, nil
}

// SaveCommitTemplate saves repository commit template file (.gitmessage) and optionally configures git config
func (GitService) SaveCommitTemplate(repoPath string, content string, setGitConfig bool) error {
	p := filepath.Join(repoPath, ".gitmessage")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		return err
	}
	if setGitConfig {
		_, err := ExecGit(repoPath, "config", "commit.template", ".gitmessage")
		if err != nil {
			return err
		}
	}
	return nil
}

func defaultCustomActions() []CustomActionConfig {
	return []CustomActionConfig{
		{
			ID:            "act_term",
			Name:          "打开终端 (Terminal)",
			Description:   "在当前工作区路径快速打开系统终端命令行",
			Command:       "open",
			Args:          "-a Terminal .",
			WorkingDir:    "",
			Icon:          "💻",
			ShowInToolbar: true,
		},
		{
			ID:            "act_vscode",
			Name:          "VS Code 打开项目",
			Description:   "使用 Visual Studio Code 打开当前项目目录",
			Command:       "code",
			Args:          ".",
			WorkingDir:    "",
			Icon:          "📝",
			ShowInToolbar: true,
		},
		{
			ID:            "act_lfs_fsck",
			Name:          "LFS 资产完整性核查",
			Description:   "执行 git lfs fsck 校验本地所有大文件指针与资产有效性",
			Command:       "git",
			Args:          "lfs fsck",
			WorkingDir:    "",
			Icon:          "📦",
			ShowInToolbar: true,
		},
		{
			ID:            "act_diff_check",
			Name:          "空白与冲突标记巡检",
			Description:   "执行 git diff --check 快速发现意外遗留的合并冲突标记或无效空白",
			Command:       "git",
			Args:          "diff --check",
			WorkingDir:    "",
			Icon:          "🔍",
			ShowInToolbar: false,
		},
		{
			ID:            "act_cmake",
			Name:          "CMake 自动化构建配置",
			Description:   "在 build 目录生成 CMake 工程构建文件",
			Command:       "cmake",
			Args:          "-B build",
			WorkingDir:    "",
			Icon:          "⚙️",
			ShowInToolbar: false,
		},
	}
}

// ExecuteCustomAction executes a pipeline action script in repository path
func (s GitService) ExecuteCustomAction(repoPath string, actionID string) (*CustomActionResult, error) {
	actions, err := s.GetCustomActions(repoPath)
	if err != nil {
		return nil, err
	}

	var target *CustomActionConfig
	for _, a := range actions {
		if a.ID == actionID {
			target = &a
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("找不到自定义动作: %s", actionID)
	}

	workDir := repoPath
	if target.WorkingDir != "" {
		if filepath.IsAbs(target.WorkingDir) {
			workDir = target.WorkingDir
		} else {
			workDir = filepath.Join(repoPath, target.WorkingDir)
		}
	}

	cmdStr := target.Command
	if target.Args != "" {
		cmdStr += " " + target.Args
	}

	startTime := time.Now()
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	runErr := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	exitCode := 0
	success := true
	if runErr != nil {
		success = false
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &CustomActionResult{
		ActionID:   target.ID,
		Name:       target.Name,
		Command:    cmdStr,
		Output:     outBuf.String(),
		ExitCode:   exitCode,
		DurationMs: duration,
		Success:    success,
	}, nil
}

// GetRepositoryDiskBreakdown provides a comprehensive breakdown of repository storage, git objects, LFS, and working tree
func (GitService) GetRepositoryDiskBreakdown(repoPath string) (*DiskBreakdownResult, error) {
	result := &DiskBreakdownResult{
		TopFolders:      []DiskFolderItem{},
		AssetCategories: []StorageBreakdownItem{},
		Recommendations: []string{},
	}

	var gitInternalSize int64
	var lfsStoreSize int64
	var workingTreeSize int64

	// 1. Walk .git directory
	gitDir := gitCommonDir(repoPath)
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		_ = filepath.Walk(gitDir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(gitDir, p)
			if strings.HasPrefix(rel, "lfs") {
				lfsStoreSize += info.Size()
			} else {
				gitInternalSize += info.Size()
			}
			return nil
		})
	}

	// 2. Measure top root folders & file categories in working tree
	folderSizes := make(map[string]int64)
	catSizes := make(map[string]int64)
	catCounts := make(map[string]int)

	entries, err := os.ReadDir(repoPath)
	if err == nil {
		for _, e := range entries {
			if e.Name() == ".git" {
				continue
			}
			subPath := filepath.Join(repoPath, e.Name())
			if e.IsDir() {
				var subSize int64
				_ = filepath.Walk(subPath, func(p string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					sz := info.Size()
					subSize += sz
					workingTreeSize += sz

					ext := strings.ToLower(filepath.Ext(p))
					cat := classifyAssetCategory(ext)
					catSizes[cat] += sz
					catCounts[cat]++
					return nil
				})
				folderSizes[e.Name()] = subSize
			} else {
				if fi, err := e.Info(); err == nil {
					sz := fi.Size()
					workingTreeSize += sz
					ext := strings.ToLower(filepath.Ext(e.Name()))
					cat := classifyAssetCategory(ext)
					catSizes[cat] += sz
					catCounts[cat]++
				}
			}
		}
	}

	// Sort top folders
	type fItem struct {
		name string
		size int64
	}
	var fList []fItem
	for k, v := range folderSizes {
		fList = append(fList, fItem{name: k, size: v})
	}
	sort.Slice(fList, func(i, j int) bool {
		return fList[i].size > fList[j].size
	})

	for i, f := range fList {
		if i >= 6 {
			break
		}
		result.TopFolders = append(result.TopFolders, DiskFolderItem{
			Path:    f.name,
			Size:    f.size,
			SizeStr: formatByteSize(f.size),
		})
	}

	// Asset categories breakdown
	var catList []StorageBreakdownItem
	for cat, sz := range catSizes {
		catList = append(catList, StorageBreakdownItem{
			Extension: cat,
			Count:     catCounts[cat],
			SizeBytes: sz,
			SizeHuman: formatByteSize(sz),
		})
	}
	sort.Slice(catList, func(i, j int) bool {
		return catList[i].SizeBytes > catList[j].SizeBytes
	})
	result.AssetCategories = catList

	total := gitInternalSize + lfsStoreSize + workingTreeSize
	result.TotalSize = total
	result.TotalSizeStr = formatByteSize(total)
	result.WorkingTreeSize = workingTreeSize
	result.WorkingTreeStr = formatByteSize(workingTreeSize)
	result.GitInternalSize = gitInternalSize
	result.GitInternalStr = formatByteSize(gitInternalSize)
	result.LfsStoreSize = lfsStoreSize
	result.LfsStoreStr = formatByteSize(lfsStoreSize)

	// Engine caches
	var engineCacheTotal int64
	workflows, _ := GitService{}.DetectGameEngineWorkflows(repoPath)
	for _, wf := range workflows {
		engineCacheTotal += wf.CacheSizeBytes
	}
	result.EngineCacheSize = engineCacheTotal
	result.EngineCacheStr = formatByteSize(engineCacheTotal)

	// Recommendations
	if lfsStoreSize > 500*1024*1024 {
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("LFS 本地缓存占用较多 (%s)，建议运行 LFS Prune 修剪历史版本资产", formatByteSize(lfsStoreSize)))
	}
	if engineCacheTotal > 300*1024*1024 {
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("游戏引擎中间生成文件占用 %s，可在引擎工作流中一键安全清除", formatByteSize(engineCacheTotal)))
	}
	if gitInternalSize > 200*1024*1024 {
		result.Recommendations = append(result.Recommendations, "Git 版本库内部对象较多，建议运行 Git GC 执行松散对象打包与存储整理")
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = append(result.Recommendations, "当前代码仓库存储健康，未发现异常膨胀或大量未压缩资产。")
	}

	return result, nil
}

func classifyAssetCategory(ext string) string {
	switch ext {
	case ".fbx", ".obj", ".blend", ".max", ".ma", ".mb", ".3ds", ".dae":
		return "3D 模型与工程资产"
	case ".png", ".jpg", ".jpeg", ".tga", ".psd", ".exr", ".hdr", ".dds", ".bmp", ".tif", ".tiff", ".svg":
		return "纹理贴图与图像美术"
	case ".wav", ".mp3", ".ogg", ".flac", ".aiff", ".m4a":
		return "音频音效与音乐"
	case ".mp4", ".mov", ".avi", ".webm":
		return "动画与视频剪辑"
	case ".uasset", ".umap", ".unity", ".prefab", ".mat", ".asset", ".anim", ".controller":
		return "游戏引擎预制体与资产"
	case ".cpp", ".c", ".h", ".hpp", ".cs", ".ts", ".js", ".go", ".rs", ".lua", ".py", ".sh":
		return "源代码与核心逻辑"
	case ".json", ".yaml", ".yml", ".xml", ".csv", ".ini", ".txt", ".md":
		return "游戏数值配置与数据表"
	case ".dll", ".dylib", ".so", ".a", ".lib", ".exe", ".bin":
		return "二进制插件与依赖库"
	default:
		if ext == "" {
			return "无后缀其他文件"
		}
		return ext + " 格式文件"
	}
}

// GetMultiRemoteStatus inspects ahead/behind and connection URLs across all configured git remotes
func (GitService) GetMultiRemoteStatus(repoPath string) (*MultiRemoteStatusResult, error) {
	currentBranch, err := ExecGit(repoPath, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	currentBranch = strings.TrimSpace(currentBranch)

	out, err := ExecGit(repoPath, "remote", "-v")
	if err != nil {
		return nil, err
	}

	remoteMap := make(map[string]*RemoteSyncStatusItem)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			name := fields[0]
			url := fields[1]
			kind := strings.Trim(fields[2], "()")

			item, exists := remoteMap[name]
			if !exists {
				item = &RemoteSyncStatusItem{
					Name:        name,
					Branch:      currentBranch,
					LastFetched: "未拉取",
				}
				remoteMap[name] = item
			}
			if kind == "fetch" {
				item.FetchURL = url
			} else if kind == "push" {
				item.PushURL = url
			}
		}
	}

	var list []RemoteSyncStatusItem = make([]RemoteSyncStatusItem, 0, len(remoteMap))
	for _, item := range remoteMap {
		remoteRef := fmt.Sprintf("refs/remotes/%s/%s", item.Name, currentBranch)
		if _, err := ExecGit(repoPath, "rev-parse", "--verify", remoteRef); err == nil {
			aheadStr, aheadErr := ExecGit(repoPath, "rev-list", "--count", fmt.Sprintf("%s/%s..%s", item.Name, currentBranch, currentBranch))
			behindStr, behindErr := ExecGit(repoPath, "rev-list", "--count", fmt.Sprintf("%s..%s/%s", currentBranch, item.Name, currentBranch))
			if aheadErr != nil || behindErr != nil {
				readErr := aheadErr
				if readErr == nil {
					readErr = behindErr
				}
				item.StatusWarning = "读取领先/落后失败: " + readErr.Error()
				item.LastFetched = "领先/落后未知"
			} else {
				ahead, aheadParseErr := strconv.Atoi(strings.TrimSpace(aheadStr))
				behind, behindParseErr := strconv.Atoi(strings.TrimSpace(behindStr))
				if aheadParseErr != nil || behindParseErr != nil {
					parseErr := aheadParseErr
					if parseErr == nil {
						parseErr = behindParseErr
					}
					item.StatusWarning = "解析领先/落后失败: " + parseErr.Error()
					item.LastFetched = "领先/落后未知"
				} else {
					item.Ahead = ahead
					item.Behind = behind
					item.LastFetched = "已跟踪"
				}
			}
		} else {
			item.Ahead = 0
			item.Behind = 0
			item.LastFetched = "远端无对应同名分支"
		}
		list = append(list, *item)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return &MultiRemoteStatusResult{
		CurrentBranch: currentBranch,
		Remotes:       list,
	}, nil
}

// SyncMultiRemotes performs batch fetch or batch push across all remotes
func (GitService) SyncMultiRemotes(repoPath string, action string) (*MultiRemoteSyncResult, error) {
	currentBranch, _ := ExecGit(repoPath, "branch", "--show-current")
	currentBranch = strings.TrimSpace(currentBranch)

	result := &MultiRemoteSyncResult{
		Action:   action,
		Success:  true,
		Messages: []string{},
	}

	if action == "fetch-all" {
		out, err := ExecGit(repoPath, "fetch", "--all", "--prune")
		if err != nil {
			result.Success = false
			result.Messages = append(result.Messages, fmt.Sprintf("多远端拉取失败: %v", err))
		} else {
			result.Messages = append(result.Messages, "已成功从所有远端 (Fetch All) 获取最新分支与标签索引。")
			if strings.TrimSpace(out) != "" {
				result.Messages = append(result.Messages, out)
			}
		}
		return result, nil
	}

	if action == "push-all" {
		if currentBranch == "" {
			return nil, errors.New("当前处于分离 HEAD 或无法确定分支，无法批量推送")
		}
		remotesOut, err := ExecGit(repoPath, "remote")
		if err != nil {
			return nil, err
		}
		remotes := strings.Fields(remotesOut)
		if len(remotes) == 0 {
			return nil, errors.New("当前仓库未配置任何远端仓库 (Remote)")
		}

		for _, r := range remotes {
			out, err := ExecGit(repoPath, "push", r, currentBranch)
			if err != nil {
				result.Success = false
				result.Messages = append(result.Messages, fmt.Sprintf("❌ 推送到远端 [%s] 失败: %v\n%s", r, err, out))
			} else {
				result.Messages = append(result.Messages, fmt.Sprintf("✅ 成功推送到远端 [%s] 分支 %s", r, currentBranch))
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("不支持的多远端同步动作: %s", action)
}

func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// ListSubtrees discovers git-subtree components configured in repository history
type subtreeMetadata struct {
	RemoteOrURL string `json:"remoteOrUrl"`
	Branch      string `json:"branch"`
	Squash      bool   `json:"squash"`
}

func subtreeMetadataPath(repoPath string) string {
	return filepath.Join(gitCommonDir(repoPath), "wgit-subtrees.json")
}

func readSubtreeMetadata(repoPath string) (map[string]subtreeMetadata, error) {
	metas := make(map[string]subtreeMetadata)
	data, err := os.ReadFile(subtreeMetadataPath(repoPath))
	if err != nil {
		if os.IsNotExist(err) {
			return metas, nil
		}
		return metas, fmt.Errorf("读取子树元数据失败: %w", err)
	}
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("子树元数据文件 %s 已损坏，已拒绝覆盖；请备份或删除后重试: %w",
			subtreeMetadataPath(repoPath), err)
	}
	return metas, nil
}

func saveSubtreeMetadata(repoPath string, prefix string, meta subtreeMetadata) error {
	metas, err := readSubtreeMetadata(repoPath)
	if err != nil {
		return err
	}
	metas[prefix] = meta
	data, err := json.MarshalIndent(metas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(subtreeMetadataPath(repoPath), data, 0644)
}

func (GitService) ListSubtrees(repoPath string) (*SubtreeListResult, error) {
	out, err := ExecGit(repoPath, "log", "--grep=git-subtree-dir:", "-n", "50", "--pretty=format:%H%x00%s%x00%b")
	if err != nil {
		return &SubtreeListResult{Subtrees: []SubtreeItem{}}, nil
	}

	subtreeMap := make(map[string]*SubtreeItem)
	metadata, err := readSubtreeMetadata(repoPath)
	if err != nil {
		return nil, err
	}

	for _, block := range strings.Split(out, "\n") {
		parts := strings.Split(block, "\x00")
		if len(parts) >= 3 {
			commitHash := parts[0]
			body := parts[2]

			var dir string
			var split string
			for _, line := range strings.Split(body, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "git-subtree-dir:") {
					dir = strings.TrimSpace(strings.TrimPrefix(line, "git-subtree-dir:"))
				}
				if strings.HasPrefix(line, "git-subtree-split:") {
					split = strings.TrimSpace(strings.TrimPrefix(line, "git-subtree-split:"))
				}
			}

			if dir != "" {
				if _, exists := subtreeMap[dir]; !exists {
					meta := metadata[dir]
					branch := meta.Branch
					if branch == "" {
						branch = "main"
					}
					subtreeMap[dir] = &SubtreeItem{
						Prefix:      dir,
						RemoteOrURL: meta.RemoteOrURL,
						Branch:      branch,
						LastCommit:  commitHash[:8],
						LastSquash:  split,
					}
				}
			}
		}
	}

	var list []SubtreeItem
	for _, it := range subtreeMap {
		list = append(list, *it)
	}

	return &SubtreeListResult{Subtrees: list}, nil
}

// AddSubtree adds an external git repository as a subtree component
func (GitService) AddSubtree(repoPath string, prefix string, remoteOrUrl string, branch string, squash bool) (*SubtreeActionResult, error) {
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "add", "--prefix=" + prefix, remoteOrUrl, branch}
	if squash {
		args = append(args, "--squash")
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("添加 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	msg := fmt.Sprintf("成功将子树 %s 集成至目录 %s", remoteOrUrl, prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// PullSubtree pulls updates for an integrated subtree component
func (GitService) PullSubtree(repoPath string, prefix string, remoteOrUrl string, branch string, squash bool) (*SubtreeActionResult, error) {
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "pull", "--prefix=" + prefix, remoteOrUrl, branch}
	if squash {
		args = append(args, "--squash")
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("拉取 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	msg := fmt.Sprintf("成功从远端同步子树 %s 最新更新", prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// PushSubtree pushes local changes back to the upstream subtree repository
func (GitService) PushSubtree(repoPath string, prefix string, remoteOrUrl string, branch string) (*SubtreeActionResult, error) {
	if prefix == "" || remoteOrUrl == "" || branch == "" {
		return nil, errors.New("子树前缀路径、仓库地址与分支不能为空")
	}

	args := []string{"subtree", "push", "--prefix=" + prefix, remoteOrUrl, branch}
	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return &SubtreeActionResult{
			Success: false,
			Message: fmt.Sprintf("推送 Subtree 失败: %v", err),
			Output:  out,
		}, err
	}

	existing := subtreeMetadata{}
	if metas, metaErr := readSubtreeMetadata(repoPath); metaErr == nil {
		existing = metas[prefix]
	}
	msg := fmt.Sprintf("成功将子树 %s 变更推送回上游", prefix)
	if err := saveSubtreeMetadata(repoPath, prefix, subtreeMetadata{
		RemoteOrURL: remoteOrUrl,
		Branch:      branch,
		Squash:      existing.Squash,
	}); err != nil {
		msg += fmt.Sprintf("；但本地子树信息保存失败（%v），下次列表可能缺少远端/分支信息", err)
	}
	return &SubtreeActionResult{
		Success: true,
		Message: msg,
		Output:  out,
	}, nil
}

// GetCommitSigningStatus checks GPG/SSH commit signature configuration and audits recent commits
func (GitService) GetCommitSigningStatus(repoPath string) (*CommitSigningStatus, error) {
	status := &CommitSigningStatus{
		RecentCommits:    []SignedCommitItem{},
		AvailableSSHKeys: []string{},
	}

	gpgSignVal, _ := ExecGit(repoPath, "config", "--get", "commit.gpgsign")
	status.GpgSignEnabled = strings.TrimSpace(gpgSignVal) == "true"

	keyVal, _ := ExecGit(repoPath, "config", "--get", "user.signingkey")
	status.SigningKey = strings.TrimSpace(keyVal)

	fmtVal, _ := ExecGit(repoPath, "config", "--get", "gpg.format")
	status.GpgFormat = strings.TrimSpace(fmtVal)
	if status.GpgFormat == "" {
		status.GpgFormat = "openpgp"
	}

	if _, err := exec.LookPath("gpg"); err == nil {
		status.HasGpgBinary = true
	}

	if home, err := os.UserHomeDir(); err == nil {
		sshDir := filepath.Join(home, ".ssh")
		entries, _ := os.ReadDir(sshDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".pub") {
				status.AvailableSSHKeys = append(status.AvailableSSHKeys, filepath.Join(sshDir, e.Name()))
			}
		}
	}

	// Audit recent 15 commits with signature status
	logOut, err := ExecGit(repoPath, "log", "-n", "15", "--show-signature", "--pretty=format:COMMIT_SIGN_DELIM%H|%h|%an|%ae|%cd|%s|%G?|%GS|%GK")
	if err == nil && logOut != "" {
		records := strings.Split(logOut, "COMMIT_SIGN_DELIM")
		for _, r := range records {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			parts := strings.Split(r, "|")
			if len(parts) >= 7 {
				sigCode := strings.TrimSpace(parts[6])
				signer := ""
				keyId := ""
				if len(parts) > 7 {
					signer = strings.TrimSpace(parts[7])
				}
				if len(parts) > 8 {
					keyId = strings.TrimSpace(parts[8])
				}

				human := "未签名 (存在仿冒风险)"
				switch sigCode {
				case "G":
					human = "已验证 (有效公钥签名)"
					status.CompliantCount++
				case "U":
					human = "有效签名 (未受信任公钥)"
					status.CompliantCount++
				case "B":
					human = "签名损坏 (Bad Signature)"
				case "X":
					human = "密钥已过期"
				case "Y":
					human = "签名者已过期"
				case "R":
					human = "签名已被吊销"
				case "E":
					human = "缺少公钥无法校验"
				default:
					sigCode = "N"
				}

				status.TotalChecked++
				status.RecentCommits = append(status.RecentCommits, SignedCommitItem{
					CommitHash:      parts[0],
					ShortHash:       parts[1],
					AuthorName:      parts[2],
					AuthorEmail:     parts[3],
					Date:            parts[4],
					Subject:         parts[5],
					SignatureStatus: sigCode,
					Signer:          signer,
					KeyID:           keyId,
					StatusHuman:     human,
				})
			}
		}
	}

	if status.TotalChecked > 0 {
		pct := float64(status.CompliantCount) / float64(status.TotalChecked) * 100
		status.ComplianceRate = fmt.Sprintf("%.1f%%", pct)
	} else {
		status.ComplianceRate = "0%"
	}

	return status, nil
}

// ConfigureCommitSigning enables or disables GPG/SSH commit signature protection
func (GitService) ConfigureCommitSigning(repoPath string, enable bool, keyPath string, format string) (*CommitSigningResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if enable {
		formatValue := "openpgp"
		if strings.TrimSpace(format) == "ssh" {
			formatValue = "ssh"
		}
		if _, err := ExecGit(repoPath, "config", "gpg.format", formatValue); err != nil {
			return nil, fmt.Errorf("写入 gpg.format 失败: %w", err)
		}
		if keyPath != "" {
			if _, err := ExecGit(repoPath, "config", "user.signingkey", strings.TrimSpace(keyPath)); err != nil {
				return nil, fmt.Errorf("写入 user.signingkey 失败: %w", err)
			}
		}
		if _, err := ExecGit(repoPath, "config", "commit.gpgsign", "true"); err != nil {
			return nil, fmt.Errorf("启用 commit.gpgsign 失败: %w", err)
		}
		return &CommitSigningResult{
			Success: true,
			Message: "已成功启用提交防伪签名！后续提交将自动进行密码学认证。",
		}, nil
	}

	_, err := ExecGit(repoPath, "config", "commit.gpgsign", "false")
	if err != nil {
		return nil, err
	}
	return &CommitSigningResult{
		Success: true,
		Message: "已停用提交签名保护。",
	}, nil
}

// AuditIgnoreRules inspects .gitignore completeness and discovers tracked engine build artifacts
func (GitService) AuditIgnoreRules(repoPath string) (*IgnoreSanitizerResult, error) {
	res := &IgnoreSanitizerResult{
		TrackedArtifacts: []IgnoreArtifactItem{},
		MissingRules:     []string{},
		HealthScore:      100,
	}

	// 1. Detect engine
	if _, err := os.Stat(filepath.Join(repoPath, "Assets")); err == nil {
		res.EngineType = "Unity 3D"
	} else if matches, _ := filepath.Glob(filepath.Join(repoPath, "*.uproject")); len(matches) > 0 {
		res.EngineType = "Unreal Engine"
	} else if _, err := os.Stat(filepath.Join(repoPath, "project.godot")); err == nil {
		res.EngineType = "Godot Engine"
	} else if _, err := os.Stat(filepath.Join(repoPath, "project.json")); err == nil {
		res.EngineType = "Cocos Creator"
	} else {
		res.EngineType = "通用游戏工程"
	}

	// 2. Read .gitignore
	ignorePath := filepath.Join(repoPath, ".gitignore")
	ignoreBytes, err := os.ReadFile(ignorePath)
	res.HasGitIgnore = (err == nil)
	ignoreContent := string(ignoreBytes)

	// Check missing recommended rules
	type ruleCheck struct {
		pattern  string
		category string
	}
	recommended := []ruleCheck{}
	switch res.EngineType {
	case "Unity 3D":
		recommended = []ruleCheck{
			{"[Ll]ibrary/", "Unity Library 临时缓存"},
			{"[Tt]emp/", "Unity Temp 编辑器构建目录"},
			{"[Oo]bj/", "编译中间产物 Obj"},
			{"[Bb]uild/", "构建输出目录"},
			{"[Ll]ogs/", "Unity 编辑器日志"},
			{"MemoryCaptures/", "内存诊断抓包"},
			{"*.apk", "Android 包文件"},
			{"*.unitypackage", "导出的包文件"},
		}
	case "Unreal Engine":
		recommended = []ruleCheck{
			{"Binaries/", "虚幻可执行文件 Binaries"},
			{"DerivedDataCache/", "DDC 派生数据缓存"},
			{"Intermediate/", "虚幻中间生成文件"},
			{"Saved/", "本地保存与 Crash 报告"},
			{".vs/", "Visual Studio 配置"},
			{"*.pdb", "调试符号文件 PDB"},
		}
	case "Godot Engine":
		recommended = []ruleCheck{
			{".godot/", "Godot 4 导入缓存"},
			{".import/", "Godot 3 导入缓存"},
			{"export_presets.cfg", "导出预设密码凭证"},
		}
	default:
		recommended = []ruleCheck{
			{".DS_Store", "macOS 系统元数据"},
			{"Thumbs.db", "Windows 缩略图缓存"},
			{".idea/", "JetBrains IDE 配置"},
			{".vscode/", "VSCode 本地配置"},
			{"*.log", "运行时日志"},
			{"*.tmp", "临时文件"},
		}
	}

	for _, rc := range recommended {
		if !hasIgnoreRule(ignoreContent, rc.pattern) {
			res.MissingRules = append(res.MissingRules, fmt.Sprintf("%s (%s)", rc.pattern, rc.category))
		}
	}

	// 3. Scan git ls-files for dangerously tracked build artifacts
	lsOut, err := ExecGit(repoPath, "ls-files")
	if err == nil && lsOut != "" {
		lines := strings.Split(lsOut, "\n")
		for _, file := range lines {
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}

			category := ""
			lower := strings.ToLower(file)
			if strings.HasPrefix(file, "Library/") || strings.HasPrefix(lower, "library/") {
				category = "Unity Library 缓存 (极度臃肿)"
			} else if strings.HasPrefix(file, "Temp/") || strings.HasPrefix(lower, "temp/") {
				category = "Unity Temp 临时目录"
			} else if strings.HasPrefix(file, "Binaries/") || strings.HasPrefix(file, "Intermediate/") || strings.HasPrefix(file, "DerivedDataCache/") {
				category = "Unreal Engine 构建产物"
			} else if strings.HasPrefix(file, ".vs/") || strings.HasPrefix(file, ".idea/") {
				category = "IDE 配置文件"
			} else if strings.HasSuffix(lower, ".pdb") || strings.HasSuffix(lower, ".obj") || strings.HasSuffix(lower, ".apk") || strings.HasSuffix(lower, ".exe") {
				category = "二进制构建产物"
			} else if strings.HasSuffix(lower, ".ds_store") || strings.HasSuffix(lower, "thumbs.db") {
				category = "操作系统无用元数据"
			}

			if category != "" {
				full := filepath.Join(repoPath, file)
				var sz int64 = 0
				szHuman := "0 B"
				if fi, sErr := os.Stat(full); sErr == nil {
					sz = fi.Size()
					if sz < 1024 {
						szHuman = strconv.FormatInt(sz, 10) + " B"
					} else if sz < 1024*1024 {
						szHuman = fmt.Sprintf("%.1f KB", float64(sz)/1024)
					} else {
						szHuman = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
					}
				}

				res.TrackedArtifacts = append(res.TrackedArtifacts, IgnoreArtifactItem{
					Path:      file,
					Category:  category,
					IsTracked: true,
					SizeBytes: sz,
					SizeHuman: szHuman,
					Ignored:   false,
					Remedy:    "执行 git rm --cached 移出跟踪",
				})
			}
		}
	}

	res.TrackedCount = len(res.TrackedArtifacts)
	res.TotalArtifacts = res.TrackedCount

	// Compute score
	score := 100 - (res.TrackedCount * 15) - (len(res.MissingRules) * 5)
	if !res.HasGitIgnore {
		score -= 30
	}
	if score < 0 {
		score = 0
	}
	res.HealthScore = score

	if res.TrackedCount > 0 {
		res.Summary = fmt.Sprintf("⚠️ 风险发现：检测到 %d 个编译产物/临时目录被错误推入 Git 跟踪！缺失 %d 条关键忽略规则，健康度评分: %d/100。", res.TrackedCount, len(res.MissingRules), res.HealthScore)
	} else if len(res.MissingRules) > 0 {
		res.Summary = fmt.Sprintf("ℹ️ 提示：未发现错误跟踪的产物，但缺少 %d 条推荐的 %s 规范规则，健康度评分: %d/100。", len(res.MissingRules), res.EngineType, res.HealthScore)
	} else {
		res.Summary = fmt.Sprintf("✅ 规范体检完美通过！.gitignore 完备，无临时构建产物跟踪，健康度评分: %d/100。", res.HealthScore)
	}

	return res, nil
}

func normalizeIgnoreRule(rule string) string {
	lower := strings.ToLower(strings.TrimSpace(rule))
	lower = strings.TrimPrefix(lower, "/")
	var normalized strings.Builder
	for i := 0; i < len(lower); {
		if i+3 < len(lower) && lower[i] == '[' && lower[i+3] == ']' && lower[i+1] == lower[i+2] {
			normalized.WriteByte(lower[i+1])
			i += 4
			continue
		}
		normalized.WriteByte(lower[i])
		i++
	}
	return normalized.String()
}

func hasIgnoreRule(content string, pattern string) bool {
	normalizedPattern := normalizeIgnoreRule(pattern)
	if normalizedPattern == "" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if normalizedLine := normalizeIgnoreRule(line); normalizedLine != "" && strings.Contains(normalizedLine, normalizedPattern) {
			return true
		}
	}
	return false
}

// ApplyIgnoreSanitizer fixes .gitignore rules and untracks dangerous build artifacts
func (s GitService) ApplyIgnoreSanitizer(repoPath string, engineType string, untrackExisting bool) (*IgnoreSanitizeActionResult, error) {
	res := &IgnoreSanitizeActionResult{
		RulesAdded:     []string{},
		UntrackedFiles: []string{},
	}

	// 1. Audit first
	audit, err := s.AuditIgnoreRules(repoPath)
	if err != nil {
		return nil, err
	}

	// 2. Append rules to .gitignore
	ignorePath := filepath.Join(repoPath, ".gitignore")
	var existingContent string
	if b, readErr := os.ReadFile(ignorePath); readErr == nil {
		existingContent = string(b)
	} else if !os.IsNotExist(readErr) {
		// Treating an unreadable file as empty would append duplicate rules.
		return nil, fmt.Errorf("读取 .gitignore 失败: %w", readErr)
	}

	rulesToAdd := []string{}
	ruleTemplates := map[string][]string{
		"Unity 3D": {
			"[Ll]ibrary/", "[Tt]emp/", "[Oo]bj/", "[Bb]uild/", "[Bb]uilds/",
			"[Ll]ogs/", "MemoryCaptures/", "*.apk", "*.unitypackage",
		},
		"Unreal Engine": {
			"Binaries/", "DerivedDataCache/", "Intermediate/", "Saved/",
			".vs/", "*.pdb", "*.sln",
		},
		"Godot Engine": {
			".godot/", ".import/", "export_presets.cfg",
		},
		"通用游戏工程": {
			".DS_Store", "Thumbs.db", ".idea/", ".vscode/", "*.log", "*.tmp",
		},
	}

	tpl, ok := ruleTemplates[engineType]
	if !ok {
		tpl = ruleTemplates["通用游戏工程"]
	}

	for _, r := range tpl {
		if !hasIgnoreRule(existingContent, r) {
			rulesToAdd = append(rulesToAdd, r)
		}
	}

	if len(rulesToAdd) > 0 {
		appendStr := "\n# --- Studio Game Engine Ignore Rules ---\n" + strings.Join(rulesToAdd, "\n") + "\n"
		f, fErr := os.OpenFile(ignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fErr != nil {
			return nil, fmt.Errorf("写入 .gitignore 失败: %w", fErr)
		}
		if _, writeErr := f.WriteString(appendStr); writeErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("写入 .gitignore 失败: %w", writeErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return nil, fmt.Errorf("关闭 .gitignore 失败: %w", closeErr)
		}
		res.RulesAdded = rulesToAdd
	}

	// 3. Untrack tracked artifacts if requested
	if untrackExisting && len(audit.TrackedArtifacts) > 0 {
		for _, item := range audit.TrackedArtifacts {
			_, rmErr := ExecGit(repoPath, "rm", "--cached", "--quiet", "--", item.Path)
			if rmErr == nil {
				res.UntrackedFiles = append(res.UntrackedFiles, item.Path)
			}
		}
	}

	res.Success = true
	res.Message = fmt.Sprintf("已成功优化 .gitignore（新增 %d 条规则），并安全移出 %d 个误跟踪产物！", len(res.RulesAdded), len(res.UntrackedFiles))
	return res, nil
}

func formatAssetSize(sz int64) string {
	if sz < 1024 {
		return strconv.FormatInt(sz, 10) + " B"
	} else if sz < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(sz)/1024)
	}
	return fmt.Sprintf("%.2f MB", float64(sz)/(1024*1024))
}

// GetFileRevisionTimeline retrieves the chronological commit history of a single file for historical time-travel
func (GitService) GetFileRevisionTimeline(repoPath string, filePath string) (*FileTimelineResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if filePath == "" {
		return nil, errors.New("filePath 不能为空")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	logOut, err := ExecGit(repoPath, "log", "--follow", "-n", "30", "--pretty=format:%H|%h|%an|%cd|%s", "--", cleanPath)
	if err != nil {
		// An empty repository has no commits yet, which is not a read failure.
		if _, headErr := ExecGit(repoPath, "rev-parse", "--verify", "HEAD"); headErr != nil {
			return &FileTimelineResult{
				FilePath:  cleanPath,
				Revisions: []FileRevisionItem{},
				Message:   "仓库尚无提交，暂无文件历史",
			}, nil
		}
		return nil, fmt.Errorf("读取文件历史失败: %w", err)
	}
	if strings.TrimSpace(logOut) == "" {
		return &FileTimelineResult{
			FilePath:  cleanPath,
			Revisions: []FileRevisionItem{},
			Message:   "未检索到该文件的提交历史记录",
		}, nil
	}

	revisions := []FileRevisionItem{}
	lines := strings.Split(logOut, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Split(l, "|")
		if len(parts) >= 5 {
			h := parts[0]
			szHuman := "--"
			var szBytes int64 = 0
			szOut, szErr := ExecGit(repoPath, "cat-file", "-s", fmt.Sprintf("%s:%s", h, cleanPath))
			if szErr == nil && szOut != "" {
				if parsedSz, pErr := strconv.ParseInt(strings.TrimSpace(szOut), 10, 64); pErr == nil {
					szBytes = parsedSz
					szHuman = formatAssetSize(parsedSz)
				}
			}

			revisions = append(revisions, FileRevisionItem{
				CommitHash: h,
				ShortHash:  parts[1],
				AuthorName: parts[2],
				Date:       parts[3],
				Subject:    parts[4],
				SizeBytes:  szBytes,
				SizeStr:    szHuman,
			})
		}
	}

	return &FileTimelineResult{
		FilePath:  cleanPath,
		Revisions: revisions,
		Message:   fmt.Sprintf("成功提取 %d 条历史修订版本记录", len(revisions)),
	}, nil
}

// RevertFileToRevision reverts a single file to a specific historical commit without modifying other files
func (GitService) RevertFileToRevision(repoPath string, filePath string, commitHash string) (*FileRevertResult, error) {
	if filePath == "" || commitHash == "" {
		return nil, errors.New("filePath 与 commitHash 均不能为空")
	}

	cleanPath, err := repoRelativePathFromInput(repoPath, filePath)
	if err != nil {
		return nil, err
	}

	if _, err := ExecGit(repoPath, "checkout", strings.TrimSpace(commitHash), "--", cleanPath); err != nil {
		return nil, fmt.Errorf("还原文件失败: %v", err)
	}

	return &FileRevertResult{
		Success:    true,
		FilePath:   cleanPath,
		CommitHash: commitHash,
		Message:    fmt.Sprintf("已成功将文件 %s 精准还原至版本 %s！工作区其他资产保持完整无损。", filepath.Base(cleanPath), shortHashOf(commitHash)),
	}, nil
}
