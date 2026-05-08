package updatecheck

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://api.github.com/repos/MagnumGoYB/aitok/releases/latest"
	cacheFile       = ".aitok/version-check.json"
)

type InstallMethod string

const (
	InstallHomebrew InstallMethod = "homebrew"
	InstallGo       InstallMethod = "go"
	InstallRelease  InstallMethod = "release"
	InstallDev      InstallMethod = "dev"
)

type Options struct {
	Home        string
	Current     string
	Endpoint    string
	Executable  string
	Now         func() time.Time
	In          io.Reader
	Err         io.Writer
	HTTPClient  *http.Client
	RunCommand  func(context.Context, string, []string, io.Writer) error
	CheckPeriod time.Duration
}

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	URL       string    `json:"url"`
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	URL     string `json:"html_url"`
}

func MaybeRun(ctx context.Context, opts Options) error {
	if disabled() || !releaseVersion(opts.Current) {
		return nil
	}
	opts = defaults(opts)
	if fresh(opts) {
		return nil
	}
	latest, err := fetchLatest(ctx, opts)
	if err != nil {
		return nil
	}
	if latest.TagName == "" {
		return nil
	}
	latestVersion := strings.TrimPrefix(latest.TagName, "v")
	_ = writeCache(opts, cache{CheckedAt: opts.Now(), Latest: latestVersion, URL: latest.URL})
	if compareVersion(latestVersion, opts.Current) <= 0 {
		return nil
	}
	method := DetectInstallMethod(opts.Executable)
	command := UpgradeCommand(method)
	if command == "" {
		fmt.Fprintf(opts.Err, "aitok %s is available, current version is %s. Download: %s\n", latestVersion, opts.Current, latest.URL)
		return nil
	}
	if !interactive(opts.In) {
		fmt.Fprintf(opts.Err, "aitok %s is available, current version is %s. Upgrade with: %s\n", latestVersion, opts.Current, command)
		return nil
	}
	fmt.Fprintf(opts.Err, "aitok %s is available, current version is %s. Run `%s` now? [y/N] ", latestVersion, opts.Current, command)
	answer, _ := bufio.NewReader(opts.In).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		return nil
	}
	return runUpgrade(ctx, opts, method)
}

func RunUpdate(ctx context.Context, opts Options) error {
	if !releaseVersion(opts.Current) {
		return fmt.Errorf("current version %q is not a release version", opts.Current)
	}
	opts = defaults(opts)
	latest, err := fetchLatest(ctx, opts)
	if err != nil {
		return err
	}
	if latest.TagName == "" {
		return fmt.Errorf("latest release response did not include a tag name")
	}
	latestVersion := strings.TrimPrefix(latest.TagName, "v")
	_ = writeCache(opts, cache{CheckedAt: opts.Now(), Latest: latestVersion, URL: latest.URL})
	if compareVersion(latestVersion, opts.Current) <= 0 {
		fmt.Fprintf(opts.Err, "aitok %s is already up to date.\n", opts.Current)
		return nil
	}
	method := DetectInstallMethod(opts.Executable)
	command := UpgradeCommand(method)
	if command == "" {
		fmt.Fprintf(opts.Err, "aitok %s is available, current version is %s. Download: %s\n", latestVersion, opts.Current, latest.URL)
		return nil
	}
	fmt.Fprintf(opts.Err, "Updating aitok from %s to %s with: %s\n", opts.Current, latestVersion, command)
	return runUpgrade(ctx, opts, method)
}

func DetectInstallMethod(executable string) InstallMethod {
	if executable == "" {
		return InstallDev
	}
	path := executable
	if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != "" {
		path = resolved
	}
	clean := filepath.ToSlash(path)
	if strings.Contains(clean, "/Caskroom/aitok/") {
		return InstallHomebrew
	}
	if strings.HasSuffix(clean, "/go/bin/aitok") || strings.Contains(clean, "/go/bin/") {
		return InstallGo
	}
	if strings.Contains(clean, "/go-build") || strings.Contains(clean, "/Temp/") || strings.Contains(clean, "/tmp/") {
		return InstallDev
	}
	return InstallRelease
}

func UpgradeCommand(method InstallMethod) string {
	switch method {
	case InstallHomebrew:
		return "brew update && brew upgrade --cask aitok"
	case InstallGo:
		return "go install github.com/MagnumGoYB/aitok/cmd/aitok@latest"
	default:
		return ""
	}
}

func defaults(opts Options) Options {
	if opts.Endpoint == "" {
		opts.Endpoint = defaultEndpoint
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 1500 * time.Millisecond}
	}
	if opts.RunCommand == nil {
		opts.RunCommand = runCommand
	}
	if opts.CheckPeriod == 0 {
		opts.CheckPeriod = 24 * time.Hour
	}
	return opts
}

func disabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AITOK_NO_VERSION_CHECK")))
	return value == "1" || value == "true" || value == "yes"
}

func releaseVersion(version string) bool {
	return regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`).MatchString(version)
}

func fresh(opts Options) bool {
	data, err := os.ReadFile(filepath.Join(opts.Home, cacheFile))
	if err != nil {
		return false
	}
	var existing cache
	if err := json.Unmarshal(data, &existing); err != nil {
		return false
	}
	return opts.Now().Sub(existing.CheckedAt) < opts.CheckPeriod
}

func fetchLatest(ctx context.Context, opts Options) (latestRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.Endpoint, nil)
	if err != nil {
		return latestRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aitok-version-check")
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return latestRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return latestRelease{}, fmt.Errorf("version check returned %s", resp.Status)
	}
	var latest latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return latestRelease{}, err
	}
	return latest, nil
}

func writeCache(opts Options, value cache) error {
	path := filepath.Join(opts.Home, cacheFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func interactive(in io.Reader) bool {
	file, ok := in.(interface {
		Stat() (os.FileInfo, error)
	})
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func runUpgrade(ctx context.Context, opts Options, method InstallMethod) error {
	switch method {
	case InstallHomebrew:
		if err := opts.RunCommand(ctx, "brew", []string{"update"}, opts.Err); err != nil {
			return err
		}
		return opts.RunCommand(ctx, "brew", []string{"upgrade", "--cask", "aitok"}, opts.Err)
	case InstallGo:
		return opts.RunCommand(ctx, "go", []string{"install", "github.com/MagnumGoYB/aitok/cmd/aitok@latest"}, opts.Err)
	default:
		return nil
	}
}

func runCommand(ctx context.Context, name string, args []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func compareVersion(a, b string) int {
	left := versionCore(a)
	right := versionCore(b)
	for i := 0; i < 3; i++ {
		if left[i] > right[i] {
			return 1
		}
		if left[i] < right[i] {
			return -1
		}
	}
	return 0
}

func versionCore(version string) [3]int {
	base := strings.FieldsFunc(version, func(r rune) bool { return r == '-' || r == '+' })[0]
	parts := strings.Split(base, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		value, _ := strconv.Atoi(parts[i])
		out[i] = value
	}
	return out
}
