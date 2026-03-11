package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/scanner"
)

func TestAppsModuleScanUsesOrphanFinderRiskMetadata(t *testing.T) {
	homeDir := t.TempDir()
	module := newAppsModule(homeDir)

	prefPath := createAppsModuleFile(t, homeDir, filepath.Join("Library", "Preferences", "com.acme.orphan-tool.plist"), 64)
	groupContainerPath := createAppsModuleDirWithFile(t, homeDir, filepath.Join("Library", "Group Containers", "ABCD1234.com.acme.orphan-tool"), "shared.db", 128)

	entries, err := module.Scan(context.Background())
	require.NoError(t, err)

	byPath := make(map[string]scanner.Entry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	require.Contains(t, byPath, prefPath)
	require.Contains(t, byPath, groupContainerPath)

	assert.Equal(t, scanner.RiskCaution, byPath[prefPath].Risk)
	assert.Equal(t, "com.acme.orphan-tool", byPath[prefPath].BundleID)
	assert.Equal(t, scanner.RiskDangerous, byPath[groupContainerPath].Risk)
	assert.Equal(t, "com.acme.orphan-tool", byPath[groupContainerPath].BundleID)
}

func TestAppsModuleScanContextCanceled(t *testing.T) {
	homeDir := t.TempDir()
	module := newAppsModule(homeDir)
	createAppsModuleFile(t, homeDir, filepath.Join("Library", "Preferences", "com.acme.orphan-tool.plist"), 64)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := module.Scan(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func createAppsModuleFile(t *testing.T, homeDir, relPath string, size int64) string {
	t.Helper()

	path := filepath.Join(homeDir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(size))
	require.NoError(t, file.Close())

	return path
}

func createAppsModuleDirWithFile(t *testing.T, homeDir, relPath, fileName string, size int64) string {
	t.Helper()

	dir := filepath.Join(homeDir, relPath)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	createAppsModuleFile(t, homeDir, filepath.Join(relPath, fileName), size)
	return dir
}
