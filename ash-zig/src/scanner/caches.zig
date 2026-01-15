const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");

/// Scan user cache directories
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    const cache_path = "~/Library/Caches";
    const expanded = try utils.expandPath(allocator, cache_path);
    defer allocator.free(expanded);

    var dir = std.fs.openDirAbsolute(expanded, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied, error.FileNotFound => return entries,
        else => return err,
    };
    defer dir.close();

    var iter = dir.iterate();
    while (try iter.next()) |item| {
        // Skip hidden files
        if (item.name[0] == '.') continue;

        // Skip Homebrew (handled by homebrew module)
        if (std.mem.eql(u8, item.name, "Homebrew")) continue;

        // Skip browser caches (handled by browsers module)
        if (std.mem.eql(u8, item.name, "com.apple.Safari") or
            std.mem.startsWith(u8, item.name, "Google") or
            std.mem.startsWith(u8, item.name, "Firefox") or
            std.mem.startsWith(u8, item.name, "com.brave") or
            std.mem.startsWith(u8, item.name, "com.microsoft.Edge"))
        {
            continue;
        }

        var entry = scanner.Entry{
            .category = .caches,
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

        // Extract bundle ID from name if possible
        if (looksLikeBundleId(item.name)) {
            entry.setBundleId(item.name);
        }

        try entries.append(allocator, entry);
    }

    return entries;
}

/// Check if a name looks like a bundle ID
fn looksLikeBundleId(name: []const u8) bool {
    // Bundle IDs typically have format: com.company.app or org.company.app
    var dot_count: usize = 0;
    for (name) |c| {
        if (c == '.') dot_count += 1;
    }
    return dot_count >= 2;
}

/// Get total cache size
pub fn getTotalSize(allocator: std.mem.Allocator) u64 {
    const cache_path = "~/Library/Caches";
    const expanded = utils.expandPath(allocator, cache_path) catch return 0;
    defer allocator.free(expanded);
    return utils.getDirSize(allocator, expanded) catch 0;
}

// Tests
test "looksLikeBundleId" {
    try std.testing.expect(looksLikeBundleId("com.apple.Safari"));
    try std.testing.expect(looksLikeBundleId("org.example.app"));
    try std.testing.expect(!looksLikeBundleId("Safari"));
    try std.testing.expect(!looksLikeBundleId("com.apple"));
}
