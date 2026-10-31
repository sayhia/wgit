package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIChatAttachedFileContext(t *testing.T) {
	repo := initTestRepo(t)
	file := filepath.Join(repo, "src", "answer.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	const source = "func Answer() int { return 42 }"
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	attached, err := buildChatPrompt(repo, "src/answer.go", "", "repo", "How does it work?", nil, true)
	if err != nil {
		t.Fatalf("attached prompt: %v", err)
	}
	for _, want := range []string{"Explicitly attached file", "src/answer.go", source} {
		if !strings.Contains(attached, want) {
			t.Fatalf("attached prompt missing %q:\n%s", want, attached)
		}
	}

	plain, err := buildChatPrompt(repo, "src/answer.go", "", "repo", "How does it work?", nil, false)
	if err != nil {
		t.Fatalf("plain prompt: %v", err)
	}
	if strings.Contains(plain, source) {
		t.Fatalf("source leaked without opt-in:\n%s", plain)
	}
}

func TestAIChatRejectsUnsafeAttachments(t *testing.T) {
	repo := initTestRepo(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(repo, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := buildChatPrompt(repo, "link.txt", "", "repo", "q", nil, true); err == nil {
		t.Fatal("symlink attachment was accepted")
	}
	if err := os.WriteFile(filepath.Join(repo, "binary.txt"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := buildChatPrompt(repo, "binary.txt", "", "repo", "q", nil, true); err == nil {
		t.Fatal("binary attachment was accepted")
	}
	if _, err := buildChatPrompt(repo, "src", "", "repo", "q", nil, true); err == nil {
		t.Fatal("directory attachment was accepted")
	}
	if _, err := buildChatPrompt(repo, "../outside.txt", "", "repo", "q", nil, true); err == nil {
		t.Fatal("escaping attachment was accepted")
	}
	if _, err := buildChatPrompt(repo, "", "", "repo", "q", nil, true); err == nil {
		t.Fatal("empty attachment path was accepted")
	}
}

func TestAIChatAttachedFileIsBounded(t *testing.T) {
	repo := initTestRepo(t)
	large := strings.Repeat("Z", aiChatMaxSourceBytes+4096)
	if err := os.WriteFile(filepath.Join(repo, "large.txt"), []byte(large), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt, err := buildChatPrompt(repo, "large.txt", "", "repo", "q", nil, true)
	if err != nil {
		t.Fatalf("large prompt: %v", err)
	}
	if !strings.Contains(prompt, "[file truncated]") {
		t.Fatalf("large attachment did not report truncation")
	}
	if strings.Count(prompt, "Z") > aiChatMaxSourceBytes {
		t.Fatalf("attachment exceeded its byte cap")
	}
}
