package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

func main() {
	checkRef := flag.Bool("check-ref", false, "require GITHUB_REF_NAME to match VERSION as v<version>")
	flag.Parse()

	version, err := readVersion("VERSION")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *checkRef {
		if err := checkRefName(version, os.Getenv("GITHUB_REF_NAME")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Println(version)
}

func readVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", errors.New("VERSION is empty")
	}
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("VERSION %q must be semver without leading v", version)
	}
	return version, nil
}

func checkRefName(version, refName string) error {
	if refName == "" {
		return errors.New("GITHUB_REF_NAME is empty")
	}
	expected := "v" + version
	if refName != expected {
		return fmt.Errorf("tag %q must match VERSION as %q", refName, expected)
	}
	return nil
}
