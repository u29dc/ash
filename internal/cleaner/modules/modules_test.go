package modules_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ash/internal/cleaner/modules"
	"ash/internal/scanner"
)

func TestNewRegistry(t *testing.T) {
	registry, err := modules.NewRegistry()
	require.NoError(t, err)

	mods := registry.Modules()
	assert.NotEmpty(t, mods)

	// Should have at least these modules
	names := make([]string, len(mods))
	for i, m := range mods {
		names[i] = m.Name()
	}

	assert.Contains(t, names, "User Caches")
	assert.Contains(t, names, "Logs")
	assert.Contains(t, names, "Xcode")
	assert.Contains(t, names, "Homebrew")
	assert.Contains(t, names, "Browsers")
	assert.Contains(t, names, "App Leftovers")
}

func TestRegistry_EnableDisable(t *testing.T) {
	registry, err := modules.NewRegistry()
	require.NoError(t, err)

	// All should be enabled by default except App Leftovers
	for _, m := range registry.Modules() {
		if m.Name() == "App Leftovers" {
			assert.False(t, m.IsEnabled(), "Module %s should be disabled by default", m.Name())
			continue
		}
		assert.True(t, m.IsEnabled(), "Module %s should be enabled by default", m.Name())
	}

	// Disable all
	registry.DisableAll()
	for _, m := range registry.Modules() {
		assert.False(t, m.IsEnabled(), "Module %s should be disabled", m.Name())
	}

	// Enable all
	registry.EnableAll()
	for _, m := range registry.Modules() {
		assert.True(t, m.IsEnabled(), "Module %s should be enabled", m.Name())
	}
}

func TestRegistry_EnabledModules(t *testing.T) {
	registry, err := modules.NewRegistry()
	require.NoError(t, err)

	// Disable half
	mods := registry.Modules()
	for i, m := range mods {
		if i%2 == 0 {
			m.SetEnabled(false)
		}
	}

	enabled := registry.EnabledModules()
	for _, m := range enabled {
		assert.True(t, m.IsEnabled())
	}
}

func TestRegistry_ModuleByCategory(t *testing.T) {
	registry, err := modules.NewRegistry()
	require.NoError(t, err)

	// Find caches modules
	cachesMods := registry.ModuleByCategory(scanner.CategoryCaches)
	assert.NotEmpty(t, cachesMods)

	for _, m := range cachesMods {
		assert.Equal(t, scanner.CategoryCaches, m.Category())
	}
}

func TestCachesModule(t *testing.T) {
	mod, err := modules.NewCachesModule()
	require.NoError(t, err)

	assert.Equal(t, "User Caches", mod.Name())
	assert.Equal(t, scanner.CategoryCaches, mod.Category())
	assert.Equal(t, scanner.RiskSafe, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.True(t, mod.IsEnabled())
	assert.NotEmpty(t, mod.Paths())
}

func TestLogsModule(t *testing.T) {
	mod, err := modules.NewLogsModule()
	require.NoError(t, err)

	assert.Equal(t, "Logs", mod.Name())
	assert.Equal(t, scanner.CategoryLogs, mod.Category())
	assert.Equal(t, scanner.RiskSafe, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.True(t, mod.IsEnabled())
	assert.NotEmpty(t, mod.Paths())
}

func TestXcodeModule(t *testing.T) {
	mod, err := modules.NewXcodeModule()
	require.NoError(t, err)

	assert.Equal(t, "Xcode", mod.Name())
	assert.Equal(t, scanner.CategoryXcode, mod.Category())
	assert.Equal(t, scanner.RiskSafe, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.True(t, mod.IsEnabled())
	assert.NotEmpty(t, mod.Paths())

	// Xcode module should have DerivedData path
	paths := mod.Paths()
	hasDerivedData := false
	for _, p := range paths {
		if contains(p, "DerivedData") {
			hasDerivedData = true
			break
		}
	}
	assert.True(t, hasDerivedData, "Xcode module should include DerivedData path")
}

func TestHomebrewModule(t *testing.T) {
	mod, err := modules.NewHomebrewModule()
	require.NoError(t, err)

	assert.Equal(t, "Homebrew", mod.Name())
	assert.Equal(t, scanner.CategoryHomebrew, mod.Category())
	assert.Equal(t, scanner.RiskSafe, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.True(t, mod.IsEnabled())
	assert.NotEmpty(t, mod.Paths())
}

func TestBrowsersModule(t *testing.T) {
	mod, err := modules.NewBrowsersModule()
	require.NoError(t, err)

	assert.Equal(t, "Browsers", mod.Name())
	assert.Equal(t, scanner.CategoryBrowsers, mod.Category())
	assert.Equal(t, scanner.RiskSafe, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.True(t, mod.IsEnabled())
	assert.NotEmpty(t, mod.Paths())
}

func TestAppsModule(t *testing.T) {
	mod, err := modules.NewAppsModule()
	require.NoError(t, err)

	assert.Equal(t, "App Leftovers", mod.Name())
	assert.Equal(t, scanner.CategoryAppData, mod.Category())
	assert.Equal(t, scanner.RiskCaution, mod.RiskLevel())
	assert.False(t, mod.RequiresSudo())
	assert.False(t, mod.IsEnabled()) // Should be disabled by default
	assert.NotEmpty(t, mod.Paths())
}

func TestModule_SetEnabled(t *testing.T) {
	mod, err := modules.NewCachesModule()
	require.NoError(t, err)

	assert.True(t, mod.IsEnabled())

	mod.SetEnabled(false)
	assert.False(t, mod.IsEnabled())

	mod.SetEnabled(true)
	assert.True(t, mod.IsEnabled())
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
