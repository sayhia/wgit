package main

import (
	"path/filepath"
	"testing"
)

func TestAIModelCatalogPricing(t *testing.T) {
	catalog := (AiService{}).GetAIProviderCatalog()
	if len(catalog.Providers) < 6 || len(catalog.Models) < 8 {
		t.Fatalf("catalog too small: %d providers, %d models", len(catalog.Providers), len(catalog.Models))
	}
	price, ok := aiModelPriceFor("gpt-4.1-mini-2025-04-14")
	if !ok || price.Model != "gpt-4.1-mini" {
		t.Fatalf("dated model price = %+v ok=%v", price, ok)
	}
	if _, ok := aiModelPriceFor("openai/gpt-4.1-mini"); !ok {
		t.Fatal("namespaced model did not resolve")
	}

	cloud := AIConfig{Mode: "api", BaseURL: "https://api.example.com/v1"}
	cost, known := aiCostForModel(cloud, "gpt-4.1-mini", 1_000_000, 1_000_000)
	if !known || cost < 1.999 || cost > 2.001 {
		t.Fatalf("catalog cost = %v known=%v, want 2.0", cost, known)
	}
	cost, known = aiCostForModel(cloud, "totally-unknown-model", 1000, 1000)
	if known || cost != 0 {
		t.Fatalf("unknown cloud cost = %v known=%v", cost, known)
	}
	local := AIConfig{Mode: "api", BaseURL: "http://localhost:11434/v1"}
	if cost, known := aiCostForModel(local, "some-local-model", 1000, 1000); !known || cost != 0 {
		t.Fatalf("local cost = %v known=%v", cost, known)
	}
	cli := AIConfig{Mode: "cli", CLICommand: "claude"}
	if cost, known := aiCostForModel(cli, "unknown", 1000, 1000); known || cost != 0 {
		t.Fatalf("cli cost = %v known=%v", cost, known)
	}
}

func TestAIProfilesAutoUpsertAndSwitch(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "https://a.example.com/v1", Model: "model-a", TimeoutSeconds: 10,
	}); err != nil {
		t.Fatalf("save A: %v", err)
	}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "https://b.example.com/v1", Model: "model-b", TimeoutSeconds: 20,
	}); err != nil {
		t.Fatalf("save B: %v", err)
	}
	profiles := svc.ListAIProfiles()
	if len(profiles) != 2 {
		t.Fatalf("profiles = %+v", profiles)
	}
	if profiles[0].Model != "model-b" || !profiles[0].Active {
		t.Fatalf("newest profile should be active: %+v", profiles[0])
	}
	firstID := profiles[1].ID
	if err := svc.ActivateAIProfile(firstID); err != nil {
		t.Fatalf("activate: %v", err)
	}
	cfg := LoadConfig().AI
	if cfg.BaseURL != "https://a.example.com/v1" || cfg.Model != "model-a" || cfg.TimeoutSeconds != 10 {
		t.Fatalf("activated config = %+v", cfg)
	}
	profiles = svc.ListAIProfiles()
	if !profiles[1].Active {
		t.Fatalf("profile not marked active: %+v", profiles)
	}
	if err := svc.DeleteAIProfile(firstID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	profiles = svc.ListAIProfiles()
	if len(profiles) != 1 || profiles[0].Active || LoadConfig().AI.ActiveProfile != "" {
		t.Fatalf("delete active profile left state: %+v active=%q", profiles, LoadConfig().AI.ActiveProfile)
	}
	if err := svc.ActivateAIProfile("missing"); err == nil {
		t.Fatal("missing profile was accepted")
	}
}

func TestAIRepoOverrideRPCs(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	repo := initTestRepo(t)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "https://api.example.com/v1", Model: "global-model", TimeoutSeconds: 10,
		CommitLanguage: "zh", CommitIncludeBody: true,
	}); err != nil {
		t.Fatalf("save global: %v", err)
	}
	override := AIRepoOverride{Model: "repo-model"}
	if err := svc.SaveAIRepoOverride(repo, override); err != nil {
		t.Fatalf("save override: %v", err)
	}
	got := svc.GetAIRepoOverride(filepath.Clean(repo))
	if got.Model != "repo-model" || got.CommitLanguage != "" || got.CommitIncludeBody != nil {
		t.Fatalf("override = %+v", got)
	}
	resolved := aiResolvedAIConfig(LoadConfig().AI, repo, "commit")
	if resolved.Model != "repo-model" || resolved.CommitLanguage != "zh" || !resolved.CommitIncludeBody {
		t.Fatalf("sparse repo override did not inherit globals: %+v", resolved)
	}
	if err := svc.DeleteAIRepoOverride(repo); err != nil {
		t.Fatalf("delete override: %v", err)
	}
	if got := svc.GetAIRepoOverride(repo); got.Model != "" || got.CommitLanguage != "" {
		t.Fatalf("override survived delete: %+v", got)
	}
	if resolved := aiResolvedAIConfig(LoadConfig().AI, repo, "commit"); resolved.Model != "global-model" {
		t.Fatalf("global model not restored: %+v", resolved)
	}
	if err := svc.SaveAIRepoOverride("relative/path", AIRepoOverride{Model: "x"}); err == nil {
		t.Fatal("relative repo path was accepted")
	}
}

func TestAIProfileRenameAndLabelCleanup(t *testing.T) {
	writeFeatureFlagsConfigFile(t, `{"schemaVersion":2}`)
	resetAISessionKeyForTest(t)
	svc := AiService{}
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "https://a.example.com/v1", Model: "model-a", TimeoutSeconds: 10,
	}); err != nil {
		t.Fatalf("save A: %v", err)
	}
	profiles := svc.ListAIProfiles()
	if len(profiles) != 1 {
		t.Fatalf("profiles = %+v", profiles)
	}
	id := profiles[0].ID

	advanced := svc.GetAIAdvancedConfig()
	advanced.ProfileNames = map[string]string{id: "  生产端点  ", "missing": "Ghost"}
	if err := svc.SaveAIAdvancedConfig(advanced); err != nil {
		t.Fatalf("save advanced: %v", err)
	}
	profiles = svc.ListAIProfiles()
	if profiles[0].Name != "生产端点" {
		t.Fatalf("renamed profile = %q", profiles[0].Name)
	}
	if got := svc.GetAIAdvancedConfig().ProfileNames; len(got) != 1 || got[id] != "生产端点" {
		t.Fatalf("stored labels = %+v", got)
	}

	// Re-saving the same connection must not overwrite the user label.
	if err := svc.SaveAISettings(AISaveInput{
		Mode: "api", BaseURL: "https://a.example.com/v1", Model: "model-a", TimeoutSeconds: 10,
	}); err != nil {
		t.Fatalf("re-save A: %v", err)
	}
	if profiles = svc.ListAIProfiles(); profiles[0].Name != "生产端点" {
		t.Fatalf("label lost after upsert: %q", profiles[0].Name)
	}

	if err := svc.DeleteAIProfile(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := svc.GetAIAdvancedConfig().ProfileNames; len(got) != 0 {
		t.Fatalf("labels survived delete: %+v", got)
	}
}
