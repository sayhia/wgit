package main

import (
	"fmt"
	"strings"
)

// Role is the access tier from plan §3.4: viewer (read-only surfaces),
// operator (day-to-day writes) and admin (everything, including maintenance
// and secrets remediation). The tiers are ordered so comparisons read
// naturally: a role satisfies ensureRole(min) when it is >= min.
type Role int

const (
	RoleViewer Role = iota
	RoleOperator
	RoleAdmin
)

// String renders the wire name of the role, which is also the spelling
// accepted by parseRole and persisted in config.json ("role" field).
func (r Role) String() string {
	switch r {
	case RoleViewer:
		return "viewer"
	case RoleOperator:
		return "operator"
	case RoleAdmin:
		return "admin"
	default:
		return fmt.Sprintf("role(%d)", int(r))
	}
}

// parseRole converts a config/environment role name into a Role. Matching is
// case-insensitive and tolerates surrounding whitespace, mirroring the
// WGIT_FLAG_* value parsing; anything else is an error so callers can fall
// back to a safe default.
func parseRole(raw string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "viewer":
		return RoleViewer, nil
	case "operator":
		return RoleOperator, nil
	case "admin":
		return RoleAdmin, nil
	}
	return RoleViewer, fmt.Errorf("invalid role %q (want viewer, operator or admin)", raw)
}

// accessModeServerFlag re-states the compile-time accessModeServer constant as
// a var so tests can flip the mode at runtime (t.Cleanup restores it). In
// production it is fixed at link time by the build tags on access_mode*.go.
var accessModeServerFlag = accessModeServer

// effectiveRole resolves the caller's tier. Desktop builds (accessModeServer
// false) never consult config or environment: the single local user is always
// admin, so behaviour is byte-for-byte identical to before the access layer.
// Server builds take the tier from AppConfig.Access (defaulting to admin),
// with a valid WGIT_ROLE environment value overriding the file — the same
// env-beats-file contract as the WGIT_FLAG_* variables.
func effectiveRole() Role {
	if !accessModeServerFlag {
		return RoleAdmin
	}
	role := RoleAdmin
	// LoadConfig already normalises an invalid/missing access.role to "admin"
	// (applyAccessDefaults), so a parse failure here is only a defensive
	// fallback that keeps the admin tier rather than locking the server out.
	if parsed, err := parseRole(LoadConfig().Access.Role); err == nil {
		role = parsed
	}
	return role
}

// ensureRole reports whether the effective tier is at least min, returning the
// wire error that gated RPCs surface to the frontend otherwise. Every guard is
// a pure precondition check: passing roles proceed to untouched logic.
func ensureRole(min Role) error {
	current := effectiveRole()
	if current >= min {
		return nil
	}
	return fmt.Errorf("forbidden: requires %s role (current: %s)", min, current)
}
