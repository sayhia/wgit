package main

import (
	"fmt"
	"strings"
	"testing"
)

// generateSyntheticGitLog records generates n synthetic git log entries formatted with \x1e and \x1f
func generateSyntheticGitLog(count int) string {
	var sb strings.Builder
	// Allocate approximate buffer: ~120 bytes per commit
	sb.Grow(count * 120)

	for i := 0; i < count; i++ {
		hash := fmt.Sprintf("%040x", i+1)
		shortHash := hash[:7]
		parentHash := ""
		if i > 0 {
			parentHash = fmt.Sprintf("%040x", i)
		}
		// Occasionally add a second parent to simulate a merge commit every 50 commits
		if i > 50 && i%50 == 0 {
			secondParent := fmt.Sprintf("%040x", i-20)
			parentHash = parentHash + " " + secondParent
		}

		author := "Author Name"
		email := "author@example.com"
		date := "2026-10-27 12:00:00 +0800"
		relDate := "2 hours ago"
		subject := fmt.Sprintf("commit message %d for benchmarking", i+1)
		body := ""
		refs := ""
		if i == count-1 {
			refs = "HEAD -> main, origin/main"
		}
		sigStatus := "G"
		signer := "Valid Signer <signer@example.com>"

		sb.WriteString("\x1e")
		sb.WriteString(hash)
		sb.WriteString("\x1f")
		sb.WriteString(shortHash)
		sb.WriteString("\x1f")
		sb.WriteString(parentHash)
		sb.WriteString("\x1f")
		sb.WriteString(author)
		sb.WriteString("\x1f")
		sb.WriteString(email)
		sb.WriteString("\x1f")
		sb.WriteString(date)
		sb.WriteString("\x1f")
		sb.WriteString(relDate)
		sb.WriteString("\x1f")
		sb.WriteString(subject)
		sb.WriteString("\x1f")
		sb.WriteString(body)
		sb.WriteString("\x1f")
		sb.WriteString(refs)
		sb.WriteString("\x1f")
		sb.WriteString(sigStatus)
		sb.WriteString("\x1f")
		sb.WriteString(signer)
	}

	return sb.String()
}

func TestLargeScaleCommitGraph100k(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100k commit benchmark test in short mode")
	}

	const count = 100_000
	rawLog := generateSyntheticGitLog(count)
	commits := ParseRawGitLog(rawLog)

	if len(commits) != count {
		t.Fatalf("expected %d commits parsed, got %d", count, len(commits))
	}

	first := commits[0]
	if first.Hash != fmt.Sprintf("%040x", 1) {
		t.Errorf("unexpected first commit hash: %s", first.Hash)
	}

	// Verify graph building on 10,000 commits finishes accurately
	subCommits := commits[:10_000]
	graphed := BuildCommitGraph(subCommits)
	if len(graphed) != 10_000 {
		t.Fatalf("expected 10000 graphed commits, got %d", len(graphed))
	}
}

func BenchmarkParseRawGitLog10k(b *testing.B) {
	rawLog := generateSyntheticGitLog(10_000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		commits := ParseRawGitLog(rawLog)
		if len(commits) != 10_000 {
			b.Fatalf("expected 10000 commits, got %d", len(commits))
		}
	}
}

func BenchmarkBuildCommitGraph10k(b *testing.B) {
	rawLog := generateSyntheticGitLog(10_000)
	commits := ParseRawGitLog(rawLog)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Clone shallowly for graph run
		cloned := make([]CommitItem, len(commits))
		copy(cloned, commits)
		_ = BuildCommitGraph(cloned)
	}
}
