const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");

/// Homebrew cache paths
const homebrew_paths = [_][]const u8{
    "~/Library/Caches/Homebrew",
    "/opt/homebrew/Caches",
    "/usr/local/Caches/Homebrew",
};

/// Scan Homebrew cache directories
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    for (homebrew_paths) |brew_path| {
        const expanded = utils.expandPath(allocator, brew_path) catch continue;
        defer allocator.free(expanded);

        var dir = std.fs.openDirAbsolute(expanded, .{ .iterate = true }) catch |err| switch (err) {
            error.AccessDenied, error.FileNotFound => continue,
            else => return err,
        };
        defer dir.close();

        var iter = dir.iterate();
        while (try iter.next()) |item| {
            // Skip hidden files
            if (item.name[0] == '.') continue;

            var entry = scanner.Entry{
                .category = .homebrew,
                .risk = .safe,
                .is_dir = item.kind == .directory,
            };

            const full_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ expanded, item.name });
            defer allocator.free(full_path);

            entry.setPath(full_path);
            entry.setName(item.name);

            if (item.kind == .directory) {
                entry.size = utils.getDirSize(allocator, full_path) catch 0;
            } else {
                const stat = dir.statFile(item.name) catch continue;
                entry.size = stat.size;
                entry.mod_time = stat.mtime;
            }

            // Skip empty entries
            if (entry.size == 0) continue;

            try entries.append(allocator, entry);
        }
    }

    return entries;
}

/// Check if Homebrew is installed
pub fn isHomebrewInstalled() bool {
    // Check common Homebrew paths
    const brew_paths = [_][]const u8{
        "/opt/homebrew/bin/brew",
        "/usr/local/bin/brew",
    };

    for (brew_paths) |path| {
        std.fs.accessAbsolute(path, .{}) catch continue;
        return true;
    }

    return false;
}

/// Get Homebrew cache size
pub fn getCacheSize(allocator: std.mem.Allocator) u64 {
    var total: u64 = 0;
    for (homebrew_paths) |path| {
        const expanded = utils.expandPath(allocator, path) catch continue;
        defer allocator.free(expanded);
        total += utils.getDirSize(allocator, expanded) catch 0;
    }
    return total;
}

/// Run `brew cleanup --dry-run` and return output
pub fn getCleanupPreview(allocator: std.mem.Allocator) ![]const u8 {
    var child = std.process.Child.init(&.{ "brew", "cleanup", "--dry-run" }, allocator);
    child.stdout_behavior = .Pipe;
    child.stderr_behavior = .Ignore;

    try child.spawn();

    const stdout = child.stdout orelse return error.NoStdout;
    const output = try stdout.readToEndAlloc(allocator, 1024 * 1024);

    _ = try child.wait();

    return output;
}

/// Run `brew cleanup` to clean Homebrew cache
pub fn runCleanup(allocator: std.mem.Allocator) !void {
    var child = std.process.Child.init(&.{ "brew", "cleanup" }, allocator);
    child.stdout_behavior = .Ignore;
    child.stderr_behavior = .Ignore;

    try child.spawn();
    _ = try child.wait();
}

// Tests
test "isHomebrewInstalled" {
    // This test will pass regardless of Homebrew installation status
    _ = isHomebrewInstalled();
}
