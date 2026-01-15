package scanner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/scanner"
)

func TestOrphanFinder_GenerateSearchTerms(t *testing.T) {
	finder, err := scanner.NewOrphanFinder()
	require.NoError(t, err)

	tests := []struct {
		name     string
		appInfo  *scanner.AppInfo
		expected []string
	}{
		{
			name: "full app info",
			appInfo: &scanner.AppInfo{
				BundleID:    "com.example.app",
				Name:        "Example",
				DisplayName: "Example App",
				Company:     "example",
			},
			expected: []string{"com.example.app", "Example", "example", "Example App"},
		},
		{
			name: "minimal app info",
			appInfo: &scanner.AppInfo{
				BundleID: "com.test.minimal",
				Name:     "Minimal",
			},
			expected: []string{"com.test.minimal", "Minimal", "minimal"},
		},
		{
			name: "no display name",
			appInfo: &scanner.AppInfo{
				BundleID: "com.test.nodisplay",
				Name:     "NoDisplay",
				Company:  "test",
			},
			expected: []string{"com.test.nodisplay", "NoDisplay", "nodisplay", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms := finder.GenerateSearchTerms(tt.appInfo)
			assert.NotEmpty(t, terms)

			// Check that key terms are present
			for _, exp := range tt.expected {
				found := false
				for _, term := range terms {
					if term == exp {
						found = true
						break
					}
				}
				if !found {
					// Check lowercase version
					expLower := strings.ToLower(exp)
					for _, term := range terms {
						if term == expLower {
							found = true
							break
						}
					}
				}
				assert.True(t, found, "Expected term %q or its lowercase to be present", exp)
			}
		})
	}
}

func TestOrphanFinder_ExtractAppInfo(t *testing.T) {
	// Create a mock app bundle
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Test.app", "Contents")
	err := os.MkdirAll(appDir, 0755)
	require.NoError(t, err)

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.test.testapp</string>
	<key>CFBundleName</key>
	<string>TestApp</string>
	<key>CFBundleDisplayName</key>
	<string>Test Application</string>
</dict>
</plist>`

	err = os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(plistContent), 0644)
	require.NoError(t, err)

	finder, err := scanner.NewOrphanFinder()
	require.NoError(t, err)

	appPath := filepath.Join(tmpDir, "Test.app")
	info, err := finder.ExtractAppInfo(appPath)
	require.NoError(t, err)

	assert.Equal(t, "com.test.testapp", info.BundleID)
	assert.Equal(t, "TestApp", info.Name)
	assert.Equal(t, "Test Application", info.DisplayName)
	assert.Equal(t, "test", info.Company)
}

func TestOrphanFinder_ExtractAppInfo_NotFound(t *testing.T) {
	finder, err := scanner.NewOrphanFinder()
	require.NoError(t, err)

	_, err = finder.ExtractAppInfo("/nonexistent/app.app")
	assert.Error(t, err)
}

func TestConfidenceLevel(t *testing.T) {
	assert.True(t, scanner.ConfidenceHigh < scanner.ConfidenceMedium)
	assert.True(t, scanner.ConfidenceMedium < scanner.ConfidenceLow)
}

func TestLeftover_Properties(t *testing.T) {
	leftover := scanner.Leftover{
		Path: "/Library/Caches/com.test.app",
		Size: 1024,
		AppInfo: &scanner.AppInfo{
			BundleID: "com.test.app",
			Name:     "Test",
		},
		Confidence: scanner.ConfidenceHigh,
		Location:   "Caches",
	}

	assert.Equal(t, "/Library/Caches/com.test.app", leftover.Path)
	assert.Equal(t, int64(1024), leftover.Size)
	assert.Equal(t, scanner.ConfidenceHigh, leftover.Confidence)
	assert.Equal(t, "Caches", leftover.Location)
	assert.NotNil(t, leftover.AppInfo)
}
