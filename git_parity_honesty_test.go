package main

import (
	"strings"
	"testing"
)

func TestParityHubsAreExplicitlySelfReported(t *testing.T) {
	repo := t.TempDir()
	svc := GitService{}

	full, err := svc.GetFullParityStatusSummary(repo)
	if err != nil {
		t.Fatalf("full parity hub: %v", err)
	}
	if strings.Contains(full.SystemReadiness, "CERTIFIED") {
		t.Fatalf("static inventory must not claim certification: %q", full.SystemReadiness)
	}
	if !strings.EqualFold(full.SystemReadiness, "SELF_REPORTED_STATIC") {
		t.Fatalf("unexpected readiness: %q", full.SystemReadiness)
	}
	if !strings.Contains(strings.ToLower(full.CertificationBadge), "not a certification") {
		t.Fatalf("badge must disclaim certification: %q", full.CertificationBadge)
	}
	if full.ReleaseSignature != "" || full.TotalTestCoverageCount != 0 {
		t.Fatalf("static inventory must not carry fake signature or test count: %+v", full)
	}
	if !strings.Contains(strings.Join(allParityHighlights(full.Domains, nil), " "), "未接入") {
		t.Fatalf("external integrations must state their limits")
	}

	pinnacle, err := svc.GetPinnacleParityStatusSummary(repo)
	if err != nil {
		t.Fatalf("pinnacle parity hub: %v", err)
	}
	if strings.Contains(pinnacle.SystemReadiness, "CERTIFIED") {
		t.Fatalf("pinnacle static inventory must not claim certification: %q", pinnacle.SystemReadiness)
	}
	if pinnacle.ReleaseSignature != "" || pinnacle.TotalTestCoverageCount != 0 || pinnacle.TotalToolsCount != 0 {
		t.Fatalf("pinnacle inventory must not fabricate counts or signatures: %+v", pinnacle)
	}
	highlights := strings.Join(allParityHighlights(nil, pinnacle.Domains), " ")
	// TAPD（OpenAPI 未接入）已随集成下线移除；剩余未接入的外部集成仍须如实标注。
	if !strings.Contains(highlights, "T-Build 未接入") {
		t.Fatalf("pinnacle highlights must state integration limits: %q", highlights)
	}
	if strings.Contains(highlights, "局域网共享 DDC") {
		t.Fatalf("pinnacle highlights must not claim unimplemented LAN DDC sync: %q", highlights)
	}
}

func allParityHighlights(full []ParityDomainSummary, pinnacle []PinnacleDomainSummary) []string {
	highlights := []string{}
	for _, domain := range full {
		highlights = append(highlights, domain.Highlights...)
	}
	for _, domain := range pinnacle {
		highlights = append(highlights, domain.Highlights...)
	}
	return highlights
}
