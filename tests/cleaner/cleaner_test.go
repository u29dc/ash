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

func TestCleaner_New(t *testing.T) {
	tests := []struct {
		name     string
		opts     []cleaner.Option
		wantDry  bool
		wantTrash bool
	}{
		{
			name:      "default options",
			opts:      nil,
			wantDry:   false,
			wantTrash: true,
		},
		{
			name:      "dry run enabled",
			opts:      []cleaner.Option{cleaner.WithDryRun(true)},
			wantDry:   true,
			wantTrash: true,
		},
		{
			name:      "trash disabled",
			opts:      []cleaner.Option{cleaner.WithTrash(false)},
			wantDry:   false,
			wantTrash: false,
		},
		{
			name: "multiple options",
			opts: []cleaner.Option{
				cleaner.WithDryRun(true),
				cleaner.WithTrash(false),
				cleaner.WithParallelism(8),
			},
			wantDry:   true,
			wantTrash: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cleaner.New(tt.opts...)
			assert.Equal(t, tt.wantDry, c.IsDryRun())
			assert.Equal(t, tt.wantTrash, c.UsesTrash())
		})
	}
}

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
	assert.Equal(t, 1, stats.SuccessCount) // Only /tmp/safe.txt
	assert.Equal(t, 2, stats.FailedCount)  // .ssh and /System
	assert.Len(t, stats.Errors, 2)
}

func TestCleaner_Clean_DryRun(t *testing.T) {
	// Create a temporary file
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	entries := []scanner.Entry{
		{Path: testFile, Size: 4},
	}

	// Clean with dry run
	c := cleaner.New(cleaner.WithDryRun(true))
	ctx := context.Background()
	stats, err := c.Clean(ctx, entries)

	require.NoError(t, err)
	assert.Equal(t, 1, stats.SuccessCount)

	// File should still exist
	_, err = os.Stat(testFile)
	assert.NoError(t, err, "File should still exist after dry run")
}

func TestCleaner_Clean_EmptyEntries(t *testing.T) {
	c := cleaner.New()
	ctx := context.Background()
	stats, err := c.Clean(ctx, []scanner.Entry{})

	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalCount)
	assert.Equal(t, 0, stats.SuccessCount)
}

func TestCleaner_CleanSingle_ProtectedPath(t *testing.T) {
	c := cleaner.New()
	ctx := context.Background()

	entry := scanner.Entry{
		Path: "~/.ssh/id_rsa",
		Size: 100,
	}

	_, err := c.CleanSingle(ctx, entry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "protected path")
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
