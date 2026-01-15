const std = @import("std");
const builtin = @import("builtin");

const App = @import("app.zig").App;
const utils = @import("utils.zig");

const version = "0.1.0";
const commit = "dev";
const build_time = "2025-01-15";

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

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
