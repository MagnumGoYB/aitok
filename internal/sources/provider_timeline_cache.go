package sources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type providerTimelineCache struct {
	Signature   string                `json:"signature"`
	WindowStart time.Time             `json:"window_start"`
	WindowEnd   time.Time             `json:"window_end"`
	Generated   time.Time             `json:"generated_at"`
	Timeline    codexProviderTimeline `json:"timeline"`
}

func readProviderTimelineCache(signature string, windowStart, windowEnd time.Time) (codexProviderTimeline, bool) {
	if signature == "" {
		return nil, false
	}
	data, err := os.ReadFile(providerTimelineCachePath())
	if err != nil {
		return nil, false
	}
	var cache providerTimelineCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}
	if cache.Signature != signature || !cache.WindowStart.Equal(windowStart) || !cache.WindowEnd.Equal(windowEnd) {
		return nil, false
	}
	return cache.Timeline, true
}

func writeProviderTimelineCache(signature string, windowStart, windowEnd time.Time, timeline codexProviderTimeline) {
	if signature == "" {
		return
	}
	cache := providerTimelineCache{
		Signature:   signature,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Generated:   time.Now().UTC(),
		Timeline:    timeline,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	path := providerTimelineCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		return
	}
}

func providerTimelineCachePath() string {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	return filepath.Join(cwd, ".cache", "aitok", "provider-timeline.json")
}

func providerTimelineCacheSignature(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	sorted := providerTimelineCacheSignaturePaths(paths)
	sort.Strings(sorted)
	h := sha256.New()
	for _, path := range sorted {
		_, _ = h.Write([]byte(path))
		_, _ = h.Write([]byte{0})
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		_, _ = h.Write([]byte(info.ModTime().UTC().Format(time.RFC3339Nano)))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(strings.TrimSpace(filepath.Base(path))))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(time.Duration(info.Size()).String()))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func providerTimelineCacheSignaturePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		for _, candidate := range providerTimelineCacheInputPaths(path) {
			if candidate == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			out = append(out, candidate)
		}
	}
	return out
}

func providerTimelineCacheInputPaths(path string) []string {
	if filepath.Base(path) != "logs_2.sqlite" {
		return []string{path}
	}
	return []string{path, path + "-wal", path + "-shm"}
}
