package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadVersionAcceptsProjectSemver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	version, err := readVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" {
		t.Fatalf("version = %q", version)
	}
}

func TestReadVersionRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "v0.1.0", "next"} {
		path := filepath.Join(t.TempDir(), "VERSION")
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readVersion(path); err == nil {
			t.Fatalf("expected %q to fail", value)
		}
	}
}

func TestCheckRefNameRequiresTagToMatchVersion(t *testing.T) {
	if err := checkRefName("0.1.0", "v0.1.0"); err != nil {
		t.Fatal(err)
	}
	err := checkRefName("0.1.0", "v0.2.0")
	if err == nil {
		t.Fatal("expected mismatched tag to fail")
	}
	if !strings.Contains(err.Error(), "must match VERSION") {
		t.Fatalf("unexpected error: %v", err)
	}
}
