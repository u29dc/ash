package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charlievieth/fastwalk"
)

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
	Path     string
	Name     string
	Size     int64
	ModTime  time.Time
	Category Category
	Risk     RiskLevel
	Selected bool
	BundleID string // For app-related entries
	IsDir    bool
}

// ScanResult contains the results of a scan operation.
type ScanResult struct {
	Entries    []Entry
	TotalSize  int64
	TotalCount int
	Duration   time.Duration
	Errors     []ScanError
}

// ScanError represents an error that occurred during scanning.
type ScanError struct {
	Path    string
	Message string
	Code    ErrorCode
}

// ErrorCode categorizes the type of scan error.
type ErrorCode string

const (
	ErrPermissionDenied ErrorCode = "permission_denied"
	ErrNotFound         ErrorCode = "not_found"
	ErrIOError          ErrorCode = "io_error"
)

// ScanOptions configures the scanning behavior.
type ScanOptions struct {
	Categories    []Category
	MinSize       int64
	MaxAge        time.Duration
	IncludeHidden bool
	Parallelism   int
}

// Scanner defines the interface for scanning directories.
type Scanner interface {
	Scan(ctx context.Context, opts ScanOptions) (<-chan Entry, <-chan error)
	Categories() []Category
}

// DefaultScanner implements the Scanner interface using fastwalk.
type DefaultScanner struct {
	basePaths map[Category][]string
	homeDir   string
}

// New creates a new DefaultScanner.
func New() (*DefaultScanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	s := &DefaultScanner{
		homeDir:   homeDir,
		basePaths: make(map[Category][]string),
	}
	s.initPaths()
	return s, nil
}

func (s *DefaultScanner) initPaths() {
	lib := filepath.Join(s.homeDir, "Library")

	s.basePaths[CategoryCaches] = []string{
		filepath.Join(lib, "Caches"),
	}

	s.basePaths[CategoryLogs] = []string{
		filepath.Join(lib, "Logs"),
	}

	s.basePaths[CategoryXcode] = []string{
		filepath.Join(lib, "Developer", "Xcode", "DerivedData"),
		filepath.Join(lib, "Developer", "Xcode", "Archives"),
		filepath.Join(lib, "Developer", "Xcode", "iOS DeviceSupport"),
		filepath.Join(lib, "Developer", "CoreSimulator", "Devices"),
	}

	s.basePaths[CategoryHomebrew] = []string{
		filepath.Join(lib, "Caches", "Homebrew"),
	}

	s.basePaths[CategoryBrowsers] = []string{
		filepath.Join(lib, "Caches", "com.apple.Safari"),
		filepath.Join(lib, "Caches", "Google", "Chrome"),
		filepath.Join(lib, "Caches", "Firefox"),
	}
}

// Categories returns all available scan categories.
func (s *DefaultScanner) Categories() []Category {
	return []Category{
		CategoryCaches,
		CategoryLogs,
		CategoryXcode,
		CategoryHomebrew,
		CategoryBrowsers,
	}
}

// Scan performs a parallel scan of the specified categories.
func (s *DefaultScanner) Scan(ctx context.Context, opts ScanOptions) (<-chan Entry, <-chan error) {
	entries := make(chan Entry, 100)
	errs := make(chan error, 10)

	go func() {
		defer close(entries)
		defer close(errs)

		var wg sync.WaitGroup
		categories := opts.Categories
		if len(categories) == 0 {
			categories = s.Categories()
		}

		for _, cat := range categories {
			paths, ok := s.basePaths[cat]
			if !ok {
				continue
			}

			for _, basePath := range paths {
				wg.Add(1)
				go func(path string, category Category) {
					defer wg.Done()
					s.scanPath(ctx, path, category, opts, entries, errs)
				}(basePath, cat)
			}
		}

		wg.Wait()
	}()

	return entries, errs
}

func (s *DefaultScanner) scanPath(
	ctx context.Context,
	basePath string,
	category Category,
	opts ScanOptions,
	entries chan<- Entry,
	errs chan<- error,
) {
	info, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		select {
		case errs <- err:
		case <-ctx.Done():
		}
		return
	}

	if !info.IsDir() {
		s.processFile(ctx, basePath, info, category, opts, entries)
		return
	}

	conf := fastwalk.Config{
		Follow: false,
	}

	walkFn := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil // Skip permission denied
			}
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !opts.IncludeHidden && len(d.Name()) > 0 && d.Name()[0] == '.' {
			if d.IsDir() {
				return fastwalk.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil // Skip files we can't stat
		}

		s.processFile(ctx, path, info, category, opts, entries)
		return nil
	}

	if err := fastwalk.Walk(&conf, basePath, walkFn); err != nil && !errors.Is(err, context.Canceled) {
		select {
		case errs <- err:
		case <-ctx.Done():
		}
	}
}

func (s *DefaultScanner) processFile(
	ctx context.Context,
	path string,
	info os.FileInfo,
	category Category,
	opts ScanOptions,
	entries chan<- Entry,
) {
	size := info.Size()
	if info.IsDir() {
		// For directories, calculate total size
		size = s.calcDirSize(path)
	}

	if opts.MinSize > 0 && size < opts.MinSize {
		return
	}

	if opts.MaxAge > 0 && time.Since(info.ModTime()) < opts.MaxAge {
		return
	}

	entry := Entry{
		Path:     path,
		Name:     info.Name(),
		Size:     size,
		ModTime:  info.ModTime(),
		Category: category,
		Risk:     s.assessRisk(path, category),
		IsDir:    info.IsDir(),
	}

	select {
	case entries <- entry:
	case <-ctx.Done():
	}
}

func (s *DefaultScanner) calcDirSize(path string) int64 {
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

func (s *DefaultScanner) assessRisk(path string, category Category) RiskLevel {
	switch category {
	case CategoryCaches, CategoryLogs:
		return RiskSafe
	case CategoryXcode:
		if filepath.Base(filepath.Dir(path)) == "Archives" {
			return RiskCaution
		}
		return RiskSafe
	case CategoryHomebrew:
		return RiskSafe
	case CategoryBrowsers:
		return RiskSafe
	default:
		return RiskCaution
	}
}

// ScanAll performs a full scan of all categories and returns a ScanResult.
func (s *DefaultScanner) ScanAll(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{
		Entries: make([]Entry, 0),
		Errors:  make([]ScanError, 0),
	}

	entries, errs := s.Scan(ctx, opts)

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				entries = nil
				continue
			}
			result.Entries = append(result.Entries, entry)
			result.TotalSize += entry.Size
			result.TotalCount++
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			result.Errors = append(result.Errors, ScanError{
				Message: err.Error(),
				Code:    ErrIOError,
			})
		}

		if entries == nil && errs == nil {
			break
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}
