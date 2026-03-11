package safety_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/safety"
)

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Safe paths
		{"cache dir", "~/Library/Caches/com.example.app", true},
		{"log file", "~/Library/Logs/app.log", true},
		{"derived data", "~/Library/Developer/Xcode/DerivedData/Project-abc123", true},
		{"brew cache", "~/Library/Caches/Homebrew", true},
		{"temp file", "/tmp/test.txt", true},

		// Dangerous paths - must return false
		{"keychain", "~/Library/Keychains/login.keychain-db", false},
		{"ssh key", "~/.ssh/id_ed25519", false},
		{"ssh dir", "~/.ssh", false},
		{"gnupg", "~/.gnupg/private-keys-v1.d", false},
		{"gnupg dir", "~/.gnupg", false},
		{"system", "/System/Library/CoreServices", false},
		{"usr", "/usr/bin/bash", false},
		{"bin", "/bin/ls", false},
		{"sbin", "/sbin/mount", false},
		{"git dir", "~/projects/app/.git/objects", false},
		{"git file", "~/projects/app/.git/config", false},
		{"applications", "/Applications/Safari.app", false},

		// Pattern matches
		{"pem file", "/tmp/server.pem", false},
		{"key file", "/tmp/private.key", false},
		{"keychain file", "/tmp/test.keychain", false},
		{"rsa key", "~/.ssh/id_rsa", false},
		{"ed25519 key", "~/.ssh/id_ed25519", false},

		// Edge cases
		{"hidden in cache", "~/Library/Caches/.hidden", true},
		{"keychain in name only", "~/Library/Caches/keychain-backup", true},
		{"nested safe path", "~/Library/Caches/com.app/subdir/file.dat", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safety.IsSafePath(tt.path)
			assert.Equal(t, tt.want, got, "IsSafePath(%q)", tt.path)
		})
	}
}

func TestIsProtectedApp(t *testing.T) {
	tests := []struct {
		name     string
		bundleID string
		want     bool
	}{
		// Protected apps
		{"apple safari", "com.apple.Safari", true},
		{"apple finder", "com.apple.finder", true},
		{"apple mail", "com.apple.mail", true},
		{"microsoft vscode", "com.microsoft.VSCode", true},
		{"microsoft word", "com.microsoft.Word", true},

		// Third party apps - not protected
		{"spotify", "com.spotify.client", false},
		{"slack", "com.tinyspeck.slackmacgap", false},
		{"homebrew cask", "com.homebrew.cask", false},
		{"chrome", "com.google.Chrome", false},
		{"firefox", "org.mozilla.firefox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safety.IsProtectedApp(tt.bundleID)
			assert.Equal(t, tt.want, got, "IsProtectedApp(%q)", tt.bundleID)
		})
	}
}

func TestRequiresConfirmation(t *testing.T) {
	tests := []struct {
		name string
		path string
		size int64
		want bool
	}{
		// Requires confirmation
		{"ios backup", "~/Library/Application Support/MobileSync/Backup/abc123", 1024, true},
		{"xcode archives", "~/Library/Developer/Xcode/Archives/2024", 1024, true},
		{"large file", "/tmp/large.dat", 2 * 1024 * 1024 * 1024, true}, // 2GB
		{"app support", "~/Library/Application Support/SomeApp", 1024, true},

		// Does not require confirmation
		{"small cache", "~/Library/Caches/com.example.app", 1024, false},
		{"logs", "~/Library/Logs/app.log", 1024, false},
		{"derived data", "~/Library/Developer/Xcode/DerivedData/Project", 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safety.RequiresConfirmation(tt.path, tt.size)
			assert.Equal(t, tt.want, got, "RequiresConfirmation(%q, %d)", tt.path, tt.size)
		})
	}
}

func TestValidatePaths(t *testing.T) {
	paths := []string{
		"~/Library/Caches/safe",
		"~/.ssh/id_rsa",
		"~/Library/Logs/app.log",
		"/System/Library/file",
		"/tmp/temp.dat",
	}

	safe, blocked := safety.ValidatePaths(paths)

	assert.Contains(t, safe, "~/Library/Caches/safe")
	assert.Contains(t, safe, "~/Library/Logs/app.log")
	assert.Contains(t, safe, "/tmp/temp.dat")

	assert.Contains(t, blocked, "~/.ssh/id_rsa")
	assert.Contains(t, blocked, "/System/Library/file")
}

func TestGetNeverDeletePaths(t *testing.T) {
	paths := safety.GetNeverDeletePaths()

	assert.NotEmpty(t, paths)
	assert.Contains(t, paths, "~/Library/Keychains")
	assert.Contains(t, paths, "~/.ssh")
	assert.Contains(t, paths, "~/.gnupg")
	assert.Contains(t, paths, "/System")
}

func TestGetNeverDeletePatterns(t *testing.T) {
	patterns := safety.GetNeverDeletePatterns()

	assert.NotEmpty(t, patterns)
	assert.Contains(t, patterns, "*.pem")
	assert.Contains(t, patterns, "*.key")
	assert.Contains(t, patterns, ".git")
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal path", "/tmp/file.txt", "/tmp/file.txt"},
		{"path with spaces", "/tmp/my file.txt", "/tmp/my file.txt"},
		{"path with null", "/tmp/file\x00.txt", "/tmp/file.txt"},
		{"path with control chars", "/tmp/file\x01\x02.txt", "/tmp/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safety.SanitizePath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSafePath_BlocksSymlinkIntoProtectedDirectory(t *testing.T) {
	cacheDir := t.TempDir()
	linkPath := filepath.Join(cacheDir, "applications-link")
	require.NoError(t, os.Symlink("/Applications", linkPath))

	assert.False(t, safety.IsSafePath(linkPath))
}
