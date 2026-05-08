package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/report"
)

func TestRenderSmoke(t *testing.T) {
	view := Render(report.Payload{
		Window: query.Window{Start: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)},
	})
	if !strings.Contains(view, "aitok token usage") || !strings.Contains(view, "Press q to quit") {
		t.Fatalf("unexpected view: %s", view)
	}
}
