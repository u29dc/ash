package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"ash/internal/safety"
	"ash/internal/scanner"
	plistutil "ash/pkg/plist"
)

// AppsModule handles app leftover cleanup.
type AppsModule struct {
	BaseModule
	homeDir string
}

// NewAppsModule creates a new app leftover cleanup module.
func NewAppsModule() (*AppsModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

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

	return m, nil
}

// Scan discovers app leftover files.
func (m *AppsModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
	var entries []scanner.Entry

	// Build set of installed app bundle IDs
	installedApps := m.getInstalledAppBundleIDs()

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

			name := item.Name()
			path := filepath.Join(basePath, name)

			// Skip protected items
			if !safety.IsSafePath(path) {
				continue
			}

			// Skip system items
			if strings.HasPrefix(name, "com.apple.") {
				continue
			}

			// Check if this looks like an orphan
			bundleID := m.extractBundleID(name)
			if bundleID == "" {
				continue
			}

			// Skip if app is still installed
			if installedApps[bundleID] {
				continue
			}

			// Skip protected apps
			if safety.IsProtectedApp(bundleID) {
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
				Name:     name,
				Size:     size,
				ModTime:  info.ModTime(),
				Category: scanner.CategoryAppData,
				Risk:     scanner.RiskCaution,
				BundleID: bundleID,
				IsDir:    item.IsDir(),
			})
		}
	}

	return entries, nil
}

func (m *AppsModule) getInstalledAppBundleIDs() map[string]bool {
	installed := make(map[string]bool)

	appDirs := []string{
		"/Applications",
		filepath.Join(m.homeDir, "Applications"),
	}

	for _, appDir := range appDirs {
		_ = filepath.WalkDir(appDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".app") {
				return nil
			}

			bundleID := m.getBundleID(path)
			if bundleID != "" {
				installed[bundleID] = true
			}

			return filepath.SkipDir
		})
	}

	return installed
}

func (m *AppsModule) getBundleID(appPath string) string {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	bundleID, err := plistutil.GetBundleID(plistPath)
	if err != nil {
		return ""
	}
	return bundleID
}

func (m *AppsModule) extractBundleID(name string) string {
	// Remove common suffixes
	suffixes := []string{".plist", ".savedState", ".binarycookies"}
	result := name
	for _, suffix := range suffixes {
		result = strings.TrimSuffix(result, suffix)
	}

	// Check if it looks like a bundle ID (contains dots)
	if strings.Count(result, ".") >= 2 {
		return result
	}

	return ""
}
