package sources

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderTimelineCacheRoundTripAndSignatureInvalidation(t *testing.T) {
	root := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(origWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	inputPath := filepath.Join(root, "input.txt")
	mustWrite(t, inputPath, "alpha\n")
	sig1 := providerTimelineCacheSignature([]string{inputPath})
	timeline := codexProviderTimeline{
		"thread-a": {
			{At: time.Date(2026, 5, 8, 1, 0, 0, 0, time.UTC), TurnID: "turn-a", Provider: "openai", Strength: codexProviderStrengthStrong},
		},
	}
	windowStart := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	writeProviderTimelineCache(sig1, windowStart, windowEnd, timeline)

	got, ok := readProviderTimelineCache(sig1, windowStart, windowEnd)
	if !ok {
		t.Fatal("expected cache hit after round trip")
	}
	if len(got["thread-a"]) != 1 || got["thread-a"][0].Provider != "openai" {
		t.Fatalf("unexpected cached timeline: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".cache", "aitok", "provider-timeline.json")); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, inputPath, "alpha\nbeta\n")
	sig2 := providerTimelineCacheSignature([]string{inputPath})
	if sig1 == sig2 {
		t.Fatal("expected signature to change when input file changes")
	}
	if _, ok := readProviderTimelineCache(sig2, windowStart, windowEnd); ok {
		t.Fatal("expected cache miss for changed signature")
	}
	if _, ok := readProviderTimelineCache(sig1, windowStart.Add(-time.Hour), windowEnd); ok {
		t.Fatal("expected cache miss for changed window")
	}
}
