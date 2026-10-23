package main

import (
	"strings"
	"testing"
	"time"
)

func TestChangeReviewFlow(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	runGitTest(t, dir, nil, "checkout", "-b", "feature/review-demo")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle { /* rebalanced */ }\n", "feat: 数值重调", time.Now(), "Alice")
	writeAndCommit(t, dir, "art/splash.png", "\x89PNG\r\n\x1a\nfake-binary", "art: 启动图更新", time.Now(), "Alice")

	svc := GitService{}

	// invalid refs surface as readable errors
	if _, err := svc.GetChangeReview(dir, "main", "main"); err == nil {
		t.Fatalf("expected error for identical refs")
	}
	if _, err := svc.GetChangeReview(dir, "no-such-ref", "feature/review-demo"); err == nil {
		t.Fatalf("expected error for missing base ref")
	}

	review, err := svc.GetChangeReview(dir, base, "feature/review-demo")
	if err != nil {
		t.Fatalf("GetChangeReview failed: %v", err)
	}
	if review.Stats.Total != 2 {
		t.Fatalf("expected 2 changed files, got %d", review.Stats.Total)
	}
	if review.Stats.Pending != 2 || review.Stats.Approved != 0 {
		t.Fatalf("expected all pending, got %+v", review.Stats)
	}
	if len(review.Commits) != 2 {
		t.Fatalf("expected 2 commits in range, got %d", len(review.Commits))
	}
	if review.Stats.Verdict != "评审中" {
		t.Fatalf("unexpected verdict: %s", review.Stats.Verdict)
	}

	// file decision
	review, err = svc.SetReviewFileState(dir, base, "feature/review-demo", "src/Battle.cs", "approved", "数值 OK")
	if err != nil {
		t.Fatalf("SetReviewFileState failed: %v", err)
	}
	if review.Stats.Approved != 1 || review.Stats.Pending != 1 {
		t.Fatalf("expected 1 approved 1 pending, got %+v", review.Stats)
	}
	if review.Stats.Progress != 50 {
		t.Fatalf("expected progress 50, got %d", review.Stats.Progress)
	}
	if review.Ready {
		t.Fatalf("session must not be ready with pending files")
	}

	// second decision makes it mergeable
	review, err = svc.SetReviewFileState(dir, base, "feature/review-demo", "art/splash.png", "approved", "")
	if err != nil {
		t.Fatalf("SetReviewFileState failed: %v", err)
	}
	if !review.Ready || review.Stats.Verdict != "可以合并" {
		t.Fatalf("expected ready session, got %+v", review.Stats)
	}

	// invalid status rejected
	if _, err := svc.SetReviewFileState(dir, base, "feature/review-demo", "src/Battle.cs", "bogus", ""); err == nil {
		t.Fatalf("expected error for unknown status")
	}

	// per-file diff for the review pane
	fileDiff, err := svc.GetReviewFileDiff(dir, base, "feature/review-demo", "src/Battle.cs")
	if err != nil {
		t.Fatalf("GetReviewFileDiff failed: %v", err)
	}
	if !strings.Contains(fileDiff, "rebalanced") || !strings.Contains(fileDiff, "@@") {
		t.Fatalf("diff missing content:\n%s", fileDiff)
	}
	if binaryDiff, err := svc.GetReviewFileDiff(dir, base, "feature/review-demo", "art/splash.png"); err != nil {
		t.Fatalf("binary diff failed: %v", err)
	} else if !strings.Contains(binaryDiff, "Binary files") && binaryDiff != "" {
		t.Fatalf("expected a binary marker, got:\n%s", binaryDiff)
	}
	if _, err := svc.GetReviewFileDiff(dir, base, "feature/review-demo", ""); err == nil {
		t.Fatalf("expected error for an empty path")
	}
	if _, err := svc.GetReviewFileDiff(dir, "no-such-ref", "feature/review-demo", "src/Battle.cs"); err == nil {
		t.Fatalf("expected error for a missing base ref")
	}

	// the session list must report the same verdict as the detail view
	sessions, err := svc.ListChangeReviews(dir)
	if err != nil {
		t.Fatalf("ListChangeReviews failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 saved session, got %d", len(sessions))
	}
	if sessions[0].Verdict != "可以合并" || !sessions[0].Ready {
		t.Fatalf("session list disagrees with detail: %+v vs %+v", sessions[0], review.Stats)
	}
	if sessions[0].Total != 2 || sessions[0].Approved != 2 || sessions[0].Pending != 0 {
		t.Fatalf("session counters wrong: %+v", sessions[0])
	}

	// markdown export round-trips the decisions
	report, err := svc.ExportChangeReviewMarkdown(dir, base, "feature/review-demo")
	if err != nil {
		t.Fatalf("ExportChangeReviewMarkdown failed: %v", err)
	}
	if !strings.Contains(report, "✅ 通过") || !strings.Contains(report, "src/Battle.cs") {
		t.Fatalf("markdown missing decisions:\n%s", report)
	}

	// comments
	review, err = svc.AddReviewComment(dir, base, "feature/review-demo", "src/Battle.cs", 12, "new", "这里的除法建议加保护")
	if err != nil {
		t.Fatalf("AddReviewComment failed: %v", err)
	}
	if review.Stats.Comments != 1 || review.Stats.OpenComments != 1 {
		t.Fatalf("expected 1 open comment, got %+v", review.Stats)
	}
	if !review.Ready && review.Stats.Verdict != "需修改" {
		t.Fatalf("open comments must block merge: %+v", review.Stats)
	}
	for _, f := range review.Files {
		if f.Path == "src/Battle.cs" {
			if f.CommentCount != 1 || f.OpenCommentCount != 1 {
				t.Fatalf("per-file comment rollup wrong: %+v", f)
			}
		}
	}

	if _, err := svc.AddReviewComment(dir, base, "feature/review-demo", "src/Battle.cs", 0, "new", "  "); err == nil {
		t.Fatalf("expected error for empty comment body")
	}

	commentID := review.Comments[0].ID
	review, err = svc.ResolveReviewComment(dir, base, "feature/review-demo", commentID, true)
	if err != nil {
		t.Fatalf("ResolveReviewComment failed: %v", err)
	}
	if review.Stats.OpenComments != 0 {
		t.Fatalf("expected no open comments after resolve, got %+v", review.Stats)
	}
	if !review.Ready {
		t.Fatalf("resolved session should be ready: %+v", review.Stats)
	}

	// reopen then delete
	review, err = svc.ResolveReviewComment(dir, base, "feature/review-demo", commentID, false)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if review.Stats.OpenComments != 1 {
		t.Fatalf("expected 1 open after reopen, got %+v", review.Stats)
	}
	review, err = svc.DeleteReviewComment(dir, base, "feature/review-demo", commentID)
	if err != nil {
		t.Fatalf("DeleteReviewComment failed: %v", err)
	}
	if review.Stats.Comments != 0 {
		t.Fatalf("expected no comments, got %+v", review.Stats)
	}
	if _, err := svc.ResolveReviewComment(dir, base, "feature/review-demo", "nope", true); err == nil {
		t.Fatalf("expected error for unknown comment id")
	}

	// persistence: a fresh read must still carry the decisions
	reloaded, err := svc.GetChangeReview(dir, base, "feature/review-demo")
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	approvedCount := 0
	for _, f := range reloaded.Files {
		if f.Status == "approved" {
			approvedCount++
		}
		if f.Path == "src/Battle.cs" && f.Note != "数值 OK" {
			t.Fatalf("note lost after reload: %+v", f)
		}
	}
	if approvedCount != 2 {
		t.Fatalf("decisions lost after reload: %d approved", approvedCount)
	}

	// session list
	list, err := svc.ListChangeReviews(dir)
	if err != nil {
		t.Fatalf("ListChangeReviews failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 saved session, got %d", len(list))
	}
	if list[0].Base != base || list[0].Head != "feature/review-demo" {
		t.Fatalf("unexpected session summary: %+v", list[0])
	}

	// markdown export
	md, err := svc.ExportChangeReviewMarkdown(dir, base, "feature/review-demo")
	if err != nil {
		t.Fatalf("ExportChangeReviewMarkdown failed: %v", err)
	}
	for _, want := range []string{"# 变更评审", "可以合并", "src/Battle.cs", "✅ 通过", "数值 OK"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}

	// clear wipes the session
	if _, err := svc.ClearChangeReview(dir, base, "feature/review-demo"); err != nil {
		t.Fatalf("ClearChangeReview failed: %v", err)
	}
	cleared, err := svc.GetChangeReview(dir, base, "feature/review-demo")
	if err != nil {
		t.Fatalf("GetChangeReview after clear failed: %v", err)
	}
	if cleared.Stats.Approved != 0 || cleared.Stats.Pending != 2 {
		t.Fatalf("expected clean session after clear, got %+v", cleared.Stats)
	}
}
