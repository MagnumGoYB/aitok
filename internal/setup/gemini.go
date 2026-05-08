package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type GeminiResult struct {
	Path    string `json:"path"`
	Outfile string `json:"outfile"`
	DryRun  bool   `json:"dry_run"`
	Changed bool   `json:"changed"`
	Content string `json:"content"`
}

func ConfigureGemini(home string, dryRun bool) (GeminiResult, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return GeminiResult{}, err
		}
	}
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	outfile := filepath.Join(home, ".gemini", "telemetry.log")
	settings := map[string]any{}
	if data, err := os.ReadFile(settingsPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return GeminiResult{}, fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return GeminiResult{}, err
	}

	telemetry, _ := settings["telemetry"].(map[string]any)
	if telemetry == nil {
		telemetry = map[string]any{}
		settings["telemetry"] = telemetry
	}
	before, _ := json.Marshal(settings)
	telemetry["enabled"] = true
	telemetry["target"] = "local"
	telemetry["outfile"] = outfile
	telemetry["logPrompts"] = false
	after, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return GeminiResult{}, err
	}
	changed := string(before) != string(mustCompact(after))
	result := GeminiResult{
		Path:    settingsPath,
		Outfile: outfile,
		DryRun:  dryRun,
		Changed: changed,
		Content: string(after) + "\n",
	}
	if dryRun {
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return GeminiResult{}, err
	}
	if err := os.WriteFile(settingsPath, []byte(result.Content), 0o600); err != nil {
		return GeminiResult{}, err
	}
	return result, nil
}

func mustCompact(data []byte) []byte {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data
	}
	out, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return out
}
