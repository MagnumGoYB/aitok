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
	"sort"
	"strings"
	"time"
)

type codexProviderPoint struct {
	At       time.Time
	TurnID   string
	Model    string
	Provider string
	Strength int
	Source   codexProviderEvidenceSource
}

type codexProviderPointMatch struct {
	Provider string
	Strength int
	Found    bool
	Source   codexProviderEvidenceSource
}

type codexProviderEvidenceSource int

const (
	codexProviderEvidenceUnknown codexProviderEvidenceSource = iota
	codexProviderEvidenceTextLog
	codexProviderEvidenceSQLite
)

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
	codexConversationIDPattern = regexp.MustCompile(`conversation\.id=([0-9a-fA-F-]{36})`)
	codexTurnIDPattern         = regexp.MustCompile(`turn\.id=([^}\s]+)`)
	codexHTTPURLPattern        = regexp.MustCompile(`\b(?:POST|GET) to (https?://[^\s:"]+)`)
	codexURLFieldPattern       = regexp.MustCompile(`\burl[:=] ?(https?://[^\s,"}]+)`)
	codexConnectURLPattern     = regexp.MustCompile(`starting new connection: (https?://[^\s]+)`)
	codexHostSomePattern       = regexp.MustCompile(`host=Some\("([^"]+)"\)`)
	codexPoolHostPattern       = regexp.MustCompile(`\("https", ([^)]+)\)`)
	codexAuthModeQuotedPattern = regexp.MustCompile(`auth_mode="([^"]+)"`)
	codexAuthModeSomePattern   = regexp.MustCompile(`auth_mode=Some\(([^)]+)\)`)
	codexLogModelPattern       = regexp.MustCompile(`\bmodel="?([^",}\s]+)"?`)
	codexLogSlugPattern        = regexp.MustCompile(`\bslug="?([^",}\s]+)"?`)
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
		`event.name="codex.tool_result"`,
		`event.name="codex.tool_decision"`,
		`event.name=\"codex.tool_result\"`,
		`event.name=\"codex.tool_decision\"`,
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

func readCodexProviderTimeline(ctx context.Context, home string, targets codexProviderTargets) codexProviderTimeline {
	if targets.empty() {
		return nil
	}
	matcher := newCodexProviderMatcher(readCodexProviderHosts(filepath.Join(home, ".codex", "config.toml")))
	if matcher.empty() {
		return nil
	}
	timeline := codexProviderTimeline{}
	timeline.merge(readCodexTextLogProviderTimeline(ctx, filepath.Join(home, ".codex", "log", "codex-tui.log"), matcher, targets))
	timeline.merge(readCodexSQLiteProviderTimeline(ctx, filepath.Join(home, ".codex", "logs_2.sqlite"), matcher, targets))
	for threadID := range timeline {
		sort.SliceStable(timeline[threadID], func(i, j int) bool {
			return timeline[threadID][i].At.Before(timeline[threadID][j].At)
		})
	}
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
	if match := t.exactProviderMatchForTurnAt(threadID, turnID, at); match.Found {
		return codexProviderInference{Provider: match.Provider, Exact: true}
	}
	return t.inferredProviderForTime(threadID, at)
}

func (t codexProviderTimeline) exactProviderForTurn(threadID, turnID string) (string, bool) {
	match := t.exactProviderMatchForTurn(threadID, turnID)
	return match.Provider, match.Found
}

func (t codexProviderTimeline) exactProviderMatchForTurn(threadID, turnID string) codexProviderPointMatch {
	if threadID == "" || turnID == "" {
		return codexProviderPointMatch{}
	}
	points := t.turnPoints(threadID, turnID)
	var match codexProviderPointMatch
	for i := 0; i < len(points); {
		j := i + 1
		for j < len(points) && points[j].At.Equal(points[i].At) {
			j++
		}
		resolved := resolveCodexProviderPointGroup(points[i:j])
		if resolved.Found {
			if !match.Found {
				match = resolved
			} else if match.Provider != resolved.Provider {
				return codexProviderPointMatch{Found: true}
			} else if resolved.Strength > match.Strength {
				match.Strength = resolved.Strength
				match.Source = resolved.Source
			} else if resolved.Strength == match.Strength && codexProviderEvidenceSourceRank(resolved.Source) > codexProviderEvidenceSourceRank(match.Source) {
				match.Source = resolved.Source
			}
		} else {
			return codexProviderPointMatch{Found: true}
		}
		i = j
	}
	return match
}

func (t codexProviderTimeline) turnHasMixedProviders(threadID, turnID string) bool {
	match := t.exactProviderMatchForTurn(threadID, turnID)
	return match.Found && match.Provider == ""
}

func (t codexProviderTimeline) exactProviderForTurnAt(threadID, turnID string, at time.Time) (string, bool) {
	match := t.exactProviderMatchForTurnAt(threadID, turnID, at)
	return match.Provider, match.Found
}

func (t codexProviderTimeline) exactProviderMatchForTurnAt(threadID, turnID string, at time.Time) codexProviderPointMatch {
	if threadID == "" || turnID == "" || at.IsZero() {
		return codexProviderPointMatch{}
	}
	points := t.turnPoints(threadID, turnID)
	if len(points) == 0 {
		return codexProviderPointMatch{}
	}
	end := sort.Search(len(points), func(i int) bool {
		return points[i].At.After(at)
	})
	if end == 0 {
		return codexProviderPointMatch{}
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

func resolveCodexProviderPointGroup(points []codexProviderPoint) codexProviderPointMatch {
	if len(points) == 0 {
		return codexProviderPointMatch{}
	}
	var match codexProviderPointMatch
	for _, point := range points {
		if point.Provider == "" {
			continue
		}
		if point.Strength > match.Strength {
			match = codexProviderPointMatch{
				Provider: point.Provider,
				Strength: point.Strength,
				Found:    true,
				Source:   point.Source,
			}
			continue
		}
		if point.Strength < match.Strength {
			continue
		}
		if point.Provider == match.Provider && codexProviderEvidenceSourceRank(point.Source) > codexProviderEvidenceSourceRank(match.Source) {
			match.Source = point.Source
			continue
		}
		if match.Provider != point.Provider {
			return codexProviderPointMatch{Found: true}
		}
	}
	return match
}

func codexProviderEvidenceSourceRank(source codexProviderEvidenceSource) int {
	switch source {
	case codexProviderEvidenceTextLog:
		return 2
	case codexProviderEvidenceSQLite:
		return 1
	default:
		return 0
	}
}

func (t codexProviderTimeline) hasLaterConflictingProvider(threadID, turnID, provider string, at time.Time) bool {
	if threadID == "" || turnID == "" || provider == "" || at.IsZero() {
		return false
	}
	for _, point := range t.turnPoints(threadID, turnID) {
		if !point.At.After(at) || point.Provider == "" || point.Provider == provider {
			continue
		}
		return true
	}
	return false
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

func readCodexSQLiteProviderTimeline(ctx context.Context, path string, matcher codexProviderMatcher, targets codexProviderTargets) codexProviderTimeline {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		return nil
	}
	timeline := codexProviderTimeline{}
	queries := []string{
		codexSQLiteProviderThreadQuery(matcher, targets),
		codexSQLiteProviderThreadlessQuery(matcher),
	}
	for _, query := range queries {
		scanCodexSQLiteProviderTimelineQuery(ctx, sqlite, path, query, matcher, targets, timeline)
	}
	return timeline
}

func scanCodexSQLiteProviderTimelineQuery(ctx context.Context, sqlite, path, query string, matcher codexProviderMatcher, targets codexProviderTargets, timeline codexProviderTimeline) {
	if query == "" {
		return
	}
	cmd := exec.CommandContext(ctx, sqlite, "-json", path, query)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return
	}
	defer func() {
		_ = cmd.Wait()
	}()

	dec := json.NewDecoder(stdout)
	tok, err := dec.Token()
	if err != nil {
		return
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return
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
			return
		}
		threadID := strings.TrimSpace(row.ThreadID)
		if threadID == "" {
			threadID = codexBodyThreadID(row.Body)
		}
		if threadID == "" || row.TS <= 0 || !targets.hasThread(threadID) || !isTrustedCodexSQLiteProviderTarget(row.Target) {
			continue
		}
		if shouldIgnoreCodexRequestEvidence(row.Body) {
			continue
		}
		provider, strength := providerFromCodexRequestEvidence(row.Body, matcher)
		if provider == "" {
			continue
		}
		turnID := codexLogTurnID(row.Body)
		timeline[threadID] = append(timeline[threadID], codexProviderPoint{
			At:       time.Unix(row.TS, row.TSNanos).UTC(),
			TurnID:   turnID,
			Model:    extractCodexProviderEvidenceModel(row.Body),
			Provider: provider,
			Strength: strength,
			Source:   codexProviderEvidenceSQLite,
		})
	}
}

func codexSQLiteProviderThreadQuery(matcher codexProviderMatcher, targets codexProviderTargets) string {
	base := codexSQLiteProviderQueryBase(matcher)
	if base == "" {
		return ""
	}
	threadIDs := targets.threadIDs()
	if len(threadIDs) == 0 {
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
  and ` + base
}

func codexSQLiteProviderThreadlessQuery(matcher codexProviderMatcher) string {
	base := codexSQLiteProviderQueryBase(matcher)
	if base == "" {
		return ""
	}
	return `select ts, ts_nanos, thread_id, target, feedback_log_body as body
from logs
where (thread_id is null or thread_id = '')
  and ` + base
}

func codexSQLiteProviderQueryBase(matcher codexProviderMatcher) string {
	var hostFilters []string
	for _, host := range matcher.hosts() {
		escaped := strings.ReplaceAll(host, "'", "''")
		hostFilters = append(hostFilters, "feedback_log_body like '%"+escaped+"%'")
	}
	if len(hostFilters) == 0 {
		return ""
	}
	sort.Strings(hostFilters)
	providerFilters := append([]string{}, hostFilters...)
	providerFilters = append(providerFilters, "feedback_log_body like '%auth_mode=%'")
	providerFilters = append(providerFilters, "feedback_log_body like '%provider=%'")
	trustedTargets := make([]string, 0, len(codexTrustedSQLiteProviderTargets))
	for target := range codexTrustedSQLiteProviderTargets {
		trustedTargets = append(trustedTargets, "'"+strings.ReplaceAll(target, "'", "''")+"'")
	}
	sort.Strings(trustedTargets)
	return `target in (` + strings.Join(trustedTargets, ",") + `)
  and (` + strings.Join(providerFilters, " or ") + `)
  and feedback_log_body not like '%ToolCall:%'
  and (
    feedback_log_body like '%model_client.%'
    or feedback_log_body like '%endpoint_session.%'
    or feedback_log_body like '%Request completed%'
    or feedback_log_body like '%POST to https://%'
    or feedback_log_body like '%GET to https://%'
    or feedback_log_body like '%starting new connection:%'
    or feedback_log_body like '%Http::connect;%'
    or feedback_log_body like '%checkout waiting for idle connection%'
    or feedback_log_body like '%Turn error:%'
    or feedback_log_body like '%auth_mode=%'
    or feedback_log_body like '%provider=%'
  )`
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
		Model:    extractCodexProviderEvidenceModel(line),
		Provider: provider,
		Strength: maxCodexProviderStrength(strength, codexProviderStrengthForLogLine(line)),
		Source:   codexProviderEvidenceTextLog,
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
		return "openai", codexProviderStrengthWeak
	case "":
		return "", 0
	default:
		for _, endpoint := range matcher.endpoints {
			if strings.EqualFold(endpoint.Provider, provider) {
				return endpoint.Provider, codexProviderStrengthWeak
			}
		}
		for hostProvider := range matcher.hostProviders {
			if strings.EqualFold(matcher.hostProviders[hostProvider], provider) {
				return matcher.hostProviders[hostProvider], codexProviderStrengthWeak
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

func extractCodexProviderEvidenceModel(line string) string {
	for _, pattern := range []*regexp.Regexp{
		codexLogModelPattern,
		codexLogSlugPattern,
	} {
		if match := pattern.FindStringSubmatch(line); len(match) == 2 {
			return normalizeCodexModel(strings.TrimSpace(match[1]))
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
	if strings.Contains(line, "Turn error:") || strings.Contains(line, "websocket_connection{provider=") {
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
	for _, pattern := range []*regexp.Regexp{
		codexBodyThreadIDPattern,
		codexConversationIDPattern,
	} {
		match := pattern.FindStringSubmatch(line)
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
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
