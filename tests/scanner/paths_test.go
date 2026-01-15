package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/scanner"
)

func TestPathConfig_New(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	assert.NotEmpty(t, cfg.HomeDir)
	assert.NotEmpty(t, cfg.LibDir)
	assert.True(t, filepath.IsAbs(cfg.HomeDir))
	assert.True(t, filepath.IsAbs(cfg.LibDir))
}

func TestPathConfig_CachePaths(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.CachePaths()
	assert.NotEmpty(t, paths)

	for _, p := range paths {
		assert.True(t, filepath.IsAbs(p))
		assert.Contains(t, p, "Caches")
	}
}

func TestPathConfig_LogPaths(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.LogPaths()
	assert.NotEmpty(t, paths)

	for _, p := range paths {
		assert.True(t, filepath.IsAbs(p))
		assert.Contains(t, p, "Logs")
	}
}

func TestPathConfig_XcodePaths(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.XcodePaths()
	assert.NotEmpty(t, paths)

	// Should include DerivedData
	hasDerivedData := false
	for _, p := range paths {
		if filepath.Base(p) == "DerivedData" {
			hasDerivedData = true
			break
		}
	}
	assert.True(t, hasDerivedData)
}

func TestPathConfig_HomebrewPaths(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.HomebrewPaths()
	assert.NotEmpty(t, paths)

	for _, p := range paths {
		assert.Contains(t, p, "Homebrew")
	}
}

func TestPathConfig_BrowserPaths(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.BrowserPaths()
	assert.NotEmpty(t, paths)
}

func TestPathConfig_AppLeftoverLocations(t *testing.T) {
	cfg, err := scanner.NewPathConfig()
	require.NoError(t, err)

	paths := cfg.AppLeftoverLocations()
	assert.NotEmpty(t, paths)

	// Should include common leftover locations
	expected := []string{
		"Application Support",
		"Preferences",
		"Caches",
		"Containers",
	}

	for _, exp := range expected {
		found := false
		for _, p := range paths {
			if filepath.Base(p) == exp {
				found = true
				break
			}
		}
		assert.True(t, found, "Should include %s", exp)
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "tilde path",
			input:  "~/Documents",
			expect: filepath.Join(home, "Documents"),
		},
		{
			name:   "absolute path",
			input:  "/usr/local/bin",
			expect: "/usr/local/bin",
		},
		{
			name:   "relative path",
			input:  "relative/path",
			expect: "relative/path",
		},
		{
			name:   "tilde only",
			input:  "~",
			expect: "~", // Doesn't expand bare tilde
		},
		{
			name:   "nested tilde path",
			input:  "~/Library/Caches/test",
			expect: filepath.Join(home, "Library/Caches/test"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.ExpandPath(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name   string
		parent string
		child  string
		want   bool
	}{
		{"same path", "/foo/bar", "/foo/bar", true},
		{"direct child", "/foo", "/foo/bar", true},
		{"nested child", "/foo", "/foo/bar/baz/qux", true},
		{"not child", "/foo", "/bar", false},
		{"sibling", "/foo/bar", "/foo/baz", false},
		{"parent of", "/foo/bar", "/foo", false},
		{"partial name match", "/foo/bar", "/foo/barbaz", false},
		{"empty parent", "", "/foo", false},
		{"root parent", "/", "/foo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.IsSubPath(tt.parent, tt.child)
			assert.Equal(t, tt.want, got, "IsSubPath(%q, %q)", tt.parent, tt.child)
		})
	}
}
