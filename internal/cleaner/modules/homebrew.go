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

var brewBinaryCandidates = []string{
	"/opt/homebrew/bin/brew",
	"/usr/local/bin/brew",
}

const (
	homebrewOptBrew   = "/opt/homebrew/bin/brew"
	homebrewLocalBrew = "/usr/local/bin/brew"
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

			isSymlink := info.Mode()&os.ModeSymlink != 0
			var symlinkTarget string
			if isSymlink {
				symlinkTarget, _ = os.Readlink(path)
			}

			size := info.Size()
			if item.IsDir() && !isSymlink {
				size, err = calcDirSizeWithContext(ctx, path)
				if err != nil {
					return entries, err
				}
			}

			entries = append(entries, scanner.Entry{
				Path:          path,
				Name:          item.Name(),
				Size:          size,
				ModTime:       info.ModTime(),
				Category:      scanner.CategoryHomebrew,
				Risk:          scanner.RiskSafe,
				IsDir:         item.IsDir(),
				IsSymlink:     isSymlink,
				SymlinkTarget: symlinkTarget,
			})
		}
	}

	return entries, nil
}

// IsHomebrewInstalled checks if Homebrew is installed.
func IsHomebrewInstalled() bool {
	_, ok := brewBinaryPath()
	return ok
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
	cmd, ok := brewOutdatedCommand()
	if !ok {
		return nil, nil
	}

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
	cmd, ok := brewCleanupCommand(dryRun)
	if !ok {
		return nil
	}
	return cmd.Run()
}

func brewBinaryPath() (string, bool) {
	for _, candidate := range brewBinaryCandidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, true
	}
	return "", false
}

func brewOutdatedCommand() (*exec.Cmd, bool) {
	brewBinary, ok := brewBinaryPath()
	if !ok {
		return nil, false
	}

	switch brewBinary {
	case homebrewOptBrew:
		return exec.Command(homebrewOptBrew, "outdated", "--quiet"), true
	case homebrewLocalBrew:
		return exec.Command(homebrewLocalBrew, "outdated", "--quiet"), true
	default:
		return nil, false
	}
}

func brewCleanupCommand(dryRun bool) (*exec.Cmd, bool) {
	brewBinary, ok := brewBinaryPath()
	if !ok {
		return nil, false
	}

	switch brewBinary {
	case homebrewOptBrew:
		if dryRun {
			return exec.Command(homebrewOptBrew, "cleanup", "--dry-run"), true
		}
		return exec.Command(homebrewOptBrew, "cleanup"), true
	case homebrewLocalBrew:
		if dryRun {
			return exec.Command(homebrewLocalBrew, "cleanup", "--dry-run"), true
		}
		return exec.Command(homebrewLocalBrew, "cleanup"), true
	default:
		return nil, false
	}
}
