package sources

import (
	"context"
	"errors"
	"sync"

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

func ForEachConcurrent(ctx context.Context, sources []Source, handle func(usage.UsageEvent) error) error {
	type sourceResult struct {
		index  int
		events []usage.UsageEvent
		err    error
	}
	results := make([][]usage.UsageEvent, len(sources))
	errs := make([]error, 0, len(sources))
	resultCh := make(chan sourceResult, len(sources))

	var wg sync.WaitGroup
	for i, source := range sources {
		wg.Add(1)
		go func(index int, source Source) {
			defer wg.Done()
			events := make([]usage.UsageEvent, 0, 128)
			err := source.Scan(ctx, func(event usage.UsageEvent) error {
				events = append(events, event)
				return nil
			})
			resultCh <- sourceResult{index: index, events: events, err: err}
		}(i, source)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		results[result.index] = result.events
		if result.err != nil && !errors.Is(result.err, context.Canceled) && !errors.Is(result.err, context.DeadlineExceeded) {
			errs = append(errs, result.err)
		}
	}
	if len(errs) > 0 {
		return JoinErrors(errs)
	}
	for _, events := range results {
		for _, event := range events {
			if err := handle(event); err != nil {
				return err
			}
		}
	}
	return nil
}
