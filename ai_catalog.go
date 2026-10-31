package main

import (
	"net"
	"net/url"
	"strings"
)

// =========================================================================
// Provider/model catalog. Presets remove most manual endpoint/protocol/model
// configuration from the settings page; prices power the usage estimate
// without asking users to type per-million-token rates.
// =========================================================================

type AIProviderPreset struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Mode         string   `json:"mode"` // api
	Protocol     string   `json:"protocol"`
	BaseURL      string   `json:"baseUrl"`
	Models       []string `json:"models"`
	DefaultModel string   `json:"defaultModel"`
	KeyRequired  bool     `json:"keyRequired"`
	Local        bool     `json:"local,omitempty"`
}

// AIModelPrice is a reference price in USD per one million tokens. Values are
// approximate list prices and are used only for the local estimate.
type AIModelPrice struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	InputUSDPerMTok  float64 `json:"inputUsdPerMTok"`
	OutputUSDPerMTok float64 `json:"outputUsdPerMTok"`
	ContextWindow    int     `json:"contextWindow"`
}

type AIProviderCatalog struct {
	Providers []AIProviderPreset `json:"providers"`
	Models    []AIModelPrice     `json:"models"`
}

var aiProviderPresets = []AIProviderPreset{
	{
		ID: "openai", Name: "OpenAI", Mode: "api", Protocol: "openai",
		BaseURL: "https://api.openai.com/v1", KeyRequired: true,
		Models: []string{"gpt-4.1-mini", "gpt-4.1", "o4-mini"}, DefaultModel: "gpt-4.1-mini",
	},
	{
		ID: "anthropic", Name: "Anthropic", Mode: "api", Protocol: "anthropic",
		BaseURL: "https://api.anthropic.com/v1", KeyRequired: true,
		Models: []string{"claude-sonnet-4-5", "claude-haiku-4-5", "claude-opus-4-1"}, DefaultModel: "claude-sonnet-4-5",
	},
	{
		ID: "gemini", Name: "Google Gemini", Mode: "api", Protocol: "gemini",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta", KeyRequired: true,
		Models: []string{"gemini-2.5-flash", "gemini-2.5-pro"}, DefaultModel: "gemini-2.5-flash",
	},
	{
		ID: "deepseek", Name: "DeepSeek", Mode: "api", Protocol: "openai",
		BaseURL: "https://api.deepseek.com/v1", KeyRequired: true,
		Models: []string{"deepseek-chat", "deepseek-reasoner"}, DefaultModel: "deepseek-chat",
	},
	{
		ID: "openrouter", Name: "OpenRouter", Mode: "api", Protocol: "openai",
		BaseURL: "https://openrouter.ai/api/v1", KeyRequired: true,
		Models:       []string{"openai/gpt-4.1-mini", "anthropic/claude-sonnet-4-5", "google/gemini-2.5-flash"},
		DefaultModel: "openai/gpt-4.1-mini",
	},
	{
		ID: "azure", Name: "Azure OpenAI", Mode: "api", Protocol: "azure",
		BaseURL: "", KeyRequired: true, Models: nil,
	},
	{
		ID: "ollama", Name: "Ollama", Mode: "api", Protocol: "openai",
		BaseURL: "http://localhost:11434/v1", KeyRequired: false, Local: true,
		Models: []string{"llama3.1", "qwen2.5-coder"}, DefaultModel: "llama3.1",
	},
	{
		ID: "lmstudio", Name: "LM Studio", Mode: "api", Protocol: "openai",
		BaseURL: "http://localhost:1234/v1", KeyRequired: false, Local: true,
		Models: nil,
	},
}

var aiModelPrices = []AIModelPrice{
	{Model: "gpt-4.1-mini", Provider: "openai", InputUSDPerMTok: 0.40, OutputUSDPerMTok: 1.60, ContextWindow: 1047576},
	{Model: "gpt-4.1", Provider: "openai", InputUSDPerMTok: 2.00, OutputUSDPerMTok: 8.00, ContextWindow: 1047576},
	{Model: "o4-mini", Provider: "openai", InputUSDPerMTok: 1.10, OutputUSDPerMTok: 4.40, ContextWindow: 200000},
	{Model: "gpt-4o-mini", Provider: "openai", InputUSDPerMTok: 0.15, OutputUSDPerMTok: 0.60, ContextWindow: 128000},
	{Model: "claude-sonnet-4-5", Provider: "anthropic", InputUSDPerMTok: 3.00, OutputUSDPerMTok: 15.00, ContextWindow: 200000},
	{Model: "claude-haiku-4-5", Provider: "anthropic", InputUSDPerMTok: 1.00, OutputUSDPerMTok: 5.00, ContextWindow: 200000},
	{Model: "claude-opus-4-1", Provider: "anthropic", InputUSDPerMTok: 15.00, OutputUSDPerMTok: 75.00, ContextWindow: 200000},
	{Model: "gemini-2.5-flash", Provider: "gemini", InputUSDPerMTok: 0.30, OutputUSDPerMTok: 2.50, ContextWindow: 1048576},
	{Model: "gemini-2.5-pro", Provider: "gemini", InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10.00, ContextWindow: 1048576},
	{Model: "deepseek-chat", Provider: "deepseek", InputUSDPerMTok: 0.27, OutputUSDPerMTok: 1.10, ContextWindow: 65536},
	{Model: "deepseek-reasoner", Provider: "deepseek", InputUSDPerMTok: 0.55, OutputUSDPerMTok: 2.19, ContextWindow: 65536},
	{Model: "llama3.1", Provider: "local", InputUSDPerMTok: 0, OutputUSDPerMTok: 0, ContextWindow: 131072},
	{Model: "qwen2.5-coder", Provider: "local", InputUSDPerMTok: 0, OutputUSDPerMTok: 0, ContextWindow: 32768},
}

// GetAIProviderCatalog exposes presets and reference prices to the quick setup.
func (AiService) GetAIProviderCatalog() AIProviderCatalog {
	providers := make([]AIProviderPreset, 0, len(aiProviderPresets))
	for _, preset := range aiProviderPresets {
		preset.Models = append([]string(nil), preset.Models...)
		providers = append(providers, preset)
	}
	models := append([]AIModelPrice(nil), aiModelPrices...)
	return AIProviderCatalog{Providers: providers, Models: models}
}

// aiModelPriceFor matches exact names first, then dated/namespaced variants
// (e.g. claude-sonnet-4-5-20250929, openai/gpt-4.1-mini) by longest prefix.
func aiModelPriceFor(model string) (AIModelPrice, bool) {
	key := strings.ToLower(strings.TrimSpace(model))
	if key == "" {
		return AIModelPrice{}, false
	}
	if index := strings.LastIndex(key, "/"); index >= 0 {
		key = key[index+1:]
	}
	var best AIModelPrice
	found := false
	for _, price := range aiModelPrices {
		candidate := strings.ToLower(price.Model)
		if key == candidate || strings.HasPrefix(key, candidate) {
			if !found || len(candidate) > len(best.Model) {
				best = price
				found = true
			}
		}
	}
	return best, found
}

func isLocalAIEndpoint(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return false
	}
	if host == "localhost" || host == "0.0.0.0" || host == "::1" || strings.HasSuffix(host, ".local") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// aiCostForModel returns the estimated cost and whether the model price is
// known. Local endpoints and local models are free; unknown cloud models fall
// back to legacy user-configured rates when present.
func aiCostForModel(cfg AIConfig, model string, tokensIn, tokensOut int) (float64, bool) {
	if tokensIn+tokensOut <= 0 {
		return 0, true
	}
	if price, ok := aiModelPriceFor(model); ok {
		if price.Provider == "local" || (price.InputUSDPerMTok == 0 && price.OutputUSDPerMTok == 0) {
			return 0, true
		}
		return float64(tokensIn)/1_000_000*price.InputUSDPerMTok +
			float64(tokensOut)/1_000_000*price.OutputUSDPerMTok, true
	}
	if aiMode(cfg) == "cli" {
		return 0, false
	}
	// Explicit legacy rates win over the local-endpoint free assumption: a
	// self-hosted gateway can still have a metered upstream.
	if cfg.InputUSDPerMTok > 0 || cfg.OutputUSDPerMTok > 0 {
		return float64(tokensIn)/1_000_000*cfg.InputUSDPerMTok +
			float64(tokensOut)/1_000_000*cfg.OutputUSDPerMTok, true
	}
	if isLocalAIEndpoint(cfg.BaseURL) {
		return 0, true
	}
	return 0, false
}
