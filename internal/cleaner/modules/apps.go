package modules

import (
	"context"
	"os"
	"path/filepath"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// AppsModule handles app leftover cleanup.
type AppsModule struct {
	BaseModule
	homeDir string
	finder  *scanner.OrphanFinder
}

// NewAppsModule creates a new app leftover cleanup module.
func NewAppsModule() (*AppsModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return newAppsModule(homeDir), nil
}

func newAppsModule(homeDir string) *AppsModule {
	libDir := filepath.Join(homeDir, "Library")
	m := &AppsModule{
		BaseModule: BaseModule{
			name:         "App Leftovers",
			description:  "Orphaned files from uninstalled applications",
			category:     scanner.CategoryAppData,
			riskLevel:    scanner.RiskCaution,
			requiresSudo: false,
			enabled:      false, // Disabled by default - requires user opt-in
		},
		homeDir: homeDir,
		finder:  scanner.NewOrphanFinderForHome(homeDir),
	}

	m.paths = []string{
		filepath.Join(libDir, "Application Support"),
		filepath.Join(libDir, "Preferences"),
		filepath.Join(libDir, "Caches"),
		filepath.Join(libDir, "Containers"),
		filepath.Join(libDir, "Group Containers"),
		filepath.Join(libDir, "Saved Application State"),
		filepath.Join(libDir, "WebKit"),
		filepath.Join(libDir, "HTTPStorages"),
		filepath.Join(libDir, "Cookies"),
		filepath.Join(libDir, "LaunchAgents"),
	}

	return m
}

// Scan discovers app leftover files.
func (m *AppsModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
	if m.finder == nil {
		m.finder = scanner.NewOrphanFinderForHome(m.homeDir)
	}

	leftovers, err := m.finder.FindAllOrphansContext(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]scanner.Entry, 0, len(leftovers))
	for _, leftover := range leftovers {
		if err := ctx.Err(); err != nil {
			return entries, err
		}
		if !safety.IsSafePath(leftover.Path) {
			continue
		}

		info, err := os.Lstat(leftover.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return entries, err
		}

		bundleID := ""
		if leftover.AppInfo != nil {
			bundleID = leftover.AppInfo.BundleID
		}
		if bundleID != "" && safety.IsProtectedApp(bundleID) {
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		var symlinkTarget string
		if isSymlink {
			symlinkTarget, _ = os.Readlink(leftover.Path)
		}

		entries = append(entries, scanner.Entry{
			Path:          leftover.Path,
			Name:          filepath.Base(leftover.Path),
			Size:          leftover.Size,
			ModTime:       info.ModTime(),
			Category:      scanner.CategoryAppData,
			Risk:          leftover.RiskLevel(),
			BundleID:      bundleID,
			IsDir:         info.IsDir(),
			IsSymlink:     isSymlink,
			SymlinkTarget: symlinkTarget,
		})
	}

	return entries, nil
}
