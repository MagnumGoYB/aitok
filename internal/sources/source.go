package sources

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

type Source interface {
	Name() usage.Tool
	Read(ctx context.Context) ([]usage.UsageEvent, error)
	Scan(ctx context.Context, handle func(usage.UsageEvent) error) error
}

type Options struct {
	Home string
}

func DefaultOptions() Options {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return Options{Home: home}
}

func cleanHome(home string) string {
	if home == "" {
		return DefaultOptions().Home
	}
	return home
}

func expandHome(home, path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) > 2 && path[0] == '~' && os.IsPathSeparator(path[1]) {
		return filepath.Join(home, path[2:])
	}
	return path
}

func JoinErrors(errs []error) error {
	var out error
	for _, err := range errs {
		if err != nil {
			out = errors.Join(out, err)
		}
	}
	return out
}
