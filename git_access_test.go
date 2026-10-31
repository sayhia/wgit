package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"
)

// accessConfigFile plants a config.json whose access.role equals the given raw
// value (which may be intentionally invalid), so LoadConfig / effectiveRole
// see a controlled tier instead of the developer's real settings.
func accessConfigFile(t *testing.T, role string) {
	t.Helper()
	raw, err := json.Marshal(AppConfig{Theme: "dark", Access: AccessConfig{Role: role}, SchemaVersion: CurrentSchemaVersion})
	if err != nil {
		t.Fatalf("marshal access config: %v", err)
	}
	writeFeatureFlagsConfigFile(t, string(raw))
}

// setRoleEnv installs (or removes) WGIT_ROLE for one test and restores the
// previous value afterwards — os.LookupEnv semantics, because t.Setenv cannot
// express "unset" and a developer's exported WGIT_ROLE must not leak into the
// cases that expect no environment override.
func setRoleEnv(t *testing.T, value string, present bool) {
	t.Helper()
	old, hadOld := os.LookupEnv("WGIT_ROLE")
	if present {
		if err := os.Setenv("WGIT_ROLE", value); err != nil {
			t.Fatalf("set WGIT_ROLE: %v", err)
		}
	} else {
		_ = os.Unsetenv("WGIT_ROLE")
	}
	t.Cleanup(func() {
		if hadOld {
			_ = os.Setenv("WGIT_ROLE", old)
		} else {
			_ = os.Unsetenv("WGIT_ROLE")
		}
	})
}

// setServerMode flips the build-mode indirection for one test; t.Cleanup puts
// the compile-time value back so later tests are unaffected.
func setServerMode(t *testing.T, on bool) {
	t.Helper()
	old := accessModeServerFlag
	accessModeServerFlag = on
	t.Cleanup(func() { accessModeServerFlag = old })
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Role
		wantErr bool
	}{
		{name: "viewer", raw: "viewer", want: RoleViewer},
		{name: "operator", raw: "operator", want: RoleOperator},
		{name: "admin", raw: "admin", want: RoleAdmin},
		{name: "case-insensitive", raw: "AdMiN", want: RoleAdmin},
		{name: "operator uppercase", raw: "OPERATOR", want: RoleOperator},
		{name: "surrounding whitespace", raw: "  viewer\t", want: RoleViewer},
		{name: "empty is not a role", raw: "", wantErr: true},
		{name: "root is rejected", raw: "root", wantErr: true},
		{name: "sudo is rejected", raw: "sudo", wantErr: true},
		{name: "trailing whitespace tolerated", raw: "admin ", want: RoleAdmin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRole(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRole(%q) = %v, want an error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRole(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseRole(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			if got.String() != strings.ToLower(strings.TrimSpace(tt.raw)) {
				t.Errorf("parseRole(%q).String() = %q, want the lowercased wire name", tt.raw, got)
			}
		})
	}
}

func TestRoleOrdering(t *testing.T) {
	if !(RoleViewer < RoleOperator && RoleOperator < RoleAdmin) {
		t.Fatalf("role ordering broken: viewer=%d operator=%d admin=%d", RoleViewer, RoleOperator, RoleAdmin)
	}
	if RoleViewer.String() != "viewer" || RoleOperator.String() != "operator" || RoleAdmin.String() != "admin" {
		t.Errorf("String() wire names drifted: %v %v %v", RoleViewer, RoleOperator, RoleAdmin)
	}
}

func TestEffectiveRoleDesktopIsAlwaysAdmin(t *testing.T) {
	setServerMode(t, false)
	accessConfigFile(t, "viewer")
	setRoleEnv(t, "viewer", true)

	// Desktop builds ignore both the config tier and the environment: the
	// single local user is admin, zero behaviour change from before §3.4.
	if got := effectiveRole(); got != RoleAdmin {
		t.Errorf("effectiveRole() in desktop mode = %v, want RoleAdmin regardless of config/env", got)
	}
	if err := ensureRole(RoleAdmin); err != nil {
		t.Errorf("ensureRole(RoleAdmin) in desktop mode = %v, want nil (always admin)", err)
	}
}

func TestEffectiveRoleServerMatrix(t *testing.T) {
	tests := []struct {
		name      string
		configSum string // config.json content; "" plants no file at all
		envRole   string
		envSet    bool
		want      Role
	}{
		{
			name:   "no config file defaults to admin",
			envSet: false,
			want:   RoleAdmin,
		},
		{
			name:      "empty access.role defaults to admin",
			configSum: `{"theme":"dark","schemaVersion":2}`,
			envSet:    false,
			want:      RoleAdmin,
		},
		{
			name:      "config viewer",
			configSum: `{"access":{"role":"viewer"},"schemaVersion":2}`,
			envSet:    false,
			want:      RoleViewer,
		},
		{
			name:      "config operator",
			configSum: `{"access":{"role":"OPERATOR"},"schemaVersion":2}`,
			envSet:    false,
			want:      RoleOperator,
		},
		{
			name:      "invalid config role falls back to admin",
			configSum: `{"access":{"role":"root"},"schemaVersion":2}`,
			envSet:    false,
			want:      RoleAdmin,
		},
		{
			name:      "env viewer downgrades an admin config",
			configSum: `{"access":{"role":"admin"},"schemaVersion":2}`,
			envRole:   "viewer",
			envSet:    true,
			want:      RoleViewer,
		},
		{
			name:      "env admin upgrades a viewer config",
			configSum: `{"access":{"role":"viewer"},"schemaVersion":2}`,
			envRole:   "ADMIN",
			envSet:    true,
			want:      RoleAdmin,
		},
		{
			name:      "invalid env value is ignored, config tier survives",
			configSum: `{"access":{"role":"operator"},"schemaVersion":2}`,
			envRole:   "maybe",
			envSet:    true,
			want:      RoleOperator,
		},
		{
			name:      "empty env value is ignored, config tier survives",
			configSum: `{"access":{"role":"viewer"},"schemaVersion":2}`,
			envRole:   "",
			envSet:    true,
			want:      RoleViewer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setServerMode(t, true)
			if tt.configSum == "" {
				writeFeatureFlagsConfigFile(t, `{"theme":"dark"}`)
				// A config file is planted anyway so HOME is isolated; the
				// "no file" default is covered by TestLoadConfigAccessDefaults.
				_ = os.Remove(getConfigPath())
			} else {
				writeFeatureFlagsConfigFile(t, tt.configSum)
			}
			setRoleEnv(t, tt.envRole, tt.envSet)

			if got := effectiveRole(); got != tt.want {
				t.Errorf("effectiveRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureRoleMatrix(t *testing.T) {
	// Pin the effective tier so the matrix is about ensureRole alone.
	setServerMode(t, true)
	accessConfigFile(t, "operator")
	setRoleEnv(t, "", false)

	tests := []struct {
		min     Role
		allowed bool
	}{
		{RoleViewer, true},
		{RoleOperator, true},
		{RoleAdmin, false},
	}
	for _, tt := range tests {
		err := ensureRole(tt.min)
		if tt.allowed && err != nil {
			t.Errorf("ensureRole(%v) as operator = %v, want nil", tt.min, err)
		}
		if !tt.allowed {
			want := "forbidden: requires admin role (current: operator)"
			if err == nil {
				t.Errorf("ensureRole(%v) as operator = nil, want %q", tt.min, want)
			} else if err.Error() != want {
				t.Errorf("ensureRole(%v) as operator = %q, want exact wire message %q", tt.min, err, want)
			}
		}
	}
}

func TestLoadConfigAccessDefaultsAndWarnings(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		wantRole   string
		wantWarn   bool
	}{
		{
			name:       "missing access object defaults to admin",
			configJSON: `{"theme":"dark","schemaVersion":2}`,
			wantRole:   "admin",
		},
		{
			name:       "valid role is normalised to lowercase",
			configJSON: `{"access":{"role":"Operator"},"schemaVersion":2}`,
			wantRole:   "operator",
		},
		{
			name:       "invalid role falls back to admin with a warning",
			configJSON: `{"access":{"role":"superuser"},"schemaVersion":2}`,
			wantRole:   "admin",
			wantWarn:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFeatureFlagsConfigFile(t, tt.configJSON)
			setRoleEnv(t, "", false)

			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(os.Stderr)

			cfg := LoadConfig()

			if cfg.Access.Role != tt.wantRole {
				t.Errorf("Access.Role = %q, want %q", cfg.Access.Role, tt.wantRole)
			}
			out := buf.String()
			if tt.wantWarn {
				if n := strings.Count(out, "\n"); n != 1 {
					t.Fatalf("LoadConfig logged %d lines, want exactly one [Access] warning: %q", n, out)
				}
				if !strings.Contains(out, "[Access]") || !strings.Contains(out, `"superuser"`) {
					t.Errorf("warning %q does not carry the [Access] tag and the rejected value", out)
				}
			} else if strings.Contains(out, "[Access]") {
				t.Errorf("unexpected [Access] log line: %q", out)
			}
		})
	}
}

func TestWGITRoleOverrideDoesNotRewriteConfigFile(t *testing.T) {
	original := `{"theme":"dark","access":{"role":"viewer"},"schemaVersion":2}`
	writeFeatureFlagsConfigFile(t, original)
	setRoleEnv(t, "admin", true)

	cfg := LoadConfig()
	if cfg.Access.Role != "admin" {
		t.Fatalf("Access.Role = %q, want the WGIT_ROLE override to win in memory", cfg.Access.Role)
	}

	// Same memory-only contract as WGIT_FLAG_*: config.json stays byte-for-byte
	// identical, and clearing the env brings the file tier back.
	raw, err := os.ReadFile(getConfigPath())
	if err != nil {
		t.Fatalf("re-read config.json: %v", err)
	}
	if string(raw) != original {
		t.Errorf("LoadConfig under WGIT_ROLE rewrote config.json:\n got %q\nwant %q", string(raw), original)
	}
	os.Unsetenv("WGIT_ROLE")
	if got := LoadConfig(); got.Access.Role != "viewer" {
		t.Errorf("Access.Role after clearing the env = %q, want the file value viewer", got.Access.Role)
	}
}

// TestRoleGatedRPCsDenyViewerAllowAdmin walks two gated entry points — one
// operator-level (ResetBranch) and one admin-level (RemediateStagedSecrets) —
// in server mode. A viewer must be stopped by the precondition alone, an admin
// must get past it into the regular argument validation (proof the guard did
// not disturb the existing logic).
func TestRoleGatedRPCsDenyViewerAllowAdmin(t *testing.T) {
	setServerMode(t, true)
	accessConfigFile(t, "viewer")
	setRoleEnv(t, "", false)

	if err := (GitService{}).ResetBranch("", "abc", "hard"); err == nil || err.Error() != "forbidden: requires operator role (current: viewer)" {
		t.Errorf("viewer ResetBranch error = %v, want the exact operator forbidden message", err)
	}
	if _, err := (GitService{}).RemediateStagedSecrets("", nil, false, false); err == nil || err.Error() != "forbidden: requires admin role (current: viewer)" {
		t.Errorf("viewer RemediateStagedSecrets error = %v, want the exact admin forbidden message", err)
	}

	accessConfigFile(t, "admin")
	if err := (GitService{}).ResetBranch("", "abc", "hard"); err == nil || err.Error() != "repoPath cannot be empty" {
		t.Errorf("admin ResetBranch error = %v, want the ordinary validation error past the guard", err)
	}
	if _, err := (GitService{}).RemediateStagedSecrets("", nil, false, false); err == nil || err.Error() != "repository path is required" {
		t.Errorf("admin RemediateStagedSecrets error = %v, want the ordinary validation error past the guard", err)
	}
}

// TestAccessGatedRPCsPassInDesktopMode is the zero-behaviour-change guarantee:
// a desktop build (accessModeServer false) lets every caller through even when
// both the config and the environment ask for viewer.
func TestAccessGatedRPCsPassInDesktopMode(t *testing.T) {
	setServerMode(t, false)
	accessConfigFile(t, "viewer")
	setRoleEnv(t, "viewer", true)

	if err := (GitService{}).ResetBranch("", "abc", "hard"); err == nil || err.Error() != "repoPath cannot be empty" {
		t.Errorf("desktop ResetBranch error = %v, want the ordinary validation error (guard must be inert)", err)
	}
	if _, err := (GitService{}).RemediateStagedSecrets("", nil, false, false); err == nil || err.Error() != "repository path is required" {
		t.Errorf("desktop RemediateStagedSecrets error = %v, want the ordinary validation error (guard must be inert)", err)
	}
	// An unsupported hook name proves the call moved past the guard into the
	// regular hook-name validation (and touches no filesystem state).
	if err := (GitService{}).ToggleGitHook("", "not-a-hook", false, ""); err == nil || !strings.Contains(err.Error(), "not-a-hook") {
		t.Errorf("desktop ToggleGitHook error = %v, want it to proceed past the guard into hook-name validation", err)
	}
}

// TestServerModeGatesRejectViewerForMutatingRPCs pins the second-wave access
// gates: in server mode a viewer gets the forbidden wire error from every
// newly gated mutating RPC, before any repo access happens (empty args are
// enough — the gate fires first).
func TestServerModeGatesRejectViewerForMutatingRPCs(t *testing.T) {
	old := accessModeServerFlag
	accessModeServerFlag = true
	t.Cleanup(func() { accessModeServerFlag = old })
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2,"theme":"dark","access":{"role":"viewer"}}`)

	svc := GitService{}
	for name, call := range map[string]func() error{
		"DeleteTag":            func() error { return svc.DeleteTag("", "") },
		"DeleteRemoteBranch":   func() error { return svc.DeleteRemoteBranch("", "", "") },
		"DiscardAll":           func() error { return svc.DiscardAll("") },
		"DiscardFile":          func() error { return svc.DiscardFile("", "") },
		"DiscardUntrackedFile": func() error { return svc.DiscardUntrackedFile("", "") },
		"CherryPick":           func() error { return svc.CherryPick("", "") },
		"RevertCommit":         func() error { return svc.RevertCommit("", "") },
		"StashApply":           func() error { return svc.StashApply("", 0) },
		"StashPop":             func() error { return svc.StashPop("", 0) },
		"StashDrop":            func() error { return svc.StashDrop("", 0) },
		"StashClear":           func() error { return svc.StashClear("") },
		"RenameBranch":         func() error { return svc.RenameBranch("", "", "") },
		"RemoveRemote":         func() error { return svc.RemoveRemote("", "") },
		"SetGitProxy":          func() error { return svc.SetGitProxy("", "", false) },
		"SetUpstream":          func() error { return svc.SetUpstream("", "", "") },
		"UnsetUpstream":        func() error { return svc.UnsetUpstream("", "") },
		"PushAdvanced": func() error {
			_, err := svc.PushAdvanced("", PushAdvancedOptions{})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil {
				t.Fatalf("%s succeeded for a viewer, want a forbidden error", name)
			}
			if !strings.Contains(err.Error(), "forbidden") {
				t.Errorf("%s error = %v, want a forbidden mention", name, err)
			}
		})
	}
}
