# ash

macOS cleanup utility - two implementations for benchmarking.

| Folder | Language | TUI | Dependencies |
|--------|----------|-----|--------------|
| `ash-go/` | Go 1.24+ | Bubble Tea | 6 external |
| `ash-zig/` | Zig 0.13+ | Custom ANSI | Zero |

## Build

```bash
# Go
cd ash-go && bun run build:release && cd ..

# Zig
cd ash-zig && bun run build:release && cd ..
```

## Benchmark

```bash
# Binary size
ls -lh ash-go/bin/ash ash-zig/zig-out/bin/ash

# Startup time (20 runs)
hyperfine --runs 20 --warmup 3 \
  'ash-go/bin/ash --help' \
  'ash-zig/zig-out/bin/ash --help'

# Full scan dry run (20 runs)
hyperfine --runs 20 --warmup 3 \
  'ash-go/bin/ash --dry-run' \
  'ash-zig/zig-out/bin/ash --dry-run'
```
