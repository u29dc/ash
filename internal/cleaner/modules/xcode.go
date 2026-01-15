package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// XcodeModule handles Xcode-related cleanup.
type XcodeModule struct {
	BaseModule
	homeDir string
}

// NewXcodeModule creates a new Xcode cleanup module.
func NewXcodeModule() (*XcodeModule, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	devDir := filepath.Join(homeDir, "Library", "Developer")

	m := &XcodeModule{
		BaseModule: BaseModule{
			name:         "Xcode",
			description:  "DerivedData, DeviceSupport, Archives, and Simulators",
			category:     scanner.CategoryXcode,
			riskLevel:    scanner.RiskSafe,
			requiresSudo: false,
			enabled:      true,
		},
		homeDir: homeDir,
	}

	m.paths = []string{
		filepath.Join(devDir, "Xcode", "DerivedData"),
		filepath.Join(devDir, "Xcode", "Archives"),
		filepath.Join(devDir, "Xcode", "iOS DeviceSupport"),
		filepath.Join(devDir, "CoreSimulator", "Devices"),
	}

	return m, nil
}

// Scan discovers Xcode-related files and directories.
func (m *XcodeModule) Scan(ctx context.Context) ([]scanner.Entry, error) {
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
				size = calcDirSize(path)
			}

			// Determine risk level based on path
			risk := m.assessRisk(basePath)

			entries = append(entries, scanner.Entry{
				Path:          path,
				Name:          item.Name(),
				Size:          size,
				ModTime:       info.ModTime(),
				Category:      scanner.CategoryXcode,
				Risk:          risk,
				IsDir:         item.IsDir(),
				IsSymlink:     isSymlink,
				SymlinkTarget: symlinkTarget,
			})
		}
	}

	return entries, nil
}

func (m *XcodeModule) assessRisk(basePath string) scanner.RiskLevel {
	// Archives may contain dSYMs needed for crash symbolication
	if strings.Contains(basePath, "Archives") {
		return scanner.RiskCaution
	}

	// Device support for older iOS versions is safe to delete
	if strings.Contains(basePath, "iOS DeviceSupport") {
		return scanner.RiskSafe
	}

	// DerivedData is always safe
	if strings.Contains(basePath, "DerivedData") {
		return scanner.RiskSafe
	}

	// Simulators may contain user data
	if strings.Contains(basePath, "CoreSimulator") {
		return scanner.RiskCaution
	}

	return scanner.RiskSafe
}
