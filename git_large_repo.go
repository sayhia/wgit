package main

// git_large_repo.go contains the scale-oriented helpers that keep wgit usable on
// very large repositories (monorepos, game/asset repos, hundreds of branches and
// thousands of commits): commit counting, server-side fuzzy file search, encoded
// path handling for non-ASCII filenames, parallel file-size collection and diff
// payload truncation.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ProjectFileMatch is one row returned by SearchProjectFiles (large-repo ⌘P).
type ProjectFileMatch struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	IsBinary bool   `json:"isBinary"`
}

// maxDiffPayloadBytes / maxDiffPayloadLines bound what a single diff RPC may ship
// to the webview. Real-world game projects routinely contain commits that touch
// thousands of binary assets; without a guard the JSON payload alone can freeze
// the UI. The UI still renders huge diffs progressively.
const (
	maxDiffPayloadBytes = 4 * 1024 * 1024
	maxDiffPayloadLines = 60000
)

// splitNUL splits `git ... -z` output into its NUL-terminated fields, dropping the
// trailing empty field. Unlike line-based parsing this keeps raw (non-quoted) UTF-8
// paths, spaces and even newlines inside filenames intact.
func splitNUL(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// porcelainZEntry is a parsed `git status --porcelain -z` record.
type porcelainZEntry struct {
	Index   byte
	Work    byte
	Path    string
	OldPath string
}

// parsePorcelainZ parses `git status --porcelain=v1 -z` output. In -z mode the
// rename/copy records are emitted as "XY <new>\0<old>\0" (the new path first,
// unlike the human-readable "old -> new" form).
func parsePorcelainZ(raw string) []porcelainZEntry {
	fields := splitNUL(raw)
	entries := make([]porcelainZEntry, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 3 {
			continue
		}
		e := porcelainZEntry{Index: f[0], Work: f[1], Path: f[3:]}
		if e.Index == 'R' || e.Index == 'C' || e.Work == 'R' || e.Work == 'C' {
			if i+1 < len(fields) {
				e.OldPath = fields[i+1]
				i++
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// porcelainStatusMap returns path -> "staged" | "modified" | "untracked" for the
// whole work tree using -z output (safe for spaces and UTF-8 paths).
func porcelainStatusMap(repoPath string) map[string]string {
	statusMap := make(map[string]string)
	raw, err := ExecGitRaw(repoPath, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
	if err != nil {
		return statusMap
	}
	for _, e := range parsePorcelainZ(raw) {
		switch {
		case e.Index == '?' && e.Work == '?':
			statusMap[e.Path] = "untracked"
		case (e.Index == 'R' || e.Index == 'C') && e.OldPath != "":
			// The old path disappeared from the work tree; the new one is staged.
			delete(statusMap, e.OldPath)
			statusMap[e.Path] = "staged"
		case e.Index != ' ' && e.Index != '?':
			statusMap[e.Path] = "staged"
		case e.Work != ' ':
			statusMap[e.Path] = "modified"
		}
	}
	return statusMap
}

// listRepoFiles returns every tracked and untracked-but-not-ignored file path with
// raw UTF-8 encoding preserved (uses `git ls-files -z`).
func listRepoFiles(repoPath string) ([]string, error) {
	raw, err := ExecGitRaw(repoPath, "ls-files", "-z", "-c", "-o", "--exclude-standard")
	if err != nil {
		raw, err = ExecGitRaw(repoPath, "ls-files", "-z")
		if err != nil {
			return nil, err
		}
	}
	files := splitNUL(raw)
	// -c + -o should not overlap, but a file staged in the index may also appear as
	// untracked in edge cases; de-duplicate while preserving order.
	seen := make(map[string]struct{}, len(files))
	unique := files[:0]
	for _, f := range files {
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		unique = append(unique, f)
	}
	return unique, nil
}

// GetCommitCount returns the number of commits reachable from the requested
// scope: a branch name, or every ref when branch is empty / "HEAD".
func (GitService) GetCommitCount(repoPath string, branch string) (int, error) {
	if repoPath == "" {
		return 0, errors.New("repoPath cannot be empty")
	}
	args := []string{"rev-list", "--count"}
	if branch != "" && branch != "HEAD" {
		args = append(args, branch)
	} else {
		args = append(args, "--all")
	}
	out, err := ExecGit(repoPath, args...)
	if err != nil {
		// A freshly initialized repository has no commits yet.
		if strings.Contains(err.Error(), "does not have any commits yet") ||
			strings.Contains(strings.ToLower(out+err.Error()), "unknown revision") {
			return 0, nil
		}
		return 0, err
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return 0, nil
	}
	return n, nil
}

// fuzzyScore ranks a path against a single lower-case token.
func fuzzyScore(path string, token string) int {
	if token == "" {
		return 0
	}
	lowerPath := strings.ToLower(path)
	name := strings.ToLower(filepath.Base(path))
	dir := strings.ToLower(filepath.Dir(path))
	score := 0

	if name == token {
		score += 2000
	}
	if strings.HasPrefix(name, token) {
		score += 900
	} else if idx := strings.Index(name, token); idx >= 0 {
		score += 600 - minInt(idx, 100)
	}
	if strings.HasPrefix(lowerPath, token) {
		score += 350
	}
	if idx := strings.Index(dir, token); idx >= 0 {
		score += 220 - minInt(idx, 120)
	}
	if idx := strings.Index(lowerPath, token); idx >= 0 {
		score += 180 - minInt(idx, 120)
	}
	// Subsequence ("qop" -> "quick_open_panel.vue") fallback.
	if score == 0 {
		if ss := subsequenceScore(lowerPath, token); ss > 0 {
			score += ss
		}
	}
	return score
}

// subsequenceScore gives a small positive score when all token runes appear in
// order inside path, rewarding consecutive runs. Returns 0 when not a match.
func subsequenceScore(path string, token string) int {
	pi := 0
	score := 0
	run := 0
	for _, r := range token {
		found := false
		for pi < len(path) {
			pr := rune(path[pi])
			pi++
			if pr == r {
				found = true
				run++
				score += 10 + run*4
				break
			}
			run = 0
		}
		if !found {
			return 0
		}
	}
	return score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchProjectFiles performs a server-side fuzzy search over every file known to
// the repository. It is fast even for monorepos because it only shells out to
// `git ls-files` and scores paths in Go — the webview never downloads the full
// file list for quick-open.
//
// Multi-word queries are AND-ed: every whitespace-separated token must match.
func (GitService) SearchProjectFiles(repoPath string, query string, limit int) ([]ProjectFileMatch, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}
	if limit <= 0 {
		limit = 80
	}
	if limit > 500 {
		limit = 500
	}

	files, err := listRepoFiles(repoPath)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []ProjectFileMatch{}, nil
	}

	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	type scoredMatch struct {
		item  ProjectFileMatch
		score int
	}
	matches := make([]scoredMatch, 0, minInt(len(files), limit*4))

	for _, p := range files {
		score := 0
		allMatch := true
		for _, token := range tokens {
			s := fuzzyScore(p, token)
			if s <= 0 {
				allMatch = false
				break
			}
			score += s
		}
		if !allMatch {
			continue
		}
		name := filepath.Base(p)
		dir := filepath.Dir(p)
		if dir == "." {
			dir = ""
		}
		matches = append(matches, scoredMatch{
			item: ProjectFileMatch{
				Path:     p,
				Name:     name,
				Dir:      dir,
				IsBinary: isBinaryExtension(strings.ToLower(filepath.Ext(p))),
			},
			score: score,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if len(matches[i].item.Path) != len(matches[j].item.Path) {
			return len(matches[i].item.Path) < len(matches[j].item.Path)
		}
		return matches[i].item.Path < matches[j].item.Path
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]ProjectFileMatch, len(matches))
	for i := range matches {
		out[i] = matches[i].item
	}
	return out, nil
}

// collectFileSizes walks a list of repo-relative paths and fills their sizes with
// a bounded worker pool. Serial os.Stat calls for 100k+ tracked files are the main
// reason a monorepo source explorer used to take seconds to appear.
func collectFileSizes(repoPath string, items []ProjectSourceFileItem) {
	const workers = 32
	if len(items) == 0 {
		return
	}
	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				fi, err := os.Stat(filepath.Join(repoPath, items[idx].Path))
				if err == nil && !fi.IsDir() {
					items[idx].Size = fi.Size()
				}
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// truncateDiffPayload caps an oversized unified diff and appends a machine- and
// human-readable marker. The boolean reports whether truncation happened.
func truncateDiffPayload(diff string) (string, bool) {
	if diff == "" {
		return diff, false
	}
	lineCount := strings.Count(diff, "\n") + 1
	if len(diff) <= maxDiffPayloadBytes && lineCount <= maxDiffPayloadLines {
		return diff, false
	}

	limitBytes := maxDiffPayloadBytes
	limitLines := maxDiffPayloadLines
	if len(diff) > maxDiffPayloadBytes {
		// Cut at the last complete line within the byte budget.
		if idx := strings.LastIndex(diff[:maxDiffPayloadBytes], "\n"); idx > 0 {
			limitBytes = idx
		}
	} else {
		limitBytes = len(diff)
	}
	if lineCount > maxDiffPayloadLines {
		pos := 0
		for i := 0; i < maxDiffPayloadLines; i++ {
			nl := strings.IndexByte(diff[pos:], '\n')
			if nl < 0 {
				pos = len(diff)
				break
			}
			pos += nl + 1
		}
		if pos < limitBytes {
			limitBytes = pos
		}
		limitLines = maxDiffPayloadLines
	}

	keptLines := strings.Count(diff[:limitBytes], "\n") + 1
	marker := "\n... [wgit] 差异过大，已截断：仅显示前 " +
		strconv.Itoa(minInt(keptLines, limitLines)) + " / " + strconv.Itoa(lineCount) +
		" 行（共 " + formatByteSize(int64(len(diff))) + "）...\n"
	return diff[:limitBytes] + marker, true
}
