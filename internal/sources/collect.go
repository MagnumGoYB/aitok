package sources

import (
	"context"

	"github.com/MagnumGoYB/aitok/internal/usage"
)

func Defaults(opts Options) []Source {
	return []Source{
		NewClaude(opts),
		NewCodex(opts),
		NewGemini(opts),
	}
}

func Collect(ctx context.Context, sources []Source) ([]usage.UsageEvent, error) {
	var all []usage.UsageEvent
	err := ForEach(ctx, sources, func(event usage.UsageEvent) error {
		all = append(all, event)
		return nil
	})
	return all, err
}

func ForEach(ctx context.Context, sources []Source, handle func(usage.UsageEvent) error) error {
	var errs []error
	for _, source := range sources {
		if err := source.Scan(ctx, handle); err != nil {
			errs = append(errs, err)
		}
	}
	return JoinErrors(errs)
}
