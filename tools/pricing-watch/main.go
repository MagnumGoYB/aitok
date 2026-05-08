package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type PricingManifest struct {
	Sources []PricingSource `json:"sources"`
}

type PricingSource struct {
	Name        string   `json:"name"`
	URL         string   `json:"url"`
	Manual      bool     `json:"manual_review,omitempty"`
	SHA256      string   `json:"sha256"`
	Required    []string `json:"required_text,omitempty"`
	ReviewHint  string   `json:"review_hint"`
	ModelsOwned []string `json:"models_owned"`
}

type Report struct {
	CheckedAt time.Time
	Changes   []Change
}

type Change struct {
	Name        string
	URL         string
	Expected    string
	Actual      string
	Reason      string
	ReviewHint  string
	ModelsOwned []string
}

type fetcher func(string) ([]byte, error)

func main() {
	manifestPath := flag.String("manifest", "docs/pricing-sources.json", "pricing source manifest")
	outPath := flag.String("out", "", "write markdown report to path")
	flag.Parse()

	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report, err := CheckSources(manifest, fetchURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	markdown := report.Markdown()
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(markdown), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		fmt.Print(markdown)
	}
	if report.Changed() {
		os.Exit(1)
	}
}

func loadManifest(path string) (PricingManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PricingManifest{}, err
	}
	var manifest PricingManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PricingManifest{}, err
	}
	return manifest, nil
}

func CheckSources(manifest PricingManifest, fetch fetcher) (Report, error) {
	report := Report{CheckedAt: time.Now().UTC()}
	for _, source := range manifest.Sources {
		if source.Manual {
			continue
		}
		body, err := fetch(source.URL)
		if err != nil {
			return report, fmt.Errorf("fetch %s: %w", source.URL, err)
		}
		normalized := normalize(body)
		if missing := missingRequiredText(normalized, source.Required); len(missing) > 0 {
			report.Changes = append(report.Changes, Change{
				Name:        source.Name,
				URL:         source.URL,
				Reason:      "required text missing: " + strings.Join(missing, ", "),
				ReviewHint:  source.ReviewHint,
				ModelsOwned: source.ModelsOwned,
			})
			continue
		}
		if source.SHA256 != "" {
			actual := HashBytes(normalized)
			if !strings.EqualFold(actual, source.SHA256) {
				report.Changes = append(report.Changes, Change{
					Name:        source.Name,
					URL:         source.URL,
					Expected:    source.SHA256,
					Actual:      actual,
					Reason:      "source hash changed",
					ReviewHint:  source.ReviewHint,
					ModelsOwned: source.ModelsOwned,
				})
			}
		}
	}
	return report, nil
}

func (r Report) Changed() bool {
	return len(r.Changes) > 0
}

func (r Report) Markdown() string {
	var b strings.Builder
	if !r.Changed() {
		b.WriteString("No official pricing source changes detected.\n")
		return b.String()
	}
	b.WriteString("## Official pricing source changes detected\n\n")
	b.WriteString("The default `internal/pricing` catalog may need review. This check does not update prices automatically.\n\n")
	for _, change := range r.Changes {
		b.WriteString(fmt.Sprintf("### %s\n\n", displayName(change.Name)))
		b.WriteString(fmt.Sprintf("- Source: %s\n", change.URL))
		if change.Reason != "" {
			b.WriteString(fmt.Sprintf("- Reason: %s\n", change.Reason))
		}
		if change.Expected != "" || change.Actual != "" {
			b.WriteString(fmt.Sprintf("- Previous SHA256: `%s`\n", change.Expected))
			b.WriteString(fmt.Sprintf("- Current SHA256: `%s`\n", change.Actual))
		}
		if len(change.ModelsOwned) > 0 {
			b.WriteString(fmt.Sprintf("- Catalog models to review: `%s`\n", strings.Join(change.ModelsOwned, "`, `")))
		}
		if change.ReviewHint != "" {
			b.WriteString(fmt.Sprintf("- Review hint: %s\n", change.ReviewHint))
		}
		b.WriteString("\n")
	}
	b.WriteString("After verifying official pricing, update `internal/pricing/pricing.go`, tests, README notes if needed, and `docs/pricing-sources.json` with the new SHA256 values.\n")
	return b.String()
}

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; aitok-pricing-watch/1.0; +https://github.com/MagnumGoYB/aitok)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fetchWithCurl(url)
	}
	return io.ReadAll(resp.Body)
}

func fetchWithCurl(url string) ([]byte, error) {
	cmd := exec.Command("curl", "-L", "-sS", "--fail", "-A", "Mozilla/5.0 (compatible; aitok-pricing-watch/1.0; +https://github.com/MagnumGoYB/aitok)", url)
	body, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("fallback curl fetch failed: %w", err)
	}
	return body, nil
}

func normalize(data []byte) []byte {
	return []byte(strings.Join(strings.Fields(string(data)), " "))
}

func missingRequiredText(data []byte, required []string) []string {
	text := strings.ToLower(string(data))
	var missing []string
	for _, item := range required {
		if !strings.Contains(text, strings.ToLower(item)) {
			missing = append(missing, item)
		}
	}
	return missing
}

func displayName(name string) string {
	switch strings.ToLower(name) {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "gemini":
		return "Gemini"
	default:
		return name
	}
}
