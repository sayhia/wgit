package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditPrivacyAndComplianceUsesRealManifests(t *testing.T) {
	svc := GitService{}

	iosOnly := t.TempDir()
	writeTestFile(t, filepath.Join(iosOnly, "App", "Info.plist"), `<?xml version="1.0"?><plist><dict/></plist>`)
	res, err := svc.AuditPrivacyAndCompliance(iosOnly)
	if err != nil {
		t.Fatalf("AuditPrivacyAndCompliance failed: %v", err)
	}
	if res.Platform != "iOS" || res.HasPrivacyManifest || res.IsStoreReady || res.StoreReadinessGrade != "FAIL" {
		t.Fatalf("missing iOS privacy manifest should block: %+v", res)
	}
	if len(res.Violations) != 1 || res.Violations[0].RuleID != "MISSING_PRIVACY_MANIFEST" {
		t.Fatalf("unexpected violations: %+v", res.Violations)
	}

	crossPlatform := t.TempDir()
	writeTestFile(t, filepath.Join(crossPlatform, "App", "PrivacyInfo.xcprivacy"),
		`<plist><dict><key>NSPrivacyTrackingDomains</key><array><string>a.example</string><string>b.example</string></array></dict></plist>`)
	writeTestFile(t, filepath.Join(crossPlatform, "android", "app", "AndroidManifest.xml"),
		`<manifest><uses-permission android:name="android.permission.CAMERA"/><uses-permission android:name="android.permission.INTERNET"/></manifest>`)
	res, err = svc.AuditPrivacyAndCompliance(crossPlatform)
	if err != nil {
		t.Fatalf("AuditPrivacyAndCompliance failed: %v", err)
	}
	if res.Platform != "CrossPlatform" || !res.HasPrivacyManifest || !res.IsStoreReady || res.StoreReadinessGrade != "B" {
		t.Fatalf("unexpected cross-platform result: %+v", res)
	}
	if res.TotalPermissionsDeclared != 2 || res.SensitivePermissionsCount != 1 || res.TrackingDomainsCount != 2 {
		t.Fatalf("manifest counts are wrong: %+v", res)
	}
	if len(res.Violations) != 1 || !strings.Contains(res.Violations[0].Description, "CAMERA") {
		t.Fatalf("expected CAMERA warning: %+v", res.Violations)
	}

	unknown := t.TempDir()
	res, err = svc.AuditPrivacyAndCompliance(unknown)
	if err != nil {
		t.Fatalf("AuditPrivacyAndCompliance failed: %v", err)
	}
	if res.Platform != "Unknown" || res.StoreReadinessGrade != "N/A" || res.IsStoreReady {
		t.Fatalf("empty repo should be reported as not assessable: %+v", res)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
