package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// ConfidenceLevel indicates how confident we are that a leftover belongs to an app.
type ConfidenceLevel int

const (
	ConfidenceHigh   ConfidenceLevel = iota // Exact bundle ID match
	ConfidenceMedium                        // App name match
	ConfidenceLow                           // Company name only
)

// AppInfo contains information extracted from an app bundle.
type AppInfo struct {
	BundleID    string
	Name        string
	DisplayName string
	Company     string
	Path        string
}

// Leftover represents an orphaned app file or directory.
type Leftover struct {
	Path       string
	Size       int64
	AppInfo    *AppInfo
	Confidence ConfidenceLevel
	Location   string // Which Library subdirectory
}

// OrphanFinder detects leftover files from uninstalled applications.
type OrphanFinder struct {
	pathConfig *PathConfig
}

// NewOrphanFinder creates a new OrphanFinder.
func NewOrphanFinder() (*OrphanFinder, error) {
	cfg, err := NewPathConfig()
	if err != nil {
		return nil, err
	}
	return &OrphanFinder{pathConfig: cfg}, nil
}

// ExtractAppInfo extracts bundle information from an app bundle.
func (o *OrphanFinder) ExtractAppInfo(appPath string) (*AppInfo, error) {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, err
	}

	var info struct {
		CFBundleIdentifier  string `plist:"CFBundleIdentifier"`
		CFBundleName        string `plist:"CFBundleName"`
		CFBundleDisplayName string `plist:"CFBundleDisplayName"`
	}

	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	appInfo := &AppInfo{
		BundleID:    info.CFBundleIdentifier,
		Name:        info.CFBundleName,
		DisplayName: info.CFBundleDisplayName,
		Path:        appPath,
	}

	// Extract company from bundle ID (e.g., com.company.app -> company)
	parts := strings.Split(info.CFBundleIdentifier, ".")
	if len(parts) >= 2 {
		appInfo.Company = parts[1]
	}

	return appInfo, nil
}

// GenerateSearchTerms creates search terms from app info.
func (o *OrphanFinder) GenerateSearchTerms(info *AppInfo) []string {
	terms := make([]string, 0, 4)

	if info.BundleID != "" {
		terms = append(terms, info.BundleID)
	}
	if info.Name != "" {
		terms = append(terms, info.Name)
		terms = append(terms, strings.ToLower(info.Name))
	}
	if info.DisplayName != "" && info.DisplayName != info.Name {
		terms = append(terms, info.DisplayName)
	}
	if info.Company != "" {
		terms = append(terms, info.Company)
	}

	return terms
}

// FindLeftovers finds orphaned files for a given app.
func (o *OrphanFinder) FindLeftovers(info *AppInfo) ([]Leftover, error) {
	var leftovers []Leftover
	searchTerms := o.GenerateSearchTerms(info)

	locations := o.pathConfig.AppLeftoverLocations()

	for _, location := range locations {
		entries, err := os.ReadDir(location)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			name := entry.Name()
			confidence, matched := o.matchEntry(name, searchTerms, info.BundleID)
			if !matched {
				continue
			}

			path := filepath.Join(location, name)
			size := o.getSize(path, entry.IsDir())

			leftovers = append(leftovers, Leftover{
				Path:       path,
				Size:       size,
				AppInfo:    info,
				Confidence: confidence,
				Location:   filepath.Base(location),
			})
		}
	}

	return leftovers, nil
}

func (o *OrphanFinder) matchEntry(name string, terms []string, bundleID string) (ConfidenceLevel, bool) {
	nameLower := strings.ToLower(name)

	// Check for exact bundle ID match (highest confidence)
	if bundleID != "" && strings.Contains(nameLower, strings.ToLower(bundleID)) {
		return ConfidenceHigh, true
	}

	// Check each search term
	for i, term := range terms {
		termLower := strings.ToLower(term)
		if strings.Contains(nameLower, termLower) {
			// First term (bundle ID) = high, next terms = medium, company = low
			switch {
			case i == 0:
				return ConfidenceHigh, true
			case i < 3:
				return ConfidenceMedium, true
			default:
				return ConfidenceLow, true
			}
		}
	}

	return ConfidenceLow, false
}

func (o *OrphanFinder) getSize(path string, isDir bool) int64 {
	if !isDir {
		info, err := os.Stat(path)
		if err != nil {
			return 0
		}
		return info.Size()
	}

	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// FindAllOrphans scans for leftovers from apps that are no longer installed.
func (o *OrphanFinder) FindAllOrphans() ([]Leftover, error) {
	var allLeftovers []Leftover
	knownBundleIDs := make(map[string]bool)

	// First, get all installed app bundle IDs
	appsDirs := []string{"/Applications", filepath.Join(o.pathConfig.HomeDir, "Applications")}
	for _, appsDir := range appsDirs {
		entries, err := os.ReadDir(appsDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".app") {
				continue
			}
			appPath := filepath.Join(appsDir, entry.Name())
			info, err := o.ExtractAppInfo(appPath)
			if err != nil {
				continue
			}
			knownBundleIDs[info.BundleID] = true
		}
	}

	// Scan Library locations for potential orphans
	locations := o.pathConfig.AppLeftoverLocations()
	for _, location := range locations {
		entries, err := os.ReadDir(location)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()

			// Skip system items
			if strings.HasPrefix(name, "com.apple.") {
				continue
			}

			// Check if this looks like an app identifier
			if !o.looksLikeBundleID(name) {
				continue
			}

			// Extract potential bundle ID from the name
			bundleID := o.extractBundleID(name)
			if bundleID == "" {
				continue
			}

			// Check if app is still installed
			if knownBundleIDs[bundleID] {
				continue
			}

			path := filepath.Join(location, name)
			size := o.getSize(path, entry.IsDir())

			allLeftovers = append(allLeftovers, Leftover{
				Path:       path,
				Size:       size,
				Confidence: ConfidenceMedium,
				Location:   filepath.Base(location),
			})
		}
	}

	return allLeftovers, nil
}

func (o *OrphanFinder) looksLikeBundleID(name string) bool {
	// Bundle IDs typically have format: com.company.app or similar
	parts := strings.Split(name, ".")
	return len(parts) >= 2
}

func (o *OrphanFinder) extractBundleID(name string) string {
	// Remove common suffixes like .plist, .savedState, etc.
	suffixes := []string{".plist", ".savedState", ".binarycookies"}
	result := name
	for _, suffix := range suffixes {
		result = strings.TrimSuffix(result, suffix)
	}
	return result
}
