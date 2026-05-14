package sources

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	Provider string
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
	codexTurnIDPattern         = regexp.MustCompile(`turn\.id=([^}\s]+)`)
	codexHTTPURLPattern        = regexp.MustCompile(`\b(?:POST|GET) to (https?://[^\s:"]+)`)
	codexURLFieldPattern       = regexp.MustCompile(`\burl[:=] ?(https?://[^\s,"}]+)`)
	codexConnectURLPattern     = regexp.MustCompile(`starting new connection: (https?://[^\s]+)`)
	codexHostSomePattern       = regexp.MustCompile(`host=Some\("([^"]+)"\)`)
	codexPoolHostPattern       = regexp.MustCompile(`\("https", ([^)]+)\)`)
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
	}
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

func (t codexProviderTimeline) providerForTurn(threadID, turnID string, _ time.Time) string {
	if threadID == "" || turnID == "" {
		return ""
	}
	var provider string
	for _, point := range t[threadID] {
		if point.TurnID != turnID {
			continue
		}
		if provider != "" && provider != point.Provider {
			return ""
		}
		provider = point.Provider
	}
	return provider
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
	query := codexSQLiteProviderQuery(matcher, targets)
	if query == "" {
		return nil
	}
	out, err := exec.CommandContext(ctx, sqlite, "-json", path, query).Output()
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	var rows []struct {
		TS       int64  `json:"ts"`
		ThreadID string `json:"thread_id"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil
	}
	for _, row := range rows {
		if row.ThreadID == "" || row.TS <= 0 {
			continue
		}
		provider := providerFromCodexRequestEvidence(row.Body, matcher)
		if provider == "" {
			continue
		}
		turnID := codexLogTurnID(row.Body)
		if turnID == "" {
			continue
		}
		timeline[row.ThreadID] = append(timeline[row.ThreadID], codexProviderPoint{
			At:       time.Unix(row.TS, 0).UTC(),
			TurnID:   turnID,
			Provider: provider,
		})
	}
	return timeline
}

func codexSQLiteProviderQuery(matcher codexProviderMatcher, targets codexProviderTargets) string {
	var hostFilters []string
	for _, host := range matcher.hosts() {
		escaped := strings.ReplaceAll(host, "'", "''")
		hostFilters = append(hostFilters, "feedback_log_body like '%"+escaped+"%'")
	}
	if len(hostFilters) == 0 {
		return ""
	}
	threadIDs := targets.threadIDs()
	if len(threadIDs) == 0 {
		return ""
	}
	threadFilters := make([]string, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		escaped := strings.ReplaceAll(threadID, "'", "''")
		threadFilters = append(threadFilters, "'"+escaped+"'")
	}
	sort.Strings(hostFilters)
	return `select ts, thread_id, feedback_log_body as body
from logs
where thread_id is not null
  and thread_id in (` + strings.Join(threadFilters, ",") + `)
  and (` + strings.Join(hostFilters, " or ") + `)
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
  )
order by ts;`
}

func addCodexProviderPointFromLogLine(timeline codexProviderTimeline, line string, matcher codexProviderMatcher, targets codexProviderTargets) {
	if !isCodexRequestEvidenceLine(line) {
		return
	}
	if strings.Contains(line, "ToolCall:") {
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
	provider := providerFromCodexRequestEvidence(line, matcher)
	if provider == "" {
		return
	}
	timeline[threadID] = append(timeline[threadID], codexProviderPoint{
		At:       ts,
		TurnID:   turnID,
		Provider: provider,
	})
}

func providerFromCodexRequestEvidence(line string, matcher codexProviderMatcher) string {
	rawURL, host := extractCodexRequestURLAndHost(line)
	if rawURL != "" {
		if provider := matcher.providerForURL(rawURL); provider != "" {
			return provider
		}
	}
	if host == "" {
		return ""
	}
	return matcher.hostProviders[host]
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

func codexThreadID(line string) string {
	match := codexThreadIDPattern.FindStringSubmatch(line)
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
