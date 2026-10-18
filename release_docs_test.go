package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The README and docs describe main; these checks keep the pointers to the latest release
// consistent between releases. scripts/check-release-docs.sh does the stricter check on a tag.

func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// latestRelease returns the first version section after "## Unreleased" in CHANGELOG.md.
func latestRelease(t *testing.T) string {
	t.Helper()
	var headings []string
	for _, line := range strings.Split(readFile(t, "CHANGELOG.md"), "\n") {
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			headings = append(headings, heading)
		}
	}
	if len(headings) < 2 || headings[0] != "Unreleased" {
		t.Fatalf("CHANGELOG.md must start with \"## Unreleased\" followed by the latest release, got %q", headings)
	}
	version, _, _ := strings.Cut(headings[1], " ")
	return version
}

func TestReadmeNotePointsToLatestRelease(t *testing.T) {
	version := latestRelease(t)
	head := strings.Join(strings.SplitN(readFile(t, "README.md"), "\n", 6)[:5], "\n")
	for _, want := range []string{"**" + version + "**", "/blob/" + version + "/README.md", "(CHANGELOG.md)"} {
		if !strings.Contains(head, want) {
			t.Errorf("the note at the top of README.md must contain %q (latest release in CHANGELOG.md is %s)", want, version)
		}
	}
}

func TestDocsHaveDevelopmentNote(t *testing.T) {
	for _, name := range []string{"docs/environment_variables.md", "docs/installation.md"} {
		if !strings.Contains(readFile(t, name), "(../CHANGELOG.md)") {
			t.Errorf("%s needs the note that it describes main, with a link to the CHANGELOG", name)
		}
	}
}

// Markers must be a line of their own, so the release check finds them and heading anchors
// (like docs/environment_variables.md#trustedorigins) stay stable across releases.
func TestReleaseMarkersAreOwnLines(t *testing.T) {
	marker := regexp.MustCompile(`(?i)\*\((unreleased|since [0-9.]+)\)\*`)
	for _, name := range []string{"README.md", "docs/environment_variables.md", "docs/installation.md"} {
		for i, line := range strings.Split(readFile(t, name), "\n") {
			if strings.HasPrefix(line, "> **Note:**") || !marker.MatchString(line) {
				continue
			}
			if trimmed := strings.TrimSpace(line); trimmed != "*(unreleased)*" && !regexp.MustCompile(`^\*\(since [0-9.]+\)\*$`).MatchString(trimmed) {
				t.Errorf("%s:%d: put the marker on a line of its own, below the heading: %q", name, i+1, line)
			}
		}
	}
}
