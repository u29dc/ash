const std = @import("std");
const builtin = @import("builtin");

/// Format bytes into human-readable string (e.g., "1.5 GB")
pub fn formatSize(size: u64) [32]u8 {
    var buf: [32]u8 = undefined;
    @memset(&buf, 0);

    const units = [_][]const u8{ "B", "KB", "MB", "GB", "TB" };
    var value: f64 = @floatFromInt(size);
    var unit_idx: usize = 0;

    while (value >= 1024.0 and unit_idx < units.len - 1) {
        value /= 1024.0;
        unit_idx += 1;
    }

    if (unit_idx == 0) {
        _ = std.fmt.bufPrint(&buf, "{d} {s}", .{ size, units[0] }) catch {};
    } else if (value < 10.0) {
        _ = std.fmt.bufPrint(&buf, "{d:.2} {s}", .{ value, units[unit_idx] }) catch {};
    } else if (value < 100.0) {
        _ = std.fmt.bufPrint(&buf, "{d:.1} {s}", .{ value, units[unit_idx] }) catch {};
    } else {
        _ = std.fmt.bufPrint(&buf, "{d:.0} {s}", .{ value, units[unit_idx] }) catch {};
    }

    return buf;
}

/// Get the home directory path
pub fn getHomeDir(allocator: std.mem.Allocator) ![]const u8 {
    if (std.posix.getenv("HOME")) |home| {
        return try allocator.dupe(u8, home);
    }
    return error.HomeNotFound;
}

/// Expand ~ in path to home directory
pub fn expandPath(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    if (path.len == 0) return try allocator.dupe(u8, path);

    if (std.mem.startsWith(u8, path, "~/")) {
        const home = try getHomeDir(allocator);
        defer allocator.free(home);
        return try std.fmt.allocPrint(allocator, "{s}{s}", .{ home, path[1..] });
    }

    return try allocator.dupe(u8, path);
}

/// Check if path starts with prefix (handles trailing slashes)
pub fn pathStartsWith(path: []const u8, prefix: []const u8) bool {
    const clean_prefix = std.mem.trimRight(u8, prefix, "/");
    if (path.len < clean_prefix.len) return false;
    if (!std.mem.startsWith(u8, path, clean_prefix)) return false;
    if (path.len == clean_prefix.len) return true;
    return path[clean_prefix.len] == '/';
}

/// Match a simple glob pattern (supports * and ?)
pub fn matchGlob(name: []const u8, pattern: []const u8) bool {
    var n_idx: usize = 0;
    var p_idx: usize = 0;
    var star_idx: ?usize = null;
    var match_idx: usize = 0;

    while (n_idx < name.len) {
        if (p_idx < pattern.len and (pattern[p_idx] == '?' or pattern[p_idx] == name[n_idx])) {
            n_idx += 1;
            p_idx += 1;
        } else if (p_idx < pattern.len and pattern[p_idx] == '*') {
            star_idx = p_idx;
            match_idx = n_idx;
            p_idx += 1;
        } else if (star_idx) |si| {
            p_idx = si + 1;
            match_idx += 1;
            n_idx = match_idx;
        } else {
            return false;
        }
    }

    while (p_idx < pattern.len and pattern[p_idx] == '*') {
        p_idx += 1;
    }

    return p_idx == pattern.len;
}

/// Get directory size recursively
pub fn getDirSize(allocator: std.mem.Allocator, path: []const u8) !u64 {
    var total: u64 = 0;

    var dir = std.fs.openDirAbsolute(path, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied, error.FileNotFound => return 0,
        else => return err,
    };
    defer dir.close();

    var walker = dir.walk(allocator) catch return 0;
    defer walker.deinit();

    while (walker.next() catch null) |entry| {
        if (entry.kind == .file) {
            const stat = entry.dir.statFile(entry.basename) catch continue;
            total += stat.size;
        }
    }

    return total;
}

/// Get current timestamp in nanoseconds
pub fn nowNs() i128 {
    return std.time.nanoTimestamp();
}

/// Sleep for specified milliseconds
pub fn sleepMs(ms: u64) void {
    std.time.sleep(ms * std.time.ns_per_ms);
}

/// Check if running on macOS
pub fn isMacOS() bool {
    return builtin.os.tag == .macos;
}

/// Truncate string to max length with ellipsis
pub fn truncate(str: []const u8, max_len: usize) []const u8 {
    if (str.len <= max_len) return str;
    if (max_len < 4) return str[0..max_len];
    return str[0 .. max_len - 3];
}

/// Get basename of a path
pub fn basename(path: []const u8) []const u8 {
    return std.fs.path.basename(path);
}

/// Get dirname of a path
pub fn dirname(path: []const u8) []const u8 {
    return std.fs.path.dirname(path) orelse "";
}

// Tests
test "formatSize" {
    const result1 = formatSize(0);
    try std.testing.expectEqualStrings("0 B", std.mem.sliceTo(&result1, 0));

    const result2 = formatSize(1024);
    try std.testing.expectEqualStrings("1.00 KB", std.mem.sliceTo(&result2, 0));

    const result3 = formatSize(1024 * 1024);
    try std.testing.expectEqualStrings("1.00 MB", std.mem.sliceTo(&result3, 0));

    const result4 = formatSize(1024 * 1024 * 1024);
    try std.testing.expectEqualStrings("1.00 GB", std.mem.sliceTo(&result4, 0));
}

test "matchGlob" {
    try std.testing.expect(matchGlob("test.log", "*.log"));
    try std.testing.expect(matchGlob("test.log", "test.*"));
    try std.testing.expect(matchGlob("test.log", "*"));
    try std.testing.expect(matchGlob("test.log", "t?st.log"));
    try std.testing.expect(!matchGlob("test.log", "*.txt"));
    try std.testing.expect(!matchGlob("test.log", "foo.*"));
}

test "pathStartsWith" {
    try std.testing.expect(pathStartsWith("/Users/test/Library", "/Users/test"));
    try std.testing.expect(pathStartsWith("/Users/test/Library", "/Users/test/"));
    try std.testing.expect(!pathStartsWith("/Users/test", "/Users/testing"));
    try std.testing.expect(pathStartsWith("/Users/test", "/Users/test"));
}
