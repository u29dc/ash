package modules

import (
	"context"
	"os"
	"path/filepath"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// BrowsersModule handles browser cache cleanup.
type BrowsersModule struct {
	BaseModule
	homeDir string
}

// BrowserInfo contains information about a browser.
type BrowserInfo struct {
	Name       string
	BundleID   string
	CachePaths []string
}

// NewBrowsersModule creates a new browsers cleanup module.
func NewBrowsersModule() (*BrowsersModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cacheDir := filepath.Join(homeDir, "Library", "Caches")

	m := &BrowsersModule{
		BaseModule: BaseModule{
			name:         "Browsers",
			description:  "Safari, Chrome, Firefox, and other browser caches",
			category:     scanner.CategoryBrowsers,
			riskLevel:    scanner.RiskSafe,
			requiresSudo: false,
			enabled:      true,
		},
		homeDir: homeDir,
	}

	m.paths = []string{
		filepath.Join(cacheDir, "com.apple.Safari"),
		filepath.Join(cacheDir, "Google", "Chrome"),
		filepath.Join(cacheDir, "Firefox"),
		filepath.Join(cacheDir, "com.brave.Browser"),
		filepath.Join(cacheDir, "com.microsoft.edgemac"),
		filepath.Join(cacheDir, "org.mozilla.firefox"),
		filepath.Join(cacheDir, "com.operasoftware.Opera"),
		filepath.Join(cacheDir, "company.thebrowser.Browser"), // Arc browser
	}

	return m, nil
}

// Scan discovers browser cache files.
func (m *BrowsersModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
	var entries []scanner.Entry

	for _, basePath := range m.paths {
		info, err := os.Lstat(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		select {
		case <-ctx.Done():
			return entries, ctx.Err()
		default:
		}

		// Skip protected items
		if !safety.IsSafePath(basePath) {
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		var symlinkTarget string
		if isSymlink {
			symlinkTarget, _ = os.Readlink(basePath)
		}

		size := info.Size()
		if info.IsDir() && !isSymlink {
			size = calcDirSize(basePath)
		}

		browserName := m.identifyBrowser(basePath)

		entries = append(entries, scanner.Entry{
			Path:          basePath,
			Name:          browserName,
			Size:          size,
			ModTime:       info.ModTime(),
			Category:      scanner.CategoryBrowsers,
			Risk:          scanner.RiskSafe,
			IsDir:         info.IsDir(),
			IsSymlink:     isSymlink,
			SymlinkTarget: symlinkTarget,
		})
	}

	return entries, nil
}

func (m *BrowsersModule) identifyBrowser(path string) string {
	base := filepath.Base(path)

	browsers := map[string]string{
		"com.apple.Safari":           "Safari",
		"Chrome":                     "Google Chrome",
		"Firefox":                    "Firefox",
		"com.brave.Browser":          "Brave",
		"com.microsoft.edgemac":      "Microsoft Edge",
		"org.mozilla.firefox":        "Firefox",
		"com.operasoftware.Opera":    "Opera",
		"company.thebrowser.Browser": "Arc",
	}

	// Check parent directory for Chrome
	if base == "Chrome" {
		parent := filepath.Base(filepath.Dir(path))
		if parent == "Google" {
			return "Google Chrome"
		}
	}

	if name, ok := browsers[base]; ok {
		return name
	}

	return base
}

// GetInstalledBrowsers returns a list of installed browsers.
func GetInstalledBrowsers() []BrowserInfo {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	cacheDir := filepath.Join(homeDir, "Library", "Caches")

	browsers := []BrowserInfo{
		{
			Name:     "Safari",
			BundleID: "com.apple.Safari",
			CachePaths: []string{
				filepath.Join(cacheDir, "com.apple.Safari"),
			},
		},
		{
			Name:     "Google Chrome",
			BundleID: "com.google.Chrome",
			CachePaths: []string{
				filepath.Join(cacheDir, "Google", "Chrome"),
			},
		},
		{
			Name:     "Firefox",
			BundleID: "org.mozilla.firefox",
			CachePaths: []string{
				filepath.Join(cacheDir, "Firefox"),
				filepath.Join(cacheDir, "org.mozilla.firefox"),
			},
		},
		{
			Name:     "Brave",
			BundleID: "com.brave.Browser",
			CachePaths: []string{
				filepath.Join(cacheDir, "com.brave.Browser"),
			},
		},
		{
			Name:     "Microsoft Edge",
			BundleID: "com.microsoft.edgemac",
			CachePaths: []string{
				filepath.Join(cacheDir, "com.microsoft.edgemac"),
			},
		},
		{
			Name:     "Arc",
			BundleID: "company.thebrowser.Browser",
			CachePaths: []string{
				filepath.Join(cacheDir, "company.thebrowser.Browser"),
			},
		},
	}

	var installed []BrowserInfo
	for _, browser := range browsers {
		for _, cachePath := range browser.CachePaths {
			if _, err := os.Stat(cachePath); err == nil {
				installed = append(installed, browser)
				break
			}
		}
	}

	return installed
}
