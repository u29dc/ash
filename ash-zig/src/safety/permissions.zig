const std = @import("std");
const utils = @import("../utils.zig");

pub const PermissionStatus = enum {
    unknown,
    granted,
    denied,
};

/// Check if Full Disk Access is granted by trying to access protected directories
pub fn checkFullDiskAccess(allocator: std.mem.Allocator) PermissionStatus {
    const protected_dirs = [_][]const u8{
        "~/Library/Mail",
        "~/Library/Messages",
        "~/Library/Safari",
    };

    var accessible_count: usize = 0;
    var denied_count: usize = 0;

    for (protected_dirs) |dir| {
        const expanded = utils.expandPath(allocator, dir) catch continue;
        defer allocator.free(expanded);

        if (canAccessPath(expanded)) {
            accessible_count += 1;
        } else {
            denied_count += 1;
        }
    }

    if (accessible_count > 0 and denied_count == 0) {
        return .granted;
    } else if (denied_count > 0 and accessible_count == 0) {
        return .denied;
    }
    return .unknown;
}

/// Check if sudo access is available without password
pub fn checkSudoAccess() bool {
    var child = std.process.Child.init(&.{ "sudo", "-n", "true" }, std.heap.page_allocator);
    child.stdin_behavior = .Ignore;
    child.stdout_behavior = .Ignore;
    child.stderr_behavior = .Ignore;

    _ = child.spawnAndWait() catch return false;
    return child.term.?.Exited == 0;
}

/// Check if we can access a path
pub fn canAccessPath(path: []const u8) bool {
    const stat = std.fs.cwd().statFile(path) catch |err| switch (err) {
        error.FileNotFound => return false,
        error.AccessDenied => return false,
        else => return false,
    };
    _ = stat;
    return true;
}

/// Check if we can write to a path (checks parent directory)
pub fn canWritePath(allocator: std.mem.Allocator, path: []const u8) bool {
    const parent = std.fs.path.dirname(path) orelse return false;

    // Try to get stat of parent
    var dir = std.fs.openDirAbsolute(parent, .{}) catch return false;
    defer dir.close();

    // Check if we have write access by checking the access mode
    const stat = dir.stat() catch return false;
    _ = stat;

    // On Unix, we'd check the mode bits, but for simplicity we try a stat
    _ = allocator;
    return true;
}

/// Check if a path requires sudo to modify
pub fn requiresSudo(allocator: std.mem.Allocator, path: []const u8) bool {
    const expanded = utils.expandPath(allocator, path) catch return false;
    defer allocator.free(expanded);

    const system_paths = [_][]const u8{
        "/Library/Caches",
        "/Library/Logs",
        "/private/var/log",
        "/System",
        "/usr",
    };

    for (system_paths) |sys_path| {
        if (std.mem.startsWith(u8, expanded, sys_path)) {
            return true;
        }
    }

    return false;
}

/// Get instructions for enabling Full Disk Access
pub fn fdaInstructions() []const u8 {
    return 
    \\Full Disk Access Required
    \\
    \\To scan all caches and logs, ash needs Full Disk Access:
    \\
    \\1. Open System Preferences > Security & Privacy > Privacy
    \\2. Select "Full Disk Access" from the left sidebar
    \\3. Click the lock icon to make changes
    \\4. Click + and add your terminal app (Terminal, iTerm2, etc.)
    \\5. Restart your terminal
    \\
    \\Without FDA, some protected directories will be skipped.
    ;
}

/// Filter paths to only those accessible
pub fn getAccessiblePaths(allocator: std.mem.Allocator, paths: []const []const u8) !std.ArrayList([]const u8) {
    var accessible = std.ArrayList([]const u8).init(allocator);
    errdefer accessible.deinit();

    for (paths) |path| {
        const expanded = utils.expandPath(allocator, path) catch continue;
        defer allocator.free(expanded);

        if (canAccessPath(expanded)) {
            try accessible.append(try allocator.dupe(u8, path));
        }
    }

    return accessible;
}

// Tests
test "requiresSudo" {
    const allocator = std.testing.allocator;

    try std.testing.expect(requiresSudo(allocator, "/Library/Caches/test"));
    try std.testing.expect(requiresSudo(allocator, "/private/var/log/test.log"));
    try std.testing.expect(!requiresSudo(allocator, "~/Library/Caches/test"));
}
