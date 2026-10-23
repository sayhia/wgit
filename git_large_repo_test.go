package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetCommitCountScopes(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	svc := GitService{}

	// Freshly initialized repository: zero commits, no error.
	n, err := svc.GetCommitCount(dir, "")
	if err != nil {
		t.Fatalf("GetCommitCount on empty repo failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 commits on empty repo, got %d", n)
	}

	now := time.Now()
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: a", now, "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "feat: b", now, "Alice")
	writeAndCommit(t, dir, "c.txt", "c\n", "feat: c", now, "Alice")

	// main has 3 commits.
	n, err = svc.GetCommitCount(dir, "main")
	if err != nil || n != 3 {
		t.Fatalf("expected 3 commits on main, got %d err=%v", n, err)
	}

	// A side branch adds one more commit; --all sees 4, main still sees 3.
	runGitTest(t, dir, nil, "checkout", "-b", "feature/extra")
	writeAndCommit(t, dir, "d.txt", "d\n", "feat: d", now, "Alice")
	runGitTest(t, dir, nil, "checkout", "main")

	all, err := svc.GetCommitCount(dir, "")
	if err != nil || all != 4 {
		t.Fatalf("expected 4 commits across all refs, got %d err=%v", all, err)
	}
	mainOnly, err := svc.GetCommitCount(dir, "main")
	if err != nil || mainOnly != 3 {
		t.Fatalf("expected 3 commits on main scope, got %d err=%v", mainOnly, err)
	}
}

func TestSearchProjectFilesFuzzyAndUnicode(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	files := map[string]string{
		"src/quick_open_panel.vue":   "<template></template>\n",
		"src/components/中文 面板.vue":   "<template></template>\n",
		"src/app.go":                 "package main\n",
		"docs/readme.md":             "# docs\n",
		"assets/hero_albedo.png.txt": "not really a png\n",
	}
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runGitTest(t, dir, nil, "add", "-A")
	runGitTest(t, dir, nil, "commit", "-m", "feat: files")

	svc := GitService{}

	// Fuzzy subsequence: qop -> quick_open_panel.vue
	res, err := svc.SearchProjectFiles(dir, "qop", 10)
	if err != nil {
		t.Fatalf("SearchProjectFiles(qop) failed: %v", err)
	}
	if len(res) == 0 || res[0].Path != "src/quick_open_panel.vue" {
		t.Fatalf("expected quick_open_panel first for qop, got %+v", res)
	}
	if res[0].Name != "quick_open_panel.vue" || res[0].Dir != "src" {
		t.Fatalf("unexpected name/dir split: %+v", res[0])
	}

	// Non-ASCII query and raw (non-escaped) path.
	res, err = svc.SearchProjectFiles(dir, "中文", 10)
	if err != nil {
		t.Fatalf("SearchProjectFiles(中文) failed: %v", err)
	}
	if len(res) == 0 || !strings.Contains(res[0].Path, "中文") {
		t.Fatalf("expected unicode path match, got %+v", res)
	}
	for _, m := range res {
		if strings.Contains(m.Path, "\\3") {
			t.Fatalf("path should not be octal-escaped: %q", m.Path)
		}
	}

	// Multi-token AND query.
	res, err = svc.SearchProjectFiles(dir, "src go", 10)
	if err != nil {
		t.Fatalf("SearchProjectFiles(multi-token) failed: %v", err)
	}
	found := false
	for _, m := range res {
		if m.Path == "src/app.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected src/app.go for 'src go', got %+v", res)
	}

	// Empty query returns the file listing, bounded by limit.
	res, err = svc.SearchProjectFiles(dir, "", 2)
	if err != nil {
		t.Fatalf("SearchProjectFiles(empty) failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected limit=2, got %d rows", len(res))
	}
}

func TestGetProjectSourceFilesHandlesSpacesAndUnicode(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	now := time.Now()
	writeAndCommit(t, dir, "src/a file.txt", "hello\n", "feat: spaced file", now, "Alice")
	writeAndCommit(t, dir, "src/中文.txt", "中文\n", "feat: unicode file", now, "Alice")

	// Untracked file (not committed) must still appear with status "untracked".
	untracked := filepath.Join(dir, "untracked note.txt")
	if err := os.WriteFile(untracked, []byte("draft\n"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := GitService{}
	items, err := svc.GetProjectSourceFiles(dir)
	if err != nil {
		t.Fatalf("GetProjectSourceFiles failed: %v", err)
	}
	byPath := make(map[string]ProjectSourceFileItem, len(items))
	for _, it := range items {
		byPath[it.Path] = it
		if strings.Contains(it.Path, "\\3") {
			t.Fatalf("escaped path leaked through: %q", it.Path)
		}
	}
	if it, ok := byPath["src/a file.txt"]; !ok {
		t.Fatalf("missing path with space in %+v", byPath)
	} else if it.Size <= 0 {
		t.Fatalf("expected non-zero size for existing file, got %+v", it)
	}
	if _, ok := byPath["src/中文.txt"]; !ok {
		t.Fatalf("missing unicode path in %+v", byPath)
	}
	if it, ok := byPath["untracked note.txt"]; !ok {
		t.Fatalf("missing untracked file in %+v", byPath)
	} else if it.Status != "untracked" {
		t.Fatalf("expected untracked status, got %+v", it)
	}
}

func TestParsePorcelainZRenameAndUnicode(t *testing.T) {
	raw := "R  new name.txt\x00old name.txt\x00" +
		"?? untracked file.txt\x00" +
		" M modified.txt\x00" +
		"A  中文新增.txt\x00"

	entries := parsePorcelainZ(raw)
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Index != 'R' || entries[0].Path != "new name.txt" || entries[0].OldPath != "old name.txt" {
		t.Fatalf("rename entry parsed wrong: %+v", entries[0])
	}
	if entries[1].Path != "untracked file.txt" || entries[1].Index != '?' {
		t.Fatalf("untracked entry parsed wrong: %+v", entries[1])
	}
	if entries[2].Work != 'M' || entries[2].Path != "modified.txt" {
		t.Fatalf("modified entry parsed wrong: %+v", entries[2])
	}
	if entries[3].Path != "中文新增.txt" {
		t.Fatalf("unicode entry parsed wrong: %+v", entries[3])
	}
}

func TestTruncateDiffPayload(t *testing.T) {
	small := "diff --git a/a b/a\n+ok\n"
	if out, truncated := truncateDiffPayload(small); truncated || out != small {
		t.Fatalf("small diff should not be truncated")
	}

	big := strings.Repeat("+0123456789abcdef\n", 400000) // ~7.6 MB
	if len(big) <= maxDiffPayloadBytes {
		t.Fatalf("test fixture too small: %d", len(big))
	}
	out, truncated := truncateDiffPayload(big)
	if !truncated {
		t.Fatalf("expected large diff to be truncated")
	}
	if !strings.Contains(out, "[wgit] 差异过大") {
		t.Fatalf("missing truncation marker")
	}
	if len(out) > maxDiffPayloadBytes+512 {
		t.Fatalf("truncated diff still too large: %d bytes", len(out))
	}
}

func TestLargeRepoDiffGuardInCommitDetail(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	// A single commit with a multi-MB text file: the patch payload must be capped.
	big := strings.Repeat("padding line 0123456789\n", 250000) // ~5.7 MB
	writeAndCommit(t, dir, "big.txt", big, "feat: huge file", time.Now(), "Alice")

	svc := GitService{}
	commits, err := svc.GetCommits(dir, 1, "")
	if err != nil || len(commits) == 0 {
		t.Fatalf("GetCommits failed: %v", err)
	}
	detail, err := svc.GetCommitDetail(dir, commits[0].Hash)
	if err != nil {
		t.Fatalf("GetCommitDetail failed: %v", err)
	}
	if !detail.DiffTruncated {
		t.Fatalf("expected oversized commit patch to be truncated, len=%d", len(detail.Diff))
	}
	if len(detail.Diff) > maxDiffPayloadBytes+512 {
		t.Fatalf("diff payload not capped: %d bytes", len(detail.Diff))
	}
	if !strings.Contains(detail.Diff, "[wgit] 差异过大") {
		t.Fatalf("missing truncation marker in commit diff")
	}
}

func TestGetCommitsFilteredReturnsTrueTotal(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	now := time.Now()
	writeAndCommit(t, dir, "a.txt", "a\n", "feat: alpha", now, "Alice")
	writeAndCommit(t, dir, "b.txt", "b\n", "fix: bravo", now, "Alice")
	writeAndCommit(t, dir, "c.txt", "c\n", "fix: charlie", now, "Bob")
	writeAndCommit(t, dir, "d.txt", "d\n", "feat: delta", now, "Bob")
	writeAndCommit(t, dir, "e.txt", "e\n", "fix: echo", now, "Alice")

	svc := GitService{}

	// Page 1 of keyword "fix": 3 matching commits in total, 2 on this page.
	res, err := svc.GetCommitsFiltered(dir, "main", "", "", "", "fix", 0, 2)
	if err != nil {
		t.Fatalf("GetCommitsFiltered failed: %v", err)
	}
	if res.TotalCount != 3 {
		t.Fatalf("expected filtered total 3, got %d", res.TotalCount)
	}
	if len(res.Commits) != 2 || !res.HasMore {
		t.Fatalf("expected page of 2 with hasMore, got len=%d hasMore=%v", len(res.Commits), res.HasMore)
	}

	// Page 2: one remaining commit, no more.
	res, err = svc.GetCommitsFiltered(dir, "main", "", "", "", "fix", 2, 2)
	if err != nil {
		t.Fatalf("GetCommitsFiltered page 2 failed: %v", err)
	}
	if res.TotalCount != 3 || len(res.Commits) != 1 || res.HasMore {
		t.Fatalf("page 2 wrong: total=%d len=%d hasMore=%v", res.TotalCount, len(res.Commits), res.HasMore)
	}

	// Author filter composes with the keyword filter and count.
	res, err = svc.GetCommitsFiltered(dir, "main", "Bob", "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("author filter failed: %v", err)
	}
	if res.TotalCount != 2 {
		t.Fatalf("expected 2 commits by Bob, got %d", res.TotalCount)
	}
	for _, c := range res.Commits {
		if c.AuthorName != "Bob" {
			t.Fatalf("author filter leaked commit by %q", c.AuthorName)
		}
	}
}
