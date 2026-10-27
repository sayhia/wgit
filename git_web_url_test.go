package main

import (
	"strings"
	"testing"
)

// webURLFixture resolves real templates for a provider using the production
// parser so the builders are exercised against the actual template shapes.
func webURLFixture(t *testing.T, remoteURL string) RemoteWebInfo {
	t.Helper()
	info := ParseRemoteWebURL(remoteURL)
	if info == nil || !info.HasRemote {
		t.Fatalf("expected HasRemote=true for %s", remoteURL)
	}
	return *info
}

func TestEscapeWebPathSegments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain path", "art/hero.png", "art/hero.png"},
		{"space in file name", "art/my file#1.png", "art/my%20file%231.png"},
		{"hash only", "file#1.png", "file%231.png"},
		{"question mark", "docs/a?b.md", "docs/a%3Fb.md"},
		{"percent sign", "100%done.txt", "100%25done.txt"},
		{"branch with space", "feature/foo bar", "feature/foo%20bar"},
		{"trailing slash preserved", "art/", "art/"},
		{"leading slash preserved", "/art/hero.png", "/art/hero.png"},
		{"empty segments preserved", "art//hero.png", "art//hero.png"},
		{"empty value", "", ""},
		{"surrounding whitespace trimmed", "  art\\hero.png  ", "art%5Chero.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeWebPath(tc.input); got != tc.want {
				t.Errorf("escapeWebPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestEscapeWebValue(t *testing.T) {
	if got := escapeWebValue("abc123"); got != "abc123" {
		t.Errorf("plain hash should be unchanged, got %q", got)
	}
	if got := escapeWebValue("a b#c"); got != "a%20b%23c" {
		t.Errorf("expected a%%20b%%23c, got %q", got)
	}
}

func TestBuildWebURLGitHub(t *testing.T) {
	info := webURLFixture(t, "https://github.com/owner/repo.git")

	commit, err := buildCommitWebURL(info, "abc1234def")
	if err != nil {
		t.Fatalf("buildCommitWebURL: %v", err)
	}
	if want := "https://github.com/owner/repo/commit/abc1234def"; commit != want {
		t.Errorf("commit URL = %q, want %q", commit, want)
	}

	file, err := buildFileWebURL(info, "art/my file#1.png", "feature/foo bar")
	if err != nil {
		t.Fatalf("buildFileWebURL: %v", err)
	}
	if want := "https://github.com/owner/repo/blob/feature/foo%20bar/art/my%20file%231.png"; file != want {
		t.Errorf("file URL = %q, want %q", file, want)
	}

	branch, err := buildBranchWebURL(info, "feature/foo bar")
	if err != nil {
		t.Fatalf("buildBranchWebURL: %v", err)
	}
	if want := "https://github.com/owner/repo/tree/feature/foo%20bar"; branch != want {
		t.Errorf("branch URL = %q, want %q", branch, want)
	}

	compare, err := buildCompareWebURL(info, "release/1.0 rc", "feature/foo bar")
	if err != nil {
		t.Fatalf("buildCompareWebURL: %v", err)
	}
	if want := "https://github.com/owner/repo/compare/release/1.0%20rc...feature/foo%20bar"; compare != want {
		t.Errorf("compare URL = %q, want %q", compare, want)
	}

	repo, err := buildRepoWebURL(info)
	if err != nil {
		t.Fatalf("buildRepoWebURL: %v", err)
	}
	if want := "https://github.com/owner/repo"; repo != want {
		t.Errorf("repo URL = %q, want %q", repo, want)
	}
}

func TestBuildWebURLGitLab(t *testing.T) {
	info := webURLFixture(t, "https://gitlab.com/owner/repo.git")

	commit, err := buildCommitWebURL(info, "abc1234def")
	if err != nil {
		t.Fatalf("buildCommitWebURL: %v", err)
	}
	if want := "https://gitlab.com/owner/repo/-/commit/abc1234def"; commit != want {
		t.Errorf("commit URL = %q, want %q", commit, want)
	}

	file, err := buildFileWebURL(info, "art/my file#1.png", "main")
	if err != nil {
		t.Fatalf("buildFileWebURL: %v", err)
	}
	if want := "https://gitlab.com/owner/repo/-/blob/main/art/my%20file%231.png"; file != want {
		t.Errorf("file URL = %q, want %q", file, want)
	}

	branch, err := buildBranchWebURL(info, "feature/foo bar")
	if err != nil {
		t.Fatalf("buildBranchWebURL: %v", err)
	}
	if want := "https://gitlab.com/owner/repo/-/tree/feature/foo%20bar"; branch != want {
		t.Errorf("branch URL = %q, want %q", branch, want)
	}

	compare, err := buildCompareWebURL(info, "main", "feature/foo bar")
	if err != nil {
		t.Fatalf("buildCompareWebURL: %v", err)
	}
	if want := "https://gitlab.com/owner/repo/-/compare/main...feature/foo%20bar"; compare != want {
		t.Errorf("compare URL = %q, want %q", compare, want)
	}

	repo, err := buildRepoWebURL(info)
	if err != nil {
		t.Fatalf("buildRepoWebURL: %v", err)
	}
	if want := "https://gitlab.com/owner/repo"; repo != want {
		t.Errorf("repo URL = %q, want %q", repo, want)
	}
}

func TestBuildWebURLMissingRemote(t *testing.T) {
	info := RemoteWebInfo{} // HasRemote=false
	if _, err := buildCommitWebURL(info, "abc1234"); err == nil {
		t.Error("commit builder should fail without remote")
	}
	if _, err := buildFileWebURL(info, "a.png", "main"); err == nil {
		t.Error("file builder should fail without remote")
	}
	if _, err := buildBranchWebURL(info, "main"); err == nil {
		t.Error("branch builder should fail without remote")
	}
	if _, err := buildCompareWebURL(info, "main", "dev"); err == nil {
		t.Error("compare builder should fail without remote")
	}
	if _, err := buildRepoWebURL(info); err == nil {
		t.Error("repo builder should fail without remote")
	}

	// A remote with an invalid URL (e.g. plain local path) also parses without
	// a usable web base, so the builders must still fail.
	parsed := ParseRemoteWebURL("/tmp/local.git")
	if parsed.HasRemote {
		t.Fatal("expected local path to have no remote web info")
	}
	if _, err := buildRepoWebURL(*parsed); err == nil {
		t.Error("repo builder should fail for local path remote")
	}
}

func TestOpenWebURLSchemeValidation(t *testing.T) {
	svc := GitService{}
	for _, target := range []string{
		"",
		"file:///etc/passwd",
		"ftp://example.com/repo",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
	} {
		err := svc.OpenWebURL(target)
		if err == nil {
			t.Errorf("OpenWebURL(%q) should reject non-http(s) scheme", target)
			continue
		}
		if target != "" && !strings.Contains(err.Error(), "scheme") {
			t.Errorf("OpenWebURL(%q) error = %v, want scheme error", target, err)
		}
	}
}

// TestBuildWebURLNoPlaceholderLeaks guards against unescaped values leaving
// template placeholders in the final URL.
func TestBuildWebURLNoPlaceholderLeaks(t *testing.T) {
	info := webURLFixture(t, "https://github.com/owner/repo.git")
	urls := make([]string, 0, 4)
	c, _ := buildCommitWebURL(info, "abc123")
	f, _ := buildFileWebURL(info, "art/my file#1.png", "main")
	b, _ := buildBranchWebURL(info, "feature/foo bar")
	cmp, _ := buildCompareWebURL(info, "main", "feature/foo bar")
	urls = append(urls, c, f, b, cmp)
	for _, u := range urls {
		if strings.Contains(u, "{") || strings.Contains(u, "}") {
			t.Errorf("URL still contains a template placeholder: %q", u)
		}
		if strings.Contains(u, " ") || strings.Contains(u, "#") {
			t.Errorf("URL contains unescaped space or fragment marker: %q", u)
		}
		if !strings.HasPrefix(u, "https://github.com/owner/repo/") {
			t.Errorf("URL lost its provider prefix: %q", u)
		}
	}
}
