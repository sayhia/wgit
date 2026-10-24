package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Change review turns "compare two refs" into a trackable review session:
// every changed file carries a decision (pending / approved / needs-change),
// comments are anchored to a path + line, and the whole session is persisted
// inside .git/wgit-review so it never pollutes the working tree or history.

type ReviewFileState struct {
	Path             string `json:"path"`
	Status           string `json:"status"` // pending | approved | changes
	ChangeType       string `json:"changeType"`
	Note             string `json:"note"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Category         string `json:"category"`
	IsBinary         bool   `json:"isBinary"`
	UpdatedAt        string `json:"updatedAt"`
	CommentCount     int    `json:"commentCount"`
	OpenCommentCount int    `json:"openCommentCount"`
}

type ReviewComment struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side"` // new | old
	Body      string `json:"body"`
	Author    string `json:"author"`
	CreatedAt string `json:"createdAt"`
	Resolved  bool   `json:"resolved"`
}

type ReviewCommitItem struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"shortHash"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	Subject   string `json:"subject"`
}

type ChangeReviewStats struct {
	Total         int    `json:"total"`
	Approved      int    `json:"approved"`
	Changes       int    `json:"changes"`
	Pending       int    `json:"pending"`
	Comments      int    `json:"comments"`
	OpenComments  int    `json:"openComments"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	Progress      int    `json:"progress"`
	Verdict       string `json:"verdict"`
	VerdictDetail string `json:"verdictDetail"`
}

type ChangeReview struct {
	Base      string             `json:"base"`
	Head      string             `json:"head"`
	BaseLabel string             `json:"baseLabel"`
	HeadLabel string             `json:"headLabel"`
	Key       string             `json:"key"`
	CreatedAt string             `json:"createdAt"`
	UpdatedAt string             `json:"updatedAt"`
	Summary   string             `json:"summary"`
	Ready     bool               `json:"ready"`
	Files     []ReviewFileState  `json:"files"`
	Comments  []ReviewComment    `json:"comments"`
	Commits   []ReviewCommitItem `json:"commits"`
	Stats     ChangeReviewStats  `json:"stats"`
}

type ReviewSessionSummary struct {
	Key           string `json:"key"`
	Base          string `json:"base"`
	Head          string `json:"head"`
	UpdatedAt     string `json:"updatedAt"`
	Total         int    `json:"total"`
	Approved      int    `json:"approved"`
	Changes       int    `json:"changes"`
	Pending       int    `json:"pending"`
	Comments      int    `json:"comments"`
	OpenComments  int    `json:"openComments"`
	Additions     int    `json:"additions"`
	Deletions     int    `json:"deletions"`
	Ready         bool   `json:"ready"`
	Verdict       string `json:"verdict"`
	VerdictDetail string `json:"verdictDetail"`
}

// reviewFile is the on-disk payload: only the human decisions are stored,
// everything derived from git is recomputed on every read.
type reviewFile struct {
	Base      string                    `json:"base"`
	Head      string                    `json:"head"`
	CreatedAt string                    `json:"createdAt"`
	UpdatedAt string                    `json:"updatedAt"`
	Decisions map[string]reviewDecision `json:"decisions"`
	Comments  []ReviewComment           `json:"comments"`
}

type reviewDecision struct {
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updatedAt"`
}

const reviewDirName = "wgit-review"

var reviewStatuses = map[string]bool{
	"pending":  true,
	"approved": true,
	"changes":  true,
}

func reviewStoreDir(repoPath string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	if !IsGitRepo(repoPath) {
		return "", errors.New("当前目录不是 Git 仓库")
	}
	gitDir, err := ExecGit(repoPath, "rev-parse", "--absolute-git-dir")
	if err != nil || strings.TrimSpace(gitDir) == "" {
		return "", errors.New("无法定位 .git 目录")
	}
	trimmedGitDir := strings.TrimSpace(gitDir)
	newDir := filepath.Join(trimmedGitDir, reviewDirName)
	oldDir := filepath.Join(trimmedGitDir, "ugit-review")
	// If newDir doesn't exist but oldDir does, automatically migrate
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		if _, errOld := os.Stat(oldDir); errOld == nil {
			_ = os.Rename(oldDir, newDir)
		}
	}
	return newDir, nil
}

func reviewKey(base string, head string) string {
	sanitize := func(s string) string {
		var b strings.Builder
		for _, r := range strings.TrimSpace(s) {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				b.WriteRune(r)
			case r == '-' || r == '_' || r == '.':
				b.WriteRune(r)
			default:
				b.WriteRune('_')
			}
		}
		out := b.String()
		if len(out) > 48 {
			out = out[:48]
		}
		if out == "" {
			out = "ref"
		}
		return out
	}
	return sanitize(base) + "__" + sanitize(head)
}

func reviewPathFor(repoPath string, base string, head string) (string, error) {
	dir, err := reviewStoreDir(repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, reviewKey(base, head)+".json"), nil
}

func loadReviewFile(path string) (reviewFile, error) {
	data := reviewFile{Decisions: map[string]reviewDecision{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, fmt.Errorf("读取评审会话失败: %w", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return reviewFile{Decisions: map[string]reviewDecision{}},
			fmt.Errorf("评审会话文件 %s 已损坏，已拒绝覆盖；请备份或删除后重试: %w", filepath.Base(path), err)
	}
	if data.Decisions == nil {
		data.Decisions = map[string]reviewDecision{}
	}
	return data, nil
}

// resolveReviewRef makes sure a ref actually exists before we diff against it,
// so typos surface as a readable error instead of an empty review.
func resolveReviewRef(repoPath string, ref string) (string, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", errors.New("请填写基准与对比引用")
	}
	if out, err := ExecGit(repoPath, "rev-parse", "--short", ref); err == nil && strings.TrimSpace(out) != "" {
		return ref, strings.TrimSpace(out), nil
	}
	return "", "", fmt.Errorf("引用 %q 在本仓库中不存在", ref)
}

// GetChangeReview rebuilds the review session for base...head and merges in
// every decision and comment that has been saved for it.
func (GitService) GetChangeReview(repoPath string, base string, head string) (*ChangeReview, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	baseRef, baseShort, err := resolveReviewRef(repoPath, base)
	if err != nil {
		return nil, err
	}
	headRef, headShort, err := resolveReviewRef(repoPath, head)
	if err != nil {
		return nil, err
	}
	if baseRef == headRef {
		return nil, errors.New("基准与对比引用相同，没有可评审的改动")
	}

	storePath, err := reviewPathFor(repoPath, baseRef, headRef)
	if err != nil {
		return nil, err
	}
	stored, err := loadReviewFile(storePath)
	if err != nil {
		return nil, err
	}

	res := &ChangeReview{
		Base:      baseRef,
		Head:      headRef,
		BaseLabel: baseShort,
		HeadLabel: headShort,
		Key:       reviewKey(baseRef, headRef),
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
		Files:     []ReviewFileState{},
		Comments:  stored.Comments,
		Commits:   []ReviewCommitItem{},
	}
	if res.Comments == nil {
		res.Comments = []ReviewComment{}
	}
	if res.CreatedAt == "" {
		res.CreatedAt = time.Now().Format("2006-01-02 15:04")
	}

	// --- changed files -----------------------------------------------------
	rangeArg := baseRef + "..." + headRef
	nameStatus, err := ExecGit(repoPath, "diff", "--name-status", "--find-renames", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("读取变更文件失败: %v", err)
	}
	numstat, err := ExecGit(repoPath, "diff", "--numstat", "--find-renames", rangeArg)
	if err != nil {
		return nil, fmt.Errorf("读取变更统计失败: %v", err)
	}

	addDel := map[string][2]int{}
	binary := map[string]bool{}
	for _, line := range strings.Split(numstat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		p := strings.TrimSpace(parts[2])
		if parts[0] == "-" || parts[1] == "-" {
			binary[p] = true
			continue
		}
		a, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		d, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		addDel[p] = [2]int{a, d}
	}

	for _, line := range strings.Split(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		changeType := strings.TrimSpace(fields[0])
		path := strings.TrimSpace(fields[len(fields)-1])
		if path == "" {
			continue
		}
		item := ReviewFileState{
			Path:       path,
			ChangeType: changeType[:1],
			Category:   shortAssetCategory(path),
			IsBinary:   binary[path],
			Status:     "pending",
		}
		if v, ok := addDel[path]; ok {
			item.Additions = v[0]
			item.Deletions = v[1]
		}
		if d, ok := stored.Decisions[path]; ok && reviewStatuses[d.Status] {
			item.Status = d.Status
			item.Note = d.Note
			item.UpdatedAt = d.UpdatedAt
		}
		res.Files = append(res.Files, item)
	}
	sort.SliceStable(res.Files, func(i, j int) bool {
		return res.Files[i].Path < res.Files[j].Path
	})

	// --- commits in range --------------------------------------------------
	logOut, err := ExecGit(repoPath, "log", "--no-merges", "--date=short",
		"--format=%H%x1f%h%x1f%an%x1f%ad%x1f%s", rangeArg)
	if err == nil {
		for _, line := range strings.Split(logOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			f := strings.Split(line, "\x1f")
			if len(f) < 5 {
				continue
			}
			res.Commits = append(res.Commits, ReviewCommitItem{
				Hash: f[0], ShortHash: f[1], Author: f[2], Date: f[3], Subject: f[4],
			})
		}
	}

	// --- comment rollup ----------------------------------------------------
	byPath := map[string][2]int{}
	for _, c := range res.Comments {
		v := byPath[c.Path]
		v[0]++
		if !c.Resolved {
			v[1]++
		}
		byPath[c.Path] = v
	}
	for i := range res.Files {
		v := byPath[res.Files[i].Path]
		res.Files[i].CommentCount = v[0]
		res.Files[i].OpenCommentCount = v[1]
	}

	recomputeReviewStats(res)
	if len(res.Files) == 0 {
		res.Summary = fmt.Sprintf("%s 与 %s 之间没有文件改动", res.BaseLabel, res.HeadLabel)
	}
	return res, nil
}

func recomputeReviewStats(res *ChangeReview) {
	st := ChangeReviewStats{Total: len(res.Files)}
	for _, f := range res.Files {
		st.Additions += f.Additions
		st.Deletions += f.Deletions
		switch f.Status {
		case "approved":
			st.Approved++
		case "changes":
			st.Changes++
		default:
			st.Pending++
		}
	}
	st.Comments = len(res.Comments)
	for _, c := range res.Comments {
		if !c.Resolved {
			st.OpenComments++
		}
	}
	if st.Total > 0 {
		st.Progress = (st.Approved + st.Changes) * 100 / st.Total
	}
	res.Ready = st.Total > 0 && st.Pending == 0 && st.OpenComments == 0
	switch {
	case st.Total == 0:
		st.Verdict = "无改动"
		st.VerdictDetail = "所选范围没有文件变更"
	case st.OpenComments > 0 || st.Changes > 0:
		st.Verdict = "需修改"
		st.VerdictDetail = fmt.Sprintf("%d 个文件标记需修改，%d 条评论待处理", st.Changes, st.OpenComments)
	case st.Pending > 0:
		st.Verdict = "评审中"
		st.VerdictDetail = fmt.Sprintf("还有 %d 个文件未给出结论", st.Pending)
	default:
		st.Verdict = "可以合并"
		st.VerdictDetail = fmt.Sprintf("%d 个文件全部通过", st.Total)
	}
	if res.Summary == "" {
		res.Summary = fmt.Sprintf("%s → %s：%d 个文件，+%d/-%d，结论「%s」",
			res.BaseLabel, res.HeadLabel, st.Total, st.Additions, st.Deletions, st.Verdict)
	}
	res.Stats = st
}

func saveReviewFile(path string, data reviewFile, createdAt string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	now := time.Now().Format("2006-01-02 15:04")
	if createdAt != "" {
		data.CreatedAt = createdAt
	} else if data.CreatedAt == "" {
		data.CreatedAt = now
	}
	data.UpdatedAt = now
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// SetReviewFileState records the reviewer's decision for a single file.
func (GitService) SetReviewFileState(repoPath string, base string, head string, path string, status string, note string) (*ChangeReview, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("请先选择一个文件")
	}
	if !reviewStatuses[status] {
		return nil, fmt.Errorf("未知的评审状态: %s", status)
	}
	storePath, err := reviewPathFor(repoPath, base, head)
	if err != nil {
		return nil, err
	}
	stored, err := loadReviewFile(storePath)
	if err != nil {
		return nil, err
	}
	stored.Base = base
	stored.Head = head
	stored.Decisions[path] = reviewDecision{
		Status:    status,
		Note:      strings.TrimSpace(note),
		UpdatedAt: time.Now().Format("2006-01-02 15:04"),
	}
	if err := saveReviewFile(storePath, stored, ""); err != nil {
		return nil, err
	}
	return GitService{}.GetChangeReview(repoPath, base, head)
}

// AddReviewComment attaches a comment to a file (optionally a line).
func (GitService) AddReviewComment(repoPath string, base string, head string, path string, line int, side string, body string) (*ChangeReview, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("请先选择一个文件")
	}
	side = strings.TrimSpace(side)
	if side != "old" {
		side = "new"
	}
	storePath, err := reviewPathFor(repoPath, base, head)
	if err != nil {
		return nil, err
	}
	stored, err := loadReviewFile(storePath)
	if err != nil {
		return nil, err
	}
	stored.Base = base
	stored.Head = head
	author := strings.TrimSpace(firstNonEmptyLine(mustExec(repoPath, "config", "user.name")))
	if author == "" {
		author = "reviewer"
	}
	stored.Comments = append(stored.Comments, ReviewComment{
		ID:        fmt.Sprintf("c%d", time.Now().UnixNano()),
		Path:      path,
		Line:      line,
		Side:      side,
		Body:      body,
		Author:    author,
		CreatedAt: time.Now().Format("2006-01-02 15:04"),
	})
	if err := saveReviewFile(storePath, stored, ""); err != nil {
		return nil, err
	}
	return GitService{}.GetChangeReview(repoPath, base, head)
}

// ResolveReviewComment marks a comment resolved / reopened.
func (GitService) ResolveReviewComment(repoPath string, base string, head string, id string, resolved bool) (*ChangeReview, error) {
	storePath, err := reviewPathFor(repoPath, base, head)
	if err != nil {
		return nil, err
	}
	stored, err := loadReviewFile(storePath)
	if err != nil {
		return nil, err
	}
	found := false
	for i := range stored.Comments {
		if stored.Comments[i].ID == id {
			stored.Comments[i].Resolved = resolved
			found = true
		}
	}
	if !found {
		return nil, errors.New("未找到该评论")
	}
	if err := saveReviewFile(storePath, stored, ""); err != nil {
		return nil, err
	}
	return GitService{}.GetChangeReview(repoPath, base, head)
}

// DeleteReviewComment removes a comment.
func (GitService) DeleteReviewComment(repoPath string, base string, head string, id string) (*ChangeReview, error) {
	storePath, err := reviewPathFor(repoPath, base, head)
	if err != nil {
		return nil, err
	}
	stored, err := loadReviewFile(storePath)
	if err != nil {
		return nil, err
	}
	kept := []ReviewComment{}
	for _, c := range stored.Comments {
		if c.ID != id {
			kept = append(kept, c)
		}
	}
	stored.Comments = kept
	if err := saveReviewFile(storePath, stored, ""); err != nil {
		return nil, err
	}
	return GitService{}.GetChangeReview(repoPath, base, head)
}

// ClearChangeReview drops every decision and comment of a session.
func (GitService) ClearChangeReview(repoPath string, base string, head string) (string, error) {
	storePath, err := reviewPathFor(repoPath, base, head)
	if err != nil {
		return "", err
	}
	if err := os.Remove(storePath); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "已清空该评审会话的结论与评论", nil
}

// ListChangeReviews lists every saved review session of the repository.
func (GitService) ListChangeReviews(repoPath string) ([]ReviewSessionSummary, error) {
	dir, err := reviewStoreDir(repoPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ReviewSessionSummary{}, nil
		}
		return nil, fmt.Errorf("读取评审会话目录失败: %w", err)
	}
	out := []ReviewSessionSummary{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := loadReviewFile(filepath.Join(dir, e.Name()))
		if err != nil || data.Base == "" || data.Head == "" {
			continue
		}
		// Rebuild the session instead of reading the stored counters: the diff
		// may have changed since the session was last opened, and a stale
		// summary would then disagree with the detail view.
		review, err := GitService{}.GetChangeReview(repoPath, data.Base, data.Head)
		if err != nil {
			continue
		}
		out = append(out, ReviewSessionSummary{
			Key:           review.Key,
			Base:          review.Base,
			Head:          review.Head,
			UpdatedAt:     review.UpdatedAt,
			Total:         review.Stats.Total,
			Approved:      review.Stats.Approved,
			Changes:       review.Stats.Changes,
			Pending:       review.Stats.Pending,
			Comments:      review.Stats.Comments,
			OpenComments:  review.Stats.OpenComments,
			Additions:     review.Stats.Additions,
			Deletions:     review.Stats.Deletions,
			Ready:         review.Ready,
			Verdict:       review.Stats.Verdict,
			VerdictDetail: review.Stats.VerdictDetail,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

// ExportChangeReviewMarkdown renders the session as a review report that can
// be pasted straight into a merge request description.
func (GitService) ExportChangeReviewMarkdown(repoPath string, base string, head string) (string, error) {
	review, err := GitService{}.GetChangeReview(repoPath, base, head)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 变更评审：%s → %s\n\n", review.BaseLabel, review.HeadLabel))
	b.WriteString(fmt.Sprintf("- 基准：`%s`\n- 对比：`%s`\n", review.Base, review.Head))
	b.WriteString(fmt.Sprintf("- 文件：%d（+%d / -%d）\n", review.Stats.Total, review.Stats.Additions, review.Stats.Deletions))
	b.WriteString(fmt.Sprintf("- 结论：**%s** — %s\n", review.Stats.Verdict, review.Stats.VerdictDetail))
	b.WriteString(fmt.Sprintf("- 进度：%d%%\n\n", review.Stats.Progress))

	if len(review.Commits) > 0 {
		b.WriteString("## 提交范围\n\n")
		for _, c := range review.Commits {
			b.WriteString(fmt.Sprintf("- `%s` %s · %s · %s\n", c.ShortHash, c.Subject, c.Author, c.Date))
		}
		b.WriteString("\n")
	}

	b.WriteString("## 文件结论\n\n| 文件 | 变更 | 结论 | 备注 |\n|---|---|---|---|\n")
	label := map[string]string{"approved": "✅ 通过", "changes": "⚠️ 需修改", "pending": "⏳ 待评审"}
	for _, f := range review.Files {
		b.WriteString(fmt.Sprintf("| `%s` | +%d/-%d | %s | %s |\n",
			f.Path, f.Additions, f.Deletions, label[f.Status], strings.ReplaceAll(f.Note, "\n", " ")))
	}
	b.WriteString("\n")

	if len(review.Comments) > 0 {
		b.WriteString("## 评论\n\n")
		for _, c := range review.Comments {
			state := "未处理"
			if c.Resolved {
				state = "已解决"
			}
			line := ""
			if c.Line > 0 {
				line = fmt.Sprintf(" L%d", c.Line)
			}
			b.WriteString(fmt.Sprintf("- **%s** · `%s`%s · %s · %s\n  %s\n",
				c.Author, c.Path, line, state, c.CreatedAt, strings.ReplaceAll(c.Body, "\n", " ")))
		}
	}
	return b.String(), nil
}

// GetReviewFileDiff returns the unified diff of one file inside the review
// range, so a reviewer can read the change without leaving the session.
func (GitService) GetReviewFileDiff(repoPath string, base string, head string, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("请先选择一个文件")
	}
	if _, _, err := resolveReviewRef(repoPath, base); err != nil {
		return "", err
	}
	if _, _, err := resolveReviewRef(repoPath, head); err != nil {
		return "", err
	}
	out, err := ExecGitRaw(repoPath, "diff", "--no-color", "--find-renames",
		base+"..."+head, "--", path)
	if err != nil {
		return "", fmt.Errorf("读取文件差异失败: %v", err)
	}
	return out, nil
}

func mustExec(repoPath string, args ...string) string {
	out, _ := ExecGit(repoPath, args...)
	return out
}
