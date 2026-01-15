## 1. Documentation

- **Framework**: `pkg.go.dev/github.com/charmbracelet/bubbletea`, `pkg.go.dev/github.com/charmbracelet/bubbles`, `pkg.go.dev/github.com/charmbracelet/lipgloss`
- **Scanner**: `pkg.go.dev/github.com/charlievieth/fastwalk`
- **DevTools**: `bun.com/docs/llms.txt`, `golangci-lint.run/docs`

## 2. Repository Structure

```
.
├── cmd/ash/
│   └── main.go
├── internal/
│   ├── app/                # Bubble Tea application state
│   ├── cleaner/            # Deletion orchestration
│   │   └── modules/        # Pluggable cleanup modules
│   ├── config/             # User configuration
│   ├── maintenance/        # System maintenance commands
│   ├── safety/             # Path guards and permissions
│   ├── scanner/            # Parallel directory scanner
│   └── tui/                # UI components and views
│       ├── components/
│       └── views/
├── pkg/plist/              # macOS plist utilities
├── tests/                  # Test suites
├── package.json
├── go.mod
└── .golangci.yml
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

- `bun run dev` - Run development build
- `bun run build` - Build binary to `bin/ash`
- `bun run build:release` - Release build (CGO_ENABLED=0, trimpath)
- `bun run test` - Run tests with race detector
- `bun run util:check` - Format, lint, types, test (quality gate)
- `bun run util:format` - Format code
- `bun run util:lint` - Run golangci-lint
- `bun run util:types` - Run go vet
- `bun run util:clean` - Remove build artifacts

## 5. Architecture

- **Scanner** (`internal/scanner/`): Parallel directory traversal using fastwalk, supports categories (caches, logs, xcode, homebrew, browsers, app_data), risk assessment (safe/caution/dangerous)
- **Cleaner** (`internal/cleaner/`): Move files to Trash via osascript (never permanent delete), parallel deletion with semaphore, safety validation before all operations
- **Modules** (`internal/cleaner/modules/`): Pluggable cleanup modules - `CachesModule`, `LogsModule`, `XcodeModule`, `HomebrewModule`, `BrowsersModule`, `AppsModule`
- **Safety** (`internal/safety/`): Protected paths (.ssh, keychains, .git, /System, /Library), bundle ID allowlist, permission checks
- **TUI** (`internal/tui/`): Grayscale design system, Bubble Tea views (home, scanning, results, confirm, cleaning, maintenance)
- **App** (`internal/app/`): State machine with views, key bindings, scan/clean commands

## 6. Cleanup Targets

- **Caches**: `~/Library/Caches/*` (excludes Homebrew, browsers)
- **Logs**: `~/Library/Logs/*`, `/var/log/*` (user-readable)
- **Xcode**: `DerivedData`, `Archives`, `iOS DeviceSupport`
- **Homebrew**: `$(brew --prefix)/Caches`, old package versions
- **Browsers**: Safari, Chrome, Firefox, Brave cache directories
- **App Leftovers**: Orphaned `Application Support`, `Preferences`, `Containers` for uninstalled apps

## 7. Safety Guards

- **Never delete**: `~/.ssh`, `~/Library/Keychains`, `.git` directories, `/System/*`, `/Library/System*`
- **Protected apps**: `com.apple.*`, `com.microsoft.*`, system bundles
- **Trash only**: All deletions move to Trash, never permanent
- **Validation**: Every path checked against guards before clean operation

## 8. Quality

- Quality gate: `bun run util:check` (format, lint, types, test)
- golangci-lint v2 with strict rules: errcheck, govet, staticcheck, unused, exhaustive, gosec, revive, and more
- Tests in `tests/` directory (cleaner, config, maintenance, plist, safety, scanner)
- Commits: Conventional Commits format `type(scope): description`
