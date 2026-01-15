package modules

import (
	"context"
	"os"
	"path/filepath"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// LogsModule handles log file cleanup.
type LogsModule struct {
	BaseModule
	homeDir string
}

// NewLogsModule creates a new logs cleanup module.
func NewLogsModule() (*LogsModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	m := &LogsModule{
		BaseModule: BaseModule{
			name:         "Logs",
			description:  "Application logs in ~/Library/Logs",
			category:     scanner.CategoryLogs,
			riskLevel:    scanner.RiskSafe,
			requiresSudo: false,
			enabled:      true,
		},
		homeDir: homeDir,
	}

	m.paths = []string{
		filepath.Join(homeDir, "Library", "Logs"),
	}

	m.patterns = []string{
		"*.log",
		"*.log.*",
		"*.asl",
	}

	return m, nil
}

// Scan discovers log files and directories.
func (m *LogsModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
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
				Category: scanner.CategoryLogs,
				Risk:     scanner.RiskSafe,
				IsDir:    item.IsDir(),
			})
		}
	}

	return entries, nil
}
