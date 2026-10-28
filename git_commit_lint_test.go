package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestCommitLintInTempRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wgit-lint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	svc := GitService{}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// 1. Test ValidateCommitMessageFormat
	cfg := LintConfig{
		MaxHeaderLength:        50,
		MaxBodyLineLength:      72,
		RequireConventional:    true,
		DisallowTrailingPeriod: true,
	}

	// Valid Conventional Commit
	report1, err := svc.ValidateCommitMessageFormat("feat(auth): add login modal\n\nSupport OAuth2 login.", cfg)
	if err != nil {
		t.Fatalf("ValidateCommitMessageFormat failed: %v", err)
	}
	if !report1.Valid || report1.Score < 80 {
		t.Errorf("expected valid report with high score, got score %d, errors: %v", report1.Score, report1.Errors)
	}
	if report1.Type != "feat" || report1.Scope != "auth" {
		t.Errorf("expected type feat and scope auth, got %s(%s)", report1.Type, report1.Scope)
	}

	// Invalid / warning: trailing period & non-conventional
	report2, err := svc.ValidateCommitMessageFormat("just did some fixes.", cfg)
	if err != nil {
		t.Fatalf("ValidateCommitMessageFormat failed: %v", err)
	}
	if report2.Score >= 100 {
		t.Errorf("expected penalized score for non-conventional commit, got %d", report2.Score)
	}

	// 2. Test FormatCommitMessageWithTemplate
	fmtRes, err := svc.FormatCommitMessageWithTemplate("add user authentication.", "conventional")
	if err != nil {
		t.Fatalf("FormatCommitMessageWithTemplate failed: %v", err)
	}
	if fmtRes.FormattedMessage != "feat: add user authentication" {
		t.Errorf("expected 'feat: add user authentication', got '%s'", fmtRes.FormattedMessage)
	}

	// 3. Test CheckCommitGpgHealth
	gpgRep1, err := svc.CheckCommitGpgHealth(tempDir)
	if err != nil {
		t.Fatalf("CheckCommitGpgHealth failed: %v", err)
	}
	if gpgRep1.IsConfigured {
		t.Errorf("expected unconfigured gpg key")
	}

	runGit("config", "user.signingkey", "ABCD1234EF")
	runGit("config", "commit.gpgsign", "true")

	gpgRep2, err := svc.CheckCommitGpgHealth(tempDir)
	if err != nil {
		t.Fatalf("CheckCommitGpgHealth failed: %v", err)
	}
	if !gpgRep2.IsConfigured || gpgRep2.Status != "healthy" {
		t.Errorf("expected healthy gpg config, got status %s", gpgRep2.Status)
	}
}
