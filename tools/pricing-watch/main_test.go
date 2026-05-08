package main

import (
	"strings"
	"testing"
)

func TestCheckSourcesDetectsChangedOfficialPricingPage(t *testing.T) {
	manifest := PricingManifest{Sources: []PricingSource{{
		Name:        "openai",
		URL:         "https://openai.com/api/pricing/",
		SHA256:      "old",
		ReviewHint:  "Review OpenAI model pricing.",
		ModelsOwned: []string{"gpt-5.5"},
	}}}
	report, err := CheckSources(manifest, func(url string) ([]byte, error) {
		if url != "https://openai.com/api/pricing/" {
			t.Fatalf("unexpected URL %s", url)
		}
		return []byte("new official pricing page"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Changed() {
		t.Fatal("expected changed report")
	}
	if len(report.Changes) != 1 || report.Changes[0].Name != "openai" {
		t.Fatalf("unexpected changes: %+v", report.Changes)
	}
	if !strings.Contains(report.Markdown(), "OpenAI") || !strings.Contains(report.Markdown(), "gpt-5.5") {
		t.Fatalf("markdown missing source details: %s", report.Markdown())
	}
}

func TestCheckSourcesPassesWhenHashesMatch(t *testing.T) {
	hash := HashBytes([]byte("same"))
	report, err := CheckSources(PricingManifest{Sources: []PricingSource{{Name: "gemini", URL: "https://ai.google.dev/gemini-api/docs/pricing", SHA256: hash}}}, func(string) ([]byte, error) {
		return []byte("same"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed() {
		t.Fatalf("expected no changes: %+v", report)
	}
}

func TestCheckSourcesDetectsMissingRequiredPricingText(t *testing.T) {
	report, err := CheckSources(PricingManifest{Sources: []PricingSource{{
		Name:     "gemini",
		URL:      "https://ai.google.dev/gemini-api/docs/pricing",
		Required: []string{"Gemini 2.5 Pro", "Context caching"},
	}}}, func(string) ([]byte, error) {
		return []byte("Gemini 2.5 Pro only"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Changed() || !strings.Contains(report.Markdown(), "Context caching") {
		t.Fatalf("expected missing required text report: %s", report.Markdown())
	}
}

func TestCheckSourcesSkipsManualSources(t *testing.T) {
	report, err := CheckSources(PricingManifest{Sources: []PricingSource{{Name: "openai", URL: "https://openai.com/api/pricing/", Manual: true}}}, func(string) ([]byte, error) {
		t.Fatal("manual source should not be fetched")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed() {
		t.Fatalf("manual source should not report changes: %+v", report)
	}
}
