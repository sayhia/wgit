package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Git LFS 深度维护与可锁资产守护（选择性拉取/推送、缓存与锁巡检、DCC 联动）。

// lfsObjectsDir resolves the LFS media cache through the git common dir.
// A linked worktree's own .git is a file, while LFS objects stay in the main
// repository's .git/lfs/objects.
func lfsObjectsDir(repoPath string) string {
	return filepath.Join(gitCommonDir(repoPath), "lfs", "objects")
}

func isGitLfsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a git command") ||
		strings.Contains(msg, "command not found") ||
		strings.Contains(msg, "no such file or directory")
}

// GetLfsLocks returns list of files locked via Git LFS
func (GitService) GetLfsLocks(repoPath string) ([]LfsLockInfo, error) {
	if raw, err := ExecGit(repoPath, "lfs", "locks", "--json"); err == nil {
		if locks, ok := parseLfsLocksJSON(raw); ok {
			return locks, nil
		}
	}

	raw, err := ExecGit(repoPath, "lfs", "locks")
	if err != nil {
		if isGitLfsUnavailable(err) {
			// Without git-lfs there is no lock source; keep the tab usable.
			return []LfsLockInfo{}, nil
		}
		return nil, fmt.Errorf("读取 Git LFS 锁失败: %w", err)
	}
	return parseLfsLocksPlain(raw), nil
}

type lfsLockJSON struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Owner *struct {
		Name string `json:"name"`
	} `json:"owner"`
	LockedAt time.Time `json:"locked_at"`
}

func parseLfsLocksJSON(raw string) ([]LfsLockInfo, bool) {
	var parsed []lfsLockJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}
	locks := make([]LfsLockInfo, 0, len(parsed))
	for _, l := range parsed {
		owner := ""
		if l.Owner != nil {
			owner = l.Owner.Name
		}
		lockedAt := ""
		if !l.LockedAt.IsZero() {
			lockedAt = l.LockedAt.Format(time.RFC3339)
		}
		locks = append(locks, LfsLockInfo{
			ID:       l.ID,
			Path:     l.Path,
			Owner:    owner,
			LockedAt: lockedAt,
		})
	}
	return locks, true
}

func parseLfsLocksPlain(raw string) []LfsLockInfo {
	var locks []LfsLockInfo
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <path>\t<owner>\tID:<id>
		pts := strings.Split(line, "\t")
		if len(pts) >= 2 {
			path := strings.TrimSpace(pts[0])
			owner := strings.TrimSpace(pts[1])
			id := ""
			if len(pts) >= 3 {
				id = strings.TrimPrefix(strings.TrimSpace(pts[2]), "ID:")
			}
			locks = append(locks, LfsLockInfo{
				ID:    id,
				Path:  path,
				Owner: owner,
			})
		}
	}
	return locks
}

// GetLfsTrackedPatterns returns patterns currently tracked by git-lfs
func (GitService) GetLfsTrackedPatterns(repoPath string) ([]LfsTrackPattern, error) {
	raw, err := ExecGit(repoPath, "lfs", "track")
	if err != nil {
		// Without the git-lfs binary, .gitattributes still records LFS filters.
		attr, readErr := os.ReadFile(filepath.Join(repoPath, ".gitattributes"))
		if readErr != nil {
			return []LfsTrackPattern{}, nil
		}
		patterns := make([]LfsTrackPattern, 0)
		for _, line := range strings.Split(string(attr), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "filter=lfs") {
				continue
			}
			if fields := strings.Fields(line); len(fields) > 0 {
				patterns = append(patterns, LfsTrackPattern{Pattern: fields[0], Source: ".gitattributes"})
			}
		}
		return patterns, nil
	}

	var patterns []LfsTrackPattern
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			pat := parts[0]
			src := ""
			if len(parts) >= 2 {
				src = strings.Trim(parts[1], "()")
			}
			patterns = append(patterns, LfsTrackPattern{
				Pattern: pat,
				Source:  src,
			})
		}
	}
	return patterns, nil
}

// TrackLfsPattern tracks a file pattern using Git LFS
func (GitService) TrackLfsPattern(repoPath string, pattern string) error {
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	if _, err := ExecGit(repoPath, "lfs", "track", "--", pattern); err != nil {
		return err
	}
	if _, err := ExecGit(repoPath, "add", ".gitattributes"); err != nil {
		return fmt.Errorf("LFS 规则已写入但暂存 .gitattributes 失败: %w", err)
	}
	return nil
}

// UntrackLfsPattern stops tracking a file pattern via Git LFS
func (GitService) UntrackLfsPattern(repoPath string, pattern string) error {
	if pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	if _, err := ExecGit(repoPath, "lfs", "untrack", "--", pattern); err != nil {
		return err
	}
	if _, err := ExecGit(repoPath, "add", ".gitattributes"); err != nil {
		return fmt.Errorf("LFS 规则已移除但暂存 .gitattributes 失败: %w", err)
	}
	return nil
}

// LfsPull runs git lfs pull
func (GitService) LfsPull(repoPath string) (string, error) {
	return ExecGit(repoPath, "lfs", "pull")
}

// LfsPush runs git lfs push
func (GitService) LfsPush(repoPath string) (string, error) {
	return ExecGit(repoPath, "lfs", "push", defaultPushRemote(repoPath), "--all")
}

// CheckFileLockStatus checks if a file is locked in Git LFS and returns the lock owner
func (GitService) CheckFileLockStatus(repoPath string, filePath string) (string, error) {
	if filePath == "" {
		return "", errors.New("filePath cannot be empty")
	}

	if raw, err := ExecGit(repoPath, "lfs", "locks", "--json", "--path", filePath); err == nil {
		if locks, ok := parseLfsLocksJSON(raw); ok {
			if len(locks) > 0 {
				return locks[0].Owner, nil
			}
			return "", nil
		}
	}

	raw, err := ExecGit(repoPath, "lfs", "locks", "--path", filePath)
	if err != nil {
		return "", fmt.Errorf("查询 LFS 锁状态失败: %w", err)
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pts := strings.Split(line, "\t")
		if len(pts) >= 2 {
			return strings.TrimSpace(pts[1]), nil
		}
	}
	return "", nil
}

// CheckRemoteLocksConflict finds any modified files in working directory that are locked by other users
func (s GitService) CheckRemoteLocksConflict(repoPath string) ([]FileStatus, error) {
	status, err := s.GetRepoStatus(repoPath)
	if err != nil {
		return nil, err
	}
	locks, err := s.GetLfsLocks(repoPath)
	if err != nil || len(locks) == 0 {
		return []FileStatus{}, nil
	}

	curUser, _ := ExecGit(repoPath, "config", "user.name")
	curUser = strings.TrimSpace(curUser)

	lockMap := make(map[string]string)
	for _, l := range locks {
		lockMap[filepath.Clean(l.Path)] = l.Owner
	}

	var conflicts []FileStatus
	checkFile := func(f FileStatus) {
		clean := filepath.Clean(f.Path)
		if owner, locked := lockMap[clean]; locked {
			if owner != curUser {
				f.LockOwner = owner
				conflicts = append(conflicts, f)
			}
		}
	}

	for _, f := range status.UnstagedFiles {
		checkFile(f)
	}
	for _, f := range status.StagedFiles {
		checkFile(f)
	}

	return conflicts, nil
}

// InspectLfsFileHistory retrieves historical revisions of a specific asset with OID and author
func (GitService) InspectLfsFileHistory(repoPath string, filePath string) ([]LfsFileRevision, error) {
	if filePath == "" {
		return []LfsFileRevision{}, errors.New("filePath is required")
	}
	format := "%H%x1f%h%x1f%an%x1f%ae%x1f%ci%x1f%s"
	out, err := ExecGit(repoPath, "log", fmt.Sprintf("--format=%s", format), "--follow", "--", filePath)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not have any commits yet") {
			return []LfsFileRevision{}, nil
		}
		return nil, fmt.Errorf("读取 LFS 文件历史失败: %v", err)
	}

	var revs []LfsFileRevision
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		parts := strings.Split(l, "\x1f")
		if len(parts) >= 6 {
			rev := LfsFileRevision{
				CommitHash:  parts[0],
				ShortHash:   parts[1],
				AuthorName:  parts[2],
				AuthorEmail: parts[3],
				Date:        parts[4],
				Subject:     parts[5],
			}
			// Attempt to extract LFS pointer oid
			showOut, _ := ExecGit(repoPath, "show", fmt.Sprintf("%s:%s", rev.CommitHash, filePath))
			if strings.Contains(showOut, "oid sha256:") {
				for _, ptrLine := range strings.Split(showOut, "\n") {
					if strings.HasPrefix(ptrLine, "oid sha256:") {
						rev.Oid = strings.TrimPrefix(ptrLine, "oid sha256:")
					} else if strings.HasPrefix(ptrLine, "size ") {
						sz, _ := strconv.ParseInt(strings.TrimPrefix(ptrLine, "size "), 10, 64)
						rev.SizeBytes = sz
						rev.SizeHuman = formatByteSize(sz)
					}
				}
			}
			revs = append(revs, rev)
		}
	}
	return revs, nil
}

// LockAllModifiedLfs scans all modified files and acquires remote LFS locks for any tracked binary assets
func (s GitService) LockAllModifiedLfs(repoPath string) (*BulkLockResult, error) {
	status, err := s.GetRepoStatus(repoPath)
	if err != nil {
		return nil, err
	}

	var candidates []string
	seen := make(map[string]bool)
	addCandidate := func(f FileStatus) {
		if !seen[f.Path] {
			seen[f.Path] = true
			candidates = append(candidates, f.Path)
		}
	}

	for _, f := range status.UnstagedFiles {
		addCandidate(f)
	}
	for _, f := range status.StagedFiles {
		addCandidate(f)
	}

	if len(candidates) == 0 {
		return &BulkLockResult{
			LockedFiles:  []string{},
			SkippedFiles: []string{},
			FailedFiles:  []string{},
			TotalCount:   0,
			Message:      "没有检测到需要加锁的改动文件",
		}, nil
	}

	// Fetch current locks to know what's already locked
	existingLocks, _ := s.GetLfsLocks(repoPath)
	lockOwnerMap := make(map[string]string)
	for _, l := range existingLocks {
		lockOwnerMap[filepath.Clean(l.Path)] = l.Owner
	}

	curUser, _ := ExecGit(repoPath, "config", "user.name")
	curUser = strings.TrimSpace(curUser)

	var locked []string
	var skipped []string
	var failed []string

	// Check git check-attr filter for candidates
	for _, path := range candidates {
		attrOut, err := ExecGit(repoPath, "check-attr", "filter", "--", path)
		if err != nil || !strings.Contains(attrOut, ": filter: lfs") {
			continue // Not an LFS tracked asset
		}

		cleanPath := filepath.Clean(path)
		if owner, isLocked := lockOwnerMap[cleanPath]; isLocked {
			if owner == curUser {
				skipped = append(skipped, path)
				continue
			} else {
				failed = append(failed, fmt.Sprintf("%s (已由 %s 锁定)", path, owner))
				continue
			}
		}

		// Acquire lock
		_, lockErr := ExecGit(repoPath, "lfs", "lock", path)
		if lockErr != nil {
			failed = append(failed, fmt.Sprintf("%s (%s)", path, lockErr.Error()))
		} else {
			locked = append(locked, path)
		}
	}

	msg := fmt.Sprintf("成功锁定 %d 个 LFS 资产", len(locked))
	if len(skipped) > 0 {
		msg += fmt.Sprintf("，%d 个已预先锁定", len(skipped))
	}
	if len(failed) > 0 {
		msg += fmt.Sprintf("，%d 个锁定失败或冲突", len(failed))
	}

	return &BulkLockResult{
		LockedFiles:  locked,
		SkippedFiles: skipped,
		FailedFiles:  failed,
		TotalCount:   len(locked) + len(skipped) + len(failed),
		Message:      msg,
	}, nil
}

// LfsPrune prunes Git LFS local object cache to reclaim disk space
func (s GitService) LfsPrune(repoPath string, dryRun bool, verbose bool) (*LfsPruneResult, error) {
	args := []string{"lfs", "prune"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	if verbose {
		args = append(args, "--verbose")
	}

	var before *LfsCacheStats
	if !dryRun {
		before, _ = s.InspectLfsCacheStats(repoPath)
	}

	out, err := ExecGit(repoPath, args...)
	if err != nil {
		return &LfsPruneResult{
			DryRun:  dryRun,
			Output:  out,
			Success: false,
		}, err
	}

	prunedFiles := 0
	reclaimedStr := "0 B"
	if dryRun {
		prunedFiles, reclaimedStr = parseLfsPruneSummary(out)
	} else if before != nil {
		// Plain prune only prints progress, so report the cache delta instead.
		after, _ := s.InspectLfsCacheStats(repoPath)
		if after != nil {
			if delta := before.FileCount - after.FileCount; delta > 0 {
				prunedFiles = delta
			}
			if delta := before.TotalSizeBytes - after.TotalSizeBytes; delta > 0 {
				reclaimedStr = formatBytes(delta)
			}
		}
	}

	return &LfsPruneResult{
		DryRun:       dryRun,
		Output:       out,
		PrunedFiles:  prunedFiles,
		ReclaimedStr: reclaimedStr,
		Success:      true,
	}, nil
}

var lfsPruneSummaryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(\d+) files? would be pruned \(([^)]+)\)`),
	regexp.MustCompile(`Pruning (\d+) files.*?\(([^)]+)\)`),
}

func parseLfsPruneSummary(out string) (int, string) {
	for _, re := range lfsPruneSummaryPatterns {
		if m := re.FindStringSubmatch(out); len(m) == 3 {
			n, _ := strconv.Atoi(m[1])
			return n, m[2]
		}
	}
	return 0, "0 B"
}

// GetLfsDiskUsage calculates total storage space used by .git/lfs/objects
func (GitService) GetLfsDiskUsage(repoPath string) (string, error) {
	lfsDir := lfsObjectsDir(repoPath)
	var totalBytes int64
	err := filepath.Walk(lfsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalBytes += info.Size()
		}
		return nil
	})
	if err != nil || totalBytes == 0 {
		return "0 B", nil
	}
	return formatBytes(totalBytes), nil
}

// CheckStagedNonLfsLargeFiles checks if any staged files exceed thresholdMB and are NOT managed by Git LFS
func (GitService) CheckStagedNonLfsLargeFiles(repoPath string, thresholdMB int) ([]LargeFileInfo, error) {
	if thresholdMB <= 0 {
		thresholdMB = 20 // default 20MB threshold
	}
	thresholdBytes := int64(thresholdMB) * 1024 * 1024

	out, err := ExecGit(repoPath, "diff", "--name-only", "--cached")
	if err != nil {
		return nil, fmt.Errorf("读取暂存区文件列表失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var largeFiles []LargeFileInfo

	for _, file := range lines {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		// Read the staged blob size, not the working-tree size: shrinking or
		// deleting the working copy must not hide an oversized staged object.
		sizeOut, sizeErr := ExecGit(repoPath, "cat-file", "-s", ":"+file)
		if sizeErr != nil {
			return nil, fmt.Errorf("读取暂存文件 %s 大小失败: %w", file, sizeErr)
		}
		size, parseErr := strconv.ParseInt(strings.TrimSpace(sizeOut), 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("解析暂存文件 %s 大小失败: %w", file, parseErr)
		}

		if size >= thresholdBytes {
			// Check if tracked by LFS
			attrOut, _ := ExecGit(repoPath, "check-attr", "filter", "--", file)
			if !strings.Contains(attrOut, "filter: lfs") {
				largeFiles = append(largeFiles, LargeFileInfo{
					Path:      file,
					SizeBytes: size,
					SizeHuman: formatBytes(size),
					IsLFS:     false,
				})
			}
		}
	}

	return largeFiles, nil
}

// TrackExtensionsAsLfs tracks a list of file extensions or patterns via Git LFS and stages .gitattributes
func (s GitService) TrackExtensionsAsLfs(repoPath string, extensions []string) error {
	var failed []string
	tracked := 0
	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		pattern := ext
		if !strings.HasPrefix(pattern, "*") {
			pattern = "*" + pattern
		}
		args := []string{"lfs", "track"}
		// If binary asset typical for locks, also add --lockable
		lower := strings.ToLower(pattern)
		if strings.HasSuffix(lower, ".psd") || strings.HasSuffix(lower, ".fbx") || strings.HasSuffix(lower, ".uasset") || strings.HasSuffix(lower, ".blend") || strings.HasSuffix(lower, ".max") || strings.HasSuffix(lower, ".maya") {
			args = append(args, "--lockable")
		}
		out, err := ExecGit(repoPath, append(args, "--", pattern)...)
		if err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			if idx := strings.IndexByte(detail, '\n'); idx >= 0 {
				detail = detail[:idx]
			}
			failed = append(failed, fmt.Sprintf("%s（%s）", pattern, detail))
			continue
		}
		tracked++
	}
	if tracked > 0 {
		if _, err := ExecGit(repoPath, "add", ".gitattributes"); err != nil {
			return fmt.Errorf("LFS 规则已写入但暂存 .gitattributes 失败: %w", err)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分 LFS 规则添加失败: %s", strings.Join(failed, "；"))
	}
	return nil
}

// ExportLfsLocksReport exports active LFS locks into CSV or Markdown format
func (s GitService) ExportLfsLocksReport(repoPath string, format string) (string, error) {
	locks, err := s.GetLfsLocks(repoPath)
	if err != nil {
		return "", err
	}
	now := time.Now()
	if strings.EqualFold(format, "csv") {
		return renderLfsLocksCSV(locks, now)
	}
	return renderLfsLocksMarkdown(repoPath, locks, now), nil
}

func lfsLockAge(lockedAt string, now time.Time) (int, bool) {
	t, err := time.Parse(time.RFC3339, lockedAt)
	if err != nil {
		return 0, false
	}
	held := now.Sub(t)
	if held < 0 {
		held = 0
	}
	return int(held.Hours() / 24), held > 72*time.Hour
}

func renderLfsLocksCSV(locks []LfsLockInfo, now time.Time) (string, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	if err := w.Write([]string{"Path", "Owner", "ID", "LockedAt", "DaysHeld", "IsZombie"}); err != nil {
		return "", err
	}
	for _, l := range locks {
		daysHeld, isZombie := lfsLockAge(l.LockedAt, now)
		if err := w.Write([]string{
			l.Path,
			l.Owner,
			l.ID,
			l.LockedAt,
			strconv.Itoa(daysHeld),
			strconv.FormatBool(isZombie),
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func renderLfsLocksMarkdown(repoPath string, locks []LfsLockInfo, now time.Time) string {
	var sb strings.Builder
	sb.WriteString("# Git LFS 排他锁审计报表\n\n")
	sb.WriteString(fmt.Sprintf("> 仓库路径: `%s` | 生成时间: %s | 活跃锁总计: %d\n\n",
		escapeLfsMarkdownCell(repoPath), now.Format("2006-01-02 15:04:05"), len(locks)))
	sb.WriteString("| 资产路径 | 锁定持有者 | 锁定时间 | 持有时长 (天) | 状态 |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
	for _, l := range locks {
		daysHeld, isZombie := lfsLockAge(l.LockedAt, now)
		status := "正常持有"
		if isZombie {
			status = "⚠️ 僵尸锁 (>72h 未更新)"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %d | %s |\n",
			escapeLfsMarkdownCell(l.Path),
			escapeLfsMarkdownCell(l.Owner),
			escapeLfsMarkdownCell(l.LockedAt),
			daysHeld,
			status))
	}
	return sb.String()
}

func escapeLfsMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// VerifyPrePushLfs verifies unpushed commits for LFS asset integrity and non-LFS oversized files
func (s GitService) VerifyPrePushLfs(repoPath string, remote string, branch string) (*PrePushVerificationResult, error) {
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	if branch == "" {
		curr, err := ExecGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
		if err == nil {
			branch = strings.TrimSpace(curr)
		} else {
			branch = "main"
		}
	}

	result := &PrePushVerificationResult{
		UnpushedCommits: []CommitItem{},
		MissingLfsFiles: []string{},
		OversizedBlobs:  []LargeFileInfo{},
		IsSafe:          true,
	}

	// 1. Identify unpushed range
	var revRange string
	checkUpstream, err := ExecGit(repoPath, "rev-parse", "@{u}")
	if err == nil && strings.TrimSpace(checkUpstream) != "" {
		revRange = "@{u}..HEAD"
	} else {
		_, errRemote := ExecGit(repoPath, "rev-parse", remote+"/"+branch)
		if errRemote == nil {
			revRange = remote + "/" + branch + "..HEAD"
		} else {
			revRange = "HEAD"
		}
	}

	// 2. Fetch list of unpushed commits
	args := []string{"log", "--format=%H%x00%h%x00%an%x00%ae%x00%ad%x00%cr%x00%s"}
	if revRange == "HEAD" {
		args = append(args, "-n", "30", "HEAD")
	} else {
		args = append(args, revRange)
	}

	out, err := ExecGit(repoPath, args...)
	if err == nil && strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			pts := strings.Split(line, "\x00")
			if len(pts) >= 7 {
				result.UnpushedCommits = append(result.UnpushedCommits, CommitItem{
					Hash:         pts[0],
					ShortHash:    pts[1],
					AuthorName:   pts[2],
					AuthorEmail:  pts[3],
					Date:         pts[4],
					RelativeDate: pts[5],
					Subject:      pts[6],
				})
			}
		}
	}

	if len(result.UnpushedCommits) == 0 {
		result.StatusSummary = "当前分支与远端完全同步，无待推送提交。"
		return result, nil
	}

	// 3. Scan files changed in unpushed commits
	fileMap := make(map[string]bool)
	addFiles := func(diffOut string) {
		for _, f := range strings.Split(strings.TrimSpace(diffOut), "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				fileMap[f] = true
			}
		}
	}
	if revRange == "HEAD" {
		// A root commit has no parent, so "<hash>^..HEAD" would fail and skip
		// every file; collect each commit's changes with --root instead.
		for _, c := range result.UnpushedCommits {
			diffOut, err := ExecGit(repoPath, "diff-tree", "--root", "-m", "-r", "--no-commit-id", "--name-only", c.Hash)
			if err != nil {
				return nil, fmt.Errorf("读取待推送提交 %s 的变更失败: %v", shortHashOf(c.Hash), err)
			}
			addFiles(diffOut)
		}
	} else {
		diffOut, err := ExecGit(repoPath, "diff-tree", "-r", "--no-commit-id", "--name-only", revRange)
		if err != nil {
			return nil, fmt.Errorf("读取待推送变更失败: %v", err)
		}
		addFiles(diffOut)
	}

	lfsCacheDir := lfsObjectsDir(repoPath)

	for filePath := range fileMap {
		showOut, err := ExecGit(repoPath, "show", "HEAD:"+filePath)
		if err == nil && strings.HasPrefix(showOut, "version https://git-lfs.github.com/spec/v1") {
			result.TotalLfsPointers++
			for _, line := range strings.Split(showOut, "\n") {
				if strings.HasPrefix(line, "oid sha256:") {
					sha := strings.TrimPrefix(line, "oid sha256:")
					sha = strings.TrimSpace(sha)
					if len(sha) >= 4 {
						expectedPath := filepath.Join(lfsCacheDir, sha[:2], sha[2:4], sha)
						if _, statErr := os.Stat(expectedPath); statErr != nil {
							result.MissingLfsCount++
							result.MissingLfsFiles = append(result.MissingLfsFiles, filePath)
						}
					}
					break
				}
			}
		}
	}

	// 4. Check for any non-LFS oversized files (>50MB) in the committed tree.
	// Use blob sizes from HEAD rather than the working tree: a locally edited or
	// deleted file must not hide a large blob that is about to be pushed.
	treeSizes := make(map[string]int64)
	if treeOut, treeErr := ExecGit(repoPath, "ls-tree", "-r", "-l", "HEAD"); treeErr == nil {
		for _, line := range strings.Split(treeOut, "\n") {
			meta, path, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			fields := strings.Fields(meta)
			if len(fields) < 4 {
				continue
			}
			sz, parseErr := strconv.ParseInt(fields[3], 10, 64)
			if parseErr == nil {
				treeSizes[path] = sz
			}
		}
	}
	for filePath := range fileMap {
		sz, ok := treeSizes[filePath]
		if !ok || sz < 50*1024*1024 {
			continue
		}
		result.OversizedBlobs = append(result.OversizedBlobs, LargeFileInfo{
			Path:      filePath,
			SizeBytes: sz,
			SizeHuman: formatByteSize(sz),
			IsLFS:     false,
		})
	}

	if result.MissingLfsCount > 0 || len(result.OversizedBlobs) > 0 {
		result.IsSafe = false
		result.StatusSummary = fmt.Sprintf("发现 %d 个待推送提交，存在 %d 个缺失的 LFS 资源，%d 个非 LFS 大文件",
			len(result.UnpushedCommits), result.MissingLfsCount, len(result.OversizedBlobs))
	} else {
		result.StatusSummary = fmt.Sprintf("就绪: %d 个待推送提交，%d 个 LFS 资产已全部在本地就绪，可安全推送到远端。",
			len(result.UnpushedCommits), result.TotalLfsPointers)
	}

	return result, nil
}

// InspectLfsCacheStats inspects local .git/lfs/objects cache and calculates storage statistics
func (GitService) InspectLfsCacheStats(repoPath string) (*LfsCacheStats, error) {
	cacheDir := lfsObjectsDir(repoPath)
	stats := &LfsCacheStats{
		CachePath:  cacheDir,
		Extensions: []StorageBreakdownItem{},
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		stats.TotalSizeStr = "0 B"
		return stats, nil
	}

	var totalSize int64
	var fileCount int

	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	stats.TotalSizeBytes = totalSize
	stats.TotalSizeStr = formatByteSize(totalSize)
	stats.FileCount = fileCount

	return stats, nil
}

// GetLfsFilterConfig reads configured lfs.fetchinclude and lfs.fetchexclude
func (GitService) GetLfsFilterConfig(repoPath string) (*LfsFilterConfig, error) {
	cfg := &LfsFilterConfig{
		FetchInclude: []string{},
		FetchExclude: []string{},
	}
	inc, errInc := ExecGit(repoPath, "config", "--get", "lfs.fetchinclude")
	if errInc == nil && strings.TrimSpace(inc) != "" {
		for _, s := range strings.Split(strings.TrimSpace(inc), ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.FetchInclude = append(cfg.FetchInclude, s)
			}
		}
	}
	exc, errExc := ExecGit(repoPath, "config", "--get", "lfs.fetchexclude")
	if errExc == nil && strings.TrimSpace(exc) != "" {
		for _, s := range strings.Split(strings.TrimSpace(exc), ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				cfg.FetchExclude = append(cfg.FetchExclude, s)
			}
		}
	}
	cfg.IsConfigured = len(cfg.FetchInclude) > 0 || len(cfg.FetchExclude) > 0
	return cfg, nil
}

func unsetRepoConfig(repoPath string, key string) error {
	if _, err := ExecGit(repoPath, "config", "--unset", key); err != nil {
		// Unsetting an absent key is fine; a value that remains is not.
		if current, getErr := ExecGit(repoPath, "config", "--get", key); getErr == nil && strings.TrimSpace(current) != "" {
			return fmt.Errorf("清除配置 %s 失败: %w", key, err)
		}
	}
	return nil
}

// SetLfsFilterConfig configures lfs.fetchinclude and lfs.fetchexclude
func (GitService) SetLfsFilterConfig(repoPath string, includePatterns []string, excludePatterns []string) error {
	var cleanInc []string
	for _, p := range includePatterns {
		p = strings.TrimSpace(p)
		if p != "" {
			cleanInc = append(cleanInc, p)
		}
	}
	if len(cleanInc) > 0 {
		if _, err := ExecGit(repoPath, "config", "lfs.fetchinclude", strings.Join(cleanInc, ",")); err != nil {
			return err
		}
	} else if err := unsetRepoConfig(repoPath, "lfs.fetchinclude"); err != nil {
		return err
	}

	var cleanExc []string
	for _, p := range excludePatterns {
		p = strings.TrimSpace(p)
		if p != "" {
			cleanExc = append(cleanExc, p)
		}
	}
	if len(cleanExc) > 0 {
		if _, err := ExecGit(repoPath, "config", "lfs.fetchexclude", strings.Join(cleanExc, ",")); err != nil {
			return err
		}
	} else if err := unsetRepoConfig(repoPath, "lfs.fetchexclude"); err != nil {
		return err
	}

	return nil
}

// ExecuteLfsSelectivePull executes git lfs pull with configured filters
func (GitService) ExecuteLfsSelectivePull(repoPath string) (string, error) {
	return ExecGit(repoPath, "lfs", "pull")
}

// SyncLockableFilePermissions synchronizes file permissions for lockable assets:
// files locked by current user become writable (0644), unlocked or locked by others become read-only (0444)
func (s GitService) SyncLockableFilePermissions(repoPath string) (*LockableSyncResult, error) {
	res := &LockableSyncResult{
		ProtectedFiles: []string{},
	}

	// Check if lfs.locksverify is enabled
	lv, _ := ExecGit(repoPath, "config", "--get", "lfs.locksverify")
	res.IsProtectionActive = strings.TrimSpace(lv) == "true"

	// Get current user name
	userName, _ := ExecGit(repoPath, "config", "--get", "user.name")
	userName = strings.TrimSpace(userName)

	// Fetch remote locks map
	userLockedPaths := make(map[string]bool)
	locks, _ := s.GetLfsLocks(repoPath)
	for _, l := range locks {
		if userName != "" && (l.Owner == userName || strings.Contains(l.Owner, userName)) {
			userLockedPaths[l.Path] = true
		}
	}

	// Parse .gitattributes for lockable files
	lockableExts := make(map[string]bool)
	attrContent, err := os.ReadFile(filepath.Join(repoPath, ".gitattributes"))
	if err == nil {
		for _, line := range strings.Split(string(attrContent), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "lockable") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					ext := strings.ToLower(filepath.Ext(parts[0]))
					if ext != "" {
						lockableExts[ext] = true
					}
				}
			}
		}
	}

	if len(lockableExts) == 0 {
		for _, e := range []string{".uasset", ".umap", ".fbx", ".psd", ".blend", ".max", ".maya"} {
			lockableExts[e] = true
		}
	}

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				name := info.Name()
				if name == ".git" || name == "node_modules" || name == "Library" || name == "Intermediate" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if lockableExts[ext] {
			relPath, relErr := filepath.Rel(repoPath, path)
			if relErr != nil {
				return nil
			}
			res.TotalLockableFiles++

			isOwnerLocked := userLockedPaths[relPath]
			if isOwnerLocked {
				_ = os.Chmod(path, 0644)
				res.WritableCount++
			} else {
				if res.IsProtectionActive {
					_ = os.Chmod(path, 0444)
					res.ReadOnlyCount++
					res.ProtectedFiles = append(res.ProtectedFiles, relPath)
				} else {
					_ = os.Chmod(path, 0644)
					res.WritableCount++
				}
			}
		}
		return nil
	})

	return res, nil
}

// ToggleLockableReadOnlyProtection toggles lfs.locksverify and updates file permissions
func (s GitService) ToggleLockableReadOnlyProtection(repoPath string, enable bool) (*LockableSyncResult, error) {
	val := "false"
	if enable {
		val = "true"
	}
	if _, err := ExecGit(repoPath, "config", "lfs.locksverify", val); err != nil {
		return nil, err
	}
	return s.SyncLockableFilePermissions(repoPath)
}

// GetLfsTuningConfig reads current Git LFS network transfer acceleration and buffer parameters
func (GitService) GetLfsTuningConfig(repoPath string) (*LfsTuningConfig, error) {
	cfg := &LfsTuningConfig{
		ConcurrentTransfers: 3,
		PostBufferMB:        1,
		DialTimeoutSec:      30,
		CompressionLevel:    -1,
	}

	ct, err := ExecGit(repoPath, "config", "--get", "lfs.concurrenttransfers")
	if err == nil && strings.TrimSpace(ct) != "" {
		if v, errConv := strconv.Atoi(strings.TrimSpace(ct)); errConv == nil && v > 0 {
			cfg.ConcurrentTransfers = v
		}
	}

	pb, err := ExecGit(repoPath, "config", "--get", "http.postBuffer")
	if err == nil && strings.TrimSpace(pb) != "" {
		if v, errConv := strconv.ParseInt(strings.TrimSpace(pb), 10, 64); errConv == nil && v > 0 {
			cfg.PostBufferMB = int(v / (1024 * 1024))
		}
	}

	dt, err := ExecGit(repoPath, "config", "--get", "lfs.dialtimeout")
	if err == nil && strings.TrimSpace(dt) != "" {
		if v, errConv := strconv.Atoi(strings.TrimSpace(dt)); errConv == nil && v > 0 {
			cfg.DialTimeoutSec = v
		}
	}

	cl, err := ExecGit(repoPath, "config", "--get", "core.compression")
	if err == nil && strings.TrimSpace(cl) != "" {
		if v, errConv := strconv.Atoi(strings.TrimSpace(cl)); errConv == nil {
			cfg.CompressionLevel = v
		}
	}

	cfg.IsOptimized = cfg.ConcurrentTransfers >= 8 && cfg.PostBufferMB >= 256
	if cfg.IsOptimized {
		cfg.ActiveSummary = fmt.Sprintf("已开启高速传输 (并发数: %d, 缓冲区: %dMB)", cfg.ConcurrentTransfers, cfg.PostBufferMB)
	} else {
		cfg.ActiveSummary = fmt.Sprintf("当前为系统标准默认 (并发数: %d, 缓冲区: %dMB)", cfg.ConcurrentTransfers, cfg.PostBufferMB)
	}

	return cfg, nil
}

// ApplyLfsTuningOptimization applies game studio recommended high-speed LFS and Git transfer parameters
func (s GitService) ApplyLfsTuningOptimization(repoPath string, preset string) (*LfsTuningResult, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath 不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}

	settings := [][2]string{}
	switch preset {
	case "studio_high_speed":
		settings = [][2]string{
			{"lfs.concurrenttransfers", "16"},
			{"http.postBuffer", "536870912"}, // 512MB
			{"lfs.dialtimeout", "60"},
			{"core.compression", "1"},
		}
	case "standard":
		settings = [][2]string{
			{"lfs.concurrenttransfers", "8"},
			{"http.postBuffer", "268435456"}, // 256MB
			{"lfs.dialtimeout", "30"},
		}
	case "reset_default":
		settings = [][2]string{
			{"lfs.concurrenttransfers", "3"},
			{"http.postBuffer", "1048576"},
			{"lfs.dialtimeout", "30"},
		}
	default:
		return nil, fmt.Errorf("不支持的加速预设: %s", preset)
	}
	for _, setting := range settings {
		if _, err := ExecGit(repoPath, "config", setting[0], setting[1]); err != nil {
			return nil, fmt.Errorf("写入配置 %s 失败: %w", setting[0], err)
		}
	}
	if preset == "reset_default" {
		if err := unsetRepoConfig(repoPath, "core.compression"); err != nil {
			return nil, err
		}
	}

	cfg, err := s.GetLfsTuningConfig(repoPath)
	if err != nil {
		return nil, err
	}

	msg := "已成功应用游戏研发高速网络传输配置！"
	if preset == "reset_default" {
		msg = "已重置为 Git 官方默认网络参数。"
	}

	return &LfsTuningResult{
		Success: true,
		Config:  *cfg,
		Message: msg,
	}, nil
}

// GetLfsLockActivity audits team LFS lock activity, detects zombie locks, and flags collision conflicts
func (s GitService) GetLfsLockActivity(repoPath string) (*LfsLockActivityResult, error) {
	res := &LfsLockActivityResult{
		Locks: []LfsLockActivityItem{},
	}

	userName, _ := ExecGit(repoPath, "config", "user.name")
	userEmail, _ := ExecGit(repoPath, "config", "user.email")
	userName = strings.TrimSpace(userName)
	userEmail = strings.TrimSpace(userEmail)

	locks, err := s.GetLfsLocks(repoPath)
	if err != nil {
		return nil, err
	}
	if len(locks) == 0 {
		res.Message = "当前远端暂无锁定的 LFS 资产或未连接 LFS 远端服务器。"
		return res, nil
	}

	// Also check local changes for collision check
	statusOut, statusErr := ExecGitRaw(repoPath, "status", "--porcelain=v1", "-uall")
	statusWarning := ""
	if statusErr != nil {
		statusWarning = "本地工作区状态读取失败，未执行锁冲突检查: " + statusErr.Error()
	}
	localModified := make(map[string]string)
	for _, l := range strings.Split(statusOut, "\n") {
		if len(l) >= 4 {
			p := strings.TrimSpace(l[3:])
			p = strings.Trim(p, "\"")
			code := strings.TrimSpace(l[:2])
			localModified[p] = code
		}
	}

	now := time.Now()
	for _, lock := range locks {
		lockPath := lock.Path
		lockOwner := lock.Owner

		isMe := false
		if (userName != "" && strings.EqualFold(lockOwner, userName)) ||
			(userEmail != "" && strings.EqualFold(lockOwner, userEmail)) {
			isMe = true
			res.MyLockCount++
		} else {
			res.TeamLockCount++
		}

		// Category
		lower := strings.ToLower(lockPath)
		category := "Other (其他资产)"
		if strings.Contains(lower, "character") || strings.Contains(lower, "hero") || strings.Contains(lower, "model") || strings.Contains(lower, "anim") {
			category = "Character (角色动画)"
		} else if strings.Contains(lower, "scene") || strings.Contains(lower, "map") || strings.Contains(lower, "level") || strings.Contains(lower, "env") {
			category = "Environment (关卡场景)"
		} else if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".tga") || strings.HasSuffix(lower, ".psd") || strings.HasSuffix(lower, ".mat") {
			category = "Material (材质贴图)"
		} else if strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".ogg") || strings.HasSuffix(lower, ".bank") {
			category = "Audio (音频音效)"
		}

		hasConflict := false
		localStat := "Clean (本地未修改)"
		if statCode, ok := localModified[lockPath]; ok {
			localStat = fmt.Sprintf("已修改 [%s]", statCode)
			if !isMe {
				hasConflict = true
				res.CollisionCount++
			}
		}

		durationStr, isZombie := lfsLockDuration(lock.LockedAt, now)
		if isZombie {
			res.ZombieCount++
		}

		res.TotalLocks++
		res.Locks = append(res.Locks, LfsLockActivityItem{
			Path:        lockPath,
			Owner:       lockOwner,
			IsMe:        isMe,
			LockedAt:    lock.LockedAt,
			DurationStr: durationStr,
			IsZombie:    isZombie,
			HasConflict: hasConflict,
			LocalStatus: localStat,
			Category:    category,
		})
	}

	switch {
	case res.CollisionCount > 0 && res.ZombieCount > 0:
		res.Message = fmt.Sprintf("⚠️ 警告：检测到 %d 个文件被队友锁定但本地存在修改冲突，另有 %d 个超过 72 小时的僵尸锁！", res.CollisionCount, res.ZombieCount)
	case res.CollisionCount > 0:
		res.Message = fmt.Sprintf("⚠️ 警告：检测到 %d 个文件被队友锁定，但在本地存在修改冲突风险！", res.CollisionCount)
	case res.ZombieCount > 0:
		res.Message = fmt.Sprintf("⚠️ 检测到 %d 个超过 72 小时的僵尸锁，建议联系持有人确认后解锁（共 %d 个锁定资产）。", res.ZombieCount, res.TotalLocks)
	default:
		res.Message = fmt.Sprintf("✅ 团队 LFS 锁状态健康：共 %d 个锁定资产（我的锁: %d，队友锁: %d）。", res.TotalLocks, res.MyLockCount, res.TeamLockCount)
	}

	if statusWarning != "" {
		res.Message += " " + statusWarning
	}

	return res, nil
}

func lfsLockDuration(lockedAt string, now time.Time) (string, bool) {
	t, err := time.Parse(time.RFC3339, lockedAt)
	if err != nil {
		return "持有时间未知", false
	}
	held := now.Sub(t)
	if held < 0 {
		held = 0
	}
	switch {
	case held >= 24*time.Hour:
		return fmt.Sprintf("持有 %d 天", int(held.Hours()/24)), held > 72*time.Hour
	case held >= time.Hour:
		return fmt.Sprintf("持有 %d 小时", int(held.Hours())), false
	default:
		return "刚刚锁定", false
	}
}
