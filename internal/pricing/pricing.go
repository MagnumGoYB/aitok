package pricing

import (
	"sort"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

const userConfigPath = ".aitok/pricing.json"

type ModelPrice struct {
	Match                             string  `json:"match"`
	Provider                          string  `json:"provider,omitempty"`
	InputUSDPerMTok                   float64 `json:"input_usd_per_mtok"`
	OutputUSDPerMTok                  float64 `json:"output_usd_per_mtok"`
	CacheHitUSDPerMTok                float64 `json:"cache_hit_usd_per_mtok"`
	CacheMakeUSDPerMTok               float64 `json:"cache_make_usd_per_mtok"`
	CacheMake1hUSDPerMTok             float64 `json:"cache_make_1h_usd_per_mtok,omitempty"`
	PromptThresholdTokens             int64   `json:"prompt_threshold_tokens,omitempty"`
	AboveThresholdInputUSDPerMTok     float64 `json:"above_threshold_input_usd_per_mtok,omitempty"`
	AboveThresholdOutputUSDPerMTok    float64 `json:"above_threshold_output_usd_per_mtok,omitempty"`
	AboveThresholdCacheHitUSDPerMTok  float64 `json:"above_threshold_cache_hit_usd_per_mtok,omitempty"`
	AboveThresholdCacheMakeUSDPerMTok float64 `json:"above_threshold_cache_make_usd_per_mtok,omitempty"`
	Multiplier                        float64 `json:"multiplier,omitempty"`
	Source                            string  `json:"source,omitempty"`
}

type Catalog struct {
	Models []ModelPrice `json:"models"`
}

type Cost struct {
	USD                   float64 `json:"usd"`
	Currency              string  `json:"currency"`
	Multiplier            float64 `json:"multiplier"`
	Source                string  `json:"source"`
	InputUSDPerMTok       float64 `json:"input_usd_per_mtok,omitempty"`
	OutputUSDPerMTok      float64 `json:"output_usd_per_mtok,omitempty"`
	CacheHitUSDPerMTok    float64 `json:"cache_hit_usd_per_mtok,omitempty"`
	CacheMakeUSDPerMTok   float64 `json:"cache_make_usd_per_mtok,omitempty"`
	CacheMake1hUSDPerMTok float64 `json:"cache_make_1h_usd_per_mtok,omitempty"`
}

func Load(home string) (Catalog, error) {
	catalog := DefaultCatalog()
	user, err := LoadUserConfig(home)
	if err != nil {
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
		{Match: "codex-auto-review", Provider: "bcb", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 5, Multiplier: 1, Source: "default"},
		{Match: "gpt-5.5", Provider: "openai", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 5, Multiplier: 1, Source: "default"},
		{Match: "gpt-5.4-mini", Provider: "openai", InputUSDPerMTok: 0.75, OutputUSDPerMTok: 4.5, CacheHitUSDPerMTok: 0.075, CacheMakeUSDPerMTok: 0.75, Multiplier: 1, Source: "default"},
		{Match: "gpt-5.4", Provider: "openai", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.25, CacheMakeUSDPerMTok: 2.5, Multiplier: 1, Source: "default"},
		{Match: "gpt-5", Provider: "openai", InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 0.125, CacheMakeUSDPerMTok: 1.25, Multiplier: 1, Source: "default"},
		{Match: "gpt-4.1", Provider: "openai", InputUSDPerMTok: 2, OutputUSDPerMTok: 8, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 2, Multiplier: 1, Source: "default"},
		{Match: "gpt-4o", Provider: "openai", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 1.25, CacheMakeUSDPerMTok: 2.5, Multiplier: 1, Source: "default"},
		{Match: "claude-opus-4-7", Provider: "anthropic", InputUSDPerMTok: 5, OutputUSDPerMTok: 25, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 6.25, CacheMake1hUSDPerMTok: 10, Multiplier: 1, Source: "default"},
		{Match: "claude-opus-4-6", Provider: "anthropic", InputUSDPerMTok: 5, OutputUSDPerMTok: 25, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 6.25, CacheMake1hUSDPerMTok: 10, Multiplier: 1, Source: "default"},
		{Match: "claude-opus-4-5", Provider: "anthropic", InputUSDPerMTok: 5, OutputUSDPerMTok: 25, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 6.25, CacheMake1hUSDPerMTok: 10, Multiplier: 1, Source: "default"},
		{Match: "claude-opus-4", Provider: "anthropic", InputUSDPerMTok: 15, OutputUSDPerMTok: 75, CacheHitUSDPerMTok: 1.5, CacheMakeUSDPerMTok: 18.75, CacheMake1hUSDPerMTok: 30, Multiplier: 1, Source: "default"},
		{Match: "claude-sonnet-4-6", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, CacheMake1hUSDPerMTok: 6, Multiplier: 1, Source: "default"},
		{Match: "claude-sonnet-4-5", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, CacheMake1hUSDPerMTok: 6, Multiplier: 1, Source: "default"},
		{Match: "claude-sonnet-4", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, CacheMake1hUSDPerMTok: 6, Multiplier: 1, Source: "default"},
		{Match: "claude-haiku-4-5", Provider: "anthropic", InputUSDPerMTok: 1, OutputUSDPerMTok: 5, CacheHitUSDPerMTok: 0.1, CacheMakeUSDPerMTok: 1.25, CacheMake1hUSDPerMTok: 2, Multiplier: 1, Source: "default"},
		{Match: "claude-3-5-haiku", Provider: "anthropic", InputUSDPerMTok: 0.8, OutputUSDPerMTok: 4, CacheHitUSDPerMTok: 0.08, CacheMakeUSDPerMTok: 1, CacheMake1hUSDPerMTok: 1.6, Multiplier: 1, Source: "default"},
		{Match: "claude-3-haiku", Provider: "anthropic", InputUSDPerMTok: 0.25, OutputUSDPerMTok: 1.25, CacheHitUSDPerMTok: 0.03, CacheMakeUSDPerMTok: 0.3, CacheMake1hUSDPerMTok: 0.5, Multiplier: 1, Source: "default"},
		{Match: "claude-3-7-sonnet", Provider: "anthropic", InputUSDPerMTok: 3, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.3, CacheMakeUSDPerMTok: 3.75, CacheMake1hUSDPerMTok: 6, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.5-pro", Provider: "google", InputUSDPerMTok: 1.25, OutputUSDPerMTok: 10, CacheHitUSDPerMTok: 0.125, CacheMakeUSDPerMTok: 1.25, PromptThresholdTokens: 200000, AboveThresholdInputUSDPerMTok: 2.5, AboveThresholdOutputUSDPerMTok: 15, AboveThresholdCacheHitUSDPerMTok: 0.25, AboveThresholdCacheMakeUSDPerMTok: 2.5, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.5-flash", Provider: "google", InputUSDPerMTok: 0.3, OutputUSDPerMTok: 2.5, CacheHitUSDPerMTok: 0.03, CacheMakeUSDPerMTok: 0.3, Multiplier: 1, Source: "default"},
		{Match: "gemini-2.0-flash", Provider: "google", InputUSDPerMTok: 0.1, OutputUSDPerMTok: 0.4, CacheHitUSDPerMTok: 0.025, CacheMakeUSDPerMTok: 0.1, Multiplier: 1, Source: "default"},
	}}
}

func (c Catalog) CostFor(event usage.UsageEvent) Cost {
	price, ok := c.match(event)
	if !ok {
		return Cost{Currency: "USD", Multiplier: 1, Source: "unknown"}
	}
	price = priceForUsage(price, event.Usage)
	multiplier := price.Multiplier
	if multiplier == 0 {
		multiplier = 1
	}
	cacheMake := cacheMakePrice(price)
	cacheMake1h := cacheMake1hPrice(price)
	cacheCreation5m, cacheCreation1h, cacheCreationOther := cacheCreationParts(event.Usage)
	usd := perMillion(billableInput(event), price.InputUSDPerMTok) +
		perMillion(event.Usage.Output, price.OutputUSDPerMTok) +
		perMillion(event.Usage.CachedInput, price.CacheHitUSDPerMTok) +
		perMillion(cacheCreation5m+cacheCreationOther, cacheMake) +
		perMillion(cacheCreation1h, cacheMake1h)
	return Cost{
		USD:                   usd * multiplier,
		Currency:              "USD",
		Multiplier:            multiplier,
		Source:                price.Source,
		InputUSDPerMTok:       price.InputUSDPerMTok,
		OutputUSDPerMTok:      price.OutputUSDPerMTok,
		CacheHitUSDPerMTok:    price.CacheHitUSDPerMTok,
		CacheMakeUSDPerMTok:   cacheMake,
		CacheMake1hUSDPerMTok: cacheMake1h,
	}
}

func (c Catalog) Covers(event usage.UsageEvent) bool {
	_, ok := c.match(event)
	return ok
}

func billableInput(event usage.UsageEvent) int64 {
	input := event.Usage.Input
	cached := event.Usage.CachedInput + event.Usage.CacheCreation
	if input <= cached {
		return 0
	}
	return input - cached
}

func (c Catalog) match(event usage.UsageEvent) (ModelPrice, bool) {
	model := strings.ToLower(event.Model)
	provider := strings.ToLower(event.Provider)
	models := c.sortedModels()
	for _, price := range models {
		if price.Provider == "" || provider == "" || !strings.Contains(provider, strings.ToLower(price.Provider)) {
			continue
		}
		if strings.Contains(model, strings.ToLower(price.Match)) {
			if price.Source == "" {
				price.Source = "configured"
			}
			return price, true
		}
	}
	for _, price := range models {
		if price.Provider != "" {
			continue
		}
		if strings.Contains(model, strings.ToLower(price.Match)) {
			if price.Source == "" {
				price.Source = "configured"
			}
			return price, true
		}
	}
	for _, price := range models {
		if price.Provider == "" || price.Source != "default" {
			continue
		}
		if strings.Contains(model, strings.ToLower(price.Match)) {
			if price.Source == "" {
				price.Source = "configured"
			}
			return price, true
		}
	}
	return ModelPrice{}, false
}

func (c Catalog) sortedModels() []ModelPrice {
	models := append([]ModelPrice(nil), c.Models...)
	sort.SliceStable(models, func(i, j int) bool {
		return len(models[i].Match) > len(models[j].Match)
	})
	return models
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

func priceForUsage(price ModelPrice, tokens usage.TokenUsage) ModelPrice {
	if price.PromptThresholdTokens <= 0 || tokens.Input <= price.PromptThresholdTokens {
		return price
	}
	if price.AboveThresholdInputUSDPerMTok != 0 {
		price.InputUSDPerMTok = price.AboveThresholdInputUSDPerMTok
	}
	if price.AboveThresholdOutputUSDPerMTok != 0 {
		price.OutputUSDPerMTok = price.AboveThresholdOutputUSDPerMTok
	}
	if price.AboveThresholdCacheHitUSDPerMTok != 0 {
		price.CacheHitUSDPerMTok = price.AboveThresholdCacheHitUSDPerMTok
	}
	if price.AboveThresholdCacheMakeUSDPerMTok != 0 {
		price.CacheMakeUSDPerMTok = price.AboveThresholdCacheMakeUSDPerMTok
	}
	return price
}

func cacheMakePrice(price ModelPrice) float64 {
	if price.CacheMakeUSDPerMTok != 0 {
		return price.CacheMakeUSDPerMTok
	}
	return price.InputUSDPerMTok
}

func cacheMake1hPrice(price ModelPrice) float64 {
	if price.CacheMake1hUSDPerMTok != 0 {
		return price.CacheMake1hUSDPerMTok
	}
	return cacheMakePrice(price)
}

func cacheCreationParts(tokens usage.TokenUsage) (fiveMinute int64, oneHour int64, other int64) {
	fiveMinute = tokens.CacheCreation5m
	oneHour = tokens.CacheCreation1h
	known := fiveMinute + oneHour
	if tokens.CacheCreation > known {
		other = tokens.CacheCreation - known
	}
	return fiveMinute, oneHour, other
}
