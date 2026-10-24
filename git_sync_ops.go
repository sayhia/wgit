package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// =========================================================================
// Advanced Pull Options & Results
// =========================================================================

type PullAdvancedOptions struct {
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
	Strategy   string `json:"strategy"` // "default", "rebase", "ff-only", "no-ff", "merge"
	Autostash  bool   `json:"autostash"`
	Prune      bool   `json:"prune"`
	Submodules bool   `json:"submodules"`
}

type PullResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	Updated      bool     `json:"updated"`
	AlreadyUp    bool     `json:"alreadyUp"`
	FastForward  bool     `json:"fastForward"`
	HasConflicts bool     `json:"hasConflicts"`
	Conflicts    []string `json:"conflicts,omitempty"`
}

// PullAdvanced executes a tailored git pull with strategy and safety flags
func (GitService) PullAdvanced(repoPath string, opts PullAdvancedOptions) (*PullResult, error) {
	args := []string{"pull"}
	switch opts.Strategy {
	case "rebase":
		args = append(args, "--rebase")
	case "ff-only":
		args = append(args, "--ff-only")
	case "no-ff":
		args = append(args, "--no-ff")
	case "merge":
		args = append(args, "--no-rebase")
	}
	if opts.Autostash {
		args = append(args, "--autostash")
	}
	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Submodules {
		args = append(args, "--recurse-submodules")
	}
	if opts.Remote != "" {
		args = append(args, opts.Remote)
		if opts.Branch != "" {
			args = append(args, opts.Branch)
		}
	}

	out, err := ExecGit(repoPath, args...)
	res := &PullResult{
		Success: err == nil,
		Message: out,
	}
	if err != nil {
		res.Message = err.Error() + "\n" + out
		if strings.Contains(res.Message, "CONFLICT") || strings.Contains(res.Message, "conflict") {
			res.HasConflicts = true
			for _, line := range strings.Split(res.Message, "\n") {
				if strings.Contains(line, "CONFLICT") {
					res.Conflicts = append(res.Conflicts, strings.TrimSpace(line))
				}
			}
			// Conflicts leave a resolvable merge/rebase state; let the UI handle it.
			return res, nil
		}
		return res, err
	}

	res.AlreadyUp = strings.Contains(out, "Already up to date") || strings.Contains(out, "已经是最新")
	res.FastForward = strings.Contains(out, "Fast-forward")
	res.Updated = !res.AlreadyUp
	return res, nil
}

// =========================================================================
// Advanced Push Options & Ahead/Behind Inspection
// =========================================================================

type PushAdvancedOptions struct {
	Remote       string `json:"remote"`
	Branch       string `json:"branch"`
	RemoteBranch string `json:"remoteBranch,omitempty"`
	ForceMode    string `json:"forceMode"` // "none", "lease", "force"
	SetUpstream  bool   `json:"setUpstream"`
	PushTags     bool   `json:"pushTags"`
	FollowTags   bool   `json:"followTags"`
	SkipCI       bool   `json:"skipCI"`
}

type PushResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetAheadCommits returns the list of commits that exist locally but haven't been pushed to remote
func (s GitService) GetAheadCommits(repoPath string, remote string, branch string) ([]CommitItem, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	if branch == "" {
		st, err := s.GetRepoStatus(repoPath)
		if err != nil {
			return nil, err
		}
		branch = st.CurrentBranch
	}

	if _, err := ExecGit(repoPath, "rev-parse", "--verify", branch+"^{commit}"); err != nil {
		return nil, fmt.Errorf("分支 %s 不存在或不可解析: %w", branch, err)
	}
	tracking := fmt.Sprintf("%s/%s", remote, branch)
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", tracking+"^{commit}"); err != nil {
		return nil, fmt.Errorf("远端分支 %s 不存在，无法计算领先提交；请先推送并设置上游", tracking)
	}
	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	raw, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("%s..%s", tracking, branch))
	if err != nil {
		return nil, fmt.Errorf("读取领先提交失败: %w", err)
	}
	return ParseRawGitLog(raw), nil
}

// GetBehindCommits returns commits on the remote branch that have not been pulled locally
func (s GitService) GetBehindCommits(repoPath string, remote string, branch string) ([]CommitItem, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	if branch == "" {
		st, err := s.GetRepoStatus(repoPath)
		if err != nil {
			return nil, err
		}
		branch = st.CurrentBranch
	}

	if _, err := ExecGit(repoPath, "rev-parse", "--verify", branch+"^{commit}"); err != nil {
		return nil, fmt.Errorf("分支 %s 不存在或不可解析: %w", branch, err)
	}
	tracking := fmt.Sprintf("%s/%s", remote, branch)
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", tracking+"^{commit}"); err != nil {
		// No remote branch yet: there is nothing to pull.
		return []CommitItem{}, nil
	}
	format := "%x1e%H%x1f%h%x1f%P%x1f%an%x1f%ae%x1f%ci%x1f%cr%x1f%s%x1f%b%x1f%D%x1f%G?%x1f%GS"
	raw, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), fmt.Sprintf("%s..%s", branch, tracking))
	if err != nil {
		return nil, fmt.Errorf("读取落后提交失败: %w", err)
	}
	return ParseRawGitLog(raw), nil
}

// PushAdvanced pushes to remote with granular force, upstream, and tagging configurations
func (GitService) PushAdvanced(repoPath string, opts PushAdvancedOptions) (*PushResult, error) {
	if opts.Remote == "" {
		opts.Remote = defaultPushRemote(repoPath)
	}
	args := []string{"push"}
	if opts.SetUpstream {
		args = append(args, "-u")
	}
	switch opts.ForceMode {
	case "lease":
		args = append(args, "--force-with-lease")
	case "force":
		args = append(args, "--force")
	}
	if opts.PushTags {
		args = append(args, "--tags")
	} else if opts.FollowTags {
		args = append(args, "--follow-tags")
	}
	if opts.SkipCI {
		args = append(args, "-o", "ci.skip")
	}
	args = append(args, opts.Remote)

	refSpec := opts.Branch
	if opts.Branch != "" && opts.RemoteBranch != "" && opts.Branch != opts.RemoteBranch {
		refSpec = fmt.Sprintf("%s:%s", opts.Branch, opts.RemoteBranch)
	}
	if refSpec != "" {
		args = append(args, refSpec)
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return &PushResult{Success: false, Message: err.Error() + "\n" + out}, err
	}
	return &PushResult{Success: true, Message: out}, nil
}

// =========================================================================
// Advanced Merge & Pre-merge Conflict Preview
// =========================================================================

type MergePreviewResult struct {
	CanMergeCleanly bool     `json:"canMergeCleanly"`
	HasConflicts    bool     `json:"hasConflicts"`
	Conflicts       []string `json:"conflicts"`
	ChangedFiles    []string `json:"changedFiles"`
	AheadCount      int      `json:"aheadCount"`
	BehindCount     int      `json:"behindCount"`
	BaseCommit      string   `json:"baseCommit"`
	TargetCommit    string   `json:"targetCommit"`
	Details         string   `json:"details"`
}

type MergeAdvancedOptions struct {
	TargetBranch   string `json:"targetBranch"`
	Strategy       string `json:"strategy"`       // "default", "no-ff", "ff-only", "squash"
	StrategyOption string `json:"strategyOption"` // "", "ours", "theirs"
	CommitMessage  string `json:"commitMessage"`
	Autostash      bool   `json:"autostash"`
}

type MergeResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	HasConflicts bool     `json:"hasConflicts"`
	Conflicts    []string `json:"conflicts,omitempty"`
}

// PreviewMerge previews merge status, calculating ahead/behind diffs and conflict dry-run
func (GitService) PreviewMerge(repoPath string, targetBranch string) (*MergePreviewResult, error) {
	if targetBranch == "" {
		return nil, errors.New("targetBranch is required")
	}
	head, err := ExecGit(repoPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	target, err := ExecGit(repoPath, "rev-parse", "--verify", targetBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to get target ref: %w", err)
	}
	head = strings.TrimSpace(head)
	target = strings.TrimSpace(target)

	base, err := ExecGit(repoPath, "merge-base", head, target)
	if err != nil {
		return nil, fmt.Errorf("无法计算共同祖先: %w", err)
	}
	base = strings.TrimSpace(base)

	res := &MergePreviewResult{
		BaseCommit:   base,
		TargetCommit: target,
		Conflicts:    []string{},
		ChangedFiles: []string{},
	}

	// Count commits
	aheadRaw, err := ExecGit(repoPath, "rev-list", "--count", fmt.Sprintf("%s..HEAD", target))
	if err != nil {
		return nil, fmt.Errorf("统计领先提交失败: %w", err)
	}
	res.AheadCount, err = strconv.Atoi(strings.TrimSpace(aheadRaw))
	if err != nil {
		return nil, fmt.Errorf("解析领先提交数失败: %w", err)
	}
	behindRaw, err := ExecGit(repoPath, "rev-list", "--count", fmt.Sprintf("HEAD..%s", target))
	if err != nil {
		return nil, fmt.Errorf("统计落后提交失败: %w", err)
	}
	res.BehindCount, err = strconv.Atoi(strings.TrimSpace(behindRaw))
	if err != nil {
		return nil, fmt.Errorf("解析落后提交数失败: %w", err)
	}

	// List changed files between base and target
	diffFilesRaw, err := ExecGit(repoPath, "diff", "--name-only", base, target)
	if err != nil {
		return nil, fmt.Errorf("读取合并差异文件失败: %w", err)
	}
	for _, f := range strings.Split(diffFilesRaw, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			res.ChangedFiles = append(res.ChangedFiles, f)
		}
	}

	// Execute non-destructive dry-run merge with merge-tree
	out, mergeErr := ExecGit(repoPath, "merge-tree", "--write-tree", "HEAD", targetBranch)
	if mergeErr == nil {
		res.CanMergeCleanly = true
		res.HasConflicts = false
		res.Details = "可以干净合并，无任何代码冲突。"
		return res, nil
	}

	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if strings.Contains(l, "CONFLICT") {
			parts := strings.Split(l, " in ")
			if len(parts) >= 2 {
				file := strings.TrimSpace(parts[len(parts)-1])
				res.Conflicts = append(res.Conflicts, file)
			} else {
				res.Conflicts = append(res.Conflicts, strings.TrimSpace(l))
			}
		}
	}
	if len(res.Conflicts) == 0 {
		// An unsupported merge-tree or another git failure must not be
		// presented as a conflict without any conflict files.
		return nil, fmt.Errorf("合并预检失败: %w (%s)", mergeErr, strings.TrimSpace(out))
	}

	res.CanMergeCleanly = false
	res.HasConflicts = true
	res.Details = out
	return res, nil
}

// MergeAdvanced merges target branch with strategy and autostash options
func (GitService) MergeAdvanced(repoPath string, opts MergeAdvancedOptions) (*MergeResult, error) {
	if opts.TargetBranch == "" {
		return nil, errors.New("targetBranch is required")
	}
	args := []string{"merge"}
	switch opts.Strategy {
	case "no-ff":
		args = append(args, "--no-ff")
	case "ff-only":
		args = append(args, "--ff-only")
	case "squash":
		args = append(args, "--squash")
	}
	if opts.StrategyOption == "ours" {
		args = append(args, "-X", "ours")
	} else if opts.StrategyOption == "theirs" {
		args = append(args, "-X", "theirs")
	}
	if opts.Autostash {
		args = append(args, "--autostash")
	}
	if opts.CommitMessage != "" {
		args = append(args, "-m", opts.CommitMessage)
	}
	args = append(args, opts.TargetBranch)

	out, err := ExecGit(repoPath, args...)
	res := &MergeResult{
		Success: err == nil,
		Message: out,
	}
	if err != nil {
		res.Message = err.Error() + "\n" + out
		if strings.Contains(res.Message, "CONFLICT") || strings.Contains(res.Message, "conflict") {
			res.HasConflicts = true
			for _, l := range strings.Split(res.Message, "\n") {
				if strings.Contains(l, "CONFLICT") {
					res.Conflicts = append(res.Conflicts, strings.TrimSpace(l))
				}
			}
			// A conflict leaves a resolvable merge state; return it as a normal
			// result so the UI can route the user to conflict resolution.
			return res, nil
		}
		return res, err
	}

	// --squash only stages the result and ignores -m; honor the merge dialog's
	// message by creating the promised single commit here.
	if opts.Strategy == "squash" && opts.CommitMessage != "" {
		staged, stagedErr := ExecGit(repoPath, "diff", "--cached", "--name-only")
		if stagedErr == nil && strings.TrimSpace(staged) != "" {
			commitOut, commitErr := ExecGit(repoPath, "commit", "-m", opts.CommitMessage)
			out += commitOut
			res.Message = out
			if commitErr != nil {
				res.Success = false
				return res, commitErr
			}
		}
	}
	return res, nil
}

// =========================================================================
// Advanced Rebase & Progress Monitoring
// =========================================================================

type RebaseAdvancedOptions struct {
	Upstream   string `json:"upstream"`
	Onto       string `json:"onto,omitempty"`
	Autostash  bool   `json:"autostash"`
	Autosquash bool   `json:"autosquash"`
}

type RebaseResult struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	HasConflicts bool     `json:"hasConflicts"`
	Conflicts    []string `json:"conflicts,omitempty"`
}

type RebaseProgressStatus struct {
	IsRebasing      bool     `json:"isRebasing"`
	HeadName        string   `json:"headName"`
	Onto            string   `json:"onto"`
	CurrentStep     int      `json:"currentStep"`
	TotalSteps      int      `json:"totalSteps"`
	CurrentCommit   string   `json:"currentCommit"`
	HasConflicts    bool     `json:"hasConflicts"`
	ConflictedFiles []string `json:"conflictedFiles"`
}

// GetRebaseProgress inspects current rebase state, step count, target branch, and active conflicts
func (s GitService) GetRebaseProgress(repoPath string) (*RebaseProgressStatus, error) {
	status := &RebaseProgressStatus{
		ConflictedFiles: []string{},
	}
	stateDir := gitStateDir(repoPath)
	rebaseMergeDir := filepath.Join(stateDir, "rebase-merge")
	rebaseApplyDir := filepath.Join(stateDir, "rebase-apply")

	dir := ""
	if fi, err := os.Stat(rebaseMergeDir); err == nil && fi.IsDir() {
		dir = rebaseMergeDir
	} else if fi, err := os.Stat(rebaseApplyDir); err == nil && fi.IsDir() {
		dir = rebaseApplyDir
	}

	if dir == "" {
		status.IsRebasing = false
		return status, nil
	}

	status.IsRebasing = true
	if hn, err := os.ReadFile(filepath.Join(dir, "head-name")); err == nil {
		status.HeadName = strings.TrimPrefix(strings.TrimSpace(string(hn)), "refs/heads/")
	}
	if onto, err := os.ReadFile(filepath.Join(dir, "onto")); err == nil {
		status.Onto = strings.TrimSpace(string(onto))
	}
	if step, err := os.ReadFile(filepath.Join(dir, "msgnum")); err == nil {
		status.CurrentStep, _ = strconv.Atoi(strings.TrimSpace(string(step)))
	} else if step, err := os.ReadFile(filepath.Join(dir, "next")); err == nil {
		status.CurrentStep, _ = strconv.Atoi(strings.TrimSpace(string(step)))
	}
	if total, err := os.ReadFile(filepath.Join(dir, "end")); err == nil {
		status.TotalSteps, _ = strconv.Atoi(strings.TrimSpace(string(total)))
	} else if total, err := os.ReadFile(filepath.Join(dir, "last")); err == nil {
		status.TotalSteps, _ = strconv.Atoi(strings.TrimSpace(string(total)))
	}

	// Current stopped commit
	if stoppedSha, err := os.ReadFile(filepath.Join(dir, "stopped-sha")); err == nil {
		sha := strings.TrimSpace(string(stoppedSha))
		if sha != "" {
			subject, logErr := ExecGit(repoPath, "log", "-1", "--format=%h %s", sha)
			if logErr != nil {
				status.CurrentCommit = shortHashOf(sha)
			} else {
				status.CurrentCommit = strings.TrimSpace(subject)
			}
		}
	}

	// Check conflicts in status
	st, err := s.GetRepoStatus(repoPath)
	if err == nil && len(st.ConflictedFiles) > 0 {
		status.HasConflicts = true
		status.ConflictedFiles = append(status.ConflictedFiles, st.ConflictedFiles...)
	}

	return status, nil
}

// RebaseAdvanced rebases current branch onto upstream with --onto, autostash, and autosquash
func (GitService) RebaseAdvanced(repoPath string, opts RebaseAdvancedOptions) (*RebaseResult, error) {
	if opts.Upstream == "" {
		return nil, errors.New("upstream is required")
	}
	args := []string{"rebase"}
	if opts.Onto != "" {
		args = append(args, "--onto", opts.Onto)
	}
	if opts.Autostash {
		args = append(args, "--autostash")
	}
	if opts.Autosquash {
		args = append(args, "--autosquash")
	}
	args = append(args, opts.Upstream)

	out, err := ExecGit(repoPath, args...)
	res := &RebaseResult{
		Success: err == nil,
		Message: out,
	}
	if err != nil {
		res.Message = err.Error() + "\n" + out
		if strings.Contains(res.Message, "CONFLICT") || strings.Contains(res.Message, "conflict") {
			res.HasConflicts = true
			for _, l := range strings.Split(res.Message, "\n") {
				if strings.Contains(l, "CONFLICT") {
					res.Conflicts = append(res.Conflicts, strings.TrimSpace(l))
				}
			}
			// Keep the same contract as MergeAdvanced: conflicts are an
			// actionable result, not an RPC failure.
			return res, nil
		}
		return res, err
	}
	return res, nil
}

// =========================================================================
// Universal Conflict Operation Lifecycle (Abort / Continue / Skip)
// =========================================================================

// AbortConflictOperation detects whether repo is in merge, rebase, or cherry-pick and safely aborts it
func (GitService) AbortConflictOperation(repoPath string) (string, error) {
	stateDir := gitStateDir(repoPath)
	if _, err := os.Stat(filepath.Join(stateDir, "MERGE_HEAD")); err == nil {
		return ExecGit(repoPath, "merge", "--abort")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-merge")); err == nil {
		return ExecGit(repoPath, "rebase", "--abort")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-apply")); err == nil {
		return ExecGit(repoPath, "rebase", "--abort")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "CHERRY_PICK_HEAD")); err == nil {
		return ExecGit(repoPath, "cherry-pick", "--abort")
	}
	return "", errors.New("当前没有进行中的合并、变基或 Cherry-Pick 操作")
}

// ContinueConflictOperation continues whichever operation is in progress (merge, rebase, cherry-pick)
func (GitService) ContinueConflictOperation(repoPath string) (string, error) {
	stateDir := gitStateDir(repoPath)
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-merge")); err == nil {
		return ExecGit(repoPath, "rebase", "--continue")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-apply")); err == nil {
		return ExecGit(repoPath, "rebase", "--continue")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "CHERRY_PICK_HEAD")); err == nil {
		return ExecGit(repoPath, "cherry-pick", "--continue")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "MERGE_HEAD")); err == nil {
		return ExecGit(repoPath, "commit", "--no-edit")
	}
	return "", errors.New("当前没有进行中的合并、变基或 Cherry-Pick 操作")
}

// SkipConflictOperation skips current commit in rebase or cherry-pick
func (GitService) SkipConflictOperation(repoPath string) (string, error) {
	stateDir := gitStateDir(repoPath)
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-merge")); err == nil {
		return ExecGit(repoPath, "rebase", "--skip")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "rebase-apply")); err == nil {
		return ExecGit(repoPath, "rebase", "--skip")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "CHERRY_PICK_HEAD")); err == nil {
		return ExecGit(repoPath, "cherry-pick", "--skip")
	}
	return "", errors.New("仅变基或 Cherry-Pick 支持跳过提交操作")
}
