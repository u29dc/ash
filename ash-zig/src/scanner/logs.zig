const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");

/// Log file patterns
const log_patterns = [_][]const u8{
    "*.log",
    "*.log.*",
    "*.asl",
};

/// Scan user log directories
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    const log_paths = [_][]const u8{
        "~/Library/Logs",
    };

    for (log_paths) |log_path| {
        const expanded = try utils.expandPath(allocator, log_path);
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
                .category = .logs,
                .risk = .safe,
                .is_dir = item.kind == .directory,
            };

            const full_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ expanded, item.name });
            defer allocator.free(full_path);

            entry.setPath(full_path);
            entry.setName(item.name);

            if (item.kind == .directory) {
                entry.size = utils.getDirSizeFast(allocator, full_path) catch 0;
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

/// Get total logs size
pub fn getTotalSize(allocator: std.mem.Allocator) u64 {
    const logs_path = "~/Library/Logs";
    const expanded = utils.expandPath(allocator, logs_path) catch return 0;
    defer allocator.free(expanded);
    return utils.getDirSize(allocator, expanded) catch 0;
}

/// Check if a filename matches log patterns
pub fn isLogFile(name: []const u8) bool {
    for (log_patterns) |pattern| {
        if (utils.matchGlob(name, pattern)) return true;
    }
    return false;
}

// Tests
test "isLogFile" {
    try std.testing.expect(isLogFile("app.log"));
    try std.testing.expect(isLogFile("system.log.1"));
    try std.testing.expect(isLogFile("crash.asl"));
    try std.testing.expect(!isLogFile("app.txt"));
    try std.testing.expect(!isLogFile("readme"));
}
