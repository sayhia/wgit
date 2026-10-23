package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanStagedSecretsAndRedaction(t *testing.T) {
	svc := GitService{}
	repo := t.TempDir()
	if _, err := svc.InitRepo(repo); err != nil {
		t.Fatalf("InitRepo failed: %v", err)
	}
	if err := svc.SetGitConfig(repo, "Tester", "tester@example.com", false); err != nil {
		t.Fatalf("SetGitConfig failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.StageFile(repo, "base.txt"); err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	if err := svc.Commit(repo, "chore: base", false); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	awsKey := "AKIA1234567890123456"
	githubToken := "ghp_" + strings.Repeat("A", 36)
	privateKey := "-----BEGIN RSA PRIVATE KEY-----"
	configContent := strings.Join([]string{
		"package config",
		`const awsKey = "` + awsKey + `"`,
		`const githubToken = "` + githubToken + `"`,
		`    const api_secret = "supersecretvalue123"`,
	}, "\n") + "\n"
	files := map[string]string{
		"keys/aws.pem": privateKey + "\nMIIBOgIBAAJBAK\n",
		".env":         "API_SECRET=abcd1234efgh5678\n",
		"config.txt":   configContent,
	}
	for name, content := range files {
		full := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := svc.StageFile(repo, name); err != nil {
			t.Fatalf("StageFile(%s) failed: %v", name, err)
		}
	}

	findings, err := svc.ScanStagedSecrets(repo)
	if err != nil || len(findings) == 0 {
		t.Fatalf("ScanStagedSecrets failed: %d findings err=%v", len(findings), err)
	}
	var sawPemFile, sawEnvFile, sawAWS, sawGitHub, sawHardcoded, sawPrivateHeader bool
	for _, finding := range findings {
		if strings.Contains(finding.RedactedSnippet, awsKey) || strings.Contains(finding.RedactedSnippet, githubToken) {
			t.Fatalf("redacted snippet leaked a full secret: %+v", finding)
		}
		switch {
		case finding.FilePath == "keys/aws.pem" && finding.Severity == "CRITICAL" && strings.Contains(finding.RuleName, "私钥"):
			sawPemFile = true
		case finding.FilePath == ".env" && finding.Severity == "WARNING":
			sawEnvFile = true
		case strings.Contains(finding.RedactedSnippet, "AKIA****3456"):
			sawAWS = finding.LineNumber == 2 && finding.Severity == "CRITICAL"
		case strings.Contains(finding.RedactedSnippet, "ghp_****AAAA"):
			sawGitHub = finding.LineNumber == 3 && finding.Severity == "CRITICAL"
		case strings.Contains(finding.RuleName, "硬编码") && strings.Contains(finding.RedactedSnippet, "****"):
			sawHardcoded = finding.LineNumber == 4 && finding.Severity == "HIGH"
		case finding.FilePath == "keys/aws.pem" && strings.Contains(finding.RuleName, "私有密钥"):
			sawPrivateHeader = true
		}
	}
	if !sawPemFile || !sawEnvFile || !sawAWS || !sawGitHub || !sawHardcoded || !sawPrivateHeader {
		t.Fatalf("missing expected findings: pem=%v env=%v aws=%v github=%v hardcoded=%v header=%v all=%+v",
			sawPemFile, sawEnvFile, sawAWS, sawGitHub, sawHardcoded, sawPrivateHeader, findings)
	}

	if err := svc.UnstageAll(repo); err != nil {
		t.Fatalf("UnstageAll failed: %v", err)
	}
	empty, err := svc.ScanStagedSecrets(repo)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("expected non-nil empty findings, got %+v err=%v", empty, err)
	}

	line := "    const token = \"" + awsKey + "\""
	start := strings.Index(line, awsKey)
	redacted := redactSecretText(line, start, start+len(awsKey))
	if redacted != `const token = "AKIA****3456"` {
		t.Fatalf("unexpected indented redaction: %q", redacted)
	}
	if short := redactSecretText("token=abc123", 6, 12); short != "token=****" {
		t.Fatalf("unexpected short secret redaction: %q", short)
	}
	longLine := strings.Repeat("x", 130) + awsKey
	longStart := strings.Index(longLine, awsKey)
	longRedacted := redactSecretText(longLine, longStart, longStart+len(awsKey))
	if strings.Contains(longRedacted, awsKey) || !strings.HasSuffix(longRedacted, "...") {
		t.Fatalf("long line redaction failed: %q", longRedacted)
	}
	if out := redactSecretText("abc", 10, 20); out != "abc" {
		t.Fatalf("out-of-range redaction should be a no-op: %q", out)
	}
}
