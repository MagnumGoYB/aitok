package updatecheck

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaybeRunPromptsHomebrewUpgradeCommand(t *testing.T) {
	home := t.TempDir()
	var errOut bytes.Buffer
	if err := MaybeRun(context.Background(), Options{
		Home:       home,
		Current:    "0.1.6",
		Endpoint:   "https://example.test/latest",
		Executable: "/opt/homebrew/Caskroom/aitok/0.1.6/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		In:         strings.NewReader(""),
		Err:        &errOut,
		HTTPClient: fakeHTTPClient(`{"tag_name":"v0.1.7","html_url":"https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.7"}`),
	}); err != nil {
		t.Fatal(err)
	}
	output := errOut.String()
	if !strings.Contains(output, "aitok 0.1.7 is available") || !strings.Contains(output, "brew update && brew upgrade --cask aitok") {
		t.Fatalf("unexpected prompt: %s", output)
	}
}

func TestMaybeRunUsesCacheToAvoidRepeatedNetworkChecks(t *testing.T) {
	home := t.TempDir()
	writeCacheForTest(t, home, `{"checked_at":"2026-05-08T11:00:00Z","latest":"0.1.7","url":"https://example.test"}`)
	var called bool
	if err := MaybeRun(context.Background(), Options{
		Home:       home,
		Current:    "0.1.6",
		Endpoint:   "https://example.test/latest",
		Executable: "/opt/homebrew/Caskroom/aitok/0.1.6/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		Err:        ioDiscard{},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			return response(`{"tag_name":"v0.1.7"}`), nil
		})},
	}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("version check should use fresh cache instead of calling the network")
	}
}

func TestMaybeRunExecutesGoUpgradeWhenInteractiveUserAccepts(t *testing.T) {
	home := t.TempDir()
	var calls []string
	if err := MaybeRun(context.Background(), Options{
		Home:       home,
		Current:    "0.1.6",
		Endpoint:   "https://example.test/latest",
		Executable: "/Users/sosbs/go/bin/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		In:         interactiveReader{Reader: strings.NewReader("y\n")},
		Err:        ioDiscard{},
		HTTPClient: fakeHTTPClient(`{"tag_name":"v0.1.7","html_url":"https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.7"}`),
		RunCommand: func(ctx context.Context, name string, args []string, out io.Writer) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(calls, "\n"), "go install github.com/MagnumGoYB/aitok/cmd/aitok@latest"; got != want {
		t.Fatalf("commands = %q, want %q", got, want)
	}
}

func TestRunUpdateIgnoresFreshCacheAndExecutesUpgrade(t *testing.T) {
	home := t.TempDir()
	writeCacheForTest(t, home, `{"checked_at":"2026-05-08T11:00:00Z","latest":"0.1.7","url":"https://example.test"}`)
	var calls []string
	var out bytes.Buffer
	if err := RunUpdate(context.Background(), Options{
		Home:       home,
		Current:    "0.1.6",
		Endpoint:   "https://example.test/latest",
		Executable: "/Users/sosbs/go/bin/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		Err:        &out,
		HTTPClient: fakeHTTPClient(`{"tag_name":"v0.1.7","html_url":"https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.7"}`),
		RunCommand: func(ctx context.Context, name string, args []string, out io.Writer) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(calls, "\n"), "go install github.com/MagnumGoYB/aitok/cmd/aitok@latest"; got != want {
		t.Fatalf("commands = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "Updating aitok from 0.1.6 to 0.1.7") {
		t.Fatalf("unexpected update output: %s", out.String())
	}
}

func TestRunUpdateReportsCurrentVersionWhenLatestIsNotNewer(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	if err := RunUpdate(context.Background(), Options{
		Home:       home,
		Current:    "0.1.7",
		Endpoint:   "https://example.test/latest",
		Executable: "/Users/sosbs/go/bin/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		Err:        &out,
		HTTPClient: fakeHTTPClient(`{"tag_name":"v0.1.7","html_url":"https://github.com/MagnumGoYB/aitok/releases/tag/v0.1.7"}`),
		RunCommand: func(ctx context.Context, name string, args []string, out io.Writer) error {
			t.Fatal("update command should not run when already current")
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "aitok 0.1.7 is already up to date") {
		t.Fatalf("unexpected update output: %s", out.String())
	}
}

func TestRunUpdateFallsBackToInstallUpgradeWhenLatestCheckIsForbidden(t *testing.T) {
	home := t.TempDir()
	var calls []string
	var out bytes.Buffer
	if err := RunUpdate(context.Background(), Options{
		Home:       home,
		Current:    "0.1.9",
		Endpoint:   "https://example.test/latest",
		Executable: "/opt/homebrew/Caskroom/aitok/0.1.9/aitok",
		Now:        func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) },
		Err:        &out,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"message":"rate limit"}`)),
			}, nil
		})},
		RunCommand: func(ctx context.Context, name string, args []string, out io.Writer) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(calls, "\n"), "brew update\nbrew upgrade --cask aitok"; got != want {
		t.Fatalf("commands = %q, want %q", got, want)
	}
	output := out.String()
	if !strings.Contains(output, "Could not check the latest GitHub Release: version check returned 403 Forbidden") ||
		!strings.Contains(output, "Trying local upgrade command: brew update && brew upgrade --cask aitok") {
		t.Fatalf("unexpected update output: %s", output)
	}
}

func TestDetectInstallMethod(t *testing.T) {
	cases := map[string]InstallMethod{
		"/opt/homebrew/Caskroom/aitok/0.1.6/aitok": InstallHomebrew,
		"/Users/sosbs/go/bin/aitok":                InstallGo,
		"/tmp/go-build123/b001/exe/aitok":          InstallDev,
		"/usr/local/bin/aitok":                     InstallRelease,
	}
	for path, want := range cases {
		if got := DetectInstallMethod(path); got != want {
			t.Fatalf("DetectInstallMethod(%q) = %q, want %q", path, got, want)
		}
	}
}

func writeCacheForTest(t *testing.T, home, content string) {
	t.Helper()
	path := filepath.Join(home, cacheFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

type interactiveReader struct {
	*strings.Reader
}

func (interactiveReader) Stat() (os.FileInfo, error) {
	return fakeFileInfo{mode: os.ModeCharDevice}, nil
}

type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "stdin" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fakeHTTPClient(body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return response(body), nil
	})}
}

func response(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
