package scanner

import "time"

// Category represents the type of cleanup target.
type Category string

const (
	CategoryCaches   Category = "caches"
	CategoryLogs     Category = "logs"
	CategoryXcode    Category = "xcode"
	CategoryHomebrew Category = "homebrew"
	CategoryBrowsers Category = "browsers"
	CategoryAppData  Category = "app_data"
	CategoryOther    Category = "other"
)

// RiskLevel indicates how safe it is to delete an entry.
type RiskLevel int

const (
	RiskSafe RiskLevel = iota
	RiskCaution
	RiskDangerous
)

// String returns a human-readable string for the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskSafe:
		return "safe"
	case RiskCaution:
		return "caution"
	case RiskDangerous:
		return "dangerous"
	default:
		return "unknown"
	}
}

// Entry represents a single file or directory that can be cleaned.
type Entry struct {
	Path          string
	Name          string
	Size          int64
	ModTime       time.Time
	Category      Category
	Risk          RiskLevel
	Selected      bool
	BundleID      string
	IsDir         bool
	IsSymlink     bool
	SymlinkTarget string
}
