package modules

import (
	"context"
	"os"
	"path/filepath"

	"ash/internal/scanner"
	"ash/internal/safety"
)

// CachesModule handles user-level cache cleanup.
type CachesModule struct {
	BaseModule
	homeDir string
}

// NewCachesModule creates a new caches cleanup module.
func NewCachesModule() (*CachesModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	m := &CachesModule{
		BaseModule: BaseModule{
			name:         "User Caches",
			description:  "Application caches in ~/Library/Caches",
			category:     scanner.CategoryCaches,
			riskLevel:    scanner.RiskSafe,
			requiresSudo: false,
			enabled:      true,
		},
		homeDir: homeDir,
	}

	m.paths = []string{
		filepath.Join(homeDir, "Library", "Caches"),
	}

	return m, nil
}

// Scan discovers cache files and directories.
func (m *CachesModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
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

			// Skip Homebrew (handled by its own module)
			if item.Name() == "Homebrew" {
				continue
			}

			// Skip browser caches (handled by browsers module)
			browserPrefixes := []string{"com.apple.Safari", "Google", "Firefox", "com.brave.Browser"}
			skip := false
			for _, prefix := range browserPrefixes {
				if item.Name() == prefix || filepath.HasPrefix(item.Name(), prefix) {
					skip = true
					break
				}
			}
			if skip {
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
				Category: scanner.CategoryCaches,
				Risk:     scanner.RiskSafe,
				IsDir:    item.IsDir(),
			})
		}
	}

	return entries, nil
}

func calcDirSize(path string) int64 {
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
