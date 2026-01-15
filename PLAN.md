# ash - macOS Cleanup CLI Implementation Plan

A fast, safe macOS cleanup tool built with Go and Bubble Tea.

## Reference

Global coding standards: https://raw.githubusercontent.com/u29dc/dot/refs/heads/main/agents/AGENTS.md

The AGENTS file contains Bun-TS examples, but general rules on organisation, performance, methodology, and tooling still apply where relevant.

## Project Overview

**Name**: ash (what remains after burning away the unnecessary)
**Language**: Go 1.24+
**TUI Framework**: Bubble Tea + Lip Gloss + Bubbles
**Design**: Grayscale minimal aesthetic

## Motivation

A fast, minimal CleanMyMac replacement as a CLI/TUI. CleanMyMac targets well-documented, regenerable files (caches, logs, derived data) that macOS creates automatically. A CLI can match 90%+ of its value by systematically scanning known-safe locations while implementing strict safety guards. No GUI bloat, no background processes, just a fast tool that does one thing well.

## Why Go

Benchmarks from gdu (Go disk analyzer) show Go performs within 2% of the fastest Rust alternatives on cold cache filesystem operations. For a cleanup tool where disk I/O is the bottleneck, language runtime overhead is negligible.

| Criterion         | Go                            | Rust                  | Zig        | Bun/TS               |
| ----------------- | ----------------------------- | --------------------- | ---------- | -------------------- |
| Filesystem perf   | Excellent                     | Excellent             | Good       | Moderate             |
| TUI ecosystem     | Bubbletea (mature)            | Ratatui (mature)      | Immature   | Ink (React overhead) |
| macOS integration | Good (plist, trash via shell) | Excellent (objc2)     | Manual FFI | Experimental FFI     |
| Dev velocity      | Fast                          | Slow (borrow checker) | Moderate   | Fastest              |
| Binary size       | 3-8 MB                        | 2-5 MB                | <1 MB      | 50-100 MB            |
| Learning curve    | Gentle                        | Steep                 | Moderate   | Immediate            |

**Decision**: Go provides 95%+ of Rust's performance with significantly faster development. The Bubbletea ecosystem (lazygit, glow, Shopify CLI) proves Go can build polished TUIs. fastwalk delivers 4-6x speedup over stdlib for parallel directory traversal.

## CleanMyMac Feature Mapping

### What CleanMyMac Does → What ash Implements

| CleanMyMac Feature           | ash Module                                     | Status       |
| ---------------------------- | ---------------------------------------------- | ------------ |
| System Junk (caches, logs)   | `cleaner/modules/caches.go`, `logs.go`         | Core         |
| Developer Junk (Xcode)       | `cleaner/modules/xcode.go`                     | Core         |
| Homebrew cleanup             | `cleaner/modules/homebrew.go`                  | Core         |
| Browser caches               | `cleaner/modules/browsers.go`                  | Core         |
| App Uninstaller              | `scanner/orphan.go`, `cleaner/modules/apps.go` | Core         |
| Maintenance (DNS, Spotlight) | `maintenance/commands.go`                      | Core         |
| Malware removal              | -                                              | Out of scope |
| Privacy (browser history)    | -                                              | Out of scope |
| Large files finder           | -                                              | Future       |

### Cleanup Targets by Category

**User-level caches and logs (universally safe):**

- `~/Library/Caches/*` — Application caches, often 5-70GB on developer machines
- `~/Library/Logs/*` — Application logs; macOS retains only 4-5 days since Catalina
- `~/Library/Saved Application State/*` — Window positions, session state
- `~/Library/Cookies/*` — Browser cookies and local storage

**System-level caches (require admin):**

- `/Library/Caches/*` — System-wide application caches
- `/private/var/log/*.asl` — Apple System Logger archives
- `/Library/Logs/DiagnosticReports/*` — Crash reports

**Browser caches:**

- Safari: `~/Library/Caches/com.apple.Safari/`
- Chrome: `~/Library/Caches/Google/Chrome/Default/Cache/`
- Firefox: `~/Library/Caches/Firefox/Profiles/[profile]/`

**Developer junk (the 50GB+ goldmine):**

| Directory                                      | Purpose            | Typical Size | Safe?               |
| ---------------------------------------------- | ------------------ | ------------ | ------------------- |
| `~/Library/Developer/Xcode/DerivedData/`       | Build artifacts    | 5-50GB       | Completely safe     |
| `~/Library/Developer/Xcode/Archives/`          | App Store builds   | 500MB-20GB   | Keep current        |
| `~/Library/Developer/Xcode/iOS DeviceSupport/` | Debug symbols      | ~4GB each    | Delete old versions |
| `~/Library/Developer/CoreSimulator/Devices/`   | Simulator data     | 10-30GB      | Safe                |
| `~/Library/Caches/Homebrew/`                   | Downloaded bottles | 2-10GB       | Safe                |

**iOS device backups:**

- `~/Library/Application Support/MobileSync/Backup/` — Can grow to 30-100GB
- Safety rule: never auto-select the most recent backup

### App Leftover Detection

When apps are dragged to Trash, they leave behind 6-12 directories. ash tracks these using the bundle identifier:

```
~/Library/Application Support/[AppName or BundleID]/
~/Library/Preferences/[BundleID].plist
~/Library/Caches/[BundleID]/
~/Library/Containers/[BundleID]/
~/Library/Group Containers/[TeamID].[GroupID]/
~/Library/Saved Application State/[BundleID].savedState/
~/Library/WebKit/[BundleID]/
~/Library/HTTPStorages/[BundleID]/
~/Library/Cookies/[BundleID].binarycookies
~/Library/LaunchAgents/[BundleID].plist
```

**Detection algorithm:**

1. Extract bundleID from app's Info.plist
2. Extract appName (CFBundleName, CFBundleDisplayName)
3. Extract companyName from bundleID prefix
4. Generate search terms: [bundleID, appName, appName.lowercase(), companyName]
5. Scan all Library locations for matches
6. Assign confidence levels: exact bundleID = high, appName = medium, company only = low
7. Auto-select high confidence; show low confidence for manual review

### Maintenance Commands

| Operation               | Command                                                         | Verdict                       |
| ----------------------- | --------------------------------------------------------------- | ----------------------------- |
| DNS cache flush         | `sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder` | Useful                        |
| Spotlight reindex       | `sudo mdutil -E /`                                              | Useful                        |
| Launch Services rebuild | `lsregister -kill -r -domain local -domain user`                | Useful                        |
| Font cache clear        | `sudo atsutil databases -remove`                                | Useful                        |
| RAM purge               | `sudo purge`                                                    | Limited benefit               |
| Periodic scripts        | `sudo periodic daily weekly monthly`                            | Obsolete (removed in Sequoia) |
| Repair permissions      | N/A                                                             | Obsolete (SIP handles this)   |

### Safety Patterns

**Never-delete (hardcoded, no override):**

- `~/Library/Keychains`
- `~/.ssh`, `~/.gnupg`
- `.git` directories
- `*.keychain*`, `*.pem`, `*.key`
- `/System`, `/usr`, `/bin`, `/sbin`
- `com.apple.*` bundle IDs

**Require explicit confirmation:**

- iOS backups (often 30-100GB, irreplaceable if iCloud disabled)
- Xcode Archives (may contain dSYMs for crash symbolication)
- Any item > 1GB
- Application Support folders (may contain user data)

**Always move to Trash, never permanent delete.**

## Architecture

### Directory Structure

```
ash/
├── cmd/
│   └── ash/
│       └── main.go              # Entry point
├── internal/
│   ├── app/
│   │   └── app.go               # Application state machine, commands, messages
│   ├── scanner/
│   │   ├── scanner.go           # Parallel directory scanner
│   │   ├── paths.go             # Safe/dangerous path definitions
│   │   ├── analyzer.go          # Size calculations, categorization
│   │   └── orphan.go            # App leftover detection
│   ├── cleaner/
│   │   ├── cleaner.go           # Deletion orchestration
│   │   ├── trash.go             # macOS Trash integration
│   │   └── modules/
│   │       ├── module.go        # Module interface
│   │       ├── caches.go        # ~/Library/Caches
│   │       ├── logs.go          # Log file cleanup
│   │       ├── xcode.go         # DerivedData, DeviceSupport
│   │       ├── homebrew.go      # Brew cache, old versions
│   │       ├── browsers.go      # Safari, Chrome, Firefox
│   │       └── apps.go          # App leftover removal
│   ├── maintenance/
│   │   └── commands.go          # DNS flush, Spotlight, etc.
│   ├── tui/
│   │   ├── theme.go             # Grayscale design system
│   │   ├── styles.go            # Lip Gloss style definitions
│   │   ├── views/
│   │   │   ├── home.go          # Main dashboard view
│   │   │   ├── scanning.go      # Scanning progress view
│   │   │   ├── results.go       # Results list view
│   │   │   ├── confirm.go       # Deletion confirmation
│   │   │   └── maintenance.go   # Maintenance commands view
│   │   └── components/
│   │       ├── header.go        # App header
│   │       ├── filelist.go      # Scrollable file list
│   │       ├── progress.go      # Progress indicator
│   │       ├── spinner.go       # Custom spinner
│   │       ├── keybinds.go      # Help footer
│   │       └── toast.go         # Status messages
│   ├── config/
│   │   └── config.go            # Configuration loading and defaults
│   └── safety/
│       ├── guards.go            # Never-delete patterns
│       └── permissions.go       # TCC/FDA detection
├── pkg/
│   └── plist/
│       └── plist.go             # Info.plist parsing utilities
├── tests/
│   ├── scanner/
│   │   ├── scanner_test.go
│   │   ├── paths_test.go
│   │   └── orphan_test.go
│   ├── cleaner/
│   │   ├── cleaner_test.go
│   │   └── modules/
│   │       └── modules_test.go  # All module tests
│   ├── config/
│   │   └── config_test.go
│   ├── maintenance/
│   │   └── commands_test.go
│   ├── plist/
│   │   └── plist_test.go
│   ├── safety/
│   │   └── guards_test.go
│   └── testutil/
│       └── fixtures.go          # Test fixtures
├── .golangci.yml                # golangci-lint v2 configuration
├── package.json                 # Bun scripts for build/test/lint
├── go.mod
├── go.sum
├── AGENTS.md                    # Project documentation
├── CLAUDE.md -> AGENTS.md       # Symlink
└── README.md -> AGENTS.md       # Symlink
```

### Go Equivalents of Your Standards

| Your Standard             | Go Equivalent                                  |
| ------------------------- | ---------------------------------------------- |
| Zero `any` types          | Avoid `interface{}`, use generics where needed |
| Strict mode               | `golangci-lint v2` with strict preset          |
| Biome formatting          | `go fmt` (standard formatter)                  |
| Type checking             | Go compiler + `staticcheck`                    |
| Script runner             | Bun via package.json                           |
| Domain-based organization | `internal/` packages by domain                 |
| Comprehensive tests       | Table-driven tests + testify assertions        |

## Design System

### Grayscale Palette

```go
// internal/tui/theme.go
package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
    // Base colors - grayscale only
    Background    lipgloss.Color // #0a0a0a (near black)
    Surface       lipgloss.Color // #171717 (elevated surface)
    SurfaceHover  lipgloss.Color // #262626 (hover state)
    Border        lipgloss.Color // #404040 (subtle borders)
    BorderFocus   lipgloss.Color // #737373 (focused borders)

    // Text hierarchy
    TextPrimary   lipgloss.Color // #fafafa (primary content)
    TextSecondary lipgloss.Color // #a3a3a3 (secondary content)
    TextMuted     lipgloss.Color // #525252 (disabled/hints)

    // Semantic colors (still grayscale-adjacent)
    Success       lipgloss.Color // #22c55e (green - only accent)
    Warning       lipgloss.Color // #f59e0b (amber - only accent)
    Danger        lipgloss.Color // #ef4444 (red - only accent)

    // Selection
    Selected      lipgloss.Color // #262626 (selected row bg)
    Cursor        lipgloss.Color // #fafafa (cursor indicator)
}

var DefaultTheme = Theme{
    Background:    lipgloss.Color("#0a0a0a"),
    Surface:       lipgloss.Color("#171717"),
    SurfaceHover:  lipgloss.Color("#262626"),
    Border:        lipgloss.Color("#404040"),
    BorderFocus:   lipgloss.Color("#737373"),

    TextPrimary:   lipgloss.Color("#fafafa"),
    TextSecondary: lipgloss.Color("#a3a3a3"),
    TextMuted:     lipgloss.Color("#525252"),

    Success:       lipgloss.Color("#22c55e"),
    Warning:       lipgloss.Color("#f59e0b"),
    Danger:        lipgloss.Color("#ef4444"),

    Selected:      lipgloss.Color("#262626"),
    Cursor:        lipgloss.Color("#fafafa"),
}
```

### Style Definitions

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
    // Layout
    App           lipgloss.Style
    Header        lipgloss.Style
    Content       lipgloss.Style
    Footer        lipgloss.Style

    // Components
    Title         lipgloss.Style
    Subtitle      lipgloss.Style

    // List items
    ListItem      lipgloss.Style
    ListItemSelected lipgloss.Style

    // File entries
    FileName      lipgloss.Style
    FileSize      lipgloss.Style
    FilePath      lipgloss.Style

    // Status
    StatusBar     lipgloss.Style
    KeyBind       lipgloss.Style
    KeyBindDesc   lipgloss.Style

    // Indicators
    Spinner       lipgloss.Style
    Progress      lipgloss.Style
    Checkbox      lipgloss.Style
    CheckboxSelected lipgloss.Style
}

func NewStyles(t Theme) Styles {
    return Styles{
        App: lipgloss.NewStyle().
            Background(t.Background).
            Foreground(t.TextPrimary),

        Header: lipgloss.NewStyle().
            Padding(1, 2).
            BorderStyle(lipgloss.NormalBorder()).
            BorderBottom(true).
            BorderForeground(t.Border),

        Title: lipgloss.NewStyle().
            Foreground(t.TextPrimary).
            Bold(true),

        Subtitle: lipgloss.NewStyle().
            Foreground(t.TextSecondary),

        ListItem: lipgloss.NewStyle().
            Padding(0, 2),

        ListItemSelected: lipgloss.NewStyle().
            Padding(0, 2).
            Background(t.Selected).
            Foreground(t.TextPrimary),

        FileName: lipgloss.NewStyle().
            Foreground(t.TextPrimary),

        FileSize: lipgloss.NewStyle().
            Foreground(t.TextSecondary).
            Width(10).
            Align(lipgloss.Right),

        FilePath: lipgloss.NewStyle().
            Foreground(t.TextMuted),

        KeyBind: lipgloss.NewStyle().
            Foreground(t.TextPrimary).
            Background(t.Surface).
            Padding(0, 1),

        KeyBindDesc: lipgloss.NewStyle().
            Foreground(t.TextMuted),

        Checkbox: lipgloss.NewStyle().
            Foreground(t.TextMuted),

        CheckboxSelected: lipgloss.NewStyle().
            Foreground(t.Success),
    }
}
```

## Core Types

### Scanner Types

```go
// internal/scanner/scanner.go
package scanner

import (
    "context"
    "time"
)

type Category string

const (
    CategoryCaches    Category = "caches"
    CategoryLogs      Category = "logs"
    CategoryXcode     Category = "xcode"
    CategoryHomebrew  Category = "homebrew"
    CategoryBrowsers  Category = "browsers"
    CategoryAppData   Category = "app_data"
    CategoryOther     Category = "other"
)

type RiskLevel int

const (
    RiskSafe RiskLevel = iota
    RiskCaution
    RiskDangerous
)

type Entry struct {
    Path       string
    Name       string
    Size       int64
    ModTime    time.Time
    Category   Category
    Risk       RiskLevel
    Selected   bool
    BundleID   string // For app-related entries
}

type ScanResult struct {
    Entries    []Entry
    TotalSize  int64
    TotalCount int
    Duration   time.Duration
    Errors     []ScanError
}

type ScanError struct {
    Path    string
    Message string
    Code    ErrorCode
}

type ErrorCode string

const (
    ErrPermissionDenied ErrorCode = "permission_denied"
    ErrNotFound         ErrorCode = "not_found"
    ErrIOError          ErrorCode = "io_error"
)

type Scanner interface {
    Scan(ctx context.Context, opts ScanOptions) (<-chan Entry, <-chan error)
    Categories() []Category
}

type ScanOptions struct {
    Categories     []Category
    MinSize        int64
    MaxAge         time.Duration
    IncludeHidden  bool
    Parallelism    int
}
```

### Module Interface

```go
// internal/cleaner/modules/module.go
package modules

import (
    "context"

    "nix/internal/scanner"
)

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

type BaseModule struct {
    name        string
    description string
    category    scanner.Category
    paths       []string
    patterns    []string
    riskLevel   scanner.RiskLevel
    requiresSudo bool
    enabled     bool
}

// Default implementations for BaseModule
func (m *BaseModule) Name() string                    { return m.name }
func (m *BaseModule) Description() string             { return m.description }
func (m *BaseModule) Category() scanner.Category      { return m.category }
func (m *BaseModule) Paths() []string                 { return m.paths }
func (m *BaseModule) Patterns() []string              { return m.patterns }
func (m *BaseModule) RiskLevel() scanner.RiskLevel    { return m.riskLevel }
func (m *BaseModule) RequiresSudo() bool              { return m.requiresSudo }
func (m *BaseModule) IsEnabled() bool                 { return m.enabled }
func (m *BaseModule) SetEnabled(v bool)               { m.enabled = v }
```

### Application State

```go
// internal/app/app.go
package app

import (
    "ash/internal/scanner"
    "ash/internal/tui"

    tea "github.com/charmbracelet/bubbletea"
)

type View int

const (
    ViewHome View = iota
    ViewScanning
    ViewResults
    ViewConfirm
    ViewMaintenance
)

type Model struct {
    // View state
    currentView View
    width       int
    height      int

    // Theme and styles
    theme       tui.Theme
    styles      tui.Styles

    // Data
    entries     []scanner.Entry
    selected    map[string]bool
    totalSize   int64

    // Scan state
    scanning    bool
    scanProgress float64
    scanMessage string

    // Components
    list        list.Model
    spinner     spinner.Model
    progress    progress.Model

    // Error state
    lastError   error
}

func New() Model {
    theme := tui.DefaultTheme
    styles := tui.NewStyles(theme)

    return Model{
        currentView: ViewHome,
        theme:       theme,
        styles:      styles,
        selected:    make(map[string]bool),
    }
}

func (m Model) Init() tea.Cmd {
    return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKey(msg)
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil
    case ScanStartedMsg:
        m.scanning = true
        m.currentView = ViewScanning
        return m, nil
    case ScanProgressMsg:
        m.scanProgress = msg.Progress
        m.scanMessage = msg.Message
        return m, nil
    case ScanCompleteMsg:
        m.scanning = false
        m.entries = msg.Entries
        m.totalSize = msg.TotalSize
        m.currentView = ViewResults
        return m, nil
    case ErrorMsg:
        m.lastError = msg.Err
        return m, nil
    }
    return m, nil
}

func (m Model) View() string {
    switch m.currentView {
    case ViewHome:
        return m.renderHome()
    case ViewScanning:
        return m.renderScanning()
    case ViewResults:
        return m.renderResults()
    case ViewConfirm:
        return m.renderConfirm()
    case ViewMaintenance:
        return m.renderMaintenance()
    default:
        return ""
    }
}
```

## Safety Guards

```go
// internal/safety/guards.go
package safety

import (
    "path/filepath"
    "strings"
)

var neverDelete = []string{
    "~/Library/Keychains",
    "~/.ssh",
    "~/.gnupg",
    "~/.config",
    "~/.local/share",
    "/System",
    "/usr",
    "/bin",
    "/sbin",
    "/private/var/vm",
    "/private/var/db",
    "/Applications",
}

var neverDeletePatterns = []string{
    "*.keychain*",
    "*.pem",
    "*.key",
    ".git",
    ".gitignore",
    "id_rsa*",
    "id_ed25519*",
}

var protectedBundleIDs = []string{
    "com.apple.",
    "com.microsoft.",
}

func IsSafePath(path string) bool {
    expanded := expandPath(path)

    // Check never-delete directories
    for _, blocked := range neverDelete {
        blockedExpanded := expandPath(blocked)
        if strings.HasPrefix(expanded, blockedExpanded) {
            return false
        }
    }

    // Check never-delete patterns
    base := filepath.Base(path)
    for _, pattern := range neverDeletePatterns {
        if matched, _ := filepath.Match(pattern, base); matched {
            return false
        }
    }

    return true
}

func IsProtectedApp(bundleID string) bool {
    for _, prefix := range protectedBundleIDs {
        if strings.HasPrefix(bundleID, prefix) {
            return true
        }
    }
    return false
}

func expandPath(path string) string {
    if strings.HasPrefix(path, "~/") {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[2:])
    }
    return path
}
```

## Testing Strategy

### Test Structure

```go
// tests/scanner/scanner_test.go
package scanner_test

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "ash/internal/scanner"
    "ash/tests/testutil"
)

func TestScanner_Scan(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(t *testing.T) string // Returns temp dir
        opts     scanner.ScanOptions
        want     int                        // Expected entry count
        wantSize int64                      // Expected total size
        wantErr  bool
    }{
        {
            name: "scans cache directory",
            setup: func(t *testing.T) string {
                return testutil.CreateTempCacheDir(t, 10, 1024)
            },
            opts: scanner.ScanOptions{
                Categories: []scanner.Category{scanner.CategoryCaches},
            },
            want:     10,
            wantSize: 10240,
        },
        {
            name: "respects min size filter",
            setup: func(t *testing.T) string {
                return testutil.CreateMixedSizeDir(t)
            },
            opts: scanner.ScanOptions{
                Categories: []scanner.Category{scanner.CategoryCaches},
                MinSize:    1024 * 1024, // 1MB
            },
            want: 3, // Only large files
        },
        {
            name: "handles permission denied gracefully",
            setup: func(t *testing.T) string {
                return testutil.CreateRestrictedDir(t)
            },
            opts: scanner.ScanOptions{
                Categories: []scanner.Category{scanner.CategoryCaches},
            },
            wantErr: false, // Should not error, just skip
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := tt.setup(t)
            defer os.RemoveAll(dir)

            s := scanner.New(dir)
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            entries, errs := s.Scan(ctx, tt.opts)

            var result []scanner.Entry
            var scanErrs []error

            for {
                select {
                case e, ok := <-entries:
                    if !ok {
                        entries = nil
                        continue
                    }
                    result = append(result, e)
                case err, ok := <-errs:
                    if !ok {
                        errs = nil
                        continue
                    }
                    scanErrs = append(scanErrs, err)
                }
                if entries == nil && errs == nil {
                    break
                }
            }

            if tt.wantErr {
                require.NotEmpty(t, scanErrs)
                return
            }

            assert.Len(t, result, tt.want)

            var totalSize int64
            for _, e := range result {
                totalSize += e.Size
            }
            if tt.wantSize > 0 {
                assert.Equal(t, tt.wantSize, totalSize)
            }
        })
    }
}
```

### Test Utilities

```go
// tests/testutil/fixtures.go
package testutil

import (
    "os"
    "path/filepath"
    "testing"
)

func CreateTempCacheDir(t *testing.T, fileCount int, fileSize int64) string {
    t.Helper()

    dir, err := os.MkdirTemp("", "ash-test-cache-*")
    if err != nil {
        t.Fatal(err)
    }

    for i := 0; i < fileCount; i++ {
        path := filepath.Join(dir, fmt.Sprintf("cache-%d.dat", i))
        if err := createFile(path, fileSize); err != nil {
            t.Fatal(err)
        }
    }

    return dir
}

func CreateMixedSizeDir(t *testing.T) string {
    t.Helper()

    dir, err := os.MkdirTemp("", "ash-test-mixed-*")
    if err != nil {
        t.Fatal(err)
    }

    sizes := []int64{
        100,           // 100 bytes
        1024,          // 1 KB
        1024 * 100,    // 100 KB
        1024 * 1024,   // 1 MB
        1024 * 1024 * 5, // 5 MB
        1024 * 1024 * 10, // 10 MB
    }

    for i, size := range sizes {
        path := filepath.Join(dir, fmt.Sprintf("file-%d.dat", i))
        if err := createFile(path, size); err != nil {
            t.Fatal(err)
        }
    }

    return dir
}

func createFile(path string, size int64) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()

    if err := f.Truncate(size); err != nil {
        return err
    }

    return nil
}
```

### Safety Guard Tests

```go
// tests/safety/guards_test.go
package safety_test

import (
    "testing"

    "github.com/stretchr/testify/assert"

    "ash/internal/safety"
)

func TestIsSafePath(t *testing.T) {
    tests := []struct {
        name string
        path string
        want bool
    }{
        // Safe paths
        {"cache dir", "~/Library/Caches/com.example.app", true},
        {"log file", "~/Library/Logs/app.log", true},
        {"derived data", "~/Library/Developer/Xcode/DerivedData/Project-abc123", true},
        {"brew cache", "~/Library/Caches/Homebrew", true},

        // Dangerous paths - must return false
        {"keychain", "~/Library/Keychains/login.keychain-db", false},
        {"ssh key", "~/.ssh/id_ed25519", false},
        {"gnupg", "~/.gnupg/private-keys-v1.d", false},
        {"system", "/System/Library/CoreServices", false},
        {"usr", "/usr/bin/bash", false},
        {"git dir", "~/projects/app/.git/objects", false},

        // Edge cases
        {"hidden in cache", "~/Library/Caches/.hidden", true},
        {"nested keychain ref", "~/Library/Caches/keychain-backup", true}, // Name contains keychain but not the actual dir
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := safety.IsSafePath(tt.path)
            assert.Equal(t, tt.want, got, "IsSafePath(%q)", tt.path)
        })
    }
}

func TestIsProtectedApp(t *testing.T) {
    tests := []struct {
        name     string
        bundleID string
        want     bool
    }{
        {"apple app", "com.apple.Safari", true},
        {"apple system", "com.apple.finder", true},
        {"microsoft", "com.microsoft.VSCode", true},
        {"third party", "com.spotify.client", false},
        {"homebrew", "com.homebrew.cask", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := safety.IsProtectedApp(tt.bundleID)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

## Build Configuration

### package.json (Bun Scripts)

```json
{
  "name": "ash",
  "scripts": {
    "dev": "go run ./cmd/ash",
    "build": "go build -ldflags \"-s -w\" -o bin/ash ./cmd/ash",
    "build:release": "CGO_ENABLED=0 go build -ldflags \"-s -w\" -trimpath -o bin/ash ./cmd/ash",
    "test": "go test -race ./...",
    "test:verbose": "go test -v -race ./...",
    "test:coverage": "go test -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html",
    "util:format": "go fmt ./...",
    "util:lint": "golangci-lint run ./...",
    "util:types": "go vet ./...",
    "util:clean": "rm -rf bin/ coverage.out coverage.html",
    "util:check": "STATUS=0; bun run util:format || STATUS=1; bun run util:lint || STATUS=1; bun run util:types || STATUS=1; bun run test || STATUS=1; exit $STATUS",
    "deps": "go mod download && go mod tidy"
  }
}
```

### golangci-lint v2 Configuration

```yaml
# .golangci.yml
version: "2"

run:
  timeout: 5m
  tests: true

linters:
  default: standard
  enable:
    - bodyclose
    - durationcheck
    - errorlint
    - exhaustive
    - goconst
    - gocritic
    - gosec
    - misspell
    - nakedret
    - revive
    - unconvert
    - unparam
    - wastedassign
    - whitespace
  disable:
    - nilerr      # Many intentional nil returns in filepath.Walk callbacks
    - prealloc    # Micro-optimization, not worth the noise

  settings:
    errcheck:
      check-type-assertions: true
      check-blank: true

    govet:
      enable:
        - shadow

    gocritic:
      enabled-tags:
        - diagnostic
        - performance
      disabled-checks:
        - hugeParam  # Bubble Tea API requires value receivers

    gosec:
      excludes:
        - G104  # Unhandled errors (covered by errcheck)
        - G204  # Subprocess with variable (expected for maintenance commands)
        - G301  # Directory permissions 0755 (standard for dirs)
        - G302  # File permissions in test utilities
        - G304  # File inclusion via variable (expected for cleanup tool)
        - G306  # WriteFile permissions 0644 (standard for config files)
        - G115  # Integer conversion (handled explicitly)

    revive:
      rules:
        - name: blank-imports
        - name: context-as-argument
        - name: dot-imports
        - name: error-return
        - name: error-strings
        - name: error-naming
        - name: exported
          disabled: true
        - name: package-comments
          disabled: true
        - name: range
        - name: receiver-naming
        - name: var-declaration
        - name: var-naming

    exhaustive:
      default-signifies-exhaustive: true

  exclusions:
    presets:
      - comments
      - std-error-handling
    rules:
      - path: _test\.go
        linters:
          - goconst
          - gosec
          - errcheck
      - path: tests/
        linters:
          - gosec
          - errcheck
```

## Implementation Phases

**Commit Convention**: All commits must follow commitlint format with body required.

```
type(scope): subject line

Body explaining what changed and why.
```

Types: `feat`, `fix`, `refactor`, `docs`, `style`, `chore`, `test`, `build`, `perf`
Scopes: `core`, `scanner`, `cleaner`, `tui`, `safety`, `config`, `maintenance`, `deps`

Commit frequently after each logical unit of work. Small, atomic commits preferred.

### Phase 1: Foundation

- [ ] Initialize Go module and directory structure
- [ ] Set up tooling (golangci-lint, gofumpt, gotestsum)
- [ ] Create theme and styles (grayscale design system)
- [ ] Implement basic Bubble Tea app shell
- [ ] Set up pre-commit hooks and commitlint

### Phase 2: Scanner Core

- [ ] Implement parallel directory walker using fastwalk
- [ ] Define safe path patterns
- [ ] Create scanner module interface
- [ ] Implement cache module
- [ ] Implement logs module
- [ ] Write comprehensive scanner tests

### Phase 3: TUI Views

- [ ] Home view with category selection
- [ ] Scanning progress view with spinner
- [ ] Results view with scrollable file list
- [ ] Confirmation dialog
- [ ] Keyboard navigation

### Phase 4: Cleanup Modules

- [ ] Xcode module (DerivedData, DeviceSupport, Archives)
- [ ] Homebrew module
- [ ] Browser caches module
- [ ] App leftover detection (orphan finder)
- [ ] Trash integration (move to Trash, not permanent delete)

### Phase 5: Safety and Polish

- [ ] Safety guards implementation
- [ ] Full Disk Access detection
- [ ] Dry-run mode
- [ ] Size formatting and sorting
- [ ] Error handling and recovery
- [ ] Final test coverage

### Phase 6: Maintenance Commands

- [ ] DNS cache flush
- [ ] Spotlight reindex
- [ ] Launch Services rebuild
- [ ] Maintenance view in TUI

### Phase 7: Release

- [ ] goreleaser configuration
- [ ] README documentation
- [ ] Installation script
- [ ] First release

## Dependencies

```go
// go.mod
module ash

go 1.24.2

require (
    github.com/charmbracelet/bubbletea v1.3.10
    github.com/charmbracelet/bubbles v0.21.0
    github.com/charmbracelet/lipgloss v1.1.0
    github.com/charlievieth/fastwalk v1.0.14
    github.com/dustin/go-humanize v1.0.1
    github.com/stretchr/testify v1.10.0
    howett.net/plist v1.0.1
)
```

## Performance Targets

| Operation | Target  | Measurement               |
| --------- | ------- | ------------------------- |
| Startup   | < 50ms  | Cold start to UI render   |
| Full scan | < 3s    | Typical developer machine |
| UI frame  | < 16ms  | 60fps responsiveness      |
| Memory    | < 100MB | Peak during full scan     |

## Quality Gates

Mirrors your TypeScript approach:

1. Zero linter warnings (`bun run util:lint`)
2. Zero vet issues (`bun run util:types`)
3. All tests passing with race detector (`bun run test`)
4. Code formatted (`bun run util:format`)
5. Successful build (`bun run build`)

All enforced via `bun run util:check`.
