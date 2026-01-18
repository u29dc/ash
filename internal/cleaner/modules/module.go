package modules

import (
	"context"

	"ash/internal/scanner"
)

// Module defines the interface for cleanup modules.
type Module interface {
	// Identification
	Name() string
	Description() string
	Category() scanner.Category

	// Scanning
	Paths() []string
	Patterns() []string
	Scan(ctx context.Context) ([]scanner.Entry, error)

	// Risk assessment
	RiskLevel() scanner.RiskLevel
	RequiresSudo() bool

	// Enabled state
	IsEnabled() bool
	SetEnabled(bool)
}

// BaseModule provides a base implementation for modules.
type BaseModule struct {
	name         string
	description  string
	category     scanner.Category
	paths        []string
	patterns     []string
	riskLevel    scanner.RiskLevel
	requiresSudo bool
	enabled      bool
}

// Name returns the module name.
func (m *BaseModule) Name() string { return m.name }

// Description returns the module description.
func (m *BaseModule) Description() string { return m.description }

// Category returns the module category.
func (m *BaseModule) Category() scanner.Category { return m.category }

// Paths returns the paths this module scans.
func (m *BaseModule) Paths() []string { return m.paths }

// Patterns returns the file patterns this module matches.
func (m *BaseModule) Patterns() []string { return m.patterns }

// RiskLevel returns the risk level for this module.
func (m *BaseModule) RiskLevel() scanner.RiskLevel { return m.riskLevel }

// RequiresSudo returns whether this module needs sudo.
func (m *BaseModule) RequiresSudo() bool { return m.requiresSudo }

// IsEnabled returns whether this module is enabled.
func (m *BaseModule) IsEnabled() bool { return m.enabled }

// SetEnabled sets the enabled state of this module.
func (m *BaseModule) SetEnabled(v bool) { m.enabled = v }

// Registry holds all available cleanup modules.
type Registry struct {
	modules []Module
}

// NewRegistry creates a new module registry with all available modules.
func NewRegistry() (*Registry, error) {
	r := &Registry{
		modules: make([]Module, 0),
	}

	// Register all modules
	caches, err := NewCachesModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, caches)

	logs, err := NewLogsModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, logs)

	xcode, err := NewXcodeModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, xcode)

	homebrew, err := NewHomebrewModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, homebrew)

	browsers, err := NewBrowsersModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, browsers)

	apps, err := NewAppsModule()
	if err != nil {
		return nil, err
	}
	r.modules = append(r.modules, apps)

	return r, nil
}

// Modules returns all registered modules.
func (r *Registry) Modules() []Module {
	return r.modules
}

// EnabledModules returns only enabled modules.
func (r *Registry) EnabledModules() []Module {
	var enabled []Module
	for _, m := range r.modules {
		if m.IsEnabled() {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// ModuleByCategory returns modules matching the given category.
func (r *Registry) ModuleByCategory(cat scanner.Category) []Module {
	var matching []Module
	for _, m := range r.modules {
		if m.Category() == cat {
			matching = append(matching, m)
		}
	}
	return matching
}

// EnableAll enables all modules.
func (r *Registry) EnableAll() {
	for _, m := range r.modules {
		m.SetEnabled(true)
	}
}

// DisableAll disables all modules.
func (r *Registry) DisableAll() {
	for _, m := range r.modules {
		m.SetEnabled(false)
	}
}
