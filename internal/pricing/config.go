package pricing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var rawAPIKeyProviderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{16,}`),
	regexp.MustCompile(`(?i)\b(?:bearer|x-api-key|api[_-]?key|token|pat)[=:]\S+`),
	regexp.MustCompile(`(?i)\b(?:ghp|github_pat|glpat)-?[0-9A-Za-z_]{16,}`),
}

func UserConfigPath(home string) string {
	return filepath.Join(home, userConfigPath)
}

func LoadUserConfig(home string) (Catalog, error) {
	path := UserConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Catalog{}, nil
		}
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func SaveUserPrice(home string, price ModelPrice) (string, error) {
	if err := ValidateUserPrice(price); err != nil {
		return "", err
	}
	catalog, err := LoadUserConfig(home)
	if err != nil {
		return "", err
	}
	price.Source = ""
	catalog.upsert(price)
	path := UserConfigPath(home)
	if err := writeUserConfig(path, catalog); err != nil {
		return "", err
	}
	return path, nil
}

func ValidateUserPrice(price ModelPrice) error {
	if strings.TrimSpace(price.Match) == "" {
		return fmt.Errorf("model match is required")
	}
	if containsRawAPIKey(price.Provider) {
		return fmt.Errorf("provider must be a local provider/auth label, not a raw API key")
	}
	if price.InputUSDPerMTok < 0 || price.OutputUSDPerMTok < 0 || price.CacheHitUSDPerMTok < 0 || price.CacheMakeUSDPerMTok < 0 || price.CacheMake1hUSDPerMTok < 0 {
		return fmt.Errorf("prices must be greater than or equal to 0")
	}
	if price.InputUSDPerMTok == 0 && price.OutputUSDPerMTok == 0 && price.CacheHitUSDPerMTok == 0 && price.CacheMakeUSDPerMTok == 0 && price.CacheMake1hUSDPerMTok == 0 {
		return fmt.Errorf("at least one price must be greater than 0")
	}
	if price.Multiplier < 0 {
		return fmt.Errorf("multiplier must be greater than or equal to 0")
	}
	if price.PromptThresholdTokens < 0 {
		return fmt.Errorf("prompt threshold must be greater than or equal to 0")
	}
	return nil
}

func containsRawAPIKey(value string) bool {
	for _, pattern := range rawAPIKeyProviderPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func writeUserConfig(path string, catalog Catalog) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pricing-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	oldData, readErr := os.ReadFile(path)
	if readErr == nil && bytes.Equal(oldData, data) {
		return os.Remove(tmpPath)
	}
	return os.Rename(tmpPath, path)
}
