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
	"/Library/Keychains",
	"/Network",
	"/cores",
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
	"*.cer",
	"*.crt",
	"known_hosts",
	"authorized_keys",
}

// protectedBundleIDs contains bundle ID prefixes that should be protected.
var protectedBundleIDs = []string{
	"com.apple.",
	"com.microsoft.",
}

// SizeConfirmationThreshold is the size (1GB) above which items require confirmation.
const SizeConfirmationThreshold int64 = 1024 * 1024 * 1024

// IsSafePath checks if a path is safe to delete.
func IsSafePath(path string) bool {
	expanded := expandPath(path)

	// Resolve symlinks to prevent bypass attacks (e.g., symlink pointing to ~/.ssh)
	resolved, err := filepath.EvalSymlinks(expanded)
	if err == nil {
		expanded = resolved
	}
	// If symlink resolution fails (e.g., broken symlink), continue with original path

	// Check never-delete directories against both original and resolved paths
	originalExpanded := expandPath(path)
	for _, blocked := range neverDelete {
		blockedExpanded := expandPath(blocked)
		// Check both the resolved path and original path against blocked directories
		if strings.HasPrefix(expanded, blockedExpanded) || strings.HasPrefix(originalExpanded, blockedExpanded) {
			return false
		}
	}

	// Check never-delete patterns
	base := filepath.Base(path)
	for _, pattern := range neverDeletePatterns {
		matched, err := filepath.Match(pattern, base)
		if err != nil {
			return false // Treat invalid patterns as blocked for safety
		}
		if matched {
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
	if size > SizeConfirmationThreshold {
		return true
	}

	// Application Support
	if strings.Contains(expanded, "Application Support") {
		return true
	}

	return false
}

// GetNeverDeletePaths returns a copy of the paths that should never be deleted.
func GetNeverDeletePaths() []string {
	result := make([]string, len(neverDelete))
	copy(result, neverDelete)
	return result
}

// GetNeverDeletePatterns returns a copy of the patterns that should never be deleted.
func GetNeverDeletePatterns() []string {
	result := make([]string, len(neverDeletePatterns))
	copy(result, neverDeletePatterns)
	return result
}

// GetProtectedBundleIDs returns a copy of the protected bundle ID prefixes.
func GetProtectedBundleIDs() []string {
	result := make([]string, len(protectedBundleIDs))
	copy(result, protectedBundleIDs)
	return result
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
