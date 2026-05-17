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
	Signature string                `json:"signature"`
	Generated time.Time             `json:"generated_at"`
	Timeline  codexProviderTimeline `json:"timeline"`
}

func readProviderTimelineCache(signature string) (codexProviderTimeline, bool) {
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
	if cache.Signature != signature {
		return nil, false
	}
	return cache.Timeline, true
}

func writeProviderTimelineCache(signature string, timeline codexProviderTimeline) {
	if signature == "" {
		return
	}
	cache := providerTimelineCache{
		Signature: signature,
		Generated: time.Now().UTC(),
		Timeline:  timeline,
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
	sorted := append([]string(nil), paths...)
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
