package pricing

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

const userConfigPath = ".aitok/pricing.json"

type ModelPrice struct {
	Match               string  `json:"match"`
	Provider            string  `json:"provider,omitempty"`
	InputUSDPerMTok     float64 `json:"input_usd_per_mtok"`
	OutputUSDPerMTok    float64 `json:"output_usd_per_mtok"`
	CacheHitUSDPerMTok  float64 `json:"cache_hit_usd_per_mtok"`
	CacheMakeUSDPerMTok float64 `json:"cache_make_usd_per_mtok"`
	Multiplier          float64 `json:"multiplier,omitempty"`
	Source              string  `json:"source,omitempty"`
}

type Catalog struct {
	Models []ModelPrice `json:"models"`
}

type Cost struct {
	USD        float64 `json:"usd"`
	Currency   string  `json:"currency"`
	Multiplier float64 `json:"multiplier"`
	Source     string  `json:"source"`
}

func Load(home string) (Catalog, error) {
	catalog := DefaultCatalog()
	path := filepath.Join(home, userConfigPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return catalog, nil
		}
		return Catalog{}, err
	}
	var user Catalog
	if err := json.Unmarshal(data, &user); err != nil {
		return Catalog{}, err
	}
	for _, price := range user.Models {
		price.Source = "user"
		catalog.upsert(price)
	}
	return catalog, nil
}

func DefaultCatalog() Catalog {
	return Catalog{Models: []ModelPrice{
		{Match: "gpt-5.5", Provider: "openai", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 5, Multiplier: 1, Source: "default"},
		{Match: "gpt-5.4-mini", Provider: "openai", InputUSDPerMTok: 0.75, OutputUSDPerMTok: 4.5, CacheHitUSDPerMTok: 0.075, CacheMakeUSDPerMTok: 0.75, Multiplier: 1, Source: "default"},
		{Match: "gpt-5.4", Provider: "openai", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.25, CacheMakeUSDPerMTok: 2.5, Multiplier: 1, Source: "default"},
		{Match: "gpt-5", Provider: "openai", InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 0.125, CacheMakeUSDPerMTok: 1.25, Multiplier: 1, Source: "default"},
		{Match: "gpt-4.1", Provider: "openai", InputUSDPerMTok: 2, OutputUSDPerMTok: 8, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 2, Multiplier: 1, Source: "default"},
		{Match: "gpt-4o", Provider: "openai", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 1.25, CacheMakeUSDPerMTok: 2.5, Multiplier: 1, Source: "default"},
		{Match: "claude-opus-4", Provider: "anthropic", InputUSDPerMTok: 15, OutputUSDPerMTok: 75, CacheHitUSDPerMTok: 1.5, CacheMakeUSDPerMTok: 18.75, Multiplier: 1, Source: "default"},
		{Match: "claude-sonnet-4", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, Multiplier: 1, Source: "default"},
		{Match: "claude-3-7-sonnet", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.5-pro", Provider: "google", InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 0.125, CacheMakeUSDPerMTok: 1.25, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.5-flash", Provider: "google", InputUSDPerMTok: 0.3, OutputUSDPerMTok: 2.5, CacheHitUSDPerMTok: 0.03, CacheMakeUSDPerMTok: 0.3, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.0-flash", Provider: "google", InputUSDPerMTok: 0.1, OutputUSDPerMTok: 0.4, CacheHitUSDPerMTok: 0.025, CacheMakeUSDPerMTok: 0.1, Multiplier: 1, Source: "default"},
	}}
}

func (c Catalog) CostFor(event usage.UsageEvent) Cost {
	price, ok := c.match(event)
	if !ok {
		return Cost{Currency: "USD", Multiplier: 1, Source: "unknown"}
	}
	multiplier := price.Multiplier
	if multiplier == 0 {
		multiplier = 1
	}
	cacheMake := price.CacheMakeUSDPerMTok
	if cacheMake == 0 {
		cacheMake = price.InputUSDPerMTok
	}
	usd := perMillion(event.Usage.Input, price.InputUSDPerMTok) +
		perMillion(event.Usage.Output+event.Usage.Reasoning, price.OutputUSDPerMTok) +
		perMillion(event.Usage.CachedInput, price.CacheHitUSDPerMTok) +
		perMillion(event.Usage.CacheCreation, cacheMake)
	return Cost{USD: usd * multiplier, Currency: "USD", Multiplier: multiplier, Source: price.Source}
}

func (c Catalog) match(event usage.UsageEvent) (ModelPrice, bool) {
	model := strings.ToLower(event.Model)
	provider := strings.ToLower(event.Provider)
	for _, price := range c.Models {
		if price.Provider != "" && provider != "" && !strings.Contains(provider, strings.ToLower(price.Provider)) {
			continue
		}
		if strings.Contains(model, strings.ToLower(price.Match)) {
			if price.Source == "" {
				price.Source = "configured"
			}
			return price, true
		}
	}
	for _, price := range c.Models {
		if strings.Contains(model, strings.ToLower(price.Match)) {
			if price.Source == "" {
				price.Source = "configured"
			}
			return price, true
		}
	}
	return ModelPrice{}, false
}

func (c *Catalog) upsert(price ModelPrice) {
	for i, existing := range c.Models {
		if strings.EqualFold(existing.Match, price.Match) && strings.EqualFold(existing.Provider, price.Provider) {
			c.Models[i] = price
			return
		}
	}
	c.Models = append([]ModelPrice{price}, c.Models...)
}

func perMillion(tokens int64, price float64) float64 {
	return float64(tokens) / 1_000_000 * price
}
