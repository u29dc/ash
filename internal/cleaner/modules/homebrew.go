package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// HomebrewModule handles Homebrew cache cleanup.
type HomebrewModule struct {
	BaseModule
	homeDir string
}

// NewHomebrewModule creates a new Homebrew cleanup module.
func NewHomebrewModule() (*HomebrewModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	m := &HomebrewModule{
		BaseModule: BaseModule{
			name:         "Homebrew",
			description:  "Downloaded bottles and old versions",
			category:     scanner.CategoryHomebrew,
			riskLevel:    scanner.RiskSafe,
			requiresSudo: false,
			enabled:      true,
		},
		homeDir: homeDir,
	}

	m.paths = []string{
		filepath.Join(homeDir, "Library", "Caches", "Homebrew"),
	}

	return m, nil
}

// Scan discovers Homebrew cache files.
func (m *HomebrewModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
	var entries []scanner.Entry

	for _, basePath := range m.paths {
		items, err := os.ReadDir(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, item := range items {
			select {
			case <-ctx.Done():
				return entries, ctx.Err()
			default:
			}

			path := filepath.Join(basePath, item.Name())

			// Skip protected items
			if !safety.IsSafePath(path) {
				continue
			}

			info, err := item.Info()
			if err != nil {
				continue
			}

			size := info.Size()
			if item.IsDir() {
				size = calcDirSize(path)
			}

			entries = append(entries, scanner.Entry{
				Path:     path,
				Name:     item.Name(),
				Size:     size,
				ModTime:  info.ModTime(),
				Category: scanner.CategoryHomebrew,
				Risk:     scanner.RiskSafe,
				IsDir:    item.IsDir(),
			})
		}
	}

	return entries, nil
}

// IsHomebrewInstalled checks if Homebrew is installed.
func IsHomebrewInstalled() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// GetHomebrewCacheSize returns the total size of Homebrew caches.
func GetHomebrewCacheSize() (int64, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	cachePath := filepath.Join(homeDir, "Library", "Caches", "Homebrew")
	return calcDirSize(cachePath), nil
}

// GetOutdatedPackages returns a list of outdated Homebrew packages.
func GetOutdatedPackages() ([]string, error) {
	if !IsHomebrewInstalled() {
		return nil, nil
	}

	cmd := exec.Command("brew", "outdated", "--quiet")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var packages []string
	for _, line := range lines {
		if line != "" {
			packages = append(packages, line)
		}
	}

	return packages, nil
}

// CleanupHomebrew runs brew cleanup to remove old versions.
func CleanupHomebrew(dryRun bool) error {
	if !IsHomebrewInstalled() {
		return nil
	}

	args := []string{"cleanup"}
	if dryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.Command("brew", args...)
	return cmd.Run()
}
