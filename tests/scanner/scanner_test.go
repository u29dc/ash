package scanner_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/scanner"
	"ash/tests/testutil"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 500, "500 B"},
		{"kilobytes", 1024, "1.0 KiB"},
		{"megabytes", 1024 * 1024, "1.0 MiB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GiB"},
		{"mixed", 1536, "1.5 KiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.FormatSize(tt.size)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCategoryName(t *testing.T) {
	tests := []struct {
		category scanner.Category
		want     string
	}{
		{scanner.CategoryCaches, "Caches"},
		{scanner.CategoryLogs, "Logs"},
		{scanner.CategoryXcode, "Xcode"},
		{scanner.CategoryHomebrew, "Homebrew"},
		{scanner.CategoryBrowsers, "Browsers"},
		{scanner.CategoryAppData, "App Data"},
		{scanner.CategoryOther, "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := scanner.CategoryName(tt.category)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		risk scanner.RiskLevel
		want string
	}{
		{scanner.RiskSafe, "safe"},
		{scanner.RiskCaution, "caution"},
		{scanner.RiskDangerous, "dangerous"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.risk.String())
		})
	}
}

func TestAnalyzer_SortBySize(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Size: 100},
		{Path: "/b", Size: 500},
		{Path: "/c", Size: 200},
	}

	analyzer := scanner.NewAnalyzer()
	sorted := analyzer.SortBySize(entries)

	require.Len(t, sorted, 3)
	assert.Equal(t, int64(500), sorted[0].Size)
	assert.Equal(t, int64(200), sorted[1].Size)
	assert.Equal(t, int64(100), sorted[2].Size)
}

func TestAnalyzer_SortByName(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/c", Name: "zebra"},
		{Path: "/a", Name: "apple"},
		{Path: "/b", Name: "banana"},
	}

	analyzer := scanner.NewAnalyzer()
	sorted := analyzer.SortByName(entries)

	require.Len(t, sorted, 3)
	assert.Equal(t, "apple", sorted[0].Name)
	assert.Equal(t, "banana", sorted[1].Name)
	assert.Equal(t, "zebra", sorted[2].Name)
}

func TestAnalyzer_FilterByCategory(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Category: scanner.CategoryCaches},
		{Path: "/b", Category: scanner.CategoryLogs},
		{Path: "/c", Category: scanner.CategoryCaches},
	}

	analyzer := scanner.NewAnalyzer()
	filtered := analyzer.FilterByCategory(entries, scanner.CategoryCaches)

	require.Len(t, filtered, 2)
	assert.Equal(t, "/a", filtered[0].Path)
	assert.Equal(t, "/c", filtered[1].Path)
}

func TestAnalyzer_FilterByRisk(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Risk: scanner.RiskSafe},
		{Path: "/b", Risk: scanner.RiskCaution},
		{Path: "/c", Risk: scanner.RiskSafe},
	}

	analyzer := scanner.NewAnalyzer()
	filtered := analyzer.FilterByRisk(entries, scanner.RiskSafe)

	require.Len(t, filtered, 2)
	assert.Equal(t, "/a", filtered[0].Path)
	assert.Equal(t, "/c", filtered[1].Path)
}

func TestAnalyzer_FilterSelected(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Selected: true},
		{Path: "/b", Selected: false},
		{Path: "/c", Selected: true},
	}

	analyzer := scanner.NewAnalyzer()
	filtered := analyzer.FilterSelected(entries)

	require.Len(t, filtered, 2)
	assert.Equal(t, "/a", filtered[0].Path)
	assert.Equal(t, "/c", filtered[1].Path)
}

func TestAnalyzer_Analyze(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Size: 100, Category: scanner.CategoryCaches, Selected: true},
		{Path: "/b", Size: 200, Category: scanner.CategoryCaches, Selected: false},
		{Path: "/c", Size: 300, Category: scanner.CategoryLogs, Selected: true},
	}

	analyzer := scanner.NewAnalyzer()
	analysis := analyzer.Analyze(entries)

	assert.Equal(t, int64(600), analysis.TotalSize)
	assert.Equal(t, 3, analysis.TotalCount)
	assert.Equal(t, int64(400), analysis.SelectedSize)
	assert.Equal(t, 2, analysis.SelectedCount)

	// Check category summaries
	cachesSummary := analysis.ByCategory[scanner.CategoryCaches]
	assert.Equal(t, int64(300), cachesSummary.TotalSize)
	assert.Equal(t, 2, cachesSummary.TotalCount)

	logsSummary := analysis.ByCategory[scanner.CategoryLogs]
	assert.Equal(t, int64(300), logsSummary.TotalSize)
	assert.Equal(t, 1, logsSummary.TotalCount)
}

func TestAnalyzer_TopBySize(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/a", Size: 100},
		{Path: "/b", Size: 500},
		{Path: "/c", Size: 200},
		{Path: "/d", Size: 300},
		{Path: "/e", Size: 400},
	}

	analyzer := scanner.NewAnalyzer()

	// Get top 3
	top := analyzer.TopBySize(entries, 3)
	require.Len(t, top, 3)
	assert.Equal(t, int64(500), top[0].Size)
	assert.Equal(t, int64(400), top[1].Size)
	assert.Equal(t, int64(300), top[2].Size)

	// Request more than available
	all := analyzer.TopBySize(entries, 10)
	require.Len(t, all, 5)
}

func TestPathConfig_ExpandPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, result string)
	}{
		{
			name:  "expands tilde",
			input: "~/Documents",
			check: func(t *testing.T, result string) {
				assert.NotContains(t, result, "~")
				assert.Contains(t, result, "Documents")
			},
		},
		{
			name:  "preserves absolute path",
			input: "/usr/local/bin",
			check: func(t *testing.T, result string) {
				assert.Equal(t, "/usr/local/bin", result)
			},
		},
		{
			name:  "preserves relative path",
			input: "relative/path",
			check: func(t *testing.T, result string) {
				assert.Equal(t, "relative/path", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.ExpandPath(tt.input)
			tt.check(t, result)
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
		{"nested child", "/foo", "/foo/bar/baz", true},
		{"not child", "/foo", "/bar", false},
		{"sibling", "/foo/bar", "/foo/baz", false},
		{"parent of", "/foo/bar", "/foo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanner.IsSubPath(tt.parent, tt.child)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Integration test with temp directory
func TestScanner_CreateTempDir(t *testing.T) {
	dir := testutil.CreateTempCacheDir(t, 5, 1024)
	defer testutil.Cleanup(t, dir)

	// Verify directory was created with correct files
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
}
