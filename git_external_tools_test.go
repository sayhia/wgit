package main

import (
	"testing"
)

func TestParseRemoteWebURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		provider   string
		webBase    string
		commitTpl  string
		compareTpl string
	}{
		{
			name:       "GitHub SSH",
			url:        "git@github.com:torvalds/linux.git",
			provider:   "github",
			webBase:    "https://github.com/torvalds/linux",
			commitTpl:  "https://github.com/torvalds/linux/commit/{hash}",
			compareTpl: "https://github.com/torvalds/linux/compare/{base}...{head}",
		},
		{
			name:       "GitHub HTTPS",
			url:        "https://github.com/sunmking/wgit.git",
			provider:   "github",
			webBase:    "https://github.com/sunmking/wgit",
			commitTpl:  "https://github.com/sunmking/wgit/commit/{hash}",
			compareTpl: "https://github.com/sunmking/wgit/compare/{base}...{head}",
		},
		{
			name:       "GitLab SSH with port",
			url:        "ssh://git@gitlab.company.com:2222/game-team/engine/client.git",
			provider:   "gitlab",
			webBase:    "https://gitlab.company.com/game-team/engine/client",
			commitTpl:  "https://gitlab.company.com/game-team/engine/client/-/commit/{hash}",
			compareTpl: "https://gitlab.company.com/game-team/engine/client/-/compare/{base}...{head}",
		},
		{
			name:       "Gitee HTTPS",
			url:        "https://gitee.com/oschina/git-osc.git",
			provider:   "gitee",
			webBase:    "https://gitee.com/oschina/git-osc",
			commitTpl:  "https://gitee.com/oschina/git-osc/commit/{hash}",
			compareTpl: "https://gitee.com/oschina/git-osc/compare/{base}...{head}",
		},
		{
			name:       "Bitbucket HTTPS",
			url:        "https://bitbucket.org/atlassian/sample-repo.git",
			provider:   "bitbucket",
			webBase:    "https://bitbucket.org/atlassian/sample-repo",
			commitTpl:  "https://bitbucket.org/atlassian/sample-repo/commits/{hash}",
			compareTpl: "https://bitbucket.org/atlassian/sample-repo/branches/compare/{head}%0D{base}#diff",
		},
		{
			name:       "Self-hosted Gitea",
			url:        "https://gitea.internal.net/studio/assets.git",
			provider:   "gitea",
			webBase:    "https://gitea.internal.net/studio/assets",
			commitTpl:  "https://gitea.internal.net/studio/assets/commit/{hash}",
			compareTpl: "https://gitea.internal.net/studio/assets/compare/{base}...{head}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := ParseRemoteWebURL(tc.url)
			if !res.HasRemote {
				t.Fatalf("expected HasRemote=true for %s", tc.url)
			}
			if res.Provider != tc.provider {
				t.Errorf("expected provider %s, got %s", tc.provider, res.Provider)
			}
			if res.WebBaseURL != tc.webBase {
				t.Errorf("expected webBase %s, got %s", tc.webBase, res.WebBaseURL)
			}
			if res.CommitURLTpl != tc.commitTpl {
				t.Errorf("expected commitTpl %s, got %s", tc.commitTpl, res.CommitURLTpl)
			}
			if res.CompareURLTpl != tc.compareTpl {
				t.Errorf("expected compareTpl %s, got %s", tc.compareTpl, res.CompareURLTpl)
			}
		})
	}
}

func TestFindIssueReferences(t *testing.T) {
	msg := "fix: resolve memory leak in physics engine (#482, GH-991)\n\nAlso closes PROJ-1024 and #482."
	refs := FindIssueReferences(msg)
	expected := []string{"#482", "GH-991", "PROJ-1024"}

	if len(refs) != len(expected) {
		t.Fatalf("expected %d refs, got %d: %v", len(expected), len(refs), refs)
	}
	for i, exp := range expected {
		if refs[i] != exp {
			t.Errorf("expected %s at %d, got %s", exp, i, refs[i])
		}
	}
}

func TestDetectExternalTools(t *testing.T) {
	svc := GitService{}
	tools, err := svc.DetectExternalTools()
	if err != nil {
		t.Fatalf("DetectExternalTools returned error: %v", err)
	}
	if len(tools) == 0 {
		t.Fatalf("expected at least some tool candidates in list")
	}

	hasAny := false
	for _, tool := range tools {
		if tool.IsAvailable {
			hasAny = true
			t.Logf("Detected tool available: %s (%s) at %s", tool.Name, tool.ID, tool.Executable)
		}
	}
	t.Logf("Total tools evaluated: %d, hasAnyAvailable=%v", len(tools), hasAny)
}
