package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestChangeReviewRefusesToOverwriteCorruptSessionFile(t *testing.T) {
	dir := t.TempDir()
	runGitTest(t, dir, nil, "init", "-b", "main")
	runGitTest(t, dir, nil, "config", "user.name", "Alice")
	runGitTest(t, dir, nil, "config", "user.email", "alice@studio.dev")

	writeAndCommit(t, dir, "src/Battle.cs", "class Battle {}\n", "feat: 战斗结算", time.Now(), "Alice")
	base := strings.TrimSpace(runGitTest(t, dir, nil, "rev-parse", "HEAD"))

	runGitTest(t, dir, nil, "checkout", "-b", "feature/corrupt")
	writeAndCommit(t, dir, "src/Battle.cs", "class Battle { void Update() {} }\n", "feat: 帧同步", time.Now(), "Alice")

	svc := GitService{}
	if _, err := svc.SetReviewFileState(dir, base, "feature/corrupt", "src/Battle.cs", "approved", "数值 OK"); err != nil {
		t.Fatalf("SetReviewFileState failed: %v", err)
	}

	storePath, err := reviewPathFor(dir, base, "feature/corrupt")
	if err != nil {
		t.Fatalf("reviewPathFor failed: %v", err)
	}
	corrupt := []byte(`{"base":"` + base + `","head":"feature/corrupt","decisions":`)
	if err := os.WriteFile(storePath, corrupt, 0o644); err != nil {
		t.Fatalf("write corrupt session: %v", err)
	}

	for name, call := range map[string]func() error{
		"GetChangeReview": func() error {
			_, err := svc.GetChangeReview(dir, base, "feature/corrupt")
			return err
		},
		"SetReviewFileState": func() error {
			_, err := svc.SetReviewFileState(dir, base, "feature/corrupt", "src/Battle.cs", "changes", "")
			return err
		},
		"AddReviewComment": func() error {
			_, err := svc.AddReviewComment(dir, base, "feature/corrupt", "src/Battle.cs", 1, "new", "这里需要保护")
			return err
		},
		"ResolveReviewComment": func() error {
			_, err := svc.ResolveReviewComment(dir, base, "feature/corrupt", "c1", true)
			return err
		},
		"DeleteReviewComment": func() error {
			_, err := svc.DeleteReviewComment(dir, base, "feature/corrupt", "c1")
			return err
		},
	} {
		err := call()
		if err == nil {
			t.Fatalf("%s must report the corrupt session instead of treating it as empty", name)
		}
		if !strings.Contains(err.Error(), "损坏") {
			t.Fatalf("%s error should mention corruption, got %v", name, err)
		}
	}

	after, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read back corrupt session: %v", err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatalf("corrupt session must not be overwritten, got %q", after)
	}

	list, err := svc.ListChangeReviews(dir)
	if err != nil {
		t.Fatalf("ListChangeReviews should skip unreadable entries instead of failing: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("corrupt session must not be reported as a valid session: %+v", list)
	}
}
