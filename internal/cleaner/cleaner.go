package cleaner

import (
	"context"
	"errors"
	"sync"

	"ash/internal/safety"
	"ash/internal/scanner"
)

// CleanResult represents the result of a cleanup operation.
type CleanResult struct {
	Path    string
	Success bool
	Error   error
	Size    int64
}

// CleanStats contains statistics about a cleanup operation.
type CleanStats struct {
	TotalCount   int
	SuccessCount int
	FailedCount  int
	TotalSize    int64
	CleanedSize  int64
	Errors       []CleanResult
}

// Cleaner handles the deletion of files and directories.
type Cleaner struct {
	dryRun      bool
	useTrash    bool
	parallelism int
}

// Option configures the Cleaner.
type Option func(*Cleaner)

// WithDryRun sets whether to perform a dry run (no actual deletion).
func WithDryRun(dryRun bool) Option {
	return func(c *Cleaner) {
		c.dryRun = dryRun
	}
}

// WithTrash sets whether to move files to Trash instead of permanent deletion.
func WithTrash(useTrash bool) Option {
	return func(c *Cleaner) {
		c.useTrash = useTrash
	}
}

// WithParallelism sets the number of parallel deletion operations.
func WithParallelism(n int) Option {
	return func(c *Cleaner) {
		c.parallelism = n
	}
}

// New creates a new Cleaner with the given options.
func New(opts ...Option) *Cleaner {
	c := &Cleaner{
		dryRun:      false,
		useTrash:    true, // Default to using Trash for safety
		parallelism: 4,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Clean removes the specified entries.
func (c *Cleaner) Clean(ctx context.Context, entries []scanner.Entry) (*CleanStats, error) {
	stats := &CleanStats{
		TotalCount: len(entries),
	}

	if len(entries) == 0 {
		return stats, nil
	}

	// Validate all paths first
	for _, entry := range entries {
		if !safety.IsSafePath(entry.Path) {
			return nil, errors.New("attempted to delete protected path: " + entry.Path)
		}
		stats.TotalSize += entry.Size
	}

	// Process entries
	results := make(chan CleanResult, len(entries))
	sem := make(chan struct{}, c.parallelism)
	var wg sync.WaitGroup

	for _, entry := range entries {
		wg.Add(1)
		go func(e scanner.Entry) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				results <- CleanResult{
					Path:    e.Path,
					Success: false,
					Error:   ctx.Err(),
					Size:    e.Size,
				}
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			var err error
			if !c.dryRun {
				if c.useTrash {
					err = MoveToTrash(e.Path)
				} else {
					err = permanentDelete(e.Path)
				}
			}

			results <- CleanResult{
				Path:    e.Path,
				Success: err == nil,
				Error:   err,
				Size:    e.Size,
			}
		}(entry)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		if result.Success {
			stats.SuccessCount++
			stats.CleanedSize += result.Size
		} else {
			stats.FailedCount++
			stats.Errors = append(stats.Errors, result)
		}
	}

	return stats, nil
}

// CleanSingle removes a single entry.
func (c *Cleaner) CleanSingle(ctx context.Context, entry scanner.Entry) (*CleanResult, error) {
	if !safety.IsSafePath(entry.Path) {
		return nil, errors.New("attempted to delete protected path: " + entry.Path)
	}

	select {
	case <-ctx.Done():
		return &CleanResult{
			Path:    entry.Path,
			Success: false,
			Error:   ctx.Err(),
			Size:    entry.Size,
		}, ctx.Err()
	default:
	}

	var err error
	if !c.dryRun {
		if c.useTrash {
			err = MoveToTrash(entry.Path)
		} else {
			err = permanentDelete(entry.Path)
		}
	}

	return &CleanResult{
		Path:    entry.Path,
		Success: err == nil,
		Error:   err,
		Size:    entry.Size,
	}, err
}

// Preview returns what would be cleaned without actually cleaning.
func (c *Cleaner) Preview(entries []scanner.Entry) (*CleanStats, error) {
	stats := &CleanStats{
		TotalCount: len(entries),
	}

	for _, entry := range entries {
		if !safety.IsSafePath(entry.Path) {
			stats.FailedCount++
			stats.Errors = append(stats.Errors, CleanResult{
				Path:    entry.Path,
				Success: false,
				Error:   errors.New("protected path"),
				Size:    entry.Size,
			})
			continue
		}

		stats.SuccessCount++
		stats.TotalSize += entry.Size
		stats.CleanedSize += entry.Size
	}

	return stats, nil
}

// IsDryRun returns whether the cleaner is in dry-run mode.
func (c *Cleaner) IsDryRun() bool {
	return c.dryRun
}

// UsesTrash returns whether the cleaner moves files to Trash.
func (c *Cleaner) UsesTrash() bool {
	return c.useTrash
}
