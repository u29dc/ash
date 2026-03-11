package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/cleaner/modules"
	"ash/internal/safety"
	"ash/internal/scanner"
)

type fakeModule struct {
	name     string
	category scanner.Category
	entries  []scanner.Entry
	err      error
	enabled  bool
}

func (m *fakeModule) Name() string { return m.name }

func (m *fakeModule) Description() string { return m.name }

func (m *fakeModule) Category() scanner.Category { return m.category }

func (m *fakeModule) Paths() []string { return nil }

func (m *fakeModule) Patterns() []string { return nil }

func (m *fakeModule) Scan(context.Context) ([]scanner.Entry, error) { return m.entries, m.err }

func (m *fakeModule) RiskLevel() scanner.RiskLevel { return scanner.RiskSafe }

func (m *fakeModule) RequiresSudo() bool { return false }

func (m *fakeModule) IsEnabled() bool { return m.enabled }

func (m *fakeModule) SetEnabled(v bool) { m.enabled = v }

var _ modules.Module = (*fakeModule)(nil)

func TestBeginCleanupSkipsAuthorization(t *testing.T) {
	model := New()

	updatedModel, cmd := (&model).beginCleanup([]scanner.Entry{
		{Path: "/tmp/cache.dat", Size: 128},
	})
	require.NotNil(t, cmd)

	updated, ok := updatedModel.(*Model)
	require.True(t, ok)
	assert.True(t, updated.cleaning)
	assert.False(t, updated.authInProgress)
	assert.Equal(t, ViewCleaning, updated.currentView)
}

func TestHandleConfirmResultsFlagsRiskySelection(t *testing.T) {
	model := New()
	model.entries = []scanner.Entry{
		{Path: "/tmp/cache.dat", Size: 128},
		{Path: "/tmp/Library/Application Support/Foo", Size: 1024},
	}
	model.selected = map[string]bool{
		model.entries[1].Path: true,
	}
	model.selectedCount = 1
	model.selectedSize = model.entries[1].Size

	updatedModel, cmd := (&model).handleConfirmResults()
	require.Nil(t, cmd)

	updated, ok := updatedModel.(*Model)
	require.True(t, ok)
	assert.Equal(t, ViewConfirm, updated.currentView)
	assert.False(t, updated.confirmAck)
	require.NotEmpty(t, updated.confirmIssues)
	assert.Contains(t, updated.confirmIssues[0], "Application Support")
}

func TestHandleConfirmCleanupAcknowledgesRiskBeforeCleaning(t *testing.T) {
	model := New()
	model.currentView = ViewConfirm
	model.entries = []scanner.Entry{
		{Path: "/tmp/Library/Application Support/Foo", Size: 1024},
	}
	model.selected = map[string]bool{
		model.entries[0].Path: true,
	}
	model.selectedCount = 1
	model.selectedSize = model.entries[0].Size
	model.prepareCleanupConfirmation()

	updatedModel, cmd := (&model).handleConfirmCleanup()
	require.Nil(t, cmd)

	updated, ok := updatedModel.(*Model)
	require.True(t, ok)
	assert.True(t, updated.confirmAck)
	assert.Equal(t, ViewConfirm, updated.currentView)
	assert.False(t, updated.cleaning)
}

func TestUpdateStoresPartialScanState(t *testing.T) {
	model := New()

	updatedModel, cmd := model.Update(ScanCompleteMsg{
		Entries: []scanner.Entry{
			{Path: "/tmp/cache.dat", Size: 42},
		},
		TotalSize:      42,
		Status:         ScanStatusPartial,
		Issues:         []ScanIssue{{Source: "Logs", Message: "permission denied"}},
		FullDiskAccess: safety.PermissionDenied,
	})
	require.Nil(t, cmd)

	updated, ok := updatedModel.(Model)
	require.True(t, ok)
	assert.Equal(t, ScanStatusPartial, updated.scanStatus)
	assert.Equal(t, safety.PermissionDenied, updated.fullDiskAccess)
	require.Len(t, updated.scanIssues, 1)
	assert.Equal(t, "Logs", updated.scanIssues[0].Source)
	assert.Equal(t, ViewResults, updated.currentView)
}

func TestScanModulesCollectsIssuesAndFdaWarning(t *testing.T) {
	result, err := scanModules(context.Background(), []modules.Module{
		&fakeModule{
			name:     "User Caches",
			category: scanner.CategoryCaches,
			entries: []scanner.Entry{{
				Path:     "/tmp/cache.dat",
				Name:     "cache.dat",
				Size:     64,
				Category: scanner.CategoryCaches,
			}},
			enabled: true,
		},
		&fakeModule{
			name:     "Logs",
			category: scanner.CategoryLogs,
			err:      errors.New("permission denied"),
			enabled:  true,
		},
	}, func() safety.PermissionStatus {
		return safety.PermissionDenied
	})
	require.NoError(t, err)
	assert.Equal(t, ScanStatusPartial, result.Status)
	assert.Equal(t, int64(64), result.TotalSize)
	assert.Len(t, result.Entries, 1)
	assert.Len(t, result.Issues, 2)
	assert.Equal(t, safety.PermissionDenied, result.FullDiskAccess)
}

func TestScanModulesMarksFailedWhenAllModulesFail(t *testing.T) {
	result, err := scanModules(context.Background(), []modules.Module{
		&fakeModule{
			name:     "Logs",
			category: scanner.CategoryLogs,
			err:      errors.New("permission denied"),
			enabled:  true,
		},
	}, func() safety.PermissionStatus {
		return safety.PermissionGranted
	})
	require.NoError(t, err)
	assert.Equal(t, ScanStatusFailed, result.Status)
	assert.Empty(t, result.Entries)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, "Logs", result.Issues[0].Source)
}

func TestRenderConfirmShowsRiskWarnings(t *testing.T) {
	model := New()
	model.selectedCount = 2
	model.selectedSize = safety.SizeConfirmationThreshold + 1
	model.confirmIssues = []string{"Selection includes items larger than 1.0 GiB."}

	rendered := model.renderConfirm()
	assert.Contains(t, rendered, "High-Risk Cleanup")
	assert.Contains(t, rendered, "acknowledge the risks")
	assert.Contains(t, rendered, "items larger than")
}

func TestUpdateHandlesScanStarted(t *testing.T) {
	model := New()

	updatedModel, cmd := model.Update(ScanStartedMsg{})
	require.NotNil(t, cmd)

	updated, ok := updatedModel.(Model)
	require.True(t, ok)
	assert.True(t, updated.scanning)
	assert.Equal(t, ViewScanning, updated.currentView)

	if tickMsg := cmd(); tickMsg != nil {
		assert.NotNil(t, tickMsg)
	}
}
