package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// =========================================================================
// Connection profiles: one-click switching between saved non-secret
// connection snapshots. Keys stay in the per-origin secret store.
// =========================================================================

func aiProfileID(ai AIConfig) string {
	payload := strings.Join([]string{
		aiMode(ai),
		normalizeAPIOrigin(ai.BaseURL),
		strings.TrimSpace(ai.Model),
		aiProtocol(ai),
		strings.TrimSpace(ai.CLICommand),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])[:12]
}

func aiProfileName(ai AIConfig) string {
	model := strings.TrimSpace(ai.Model)
	if aiMode(ai) == "cli" {
		command := strings.TrimSpace(ai.CLICommand)
		name := filepath.Base(command)
		if name == "" || name == "." {
			name = "CLI"
		}
		if model != "" {
			name += " · " + model
		}
		return truncateRunes(name, 80)
	}
	name := "API"
	if parsed, err := url.Parse(strings.TrimSpace(ai.BaseURL)); err == nil && parsed.Hostname() != "" {
		name = parsed.Hostname()
	}
	if model != "" {
		name += " · " + model
	}
	return truncateRunes(name, 80)
}

// upsertAIProfile records the current connection and marks it active. The ID is
// derived from the connection (not prompt preferences), so editing language or
// headers updates the same profile instead of creating noise.
func upsertAIProfile(ai *AIConfig) string {
	id := aiProfileID(*ai)
	profile := AIProfile{
		ID:              id,
		Name:            aiProfileName(*ai),
		Mode:            aiMode(*ai),
		BaseURL:         strings.TrimSpace(ai.BaseURL),
		Model:           strings.TrimSpace(ai.Model),
		Protocol:        aiProtocol(*ai),
		AzureAPIVersion: strings.TrimSpace(ai.AzureAPIVersion),
		CLICommand:      strings.TrimSpace(ai.CLICommand),
		CLIArgs:         append([]string(nil), ai.CLIArgs...),
		CLIInput:        strings.TrimSpace(ai.CLIInput),
		CLIEnvMode:      cliEnvMode(*ai),
		TimeoutSeconds:  ai.TimeoutSeconds,
		ExtraHeaders:    safeExtraHeaders(ai.ExtraHeaders),
		KeyFingerprint:  ai.APIKey,
		UpdatedAt:       time.Now().Unix(),
	}
	if profile.CLIInput == "" {
		profile.CLIInput = "stdin"
	}
	profiles := make([]AIProfile, 0, len(ai.Profiles)+1)
	profiles = append(profiles, profile)
	for _, existing := range ai.Profiles {
		if existing.ID == id {
			continue
		}
		profiles = append(profiles, existing)
	}
	if len(profiles) > 20 {
		profiles = profiles[:20]
	}
	ai.Profiles = profiles
	// Drop labels for profiles that fell off the cap so the config cannot grow
	// orphan entries over time.
	if len(ai.ProfileNames) > 0 {
		valid := make(map[string]struct{}, len(profiles))
		for _, existing := range profiles {
			valid[existing.ID] = struct{}{}
		}
		for profileID := range ai.ProfileNames {
			if _, ok := valid[profileID]; !ok {
				delete(ai.ProfileNames, profileID)
			}
		}
	}
	ai.ActiveProfile = id
	return id
}

func (AiService) ListAIProfiles() []AIProfile {
	cfg := LoadConfig()
	out := make([]AIProfile, 0, len(cfg.AI.Profiles))
	for _, profile := range cfg.AI.Profiles {
		profile.CLIArgs = append([]string(nil), profile.CLIArgs...)
		profile.ExtraHeaders = safeExtraHeaders(profile.ExtraHeaders)
		profile.Active = profile.ID == cfg.AI.ActiveProfile
		if label := strings.TrimSpace(cfg.AI.ProfileNames[profile.ID]); label != "" {
			profile.Name = label
		}
		out = append(out, profile)
	}
	return out
}

// ActivateAIProfile restores a connection snapshot and resolves the API key
// for its origin from the secret store.
func (AiService) ActivateAIProfile(id string) error {
	id = strings.TrimSpace(id)
	cfg := LoadConfig()
	var profile *AIProfile
	for i := range cfg.AI.Profiles {
		if cfg.AI.Profiles[i].ID == id {
			candidate := cfg.AI.Profiles[i]
			profile = &candidate
			break
		}
	}
	if profile == nil {
		return errAIProfileNotFound
	}
	cfg.AI.Mode = profile.Mode
	cfg.AI.BaseURL = profile.BaseURL
	cfg.AI.Model = profile.Model
	cfg.AI.Protocol = profile.Protocol
	cfg.AI.AzureAPIVersion = profile.AzureAPIVersion
	cfg.AI.CLICommand = profile.CLICommand
	cfg.AI.CLIArgs = append([]string(nil), profile.CLIArgs...)
	cfg.AI.CLIInput = profile.CLIInput
	cfg.AI.CLIEnvMode = profile.CLIEnvMode
	cfg.AI.TimeoutSeconds = clampAITimeout(profile.TimeoutSeconds)
	cfg.AI.ExtraHeaders = safeExtraHeaders(profile.ExtraHeaders)
	cfg.AI.APIKey = ""
	if aiMode(cfg.AI) == "api" {
		origin := normalizeAPIOrigin(profile.BaseURL)
		if stored, err := getAIStoredKeyForOrigin(origin, true); err == nil && stored != "" {
			cfg.AI.APIKey = aiKeyFingerprint(stored)
			setAISessionKeyForOrigin(origin, stored)
		}
	}
	cfg.AI.ActiveProfile = id
	for i := range cfg.AI.Profiles {
		if cfg.AI.Profiles[i].ID == id {
			cfg.AI.Profiles[i].KeyFingerprint = cfg.AI.APIKey
			break
		}
	}
	cfg.SchemaVersion = CurrentSchemaVersion
	return SaveConfig(cfg)
}

var errAIProfileNotFound = errors.New("配置档案不存在")

func (AiService) DeleteAIProfile(id string) error {
	id = strings.TrimSpace(id)
	cfg := LoadConfig()
	profiles := make([]AIProfile, 0, len(cfg.AI.Profiles))
	for _, profile := range cfg.AI.Profiles {
		if profile.ID != id {
			profiles = append(profiles, profile)
		}
	}
	cfg.AI.Profiles = profiles
	if cfg.AI.ActiveProfile == id {
		cfg.AI.ActiveProfile = ""
	}
	delete(cfg.AI.ProfileNames, id)
	cfg.SchemaVersion = CurrentSchemaVersion
	return SaveConfig(cfg)
}

// clearActiveProfileFingerprint keeps the profile display consistent after a
// key is deleted or replaced.
func clearActiveProfileFingerprint(ai *AIConfig) {
	for i := range ai.Profiles {
		if ai.Profiles[i].ID == ai.ActiveProfile {
			ai.Profiles[i].KeyFingerprint = ai.APIKey
			return
		}
	}
}
