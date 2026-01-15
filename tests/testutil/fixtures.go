package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// CreateTempCacheDir creates a temporary directory with cache files.
func CreateTempCacheDir(t *testing.T, fileCount int, fileSize int64) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-cache-*")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < fileCount; i++ {
		path := filepath.Join(dir, fmt.Sprintf("cache-%d.dat", i))
		if err := createFile(path, fileSize); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// CreateMixedSizeDir creates a directory with files of various sizes.
func CreateMixedSizeDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-mixed-*")
	if err != nil {
		t.Fatal(err)
	}

	sizes := []int64{
		100,              // 100 bytes
		1024,             // 1 KB
		1024 * 100,       // 100 KB
		1024 * 1024,      // 1 MB
		1024 * 1024 * 5,  // 5 MB
		1024 * 1024 * 10, // 10 MB
	}

	for i, size := range sizes {
		path := filepath.Join(dir, fmt.Sprintf("file-%d.dat", i))
		if err := createFile(path, size); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// CreateRestrictedDir creates a directory with restricted permissions.
func CreateRestrictedDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-restricted-*")
	if err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory with no read permission
	restrictedDir := filepath.Join(dir, "restricted")
	if err := os.Mkdir(restrictedDir, 0000); err != nil {
		t.Fatal(err)
	}

	// Create some accessible files
	for i := 0; i < 3; i++ {
		path := filepath.Join(dir, fmt.Sprintf("accessible-%d.dat", i))
		if err := createFile(path, 1024); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// CreateNestedDir creates a directory structure with nested subdirectories.
func CreateNestedDir(t *testing.T, depth int) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-nested-*")
	if err != nil {
		t.Fatal(err)
	}

	current := dir
	for i := 0; i < depth; i++ {
		subdir := filepath.Join(current, fmt.Sprintf("level-%d", i))
		if err := os.Mkdir(subdir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a file at each level
		filePath := filepath.Join(subdir, "file.dat")
		if err := createFile(filePath, 1024); err != nil {
			t.Fatal(err)
		}

		current = subdir
	}

	return dir
}

// CreateDirWithCategories creates a directory structure mimicking Library.
func CreateDirWithCategories(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-library-*")
	if err != nil {
		t.Fatal(err)
	}

	categories := []string{"Caches", "Logs", "Preferences"}

	for _, cat := range categories {
		catDir := filepath.Join(dir, cat)
		if err := os.Mkdir(catDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create some files in each category
		for i := 0; i < 3; i++ {
			path := filepath.Join(catDir, fmt.Sprintf("item-%d", i))
			if err := createFile(path, 1024*int64(i+1)); err != nil {
				t.Fatal(err)
			}
		}
	}

	return dir
}

func createFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		return err
	}

	return nil
}

// Cleanup removes a test directory.
func Cleanup(t *testing.T, dir string) {
	t.Helper()

	// Restore permissions before cleanup
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			_ = os.Chmod(path, 0755)
		}
		return nil
	})

	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to cleanup %s: %v", dir, err)
	}
}

// CreateTempFile creates a temporary file with the given content.
func CreateTempFile(t *testing.T, name, content string) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ash-test-*")
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	return path
}
