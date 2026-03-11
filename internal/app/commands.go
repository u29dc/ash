package app

import (
	"context"
	"errors"
	"os/exec"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ash/internal/cleaner"
	"ash/internal/cleaner/modules"
	"ash/internal/config"
	"ash/internal/maintenance"
	"ash/internal/safety"
	"ash/internal/scanner"
)

const defaultModuleScanParallelism = 4

type moduleScanOutput struct {
	entries []scanner.Entry
	err     error
	name    string
}

// ModuleScanResult contains the aggregated result of scanning enabled modules.
type ModuleScanResult struct {
	Entries        []scanner.Entry
	TotalSize      int64
	Status         ScanStatus
	Issues         []ScanIssue
	FullDiskAccess safety.PermissionStatus
}

// RunModuleScan scans enabled modules and returns a complete or partial result.
func RunModuleScan(ctx context.Context, includeAppData bool) (*ModuleScanResult, error) {
	registry, err := modules.NewRegistry()
	if err != nil {
		return nil, err
	}
	if _, err := config.Load(); err != nil {
		return nil, err
	}

	for _, mod := range registry.Modules() {
		if mod.Category() != scanner.CategoryAppData {
			continue
		}
		mod.SetEnabled(includeAppData)
	}

	return scanModules(ctx, registry.EnabledModules(), safety.CheckFullDiskAccess)
}

func scanModules(
	ctx context.Context,
	enabled []modules.Module,
	fdaCheck func() safety.PermissionStatus,
) (*ModuleScanResult, error) {
	result := newModuleScanResult(fdaCheck)
	if len(enabled) == 0 {
		return result, nil
	}

	outputs, err := collectModuleOutputs(ctx, enabled)
	if err != nil {
		return nil, err
	}

	for _, output := range outputs {
		if err := result.applyModuleOutput(output); err != nil {
			return nil, err
		}
	}

	if len(result.Issues) > 0 && len(result.Entries) == 0 {
		result.Status = ScanStatusFailed
	}
	sort.Slice(result.Issues, func(i, j int) bool {
		return result.Issues[i].Source < result.Issues[j].Source
	})

	return result, nil
}

func newModuleScanResult(fdaCheck func() safety.PermissionStatus) *ModuleScanResult {
	result := &ModuleScanResult{
		Entries: make([]scanner.Entry, 0, 256),
		Status:  ScanStatusComplete,
	}

	if fdaCheck == nil {
		return result
	}

	result.FullDiskAccess = fdaCheck()
	if result.FullDiskAccess == safety.PermissionDenied {
		result.Status = ScanStatusPartial
		result.Issues = append(result.Issues, ScanIssue{
			Source:  "full disk access",
			Message: "not granted; some directories may be skipped",
		})
	}

	return result
}

func collectModuleOutputs(ctx context.Context, enabled []modules.Module) ([]moduleScanOutput, error) {
	outputs := make([]moduleScanOutput, len(enabled))
	parallelism := minInt(moduleScanParallelism(), len(enabled))
	sem := make(chan struct{}, parallelism)
	var wg sync.WaitGroup

	for i, mod := range enabled {
		wg.Add(1)
		go func(index int, module modules.Module) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				outputs[index] = moduleScanOutput{
					err:  ctx.Err(),
					name: module.Name(),
				}
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			entries, err := module.Scan(ctx)
			outputs[index] = moduleScanOutput{
				entries: entries,
				err:     err,
				name:    module.Name(),
			}
		}(i, mod)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return outputs, nil
}

func (r *ModuleScanResult) applyModuleOutput(output moduleScanOutput) error {
	if output.err != nil {
		if errors.Is(output.err, context.Canceled) {
			return output.err
		}
		r.Status = ScanStatusPartial
		r.Issues = append(r.Issues, ScanIssue{
			Source:  output.name,
			Message: output.err.Error(),
		})
		return nil
	}

	for i := range output.entries {
		r.Entries = append(r.Entries, output.entries[i])
		r.TotalSize += output.entries[i].Size
	}

	return nil
}

func moduleScanParallelism() int {
	parallelism := defaultModuleScanParallelism
	cfg, err := config.Load()
	if err == nil && cfg.Parallelism > 0 {
		parallelism = cfg.Parallelism
	}
	if parallelism < 1 {
		return 1
	}
	return parallelism
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// StartModuleScan returns a command that scans using cleanup modules.
func StartModuleScan(ctx context.Context, includeAppData bool) tea.Cmd {
	return func() tea.Msg {
		result, err := RunModuleScan(ctx, includeAppData)
		if err != nil {
			return ScanErrorMsg{Err: err}
		}

		return ScanCompleteMsg{
			Entries:        result.Entries,
			TotalSize:      result.TotalSize,
			Status:         result.Status,
			Issues:         result.Issues,
			FullDiskAccess: result.FullDiskAccess,
		}
	}
}

// StartClean returns a command that starts the cleaning process.
func StartClean(ctx context.Context, entries []scanner.Entry, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		c := cleaner.New(cleaner.WithDryRun(dryRun))

		stats, err := c.Clean(ctx, entries)
		if err != nil {
			return CleanErrorMsg{Err: err}
		}

		return CleanCompleteMsg{Stats: stats}
	}
}

// RequestAuth requests authorization once for privileged operations.
func RequestAuth(ctx context.Context) tea.Cmd {
	cmd := exec.CommandContext(ctx, "/usr/bin/sudo", "-v")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return AuthErrorMsg{Err: err}
		}
		return AuthSuccessMsg{}
	})
}

// RunMaintenanceCommand returns a command that runs a maintenance operation.
func RunMaintenanceCommand(ctx context.Context, cmd *maintenance.Command) tea.Cmd {
	return func() tea.Msg {
		result := maintenance.Run(ctx, cmd)
		return MaintenanceCompleteMsg{Result: result}
	}
}

// Tick returns a command that sends a tick message after the given duration.
func Tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// DoNothing returns an empty command.
func DoNothing() tea.Cmd {
	return nil
}
