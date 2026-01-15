const std = @import("std");
const builtin = @import("builtin");

const App = @import("app.zig").App;
const utils = @import("utils.zig");
const scanner = @import("scanner/scanner.zig");
const caches = @import("scanner/caches.zig");
const logs = @import("scanner/logs.zig");
const xcode = @import("scanner/xcode.zig");
const homebrew = @import("scanner/homebrew.zig");
const browsers = @import("scanner/browsers.zig");

const version = "0.1.0";
const commit = "dev";
const build_time = "2025-01-15";

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // Initialize global thread pool for parallel scanning
    try utils.initGlobalPool(allocator);
    defer utils.deinitGlobalPool();

    // Parse command line arguments
    const args = try std.process.argsAlloc(allocator);
    defer std.process.argsFree(allocator, args);

    for (args[1..]) |arg| {
        if (std.mem.eql(u8, arg, "-v") or std.mem.eql(u8, arg, "--version")) {
            try printVersion();
            return;
        }
        if (std.mem.eql(u8, arg, "-h") or std.mem.eql(u8, arg, "--help")) {
            try printHelp();
            return;
        }
        if (std.mem.eql(u8, arg, "-n") or std.mem.eql(u8, arg, "--dry-run")) {
            try runDryRun(allocator);
            return;
        }
    }

    // Check platform
    if (!utils.isMacOS()) {
        var err_buf: [256]u8 = undefined;
        var stderr_writer = std.fs.File.stderr().writer(&err_buf);
        const stderr = &stderr_writer.interface;
        try stderr.writeAll("Error: ash only runs on macOS\n");
        try stderr.flush();
        std.process.exit(1);
    }

    // Run application
    var app = App.init(allocator);
    defer app.deinit();

    app.run() catch |err| {
        var err_buf: [256]u8 = undefined;
        var stderr_writer = std.fs.File.stderr().writer(&err_buf);
        const stderr = &stderr_writer.interface;
        stderr.print("Error: {s}\n", .{@errorName(err)}) catch {};
        stderr.flush() catch {};
        std.process.exit(1);
    };
}

fn printVersion() !void {
    var buf: [256]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&buf);
    const stdout = &stdout_writer.interface;
    try stdout.print("ash version {s}\n", .{version});
    try stdout.print("commit: {s}\n", .{commit});
    try stdout.print("built: {s}\n", .{build_time});
    try stdout.flush();
}

fn printHelp() !void {
    var buf: [1024]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&buf);
    const stdout = &stdout_writer.interface;
    try stdout.writeAll(
        \\ash - macOS cleanup utility
        \\
        \\Usage: ash [options]
        \\
        \\Options:
        \\  -h, --help     Show this help message
        \\  -v, --version  Show version information
        \\  -n, --dry-run  Scan and print report (no TTY required)
        \\
        \\Controls:
        \\  j/k, arrows    Navigate
        \\  space          Toggle selection
        \\  a              Select/deselect all
        \\  enter          Confirm action
        \\  esc            Go back
        \\  q              Quit
        \\
        \\For more information, visit: https://github.com/iinfin/ash
        \\
    );
    try stdout.flush();
}

fn runDryRun(allocator: std.mem.Allocator) !void {
    var buf: [8192]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&buf);
    const stdout = &stdout_writer.interface;

    // Get home directory for path shortening
    const home_dir = utils.getHomeDir(allocator) catch "";
    defer if (home_dir.len > 0) allocator.free(home_dir);

    // Scan all categories
    const CategoryInfo = struct {
        name: []const u8,
        category: scanner.Category,
    };

    const category_order = [_]CategoryInfo{
        .{ .name = "Caches", .category = .caches },
        .{ .name = "Logs", .category = .logs },
        .{ .name = "Xcode", .category = .xcode },
        .{ .name = "Homebrew", .category = .homebrew },
        .{ .name = "Browsers", .category = .browsers },
    };

    // Collect all entries using parallel category scanning
    var all_entries = std.ArrayList(scanner.Entry){};
    defer all_entries.deinit(allocator);

    // Use global thread pool for parallel scanning
    const pool = utils.getGlobalPool() orelse return error.NoThreadPool;
    var wg: std.Thread.WaitGroup = .{};

    // Thread-safe storage for results from each scanner
    var cache_entries: std.ArrayList(scanner.Entry) = .{};
    var log_entries: std.ArrayList(scanner.Entry) = .{};
    var xcode_entries: std.ArrayList(scanner.Entry) = .{};
    var homebrew_entries: std.ArrayList(scanner.Entry) = .{};
    var browser_entries: std.ArrayList(scanner.Entry) = .{};

    defer cache_entries.deinit(allocator);
    defer log_entries.deinit(allocator);
    defer xcode_entries.deinit(allocator);
    defer homebrew_entries.deinit(allocator);
    defer browser_entries.deinit(allocator);

    // Spawn workers for each category
    wg.start();
    pool.spawn(struct {
        fn work(w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, result: *std.ArrayList(scanner.Entry)) void {
            defer w.finish();
            const entries = caches.scan(alloc) catch return;
            result.* = entries;
        }
    }.work, .{ &wg, allocator, &cache_entries }) catch wg.finish();

    wg.start();
    pool.spawn(struct {
        fn work(w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, result: *std.ArrayList(scanner.Entry)) void {
            defer w.finish();
            const entries = logs.scan(alloc) catch return;
            result.* = entries;
        }
    }.work, .{ &wg, allocator, &log_entries }) catch wg.finish();

    wg.start();
    pool.spawn(struct {
        fn work(w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, result: *std.ArrayList(scanner.Entry)) void {
            defer w.finish();
            const entries = xcode.scan(alloc) catch return;
            result.* = entries;
        }
    }.work, .{ &wg, allocator, &xcode_entries }) catch wg.finish();

    wg.start();
    pool.spawn(struct {
        fn work(w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, result: *std.ArrayList(scanner.Entry)) void {
            defer w.finish();
            const entries = homebrew.scan(alloc) catch return;
            result.* = entries;
        }
    }.work, .{ &wg, allocator, &homebrew_entries }) catch wg.finish();

    wg.start();
    pool.spawn(struct {
        fn work(w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, result: *std.ArrayList(scanner.Entry)) void {
            defer w.finish();
            const entries = browsers.scan(alloc) catch return;
            result.* = entries;
        }
    }.work, .{ &wg, allocator, &browser_entries }) catch wg.finish();

    // Wait for all scanners to complete
    wg.wait();

    // Merge results
    for (cache_entries.items) |entry| try all_entries.append(allocator, entry);
    for (log_entries.items) |entry| try all_entries.append(allocator, entry);
    for (xcode_entries.items) |entry| try all_entries.append(allocator, entry);
    for (homebrew_entries.items) |entry| try all_entries.append(allocator, entry);
    for (browser_entries.items) |entry| try all_entries.append(allocator, entry);

    // Print header
    try stdout.writeAll("ash - dry run report\n\n");
    try stdout.flush();

    var total_size: u64 = 0;
    var total_count: usize = 0;

    // Print each category
    for (category_order) |cat_info| {
        // Count and sum entries for this category
        var cat_size: u64 = 0;
        var cat_count: usize = 0;
        for (all_entries.items) |entry| {
            if (entry.category == cat_info.category) {
                cat_size += entry.size;
                cat_count += 1;
            }
        }

        if (cat_count == 0) continue;

        total_size += cat_size;
        total_count += cat_count;

        // Print category header
        const size_str = utils.formatSize(cat_size);
        try stdout.print("{s} ({d} items, {s})\n", .{ cat_info.name, cat_count, std.mem.sliceTo(&size_str, 0) });

        // Collect entries for this category and sort by size
        var cat_entries = std.ArrayList(scanner.Entry){};
        defer cat_entries.deinit(allocator);
        for (all_entries.items) |entry| {
            if (entry.category == cat_info.category) {
                try cat_entries.append(allocator, entry);
            }
        }

        // Sort by size descending
        std.mem.sort(scanner.Entry, cat_entries.items, {}, struct {
            fn lessThan(_: void, a: scanner.Entry, b: scanner.Entry) bool {
                return a.size > b.size;
            }
        }.lessThan);

        // Print entries (limit to 20)
        const display_count = @min(cat_entries.items.len, 20);
        for (cat_entries.items[0..display_count]) |entry| {
            const path = entry.path();
            const entry_size_str = utils.formatSize(entry.size);

            // Shorten path with ~
            if (home_dir.len > 0 and std.mem.startsWith(u8, path, home_dir)) {
                try stdout.print("  ~{s:<57} {s:>10}\n", .{ path[home_dir.len..], std.mem.sliceTo(&entry_size_str, 0) });
            } else {
                try stdout.print("  {s:<58} {s:>10}\n", .{ path, std.mem.sliceTo(&entry_size_str, 0) });
            }
        }

        if (cat_entries.items.len > 20) {
            try stdout.print("  ... and {d} more items\n", .{cat_entries.items.len - 20});
        }
        try stdout.writeAll("\n");
        try stdout.flush();
    }

    // Print summary
    const total_size_str = utils.formatSize(total_size);
    try stdout.print("Summary: {d} items, {s} total\n", .{ total_count, std.mem.sliceTo(&total_size_str, 0) });
    try stdout.flush();
}

// Import all modules for testing
test {
    // Core modules
    _ = @import("utils.zig");
    _ = @import("config.zig");
    _ = @import("maintenance.zig");

    // Scanner modules
    _ = @import("scanner/scanner.zig");
    _ = @import("scanner/caches.zig");
    _ = @import("scanner/logs.zig");
    _ = @import("scanner/xcode.zig");
    _ = @import("scanner/homebrew.zig");
    _ = @import("scanner/browsers.zig");
    _ = @import("scanner/apps.zig");

    // Cleaner modules
    _ = @import("cleaner/cleaner.zig");
    _ = @import("cleaner/trash.zig");

    // Safety modules
    _ = @import("safety/guards.zig");
    _ = @import("safety/permissions.zig");

    // TUI modules
    _ = @import("tui/ansi.zig");
    _ = @import("tui/terminal.zig");
    _ = @import("tui/theme.zig");
    _ = @import("tui/render.zig");

    // TUI components
    _ = @import("tui/components/spinner.zig");
    _ = @import("tui/components/progress.zig");
    _ = @import("tui/components/filelist.zig");
    _ = @import("tui/components/header.zig");
    _ = @import("tui/components/keybinds.zig");
    _ = @import("tui/components/toast.zig");

    // Views
    _ = @import("views/home.zig");
    _ = @import("views/scanning.zig");
    _ = @import("views/results.zig");
    _ = @import("views/confirm.zig");
    _ = @import("views/cleaning.zig");
    _ = @import("views/maintenance.zig");

    // App
    _ = @import("app.zig");
}
