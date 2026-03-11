package cleaner

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
	"golang.org/x/sys/unix"
)

const trashDirPerm = 0o700

// MoveToTrash moves a file or directory to the macOS Trash.
// Uses direct filesystem move to ~/.Trash to avoid Touch ID prompts.
func MoveToTrash(path string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	trashDir := filepath.Join(homeDir, ".Trash")

	if err := ensureTrashDir(trashDir); err != nil {
		return err
	}

	baseName := filepath.Base(filepath.Clean(path))
	primaryDest := filepath.Join(trashDir, baseName)
	if err := renameExclusive(path, primaryDest); err == nil {
		return nil
	} else if !isRenameConflict(err) {
		return err
	}

	return moveToUniqueTrashDestination(path, trashDir, baseName)
}

func ensureTrashDir(trashDir string) error {
	if err := os.MkdirAll(trashDir, trashDirPerm); err != nil {
		return err
	}

	info, err := os.Lstat(trashDir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("trash path must not be a symlink: %s", trashDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("trash path is not a directory: %s", trashDir)
	}

	if info.Mode().Perm() != trashDirPerm {
		if err := os.Chmod(trashDir, trashDirPerm); err != nil {
			return err
		}
	}

	return nil
}

func uniqueTrashDestination(trashDir, baseName string) (string, error) {
	ext := filepath.Ext(baseName)
	name := baseName[:len(baseName)-len(ext)]
	if name == "" {
		name = baseName
	}

	suffix, err := randomTrashSuffix()
	if err != nil {
		return "", err
	}

	return filepath.Join(trashDir, fmt.Sprintf("%s %s%s", name, suffix, ext)), nil
}

func randomTrashSuffix() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func renameExclusive(sourcePath, destPath string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, sourcePath, unix.AT_FDCWD, destPath, unix.RENAME_EXCL)
}

func moveToUniqueTrashDestination(path, trashDir, baseName string) error {
	for attempt := 0; attempt < 100; attempt++ {
		destPath, err := uniqueTrashDestination(trashDir, baseName)
		if err != nil {
			return err
		}
		err = renameExclusive(path, destPath)
		if err == nil {
			return nil
		}
		if isRenameConflict(err) {
			continue
		}
		return err
	}

	return fmt.Errorf("too many files with name %q in trash", baseName)
}

func isRenameConflict(err error) bool {
	return errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY)
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
