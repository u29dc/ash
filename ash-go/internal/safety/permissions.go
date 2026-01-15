package safety

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PermissionStatus represents the Full Disk Access status.
type PermissionStatus int

const (
	PermissionUnknown PermissionStatus = iota
	PermissionGranted
	PermissionDenied
)

// CheckFullDiskAccess checks if the application has Full Disk Access.
// On macOS, this is required to access certain directories like ~/Library/Mail.
func CheckFullDiskAccess() PermissionStatus {
	// Try to access a directory that requires FDA
	testPaths := []string{
		"~/Library/Mail",
		"~/Library/Messages",
		"~/Library/Safari",
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return PermissionUnknown
	}

	for _, testPath := range testPaths {
		path := strings.Replace(testPath, "~", home, 1)
		_, err := os.ReadDir(path)
		if err != nil {
			if os.IsPermission(err) {
				return PermissionDenied
			}
			// Directory might not exist, which is fine
			continue
		}
		// If we can read at least one of these, we likely have FDA
		return PermissionGranted
	}

	return PermissionUnknown
}

// CheckSudoAccess checks if the current user has sudo privileges.
func CheckSudoAccess() bool {
	cmd := exec.Command("sudo", "-n", "true")
	err := cmd.Run()
	return err == nil
}

// CanAccessPath checks if we can read the given path.
func CanAccessPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CanWritePath checks if we can write to the given path.
func CanWritePath(path string) bool {
	// Check if parent directory is writable
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	// Check if directory has write permission
	return info.Mode().Perm()&0200 != 0
}

// RequiresSudo checks if accessing a path requires sudo privileges.
func RequiresSudo(path string) bool {
	systemPaths := []string{
		"/Library/Caches",
		"/Library/Logs",
		"/private/var/log",
		"/System",
	}

	for _, sysPath := range systemPaths {
		if strings.HasPrefix(path, sysPath) {
			return true
		}
	}

	return false
}

// FDAInstructions returns instructions for granting Full Disk Access.
func FDAInstructions() string {
	return `Full Disk Access is required to scan all directories.

To grant access:
1. Open System Preferences > Security & Privacy > Privacy
2. Select "Full Disk Access" from the left sidebar
3. Click the lock to make changes
4. Add Terminal (or your terminal app) to the list
5. Restart your terminal and run ash again`
}

// GetAccessiblePaths filters paths to only those we can access.
func GetAccessiblePaths(paths []string) []string {
	var accessible []string
	for _, path := range paths {
		if CanAccessPath(path) {
			accessible = append(accessible, path)
		}
	}
	return accessible
}
