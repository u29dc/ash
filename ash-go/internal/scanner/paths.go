package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// PathConfig holds path configurations for scanning.
type PathConfig struct {
	HomeDir string
	LibDir  string
}

// NewPathConfig creates a new PathConfig with the user's home directory.
func NewPathConfig() (*PathConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return &PathConfig{
		HomeDir: homeDir,
		LibDir:  filepath.Join(homeDir, "Library"),
	}, nil
}

// CachePaths returns all cache directories to scan.
func (p *PathConfig) CachePaths() []string {
	return []string{
		filepath.Join(p.LibDir, "Caches"),
	}
}

// LogPaths returns all log directories to scan.
func (p *PathConfig) LogPaths() []string {
	return []string{
		filepath.Join(p.LibDir, "Logs"),
	}
}

// XcodePaths returns all Xcode-related paths to scan.
func (p *PathConfig) XcodePaths() []string {
	devDir := filepath.Join(p.LibDir, "Developer")
	return []string{
		filepath.Join(devDir, "Xcode", "DerivedData"),
		filepath.Join(devDir, "Xcode", "Archives"),
		filepath.Join(devDir, "Xcode", "iOS DeviceSupport"),
		filepath.Join(devDir, "CoreSimulator", "Devices"),
	}
}

// HomebrewPaths returns Homebrew cache paths.
func (p *PathConfig) HomebrewPaths() []string {
	return []string{
		filepath.Join(p.LibDir, "Caches", "Homebrew"),
	}
}

// BrowserPaths returns browser cache paths.
func (p *PathConfig) BrowserPaths() []string {
	return []string{
		filepath.Join(p.LibDir, "Caches", "com.apple.Safari"),
		filepath.Join(p.LibDir, "Caches", "Google", "Chrome"),
		filepath.Join(p.LibDir, "Caches", "Firefox"),
	}
}

// AppLeftoverLocations returns paths where app leftovers typically reside.
func (p *PathConfig) AppLeftoverLocations() []string {
	return []string{
		filepath.Join(p.LibDir, "Application Support"),
		filepath.Join(p.LibDir, "Preferences"),
		filepath.Join(p.LibDir, "Caches"),
		filepath.Join(p.LibDir, "Containers"),
		filepath.Join(p.LibDir, "Group Containers"),
		filepath.Join(p.LibDir, "Saved Application State"),
		filepath.Join(p.LibDir, "WebKit"),
		filepath.Join(p.LibDir, "HTTPStorages"),
		filepath.Join(p.LibDir, "Cookies"),
		filepath.Join(p.LibDir, "LaunchAgents"),
	}
}

// ExpandPath expands ~ to the user's home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// IsSubPath checks if child is a subdirectory of parent.
func IsSubPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	if parent == child {
		return true
	}

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(rel, "..")
}
