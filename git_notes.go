package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Git notes let QA and designers annotate commits without rewriting history.

type CommitNoteItem struct {
	CommitHash string `json:"commitHash"`
	ShortHash  string `json:"shortHash"`
	Author     string `json:"authorName"`
	Date       string `json:"date"`
	Subject    string `json:"subject"`
	Content    string `json:"content"`
	Preview    string `json:"preview"`
}

type CommitNotesResult struct {
	Notes   []CommitNoteItem `json:"notes"`
	Total   int              `json:"total"`
	Ref     string           `json:"ref"`
	Summary string           `json:"summary"`
}

const defaultNotesRef = "refs/notes/commits"

// notesInspectWorkers caps how many note inspections run at once. Inspecting a
// note costs a cat-file plus a log process, so a fixed pool replaces up to 400
// sequential processes with roughly 400/8 rounds without flooding the machine
// with git children.
const notesInspectWorkers = 8

// GetCommitNotes lists every commit that carries a note, newest commit first.
func (GitService) GetCommitNotes(repoPath string, limit int) (*CommitNotesResult, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	res := &CommitNotesResult{
		Notes: []CommitNoteItem{},
		Ref:   defaultNotesRef,
	}

	raw, err := ExecGit(repoPath, "notes", "list")
	if err != nil {
		if isMissingNotesRefError(err) {
			res.Summary = "当前仓库还没有任何提交批注。"
			return res, nil
		}
		return nil, err
	}

	type noteRef struct {
		blob   string
		commit string
	}
	refs := []noteRef{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		refs = append(refs, noteRef{blob: fields[0], commit: fields[1]})
	}
	res.Total = len(refs)

	if len(refs) == 0 {
		res.Summary = "当前仓库还没有任何提交批注。"
		return res, nil
	}

	if len(refs) > limit {
		refs = refs[:limit]
	}

	// Note inspections are independent, so run them through a bounded pool.
	// Every worker writes only its own preallocated slot: no append and no
	// shared map, and the ref order stays intact for the stable sort below.
	items := make([]CommitNoteItem, len(refs))
	sem := make(chan struct{}, notesInspectWorkers)
	var wg sync.WaitGroup
	for i, ref := range refs {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int, ref noteRef) {
			defer wg.Done()
			defer func() { <-sem }()

			item := CommitNoteItem{CommitHash: ref.commit}

			if content, cerr := ExecGit(repoPath, "cat-file", "-p", ref.blob); cerr == nil {
				item.Content = strings.TrimRight(content, "\n")
				item.Preview = notePreview(item.Content)
			}

			meta, merr := ExecGit(repoPath, "log", "-1", "--date=short",
				"--format=%h%x1f%an%x1f%ad%x1f%s", ref.commit)
			if merr == nil {
				fields := strings.Split(meta, "\x1f")
				if len(fields) >= 4 {
					item.ShortHash = fields[0]
					item.Author = fields[1]
					item.Date = fields[2]
					item.Subject = fields[3]
				}
			}
			if item.ShortHash == "" {
				item.ShortHash = shortHashOf(ref.commit)
			}

			items[i] = item
		}(i, ref)
	}
	wg.Wait()

	res.Notes = append(res.Notes, items...)

	sort.SliceStable(res.Notes, func(i, j int) bool {
		return res.Notes[i].Date > res.Notes[j].Date
	})

	res.Summary = fmt.Sprintf("共 %d 条提交批注，当前展示最近 %d 条。", res.Total, len(res.Notes))
	return res, nil
}

// GetNotedCommitHashes returns just the annotated commit ids, used for history markers.
func (GitService) GetNotedCommitHashes(repoPath string) ([]string, error) {
	if repoPath == "" {
		return nil, errors.New("repository path is required")
	}
	raw, err := ExecGit(repoPath, "notes", "list")
	if err != nil {
		if isMissingNotesRefError(err) {
			return []string{}, nil
		}
		return nil, err
	}
	hashes := []string{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hashes = append(hashes, fields[1])
		}
	}
	return hashes, nil
}

// GetCommitNote returns the note attached to a commit, or an empty string.
func (GitService) GetCommitNote(repoPath string, commitHash string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	if strings.TrimSpace(commitHash) == "" {
		return "", errors.New("commit hash is required")
	}
	out, err := ExecGit(repoPath, "notes", "show", commitHash)
	if err != nil {
		if isMissingNotesRefError(err) || strings.Contains(strings.ToLower(err.Error()), "no note found") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

// SaveCommitNote creates or replaces the note of a commit.
func (GitService) SaveCommitNote(repoPath string, commitHash string, content string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	if strings.TrimSpace(commitHash) == "" {
		return "", errors.New("commit hash is required")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("批注内容不能为空")
	}
	if _, err := ExecGit(repoPath, "notes", "add", "-f", "-m", content, commitHash); err != nil {
		return "", err
	}
	return fmt.Sprintf("已保存 %s 的提交批注", shortHashOf(commitHash)), nil
}

// DeleteCommitNote removes the note of a commit.
func (GitService) DeleteCommitNote(repoPath string, commitHash string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	if strings.TrimSpace(commitHash) == "" {
		return "", errors.New("commit hash is required")
	}
	if _, err := ExecGit(repoPath, "notes", "remove", commitHash); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no note found") {
			return "", errors.New("该提交没有可删除的批注")
		}
		return "", err
	}
	return fmt.Sprintf("已删除 %s 的提交批注", shortHashOf(commitHash)), nil
}

// PushCommitNotes publishes the notes ref so teammates can see annotations.
func (GitService) PushCommitNotes(repoPath string, remote string) (string, error) {
	if repoPath == "" {
		return "", errors.New("repository path is required")
	}
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = defaultPushRemote(repoPath)
	}
	out, err := ExecGit(repoPath, "push", remote, defaultNotesRef)
	if err != nil {
		return out, fmt.Errorf("推送批注失败: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		out = "批注已是最新，无需推送"
	}
	return out, nil
}

func isMissingNotesRefError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no note found") ||
		strings.Contains(msg, "not a valid object name") ||
		strings.Contains(msg, "unknown revision")
}

func shortHashOf(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func notePreview(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 100)
		}
	}
	return ""
}
