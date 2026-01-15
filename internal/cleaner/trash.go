package cleaner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
)

// MoveToTrash moves a file or directory to the macOS Trash.
// Uses direct filesystem move to ~/.Trash to avoid Touch ID prompts.
func MoveToTrash(path string) error {
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

	counter := 1
	for counter <= 100 {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(baseName)
		name := baseName[:len(baseName)-len(ext)]
		destPath = filepath.Join(trashDir, fmt.Sprintf("%s %d%s", name, counter, ext))
		counter++
	}
	if counter > 100 {
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

	var size atomic.Int64

	conf := fastwalk.Config{
		Follow: false,
	}

	err = fastwalk.Walk(&conf, trashDir, func(_ string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !d.IsDir() {
			if info, infoErr := d.Info(); infoErr == nil {
				size.Add(info.Size())
			}
		}
		return nil
	})

	return size.Load(), err
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
