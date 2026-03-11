package scanner_test

import (
	"context"
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

			for _, exp := range tt.expected {
				found := false
				for _, term := range terms {
					if term == exp || strings.EqualFold(term, exp) {
						found = true
						break
					}
				}
				assert.True(t, found, "expected term %q or its lowercase to be present", exp)
			}
		})
	}
}

func TestOrphanFinder_ExtractAppInfo(t *testing.T) {
	tmpDir := t.TempDir()
	appPath := writeAppBundle(t, filepath.Join(tmpDir, "Applications"), "Test.app", "com.test.testapp", "TestApp", "Test Application")

	finder, err := scanner.NewOrphanFinderWithConfig(&scanner.PathConfig{
		HomeDir: tmpDir,
		LibDir:  filepath.Join(tmpDir, "Library"),
	}, []string{filepath.Join(tmpDir, "Applications")})
	require.NoError(t, err)

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

func TestOrphanFinder_FindLeftoversContext_AssignsConfidenceLevels(t *testing.T) {
	home := t.TempDir()
	libDir := createLibraryFixtures(t, home)

	writeSizedFile(t, filepath.Join(libDir, "Preferences", "com.example.app.plist"), 128)
	writeSizedFile(t, filepath.Join(libDir, "Application Support", "Example App", "cache.db"), 256)
	writeSizedFile(t, filepath.Join(libDir, "Group Containers", "TEAMID.example.shared", "shared.db"), 512)

	finder, err := scanner.NewOrphanFinderWithConfig(&scanner.PathConfig{
		HomeDir: home,
		LibDir:  libDir,
	}, []string{filepath.Join(home, "Applications")})
	require.NoError(t, err)

	appInfo := &scanner.AppInfo{
		BundleID:    "com.example.app",
		Name:        "Example App",
		DisplayName: "Example App",
		Company:     "example",
	}

	leftovers, err := finder.FindLeftoversContext(context.Background(), appInfo)
	require.NoError(t, err)

	confidenceByPath := make(map[string]scanner.ConfidenceLevel, len(leftovers))
	for _, leftover := range leftovers {
		confidenceByPath[leftover.Path] = leftover.Confidence
	}

	assert.Equal(t, scanner.ConfidenceHigh, confidenceByPath[filepath.Join(libDir, "Preferences", "com.example.app.plist")])
	assert.Equal(t, scanner.ConfidenceMedium, confidenceByPath[filepath.Join(libDir, "Application Support", "Example App")])
	assert.Equal(t, scanner.ConfidenceLow, confidenceByPath[filepath.Join(libDir, "Group Containers", "TEAMID.example.shared")])
}

func TestOrphanFinder_FindAllOrphansContext_ExcludesInstalledAppsAndExpandsSharedData(t *testing.T) {
	home := t.TempDir()
	libDir := createLibraryFixtures(t, home)

	writeAppBundle(t, filepath.Join(home, "Applications", "Vendor"), "Installed.app", "com.example.installed", "Installed", "Installed")
	writeSizedFile(t, filepath.Join(libDir, "Preferences", "com.example.installed.plist"), 64)
	writeSizedFile(t, filepath.Join(libDir, "Preferences", "com.example.orphan.plist"), 128)
	writeSizedFile(t, filepath.Join(libDir, "Application Support", "orphan", "state.db"), 256)
	writeSizedFile(t, filepath.Join(libDir, "Group Containers", "TEAMID.example.shared", "shared.db"), 512)
	writeSizedFile(t, filepath.Join(libDir, "Application Support", "Unrelated", "data.db"), 128)

	finder, err := scanner.NewOrphanFinderWithConfig(&scanner.PathConfig{
		HomeDir: home,
		LibDir:  libDir,
	}, []string{filepath.Join(home, "Applications")})
	require.NoError(t, err)

	leftovers, err := finder.FindAllOrphansContext(context.Background())
	require.NoError(t, err)

	paths := make([]string, 0, len(leftovers))
	confidenceByPath := make(map[string]scanner.ConfidenceLevel, len(leftovers))
	for _, leftover := range leftovers {
		paths = append(paths, leftover.Path)
		confidenceByPath[leftover.Path] = leftover.Confidence
	}

	assert.NotContains(t, paths, filepath.Join(libDir, "Preferences", "com.example.installed.plist"))
	assert.Contains(t, paths, filepath.Join(libDir, "Preferences", "com.example.orphan.plist"))
	assert.Contains(t, paths, filepath.Join(libDir, "Application Support", "orphan"))
	assert.Contains(t, paths, filepath.Join(libDir, "Group Containers", "TEAMID.example.shared"))
	assert.NotContains(t, paths, filepath.Join(libDir, "Application Support", "Unrelated"))
	assert.Equal(t, scanner.ConfidenceHigh, confidenceByPath[filepath.Join(libDir, "Preferences", "com.example.orphan.plist")])
	assert.Equal(t, scanner.ConfidenceMedium, confidenceByPath[filepath.Join(libDir, "Application Support", "orphan")])
	assert.Equal(t, scanner.ConfidenceLow, confidenceByPath[filepath.Join(libDir, "Group Containers", "TEAMID.example.shared")])
}

func TestOrphanFinder_FindAllOrphansContext_Canceled(t *testing.T) {
	home := t.TempDir()
	libDir := createLibraryFixtures(t, home)
	writeSizedFile(t, filepath.Join(libDir, "Preferences", "com.example.orphan.plist"), 64)

	finder, err := scanner.NewOrphanFinderWithConfig(&scanner.PathConfig{
		HomeDir: home,
		LibDir:  libDir,
	}, []string{filepath.Join(home, "Applications")})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	leftovers, err := finder.FindAllOrphansContext(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, leftovers)
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

func createLibraryFixtures(t *testing.T, home string) string {
	t.Helper()

	libDir := filepath.Join(home, "Library")
	for _, location := range []string{
		"Application Support",
		"Preferences",
		"Caches",
		"Containers",
		"Group Containers",
		"Saved Application State",
		"WebKit",
		"HTTPStorages",
		"Cookies",
		"LaunchAgents",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(libDir, location), 0755))
	}

	return libDir
}

func writeAppBundle(t *testing.T, root, appName, bundleID, bundleName, displayName string) string {
	t.Helper()

	appPath := filepath.Join(root, appName)
	contentsDir := filepath.Join(appPath, "Contents")
	require.NoError(t, os.MkdirAll(contentsDir, 0755))

	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>` + bundleID + `</string>
	<key>CFBundleName</key>
	<string>` + bundleName + `</string>
	<key>CFBundleDisplayName</key>
	<string>` + displayName + `</string>
</dict>
</plist>`

	require.NoError(t, os.WriteFile(filepath.Join(contentsDir, "Info.plist"), []byte(plistContent), 0644))
	return appPath
}

func writeSizedFile(t *testing.T, path string, size int64) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))

	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	require.NoError(t, f.Truncate(size))
}
