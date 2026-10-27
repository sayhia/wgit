package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// =========================================================================
// 闭环补齐：把「只能看」的诊断接上「能修」的写操作。
//
// 本文件覆盖 6 组能力：
//   1. TrackStagedLargeFilesAsLfs  暂存区大文件一键转 LFS
//   2. UnlockStaleLfsLocks         僵尸 LFS 锁批量破除
//   3. GetRangeDiff                force-push 后两组提交对比
//   4. CreateFixupCommit           fixup / squash / amend 提交整理
//   5. OptimizeObjectStore         commit-graph / MIDX / repack / pack-refs
//   6. GetMergeOverlapWarning      合并前队友改动重叠预警
//   7. RunHealthFix                健康体检建议真正可执行
// =========================================================================

// -------------------------------------------------------------------------
// 1. 暂存区大文件一键转 LFS
// -------------------------------------------------------------------------

type LfsConvertItem struct {
	Path     string `json:"path"`
	Pattern  string `json:"pattern"`
	Status   string `json:"status"` // "converted" | "skipped" | "failed"
	Detail   string `json:"detail,omitempty"`
	OldBytes int64  `json:"oldBytes"`
	NewBytes int64  `json:"newBytes"`
}

type LfsConvertResult struct {
	Success       bool              `json:"success"`
	Converted     int               `json:"converted"`
	Failed        int               `json:"failed"`
	Skipped       int               `json:"skipped"`
	Items         []LfsConvertItem  `json:"items"`
	AddedPatterns []string          `json:"addedPatterns"`
	Message       string            `json:"message"`
	MessageKey    string            `json:"messageKey,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
}

// stagedBlobSize reads the size of the staged blob, not the working-tree file:
// a shrunken working copy must not hide an oversized staged object.
func stagedBlobSize(repoPath, path string) (int64, error) {
	out, err := ExecGit(repoPath, "cat-file", "-s", ":"+path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

// lfsPatternForPath derives the narrowest useful .gitattributes pattern.
// The extension case is preserved verbatim: git-lfs patterns are
// case-sensitive, and silently lowercasing "*.PNG" would fail to match.
func lfsPatternForPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if len(ext) > 1 {
		return "*" + ext
	}
	return path
}

func lfsPatternIsLockable(pattern string) bool {
	switch strings.ToLower(pattern) {
	case "*.psd", "*.fbx", "*.uasset", "*.umap", "*.blend", "*.max", "*.ma", "*.mb", "*.tga", "*.exr":
		return true
	}
	return false
}

// TrackStagedLargeFilesAsLfs converts already-staged oversized blobs into LFS
// pointers in one pass: track the pattern, drop the object from the index,
// re-add it so the clean filter rewrites it, then verify the staged size.
func (s GitService) TrackStagedLargeFilesAsLfs(repoPath string, paths []string, lockable bool) (*LfsConvertResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if len(paths) == 0 {
		return nil, errors.New("至少需要选择一个待转换文件")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if _, err := ExecGit(repoPath, "lfs", "version"); err != nil {
		return nil, errors.New("未检测到 git-lfs，请先安装：brew install git-lfs && git lfs install")
	}

	res := &LfsConvertResult{Items: []LfsConvertItem{}, AddedPatterns: []string{}}
	patternsSeen := map[string]bool{}

	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		item := LfsConvertItem{Path: path, Pattern: lfsPatternForPath(path)}

		attrOut, _ := ExecGit(repoPath, "check-attr", "filter", "--", path)
		if strings.Contains(attrOut, "filter: lfs") {
			item.Status = "skipped"
			item.Detail = "已由 LFS 跟踪"
			res.Skipped++
			res.Items = append(res.Items, item)
			continue
		}

		oldSize, err := stagedBlobSize(repoPath, path)
		if err != nil {
			item.Status = "failed"
			item.Detail = "读取暂存对象失败：" + err.Error()
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		item.OldBytes = oldSize

		trackArgs := []string{"lfs", "track"}
		if lockable && lfsPatternIsLockable(item.Pattern) {
			trackArgs = append(trackArgs, "--lockable")
		}
		trackArgs = append(trackArgs, "--", item.Pattern)
		if out, err := ExecGit(repoPath, trackArgs...); err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			item.Status = "failed"
			item.Detail = "写入 LFS 规则失败：" + firstLine(detail)
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		if !patternsSeen[item.Pattern] {
			patternsSeen[item.Pattern] = true
			res.AddedPatterns = append(res.AddedPatterns, item.Pattern)
		}

		// Drop from the index (keep the working tree), then re-add so the
		// clean filter turns the blob into a pointer. -f is required because a
		// freshly added file differs from both HEAD and the working tree.
		if _, err := ExecGit(repoPath, "rm", "--cached", "-f", "--ignore-unmatch", "--", path); err != nil {
			item.Status = "failed"
			item.Detail = "从暂存区移除失败：" + err.Error()
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		if out, err := ExecGit(repoPath, "add", "--", path); err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			item.Status = "failed"
			item.Detail = "重新暂存失败：" + firstLine(detail)
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}

		newSize, sizeErr := stagedBlobSize(repoPath, path)
		if sizeErr != nil {
			newSize = 0
		}
		item.NewBytes = newSize
		if newSize > 0 && newSize >= oldSize {
			// The filter did not run (missing `git lfs install` in this repo).
			item.Status = "failed"
			item.Detail = "暂存对象仍是原始大小，请先执行 git lfs install"
			res.Failed++
			res.Items = append(res.Items, item)
			continue
		}
		item.Status = "converted"
		item.Detail = fmt.Sprintf("%s → LFS 指针 %s", formatBytes(oldSize), formatBytes(newSize))
		res.Converted++
		res.Items = append(res.Items, item)
	}

	if len(patternsSeen) > 0 {
		if _, err := ExecGit(repoPath, "add", ".gitattributes"); err != nil {
			res.Message = "LFS 规则已写入，但暂存 .gitattributes 失败：" + err.Error()
			res.MessageKey = "closed.loop.lfs.attr.failed"
			res.Params = map[string]string{"error": err.Error()}
		}
	}

	res.Success = res.Failed == 0 && res.Converted > 0
	switch {
	case res.Converted == 0 && res.Failed == 0:
		res.Message = "所选文件均已由 LFS 跟踪，无需转换。"
		res.MessageKey = "closed.loop.lfs.noop"
	case res.Failed > 0:
		res.Message = fmt.Sprintf("转换完成 %d 个，失败 %d 个，跳过 %d 个。失败项见下方明细。", res.Converted, res.Failed, res.Skipped)
		res.MessageKey = "closed.loop.lfs.partial"
		res.Params = map[string]string{"converted": fmt.Sprint(res.Converted), "failed": fmt.Sprint(res.Failed), "skipped": fmt.Sprint(res.Skipped)}
	default:
		res.Message = fmt.Sprintf("已把 %d 个文件转为 LFS，新增 %d 条 .gitattributes 规则；提交后远端只收指针。", res.Converted, len(res.AddedPatterns))
		res.MessageKey = "closed.loop.lfs.done"
		res.Params = map[string]string{"converted": fmt.Sprint(res.Converted), "patterns": fmt.Sprint(len(res.AddedPatterns))}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 2. 僵尸 LFS 锁批量破除
// -------------------------------------------------------------------------

type LfsBulkUnlockItem struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	Age    string `json:"age"`
	Status string `json:"status"` // "unlocked" | "failed" | "skipped"
	Detail string `json:"detail,omitempty"`
}

type LfsBulkUnlockResult struct {
	Success    bool                `json:"success"`
	Unlocked   int                 `json:"unlocked"`
	Failed     int                 `json:"failed"`
	Skipped    int                 `json:"skipped"`
	Items      []LfsBulkUnlockItem `json:"items"`
	Message    string              `json:"message"`
	MessageKey string              `json:"messageKey,omitempty"`
	Params     map[string]string   `json:"params,omitempty"`
}

// UnlockStaleLfsLocks force-unlocks the given lock paths in one batch. The
// caller decides which locks are stale (the activity panel already classifies
// them), so this method stays a thin, auditable loop over `git lfs unlock`.
func (s GitService) UnlockStaleLfsLocks(repoPath string, paths []string, force bool) (*LfsBulkUnlockResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	// A selection of blanks is no selection: drop it before probing git-lfs so
	// the caller gets the real problem instead of "none unlocked".
	cleanPaths := make([]string, 0, len(paths))
	for _, raw := range paths {
		if path := strings.TrimSpace(raw); path != "" {
			cleanPaths = append(cleanPaths, path)
		}
	}
	if len(cleanPaths) == 0 {
		return nil, errors.New("没有选择要破除的锁")
	}
	if _, err := ExecGit(repoPath, "lfs", "version"); err != nil {
		return nil, errors.New("未检测到 git-lfs，无法操作远端锁")
	}

	// Enrich the result with owner/age from the live lock list so the caller
	// can show an audit trail without a second round-trip.
	ownerByPath := map[string]string{}
	ageByPath := map[string]string{}
	if activity, err := s.GetLfsLockActivity(repoPath); err == nil && activity != nil {
		for _, l := range activity.Locks {
			ownerByPath[l.Path] = l.Owner
			ageByPath[l.Path] = l.DurationStr
		}
	}

	res := &LfsBulkUnlockResult{Items: []LfsBulkUnlockItem{}}
	for _, path := range cleanPaths {
		item := LfsBulkUnlockItem{
			Path:  path,
			Owner: ownerByPath[path],
			Age:   ageByPath[path],
		}
		args := []string{"lfs", "unlock"}
		if force {
			args = append(args, "--force")
		}
		args = append(args, path)
		out, err := ExecGit(repoPath, args...)
		if err != nil {
			detail := strings.TrimSpace(out)
			if detail == "" {
				detail = err.Error()
			}
			item.Status = "failed"
			item.Detail = firstLine(detail)
			res.Failed++
		} else {
			item.Status = "unlocked"
			res.Unlocked++
		}
		res.Items = append(res.Items, item)
	}

	res.Success = res.Failed == 0 && res.Unlocked > 0
	switch {
	case res.Unlocked == 0:
		res.Message = fmt.Sprintf("没有成功破除任何锁（失败 %d 个），可能是权限不足或需要 --force。", res.Failed)
		res.MessageKey = "closed.loop.lock.none"
		res.Params = map[string]string{"failed": fmt.Sprint(res.Failed)}
	case res.Failed > 0:
		res.Message = fmt.Sprintf("已破除 %d 把锁，%d 个失败。", res.Unlocked, res.Failed)
		res.MessageKey = "closed.loop.lock.partial"
		res.Params = map[string]string{"unlocked": fmt.Sprint(res.Unlocked), "failed": fmt.Sprint(res.Failed)}
	default:
		res.Message = fmt.Sprintf("已破除 %d 把僵尸锁，文件重新可编辑。", res.Unlocked)
		res.MessageKey = "closed.loop.lock.done"
		res.Params = map[string]string{"unlocked": fmt.Sprint(res.Unlocked)}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 3. range-diff：force-push 后对比两组提交
// -------------------------------------------------------------------------

type RangeDiffEntry struct {
	OldIndex int    `json:"oldIndex"` // 0 表示「新增」（旧范围里不存在）
	NewIndex int    `json:"newIndex"` // 0 表示「移除」
	Kind     string `json:"kind"`     // "=" | "!" | "<" | ">"
	OldHash  string `json:"oldHash"`
	NewHash  string `json:"newHash"`
	Subject  string `json:"subject"`
}

type RangeDiffResult struct {
	Entries    []RangeDiffEntry  `json:"entries"`
	Equal      int               `json:"equal"`
	Changed    int               `json:"changed"`
	Added      int               `json:"added"`
	Removed    int               `json:"removed"`
	Raw        string            `json:"raw"`
	Message    string            `json:"message"`
	MessageKey string            `json:"messageKey,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

var rangeDiffLineRE = regexp.MustCompile(`^(\d+|-):\s+([0-9a-fA-F]+|-+)\s+([=!<>])\s+(\d+|-):\s+([0-9a-fA-F]+|-+)\s*(.*)$`)

func parseIndexOrZero(s string) int {
	if s == "-" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func hashOrEmpty(s string) string {
	if strings.HasPrefix(s, "-") {
		return ""
	}
	return s
}

// GetRangeDiff compares two commit ranges (e.g. a PR before and after a
// force-push) using `git range-diff`, which matches patches by content rather
// than by commit hash.
func (GitService) GetRangeDiff(repoPath string, oldBase string, oldHead string, newBase string, newHead string) (*RangeDiffResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	for name, rev := range map[string]string{"旧范围起点": oldBase, "旧范围终点": oldHead, "新范围起点": newBase, "新范围终点": newHead} {
		if strings.TrimSpace(rev) == "" {
			return nil, fmt.Errorf("%s不能为空", name)
		}
	}
	oldRange := oldBase + ".." + oldHead
	newRange := newBase + ".." + newHead

	out, err := ExecGit(repoPath, "range-diff", "--no-color", oldRange, newRange)
	if err != nil {
		return nil, fmt.Errorf("range-diff 失败: %w", err)
	}

	res := &RangeDiffResult{Entries: []RangeDiffEntry{}, Raw: out}
	for _, line := range strings.Split(out, "\n") {
		m := rangeDiffLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entry := RangeDiffEntry{
			OldIndex: parseIndexOrZero(m[1]),
			OldHash:  hashOrEmpty(m[2]),
			Kind:     m[3],
			NewIndex: parseIndexOrZero(m[4]),
			NewHash:  hashOrEmpty(m[5]),
			Subject:  strings.TrimSpace(m[6]),
		}
		switch entry.Kind {
		case "=":
			res.Equal++
		case "!":
			res.Changed++
		case "<":
			res.Removed++
		case ">":
			res.Added++
		}
		res.Entries = append(res.Entries, entry)
	}

	switch {
	case len(res.Entries) == 0:
		res.Message = "两个范围没有可比较的提交，或提交内容完全一致（无输出）。"
		res.MessageKey = "closed.loop.range.empty"
	case res.Changed == 0 && res.Added == 0 && res.Removed == 0:
		res.Message = fmt.Sprintf("%d 条提交逐一对应、内容等价（force-push 后仅 hash 变化）。", res.Equal)
		res.MessageKey = "closed.loop.range.equal"
		res.Params = map[string]string{"equal": fmt.Sprint(res.Equal)}
	default:
		res.Message = fmt.Sprintf("%d 条等价、%d 条被改写、%d 条新增、%d 条移除。", res.Equal, res.Changed, res.Added, res.Removed)
		res.MessageKey = "closed.loop.range.mixed"
		res.Params = map[string]string{"equal": fmt.Sprint(res.Equal), "changed": fmt.Sprint(res.Changed), "added": fmt.Sprint(res.Added), "removed": fmt.Sprint(res.Removed)}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 4. fixup / squash / amend
// -------------------------------------------------------------------------

type FixupCommitResult struct {
	Hash       string            `json:"hash"`
	Subject    string            `json:"subject"`
	Mode       string            `json:"mode"`
	Message    string            `json:"message"`
	MessageKey string            `json:"messageKey,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

// CreateFixupCommit turns the current staged changes into a `--fixup` (or
// `--squash`) commit pointing at an earlier commit, so that
// `git rebase -i --autosquash` folds it in automatically.
func (GitService) CreateFixupCommit(repoPath string, targetHash string, mode string) (*FixupCommitResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "fixup"
	}

	// Require staged content: an empty fixup commit is almost always a mistake.
	if _, err := ExecGit(repoPath, "diff", "--cached", "--quiet"); err == nil {
		return nil, errors.New("暂存区为空，请先暂存需要整理的改动")
	}

	var args []string
	switch mode {
	case "fixup":
		if strings.TrimSpace(targetHash) == "" {
			return nil, errors.New("fixup 需要指定目标提交")
		}
		args = []string{"commit", "--fixup=" + targetHash}
	case "squash":
		if strings.TrimSpace(targetHash) == "" {
			return nil, errors.New("squash 需要指定目标提交")
		}
		args = []string{"commit", "--squash=" + targetHash}
	case "amend":
		args = []string{"commit", "--amend", "--no-edit"}
	default:
		return nil, fmt.Errorf("不支持的整理模式: %s", mode)
	}

	if out, err := ExecGit(repoPath, args...); err != nil {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("创建整理提交失败: %s", firstLine(detail))
	}

	hash, hashErr := ExecGit(repoPath, "rev-parse", "--short", "HEAD")
	subject, subjectErr := ExecGit(repoPath, "log", "-1", "--pretty=%s")
	res := &FixupCommitResult{
		Hash:    strings.TrimSpace(hash),
		Subject: strings.TrimSpace(subject),
		Mode:    mode,
	}
	if hashErr != nil || res.Hash == "" {
		reason := "读取结果为空"
		if hashErr != nil {
			reason = firstLine(hashErr.Error())
		}
		res.Message = fmt.Sprintf("提交已创建，但读取新 HEAD 失败（%s）；请运行 git log 核对，勿直接推送。", reason)
		res.MessageKey = "closed.loop.fixup.head.failed"
		res.Params = map[string]string{"reason": reason}
		return res, nil
	}
	if subjectErr != nil {
		res.Message = fmt.Sprintf("已创建 %s 提交 %s，但读取提交标题失败；请运行 git log 核对。", mode, res.Hash)
		res.MessageKey = "closed.loop.fixup.subject.failed"
		res.Params = map[string]string{"mode": mode, "hash": res.Hash}
		return res, nil
	}
	switch mode {
	case "amend":
		res.Message = "已并入上一个提交：" + res.Hash
		res.MessageKey = "closed.loop.fixup.amended"
		res.Params = map[string]string{"hash": res.Hash}
	default:
		res.Message = fmt.Sprintf("已创建 %s 提交 %s；交互式变基时勾选 autosquash 会自动折叠进目标提交。", mode, res.Hash)
		res.MessageKey = "closed.loop.fixup.created"
		res.Params = map[string]string{"mode": mode, "hash": res.Hash}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 5. 对象库优化
// -------------------------------------------------------------------------

type ObjectStoreTaskResult struct {
	Task       string `json:"task"`
	Label      string `json:"label"`
	Success    bool   `json:"success"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skipReason,omitempty"`
	Output     string `json:"output"`
	DurationMs int64  `json:"durationMs"`
}

type ObjectStoreOptimizeResult struct {
	Success      bool                    `json:"success"`
	Results      []ObjectStoreTaskResult `json:"results"`
	GitDirBefore string                  `json:"gitDirBefore"`
	GitDirAfter  string                  `json:"gitDirAfter"`
	Message      string                  `json:"message"`
	MessageKey   string                  `json:"messageKey,omitempty"`
	Params       map[string]string       `json:"params,omitempty"`
}

func dirSizeBytes(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// OptimizeObjectStore runs the object-database tuning passes that make large
// repositories fast: commit-graph (skip commit traversal), multi-pack-index
// (skip pack scans), repack (consolidate packs), pack-refs (flatten loose refs).
func (GitService) OptimizeObjectStore(repoPath string, tasks []string) (*ObjectStoreOptimizeResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	if len(tasks) == 0 {
		tasks = []string{"commit-graph", "multi-pack-index", "repack", "pack-refs"}
	}

	gitDir := gitCommonDir(repoPath)
	res := &ObjectStoreOptimizeResult{Results: []ObjectStoreTaskResult{}}
	res.GitDirBefore = formatBytes(dirSizeBytes(gitDir))

	run := func(task, label string, timeout time.Duration, args ...string) {
		start := time.Now()
		out, err := ExecGitWithTimeout(repoPath, timeout, args...)
		item := ObjectStoreTaskResult{
			Task:       task,
			Label:      label,
			Success:    err == nil,
			Output:     strings.TrimSpace(out),
			DurationMs: time.Since(start).Milliseconds(),
		}
		if err != nil {
			detail := item.Output
			if detail == "" {
				detail = err.Error()
			}
			item.Output = firstLine(detail)
		}
		res.Results = append(res.Results, item)
	}

	for _, raw := range tasks {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "commit-graph":
			run("commit-graph", "提交图索引", 10*time.Minute,
				"commit-graph", "write", "--reachable")
		case "multi-pack-index":
			run("multi-pack-index", "多包索引 MIDX", 5*time.Minute,
				"multi-pack-index", "write")
		case "repack":
			run("repack", "重打包对象", 30*time.Minute,
				"-c", "pack.windowMemory=1g", "-c", "pack.packSizeLimit=2g",
				"repack", "-adf", "--quiet")
		case "pack-refs":
			run("pack-refs", "压缩引用", 5*time.Minute,
				"pack-refs", "--all", "--prune")
		case "prune":
			run("prune", "清理不可达对象", 10*time.Minute,
				"prune", "--expire=now")
		default:
			res.Results = append(res.Results, ObjectStoreTaskResult{
				Task:       raw,
				Label:      raw,
				Skipped:    true,
				SkipReason: "未知任务",
			})
		}
	}

	res.GitDirAfter = formatBytes(dirSizeBytes(gitDir))
	okCount := 0
	failCount := 0
	for _, r := range res.Results {
		switch {
		case r.Skipped:
		case r.Success:
			okCount++
		default:
			failCount++
		}
	}
	res.Success = failCount == 0
	if failCount > 0 {
		res.Message = fmt.Sprintf("%d 项完成，%d 项失败。.git 体积 %s → %s。", okCount, failCount, res.GitDirBefore, res.GitDirAfter)
		res.MessageKey = "closed.loop.optimize.partial"
		res.Params = map[string]string{"ok": fmt.Sprint(okCount), "fail": fmt.Sprint(failCount), "before": res.GitDirBefore, "after": res.GitDirAfter}
	} else {
		res.Message = fmt.Sprintf("%d 项优化全部完成，.git 体积 %s → %s。大仓的 git log / blame 会明显变快。", okCount, res.GitDirBefore, res.GitDirAfter)
		res.MessageKey = "closed.loop.optimize.done"
		res.Params = map[string]string{"ok": fmt.Sprint(okCount), "before": res.GitDirBefore, "after": res.GitDirAfter}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 6. 合并前队友改动重叠预警
// -------------------------------------------------------------------------

type OverlapFile struct {
	Path     string   `json:"path"`
	Authors  []string `json:"authors"`
	LastTime string   `json:"lastTime"`
	MyChange bool     `json:"myChange"`
}

type MergeOverlapResult struct {
	OverlapFiles  []OverlapFile `json:"overlapFiles"`
	MyFileCount   int           `json:"myFileCount"`
	TeamFileCount int           `json:"teamFileCount"`
	Authors       []string      `json:"authors"`
	RiskLevel     string        `json:"riskLevel"` // "none" | "low" | "medium" | "high"
	Message       string        `json:"message"`
	// Catalog key + params for `Message`, so the UI can render it in its own
	// language and fall back to the text above.
	MessageKey    string            `json:"messageKey,omitempty"`
	MessageParams map[string]string `json:"messageParams,omitempty"`
}

// GetMergeOverlapWarning lists files touched by BOTH my branch and the target
// branch in the last N days, plus who last touched them. Overlap is the single
// best predictor of merge pain, and unlike a dry-run merge it also names the
// teammate you need to talk to.
func (GitService) GetMergeOverlapWarning(repoPath string, targetBranch string, days int) (*MergeOverlapResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if strings.TrimSpace(targetBranch) == "" {
		targetBranch = "HEAD"
	}
	if days <= 0 {
		days = 7
	}

	base, err := ExecGit(repoPath, "merge-base", "HEAD", targetBranch)
	if err != nil {
		return nil, fmt.Errorf("找不到共同祖先: %w", err)
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("两个分支没有共同祖先，无法比较")
	}

	collectFiles := func(args ...string) (map[string]bool, error) {
		out, err := ExecGit(repoPath, args...)
		if err != nil {
			return nil, err
		}
		set := map[string]bool{}
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				set[line] = true
			}
		}
		return set, nil
	}

	mine, err := collectFiles("diff", "--name-only", base+"..HEAD")
	if err != nil {
		return nil, fmt.Errorf("读取本分支改动失败: %w", err)
	}
	theirs, err := collectFiles("log", "--since="+strconv.Itoa(days)+".days", "--name-only",
		"--pretty=format:", base+".."+targetBranch)
	if err != nil {
		return nil, fmt.Errorf("读取目标分支近期改动失败: %w", err)
	}

	res := &MergeOverlapResult{OverlapFiles: []OverlapFile{}, Authors: []string{}}
	res.MyFileCount = len(mine)
	res.TeamFileCount = len(theirs)

	overlap := []string{}
	for path := range mine {
		if theirs[path] {
			overlap = append(overlap, path)
		}
	}
	sort.Strings(overlap)

	authorSet := map[string]bool{}
	for _, path := range overlap {
		meta, err := ExecGit(repoPath, "log", "-1", "--format=%an%x09%ar",
			base+".."+targetBranch, "--", path)
		if err != nil {
			return nil, fmt.Errorf("读取重叠文件 %s 的修改人失败: %w", path, err)
		}
		author := ""
		when := ""
		if parts := strings.SplitN(strings.TrimSpace(meta), "\t", 2); len(parts) == 2 {
			author = strings.TrimSpace(parts[0])
			when = strings.TrimSpace(parts[1])
		}
		entry := OverlapFile{Path: path, Authors: []string{}, LastTime: when, MyChange: true}
		if author != "" {
			entry.Authors = append(entry.Authors, author)
			authorSet[author] = true
		}
		res.OverlapFiles = append(res.OverlapFiles, entry)
	}
	for a := range authorSet {
		res.Authors = append(res.Authors, a)
	}
	sort.Strings(res.Authors)

	// `", "` keeps the joined lists readable in both languages.
	switch {
	case len(overlap) == 0:
		res.RiskLevel = "none"
		res.Message = fmt.Sprintf("你的 %d 个改动文件与 %s 近 %d 天的改动没有交集，合并风险低。", res.MyFileCount, targetBranch, days)
		res.MessageKey = "merge.overlap.message.none"
		res.MessageParams = map[string]string{
			"mine":   strconv.Itoa(res.MyFileCount),
			"target": targetBranch,
			"days":   strconv.Itoa(days),
		}
	case len(overlap) <= 2:
		res.RiskLevel = "low"
		res.Message = fmt.Sprintf("%d 个文件双方都在改：%s。合并前建议先对齐。", len(overlap), strings.Join(overlap, "、"))
		res.MessageKey = "merge.overlap.message.low"
		res.MessageParams = map[string]string{
			"count": strconv.Itoa(len(overlap)),
			"files": strings.Join(overlap, ", "),
		}
	case len(overlap) <= 5:
		res.RiskLevel = "medium"
		res.Message = fmt.Sprintf("%d 个文件双方都在改，涉及 %s。建议先 rebase 到最新再评审。", len(overlap), strings.Join(res.Authors, "、"))
		res.MessageKey = "merge.overlap.message.medium"
		res.MessageParams = map[string]string{
			"count":   strconv.Itoa(len(overlap)),
			"authors": strings.Join(res.Authors, ", "),
		}
	default:
		res.RiskLevel = "high"
		res.Message = fmt.Sprintf("%d 个文件双方都在改（涉及 %d 人）。这是高冲突区，建议拆分提交或提前沟通。", len(overlap), len(res.Authors))
		res.MessageKey = "merge.overlap.message.high"
		res.MessageParams = map[string]string{
			"count":  strconv.Itoa(len(overlap)),
			"people": strconv.Itoa(len(res.Authors)),
		}
	}
	return res, nil
}

// -------------------------------------------------------------------------
// 7. 健康体检建议真正可执行
// -------------------------------------------------------------------------

type HealthFixResult struct {
	Key        string `json:"key"`
	Executed   bool   `json:"executed"`
	Success    bool   `json:"success"`
	NeedsInput bool   `json:"needsInput"`
	Target     string `json:"target"` // UI 跳转目标：settings / lfs / conflict / ignore
	Output     string `json:"output"`
	Hint       string `json:"hint"`
}

// RunHealthFix executes the remediation behind a health-check dimension.
// Dimensions that need a human decision (identity, signing, conflict) return
// NeedsInput so the UI can jump to the right panel instead of pretending to fix.
func (s GitService) RunHealthFix(repoPath string, key string) (*HealthFixResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
	}
	res := &HealthFixResult{Key: key}

	switch key {
	case "identity", "signing":
		res.NeedsInput = true
		res.Target = "settings"
		res.Hint = "需要你填写用户名 / 邮箱或签名密钥，已为你定位到设置页。"
		return res, nil
	case "workspace":
		res.NeedsInput = true
		res.Target = "conflict"
		res.Hint = "存在未解决冲突或中断操作，请在工作区继续处理。"
		return res, nil
	case "largefiles":
		res.NeedsInput = true
		res.Target = "lfs"
		res.Hint = "大文件需要你确认跟踪范围，已定位到 LFS 面板；可先用「暂存区大文件一键转 LFS」。"
		return res, nil
	case "objects":
		// Connectivity fsck first (cheap), then a conservative gc only when clean.
		fsckOut, fsckErr := ExecGitWithTimeout(repoPath, 10*time.Minute,
			"fsck", "--full", "--no-progress")
		res.Executed = true
		if fsckErr != nil {
			res.Output = firstLine(fsckOut)
			if res.Output == "" {
				res.Output = fsckErr.Error()
			}
			res.Hint = "fsck 报告异常，建议先备份仓库（可用「离线 Bundle」导出）再深入排查。"
			return res, nil
		}
		gcOut, gcErr := ExecGitWithTimeout(repoPath, 30*time.Minute, "gc", "--prune=now")
		res.Success = gcErr == nil
		res.Output = strings.TrimSpace(gcOut)
		if gcErr != nil {
			res.Output = firstLine(gcErr.Error())
			res.Hint = "gc 执行失败，对象库可能被其它进程占用。"
		} else {
			res.Hint = "已完成 git fsck --full 与 gc --prune=now，不可达对象已清理。"
		}
		return res, nil
	case "ignore":
		if _, err := ExecGit(repoPath, "rev-parse", "--git-dir"); err != nil {
			return nil, fmt.Errorf("不是有效的 Git 仓库: %w", err)
		}
		audit, err := s.AuditIgnoreRules(repoPath)
		if err != nil {
			return nil, fmt.Errorf("忽略规则体检失败: %w", err)
		}
		if audit == nil || len(audit.MissingRules) == 0 && audit.TrackedCount == 0 {
			res.Success = true
			res.Hint = "忽略规则已齐备，无需修复。"
			return res, nil
		}

		// The fix rewrites .gitignore and may untrack files, so snapshot the
		// files it can touch before writing; a failed backup aborts the fix.
		backupTargets := []string{".gitignore", ".gitattributes"}
		backupDir, backed := backupRepoFiles(repoPath, backupTargets)
		backedSet := make(map[string]bool, len(backed))
		for _, path := range backed {
			backedSet[path] = true
		}
		for _, rel := range backupTargets {
			_, statErr := os.Stat(filepath.Join(repoPath, rel))
			if statErr == nil && !backedSet[rel] {
				res.Output = "无法备份 " + rel + "，已中止修复。"
				res.Hint = "修复前必须先备份现有忽略规则文件，请检查 .git/wgit-fix-backup 的写入权限后重试。"
				return res, nil
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				res.Output = "无法检查 " + rel + "，已中止修复：" + statErr.Error()
				res.Hint = "忽略规则文件状态未知，未做任何修改。"
				return res, nil
			}
		}

		action, err := s.ApplyIgnoreSanitizer(repoPath, audit.EngineType, true)
		res.Executed = true
		if err != nil {
			res.Output = err.Error()
			res.Hint = "自动修复失败，可在「工作区清理」页手动应用模板。"
			return res, nil
		}
		if action == nil {
			res.Output = "忽略规则修复未返回结果，无法确认是否已生效。"
			res.Hint = "修复结果缺失，可在「工作区清理」页手动应用模板后重试。"
			return res, nil
		}
		res.Success = action.Success
		res.Output = fmt.Sprintf("新增 %d 条规则，取消跟踪 %d 个构建产物", len(action.RulesAdded), len(action.UntrackedFiles))
		if len(action.FailedFiles) > 0 {
			res.Output += fmt.Sprintf("，%d 个移出失败", len(action.FailedFiles))
		}
		if backupDir != "" {
			res.Output += "；原文件已备份到 " + backupDir
		}
		if res.Success {
			res.Hint = "已补全忽略规则并取消跟踪误提交的构建产物。"
		} else {
			res.Hint = fmt.Sprintf("有 %d 个构建产物移出失败，可在「工作区清理」页重试，或手动执行 git rm --cached 处理。", len(action.FailedFiles))
		}
		return res, nil
	default:
		return nil, fmt.Errorf("不支持的体检项: %s", key)
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

// -------------------------------------------------------------------------
// 8. 标签签名校验（此前只有 verify-commit，没有 verify-tag）
// -------------------------------------------------------------------------

type TagVerifyResult struct {
	Tag        string            `json:"tag"`
	Verified   bool              `json:"verified"`
	Signer     string            `json:"signer"`
	Detail     string            `json:"detail"`
	Message    string            `json:"message"`
	MessageKey string            `json:"messageKey,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
}

// VerifyTagSignature checks a release tag's GPG/SSH signature. Release tags
// are the one artifact users trust blindly, so this closes the gap where only
// commits were verified.
func (GitService) VerifyTagSignature(repoPath string, tagName string) (*TagVerifyResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, errors.New("repository path is required")
	}
	if strings.TrimSpace(tagName) == "" {
		return nil, errors.New("标签名不能为空")
	}
	if _, err := ExecGit(repoPath, "rev-parse", "--verify", "refs/tags/"+tagName); err != nil {
		return nil, fmt.Errorf("标签 %s 不存在", tagName)
	}

	// A lightweight tag points straight at a commit and therefore carries no
	// signature object at all — verifying it would just produce a confusing
	// gpg error.
	objType, err := ExecGit(repoPath, "cat-file", "-t", "refs/tags/"+tagName)
	if err != nil {
		return nil, fmt.Errorf("读取标签 %s 的对象类型失败: %w", tagName, err)
	}
	if strings.TrimSpace(objType) != "tag" {
		return &TagVerifyResult{
			Tag:        tagName,
			Message:    "轻量标签没有签名对象，只有附注标签才可能带签名。",
			MessageKey: "closed.loop.tag.lightweight",
		}, nil
	}

	res := &TagVerifyResult{Tag: tagName}
	out, err := ExecGit(repoPath, "verify-tag", tagName)
	detail := strings.TrimSpace(out)
	if err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	res.Detail = detail
	res.Verified = err == nil

	for _, line := range strings.Split(detail, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Good signature from") {
			if start := strings.Index(line, "\""); start >= 0 {
				rest := line[start+1:]
				if end := strings.Index(rest, "\""); end > 0 {
					res.Signer = rest[:end]
				}
			}
		}
	}

	switch {
	case res.Verified && res.Signer != "":
		res.Message = "标签签名有效，签名者：" + res.Signer
		res.MessageKey = "closed.loop.tag.valid.signer"
		res.Params = map[string]string{"signer": res.Signer}
	case res.Verified:
		res.Message = "标签签名校验通过。"
		res.MessageKey = "closed.loop.tag.valid"
	default:
		res.Message = "标签签名校验失败："
		if detail == "" {
			res.Message += "该标签未签名或本地缺少公钥。"
			res.MessageKey = "closed.loop.tag.invalid.nokey"
		} else {
			res.Message += firstLine(detail)
			res.MessageKey = "closed.loop.tag.invalid"
			res.Params = map[string]string{"error": firstLine(detail)}
		}
	}
	return res, nil
}
