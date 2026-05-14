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

func TestSaveUserPriceRejectsRawAPIKeyProvider(t *testing.T) {
	_, err := SaveUserPrice(t.TempDir(), ModelPrice{
		Match:           "gpt-5.4",
		Provider:        "sk-test-secret",
		InputUSDPerMTok: 1,
	})
	if err == nil {
		t.Fatal("expected raw API key provider to be rejected")
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
