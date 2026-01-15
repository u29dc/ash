package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/config"
	"ash/internal/scanner"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	assert.Equal(t, int64(0), cfg.MinSize)
	assert.False(t, cfg.IncludeHidden)
	assert.True(t, cfg.UseTrash)
	assert.False(t, cfg.DryRun)
	assert.True(t, cfg.ShowSizes)
	assert.Equal(t, "size", cfg.SortBy)
	assert.Equal(t, 4, cfg.Parallelism)
	assert.NotEmpty(t, cfg.Categories)
}

func TestConfig_SaveAndLoad(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "ash")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Override home dir for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create and save config
	cfg := config.DefaultConfig()
	cfg.DryRun = true
	cfg.MinSize = 1024
	cfg.SortBy = "name"

	err = cfg.Save()
	require.NoError(t, err)

	// Load config
	loaded, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, cfg.DryRun, loaded.DryRun)
	assert.Equal(t, cfg.MinSize, loaded.MinSize)
	assert.Equal(t, cfg.SortBy, loaded.SortBy)
}

func TestConfig_LoadNonExistent(t *testing.T) {
	// Create temp dir with no config
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Load should return defaults
	cfg, err := config.Load()
	require.NoError(t, err)

	// Should be default values
	defaults := config.DefaultConfig()
	assert.Equal(t, defaults.UseTrash, cfg.UseTrash)
	assert.Equal(t, defaults.DryRun, cfg.DryRun)
}

func TestDefaultCategories(t *testing.T) {
	cats := config.DefaultCategories()

	assert.Contains(t, cats, scanner.CategoryCaches)
	assert.Contains(t, cats, scanner.CategoryLogs)
	assert.Contains(t, cats, scanner.CategoryXcode)
	assert.Contains(t, cats, scanner.CategoryHomebrew)
	assert.Contains(t, cats, scanner.CategoryBrowsers)
}

func TestValidSortOrders(t *testing.T) {
	orders := config.ValidSortOrders()

	assert.Contains(t, orders, "size")
	assert.Contains(t, orders, "name")
	assert.Contains(t, orders, "date")
	assert.Len(t, orders, 3)
}

func TestSizeLimitLarge(t *testing.T) {
	limit := config.SizeLimitLarge()

	// Should be 1GB
	assert.Equal(t, int64(1024*1024*1024), limit)
}

func TestDefaultParallelism(t *testing.T) {
	p := config.DefaultParallelism()
	assert.Equal(t, 4, p)
}

func TestDefaultMinSize(t *testing.T) {
	s := config.DefaultMinSize()
	assert.Equal(t, int64(0), s)
}

func TestDefaultSortOrder(t *testing.T) {
	s := config.DefaultSortOrder()
	assert.Equal(t, "size", s)
}
