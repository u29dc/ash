## 1. Documentation

- **Language**: Zig 0.13+ (latest stable)
- **Reference**: https://ziglang.org/documentation/master/

## 2. Repository Structure

```
.
├── build.zig              # Zig build configuration
├── build.zig.zon          # Package manifest
├── src/
│   ├── main.zig           # Entry point
│   ├── app.zig            # Application state machine
│   ├── config.zig         # Configuration
│   ├── maintenance.zig    # System maintenance commands
│   ├── utils.zig          # Utility functions
│   ├── scanner/           # Directory scanning modules
│   │   ├── scanner.zig    # Core types and interface
│   │   ├── caches.zig     # Cache scanning
│   │   ├── logs.zig       # Log scanning
│   │   ├── xcode.zig      # Xcode data
│   │   ├── homebrew.zig   # Homebrew cache
│   │   ├── browsers.zig   # Browser caches
│   │   └── apps.zig       # App leftovers
│   ├── cleaner/           # Deletion orchestration
│   │   ├── cleaner.zig    # Core cleaner
│   │   └── trash.zig      # macOS Trash integration
│   ├── safety/            # Protection and guards
│   │   ├── guards.zig     # Never-delete patterns
│   │   └── permissions.zig# TCC/FDA detection
│   ├── tui/               # Terminal UI
│   │   ├── ansi.zig       # ANSI escape codes
│   │   ├── terminal.zig   # Raw mode handling
│   │   ├── theme.zig      # Grayscale theme
│   │   ├── render.zig     # Screen rendering
│   │   └── components/    # UI components
│   └── views/             # Screen views
├── PLAN.zig.md            # Implementation plan
└── .archive/              # Original Go implementation (reference)
```

## 3. Stack

| Layer    | Choice | Notes                              |
|----------|--------|------------------------------------|
| Language | Zig 0.13+ | Zero dependencies, manual memory |
| TUI      | Custom | Raw ANSI escape codes             |
| Build    | build.zig | Standard Zig build system       |

## 4. Commands

- `zig build` - Build binary to `zig-out/bin/ash`
- `zig build run` - Build and run
- `zig build test` - Run all tests
- `zig build -Doptimize=ReleaseFast` - Release build
- `zig fmt src/` - Format code

## 5. Architecture

- **Scanner** (`src/scanner/`): Directory traversal with category-based modules (caches, logs, xcode, homebrew, browsers, apps)
- **Cleaner** (`src/cleaner/`): Move files to Trash via osascript, safety validation
- **Safety** (`src/safety/`): Protected paths, bundle ID allowlist, permission checks
- **TUI** (`src/tui/`): Custom terminal rendering with ANSI codes, grayscale theme
- **Views** (`src/views/`): Home, scanning, results, confirm, cleaning, maintenance

## 6. Cleanup Targets

- **Caches**: `~/Library/Caches/*` (excludes Homebrew, browsers)
- **Logs**: `~/Library/Logs/*`
- **Xcode**: `DerivedData`, `Archives`, `iOS DeviceSupport`, `CoreSimulator`
- **Homebrew**: `~/Library/Caches/Homebrew`
- **Browsers**: Safari, Chrome, Firefox, Brave, Edge cache directories

## 7. Safety Guards

- **Never delete**: `~/.ssh`, `~/Library/Keychains`, `.git` directories, `/System/*`
- **Protected apps**: `com.apple.*`, `com.microsoft.*`
- **Trash only**: All deletions move to Trash, never permanent
- **Validation**: Every path checked against guards before clean

## 8. Quality

- Quality gate: `zig build test`
- Format: `zig fmt src/`
- Build modes: Debug, ReleaseSafe, ReleaseFast, ReleaseSmall
- Commits: Conventional Commits format `type(scope): description`
