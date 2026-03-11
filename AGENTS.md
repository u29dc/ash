## 1. Documentation

- **Framework**: `pkg.go.dev/github.com/charmbracelet/bubbletea`, `pkg.go.dev/github.com/charmbracelet/bubbles`, `pkg.go.dev/github.com/charmbracelet/lipgloss`
- **Scanner**: `pkg.go.dev/github.com/charlievieth/fastwalk`
- **DevTools**: `golangci-lint.run/docs`

## 2. Repository Structure

```
.
├── cmd/ash/main.go        # Entry point
├── internal/
│   ├── app/               # Bubble Tea application state
│   ├── cleaner/           # Deletion orchestration
│   │   └── modules/       # Pluggable cleanup modules
│   ├── config/            # User configuration
│   ├── maintenance/       # System maintenance commands
│   ├── safety/            # Path guards and permissions
│   ├── scanner/           # Shared scanner types, analyzer, paths, orphan finder
│   ├── testutil/          # Test fixtures and helpers
│   └── tui/               # Shared TUI theme and styles
├── pkg/plist/             # macOS plist utilities
├── Makefile               # Make targets for build/test
├── package.json           # Bun script runner
├── go.mod
├── .golangci.yml
└── .goreleaser.yml        # Release automation
```

## 3. Stack

| Layer | Choice | Notes |
|-------|--------|-------|
| Language | Go 1.24+ | With race detector for tests |
| TUI | Bubble Tea | Elm architecture, grayscale theme |
| Scanner | fastwalk | 4-6x faster than stdlib filepath.Walk |
| Plist | howett.net/plist | macOS property list parsing |
| Linting | golangci-lint v2 | Strict configuration in `.golangci.yml` |
| Runtime | Bun | Script runner via package.json |

## 4. Commands

**Development:**
- `bun run dev` - Run development build
- `bun run ash` - Run built binary from `${ASH_HOME:-${TOOLS_HOME:-$HOME/.tools}/ash}/ash`

**Build:**
- `bun run build` - Build binary to `${ASH_HOME:-${TOOLS_HOME:-$HOME/.tools}/ash}/ash`
- `bun run build:release` - Release build to `${ASH_HOME:-${TOOLS_HOME:-$HOME/.tools}/ash}/ash` (CGO_ENABLED=0, trimpath)

**Test:**
- `bun run test` - Run tests with race detector
- `bun run test:verbose` - Run tests with verbose output
- `bun run test:coverage` - Run tests and generate coverage report

**Quality:**
- `bun run util:check` - Format, lint, types, test, build (quality gate)
- `bun run util:format` - Format code
- `bun run util:lint` - Run golangci-lint
- `bun run util:types` - Run go vet
- `bun run util:clean` - Remove build artifacts

**Misc:**
- `bun run deps` - Download and tidy dependencies

**Make targets (alternative):**
- `make check` - Run all quality checks
- `make coverage` - Generate coverage HTML report
- `make install` - Install binary to `${ASH_HOME:-${TOOLS_HOME:-$HOME/.tools}/ash}`

## 5. Architecture

- **Scanner** (`internal/scanner/`): Shared scanner types, analyzers, path config, and the confidence-based orphan finder used by deep scan
- **Cleaner** (`internal/cleaner/`): Move files to Trash via direct filesystem operations (never permanent delete), parallel deletion with semaphore, safety validation before all operations
- **Modules** (`internal/cleaner/modules/`): Pluggable cleanup modules - `CachesModule`, `LogsModule`, `XcodeModule`, `HomebrewModule`, `BrowsersModule`, `AppsModule`
- **Safety** (`internal/safety/`): Protected paths (.ssh, keychains, .git, /System, /Library), bundle ID allowlist, permission checks
- **TUI** (`internal/tui/`): Shared grayscale theme and style helpers
- **App** (`internal/app/`): Bubble Tea state machine, scan orchestration, result/error surfacing, confirm flow, and cleanup/maintenance commands
- **Testutil** (`internal/testutil/`): Test fixtures and helper functions for unit tests

## 6. Cleanup Targets

- **Caches**: `~/Library/Caches/*` (excludes Homebrew, browsers)
- **Logs**: `~/Library/Logs/*`, `/var/log/*` (user-readable)
- **Xcode**: `DerivedData`, `Archives`, `iOS DeviceSupport`
- **Homebrew**: `~/Library/Caches/Homebrew` cache data; `brew cleanup` helpers exist for old package versions
- **Browsers**: Safari, Chrome, Firefox, Brave cache directories
- **App Leftovers**: Confidence-based `Application Support`, `Preferences`, `Containers`, and `Group Containers` leftovers for uninstalled apps

## 7. Safety Guards

- **Never delete**: `~/.ssh`, `~/Library/Keychains`, `.git` directories, `/System/*`, `/Library/System*`
- **Protected apps**: `com.apple.*`, `com.microsoft.*`, system bundles
- **Trash only**: All deletions move to Trash, never permanent
- **Validation**: Every path checked against guards before clean operation

## 8. Quality

- Quality gate: `bun run util:check` (format, lint, types, test, build)
- golangci-lint v2 with strict rules: errcheck, govet, staticcheck, unused, exhaustive, gosec, revive
- Tests colocated with source files (`*_test.go`), test helpers in `internal/testutil/`
- Commits: Conventional Commits format `type(scope): description`
