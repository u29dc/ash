package plist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/pkg/plist"
)

func TestReadAppInfo(t *testing.T) {
	// Create a test plist file
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "Info.plist")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.testapp</string>
	<key>CFBundleName</key>
	<string>TestApp</string>
	<key>CFBundleDisplayName</key>
	<string>Test Application</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0.0</string>
	<key>CFBundleVersion</key>
	<string>100</string>
	<key>CFBundleExecutable</key>
	<string>TestApp</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
	<key>LSMinimumSystemVersion</key>
	<string>10.15</string>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(content), 0644)
	require.NoError(t, err)

	info, err := plist.ReadAppInfo(plistPath)
	require.NoError(t, err)

	assert.Equal(t, "com.example.testapp", info.BundleIdentifier)
	assert.Equal(t, "TestApp", info.BundleName)
	assert.Equal(t, "Test Application", info.DisplayName)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "100", info.BuildNumber)
	assert.Equal(t, "TestApp", info.Executable)
	assert.Equal(t, "AppIcon", info.IconFile)
	assert.Equal(t, "10.15", info.MinOSVersion)
}

func TestReadAppInfo_NotFound(t *testing.T) {
	_, err := plist.ReadAppInfo("/nonexistent/Info.plist")
	assert.Error(t, err)
}

func TestReadAppInfo_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "Info.plist")

	// Write invalid plist
	err := os.WriteFile(plistPath, []byte("not a plist"), 0644)
	require.NoError(t, err)

	_, err = plist.ReadAppInfo(plistPath)
	assert.Error(t, err)
}

func TestGetBundleID(t *testing.T) {
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "Info.plist")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.bundleid</string>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(content), 0644)
	require.NoError(t, err)

	bundleID, err := plist.GetBundleID(plistPath)
	require.NoError(t, err)
	assert.Equal(t, "com.example.bundleid", bundleID)
}

func TestGetAppName(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "display name present",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>BundleName</string>
	<key>CFBundleDisplayName</key>
	<string>Display Name</string>
</dict>
</plist>`,
			expected: "Display Name",
		},
		{
			name: "no display name",
			content: `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>BundleName</string>
</dict>
</plist>`,
			expected: "BundleName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plistPath := filepath.Join(tmpDir, tt.name+".plist")
			err := os.WriteFile(plistPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			name, err := plist.GetAppName(plistPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, name)
		})
	}
}

func TestReadPlist(t *testing.T) {
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "test.plist")

	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Name</key>
	<string>Test</string>
	<key>Count</key>
	<integer>42</integer>
</dict>
</plist>`

	err := os.WriteFile(plistPath, []byte(content), 0644)
	require.NoError(t, err)

	var data struct {
		Name  string `plist:"Name"`
		Count int    `plist:"Count"`
	}

	err = plist.ReadPlist(plistPath, &data)
	require.NoError(t, err)
	assert.Equal(t, "Test", data.Name)
	assert.Equal(t, 42, data.Count)
}

func TestWritePlist(t *testing.T) {
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "output.plist")

	data := struct {
		Name    string `plist:"Name"`
		Version string `plist:"Version"`
	}{
		Name:    "Test",
		Version: "1.0",
	}

	err := plist.WritePlist(plistPath, &data)
	require.NoError(t, err)

	// Read it back
	var readBack struct {
		Name    string `plist:"Name"`
		Version string `plist:"Version"`
	}

	err = plist.ReadPlist(plistPath, &readBack)
	require.NoError(t, err)
	assert.Equal(t, data.Name, readBack.Name)
	assert.Equal(t, data.Version, readBack.Version)
}
