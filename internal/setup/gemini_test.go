package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureGeminiDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	result, err := ConfigureGemini(home, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || !strings.Contains(result.Content, `"logPrompts": false`) {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("settings should not exist after dry run, err=%v", err)
	}
}

func TestConfigureGeminiWritesSettings(t *testing.T) {
	home := t.TempDir()
	result, err := ConfigureGemini(home, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"enabled": true`) || !strings.Contains(string(data), `"logPrompts": false`) {
		t.Fatalf("settings missing telemetry config: %s", string(data))
	}
}
