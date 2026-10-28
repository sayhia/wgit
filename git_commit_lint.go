package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type LintConfig struct {
	MaxHeaderLength        int  `json:"maxHeaderLength"`
	MaxBodyLineLength      int  `json:"maxBodyLineLength"`
	RequireConventional    bool `json:"requireConventional"`
	DisallowTrailingPeriod bool `json:"disallowTrailingPeriod"`
}

type CommitLintReport struct {
	Valid       bool     `json:"valid"`
	Score       int      `json:"score"` // 0 - 100
	Type        string   `json:"type"`
	Scope       string   `json:"scope"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	Suggestions []string `json:"suggestions"`
}

type CommitFormatResult struct {
	FormattedMessage string   `json:"formattedMessage"`
	OriginalMessage  string   `json:"originalMessage"`
	ChangesMade      []string `json:"changesMade"`
}

type GpgHealthReport struct {
	IsConfigured    bool   `json:"isConfigured"`
	SigningKey      string `json:"signingKey"`
	Format          string `json:"format"` // "openpgp", "ssh", "x509"
	AutoSignCommits bool   `json:"autoSignCommits"`
	Status          string `json:"status"` // "healthy", "warning", "unconfigured", "error"
	Message         string `json:"message"`
}

var conventionalRe = regexp.MustCompile(`^([a-zA-Z]+)(?:\(([^\)]+)\))?(!)?:\s*(.*)$`)

var standardTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"docs":     true,
	"style":    true,
	"refactor": true,
	"perf":     true,
	"test":     true,
	"build":    true,
	"ci":       true,
	"chore":    true,
	"revert":   true,
}

func execGitLint(repoPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ValidateCommitMessageFormat lints a commit message against Conventional Commits and quality rules.
func (GitService) ValidateCommitMessageFormat(message string, config LintConfig) (*CommitLintReport, error) {
	cleanMsg := strings.TrimSpace(message)
	if cleanMsg == "" {
		return &CommitLintReport{
			Valid:       false,
			Score:       0,
			Errors:      []string{"Commit message cannot be empty"},
			Suggestions: []string{"Use 'type(scope): subject' format, e.g., 'feat(auth): add login'"},
		}, nil
	}

	if config.MaxHeaderLength <= 0 {
		config.MaxHeaderLength = 72
	}
	if config.MaxBodyLineLength <= 0 {
		config.MaxBodyLineLength = 72
	}

	lines := strings.Split(message, "\n")
	header := lines[0]
	score := 100
	var errs []string
	var warns []string
	var suggs []string

	var cType, cScope, cSubject string

	// 1. Header length check
	if len(header) > config.MaxHeaderLength {
		warns = append(warns, fmt.Sprintf("Header line is %d characters (recommended <= %d)", len(header), config.MaxHeaderLength))
		score -= 15
		suggs = append(suggs, "Shorten the first line to be more concise.")
	}

	// 2. Parse Conventional Commit pattern
	matches := conventionalRe.FindStringSubmatch(header)
	if len(matches) > 0 {
		cType = strings.ToLower(matches[1])
		cScope = matches[2]
		cSubject = matches[4]

		if !standardTypes[cType] {
			warns = append(warns, fmt.Sprintf("Non-standard commit type '%s'", cType))
			score -= 10
			suggs = append(suggs, "Standard types: feat, fix, docs, style, refactor, perf, test, chore, build, ci, revert")
		}

		if strings.TrimSpace(cSubject) == "" {
			errs = append(errs, "Subject cannot be empty after type/scope prefix")
			score -= 30
		}
	} else if config.RequireConventional {
		warns = append(warns, "Message does not follow Conventional Commits format (type(scope): subject)")
		score -= 25
		suggs = append(suggs, "Format header as 'type(scope): subject', e.g., 'feat: add user profile'")
		cSubject = header
	} else {
		cSubject = header
	}

	// 3. Trailing period check on subject
	if config.DisallowTrailingPeriod && strings.HasSuffix(strings.TrimSpace(cSubject), ".") {
		warns = append(warns, "Subject should not end with a period")
		score -= 10
		suggs = append(suggs, "Remove trailing '.' from the subject line.")
	}

	// 4. Body lines check
	body := ""
	if len(lines) > 1 {
		bodyLines := lines[1:]
		if len(bodyLines) > 0 && strings.TrimSpace(bodyLines[0]) != "" {
			warns = append(warns, "Separate header from body with a blank line")
			score -= 10
			suggs = append(suggs, "Insert an empty line after the header.")
		}

		for idx, bl := range bodyLines {
			if len(bl) > config.MaxBodyLineLength {
				warns = append(warns, fmt.Sprintf("Body line %d exceeds %d characters (%d chars)", idx+2, config.MaxBodyLineLength, len(bl)))
				score -= 5
				break
			}
		}
		body = strings.Join(bodyLines, "\n")
	}

	if score < 0 {
		score = 0
	}

	valid := len(errs) == 0 && score >= 50

	return &CommitLintReport{
		Valid:       valid,
		Score:       score,
		Type:        cType,
		Scope:       cScope,
		Subject:     cSubject,
		Body:        body,
		Errors:      errs,
		Warnings:    warns,
		Suggestions: suggs,
	}, nil
}

// FormatCommitMessageWithTemplate normalizes and formats commit message lines.
func (GitService) FormatCommitMessageWithTemplate(message string, templateType string) (*CommitFormatResult, error) {
	orig := message
	cleanMsg := strings.TrimSpace(message)
	if cleanMsg == "" {
		return &CommitFormatResult{
			FormattedMessage: "",
			OriginalMessage:  orig,
			ChangesMade:      []string{},
		}, nil
	}

	lines := strings.Split(cleanMsg, "\n")
	header := strings.TrimSpace(lines[0])
	var changes []string

	// Remove trailing period in header
	if strings.HasSuffix(header, ".") {
		header = strings.TrimSuffix(header, ".")
		changes = append(changes, "Removed trailing period from header")
	}

	// If template is conventional and missing colon
	if strings.ToLower(templateType) == "conventional" {
		matches := conventionalRe.FindStringSubmatch(header)
		if len(matches) == 0 {
			// Prepend feat:
			header = "feat: " + header
			changes = append(changes, "Prepended 'feat:' prefix")
		}
	}

	// Ensure empty line between header and body
	var formattedLines []string
	formattedLines = append(formattedLines, header)

	if len(lines) > 1 {
		formattedLines = append(formattedLines, "") // Blank line
		for _, l := range lines[1:] {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				formattedLines = append(formattedLines, trimmed)
			}
		}
		changes = append(changes, "Formatted body with standard blank line separation")
	}

	formatted := strings.Join(formattedLines, "\n")

	return &CommitFormatResult{
		FormattedMessage: formatted,
		OriginalMessage:  orig,
		ChangesMade:      changes,
	}, nil
}

// CheckCommitGpgHealth inspects git gpg signing configuration and availability.
func (GitService) CheckCommitGpgHealth(repoPath string) (*GpgHealthReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	signingKey, _ := execGitLint(repoPath, "config", "user.signingkey")
	signingKey = strings.TrimSpace(signingKey)

	gpgSign, _ := execGitLint(repoPath, "config", "commit.gpgsign")
	autoSign := strings.TrimSpace(gpgSign) == "true"

	gpgFormat, _ := execGitLint(repoPath, "config", "gpg.format")
	gpgFormat = strings.TrimSpace(gpgFormat)
	if gpgFormat == "" {
		gpgFormat = "openpgp"
	}

	if signingKey == "" {
		return &GpgHealthReport{
			IsConfigured:    false,
			SigningKey:      "",
			Format:          gpgFormat,
			AutoSignCommits: autoSign,
			Status:          "unconfigured",
			Message:         "GPG/SSH signing key is not configured in git user.signingkey",
		}, nil
	}

	status := "healthy"
	msg := fmt.Sprintf("Signing configured with %s key: %s", gpgFormat, signingKey)
	if !autoSign {
		status = "warning"
		msg += " (commit.gpgsign is disabled)"
	}

	return &GpgHealthReport{
		IsConfigured:    true,
		SigningKey:      signingKey,
		Format:          gpgFormat,
		AutoSignCommits: autoSign,
		Status:          status,
		Message:         msg,
	}, nil
}
