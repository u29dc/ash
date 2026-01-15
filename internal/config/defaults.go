package config

import "ash/internal/scanner"

// DefaultCategories returns the default categories to scan.
func DefaultCategories() []scanner.Category {
	return []scanner.Category{
		scanner.CategoryCaches,
		scanner.CategoryLogs,
		scanner.CategoryXcode,
		scanner.CategoryHomebrew,
		scanner.CategoryBrowsers,
	}
}

// DefaultMinSize returns the default minimum size filter (0 = no filter).
func DefaultMinSize() int64 {
	return 0
}

// DefaultParallelism returns the default number of parallel operations.
func DefaultParallelism() int {
	return 4
}

// DefaultSortOrder returns the default sort order.
func DefaultSortOrder() string {
	return "size"
}

// ValidSortOrders returns the valid sort order options.
func ValidSortOrders() []string {
	return []string{"size", "name", "date"}
}

// SizeLimitLarge defines what counts as a "large" item requiring confirmation.
func SizeLimitLarge() int64 {
	return 1024 * 1024 * 1024 // 1GB
}
