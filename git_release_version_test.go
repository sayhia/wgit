package main

import "testing"

// TestSuggestNextVersions pins the release-version contract used by
// GetReleasePlan: the suggested bump, the isMinor flag, and the three explicit
// semver options shown in the release wizard.
func TestSuggestNextVersions(t *testing.T) {
	type want struct {
		suggested string
		isMinor   bool
		patch     string
		minor     string
		major     string
	}
	semverBumps := func(suggested string, isMinor bool, patch, minor, major string) want {
		return want{suggested: suggested, isMinor: isMinor, patch: patch, minor: minor, major: major}
	}
	v123 := func(suggested string, isMinor bool) want {
		return semverBumps(suggested, isMinor, "v1.2.4", "v1.3.0", "v2.0.0")
	}

	tests := []struct {
		name     string
		lastTag  string
		subjects []string
		want     want
	}{
		{
			name:    "v1.2.3 with no subjects suggests patch",
			lastTag: "v1.2.3",
			want:    v123("v1.2.4", false),
		},
		{
			name:     "v1.2.3 with fix and docs suggests patch",
			lastTag:  "v1.2.3",
			subjects: []string{"fix: x", "docs: y"},
			want:     v123("v1.2.4", false),
		},
		{
			name:     "feat colon triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"feat: x"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "art scope triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"art(character): y"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "ui bang triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"ui!: z"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "level colon triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"level: 新关卡"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "asset scope triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"asset(weapon): 新武器"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "audio colon triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"audio: 新音效"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "design bang triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"design!: 新设计"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "example feature trio triggers minor",
			lastTag:  "v1.2.3",
			subjects: []string{"feat: x", "art(character): y", "ui!: z"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "feature word needs a delimiter to trigger minor",
			lastTag:  "v1.2.3",
			subjects: []string{"feature: x", "audio-visual: y"},
			want:     v123("v1.2.4", false),
		},
		{
			name:     "subjects are matched case-insensitively after trimming",
			lastTag:  "v1.2.3",
			subjects: []string{"  FEAT(core): 大写提交  "},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "tag whitespace is trimmed before matching",
			lastTag:  " v1.2.3 ",
			subjects: []string{"fix: x"},
			want:     v123("v1.2.4", false),
		},
		{
			name:     "tag without v keeps empty prefix on patch bump",
			lastTag:  "1.2.3",
			subjects: []string{"fix: x"},
			want:     semverBumps("1.2.4", false, "1.2.4", "1.3.0", "2.0.0"),
		},
		{
			name:     "tag without v keeps empty prefix on minor bump",
			lastTag:  "1.2.3",
			subjects: []string{"feat: x"},
			want:     semverBumps("1.3.0", true, "1.2.4", "1.3.0", "2.0.0"),
		},
		{
			// Documents current behavior: with no tag the fallback still adds a
			// "v" prefix even though no tag carried one, and it returns isMinor
			// false regardless of commit subjects.
			name:    "empty tag without subjects falls back to v0.1.0",
			lastTag: "",
			want:    semverBumps("v0.1.0", false, "v0.0.1", "v0.1.0", "v1.0.0"),
		},
		{
			// Same anomaly as above: feat: x is detected as a feature commit
			// but the fallback branch discards the flag and still suggests
			// v0.1.0 instead of a minor bump.
			name:     "empty tag with feature subject still falls back to v0.1.0",
			lastTag:  "",
			subjects: []string{"feat: x"},
			want:     semverBumps("v0.1.0", false, "v0.0.1", "v0.1.0", "v1.0.0"),
		},
		{
			name:     "non-semver tag without v uses bare fallback versions",
			lastTag:  "release-2026",
			subjects: []string{"feat: x"},
			want:     semverBumps("0.1.0", false, "0.0.1", "0.1.0", "1.0.0"),
		},
		{
			name:     "incomplete semver with v keeps v in fallback versions",
			lastTag:  "v1.2",
			subjects: []string{"fix: x"},
			want:     semverBumps("v0.1.0", false, "v0.0.1", "v0.1.0", "v1.0.0"),
		},
		{
			name:     "prerelease tag keeps base and drops suffix on minor bump",
			lastTag:  "v1.2.3-rc.1",
			subjects: []string{"feat: x"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "prerelease tag keeps base and drops suffix on patch bump",
			lastTag:  "v1.2.3-rc.1",
			subjects: []string{"fix: x"},
			want:     v123("v1.2.4", false),
		},
		{
			name:     "build metadata tag keeps base and drops suffix",
			lastTag:  "v1.2.3+build.5",
			subjects: []string{"fix: x"},
			want:     v123("v1.2.4", false),
		},
		{
			name:     "build metadata tag triggers minor on feature commit",
			lastTag:  "v1.2.3+build.5",
			subjects: []string{"feat: x"},
			want:     v123("v1.3.0", true),
		},
		{
			name:     "uppercase V prefix is preserved on patch bump",
			lastTag:  "V1.2.3",
			subjects: []string{"fix: x"},
			want:     semverBumps("V1.2.4", false, "V1.2.4", "V1.3.0", "V2.0.0"),
		},
		{
			name:     "uppercase V prefix is preserved on minor bump",
			lastTag:  "V1.2.3",
			subjects: []string{"feat: x"},
			want:     semverBumps("V1.3.0", true, "V1.2.4", "V1.3.0", "V2.0.0"),
		},
		{
			name:     "uppercase V2.0.0 keeps line on patch bump",
			lastTag:  "V2.0.0",
			subjects: []string{"fix: x"},
			want:     semverBumps("V2.0.1", false, "V2.0.1", "V2.1.0", "V3.0.0"),
		},
		{
			name:     "uppercase V2.0.0 keeps line on minor bump",
			lastTag:  "V2.0.0",
			subjects: []string{"feat: x"},
			want:     semverBumps("V2.1.0", true, "V2.0.1", "V2.1.0", "V3.0.0"),
		},
		{
			name:    "uppercase V2.0.0 without subjects suggests patch",
			lastTag: "V2.0.0",
			want:    semverBumps("V2.0.1", false, "V2.0.1", "V2.1.0", "V3.0.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggested, isMinor, patchVer, minorVer, majorVer := suggestNextVersions(tt.lastTag, tt.subjects)
			if suggested != tt.want.suggested {
				t.Fatalf("suggested = %q, want %q", suggested, tt.want.suggested)
			}
			if isMinor != tt.want.isMinor {
				t.Fatalf("isMinor = %v, want %v", isMinor, tt.want.isMinor)
			}
			if patchVer != tt.want.patch {
				t.Fatalf("patchVer = %q, want %q", patchVer, tt.want.patch)
			}
			if minorVer != tt.want.minor {
				t.Fatalf("minorVer = %q, want %q", minorVer, tt.want.minor)
			}
			if majorVer != tt.want.major {
				t.Fatalf("majorVer = %q, want %q", majorVer, tt.want.major)
			}
		})
	}
}

func TestSuggestNextVersionWrapper(t *testing.T) {
	tests := []struct {
		name     string
		lastTag  string
		subjects []string
		want     string
		isMinor  bool
	}{
		{name: "patch", lastTag: "v1.2.3", subjects: []string{"fix: x"}, want: "v1.2.4"},
		{name: "minor", lastTag: "v1.2.3", subjects: []string{"feat: x"}, want: "v1.3.0", isMinor: true},
		{name: "no tag", lastTag: "", subjects: []string{"feat: x"}, want: "v0.1.0"},
		{name: "no v tag", lastTag: "1.2.3", subjects: []string{"ui!: z"}, want: "1.3.0", isMinor: true},
		{name: "fallback", lastTag: "v1.2", subjects: nil, want: "v0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotMinor := suggestNextVersion(tt.lastTag, tt.subjects)
			if got != tt.want || gotMinor != tt.isMinor {
				t.Fatalf("suggestNextVersion() = (%q, %v), want (%q, %v)", got, gotMinor, tt.want, tt.isMinor)
			}

			// The wrapper must agree with the full function it delegates to.
			suggested, isMinor, _, _, _ := suggestNextVersions(tt.lastTag, tt.subjects)
			if got != suggested || gotMinor != isMinor {
				t.Fatalf("wrapper = (%q, %v), suggestNextVersions = (%q, %v)", got, gotMinor, suggested, isMinor)
			}
		})
	}
}
