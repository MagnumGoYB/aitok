package pricing

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func TestCalculateUsesCCSwitchCostFormulaAndMultiplier(t *testing.T) {
	catalog := Catalog{
		Models: []ModelPrice{{
			Match:               "gpt-5.4",
			InputUSDPerMTok:     1,
			OutputUSDPerMTok:    10,
			CacheHitUSDPerMTok:  0.1,
			CacheMakeUSDPerMTok: 1.25,
			Multiplier:          2,
		}},
	}
	cost := catalog.CostFor(usage.UsageEvent{
		Model: "gpt-5.4",
		Usage: usage.TokenUsage{
			Input:         1_000_000,
			Output:        500_000,
			CachedInput:   2_000_000,
			CacheCreation: 1_000_000,
			Reasoning:     100_000,
		},
	})
	if got, want := cost.USD, 12.9; got != want {
		t.Fatalf("cost = %.4f, want %.4f", got, want)
	}
	if cost.Currency != "USD" || cost.Multiplier != 2 || cost.Source != "configured" {
		t.Fatalf("unexpected cost metadata: %+v", cost)
	}
}

func TestCostForCodexDoesNotChargeCachedInputAtFullInputRate(t *testing.T) {
	catalog := Catalog{
		Models: []ModelPrice{{
			Match:              "gpt-5.5",
			InputUSDPerMTok:    5,
			OutputUSDPerMTok:   30,
			CacheHitUSDPerMTok: 0.5,
		}},
	}
	cost := catalog.CostFor(usage.UsageEvent{
		Tool:  usage.ToolCodex,
		Model: "gpt-5.5",
		Usage: usage.TokenUsage{
			Input:       10_000_000,
			CachedInput: 8_000_000,
		},
	})
	if got, want := cost.USD, 14.0; got != want {
		t.Fatalf("cost = %.4f, want %.4f", got, want)
	}
}

func TestCostForClaudeDoesNotChargeCachedInputAtFullInputRate(t *testing.T) {
	catalog := Catalog{
		Models: []ModelPrice{{
			Match:                 "claude-sonnet-4",
			InputUSDPerMTok:       3,
			OutputUSDPerMTok:      15,
			CacheHitUSDPerMTok:    0.3,
			CacheMakeUSDPerMTok:   3.75,
			CacheMake1hUSDPerMTok: 6,
		}},
	}
	cost := catalog.CostFor(usage.UsageEvent{
		Tool:  usage.ToolClaude,
		Model: "claude-sonnet-4",
		Usage: usage.TokenUsage{
			Input:           10_000_000,
			CachedInput:     8_000_000,
			CacheCreation:   3_000_000,
			CacheCreation5m: 2_000_000,
			CacheCreation1h: 1_000_000,
		},
	})
	if got, want := cost.USD, 15.9; got != want {
		t.Fatalf("cost = %.4f, want %.4f", got, want)
	}
}

func TestDefaultCatalogPricesClaudeOpus47BeforeOpus4(t *testing.T) {
	cost := DefaultCatalog().CostFor(usage.UsageEvent{
		Tool:  usage.ToolClaude,
		Model: "claude-opus-4-7",
		Usage: usage.TokenUsage{
			Input:       1_280_283,
			Output:      219_935,
			CachedInput: 66_152_448,
		},
	})
	if got, want := cost.USD, 38.574599; math.Abs(got-want) > 0.000001 {
		t.Fatalf("cost = %.6f, want %.6f", got, want)
	}
}

func TestCostForGemini25ProUsesAboveThresholdPricing(t *testing.T) {
	cost := DefaultCatalog().CostFor(usage.UsageEvent{
		Tool:     usage.ToolGemini,
		Model:    "gemini-2.5-pro",
		Provider: "google",
		Usage: usage.TokenUsage{
			Input:       250_000,
			Output:      100_000,
			CachedInput: 50_000,
		},
	})
	if got, want := cost.USD, 2.0125; math.Abs(got-want) > 0.000001 {
		t.Fatalf("cost = %.4f, want %.4f", got, want)
	}
}

func TestLoadCatalogMergesUserConfigOverDefaults(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".aitok", "pricing.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
  "models": [
    {
      "match": "claude-sonnet-4",
      "input_usd_per_mtok": 4,
      "output_usd_per_mtok": 20,
      "cache_hit_usd_per_mtok": 0.4,
      "cache_make_usd_per_mtok": 5,
      "multiplier": 1.5
    }
  ]
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	cost := catalog.CostFor(usage.UsageEvent{
		Model: "claude-sonnet-4-20250514",
		Usage: usage.TokenUsage{Input: 1_000_000},
	})
	if cost.USD != 6 {
		t.Fatalf("overridden cost = %.4f, want 6", cost.USD)
	}
	if cost.Source != "user" {
		t.Fatalf("source = %q, want user", cost.Source)
	}
}

func TestSaveUserPriceUpsertsProviderSpecificOverride(t *testing.T) {
	home := t.TempDir()
	if _, err := SaveUserPrice(home, ModelPrice{
		Match:            "gpt-5.4",
		Provider:         "team-a",
		InputUSDPerMTok:  2,
		OutputUSDPerMTok: 20,
		Multiplier:       1,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUserPrice(home, ModelPrice{
		Match:            "gpt-5.4",
		Provider:         "team-b",
		InputUSDPerMTok:  8,
		OutputUSDPerMTok: 80,
		Multiplier:       1,
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	teamA := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.4",
		Provider: "team-a",
		Usage:    usage.TokenUsage{Input: 1_000_000},
	})
	teamB := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.4",
		Provider: "team-b",
		Usage:    usage.TokenUsage{Input: 1_000_000},
	})
	if teamA.USD != 2 || teamB.USD != 8 {
		t.Fatalf("provider-specific overrides not applied: team-a=%+v team-b=%+v", teamA, teamB)
	}
	data, err := os.ReadFile(UserConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(UserConfigPath(home)); err != nil {
		t.Fatal(err)
	} else if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("pricing config mode = %o, want 600", mode)
	}
	if got := string(data); !containsAll(got, `"provider": "team-a"`, `"provider": "team-b"`) {
		t.Fatalf("pricing config missing provider overrides: %s", got)
	}
}

func TestProviderSpecificUserPriceDoesNotFallbackToOtherProviders(t *testing.T) {
	home := t.TempDir()
	if _, err := SaveUserPrice(home, ModelPrice{
		Match:              "gpt-5.5",
		Provider:           "toska",
		InputUSDPerMTok:    5,
		OutputUSDPerMTok:   40,
		CacheHitUSDPerMTok: 0.5,
		Multiplier:         1,
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	toska := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.5",
		Provider: "toska",
		Usage:    usage.TokenUsage{Output: 1_000_000},
	})
	bcb := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.5",
		Provider: "bcb",
		Usage:    usage.TokenUsage{Output: 1_000_000},
	})
	unknown := catalog.CostFor(usage.UsageEvent{
		Model: "gpt-5.5",
		Usage: usage.TokenUsage{Output: 1_000_000},
	})
	if toska.Source != "user" || toska.OutputUSDPerMTok != 40 || toska.USD != 40 {
		t.Fatalf("toska should use provider-specific custom price: %+v", toska)
	}
	if bcb.Source != "default" || bcb.OutputUSDPerMTok != 30 || bcb.USD != 30 {
		t.Fatalf("bcb should fall back to official default price, not toska custom price: %+v", bcb)
	}
	if unknown.Source != "default" || unknown.OutputUSDPerMTok != 30 || unknown.USD != 30 {
		t.Fatalf("unknown provider should fall back to official default price, not provider-specific custom price: %+v", unknown)
	}
}

func TestProviderSpecificPriceTakesPriorityOverGlobalOverride(t *testing.T) {
	catalog := Catalog{Models: []ModelPrice{
		{Match: "gpt-5.5", InputUSDPerMTok: 1, OutputUSDPerMTok: 10, Source: "user"},
		{Match: "gpt-5.5", Provider: "toska", InputUSDPerMTok: 5, OutputUSDPerMTok: 40, Source: "user"},
		{Match: "gpt-5.5", Provider: "openai", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, Source: "default"},
	}}
	toska := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.5",
		Provider: "toska",
		Usage:    usage.TokenUsage{Output: 1_000_000},
	})
	bcb := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.5",
		Provider: "bcb",
		Usage:    usage.TokenUsage{Output: 1_000_000},
	})
	openai := catalog.CostFor(usage.UsageEvent{
		Model:    "gpt-5.5",
		Provider: "openai",
		Usage:    usage.TokenUsage{Output: 1_000_000},
	})
	if toska.OutputUSDPerMTok != 40 {
		t.Fatalf("provider-specific custom price should win for toska: %+v", toska)
	}
	if bcb.OutputUSDPerMTok != 10 {
		t.Fatalf("global custom price should win for providers without provider-specific custom price: %+v", bcb)
	}
	if openai.OutputUSDPerMTok != 30 {
		t.Fatalf("provider-specific default price should win for matching official provider: %+v", openai)
	}
}

func TestSaveUserPriceRejectsRawAPIKeyProvider(t *testing.T) {
	tests := []string{
		"sk-test-secret",
		"AIzaSyD8Qw9eRtYuIoPaSdFgHjKlZxCvBnMq12",
		"Bearer=secret-token-value",
		"github_pat_1234567890abcdef1234567890abcdef",
	}
	for _, provider := range tests {
		t.Run(provider[:min(len(provider), 12)], func(t *testing.T) {
			home := t.TempDir()
			_, err := SaveUserPrice(home, ModelPrice{
				Match:           "gpt-5.4",
				Provider:        provider,
				InputUSDPerMTok: 1,
			})
			if err == nil {
				t.Fatal("expected raw API key provider to be rejected")
			}
			if _, statErr := os.Stat(UserConfigPath(home)); !os.IsNotExist(statErr) {
				t.Fatalf("raw key rejection must not write pricing config, stat err=%v", statErr)
			}
		})
	}
}

func TestLoadIgnoresMissingUserConfig(t *testing.T) {
	catalog, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cost := catalog.CostFor(usage.UsageEvent{Model: "gpt-5.4", Usage: usage.TokenUsage{Input: 1_000_000}})
	if cost.USD <= 0 {
		t.Fatalf("default catalog did not price known model: %+v", cost)
	}
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func TestDefaultCatalogCoversCodexAutoReview(t *testing.T) {
	cost := DefaultCatalog().CostFor(usage.UsageEvent{
		Tool:     usage.ToolCodex,
		Model:    "codex-auto-review",
		Provider: "bcb",
		Usage:    usage.TokenUsage{Input: 1_000_000, Output: 100_000, CachedInput: 800_000},
	})
	if cost.USD <= 0 {
		t.Fatalf("codex-auto-review should be priced by default: %+v", cost)
	}
	if cost.Source != "default" {
		t.Fatalf("source = %q, want default", cost.Source)
	}
}

func TestDefaultCatalogCoversDeepSeekModels(t *testing.T) {
	catalog := DefaultCatalog()
	for _, model := range []string{"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-chat", "deepseek-reasoner"} {
		cost := catalog.CostFor(usage.UsageEvent{
			Tool:     usage.ToolReasonix,
			Model:    model,
			Provider: "deepseek",
			Usage:    usage.TokenUsage{Input: 1_000_000, Output: 500_000},
		})
		if cost.USD == 0 {
			t.Fatalf("CostFor(%q) returned $0.00", model)
		}
		if cost.Currency != "CNY" {
			t.Fatalf("CostFor(%q).Currency = %q, want CNY", model, cost.Currency)
		}
		if cost.Source == "unknown" {
			t.Fatalf("CostFor(%q).Source = unknown (unpriced)", model)
		}
		if model == "deepseek-chat" || model == "deepseek-v4-flash" {
			expectedCNY := 1.0 + 1.0 // 1M in x ¥1/M + 0.5M out x ¥2/M
			if math.Abs(cost.Amount-expectedCNY) > 0.01 {
				t.Fatalf("CostFor(%q).Amount = %.4f, want ~%.4f CNY", model, cost.Amount, expectedCNY)
			}
			if math.Abs(cost.USD-expectedCNY) > 0.001 {
				t.Fatalf("CostFor(%q).USD = %.4f, want native %.4f CNY", model, cost.USD, expectedCNY)
			}
		}
		if model == "deepseek-v4-pro" {
			expectedCNY := 3.0 + 3.0 // 1M in x ¥3/M + 0.5M out x ¥6/M
			if math.Abs(cost.Amount-expectedCNY) > 0.01 {
				t.Fatalf("CostFor(%q).Amount = %.4f, want ~%.4f CNY", model, cost.Amount, expectedCNY)
			}
			if math.Abs(cost.USD-expectedCNY) > 0.001 {
				t.Fatalf("CostFor(%q).USD = %.4f, want native %.4f CNY", model, cost.USD, expectedCNY)
			}
		}
	}
}

func TestDefaultCatalogCoversMiMoModels(t *testing.T) {
	catalog := DefaultCatalog()
	tests := []struct {
		model    string
		provider string
		input    int64
		output   int64
		wantUSD  float64
	}{
		{"mimo-v2.5-pro", "mimo", 1_000_000, 1_000_000, 0.435 + 0.87},
		{"mimo-v2.5", "mimo", 1_000_000, 1_000_000, 0.14 + 0.28},
		{"mimo-v2-pro", "mimo", 100_000, 100_000, 0.1 + 0.3},         // below threshold
		{"mimo-v2-pro", "mimo", 300_000, 100_000, 0.6 + 0.6},         // above threshold
		{"mimo-v2-omni", "mimo", 1_000_000, 1_000_000, 0.4 + 2.0},
		{"off-v2-flash", "mimo", 1_000_000, 1_000_000, 0.1 + 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cost := catalog.CostFor(usage.UsageEvent{
				Model:    tt.model,
				Provider: tt.provider,
				Usage:    usage.TokenUsage{Input: tt.input, Output: tt.output},
			})
			if cost.Source == "unknown" {
				t.Fatalf("CostFor(%q).Source = unknown (unpriced)", tt.model)
			}
			if math.Abs(cost.USD-tt.wantUSD) > 0.001 {
				t.Fatalf("CostFor(%q).USD = %.4f, want %.4f", tt.model, cost.USD, tt.wantUSD)
			}
		})
	}
}

func BenchmarkCostFor(b *testing.B) {
	catalog := DefaultCatalog()
	events := make([]usage.UsageEvent, 4096)
	models := []string{"gpt-5.5", "gpt-5.4", "claude-sonnet-4", "claude-opus-4-7", "gemini-2.5-flash", "gemini-2.5-pro", "codex-auto-review"}
	providers := []string{"openai", "anthropic", "google", "bcb"}
	for i := range events {
		events[i] = usage.UsageEvent{
			Tool:     usage.ToolCodex,
			Model:    models[i%len(models)],
			Provider: providers[i%len(providers)],
			Usage: usage.TokenUsage{
				Input:           int64(100_000 + i%10_000),
				Output:          int64(10_000 + i%1_000),
				CachedInput:     int64(50_000 + i%5_000),
				CacheCreation:   int64(5_000 + i%500),
				CacheCreation5m: int64(2_000 + i%200),
				CacheCreation1h: int64(1_000 + i%100),
			},
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, event := range events {
			_ = catalog.CostFor(event)
		}
	}
}
