# ash - macOS Cleanup Utility

Two implementations for benchmarking comparison:
- `ash-go/` - Go implementation with Bubble Tea TUI
- `ash-zig/` - Zig implementation with custom ANSI TUI

## Repository Structure

```
.
├── ash-go/                # Go implementation
│   ├── cmd/ash/main.go
│   ├── internal/
│   ├── pkg/
│   ├── tests/
│   ├── go.mod
│   └── Makefile
├── ash-zig/               # Zig implementation
│   ├── src/
│   ├── build.zig
│   └── build.zig.zon
├── PLAN.md                # Original Go implementation plan
└── AGENTS.md              # This file
```

---

## ash-go (Go Implementation)

### Stack
| Layer | Choice | Notes |
|-------|--------|-------|
| Language | Go 1.24+ | With race detector for tests |
| TUI | Bubble Tea | Elm architecture, grayscale theme |
| Scanner | fastwalk | 4-6x faster than stdlib |

### Commands
```bash
cd ash-go
bun run dev           # Run development build
bun run build         # Build binary
bun run test          # Run tests
bun run util:check    # Format, lint, test
```

---

## ash-zig (Zig Implementation)

### Stack
| Layer | Choice | Notes |
|-------|--------|-------|
| Language | Zig 0.13+ | Zero dependencies |
| TUI | Custom | Raw ANSI escape codes |
| Build | build.zig | Standard Zig build system |

### Commands
```bash
cd ash-zig
zig build                          # Build binary
zig build run                      # Build and run
zig build test                     # Run tests
zig build -Doptimize=ReleaseFast   # Release build
zig fmt src/                       # Format code
```

---

## Feature Parity

Both implementations provide identical functionality:

### Cleanup Targets
- **Caches**: `~/Library/Caches/*` (excludes Homebrew, browsers)
- **Logs**: `~/Library/Logs/*`
- **Xcode**: `DerivedData`, `Archives`, `iOS DeviceSupport`, `CoreSimulator`
- **Homebrew**: `~/Library/Caches/Homebrew`
- **Browsers**: Safari, Chrome, Firefox, Brave, Edge cache directories

### Safety Guards
- **Never delete**: `~/.ssh`, `~/Library/Keychains`, `.git` directories, `/System/*`
- **Protected apps**: `com.apple.*`, `com.microsoft.*`
- **Trash only**: All deletions move to Trash, never permanent
- **Validation**: Every path checked against guards before clean

### TUI Views
- Home menu
- Scanning progress
- Results with selection
- Confirmation dialog
- Cleaning progress
- Maintenance commands

---

## Benchmarking

Build both versions and compare:

```bash
# Build Go version
cd ash-go && go build -o ../bin/ash-go ./cmd/ash && cd ..

# Build Zig version
cd ash-zig && zig build -Doptimize=ReleaseFast && cp zig-out/bin/ash ../bin/ash-zig && cd ..

# Compare binary sizes
ls -lh bin/

# Benchmark startup time
hyperfine './bin/ash-go --help' './bin/ash-zig --help'

# Benchmark scan performance
hyperfine './bin/ash-go scan' './bin/ash-zig scan'
```

## Commits

Conventional Commits format: `type(scope): description`
