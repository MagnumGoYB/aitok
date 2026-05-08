package buildinfo

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesRepositoryVersionFile(t *testing.T) {
	data, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Version, strings.TrimSpace(string(data)); got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}
}
