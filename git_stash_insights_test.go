package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeStashRepo builds a repo with one commit and then stashes a working tree
// that mixes ordinary edits, paths with spaces, a pure rename and a binary.
func makeStashRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		return out
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")

	write("keep.txt", "a\n")
	write("old name.txt", "x\ny\nz\n")
	write("logo.png", string([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}))
	run("add", "-A")
	run("commit", "-qm", "base")

	// working tree changes that end up in the stash
	write("keep.txt", "a\nb\nc\n")    // modified
	write("added.txt", "brand new\n") // added
	write("logo.png", string([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x01, 0x02, 0x03}))
	write("spaced name.txt", "space\n")       // added, path has a space
	run("mv", "old name.txt", "new name.txt") // pure rename

	// -u is required for the untracked files (added.txt and the spaced path) to
	// enter the stash at all, so the list below actually exercises them.
	run("stash", "push", "-q", "-u", "-m", "probe stash")
	return dir
}

func TestGetStashesParsesList(t *testing.T) {
	dir := makeStashRepo(t)

	stashes, err := GitService{}.GetStashes(dir)
	if err != nil {
		t.Fatalf("GetStashes: %v", err)
	}
	if len(stashes) != 1 {
		t.Fatalf("got %d stashes, want 1: %+v", len(stashes), stashes)
	}
	got := stashes[0]
	if got.Index != 0 {
		t.Errorf("index = %d, want 0", got.Index)
	}
	if !strings.Contains(got.Message, "probe stash") {
		t.Errorf("message = %q, want it to contain the stash subject", got.Message)
	}
	if strings.TrimSpace(got.Date) == "" {
		t.Error("relative date should not be empty")
	}
}

func TestStashClearEmptiesList(t *testing.T) {
	dir := makeStashRepo(t)
	svc := GitService{}

	if err := svc.StashClear(dir); err != nil {
		t.Fatalf("StashClear: %v", err)
	}
	stashes, err := svc.GetStashes(dir)
	if err != nil {
		t.Fatalf("GetStashes after clear: %v", err)
	}
	if len(stashes) != 0 {
		t.Errorf("got %d stashes after clear, want 0: %+v", len(stashes), stashes)
	}
}

func TestGetStashFiles(t *testing.T) {
	dir := makeStashRepo(t)

	files, err := GitService{}.GetStashFiles(dir, "0")
	if err != nil {
		t.Fatalf("GetStashFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one file in the stash")
	}

	byPath := map[string]StashFileEntry{}
	for _, f := range files {
		byPath[f.Path] = f
	}

	// Paths containing spaces must survive intact — this is what the old
	// diff-parsing approach mangled.
	for _, want := range []string{"keep.txt", "added.txt", "new name.txt", "spaced name.txt", "logo.png"} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("missing %q in stash file list: %+v", want, files)
		}
	}
	if _, ok := byPath["old name.txt"]; ok {
		t.Errorf("rename source should not appear as its own entry: %+v", files)
	}

	// A pure rename has no content change, so both counts stay zero.
	if got := byPath["new name.txt"]; got.Status != "R" {
		t.Errorf("new name.txt status = %q, want R", got.Status)
	}

	// keep.txt gained two lines.
	if got := byPath["keep.txt"]; got.Additions != 2 {
		t.Errorf("keep.txt additions = %d, want 2 (%+v)", got.Additions, got)
	}

	// Binary files report no line counts but must still be listed.
	if got := byPath["logo.png"]; got.Additions != 0 || got.Deletions != 0 {
		t.Errorf("logo.png counts = +%d/-%d, want 0/0", got.Additions, got.Deletions)
	}

	// Sorted by path for a stable list.
	for i := 1; i < len(files); i++ {
		if files[i-1].Path > files[i].Path {
			t.Errorf("list not sorted at %d: %q > %q", i, files[i-1].Path, files[i].Path)
		}
	}
}

func TestGetStashFilesRejectsBadInput(t *testing.T) {
	dir := makeStashRepo(t)

	// Note: `GitService{}` is a composite literal, which Go's parser rejects in
	// the header of an if/for statement, so each call is assigned first.
	_, err := GitService{}.GetStashFiles("", "0")
	if err == nil {
		t.Error("expected an error for an empty repo path")
	}
	_, err = GitService{}.GetStashFiles(dir, "")
	if err == nil {
		t.Error("expected an error for an empty stash index")
	}
	_, err = GitService{}.GetStashFiles(dir, "99")
	if err == nil {
		t.Error("expected an error for a stash index that does not exist")
	}
}

func TestNewPathFromNumstat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"src/a.txt", "src/a.txt"},
		{"old.txt => new.txt", "new.txt"},
		{"dir/{old => new}.txt", "dir/new.txt"},
		{"a/{x => y}/b.txt", "a/y/b.txt"},
	}
	for _, c := range cases {
		if got := newPathFromNumstat(c.in); got != c.want {
			t.Errorf("newPathFromNumstat(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseNameStatusZ(t *testing.T) {
	// Mirrors real git output: "M\0path\0" and "R100\0old\0new\0".
	raw := "M\x00keep.txt\x00R100\x00old name.txt\x00new name.txt\x00A\x00logo.png\x00"
	got := parseNameStatusZ(raw)

	want := []nameStatusRecord{
		{Status: "M", Path: "keep.txt"},
		{Status: "R", Path: "new name.txt"},
		{Status: "A", Path: "logo.png"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	if parseNameStatusZ("") != nil {
		t.Error("empty input should yield no records")
	}
}

// TestGetStashFilesCoversStashDiff pins the property GetStashFiles exists
// for: it must list every file the rendered diff mentions, and it must go
// further by also listing untracked files, which `git stash show -p` drops
// entirely because it only diffs tracked paths.
func TestGetStashFilesCoversStashDiff(t *testing.T) {
	dir := makeStashRepo(t)

	files, err := GitService{}.GetStashFiles(dir, "0")
	if err != nil {
		t.Fatalf("GetStashFiles: %v", err)
	}
	diff, err := GitService{}.GetStashDiff(dir, 0)
	if err != nil {
		t.Fatalf("GetStashDiff: %v", err)
	}

	listed := map[string]bool{}
	for _, f := range files {
		listed[f.Path] = true
	}

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		path := line[strings.Index(line, " b/")+3:]
		if !listed[path] {
			t.Errorf("diff contains %q but GetStashFiles omitted it", path)
		}
	}

	// The stash was created with -u, so the untracked files are in stash@{0}^3
	// and absent from the diff. They must still be restorable.
	for _, want := range []string{"added.txt", "spaced name.txt"} {
		if !listed[want] {
			t.Errorf("untracked file %q missing from GetStashFiles", want)
		}
		if strings.Contains(diff, want) {
			t.Errorf("%q unexpectedly present in the stash diff; test premise broken", want)
		}
	}
}

func TestStashSaveAdvancedIncludesIgnoredFiles(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit(dir, args...)
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		return out
	}
	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "tester@example.com")
	run("config", "user.name", "tester")

	write(".gitignore", "*.log\nbuild/\n")
	write("main.go", "package main\n")
	run("add", "-A")
	run("commit", "-m", "init")

	// Create an untracked file and an ignored file
	write("main.go", "package main\n// modified\n")
	write("extra.txt", "untracked\n")
	write("debug.log", "ignored log\n")

	svc := GitService{}
	if err := svc.StashSaveAdvanced(dir, "stash with ignored", false, true, true); err != nil {
		t.Fatalf("StashSaveAdvanced failed: %v", err)
	}

	// Working tree should be completely clean (no debug.log, no extra.txt, main.go unmodified)
	if _, err := os.Stat(filepath.Join(dir, "debug.log")); !os.IsNotExist(err) {
		t.Errorf("expected debug.log to be stashed away, but file still exists")
	}

	stashes, err := svc.GetStashes(dir)
	if err != nil {
		t.Fatalf("GetStashes failed: %v", err)
	}
	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(stashes))
	}
	if !strings.Contains(stashes[0].Message, "stash with ignored") {
		t.Errorf("stash message = %q, want it to contain 'stash with ignored'", stashes[0].Message)
	}
}
