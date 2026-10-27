package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ComplianceRules struct {
	EnforceSignedCommits         bool     `json:"enforceSignedCommits"`
	RequireCommitMessagePattern  string   `json:"requireCommitMessagePattern"`
	DisallowDirectCommitBranches []string `json:"disallowDirectCommitBranches"`
	MaxFileSizeMb                int      `json:"maxFileSizeMb"`
	RequireEmailDomain           string   `json:"requireEmailDomain"`
}

type CommitPolicyResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
}

type RepoComplianceReport struct {
	HasRulesFile           bool            `json:"hasRulesFile"`
	Rules                  ComplianceRules `json:"rules"`
	SigningConfigured      bool            `json:"signingConfigured"`
	SigningFormat          string          `json:"signingFormat"`
	SignedCommitsRatio     float64         `json:"signedCommitsRatio"`
	CompliantCount         int             `json:"compliantCount"`
	TotalChecked           int             `json:"totalChecked"`
	UnsignedRecentCommits  []string        `json:"unsignedRecentCommits"`
	CurrentBranchProtected bool            `json:"currentBranchProtected"`
	CurrentBranch          string          `json:"currentBranch"`
}

const RulesFileName = ".wgit-rules.json"

func defaultComplianceRules() ComplianceRules {
	return ComplianceRules{
		EnforceSignedCommits:        false,
		RequireCommitMessagePattern: `^(feat|fix|docs|style|refactor|perf|test|chore|build|ci|revert)(\(.*\))?!?: .+`,
		DisallowDirectCommitBranches: []string{
			"main",
			"master",
			"release",
		},
		MaxFileSizeMb:      50,
		RequireEmailDomain: "",
	}
}

func readComplianceRules(repoPath string) (ComplianceRules, bool, error) {
	rulesPath := filepath.Join(repoPath, RulesFileName)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultComplianceRules(), false, nil
		}
		return defaultComplianceRules(), false, err
	}

	rules := defaultComplianceRules()
	if err := json.Unmarshal(data, &rules); err != nil {
		return defaultComplianceRules(), false, err
	}
	return rules, true, nil
}

func (GitService) SaveRepoComplianceRules(repoPath string, rules ComplianceRules) error {
	if repoPath == "" {
		return errors.New("repoPath cannot be empty")
	}
	rulesPath := filepath.Join(repoPath, RulesFileName)
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode compliance rules: %w", err)
	}
	return os.WriteFile(rulesPath, data, 0o644)
}

func (s GitService) InspectRepoCompliance(repoPath string) (*RepoComplianceReport, error) {
	if repoPath == "" {
		return nil, errors.New("repoPath cannot be empty")
	}

	rules, hasFile, err := readComplianceRules(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", RulesFileName, err)
	}

	report := &RepoComplianceReport{
		HasRulesFile: hasFile,
		Rules:        rules,
	}

	// Current branch
	if branchOut, err := ExecGit(repoPath, "symbolic-ref", "--short", "HEAD"); err == nil {
		report.CurrentBranch = strings.TrimSpace(branchOut)
		for _, b := range rules.DisallowDirectCommitBranches {
			if strings.EqualFold(b, report.CurrentBranch) {
				report.CurrentBranchProtected = true
				break
			}
		}
	}

	// Check signing config
	if gpgSign, err := ExecGit(repoPath, "config", "--get", "commit.gpgsign"); err == nil {
		report.SigningConfigured = strings.TrimSpace(gpgSign) == "true"
	}
	if fmtOut, err := ExecGit(repoPath, "config", "--get", "gpg.format"); err == nil {
		report.SigningFormat = strings.TrimSpace(fmtOut)
	}
	if report.SigningFormat == "" {
		report.SigningFormat = "openpgp"
	}

	// Inspect recent commits for signatures
	logOut, err := ExecGit(repoPath, "log", "-n", "20", "--show-signature", "--pretty=format:%H|%s|%G?")
	if err == nil && logOut != "" {
		lines := strings.Split(logOut, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				report.TotalChecked++
				code := strings.TrimSpace(parts[2])
				if code == "G" || code == "U" {
					report.CompliantCount++
				} else {
					shortHash := parts[0]
					if len(shortHash) > 7 {
						shortHash = shortHash[:7]
					}
					report.UnsignedRecentCommits = append(report.UnsignedRecentCommits, fmt.Sprintf("%s: %s", shortHash, parts[1]))
				}
			}
		}
	}

	if report.TotalChecked > 0 {
		report.SignedCommitsRatio = float64(report.CompliantCount) / float64(report.TotalChecked)
	}

	return report, nil
}

func (GitService) ValidateCommitPolicy(repoPath string, commitMessage string, authorEmail string, branchName string) (*CommitPolicyResult, error) {
	rules, _, err := readComplianceRules(repoPath)
	if err != nil {
		return nil, err
	}

	result := &CommitPolicyResult{
		Valid:      true,
		Violations: []string{},
	}

	if branchName == "" {
		if bOut, err := ExecGit(repoPath, "symbolic-ref", "--short", "HEAD"); err == nil {
			branchName = strings.TrimSpace(bOut)
		}
	}
	if authorEmail == "" {
		if emailOut, err := ExecGit(repoPath, "config", "--get", "user.email"); err == nil {
			authorEmail = strings.TrimSpace(emailOut)
		}
	}

	// 1. Branch protection
	if branchName != "" {
		for _, protected := range rules.DisallowDirectCommitBranches {
			if strings.EqualFold(protected, branchName) {
				result.Valid = false
				result.Violations = append(result.Violations, fmt.Sprintf("Direct commits to branch %q are prohibited by enterprise compliance rules. Please use a feature branch and pull request.", branchName))
				break
			}
		}
	}

	// 2. Commit message format
	if rules.RequireCommitMessagePattern != "" && commitMessage != "" {
		firstLine := strings.Split(commitMessage, "\n")[0]
		matched, err := regexp.MatchString(rules.RequireCommitMessagePattern, firstLine)
		if err == nil && !matched {
			result.Valid = false
			result.Violations = append(result.Violations, fmt.Sprintf("Commit message %q does not match required pattern %q (e.g. Conventional Commits).", firstLine, rules.RequireCommitMessagePattern))
		}
	}

	// 3. Author email domain
	if rules.RequireEmailDomain != "" && authorEmail != "" {
		requiredDomain := strings.ToLower(rules.RequireEmailDomain)
		if !strings.HasPrefix(requiredDomain, "@") {
			requiredDomain = "@" + requiredDomain
		}
		if !strings.HasSuffix(strings.ToLower(authorEmail), requiredDomain) {
			result.Valid = false
			result.Violations = append(result.Violations, fmt.Sprintf("Author email %q does not match enterprise required domain %q.", authorEmail, requiredDomain))
		}
	}

	return result, nil
}
