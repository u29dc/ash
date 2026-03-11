package cleaner_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/cleaner"
)

func TestMoveToTrash_CreatesPrivateTrashDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sourcePath := filepath.Join(homeDir, "work", "note.txt")
	writeTestFile(t, sourcePath, "note")

	require.NoError(t, cleaner.MoveToTrash(sourcePath))

	trashDir := filepath.Join(homeDir, ".Trash")
	info, err := os.Stat(trashDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	entries, err := os.ReadDir(trashDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "note.txt", entries[0].Name())
}

func TestMoveToTrash_SecuresExistingTrashDirectoryPermissions(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	trashDir := filepath.Join(homeDir, ".Trash")
	require.NoError(t, os.MkdirAll(trashDir, 0o755))
	require.NoError(t, os.Chmod(trashDir, 0o755))

	sourcePath := filepath.Join(homeDir, "logs", "example.log")
	writeTestFile(t, sourcePath, "log")

	require.NoError(t, cleaner.MoveToTrash(sourcePath))

	info, err := os.Stat(trashDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestMoveToTrash_HandlesDuplicateBasenamesConcurrently(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sourceA := filepath.Join(homeDir, "a", "duplicate.txt")
	sourceB := filepath.Join(homeDir, "b", "duplicate.txt")
	writeTestFile(t, sourceA, "a")
	writeTestFile(t, sourceB, "b")

	paths := []string{sourceA, sourceB}
	errs := make(chan error, len(paths))
	var wg sync.WaitGroup

	for _, path := range paths {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			errs <- cleaner.MoveToTrash(target)
		}(path)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(filepath.Join(homeDir, ".Trash"))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.NotEqual(t, entries[0].Name(), entries[1].Name())
}

func TestMoveToTrash_RejectsSymlinkedTrashDirectory(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	realTrashDir := filepath.Join(t.TempDir(), "real-trash")
	require.NoError(t, os.MkdirAll(realTrashDir, 0o700))
	require.NoError(t, os.Symlink(realTrashDir, filepath.Join(homeDir, ".Trash")))

	sourcePath := filepath.Join(homeDir, "work", "example.log")
	writeTestFile(t, sourcePath, "log")

	err := cleaner.MoveToTrash(sourcePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trash path must not be a symlink")

	_, statErr := os.Stat(sourcePath)
	require.NoError(t, statErr)
}

func TestTrashInfo_Properties(t *testing.T) {
	info := &cleaner.TrashInfo{
		ItemCount: 10,
		TotalSize: 1024 * 1024 * 100,
		Path:      "/Users/test/.Trash",
	}

	assert.Equal(t, 10, info.ItemCount)
	assert.Equal(t, int64(1024*1024*100), info.TotalSize)
	assert.Equal(t, "/Users/test/.Trash", info.Path)
}

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

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}
