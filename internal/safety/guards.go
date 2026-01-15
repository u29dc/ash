package safety

import (
	"os"
	"path/filepath"
	"strings"
)

// neverDelete contains paths that should never be deleted.
var neverDelete = []string{
	"~/Library/Keychains",
	"~/.ssh",
	"~/.gnupg",
	"~/.config",
	"~/.local/share",
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/private/var/vm",
	"/private/var/db",
	"/Applications",
}

// neverDeletePatterns contains file patterns that should never be deleted.
var neverDeletePatterns = []string{
	"*.keychain*",
	"*.pem",
	"*.key",
	".git",
	".gitignore",
	"id_rsa*",
	"id_ed25519*",
	"*.p12",
	"*.pfx",
}

// protectedBundleIDs contains bundle ID prefixes that should be protected.
var protectedBundleIDs = []string{
	"com.apple.",
	"com.microsoft.",
}

// IsSafePath checks if a path is safe to delete.
func IsSafePath(path string) bool {
	expanded := expandPath(path)

	// Check never-delete directories
	for _, blocked := range neverDelete {
		blockedExpanded := expandPath(blocked)
		if strings.HasPrefix(expanded, blockedExpanded) {
			return false
		}
	}

	// Check never-delete patterns
	base := filepath.Base(path)
	for _, pattern := range neverDeletePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return false
		}
	}

	// Check if path contains .git directory
	if containsGitDir(path) {
		return false
	}

	return true
}

// IsProtectedApp checks if a bundle ID belongs to a protected app.
func IsProtectedApp(bundleID string) bool {
	for _, prefix := range protectedBundleIDs {
		if strings.HasPrefix(bundleID, prefix) {
			return true
		}
	}
	return false
}

// RequiresConfirmation checks if a path requires explicit user confirmation.
func RequiresConfirmation(path string, size int64) bool {
	expanded := expandPath(path)

	// iOS backups
	if strings.Contains(expanded, "MobileSync/Backup") {
		return true
	}

	// Xcode Archives
	if strings.Contains(expanded, "Xcode/Archives") {
		return true
	}

	// Large items (> 1GB)
	if size > 1024*1024*1024 {
		return true
	}

	// Application Support
	if strings.Contains(expanded, "Application Support") {
		return true
	}

	return false
}

// GetNeverDeletePaths returns the list of paths that should never be deleted.
func GetNeverDeletePaths() []string {
	return neverDelete
}

// GetNeverDeletePatterns returns the patterns that should never be deleted.
func GetNeverDeletePatterns() []string {
	return neverDeletePatterns
}

// GetProtectedBundleIDs returns the protected bundle ID prefixes.
func GetProtectedBundleIDs() []string {
	return protectedBundleIDs
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func containsGitDir(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == ".git" {
			return true
		}
	}
	return false
}

// ValidatePaths checks a list of paths and returns only safe ones.
func ValidatePaths(paths []string) (safe []string, blocked []string) {
	for _, path := range paths {
		if IsSafePath(path) {
			safe = append(safe, path)
		} else {
			blocked = append(blocked, path)
		}
	}
	return safe, blocked
}

// SanitizePath removes any potentially dangerous characters from a path.
func SanitizePath(path string) string {
	// Remove null bytes and other control characters
	var sanitized strings.Builder
	for _, r := range path {
		if r >= 32 && r != 127 {
			sanitized.WriteRune(r)
		}
	}
	return sanitized.String()
}
