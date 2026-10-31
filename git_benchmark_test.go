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
		// git log 以新→旧输出：父提交总是更旧，即出现在下一行（i+1）。
		// 此前生成器把父提交指向上文（已处理方向），与真实日志相反，
		// 会让 BuildCommitGraph 为每条提交泄漏一条车道（O(n²)）。
		parentHash := ""
		if i < count-1 {
			parentHash = fmt.Sprintf("%040x", i+2)
		}
		// Occasionally add a second parent to simulate a merge commit every 50 commits.
		// 被合分支 tip 更旧、尚未出现在日志中（在 i+20 行）。
		if i > 50 && i%50 == 0 && i+20 < count {
			secondParent := fmt.Sprintf("%040x", i+21)
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
