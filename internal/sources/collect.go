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
	var errs []error
	for _, source := range sources {
		events, err := source.Read(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, events...)
	}
	return all, JoinErrors(errs)
}
