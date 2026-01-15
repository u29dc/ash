package cleaner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ash/internal/cleaner"
)

// Note: These tests are limited as actual trash operations require macOS
// We test the functions that don't require actual Trash access

func TestTrashInfo_Properties(t *testing.T) {
	info := &cleaner.TrashInfo{
		ItemCount: 10,
		TotalSize: 1024 * 1024 * 100, // 100MB
		Path:      "/Users/test/.Trash",
	}

	assert.Equal(t, 10, info.ItemCount)
	assert.Equal(t, int64(1024*1024*100), info.TotalSize)
	assert.Equal(t, "/Users/test/.Trash", info.Path)
}

// GetTrashInfo, GetTrashSize, GetTrashItemCount are tested implicitly
// through integration tests on macOS since they access the actual Trash

func TestCleanResult_Properties(t *testing.T) {
	result := cleaner.CleanResult{
		Path:    "/tmp/test.txt",
		Success: true,
		Error:   nil,
		Size:    1024,
	}

	assert.Equal(t, "/tmp/test.txt", result.Path)
	assert.True(t, result.Success)
	assert.Nil(t, result.Error)
	assert.Equal(t, int64(1024), result.Size)
}

func TestCleanResult_Failure(t *testing.T) {
	result := cleaner.CleanResult{
		Path:    "/protected/path",
		Success: false,
		Error:   assert.AnError,
		Size:    0,
	}

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}
