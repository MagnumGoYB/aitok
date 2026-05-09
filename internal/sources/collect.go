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

// Collect aggregates usage events emitted by the provided sources.
// It iterates each source, appending every observed usage event into a single slice.
// Events are ordered by the sequence in which sources are iterated and by the order emitted per source.
// It returns the collected events and any aggregated error produced while scanning the sources.
func Collect(ctx context.Context, sources []Source) ([]usage.UsageEvent, error) {
	var all []usage.UsageEvent
	err := ForEach(ctx, sources, func(event usage.UsageEvent) error {
		all = append(all, event)
		return nil
	})
	return all, err
}

// ForEach calls Scan on each Source in sources, invoking handle for each emitted usage.UsageEvent.
// It iterates sources sequentially and continues to subsequent sources if a scan returns an error,
// collecting all non-nil errors encountered.
// It returns a single error that aggregates all collected scan errors, or nil if no errors occurred.
func ForEach(ctx context.Context, sources []Source, handle func(usage.UsageEvent) error) error {
	var errs []error
	for _, source := range sources {
		if err := source.Scan(ctx, handle); err != nil {
			errs = append(errs, err)
		}
	}
	return JoinErrors(errs)
}
