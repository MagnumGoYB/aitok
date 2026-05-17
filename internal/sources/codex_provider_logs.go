package sources

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type codexProviderPoint struct {
	At       time.Time
	TurnID   string
	Provider string
	Strength int
}

type codexProviderInference struct {
	Provider string
	Exact    bool
	Prev     *codexProviderPoint
	Next     *codexProviderPoint
}

type codexProviderTimeline map[string][]codexProviderPoint

type codexProviderHost struct {
	BaseURL  string
	Host     string
	Provider string
}

type codexProviderMatcher struct {
	endpoints     []codexProviderHost
	hostProviders map[string]string
}

type codexProviderTargets struct {
	threads map[string]struct{}
}

var (
	codexThreadIDPattern       = regexp.MustCompile(`thread_id=([0-9a-fA-F-]{36})`)
	codexBodyThreadIDPattern   = regexp.MustCompile(`thread\.id=([0-9a-fA-F-]{36})`)
	codexTurnIDPattern         = regexp.MustCompile(`turn\.id=([^}\s]+)`)
	codexHTTPURLPattern        = regexp.MustCompile(`\b(?:POST|GET) to (https?://[^\s:"]+)`)
	codexURLFieldPattern       = regexp.MustCompile(`\burl[:=] ?(https?://[^\s,"}]+)`)
	codexConnectURLPattern     = regexp.MustCompile(`starting new connection: (https?://[^\s]+)`)
	codexHostSomePattern       = regexp.MustCompile(`host=Some\("([^"]+)"\)`)
	codexPoolHostPattern       = regexp.MustCompile(`\("https", ([^)]+)\)`)
	codexAuthModeQuotedPattern = regexp.MustCompile(`auth_mode="([^"]+)"`)
	codexAuthModeSomePattern   = regexp.MustCompile(`auth_mode=Some\(([^)]+)\)`)
	codexProviderFieldPattern  = regexp.MustCompile(`provider="?([A-Za-z0-9._-]+)"?`)
	codexRequestEvidenceTokens = []string{
		"model_client.",
		"endpoint_session.",
		"Request completed",
		"POST to https://",
		"GET to https://",
		"starting new connection:",
		"Http::connect;",
		"checkout waiting for idle connection",
		"Turn error:",
		"auth_mode=",
		"provider=",
	}
	codexTrustedSQLiteProviderTargets = map[string]struct{}{
		"codex_api::sse::responses":                 {},
		"codex_client::default_client":              {},
		"codex_client::transport":                   {},
		"codex_otel.log_only":                       {},
		"codex_otel.trace_safe":                     {},
		"feedback_tags":                             {},
		"hyper_util::client::legacy::connect::http": {},
	}
	codexIgnoredTextLogProviderTargets = map[string]struct{}{
		"codex_core::stream_events_utils": {},
	}
	codexIgnoredRequestEvidenceSubstrings = []string{
		`run_turn:list_models`,
		`api.path="models"`,
	}
)

const (
	codexProviderStrengthWeak   = 1
	codexProviderStrengthStrong = 2
	codexProviderStrengthAuth   = 1
)

func newCodexProviderTargets() codexProviderTargets {
	return codexProviderTargets{threads: map[string]struct{}{}}
}

func (t *codexProviderTargets) addThread(threadID string) {
	if threadID == "" {
		return
	}
	if t.threads == nil {
		t.threads = map[string]struct{}{}
	}
	t.threads[threadID] = struct{}{}
}

func (t codexProviderTargets) empty() bool {
	return len(t.threads) == 0
}

func (t codexProviderTargets) hasThread(threadID string) bool {
	_, ok := t.threads[threadID]
	return ok
}

func (t codexProviderTargets) threadIDs() []string {
	ids := make([]string, 0, len(t.threads))
	for id := range t.threads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func readCodexProviderTimeline(ctx context.Context, home string, windowStart, windowEnd time.Time, targets codexProviderTargets) codexProviderTimeline {
	if targets.empty() {
		return nil
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	textLogPath := filepath.Join(home, ".codex", "log", "codex-tui.log")
	sqlitePath := filepath.Join(home, ".codex", "logs_2.sqlite")
	cacheSignature := providerTimelineCacheSignature(append([]string{configPath, textLogPath, sqlitePath}, targets.threadIDs()...))
	if cached, ok := readProviderTimelineCache(cacheSignature); ok {
		return cached
	}
	matcher := newCodexProviderMatcher(readCodexProviderHosts(configPath))
	if matcher.empty() {
		return nil
	}
	timeline := codexProviderTimeline{}
	timeline.merge(readCodexTextLogProviderTimeline(ctx, textLogPath, matcher, targets))
	sqliteTargets := targetsWithoutTimeline(targets, timeline)
	if !sqliteTargets.empty() {
		timeline.merge(readCodexSQLiteProviderTimeline(ctx, sqlitePath, windowStart, windowEnd, matcher, sqliteTargets))
	}
	for threadID := range timeline {
		sort.SliceStable(timeline[threadID], func(i, j int) bool {
			return timeline[threadID][i].At.Before(timeline[threadID][j].At)
		})
	}
	writeProviderTimelineCache(cacheSignature, timeline)
	return timeline
}

func newCodexProviderMatcher(hosts []codexProviderHost) codexProviderMatcher {
	matcher := codexProviderMatcher{
		hostProviders: uniqueCodexHostProviders(hosts),
	}
	for _, host := range hosts {
		if host.BaseURL == "" || host.Provider == "" {
			continue
		}
		matcher.endpoints = append(matcher.endpoints, host)
	}
	sort.Slice(matcher.endpoints, func(i, j int) bool {
		if len(matcher.endpoints[i].BaseURL) != len(matcher.endpoints[j].BaseURL) {
			return len(matcher.endpoints[i].BaseURL) > len(matcher.endpoints[j].BaseURL)
		}
		if matcher.endpoints[i].BaseURL == matcher.endpoints[j].BaseURL {
			return matcher.endpoints[i].Provider < matcher.endpoints[j].Provider
		}
		return matcher.endpoints[i].BaseURL < matcher.endpoints[j].BaseURL
	})
	return matcher
}

func (m codexProviderMatcher) empty() bool {
	return len(m.endpoints) == 0 && len(m.hostProviders) == 0
}

func (m codexProviderMatcher) hosts() []string {
	seen := map[string]struct{}{}
	for host := range m.hostProviders {
		seen[host] = struct{}{}
	}
	for _, endpoint := range m.endpoints {
		if endpoint.Host != "" {
			seen[endpoint.Host] = struct{}{}
		}
	}
	hosts := make([]string, 0, len(seen))
	for host := range seen {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}

func uniqueCodexHostProviders(hosts []codexProviderHost) map[string]string {
	out := map[string]string{}
	ambiguous := map[string]struct{}{}
	for _, host := range hosts {
		name := strings.ToLower(strings.TrimSpace(host.Host))
		if name == "" {
			continue
		}
		if existing, ok := out[name]; ok && existing != host.Provider {
			ambiguous[name] = struct{}{}
			continue
		}
		out[name] = host.Provider
	}
	for host := range ambiguous {
		delete(out, host)
	}
	return out
}

func (t codexProviderTimeline) merge(other codexProviderTimeline) {
	for threadID, points := range other {
		t[threadID] = append(t[threadID], points...)
	}
}

func (t codexProviderTimeline) providerForTurn(threadID, turnID string, at time.Time) string {
	return t.inferenceForTurn(threadID, turnID, at).Provider
}

func (t codexProviderTimeline) inferenceForTurn(threadID, turnID string, at time.Time) codexProviderInference {
	if threadID == "" || turnID == "" {
		return codexProviderInference{}
	}
	if provider, exact := t.exactProviderForTurnAt(threadID, turnID, at); exact {
		return codexProviderInference{Provider: provider, Exact: true}
	}
	return t.inferredProviderForTime(threadID, at)
}

func (t codexProviderTimeline) exactProviderForTurn(threadID, turnID string) (string, bool) {
	if threadID == "" || turnID == "" {
		return "", false
	}
	points := t.turnPoints(threadID, turnID)
	if len(points) == 0 {
		return "", false
	}
	var provider string
	var found bool
	for i := 0; i < len(points); {
		j := i + 1
		for j < len(points) && points[j].At.Equal(points[i].At) {
			j++
		}
		resolved, ok := resolveCodexProviderPointGroup(points[i:j])
		if ok {
			if !found {
				provider = resolved
				found = true
			} else if provider != resolved {
				return "", true
			}
		} else {
			return "", true
		}
		i = j
	}
	return provider, found
}

func (t codexProviderTimeline) exactProviderForTurnAt(threadID, turnID string, at time.Time) (string, bool) {
	if threadID == "" || turnID == "" || at.IsZero() {
		return "", false
	}
	points := t.turnPoints(threadID, turnID)
	if len(points) == 0 {
		return "", false
	}
	end := sort.Search(len(points), func(i int) bool {
		return points[i].At.After(at)
	})
	if end == 0 {
		return "", false
	}
	start := end - 1
	for start > 0 && points[start-1].At.Equal(points[end-1].At) {
		start--
	}
	return resolveCodexProviderPointGroup(points[start:end])
}

func (t codexProviderTimeline) turnPoints(threadID, turnID string) []codexProviderPoint {
	if threadID == "" || turnID == "" {
		return nil
	}
	points := t[threadID]
	filtered := make([]codexProviderPoint, 0, len(points))
	for _, point := range points {
		if point.TurnID != turnID {
			continue
		}
		filtered = append(filtered, point)
	}
	return filtered
}

func resolveCodexProviderPointGroup(points []codexProviderPoint) (string, bool) {
	if len(points) == 0 {
		return "", false
	}
	var provider string
	var bestStrength int
	var found bool
	for _, point := range points {
		if point.Provider == "" {
			continue
		}
		if point.Strength > bestStrength {
			bestStrength = point.Strength
			provider = point.Provider
			found = true
			continue
		}
		if point.Strength < bestStrength {
			continue
		}
		if provider != point.Provider {
			return "", true
		}
		found = true
	}
	if !found {
		return "", false
	}
	return provider, true
}

func (t codexProviderTimeline) inferredProviderForTime(threadID string, at time.Time) codexProviderInference {
	return t.inferredProviderForTimeExcludingTurn(threadID, "", at)
}

func (t codexProviderTimeline) inferredProviderForTimeExcludingTurn(threadID, excludedTurnID string, at time.Time) codexProviderInference {
	if threadID == "" || at.IsZero() {
		return codexProviderInference{}
	}
	points := t[threadID]
	if excludedTurnID != "" {
		filtered := make([]codexProviderPoint, 0, len(points))
		for _, point := range points {
			if point.TurnID == excludedTurnID {
				continue
			}
			filtered = append(filtered, point)
		}
		points = filtered
	}
	if len(points) == 0 {
		return codexProviderInference{}
	}
	idx := sort.Search(len(points), func(i int) bool {
		return !points[i].At.Before(at)
	})
	var prev *codexProviderPoint
	if idx > 0 {
		prev = &points[idx-1]
	}
	var next *codexProviderPoint
	if idx < len(points) {
		next = &points[idx]
	}
	switch {
	case prev == nil:
		return codexProviderInference{Prev: prev, Next: next}
	case next == nil:
		return codexProviderInference{Prev: prev, Next: next}
	case prev.Provider == next.Provider:
		return codexProviderInference{Provider: prev.Provider, Prev: prev, Next: next}
	}
	return codexProviderInference{Provider: prev.Provider, Prev: prev, Next: next}
}

func readCodexProviderHosts(path string) []codexProviderHost {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	type providerConfig struct {
		section string
		name    string
		baseURL string
	}
	providers := map[string]*providerConfig{}
	var current *providerConfig
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[model_providers.") && strings.HasSuffix(line, "]") {
			section := strings.TrimSuffix(strings.TrimPrefix(line, "[model_providers."), "]")
			section = strings.Trim(section, `"'`)
			current = &providerConfig{section: section}
			providers[section] = current
			continue
		}
		if strings.HasPrefix(line, "[") {
			current = nil
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := parseTomlStringAssignment(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			current.name = value
		case "base_url":
			current.baseURL = value
		}
	}
	var hosts []codexProviderHost
	seen := map[string]struct{}{}
	for _, provider := range providers {
		name := strings.TrimSpace(provider.name)
		if name == "" {
			name = provider.section
		}
		baseURL, host := normalizeCodexProviderBaseURL(provider.baseURL)
		if name == "" || host == "" {
			continue
		}
		key := baseURL + "|" + host + "|" + name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hosts = append(hosts, codexProviderHost{BaseURL: baseURL, Host: host, Provider: name})
	}
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].BaseURL != hosts[j].BaseURL {
			return hosts[i].BaseURL < hosts[j].BaseURL
		}
		if hosts[i].Host == hosts[j].Host {
			return hosts[i].Provider < hosts[j].Provider
		}
		return hosts[i].Host < hosts[j].Host
	})
	return hosts
}

func parseTomlStringAssignment(line string) (string, string, bool) {
	key, raw, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", "", false
	}
	quote := raw[0]
	if quote != '"' && quote != '\'' {
		return "", "", false
	}
	end := strings.IndexByte(raw[1:], quote)
	if end < 0 {
		return "", "", false
	}
	return key, raw[1 : 1+end], true
}

func normalizeCodexProviderBaseURL(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.RawPath = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), strings.ToLower(parsed.Hostname())
}

func normalizeCodexRequestURL(raw string) (string, string) {
	normalized, host := normalizeCodexProviderBaseURL(raw)
	return normalized, host
}

func readCodexTextLogProviderTimeline(ctx context.Context, path string, matcher codexProviderMatcher, targets codexProviderTargets) codexProviderTimeline {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	timeline := codexProviderTimeline{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return timeline
		}
		addCodexProviderPointFromLogLine(timeline, scanner.Text(), matcher, targets)
	}
	return timeline
}

func readCodexSQLiteProviderTimeline(ctx context.Context, path string, windowStart, windowEnd time.Time, matcher codexProviderMatcher, targets codexProviderTargets) codexProviderTimeline {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil
	}
	queries := []string{codexSQLiteProviderThreadlessQuery(matcher, windowStart, windowEnd)}
	queries = append(queries, codexSQLiteProviderThreadQueries(matcher, windowStart, windowEnd, targets)...)
	if len(queries) == 0 {
		return nil
	}
	timeline := codexProviderTimeline{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	parallelism := minInt(len(queries), minInt(maxCodexSQLiteQueryParallelism, runtime.NumCPU()))
	if parallelism < 1 {
		parallelism = 1
	}
	sem := make(chan struct{}, parallelism)
	for _, query := range queries {
		if query == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(query string) {
			defer wg.Done()
			defer func() { <-sem }()
			local := scanCodexSQLiteProviderTimelineQuery(ctx, sqlite, path, query, matcher, targets)
			if len(local) == 0 {
				return
			}
			mu.Lock()
			timeline.merge(local)
			mu.Unlock()
		}(query)
	}
	wg.Wait()
	return timeline
}

const maxCodexSQLiteQueryParallelism = 4

func scanCodexSQLiteProviderTimelineQuery(ctx context.Context, sqlite, path, query string, matcher codexProviderMatcher, targets codexProviderTargets) codexProviderTimeline {
	timeline := codexProviderTimeline{}
	if query == "" {
		return timeline
	}
	cmd := exec.CommandContext(ctx, sqlite, "-json", path, query)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return timeline
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return timeline
	}
	defer func() {
		_ = cmd.Wait()
	}()

	dec := json.NewDecoder(stdout)
	tok, err := dec.Token()
	if err != nil {
		return timeline
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return timeline
	}
	for dec.More() {
		var row struct {
			TS       int64  `json:"ts"`
			TSNanos  int64  `json:"ts_nanos"`
			ThreadID string `json:"thread_id"`
			Target   string `json:"target"`
			Body     string `json:"body"`
		}
		if err := dec.Decode(&row); err != nil {
			return timeline
		}
		threadID := strings.TrimSpace(row.ThreadID)
		if threadID == "" {
			threadID = codexBodyThreadID(row.Body)
		}
		if threadID == "" || row.TS <= 0 || !targets.hasThread(threadID) || !isTrustedCodexSQLiteProviderTarget(row.Target) {
			continue
		}
		if shouldIgnoreCodexRequestEvidence(row.Body) || !isCodexRequestEvidenceLine(row.Body) {
			continue
		}
		provider, strength := providerFromCodexRequestEvidence(row.Body, matcher)
		if provider == "" {
			continue
		}
		turnID := codexLogTurnID(row.Body)
		if turnID == "" {
			continue
		}
		timeline[threadID] = append(timeline[threadID], codexProviderPoint{
			At:       time.Unix(row.TS, row.TSNanos).UTC(),
			TurnID:   turnID,
			Provider: provider,
			Strength: strength,
		})
	}
	return timeline
}

func codexSQLiteProviderThreadQueries(matcher codexProviderMatcher, windowStart, windowEnd time.Time, targets codexProviderTargets) []string {
	threadIDs := targets.threadIDs()
	if len(threadIDs) == 0 {
		return nil
	}
	chunkSize := len(threadIDs) / maxCodexSQLiteQueryParallelism
	if chunkSize < 8 {
		chunkSize = 8
	}
	queries := make([]string, 0, (len(threadIDs)+chunkSize-1)/chunkSize)
	for i := 0; i < len(threadIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(threadIDs) {
			end = len(threadIDs)
		}
		if query := codexSQLiteProviderThreadQuery(matcher, windowStart, windowEnd, threadIDs[i:end]); query != "" {
			queries = append(queries, query)
		}
	}
	return queries
}

func codexSQLiteProviderThreadQuery(matcher codexProviderMatcher, windowStart, windowEnd time.Time, threadIDs []string) string {
	if len(threadIDs) == 0 {
		return ""
	}
	trustedTargets := codexSQLiteProviderTrustedTargetsClause()
	if trustedTargets == "" {
		return ""
	}
	threadFilters := make([]string, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		threadFilters = append(threadFilters, "'"+strings.ReplaceAll(threadID, "'", "''")+"'")
	}
	sort.Strings(threadFilters)
	return `select ts, ts_nanos, thread_id, target, feedback_log_body as body
from logs
where thread_id in (` + strings.Join(threadFilters, ",") + `)
` + codexSQLiteTimeWindowClause(windowStart, windowEnd) + `
  and ` + trustedTargets
}

func codexSQLiteProviderThreadlessQuery(matcher codexProviderMatcher, windowStart, windowEnd time.Time) string {
	trustedTargets := codexSQLiteProviderTrustedTargetsClause()
	if trustedTargets == "" {
		return ""
	}
	return `select ts, ts_nanos, thread_id, target, feedback_log_body as body
from logs
where (thread_id is null or thread_id = '')
` + codexSQLiteTimeWindowClause(windowStart, windowEnd) + `
  and ` + trustedTargets
}

func codexSQLiteTimeWindowClause(windowStart, windowEnd time.Time) string {
	if windowStart.IsZero() || windowEnd.IsZero() {
		return ""
	}
	return `  and ts >= ` + strconv.FormatInt(windowStart.Unix(), 10) + `
  and ts < ` + strconv.FormatInt(windowEnd.Unix(), 10)
}

func targetsWithoutTimeline(targets codexProviderTargets, timeline codexProviderTimeline) codexProviderTargets {
	if targets.empty() {
		return targets
	}
	if len(timeline) == 0 {
		return targets
	}
	out := newCodexProviderTargets()
	for threadID := range targets.threads {
		if len(timeline[threadID]) == 0 {
			out.addThread(threadID)
		}
	}
	return out
}

func codexSQLiteProviderTrustedTargetsClause() string {
	trustedTargets := make([]string, 0, len(codexTrustedSQLiteProviderTargets))
	for target := range codexTrustedSQLiteProviderTargets {
		trustedTargets = append(trustedTargets, "'"+strings.ReplaceAll(target, "'", "''")+"'")
	}
	sort.Strings(trustedTargets)
	if len(trustedTargets) == 0 {
		return ""
	}
	return `target in (` + strings.Join(trustedTargets, ",") + `)`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func addCodexProviderPointFromLogLine(timeline codexProviderTimeline, line string, matcher codexProviderMatcher, targets codexProviderTargets) {
	if !isCodexRequestEvidenceLine(line) {
		return
	}
	if shouldIgnoreCodexRequestEvidence(line) {
		return
	}
	if strings.Contains(line, "ToolCall:") {
		return
	}
	if shouldIgnoreCodexTextLogProviderLine(line) {
		return
	}
	threadID := codexThreadID(line)
	if threadID == "" || !targets.hasThread(threadID) {
		return
	}
	turnID := codexLogTurnID(line)
	if turnID == "" {
		return
	}
	ts, err := parseCodexLogTimestamp(line)
	if err != nil {
		return
	}
	provider, strength := providerFromCodexRequestEvidence(line, matcher)
	if provider == "" {
		return
	}
	timeline[threadID] = append(timeline[threadID], codexProviderPoint{
		At:       ts,
		TurnID:   turnID,
		Provider: provider,
		Strength: maxCodexProviderStrength(strength, codexProviderStrengthForLogLine(line)),
	})
}

func providerFromCodexRequestEvidence(line string, matcher codexProviderMatcher) (string, int) {
	if provider, strength := providerFromCodexWebsocketProvider(line, matcher); provider != "" {
		return provider, strength
	}
	rawURL, host := extractCodexRequestURLAndHost(line)
	if rawURL != "" {
		if provider := matcher.providerForURL(rawURL); provider != "" {
			return provider, codexProviderStrengthStrong
		}
	}
	if host == "" {
		return "", 0
	}
	provider := matcher.hostProviders[host]
	if provider == "" {
		return providerFromCodexAuthMode(line)
	}
	return provider, codexProviderStrengthStrong
}

func providerFromCodexAuthMode(line string) (string, int) {
	if !isCodexAPIRequestAuthEvidence(line) {
		return "", 0
	}
	authMode := extractCodexAuthMode(line)
	switch {
	case strings.EqualFold(authMode, "Chatgpt"):
		return "openai", codexProviderStrengthAuth
	default:
		return "", 0
	}
}

func providerFromCodexWebsocketProvider(line string, matcher codexProviderMatcher) (string, int) {
	if !strings.Contains(line, "websocket_connection{provider=") {
		return "", 0
	}
	match := codexProviderFieldPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return "", 0
	}
	switch provider := strings.ToLower(strings.TrimSpace(match[1])); provider {
	case "openai":
		return "openai", codexProviderStrengthStrong
	case "":
		return "", 0
	default:
		for _, endpoint := range matcher.endpoints {
			if strings.EqualFold(endpoint.Provider, provider) {
				return endpoint.Provider, codexProviderStrengthStrong
			}
		}
		for hostProvider := range matcher.hostProviders {
			if strings.EqualFold(matcher.hostProviders[hostProvider], provider) {
				return matcher.hostProviders[hostProvider], codexProviderStrengthStrong
			}
		}
		return "", 0
	}
}

func isCodexAPIRequestAuthEvidence(line string) bool {
	return strings.Contains(line, `event.name="codex.api_request"`) ||
		strings.Contains(line, `event.name=\"codex.api_request\"`)
}

func extractCodexAuthMode(line string) string {
	for _, pattern := range []*regexp.Regexp{
		codexAuthModeQuotedPattern,
		codexAuthModeSomePattern,
	} {
		if match := pattern.FindStringSubmatch(line); len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func (m codexProviderMatcher) providerForURL(rawURL string) string {
	requestURL, host := normalizeCodexRequestURL(rawURL)
	if requestURL == "" {
		return ""
	}
	var provider string
	var matchedLen int
	for _, endpoint := range m.endpoints {
		if endpoint.Host != host || !codexURLHasBase(requestURL, endpoint.BaseURL) {
			continue
		}
		if len(endpoint.BaseURL) < matchedLen {
			continue
		}
		if len(endpoint.BaseURL) == matchedLen && provider != "" && provider != endpoint.Provider {
			return ""
		}
		provider = endpoint.Provider
		matchedLen = len(endpoint.BaseURL)
	}
	if provider != "" {
		return provider
	}
	return m.hostProviders[host]
}

func codexURLHasBase(rawURL, baseURL string) bool {
	return rawURL == baseURL || strings.HasPrefix(rawURL, baseURL+"/")
}

func extractCodexRequestURLAndHost(line string) (string, string) {
	for _, pattern := range []*regexp.Regexp{
		codexHTTPURLPattern,
		codexURLFieldPattern,
		codexConnectURLPattern,
	} {
		if match := pattern.FindStringSubmatch(line); len(match) == 2 {
			if _, host := normalizeCodexRequestURL(match[1]); host != "" {
				return match[1], host
			}
		}
	}
	if match := codexHostSomePattern.FindStringSubmatch(line); len(match) == 2 {
		return "", strings.ToLower(strings.TrimSpace(match[1]))
	}
	if match := codexPoolHostPattern.FindStringSubmatch(line); len(match) == 2 {
		return "", strings.ToLower(strings.TrimSpace(match[1]))
	}
	return "", ""
}

func isCodexRequestEvidenceLine(line string) bool {
	for _, token := range codexRequestEvidenceTokens {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

func shouldIgnoreCodexRequestEvidence(line string) bool {
	for _, token := range codexIgnoredRequestEvidenceSubstrings {
		if strings.Contains(line, token) {
			return true
		}
	}
	return false
}

func isTrustedCodexSQLiteProviderTarget(target string) bool {
	_, ok := codexTrustedSQLiteProviderTargets[strings.TrimSpace(target)]
	return ok
}

func shouldIgnoreCodexTextLogProviderLine(line string) bool {
	target := codexTextLogProviderTarget(line)
	if target == "" {
		return false
	}
	_, ignored := codexIgnoredTextLogProviderTargets[target]
	return ignored
}

func codexProviderStrengthForLogLine(line string) int {
	if strings.Contains(line, "Turn error:") {
		return codexProviderStrengthWeak
	}
	return codexProviderStrengthStrong
}

func maxCodexProviderStrength(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func codexTextLogProviderTarget(line string) string {
	for target := range codexIgnoredTextLogProviderTargets {
		if strings.Contains(line, ": "+target+":") {
			return target
		}
	}
	for target := range codexTrustedSQLiteProviderTargets {
		if strings.Contains(line, ": "+target+":") {
			return target
		}
	}
	return ""
}

func codexThreadID(line string) string {
	match := codexThreadIDPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func codexBodyThreadID(line string) string {
	match := codexBodyThreadIDPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func codexLogTurnID(line string) string {
	match := codexTurnIDPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return ""
	}
	return strings.Trim(match[1], `"`)
}

func parseCodexLogTimestamp(line string) (time.Time, error) {
	field, _, _ := strings.Cut(line, " ")
	return time.Parse(time.RFC3339Nano, field)
}
