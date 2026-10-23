package main

import (
	"strings"
	"testing"
	"time"
)

// makeTaggedRepo builds a repo with one commit, one lightweight tag and one
// annotated tag, plus a tag pointing at an older commit.
func makeTaggedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")
	run("commit", "-q", "--allow-empty", "-m", "first commit")
	run("tag", "v-light")
	run("tag", "-a", "v1.0.0", "-m", "release 1.0")
	run("commit", "-q", "--allow-empty", "-m", "second commit")
	run("tag", "-a", "v1.0.1", "-m", "release 1.0.1")

	return dir
}

func TestGetTagsDistinguishesAnnotated(t *testing.T) {
	dir := makeTaggedRepo(t)

	tags, err := GitService{}.GetTags(dir)
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("got %d tags, want 3: %+v", len(tags), tags)
	}

	byName := map[string]TagInfo{}
	for _, tg := range tags {
		byName[tg.Name] = tg
	}

	if got := byName["v-light"]; got.IsAnnotated {
		t.Errorf("v-light is annotated = true, want false (%+v)", got)
	}
	for _, name := range []string{"v1.0.0", "v1.0.1"} {
		if got := byName[name]; !got.IsAnnotated {
			t.Errorf("%s is annotated = false, want true (%+v)", name, got)
		}
	}

	// The tag messages must survive, and the lightweight tag must not pretend
	// to have one of its own.
	if got := byName["v1.0.0"]; got.Message != "release 1.0" {
		t.Errorf("v1.0.0 message = %q, want %q", got.Message, "release 1.0")
	}
}

func TestGetTagDetailFormatsTaggerDate(t *testing.T) {
	dir := makeTaggedRepo(t)

	detail, err := GitService{}.GetTagDetail(dir, "v1.0.0")
	if err != nil {
		t.Fatalf("GetTagDetail: %v", err)
	}
	if !detail.IsAnnotated {
		t.Error("v1.0.0 should be annotated")
	}
	if detail.Message != "release 1.0" {
		t.Errorf("message = %q, want %q", detail.Message, "release 1.0")
	}
	if detail.TaggerName != "tester" || detail.TaggerEmail != "t@example.com" {
		t.Errorf("tagger = %q <%s>", detail.TaggerName, detail.TaggerEmail)
	}
	firstCommit, _ := ExecGit(dir, "rev-parse", "v1.0.0^{commit}")
	tagObject, _ := ExecGit(dir, "rev-parse", "v1.0.0")
	if detail.CommitHash != strings.TrimSpace(firstCommit) || detail.CommitHash == strings.TrimSpace(tagObject) {
		t.Errorf("target commit = %q, want dereferenced %q (tag object %q)", detail.CommitHash, firstCommit, tagObject)
	}
	// Raw git tag objects store a unix timestamp; the UI needs a readable date.
	if _, err := time.Parse("2006-01-02 15:04:05 -0700", detail.Date); err != nil {
		t.Errorf("date = %q is not formatted: %v", detail.Date, err)
	}
}

// TestGetTagsReportsCommitHash pins the bug this fixed: for annotated tags the
// ref points at a tag object, so plain %(objectname) returned an OID that
// matched nothing in git log. The hash must be the commit either way.
func TestGetTagsReportsCommitHash(t *testing.T) {
	dir := makeTaggedRepo(t)

	tags, err := GitService{}.GetTags(dir)
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}

	// The authoritative answer, straight from git.
	wantFor := map[string]string{}
	for _, tg := range tags {
		want, err := ExecGit(dir, "rev-parse", "--short", tg.Name+"^{commit}")
		if err != nil {
			t.Fatalf("rev-parse %s: %v", tg.Name, err)
		}
		wantFor[tg.Name] = want
	}

	for _, tg := range tags {
		want := wantFor[tg.Name]
		if tg.CommitHash != want {
			t.Errorf("%s CommitHash = %q, want %q (dereferenced commit)",
				tg.Name, tg.CommitHash, want)
		}
	}

	// And the hash must be usable: it has to resolve to a real commit.
	for _, tg := range tags {
		if _, err := ExecGit(dir, "cat-file", "-t", tg.CommitHash); err != nil {
			t.Errorf("CommitHash %q for %s is not a valid object: %v", tg.CommitHash, tg.Name, err)
		}
	}

	// Tags at different commits must not collapse to the same hash.
	if wantFor["v1.0.0"] == wantFor["v1.0.1"] {
		t.Error("v1.0.0 and v1.0.1 should point at different commits")
	}
}

// TestGetTagsEmptyMessage guards the ExecGitRaw switch: a tag with no message
// leaves a trailing empty tab field, which a whole-stdout TrimSpace would eat.
func TestGetTagsEmptyMessage(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		if _, err := ExecGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "tester")
	run("commit", "-q", "--allow-empty", "-m", "only commit")
	// -a with an empty -m produces an annotated tag with no message.
	run("tag", "-a", "v-empty", "-m", "")

	tags, err := GitService{}.GetTags(dir)
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("got %d tags, want 1: %+v", len(tags), tags)
	}
	if tags[0].Name != "v-empty" {
		t.Errorf("name = %q, want v-empty", tags[0].Name)
	}
	if tags[0].Message != "" {
		t.Errorf("message = %q, want empty", tags[0].Message)
	}
	if !tags[0].IsAnnotated {
		t.Error("v-empty should still be annotated despite an empty message")
	}
}
