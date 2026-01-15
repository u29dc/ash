package scanner

import (
	"sort"

	"github.com/dustin/go-humanize"
)

// Analysis contains analyzed scan results.
type Analysis struct {
	Entries        []Entry
	TotalSize      int64
	TotalCount     int
	ByCategory     map[Category]CategorySummary
	TopEntries     []Entry
	SelectedSize   int64
	SelectedCount  int
}

// CategorySummary summarizes entries for a single category.
type CategorySummary struct {
	Category   Category
	Entries    []Entry
	TotalSize  int64
	TotalCount int
}

// Analyzer provides methods for analyzing scan results.
type Analyzer struct{}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// Analyze processes scan results and returns an Analysis.
func (a *Analyzer) Analyze(entries []Entry) *Analysis {
	analysis := &Analysis{
		Entries:    entries,
		ByCategory: make(map[Category]CategorySummary),
	}

	// Calculate totals and group by category
	for _, entry := range entries {
		analysis.TotalSize += entry.Size
		analysis.TotalCount++

		if entry.Selected {
			analysis.SelectedSize += entry.Size
			analysis.SelectedCount++
		}

		summary := analysis.ByCategory[entry.Category]
		summary.Category = entry.Category
		summary.Entries = append(summary.Entries, entry)
		summary.TotalSize += entry.Size
		summary.TotalCount++
		analysis.ByCategory[entry.Category] = summary
	}

	// Get top entries by size
	analysis.TopEntries = a.TopBySize(entries, 10)

	return analysis
}

// TopBySize returns the n largest entries.
func (a *Analyzer) TopBySize(entries []Entry, n int) []Entry {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Size > sorted[j].Size
	})

	if n > len(sorted) {
		n = len(sorted)
	}

	return sorted[:n]
}

// SortBySize sorts entries by size in descending order.
func (a *Analyzer) SortBySize(entries []Entry) []Entry {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Size > sorted[j].Size
	})

	return sorted
}

// SortByName sorts entries by name alphabetically.
func (a *Analyzer) SortByName(entries []Entry) []Entry {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	return sorted
}

// SortByDate sorts entries by modification time, newest first.
func (a *Analyzer) SortByDate(entries []Entry) []Entry {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ModTime.After(sorted[j].ModTime)
	})

	return sorted
}

// FilterByCategory returns entries matching the given category.
func (a *Analyzer) FilterByCategory(entries []Entry, category Category) []Entry {
	var filtered []Entry
	for _, entry := range entries {
		if entry.Category == category {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// FilterByRisk returns entries matching the given risk level.
func (a *Analyzer) FilterByRisk(entries []Entry, risk RiskLevel) []Entry {
	var filtered []Entry
	for _, entry := range entries {
		if entry.Risk == risk {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// FilterSelected returns only selected entries.
func (a *Analyzer) FilterSelected(entries []Entry) []Entry {
	var filtered []Entry
	for _, entry := range entries {
		if entry.Selected {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// FormatSize returns a human-readable size string.
func FormatSize(size int64) string {
	return humanize.IBytes(uint64(size))
}

// CategoryName returns a human-readable category name.
func CategoryName(cat Category) string {
	names := map[Category]string{
		CategoryCaches:   "Caches",
		CategoryLogs:     "Logs",
		CategoryXcode:    "Xcode",
		CategoryHomebrew: "Homebrew",
		CategoryBrowsers: "Browsers",
		CategoryAppData:  "App Data",
		CategoryOther:    "Other",
	}
	if name, ok := names[cat]; ok {
		return name
	}
	return string(cat)
}
