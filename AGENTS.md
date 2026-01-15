# ash - macOS Cleanup Utility

Two implementations for benchmarking comparison.

## Implementations

| Folder | Language | TUI Framework | Dependencies |
|--------|----------|---------------|--------------|
| `ash-go/` | Go 1.24+ | Bubble Tea | 6 external |
| `ash-zig/` | Zig 0.13+ | Custom ANSI | Zero |

Each folder has its own `AGENTS.md`, `PLAN.md`, and `package.json`.

## Quick Start

**Go version:**
```bash
cd ash-go
bun run build
./bin/ash
```

**Zig version:**
```bash
cd ash-zig
bun run build:release
./zig-out/bin/ash
```

## Benchmarking

```bash
# Build both
cd ash-go && bun run build:release && cd ..
cd ash-zig && bun run build:release && cd ..

# Compare binary sizes
ls -lh ash-go/bin/ash ash-zig/zig-out/bin/ash

# Benchmark startup
hyperfine 'ash-go/bin/ash --help' 'ash-zig/zig-out/bin/ash --help'
```

## Feature Parity

Both implementations provide identical functionality:
- Scan: caches, logs, Xcode, Homebrew, browsers
- Safety: protected paths, bundle ID allowlist, Trash-only deletion
- TUI: home, scanning, results, confirm, cleaning, maintenance views
