# ash - macOS Cleanup CLI Implementation Plan (Zig)

A fast, safe macOS cleanup tool built with Zig.

## Reference

Global coding standards: https://raw.githubusercontent.com/u29dc/dot/refs/heads/main/agents/AGENTS.md
Zig Documentation: https://ziglang.org/documentation/master/
Zig Build System: https://ziglang.org/learn/build-system/
Zig Standard Library: https://github.com/ziglang/zig/blob/master/lib/std/

## Project Overview

**Name**: ash (what remains after burning away the unnecessary)
**Language**: Zig 0.13+ (latest stable)
**TUI Framework**: Custom terminal rendering using raw ANSI escape codes
**Design**: Grayscale minimal aesthetic

## Motivation

A fast, minimal CleanMyMac replacement as a CLI/TUI. CleanMyMac targets well-documented, regenerable files (caches, logs, derived data) that macOS creates automatically. A CLI can match 90%+ of its value by systematically scanning known-safe locations while implementing strict safety guards. No GUI bloat, no background processes, just a fast tool that does one thing well.

## Why Zig

| Criterion         | Go                            | Zig                                |
| ----------------- | ----------------------------- | ---------------------------------- |
| Filesystem perf   | Excellent                     | Excellent (no runtime overhead)    |
| TUI ecosystem     | Bubbletea (mature)            | Manual (raw ANSI)                  |
| macOS integration | Good (plist, trash via shell) | Good (shell integration, no FFI)   |
| Dev velocity      | Fast                          | Moderate                           |
| Binary size       | 3-8 MB                        | <500 KB expected                   |
| Memory control    | GC-managed                    | Manual (full control)              |
| Startup time      | ~50ms                         | <5ms expected                      |
| Dependencies      | External packages             | Zero (std only)                    |

**Decision**: Zig provides excellent performance with zero dependencies, smaller binary, and faster startup. Custom TUI with raw ANSI codes will be lightweight and efficient.

## CleanMyMac Feature Mapping

### What CleanMyMac Does → What ash Implements

| CleanMyMac Feature           | ash Module                   | Status |
| ---------------------------- | ---------------------------- | ------ |
| System Junk (caches, logs)   | `scanner/caches.zig`, `logs` | Core   |
| Developer Junk (Xcode)       | `scanner/xcode.zig`          | Core   |
| Homebrew cleanup             | `scanner/homebrew.zig`       | Core   |
| Browser caches               | `scanner/browsers.zig`       | Core   |
| App Uninstaller              | `scanner/apps.zig`           | Core   |
| Maintenance (DNS, Spotlight) | `maintenance.zig`            | Core   |
| Malware removal              | -                            | Out    |
| Privacy (browser history)    | -                            | Out    |

### Cleanup Targets by Category

**User-level caches and logs (universally safe):**

- `~/Library/Caches/*` — Application caches, often 5-70GB on developer machines
- `~/Library/Logs/*` — Application logs
- `~/Library/Saved Application State/*` — Window positions, session state

**Developer junk (the 50GB+ goldmine):**

| Directory                                       | Purpose         | Typical Size | Safe?               |
| ----------------------------------------------- | --------------- | ------------ | ------------------- |
| `~/Library/Developer/Xcode/DerivedData/`        | Build artifacts | 5-50GB       | Completely safe     |
| `~/Library/Developer/Xcode/Archives/`           | App Store builds| 500MB-20GB   | Keep current        |
| `~/Library/Developer/Xcode/iOS DeviceSupport/`  | Debug symbols   | ~4GB each    | Delete old versions |
| `~/Library/Developer/CoreSimulator/Devices/`    | Simulator data  | 10-30GB      | Safe                |
| `~/Library/Caches/Homebrew/`                    | Downloaded bottles| 2-10GB     | Safe                |

**Browser caches:**

- Safari: `~/Library/Caches/com.apple.Safari/`
- Chrome: `~/Library/Caches/Google/Chrome/Default/Cache/`
- Firefox: `~/Library/Caches/Firefox/Profiles/*/`

### Safety Patterns

**Never-delete (hardcoded, no override):**

- `~/Library/Keychains`
- `~/.ssh`, `~/.gnupg`
- `.git` directories
- `*.keychain*`, `*.pem`, `*.key`
- `/System`, `/usr`, `/bin`, `/sbin`
- `com.apple.*` bundle IDs

**Always move to Trash, never permanent delete.**

## Architecture

### Directory Structure

```
ash/
├── build.zig              # Zig build configuration
├── build.zig.zon          # Package manifest
├── src/
│   ├── main.zig           # Entry point
│   ├── app.zig            # Application state machine
│   ├── scanner/
│   │   ├── scanner.zig    # Core scanner types and interface
│   │   ├── walker.zig     # Parallel directory walker
│   │   ├── caches.zig     # Cache scanning module
│   │   ├── logs.zig       # Log scanning module
│   │   ├── xcode.zig      # Xcode scanning module
│   │   ├── homebrew.zig   # Homebrew scanning module
│   │   ├── browsers.zig   # Browser cache module
│   │   └── apps.zig       # App leftover module
│   ├── cleaner/
│   │   ├── cleaner.zig    # Deletion orchestration
│   │   └── trash.zig      # macOS Trash integration
│   ├── safety/
│   │   ├── guards.zig     # Never-delete patterns
│   │   └── permissions.zig# TCC/FDA detection
│   ├── tui/
│   │   ├── terminal.zig   # Raw terminal operations
│   │   ├── ansi.zig       # ANSI escape code utilities
│   │   ├── theme.zig      # Grayscale design system
│   │   ├── render.zig     # Screen rendering
│   │   └── components/
│   │       ├── header.zig
│   │       ├── filelist.zig
│   │       ├── progress.zig
│   │       ├── spinner.zig
│   │       ├── toast.zig
│   │       └── keybinds.zig
│   ├── views/
│   │   ├── home.zig       # Main dashboard view
│   │   ├── scanning.zig   # Scanning progress view
│   │   ├── results.zig    # Results list view
│   │   ├── confirm.zig    # Deletion confirmation
│   │   ├── cleaning.zig   # Cleaning progress view
│   │   └── maintenance.zig# Maintenance commands view
│   ├── config.zig         # Configuration loading
│   ├── maintenance.zig    # System maintenance commands
│   └── utils.zig          # Utility functions
└── tests/
    └── *.zig              # Test files
```

### Zig Standards Applied

| Standard              | Implementation                          |
| --------------------- | --------------------------------------- |
| Zero runtime overhead | No GC, manual memory management         |
| Compile-time safety   | comptime checks, exhaustive switches    |
| Error handling        | Error unions, errdefer cleanup          |
| Testing               | Built-in test framework, test allocator |
| Formatting            | `zig fmt` (standard formatter)          |
| Build system          | `build.zig` with proper steps           |

## Design System

### Grayscale Palette

```zig
// src/tui/theme.zig
pub const Theme = struct {
    // Base colors - grayscale only (ANSI 256-color)
    background: u8 = 232,     // #0a0a0a (near black)
    surface: u8 = 234,        // #171717 (elevated surface)
    surface_hover: u8 = 236,  // #262626 (hover state)
    border: u8 = 240,         // #404040 (subtle borders)
    border_focus: u8 = 244,   // #737373 (focused borders)

    // Text hierarchy
    text_primary: u8 = 255,   // #fafafa (primary content)
    text_secondary: u8 = 248, // #a3a3a3 (secondary content)
    text_muted: u8 = 242,     // #525252 (disabled/hints)

    // Semantic colors (true colors for accents)
    success: [3]u8 = .{ 34, 197, 94 },   // #22c55e green
    warning: [3]u8 = .{ 245, 158, 11 },  // #f59e0b amber
    danger: [3]u8 = .{ 239, 68, 68 },    // #ef4444 red

    // Selection
    selected: u8 = 236,       // #262626 (selected row bg)
    cursor: u8 = 255,         // #fafafa (cursor indicator)
};

pub const default_theme = Theme{};
```

## Core Types

### Scanner Types

```zig
// src/scanner/scanner.zig
pub const Category = enum {
    caches,
    logs,
    xcode,
    homebrew,
    browsers,
    app_data,
    other,
};

pub const RiskLevel = enum {
    safe,
    caution,
    dangerous,
};

pub const Entry = struct {
    path: []const u8,
    name: []const u8,
    size: u64,
    mod_time: i128,
    category: Category,
    risk: RiskLevel,
    selected: bool,
    bundle_id: ?[]const u8,
    is_dir: bool,
    allocator: std.mem.Allocator,

    pub fn deinit(self: *Entry) void {
        self.allocator.free(self.path);
        self.allocator.free(self.name);
        if (self.bundle_id) |bid| {
            self.allocator.free(bid);
        }
    }
};

pub const ScanResult = struct {
    entries: std.ArrayList(Entry),
    total_size: u64,
    total_count: usize,
    duration_ns: u64,
    errors: std.ArrayList(ScanError),
    allocator: std.mem.Allocator,

    pub fn deinit(self: *ScanResult) void {
        for (self.entries.items) |*entry| {
            entry.deinit();
        }
        self.entries.deinit();
        self.errors.deinit();
    }
};

pub const ScanError = struct {
    path: []const u8,
    message: []const u8,
    code: ErrorCode,
};

pub const ErrorCode = enum {
    permission_denied,
    not_found,
    io_error,
};

pub const ScanOptions = struct {
    categories: []const Category = &.{},
    min_size: u64 = 0,
    include_hidden: bool = false,
    parallelism: usize = 4,
};
```

### Application State

```zig
// src/app.zig
pub const View = enum {
    home,
    scanning,
    results,
    confirm,
    cleaning,
    maintenance,
};

pub const App = struct {
    // View state
    current_view: View,
    width: u16,
    height: u16,

    // Theme
    theme: tui.Theme,

    // Data
    entries: std.ArrayList(scanner.Entry),
    selected: std.StringHashMap(bool),
    total_size: u64,
    selected_size: u64,
    selected_count: usize,

    // List state
    cursor: usize,
    offset: usize,
    page_size: usize,

    // Scan state
    scanning: bool,
    scan_progress: f32,
    scan_message: []const u8,

    // Clean state
    cleaning: bool,
    clean_stats: ?cleaner.CleanStats,

    // Maintenance state
    maintenance_commands: []const maintenance.Command,
    maintenance_cursor: usize,

    // Memory
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator) App { ... }
    pub fn deinit(self: *App) void { ... }
    pub fn update(self: *App, event: Event) !void { ... }
    pub fn render(self: *const App, writer: anytype) !void { ... }
};
```

## Safety Guards

```zig
// src/safety/guards.zig
const never_delete = [_][]const u8{
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
};

const never_delete_patterns = [_][]const u8{
    "*.keychain*",
    "*.pem",
    "*.key",
    ".git",
    ".gitignore",
    "id_rsa*",
    "id_ed25519*",
    "*.p12",
    "*.pfx",
};

const protected_bundle_ids = [_][]const u8{
    "com.apple.",
    "com.microsoft.",
};

pub fn isSafePath(path: []const u8) bool {
    const expanded = expandPath(path);

    // Check never-delete directories
    for (never_delete) |blocked| {
        const blocked_expanded = expandPath(blocked);
        if (std.mem.startsWith(u8, expanded, blocked_expanded)) {
            return false;
        }
    }

    // Check never-delete patterns
    const base = std.fs.path.basename(path);
    for (never_delete_patterns) |pattern| {
        if (matchPattern(base, pattern)) {
            return false;
        }
    }

    return true;
}

pub fn isProtectedApp(bundle_id: []const u8) bool {
    for (protected_bundle_ids) |prefix| {
        if (std.mem.startsWith(u8, bundle_id, prefix)) {
            return true;
        }
    }
    return false;
}
```

## Maintenance Commands

```zig
// src/maintenance.zig
pub const Command = struct {
    name: []const u8,
    description: []const u8,
    cmd: []const u8,
    args: []const []const u8,
    requires_sudo: bool,
    useful: bool,
};

pub const commands = [_]Command{
    .{
        .name = "Flush DNS Cache",
        .description = "Clear DNS resolver cache",
        .cmd = "dscacheutil",
        .args = &.{"-flushcache"},
        .requires_sudo = false,
        .useful = true,
    },
    .{
        .name = "Restart mDNSResponder",
        .description = "Restart multicast DNS service",
        .cmd = "killall",
        .args = &.{ "-HUP", "mDNSResponder" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Rebuild Spotlight Index",
        .description = "Reindex Spotlight search database",
        .cmd = "mdutil",
        .args = &.{ "-E", "/" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Rebuild Launch Services",
        .description = "Rebuild app registration database",
        .cmd = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
        .args = &.{ "-kill", "-r", "-domain", "local", "-domain", "user" },
        .requires_sudo = false,
        .useful = true,
    },
    .{
        .name = "Clear Font Cache",
        .description = "Remove cached font data",
        .cmd = "atsutil",
        .args = &.{ "databases", "-remove" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Purge RAM",
        .description = "Free inactive memory",
        .cmd = "purge",
        .args = &.{},
        .requires_sudo = true,
        .useful = false,
    },
};
```

## Build Configuration

### build.zig

```zig
const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // Main executable
    const exe = b.addExecutable(.{
        .name = "ash",
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });

    // Link libc for system calls
    exe.linkLibC();

    b.installArtifact(exe);

    // Run step
    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }
    const run_step = b.step("run", "Run ash");
    run_step.dependOn(&run_cmd.step);

    // Tests
    const unit_tests = b.addTest(.{
        .root_source_file = b.path("src/main.zig"),
        .target = target,
        .optimize = optimize,
    });
    const run_unit_tests = b.addRunArtifact(unit_tests);
    const test_step = b.step("test", "Run unit tests");
    test_step.dependOn(&run_unit_tests.step);
}
```

### build.zig.zon

```zig
.{
    .name = "ash",
    .version = "0.1.0",
    .minimum_zig_version = "0.13.0",
    .paths = .{
        "build.zig",
        "build.zig.zon",
        "src",
    },
}
```

## Implementation Phases

**Commit Convention**: All commits must follow conventional format.

```
type(scope): subject line

Body explaining what changed and why.
```

Types: `feat`, `fix`, `refactor`, `docs`, `style`, `chore`, `test`, `build`, `perf`
Scopes: `core`, `scanner`, `cleaner`, `tui`, `safety`, `config`, `maintenance`

### Phase 1: Foundation

- [ ] Initialize Zig project structure
- [ ] Set up build.zig and build.zig.zon
- [ ] Implement ANSI escape code utilities
- [ ] Implement terminal raw mode handling
- [ ] Create theme and styles

### Phase 2: Scanner Core

- [ ] Implement parallel directory walker
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
- [ ] App leftover detection
- [ ] Trash integration (osascript)

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

- [ ] README documentation
- [ ] Release build optimization
- [ ] First release binary

## Performance Targets

| Operation | Target | Measurement               |
| --------- | ------ | ------------------------- |
| Startup   | < 5ms  | Cold start to UI render   |
| Full scan | < 2s   | Typical developer machine |
| UI frame  | < 8ms  | 120fps responsiveness     |
| Memory    | < 50MB | Peak during full scan     |
| Binary    | < 500KB| Stripped release build    |

## Quality Gates

1. Zero compiler warnings (`zig build` clean)
2. All tests passing (`zig build test`)
3. Code formatted (`zig fmt`)
4. Release build successful (`zig build -Doptimize=ReleaseFast`)

## Key Zig Patterns Used

### Error Handling

```zig
fn scanDirectory(path: []const u8) ![]Entry {
    const dir = std.fs.openDirAbsolute(path, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied => return &.{},
        error.FileNotFound => return &.{},
        else => return err,
    };
    defer dir.close();
    // ...
}
```

### Memory Management

```zig
pub fn deinit(self: *ScanResult) void {
    for (self.entries.items) |*entry| {
        self.allocator.free(entry.path);
        self.allocator.free(entry.name);
    }
    self.entries.deinit();
}
```

### Compile-Time Checks

```zig
const Category = enum {
    caches,
    logs,
    xcode,
    homebrew,
    browsers,
    app_data,
    other,

    pub fn description(self: Category) []const u8 {
        return switch (self) {
            .caches => "Application Caches",
            .logs => "Log Files",
            .xcode => "Xcode Data",
            .homebrew => "Homebrew Cache",
            .browsers => "Browser Caches",
            .app_data => "App Leftovers",
            .other => "Other",
        };
    }
};
```

### Terminal I/O

```zig
pub fn enableRawMode() !std.posix.termios {
    const stdin = std.io.getStdIn();
    const old = try std.posix.tcgetattr(stdin.handle);
    var new = old;
    new.lflag.ECHO = false;
    new.lflag.ICANON = false;
    new.lflag.ISIG = false;
    new.cc[@intFromEnum(std.posix.V.MIN)] = 0;
    new.cc[@intFromEnum(std.posix.V.TIME)] = 1;
    try std.posix.tcsetattr(stdin.handle, .FLUSH, new);
    return old;
}
```

## Dependencies

**None.** Pure Zig standard library only.

This is a key advantage over the Go version which requires:
- github.com/charmbracelet/bubbletea
- github.com/charmbracelet/bubbles
- github.com/charmbracelet/lipgloss
- github.com/charlievieth/fastwalk
- github.com/dustin/go-humanize
- howett.net/plist
