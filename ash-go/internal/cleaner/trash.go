package cleaner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// MoveToTrash moves a file or directory to the macOS Trash.
// This is the safe deletion method that allows recovery.
func MoveToTrash(path string) error {
	// Use AppleScript to move to Trash (most reliable method)
	script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, path)
	cmd := exec.Command("osascript", "-e", script)

	if err := cmd.Run(); err != nil {
		// Fallback to mv command if osascript fails
		return moveToTrashFallback(path)
	}

	return nil
}

// moveToTrashFallback uses a manual approach to move files to Trash.
func moveToTrashFallback(path string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	trashDir := filepath.Join(homeDir, ".Trash")

	// Ensure trash directory exists
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return err
	}

	// Generate unique destination name if file already exists in Trash
	baseName := filepath.Base(path)
	destPath := filepath.Join(trashDir, baseName)

	// Cap iterations at 100 to prevent infinite loops
	const maxTrashIterations = 100
	counter := 1
	for counter <= maxTrashIterations {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(baseName)
		name := baseName[:len(baseName)-len(ext)]
		destPath = filepath.Join(trashDir, fmt.Sprintf("%s %d%s", name, counter, ext))
		counter++
	}
	if counter > maxTrashIterations {
		return fmt.Errorf("too many files with name %q in trash", baseName)
	}

	return os.Rename(path, destPath)
}

// permanentDelete permanently removes a file or directory.
// This is irreversible and should be used with caution.
func permanentDelete(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return os.RemoveAll(path)
	}

	return os.Remove(path)
}

// EmptyTrash empties the macOS Trash.
func EmptyTrash() error {
	script := `tell application "Finder" to empty trash`
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// GetTrashSize returns the total size of items in Trash.
func GetTrashSize() (int64, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	trashDir := filepath.Join(homeDir, ".Trash")

	var size int64
	err = filepath.Walk(trashDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// GetTrashItemCount returns the number of items in Trash.
func GetTrashItemCount() (int, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	trashDir := filepath.Join(homeDir, ".Trash")

	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	return len(entries), nil
}

// TrashInfo contains information about the Trash.
type TrashInfo struct {
	ItemCount int
	TotalSize int64
	Path      string
}

// GetTrashInfo returns information about the Trash.
func GetTrashInfo() (*TrashInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	trashDir := filepath.Join(homeDir, ".Trash")

	count, err := GetTrashItemCount()
	if err != nil {
		return nil, err
	}

	size, err := GetTrashSize()
	if err != nil {
		return nil, err
	}

	return &TrashInfo{
		ItemCount: count,
		TotalSize: size,
		Path:      trashDir,
	}, nil
}
