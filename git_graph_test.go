package main

import (
	"fmt"
	"testing"
)

func TestBuildCommitGraphLinear(t *testing.T) {
	// 3 commits in a simple linear branch
	commits := []CommitItem{
		{Hash: "c3", Parents: []string{"c2"}},
		{Hash: "c2", Parents: []string{"c1"}},
		{Hash: "c1", Parents: []string{}},
	}

	res := BuildCommitGraph(commits)
	if len(res) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(res))
	}

	for i, c := range res {
		if c.Lane != 0 {
			t.Errorf("commit %d lane = %d, want 0", i, c.Lane)
		}
	}
	// Root commit has no line down
	if len(res[2].Lines) != 0 {
		t.Errorf("root commit lines = %d, want 0", len(res[2].Lines))
	}
}

func TestBuildCommitGraphMerge(t *testing.T) {
	// Merge commit c3 has parents c2 (main) and f1 (feature)
	commits := []CommitItem{
		{Hash: "c3", Parents: []string{"c2", "f1"}},
		{Hash: "f1", Parents: []string{"c1"}},
		{Hash: "c2", Parents: []string{"c1"}},
		{Hash: "c1", Parents: []string{}},
	}

	res := BuildCommitGraph(commits)
	if len(res) != 4 {
		t.Fatalf("expected 4 commits, got %d", len(res))
	}

	// c3 should be lane 0
	if res[0].Lane != 0 {
		t.Errorf("c3 lane = %d, want 0", res[0].Lane)
	}
	// f1 should be lane 1
	if res[1].Lane != 1 {
		t.Errorf("f1 lane = %d, want 1", res[1].Lane)
	}
}

func BenchmarkBuildCommitGraph(b *testing.B) {
	// Generate 1000 synthetic commits with alternating branching
	items := make([]CommitItem, 1000)
	for i := 0; i < 1000; i++ {
		h := fmt.Sprintf("h%04d", i)
		var parents []string
		if i < 999 {
			parents = append(parents, fmt.Sprintf("h%04d", i+1))
			if i%5 == 0 && i < 995 {
				parents = append(parents, fmt.Sprintf("h%04d", i+3))
			}
		}
		items[i] = CommitItem{
			Hash:    h,
			Parents: parents,
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// Make a copy
		copyItems := make([]CommitItem, len(items))
		copy(copyItems, items)
		_ = BuildCommitGraph(copyItems)
	}
}
