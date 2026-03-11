package cleaner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/cleaner"
	"ash/internal/scanner"
)

func TestCleaner_Preview(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/tmp/safe1.txt", Size: 100},
		{Path: "/tmp/safe2.txt", Size: 200},
		{Path: "/tmp/safe3.txt", Size: 300},
	}

	c := cleaner.New()
	stats, err := c.Preview(entries)

	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalCount)
	assert.Equal(t, 3, stats.SuccessCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, int64(600), stats.TotalSize)
	assert.Equal(t, int64(600), stats.CleanedSize)
}

func TestCleaner_Preview_ProtectedPaths(t *testing.T) {
	entries := []scanner.Entry{
		{Path: "/tmp/safe.txt", Size: 100},
		{Path: "~/.ssh/id_rsa", Size: 200},
		{Path: "/System/Library/file", Size: 300},
	}

	c := cleaner.New()
	stats, err := c.Preview(entries)

	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalCount)
	assert.Equal(t, 1, stats.SuccessCount)
	assert.Equal(t, 2, stats.FailedCount)
	assert.Len(t, stats.Errors, 2)
}

func TestCleaner_Clean_DryRun(t *testing.T) {
	testFile := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0o644))

	c := cleaner.New(cleaner.WithDryRun(true))
	stats, err := c.Clean(context.Background(), []scanner.Entry{
		{Path: testFile, Size: 4},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, stats.SuccessCount)

	_, statErr := os.Stat(testFile)
	assert.NoError(t, statErr)
}

func TestCleaner_Clean_EmptyEntries(t *testing.T) {
	c := cleaner.New()
	stats, err := c.Clean(context.Background(), []scanner.Entry{})

	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalCount)
	assert.Equal(t, 0, stats.SuccessCount)
}

func TestCleaner_CleanSingle_ProtectedPath(t *testing.T) {
	c := cleaner.New()

	_, err := c.CleanSingle(context.Background(), scanner.Entry{
		Path: "~/.ssh/id_rsa",
		Size: 100,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "protected path")
}

func TestCleaner_CleanSingle_RejectsSymlinkToProtectedPath(t *testing.T) {
	tempDir := t.TempDir()
	linkPath := filepath.Join(tempDir, "applications-link")
	require.NoError(t, os.Symlink("/Applications", linkPath))

	c := cleaner.New()
	result, err := c.CleanSingle(context.Background(), scanner.Entry{
		Path: linkPath,
		Size: 1,
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "protected path")

	info, statErr := os.Lstat(linkPath)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Mode()&os.ModeSymlink)
}

func TestCleaner_Clean_RejectsUnsafeBatchBeforeAnyMove(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	safeFile := filepath.Join(t.TempDir(), "safe.log")
	require.NoError(t, os.WriteFile(safeFile, []byte("log"), 0o644))

	linkPath := filepath.Join(t.TempDir(), "applications-link")
	require.NoError(t, os.Symlink("/Applications", linkPath))

	c := cleaner.New()
	stats, err := c.Clean(context.Background(), []scanner.Entry{
		{Path: safeFile, Size: 3},
		{Path: linkPath, Size: 1},
	})

	require.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "protected path")

	_, statErr := os.Stat(safeFile)
	require.NoError(t, statErr)

	_, trashErr := os.Stat(filepath.Join(homeDir, ".Trash"))
	assert.ErrorIs(t, trashErr, os.ErrNotExist)
}

func TestCleanStats_Empty(t *testing.T) {
	stats := &cleaner.CleanStats{}

	assert.Equal(t, 0, stats.TotalCount)
	assert.Equal(t, 0, stats.SuccessCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, int64(0), stats.TotalSize)
	assert.Equal(t, int64(0), stats.CleanedSize)
	assert.Empty(t, stats.Errors)
}
