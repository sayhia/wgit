package main

import (
	"testing"
	"time"
)

// TestGetMultiRemoteStatusParallelAheadBehind pins the observable contract of
// the bounded worker pool that now probes remotes concurrently: the result must
// stay sorted by remote name and every in-sync remote must report 0/0 with
// "已跟踪", exactly as the former sequential loop did. Running it under -race
// also guards the disjoint slice-index writes the pool relies on.
func TestGetMultiRemoteStatusParallelAheadBehind(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "config", "commit.gpgsign", "false")
	writeAndCommit(t, repo, "main.go", "package main\n", "chore: init", time.Now(), "Alice")

	// Three remotes whose tracking refs all point at HEAD: every probe must
	// succeed with zero ahead/behind. Add them in non-alphabetical order so the
	// final name sort is genuinely exercised.
	for _, name := range []string{"origin", "upstream", "backup"} {
		url := "https://example.invalid/" + name + ".git"
		runGitTest(t, repo, nil, "remote", "add", name, url)
		runGitTest(t, repo, nil, "update-ref", "refs/remotes/"+name+"/main", "HEAD")
	}

	res, err := (GitService{}).GetMultiRemoteStatus(repo)
	if err != nil {
		t.Fatalf("GetMultiRemoteStatus failed: %v", err)
	}
	if res == nil {
		t.Fatal("GetMultiRemoteStatus returned a nil result")
	}
	if res.CurrentBranch != "main" {
		t.Fatalf("current branch = %q, want main", res.CurrentBranch)
	}
	if len(res.Remotes) != 3 {
		t.Fatalf("remotes = %d, want 3: %+v", len(res.Remotes), res.Remotes)
	}

	wantOrder := []string{"backup", "origin", "upstream"}
	for i, want := range wantOrder {
		got := res.Remotes[i]
		if got.Name != want {
			t.Fatalf("remote[%d] = %q, want %q (list must stay sorted by name): %+v", i, got.Name, want, res.Remotes)
		}
		if got.Branch != "main" {
			t.Fatalf("%s branch = %q, want main", want, got.Branch)
		}
		if got.Ahead != 0 || got.Behind != 0 {
			t.Fatalf("%s ahead/behind = %d/%d, want 0/0", want, got.Ahead, got.Behind)
		}
		if got.LastFetched != "已跟踪" {
			t.Fatalf("%s lastFetched = %q, want 已跟踪", want, got.LastFetched)
		}
		if got.StatusWarning != "" {
			t.Fatalf("%s must not carry a status warning: %q", want, got.StatusWarning)
		}
		if wantURL := "https://example.invalid/" + want + ".git"; got.FetchURL != wantURL {
			t.Fatalf("%s fetchUrl = %q, want %q", want, got.FetchURL, wantURL)
		}
	}
}

// TestGetMultiRemoteStatusMissingRemoteRef covers the other branch of the same
// worker: a remote without refs/remotes/<name>/<branch> must report an unknown
// sync state instead of a misleading 0/0 "已跟踪".
func TestGetMultiRemoteStatusMissingRemoteRef(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, nil, "init", "-b", "main")
	runGitTest(t, repo, nil, "config", "user.name", "Alice")
	runGitTest(t, repo, nil, "config", "user.email", "alice@studio.dev")
	runGitTest(t, repo, nil, "config", "commit.gpgsign", "false")
	writeAndCommit(t, repo, "main.go", "package main\n", "chore: init", time.Now(), "Alice")

	runGitTest(t, repo, nil, "remote", "add", "origin", "https://example.invalid/origin.git")
	runGitTest(t, repo, nil, "update-ref", "refs/remotes/origin/main", "HEAD")
	// "archive" has no remote-tracking ref at all.
	runGitTest(t, repo, nil, "remote", "add", "archive", "https://example.invalid/archive.git")

	res, err := (GitService{}).GetMultiRemoteStatus(repo)
	if err != nil {
		t.Fatalf("GetMultiRemoteStatus failed: %v", err)
	}
	if res == nil || len(res.Remotes) != 2 {
		t.Fatalf("GetMultiRemoteStatus returned %+v, want 2 remotes", res)
	}

	archive, origin := res.Remotes[0], res.Remotes[1]
	if archive.Name != "archive" || origin.Name != "origin" {
		t.Fatalf("remotes must be sorted by name, got %q then %q", archive.Name, origin.Name)
	}
	if archive.Ahead != 0 || archive.Behind != 0 {
		t.Fatalf("archive ahead/behind = %d/%d, want 0/0", archive.Ahead, archive.Behind)
	}
	if archive.LastFetched != "远端无对应同名分支" {
		t.Fatalf("archive lastFetched = %q, want 远端无对应同名分支", archive.LastFetched)
	}
	if archive.StatusWarning != "" {
		t.Fatalf("archive must not carry a status warning: %q", archive.StatusWarning)
	}
	if origin.LastFetched != "已跟踪" || origin.Ahead != 0 || origin.Behind != 0 {
		t.Fatalf("origin should stay tracked and in sync: %+v", origin)
	}
}
