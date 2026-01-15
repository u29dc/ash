const std = @import("std");
const utils = @import("../utils.zig");

pub const TrashError = error{
    AppleScriptFailed,
    ManualMoveFailed,
    PathNotFound,
    AccessDenied,
};

/// Move a file or directory to Trash using AppleScript (preferred method)
pub fn moveToTrash(allocator: std.mem.Allocator, path: []const u8) !void {
    // First, verify the path exists
    std.fs.accessAbsolute(path, .{}) catch |err| switch (err) {
        error.FileNotFound => return TrashError.PathNotFound,
        error.PermissionDenied => return TrashError.AccessDenied,
        else => return err,
    };

    // Try AppleScript first (proper Trash integration)
    if (moveToTrashAppleScript(allocator, path)) {
        return;
    } else |_| {
        // Fall back to manual move
        try moveToTrashManual(allocator, path);
    }
}

/// Move to Trash using AppleScript (preserves "Put Back" functionality)
fn moveToTrashAppleScript(allocator: std.mem.Allocator, path: []const u8) !void {
    // Escape path for AppleScript
    var escaped_path = std.ArrayList(u8){};
    defer escaped_path.deinit(allocator);

    for (path) |c| {
        if (c == '"' or c == '\\') {
            try escaped_path.append(allocator, '\\');
        }
        try escaped_path.append(allocator, c);
    }

    const script = try std.fmt.allocPrint(
        allocator,
        "tell application \"Finder\" to delete POSIX file \"{s}\"",
        .{escaped_path.items},
    );
    defer allocator.free(script);

    var child = std.process.Child.init(&.{ "osascript", "-e", script }, allocator);
    child.stdout_behavior = .Ignore;
    child.stderr_behavior = .Ignore;

    try child.spawn();
    const term = try child.wait();

    if (term.Exited != 0) {
        return TrashError.AppleScriptFailed;
    }
}

/// Manual move to ~/.Trash (fallback)
fn moveToTrashManual(allocator: std.mem.Allocator, path: []const u8) !void {
    const home = try utils.getHomeDir(allocator);
    defer allocator.free(home);

    const basename = std.fs.path.basename(path);

    // Generate unique name if file already exists in Trash
    var trash_path: []const u8 = undefined;
    var counter: usize = 0;

    while (true) {
        if (counter == 0) {
            trash_path = try std.fmt.allocPrint(allocator, "{s}/.Trash/{s}", .{ home, basename });
        } else {
            trash_path = try std.fmt.allocPrint(allocator, "{s}/.Trash/{s} ({d})", .{ home, basename, counter });
        }

        std.fs.accessAbsolute(trash_path, .{}) catch |err| switch (err) {
            error.FileNotFound => break, // Path doesn't exist, we can use it
            else => {},
        };

        allocator.free(trash_path);
        counter += 1;

        if (counter > 100) {
            return TrashError.ManualMoveFailed;
        }
    }
    defer allocator.free(trash_path);

    // Perform the move
    std.fs.renameAbsolute(path, trash_path) catch |err| switch (err) {
        error.AccessDenied => return TrashError.AccessDenied,
        error.FileNotFound => return TrashError.PathNotFound,
        else => return TrashError.ManualMoveFailed,
    };
}

/// Permanently delete a file or directory (use with caution!)
pub fn permanentDelete(path: []const u8) !void {
    const stat = try std.fs.cwd().statFile(path);

    if (stat.kind == .directory) {
        try std.fs.deleteTreeAbsolute(path);
    } else {
        try std.fs.deleteFileAbsolute(path);
    }
}

/// Empty the Trash
pub fn emptyTrash(allocator: std.mem.Allocator) !void {
    const script = "tell application \"Finder\" to empty trash";

    var child = std.process.Child.init(&.{ "osascript", "-e", script }, allocator);
    child.stdout_behavior = .Ignore;
    child.stderr_behavior = .Ignore;

    try child.spawn();
    _ = try child.wait();
}

/// Get the size of the Trash
pub fn getTrashSize(allocator: std.mem.Allocator) u64 {
    const home = utils.getHomeDir(allocator) catch return 0;
    defer allocator.free(home);

    const trash_path = std.fmt.allocPrint(allocator, "{s}/.Trash", .{home}) catch return 0;
    defer allocator.free(trash_path);

    return utils.getDirSize(allocator, trash_path) catch 0;
}

/// Get the number of items in Trash
pub fn getTrashItemCount(allocator: std.mem.Allocator) usize {
    const home = utils.getHomeDir(allocator) catch return 0;
    defer allocator.free(home);

    const trash_path = std.fmt.allocPrint(allocator, "{s}/.Trash", .{home}) catch return 0;
    defer allocator.free(trash_path);

    var dir = std.fs.openDirAbsolute(trash_path, .{ .iterate = true }) catch return 0;
    defer dir.close();

    var count: usize = 0;
    var iter = dir.iterate();
    while (iter.next() catch null) |entry| {
        if (entry.name[0] != '.') count += 1;
    }

    return count;
}

pub const TrashInfo = struct {
    item_count: usize,
    total_size: u64,
    path: []const u8,
};

/// Get comprehensive Trash information
pub fn getTrashInfo(allocator: std.mem.Allocator) TrashInfo {
    const home = utils.getHomeDir(allocator) catch return .{
        .item_count = 0,
        .total_size = 0,
        .path = "~/.Trash",
    };
    defer allocator.free(home);

    const trash_path = std.fmt.allocPrint(allocator, "{s}/.Trash", .{home}) catch return .{
        .item_count = 0,
        .total_size = 0,
        .path = "~/.Trash",
    };
    defer allocator.free(trash_path);

    return .{
        .item_count = getTrashItemCount(allocator),
        .total_size = getTrashSize(allocator),
        .path = "~/.Trash",
    };
}

// Tests
test "getTrashInfo" {
    const allocator = std.testing.allocator;
    const info = getTrashInfo(allocator);
    // Basic sanity checks
    try std.testing.expectEqualStrings("~/.Trash", info.path);
    // item_count and total_size can be any value (depends on actual trash contents)
}

test "getTrashSize" {
    const allocator = std.testing.allocator;
    // Should not crash or leak memory
    _ = getTrashSize(allocator);
}

test "getTrashItemCount" {
    const allocator = std.testing.allocator;
    // Should not crash or leak memory
    _ = getTrashItemCount(allocator);
}

test "TrashError types" {
    // Verify error types are properly defined
    const e1: TrashError = TrashError.AppleScriptFailed;
    const e2: TrashError = TrashError.ManualMoveFailed;
    const e3: TrashError = TrashError.PathNotFound;
    const e4: TrashError = TrashError.AccessDenied;
    try std.testing.expect(e1 != e2);
    try std.testing.expect(e2 != e3);
    try std.testing.expect(e3 != e4);
}

test "moveToTrash - nonexistent file" {
    const allocator = std.testing.allocator;

    // Trying to move a nonexistent file should return PathNotFound
    const result = moveToTrash(allocator, "/nonexistent/path/to/file/12345");
    try std.testing.expectError(TrashError.PathNotFound, result);
}

test "permanentDelete - nonexistent file" {
    // Trying to delete a nonexistent file should error
    const result = permanentDelete("/nonexistent/path/to/file/12345");
    try std.testing.expectError(error.FileNotFound, result);
}

test "TrashInfo struct" {
    const info = TrashInfo{
        .item_count = 5,
        .total_size = 1024 * 1024,
        .path = "~/.Trash",
    };
    try std.testing.expectEqual(@as(usize, 5), info.item_count);
    try std.testing.expectEqual(@as(u64, 1024 * 1024), info.total_size);
    try std.testing.expectEqualStrings("~/.Trash", info.path);
}
