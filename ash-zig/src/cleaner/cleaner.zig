const std = @import("std");
const scanner = @import("../scanner/scanner.zig");
const guards = @import("../safety/guards.zig");
const trash = @import("trash.zig");
const utils = @import("../utils.zig");

pub const CleanError = error{
    UnsafePath,
    AccessDenied,
    NotFound,
    Failed,
};

pub const CleanResult = struct {
    path_buf: [std.fs.max_path_bytes]u8 = undefined,
    path_len: usize = 0,
    size: u64 = 0,
    success: bool = false,
    error_msg_buf: [256]u8 = undefined,
    error_msg_len: usize = 0,

    pub fn path(self: *const CleanResult) []const u8 {
        return self.path_buf[0..self.path_len];
    }

    pub fn errorMsg(self: *const CleanResult) ?[]const u8 {
        if (self.error_msg_len == 0) return null;
        return self.error_msg_buf[0..self.error_msg_len];
    }

    pub fn setPath(self: *CleanResult, p: []const u8) void {
        const len = @min(p.len, self.path_buf.len);
        @memcpy(self.path_buf[0..len], p[0..len]);
        self.path_len = len;
    }

    pub fn setError(self: *CleanResult, msg: []const u8) void {
        const len = @min(msg.len, self.error_msg_buf.len);
        @memcpy(self.error_msg_buf[0..len], msg[0..len]);
        self.error_msg_len = len;
    }
};

pub const CleanStats = struct {
    total_count: usize = 0,
    success_count: usize = 0,
    failed_count: usize = 0,
    total_size: u64 = 0,
    cleaned_size: u64 = 0,
    duration_ns: u64 = 0,
    results: std.ArrayList(CleanResult) = .{},
    allocator: std.mem.Allocator = undefined,

    pub fn init(allocator: std.mem.Allocator) CleanStats {
        return .{
            .results = .{},
            .allocator = allocator,
        };
    }

    pub fn deinit(self: *CleanStats) void {
        self.results.deinit(self.allocator);
    }

    pub fn addResult(self: *CleanStats, result: CleanResult) !void {
        try self.results.append(self.allocator, result);
        self.total_count += 1;
        self.total_size += result.size;

        if (result.success) {
            self.success_count += 1;
            self.cleaned_size += result.size;
        } else {
            self.failed_count += 1;
        }
    }
};

pub const CleanOptions = struct {
    dry_run: bool = false,
    use_trash: bool = true,
    parallel: bool = true,
};

/// Cleaner manages the deletion of files/directories
pub const Cleaner = struct {
    allocator: std.mem.Allocator,
    options: CleanOptions,

    pub fn init(allocator: std.mem.Allocator, options: CleanOptions) Cleaner {
        return .{
            .allocator = allocator,
            .options = options,
        };
    }

    /// Clean selected entries
    pub fn clean(self: *Cleaner, entries: []scanner.Entry) !CleanStats {
        var stats = CleanStats.init(self.allocator);
        errdefer stats.deinit();

        const start = std.time.nanoTimestamp();

        for (entries) |entry| {
            if (!entry.selected) continue;

            var result = CleanResult{};
            result.setPath(entry.path());
            result.size = entry.size;

            // Validate path safety
            if (!guards.isSafePath(self.allocator, entry.path())) {
                result.success = false;
                result.setError("Path blocked by safety guards");
                try stats.addResult(result);
                continue;
            }

            // Dry run - just report what would be cleaned
            if (self.options.dry_run) {
                result.success = true;
                try stats.addResult(result);
                continue;
            }

            // Perform the actual deletion
            if (self.options.use_trash) {
                trash.moveToTrash(self.allocator, entry.path()) catch |err| {
                    result.success = false;
                    result.setError(switch (err) {
                        trash.TrashError.PathNotFound => "File not found",
                        trash.TrashError.AccessDenied => "Access denied",
                        trash.TrashError.AppleScriptFailed => "Trash operation failed",
                        trash.TrashError.ManualMoveFailed => "Could not move to Trash",
                        else => "Unknown error",
                    });
                    try stats.addResult(result);
                    continue;
                };
            } else {
                trash.permanentDelete(entry.path()) catch |err| {
                    result.success = false;
                    result.setError(switch (err) {
                        error.AccessDenied => "Access denied",
                        error.FileNotFound => "File not found",
                        else => "Delete failed",
                    });
                    try stats.addResult(result);
                    continue;
                };
            }

            result.success = true;
            try stats.addResult(result);
        }

        const end = std.time.nanoTimestamp();
        // Use saturating subtraction to prevent integer overflow if clocks are skewed
        stats.duration_ns = if (end >= start) @intCast(end - start) else 0;

        return stats;
    }

    /// Preview what would be cleaned (dry run)
    pub fn preview(self: *Cleaner, entries: []scanner.Entry) !CleanStats {
        const saved_dry_run = self.options.dry_run;
        self.options.dry_run = true;
        defer self.options.dry_run = saved_dry_run;

        return try self.clean(entries);
    }

    /// Clean a single entry
    pub fn cleanSingle(self: *Cleaner, entry: *scanner.Entry) !CleanResult {
        var result = CleanResult{};
        result.setPath(entry.path());
        result.size = entry.size;

        // Validate path safety
        if (!guards.isSafePath(self.allocator, entry.path())) {
            result.success = false;
            result.setError("Path blocked by safety guards");
            return result;
        }

        if (self.options.dry_run) {
            result.success = true;
            return result;
        }

        if (self.options.use_trash) {
            trash.moveToTrash(self.allocator, entry.path()) catch |err| {
                result.success = false;
                result.setError(switch (err) {
                    trash.TrashError.PathNotFound => "File not found",
                    trash.TrashError.AccessDenied => "Access denied",
                    else => "Trash operation failed",
                });
                return result;
            };
        } else {
            trash.permanentDelete(entry.path()) catch |err| {
                result.success = false;
                result.setError(switch (err) {
                    error.AccessDenied => "Access denied",
                    error.FileNotFound => "File not found",
                    else => "Delete failed",
                });
                return result;
            };
        }

        result.success = true;
        entry.selected = false;
        return result;
    }
};

// Tests
test "CleanStats" {
    const allocator = std.testing.allocator;
    var stats = CleanStats.init(allocator);
    defer stats.deinit();

    var result = CleanResult{};
    result.setPath("/test/path");
    result.size = 1024;
    result.success = true;

    try stats.addResult(result);

    try std.testing.expectEqual(@as(usize, 1), stats.total_count);
    try std.testing.expectEqual(@as(usize, 1), stats.success_count);
    try std.testing.expectEqual(@as(u64, 1024), stats.cleaned_size);
}

test "Cleaner dry run" {
    const allocator = std.testing.allocator;
    var cl = Cleaner.init(allocator, .{ .dry_run = true });

    var entries = [_]scanner.Entry{.{
        .selected = true,
        .size = 1024,
    }};
    entries[0].setPath("/tmp/test");
    entries[0].setName("test");

    var stats = try cl.preview(&entries);
    defer stats.deinit();

    try std.testing.expectEqual(@as(usize, 1), stats.total_count);
}

test "CleanResult error message" {
    var result = CleanResult{};

    // Test setting error message
    result.setError("Permission denied");
    const err_msg = result.errorMsg();
    try std.testing.expect(err_msg != null);
    try std.testing.expectEqualStrings("Permission denied", err_msg.?);

    // Test empty error message
    var result2 = CleanResult{};
    try std.testing.expect(result2.errorMsg() == null);
}

test "CleanResult path handling" {
    var result = CleanResult{};

    // Test short path
    result.setPath("/tmp/test");
    try std.testing.expectEqualStrings("/tmp/test", result.path());

    // Test path at max length (should truncate)
    var long_path: [std.fs.max_path_bytes + 100]u8 = undefined;
    @memset(&long_path, 'a');
    result.setPath(&long_path);
    try std.testing.expectEqual(std.fs.max_path_bytes, result.path_len);
}

test "CleanStats with mixed results" {
    const allocator = std.testing.allocator;
    var stats = CleanStats.init(allocator);
    defer stats.deinit();

    // Add successful result
    var success_result = CleanResult{};
    success_result.setPath("/tmp/success");
    success_result.size = 1000;
    success_result.success = true;
    try stats.addResult(success_result);

    // Add failed result
    var failed_result = CleanResult{};
    failed_result.setPath("/tmp/failed");
    failed_result.size = 500;
    failed_result.success = false;
    failed_result.setError("Access denied");
    try stats.addResult(failed_result);

    // Verify counts
    try std.testing.expectEqual(@as(usize, 2), stats.total_count);
    try std.testing.expectEqual(@as(usize, 1), stats.success_count);
    try std.testing.expectEqual(@as(usize, 1), stats.failed_count);
    try std.testing.expectEqual(@as(u64, 1500), stats.total_size);
    try std.testing.expectEqual(@as(u64, 1000), stats.cleaned_size);
}

test "Cleaner skips unselected entries" {
    const allocator = std.testing.allocator;
    var cl = Cleaner.init(allocator, .{ .dry_run = true });

    var entries = [_]scanner.Entry{
        .{ .selected = false, .size = 1000 },
        .{ .selected = true, .size = 2000 },
        .{ .selected = false, .size = 3000 },
    };
    entries[0].setPath("/tmp/test1");
    entries[0].setName("test1");
    entries[1].setPath("/tmp/test2");
    entries[1].setName("test2");
    entries[2].setPath("/tmp/test3");
    entries[2].setName("test3");

    var stats = try cl.clean(&entries);
    defer stats.deinit();

    // Only selected entry should be processed
    try std.testing.expectEqual(@as(usize, 1), stats.total_count);
    try std.testing.expectEqual(@as(u64, 2000), stats.total_size);
}

test "Cleaner blocks unsafe paths" {
    const allocator = std.testing.allocator;
    var cl = Cleaner.init(allocator, .{ .dry_run = true });

    var entries = [_]scanner.Entry{.{
        .selected = true,
        .size = 1024,
    }};
    // Use a protected path
    entries[0].setPath("~/.ssh/id_rsa");
    entries[0].setName("id_rsa");

    var stats = try cl.clean(&entries);
    defer stats.deinit();

    // Should process but fail due to safety guards
    try std.testing.expectEqual(@as(usize, 1), stats.total_count);
    try std.testing.expectEqual(@as(usize, 0), stats.success_count);
    try std.testing.expectEqual(@as(usize, 1), stats.failed_count);
}
