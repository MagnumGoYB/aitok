package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MagnumGoYB/aitok/internal/query"
	"github.com/MagnumGoYB/aitok/internal/usage"
	"github.com/mattn/go-runewidth"
)

func TestWriteJSONAndMarkdown(t *testing.T) {
	payload := Payload{
		GeneratedAt: time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
		GroupBy:     query.GroupBy{"tool"},
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex"},
			Tool:     "codex",
			Events:   1,
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 5, Output: 7},
			CostUSD:  0.0123,
			Price:    &query.Price{Source: "official", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.25, CacheMakeUSDPerMTok: 2.5},
		}},
	}
	var jsonOut bytes.Buffer
	if err := Write(&jsonOut, "json", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOut.String(), `"generated_at"`) || !strings.Contains(jsonOut.String(), `"requests": 1`) || !strings.Contains(jsonOut.String(), `"cost_usd": 0.0123`) || !strings.Contains(jsonOut.String(), `"source": "official"`) || !strings.Contains(jsonOut.String(), `"tool": "codex"`) {
		t.Fatalf("json output missing stable fields: %s", jsonOut.String())
	}
	var mdOut bytes.Buffer
	if err := Write(&mdOut, "markdown", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mdOut.String(), "| Group | Tool | Req | Cost | Price | Total |") || !strings.Contains(mdOut.String(), "| tool=codex | codex | 1 | $0.0123 | official in=$2.5/M out=$15/M cache=$0.25/M make=$2.5/M | 12 |") {
		t.Fatalf("markdown output unexpected: %s", mdOut.String())
	}
}

func TestWriteTableShowsToolColumn(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{{
		Key:      map[string]string{"tool": "reasonix", "model": "deepseek-v4-pro", "provider": "deepseek"},
		Tool:     "reasonix",
		Requests: 1,
		Usage:    usage.TokenUsage{Input: 1_000_000},
		CostUSD:  2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "GROUP") || !strings.Contains(got, "TOOL") || !strings.Contains(got, "reasonix") {
		t.Fatalf("table should show the tool column: %s", got)
	}
}

func TestWriteTableDisplaysCNYForDeepSeek(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{{
		Key:      map[string]string{"tool": "reasonix", "model": "deepseek-v4-flash"},
		Tool:     "reasonix",
		Requests: 1,
		Usage:    usage.TokenUsage{Input: 1_000_000, Output: 500_000},
		CostUSD:  2,
		Price:    &query.Price{Source: "default", Currency: "CNY", InputUSDPerMTok: 1, OutputUSDPerMTok: 2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "¥2.0000") {
		t.Fatalf("CNY cost should show ¥2.0000: %s", got)
	}
	if strings.Contains(got, "$2.0000") {
		t.Fatalf("CNY cost should NOT show $: %s", got)
	}
}

func TestWriteTableDisplaysCustomAndOfficialPriceRates(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{
		{
			Key:      map[string]string{"model": "gpt-5.4", "provider": "team-a"},
			Tool:     "codex",
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 1_000_000},
			CostUSD:  2,
			Price:    &query.Price{Source: "custom", InputUSDPerMTok: 2, OutputUSDPerMTok: 20, CacheHitUSDPerMTok: 0.2, CacheMakeUSDPerMTok: 2},
		},
		{
			Key:      map[string]string{"model": "gpt-5.4", "provider": "openai"},
			Tool:     "codex",
			Requests: 1,
			Usage:    usage.TokenUsage{Input: 1_000_000},
			CostUSD:  2.5,
			Price:    &query.Price{Source: "official", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.25, CacheMakeUSDPerMTok: 2.5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "PRICE") || !strings.Contains(got, "custom in=$2/M out=$20/M cache=$0.2/M make=$2/M") || !strings.Contains(got, "official in=$2.5/M out=$15/M cache=$0.25/M make=$2.5/M") {
		t.Fatalf("table missing price details: %s", got)
	}
}

func TestWriteTableDisplaysMixedPriceComponentRates(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{
		{
			Key:      map[string]string{"model": "gpt-5.5", "provider": "toska"},
			Tool:     "reasonix",
			Requests: 2,
			Usage:    usage.TokenUsage{Input: 2_000_000},
			CostUSD:  70,
			Price: &query.Price{Source: "mixed", Components: []query.Price{
				{Source: "custom", InputUSDPerMTok: 5, OutputUSDPerMTok: 40, CacheHitUSDPerMTok: 4.5, CacheMakeUSDPerMTok: 5},
				{Source: "official", InputUSDPerMTok: 5, OutputUSDPerMTok: 30, CacheHitUSDPerMTok: 0.5, CacheMakeUSDPerMTok: 5},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "| mixed |") || !strings.Contains(got, "mixed custom+official in=$5/M out=$30..40/M cache=$0.5..4.5/M make=$5/M") {
		t.Fatalf("table should describe mixed price components: %s", got)
	}
}

func TestWriteTableUsesCompactDefaultColumns(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{{
		Key:      map[string]string{"model": "gpt-5.4"},
		Tool:     "codex",
		Requests: 3,
		Events:   3,
		Usage:    usage.TokenUsage{Input: 100, Output: 20},
		CostUSD:  1.2345,
		Price:    &query.Price{Source: "official", InputUSDPerMTok: 2.5, OutputUSDPerMTok: 15, CacheHitUSDPerMTok: 0.25, CacheMakeUSDPerMTok: 2.5},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "TOOL") || !strings.Contains(got, "REQ") || !strings.Contains(got, "COST") || !strings.Contains(got, "PRICE") || !strings.Contains(got, "$1.2345") {
		t.Fatalf("table missing request/cost fields: %s", got)
	}
	if strings.Contains(got, "EVENTS") || strings.Contains(got, "CACHE_CREATE") {
		t.Fatalf("default table should stay compact: %s", got)
	}
	if !strings.Contains(got, "+-") || !strings.Contains(got, "GROUP") || !strings.Contains(got, "TOOL") || !strings.Contains(got, "REQ") {
		t.Fatalf("table must render borders and column separators: %s", got)
	}
}

func TestWriteTableFullShowsExpandedColumns(t *testing.T) {
	var out bytes.Buffer
	err := WriteTable(&out, []query.Result{{
		Key:      map[string]string{"model": "gpt-5.4"},
		Tool:     "codex",
		Requests: 3,
		Events:   3,
		Usage:    usage.TokenUsage{Input: 100, Output: 20},
		CostUSD:  1.2345,
	}}, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "TOOL") || !strings.Contains(got, "EVENTS") || !strings.Contains(got, "CACHE_CREATE") || !strings.Contains(got, "REASONING") || !strings.Contains(got, "TOOL_TOK") {
		t.Fatalf("full table should show expanded columns: %s", got)
	}
}

func TestWriteThreadsTableAlignsWideCharacters(t *testing.T) {
	var out bytes.Buffer
	err := WriteThreadsTable(&out, []query.ThreadResult{
		{
			ID:       "019e167b-b7e8-7743-8bb3-fd9951e5ef2f",
			Name:     "修正日期范围与threads列表",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "bcb",
			Requests: 199,
			Events:   199,
			Usage:    usage.TokenUsage{Input: 28_345_680},
			CostUSD:  22.0954,
		},
		{
			ID:       "019e1522-e729-70c2-b013-bf66207c6b51",
			Name:     "mini_program_wechat",
			Tool:     "codex",
			Model:    "gpt-5.5",
			Provider: "bcb",
			Requests: 61,
			Events:   61,
			Usage:    usage.TokenUsage{Input: 6_125_217},
			CostUSD:  6.7136,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := nonEmptyLines(out.String())
	if len(lines) < 5 {
		t.Fatalf("expected bordered table, got: %s", out.String())
	}
	width := runewidth.StringWidth(lines[0])
	for _, line := range lines[1:] {
		if got := runewidth.StringWidth(line); got != width {
			t.Fatalf("table row display width = %d, want %d\nrow: %q\nfull table:\n%s", got, width, line, out.String())
		}
	}
}

func TestWriteThreadsWhenPayloadIncludesThreads(t *testing.T) {
	payload := Payload{
		Results: []query.Result{{
			Key:      map[string]string{"tool": "codex"},
			Requests: 1,
			Events:   1,
			Usage:    usage.TokenUsage{Input: 5},
		}},
		Threads: []query.ThreadResult{{
			ID:       "thread-a",
			Name:     "Custom title",
			Tool:     "codex",
			Model:    "gpt-5.4",
			Provider: "openai",
			Requests: 1,
			Events:   1,
			Usage:    usage.TokenUsage{Input: 5},
			CostUSD:  0.0001,
		}},
	}
	var out bytes.Buffer
	if err := Write(&out, "markdown", payload); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "## Threads") || !strings.Contains(got, "| thread-a | Custom title | codex | gpt-5.4 | openai |") {
		t.Fatalf("markdown output missing threads: %s", got)
	}
	out.Reset()
	if err := Write(&out, "json", payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"threads"`) || !strings.Contains(out.String(), `"id": "thread-a"`) {
		t.Fatalf("json output missing threads: %s", out.String())
	}
}

func TestWriteThreadsTableUsesCompactDefaultColumns(t *testing.T) {
	var out bytes.Buffer
	err := WriteThreadsTable(&out, []query.ThreadResult{{
		ID:       "thread-a",
		Name:     "Custom title",
		Tool:     "codex",
		Model:    "gpt-5.4",
		Provider: "openai",
		Requests: 1,
		Events:   1,
		Usage:    usage.TokenUsage{Input: 5},
		CostUSD:  0.0001,
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "REQ") || strings.Contains(got, "EVENTS") {
		t.Fatalf("threads default table should stay compact: %s", got)
	}
}

func TestWriteThreadsTableShowsProviderCostBreakdown(t *testing.T) {
	var out bytes.Buffer
	err := WriteThreadsTable(&out, []query.ThreadResult{{
		ID:       "thread-a",
		Name:     "Custom title",
		Tool:     "codex",
		Model:    "gpt-5.5",
		Provider: "bcb,toska",
		Requests: 2,
		Events:   2,
		Usage:    usage.TokenUsage{Input: 5},
		CostUSD:  301.11,
		CostBreakdown: []query.ThreadCost{
			{Provider: "toska", USD: 299.7687},
			{Provider: "bcb", USD: 1.3412},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "bcb,toska") || !strings.Contains(got, "$301.1100 (toska $299.7687, bcb $1.3412)") {
		t.Fatalf("threads table should show provider list and cost breakdown: %s", got)
	}
}

func TestWriteThreadsTableFullShowsEvents(t *testing.T) {
	var out bytes.Buffer
	err := WriteThreadsTable(&out, []query.ThreadResult{{
		ID:       "thread-a",
		Name:     "Custom title",
		Tool:     "codex",
		Model:    "gpt-5.4",
		Provider: "openai",
		Requests: 1,
		Events:   1,
		Usage:    usage.TokenUsage{Input: 5},
		CostUSD:  0.0001,
	}}, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "EVENTS") {
		t.Fatalf("threads full table should show events: %s", out.String())
	}
}

func nonEmptyLines(value string) []string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
