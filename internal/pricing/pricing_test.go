package pricing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func TestCalculateUsesDefaultPricesAndMultiplier(t *testing.T) {
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
	if got, want := cost.USD, 16.9; got != want {
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

func TestCostForClaudeKeepsInputAndCacheSeparate(t *testing.T) {
	catalog := Catalog{
		Models: []ModelPrice{{
			Match:              "claude-sonnet-4",
			InputUSDPerMTok:    3,
			OutputUSDPerMTok:   15,
			CacheHitUSDPerMTok: 0.3,
		}},
	}
	cost := catalog.CostFor(usage.UsageEvent{
		Tool:  usage.ToolClaude,
		Model: "claude-sonnet-4",
		Usage: usage.TokenUsage{
			Input:       10_000_000,
			CachedInput: 8_000_000,
		},
	})
	if got, want := cost.USD, 32.4; got != want {
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
