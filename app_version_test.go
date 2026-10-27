package main

import (
	"strings"
	"testing"
)

func TestAppVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		commit   string
		expected string
	}{
		{
			name:     "Combined version and commit (twSecScan style)",
			version:  "v1.2.3(abc1234)",
			commit:   "ignored",
			expected: "v1.2.3(abc1234)",
		},
		{
			name:     "Separate version and commit (twsnmpneo style)",
			version:  "v1.2.3",
			commit:   "abc1234",
			expected: "v1.2.3(abc1234)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp(tc.version, tc.commit)
			got := app.GetVersion()
			if got != tc.expected {
				t.Errorf("GetVersion() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestResolveAppVersionFallback(t *testing.T) {
	// When version is empty or default and commit is unknown (like running `wails dev` directly),
	// it should fall back to git/buildinfo and resolve commit.
	app := NewApp("v0.1.0", "unknown")
	got := app.GetVersion()

	// It should look like "v0.1.0(d3f8249)" or similar containing parentheses
	if !strings.Contains(got, "(") || !strings.Contains(got, ")") {
		t.Errorf("Expected fallback commit resolution in git repository, got %q", got)
	}
}
