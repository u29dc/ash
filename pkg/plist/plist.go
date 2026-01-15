package plist

import (
	"os"

	"howett.net/plist"
)

// AppInfo contains information extracted from an app's Info.plist.
type AppInfo struct {
	BundleIdentifier string `plist:"CFBundleIdentifier"`
	BundleName       string `plist:"CFBundleName"`
	DisplayName      string `plist:"CFBundleDisplayName"`
	Version          string `plist:"CFBundleShortVersionString"`
	BuildNumber      string `plist:"CFBundleVersion"`
	Executable       string `plist:"CFBundleExecutable"`
	IconFile         string `plist:"CFBundleIconFile"`
	MinOSVersion     string `plist:"LSMinimumSystemVersion"`
}

// ReadAppInfo reads app information from an Info.plist file.
func ReadAppInfo(path string) (*AppInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var info AppInfo
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// ReadPlist reads a plist file into the provided struct.
func ReadPlist(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = plist.Unmarshal(data, v)
	return err
}

// WritePlist writes a struct to a plist file.
func WritePlist(path string, v interface{}) error {
	data, err := plist.MarshalIndent(v, plist.XMLFormat, "\t")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetBundleID extracts the bundle identifier from an Info.plist.
func GetBundleID(plistPath string) (string, error) {
	info, err := ReadAppInfo(plistPath)
	if err != nil {
		return "", err
	}
	return info.BundleIdentifier, nil
}

// GetAppName extracts the app name from an Info.plist.
// It prefers DisplayName over BundleName.
func GetAppName(plistPath string) (string, error) {
	info, err := ReadAppInfo(plistPath)
	if err != nil {
		return "", err
	}

	if info.DisplayName != "" {
		return info.DisplayName, nil
	}
	return info.BundleName, nil
}
