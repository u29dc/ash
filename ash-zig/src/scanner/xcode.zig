const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");

/// Xcode-related paths with their risk levels
const xcode_paths = [_]struct {
    path: []const u8,
    risk: scanner.RiskLevel,
    description: []const u8,
}{
    .{
        .path = "~/Library/Developer/Xcode/DerivedData",
        .risk = .safe,
        .description = "Build artifacts (safe to delete)",
    },
    .{
        .path = "~/Library/Developer/Xcode/Archives",
        .risk = .caution,
        .description = "App Store builds (may contain dSYMs)",
    },
    .{
        .path = "~/Library/Developer/Xcode/iOS DeviceSupport",
        .risk = .safe,
        .description = "Debug symbols for iOS devices",
    },
    .{
        .path = "~/Library/Developer/CoreSimulator/Devices",
        .risk = .caution,
        .description = "Simulator data (may contain user data)",
    },
};

/// Scan Xcode-related directories
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    for (xcode_paths) |xcode_path| {
        const expanded = try utils.expandPath(allocator, xcode_path.path);
        defer allocator.free(expanded);

        // Check if path exists
        std.fs.accessAbsolute(expanded, .{}) catch continue;

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
                .category = .xcode,
                .risk = xcode_path.risk,
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

/// Check if Xcode is installed
pub fn isXcodeInstalled() bool {
    // Check for common Xcode paths
    const xcode_paths_to_check = [_][]const u8{
        "/Applications/Xcode.app",
        "/Applications/Xcode-beta.app",
    };

    for (xcode_paths_to_check) |path| {
        std.fs.accessAbsolute(path, .{}) catch continue;
        return true;
    }

    return false;
}

/// Get total Xcode data size
pub fn getTotalSize(allocator: std.mem.Allocator) u64 {
    var total: u64 = 0;
    for (xcode_paths) |xcode_path| {
        const expanded = utils.expandPath(allocator, xcode_path.path) catch continue;
        defer allocator.free(expanded);
        total += utils.getDirSize(allocator, expanded) catch 0;
    }
    return total;
}

/// Get DerivedData size specifically
pub fn getDerivedDataSize(allocator: std.mem.Allocator) u64 {
    const path = "~/Library/Developer/Xcode/DerivedData";
    const expanded = utils.expandPath(allocator, path) catch return 0;
    defer allocator.free(expanded);
    return utils.getDirSize(allocator, expanded) catch 0;
}

// Tests
test "isXcodeInstalled" {
    // This test will pass regardless of Xcode installation status
    _ = isXcodeInstalled();
}
