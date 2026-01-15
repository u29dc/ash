const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");
const guards = @import("../safety/guards.zig");

/// Library locations where app leftovers can be found
const leftover_locations = [_][]const u8{
    "~/Library/Application Support",
    "~/Library/Preferences",
    "~/Library/Caches",
    "~/Library/Containers",
    "~/Library/Group Containers",
    "~/Library/Saved Application State",
    "~/Library/WebKit",
    "~/Library/HTTPStorages",
    "~/Library/Cookies",
    "~/Library/LaunchAgents",
};

/// Confidence level for app leftover detection
pub const ConfidenceLevel = enum {
    high, // Exact bundle ID match
    medium, // App name match
    low, // Company/prefix match only
};

/// App leftover entry with confidence
pub const Leftover = struct {
    entry: scanner.Entry,
    confidence: ConfidenceLevel,
    matched_app: []const u8,
};

/// Scan for orphaned app leftovers
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    // Get list of installed apps
    var installed = std.StringHashMap(void).init(allocator);
    defer installed.deinit();

    try collectInstalledApps(allocator, &installed);

    // Scan each leftover location
    for (leftover_locations) |location| {
        const expanded = utils.expandPath(allocator, location) catch continue;
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

            // Check if this looks like an app leftover
            const bundle_id = extractBundleId(item.name);
            if (bundle_id) |bid| {
                // Skip if app is still installed
                if (installed.contains(bid)) continue;

                // Skip protected apps
                if (guards.isProtectedApp(bid)) continue;
            }

            // Check if entry matches any installed app
            if (isInstalledAppData(&installed, item.name)) continue;

            var entry = scanner.Entry{
                .category = .app_data,
                .risk = .caution, // App leftovers are always caution
                .is_dir = item.kind == .directory,
            };

            const full_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ expanded, item.name });
            defer allocator.free(full_path);

            entry.setPath(full_path);
            entry.setName(item.name);

            if (bundle_id) |bid| {
                entry.setBundleId(bid);
            }

            if (item.kind == .directory) {
                entry.size = utils.getDirSize(allocator, full_path) catch 0;
            } else {
                const stat = dir.statFile(item.name) catch continue;
                entry.size = stat.size;
                entry.mod_time = stat.mtime;
            }

            // Skip small entries
            if (entry.size < 1024) continue;

            try entries.append(allocator, entry);
        }
    }

    return entries;
}

/// Collect bundle IDs of installed applications
fn collectInstalledApps(allocator: std.mem.Allocator, installed: *std.StringHashMap(void)) !void {
    const app_dirs = [_][]const u8{
        "/Applications",
        "~/Applications",
    };

    for (app_dirs) |app_dir| {
        const expanded = utils.expandPath(allocator, app_dir) catch continue;
        defer allocator.free(expanded);

        var dir = std.fs.openDirAbsolute(expanded, .{ .iterate = true }) catch continue;
        defer dir.close();

        var iter = dir.iterate();
        while (try iter.next()) |item| {
            if (!std.mem.endsWith(u8, item.name, ".app")) continue;

            // Try to read bundle ID from Info.plist
            const plist_path = try std.fmt.allocPrint(
                allocator,
                "{s}/{s}/Contents/Info.plist",
                .{ expanded, item.name },
            );
            defer allocator.free(plist_path);

            const bundle_id = readBundleId(allocator, plist_path) catch continue;
            defer allocator.free(bundle_id);

            try installed.put(try allocator.dupe(u8, bundle_id), {});

            // Also add the app name (without .app)
            const app_name = item.name[0 .. item.name.len - 4];
            try installed.put(try allocator.dupe(u8, app_name), {});
        }
    }
}

/// Read bundle ID from Info.plist (simplified - just look for pattern)
fn readBundleId(allocator: std.mem.Allocator, plist_path: []const u8) ![]const u8 {
    const file = try std.fs.openFileAbsolute(plist_path, .{});
    defer file.close();

    const content = try file.readToEndAlloc(allocator, 1024 * 1024);
    defer allocator.free(content);

    // Look for CFBundleIdentifier in XML plist
    const key_start = std.mem.indexOf(u8, content, "<key>CFBundleIdentifier</key>") orelse return error.NotFound;
    const string_start = std.mem.indexOfPos(u8, content, key_start, "<string>") orelse return error.NotFound;
    const value_start = string_start + 8;
    const value_end = std.mem.indexOfPos(u8, content, value_start, "</string>") orelse return error.NotFound;

    return try allocator.dupe(u8, content[value_start..value_end]);
}

/// Extract bundle ID from a directory/file name
fn extractBundleId(name: []const u8) ?[]const u8 {
    // Handle .savedState suffix first (before checking bundle ID format)
    if (std.mem.endsWith(u8, name, ".savedState")) {
        const base = name[0 .. name.len - 11];
        return extractBundleId(base);
    }

    // Handle .plist suffix
    if (std.mem.endsWith(u8, name, ".plist")) {
        const base = name[0 .. name.len - 6];
        return extractBundleId(base);
    }

    // Bundle IDs have format: com.company.app or org.company.app
    if (std.mem.startsWith(u8, name, "com.") or
        std.mem.startsWith(u8, name, "org.") or
        std.mem.startsWith(u8, name, "io.") or
        std.mem.startsWith(u8, name, "net."))
    {
        // Count dots - bundle IDs have at least 2
        var dot_count: usize = 0;
        for (name) |c| {
            if (c == '.') dot_count += 1;
        }
        if (dot_count >= 2) return name;
    }

    return null;
}

/// Check if a name matches any installed app
fn isInstalledAppData(installed: *const std.StringHashMap(void), name: []const u8) bool {
    // Direct match
    if (installed.contains(name)) return true;

    // Check bundle ID
    if (extractBundleId(name)) |bid| {
        if (installed.contains(bid)) return true;
    }

    // Check common suffixes
    const suffixes = [_][]const u8{ ".savedState", ".plist", ".binarycookies" };
    for (suffixes) |suffix| {
        if (std.mem.endsWith(u8, name, suffix)) {
            const base = name[0 .. name.len - suffix.len];
            if (installed.contains(base)) return true;
            if (extractBundleId(base)) |bid| {
                if (installed.contains(bid)) return true;
            }
        }
    }

    return false;
}

// Tests
test "extractBundleId" {
    try std.testing.expectEqualStrings("com.apple.Safari", extractBundleId("com.apple.Safari").?);
    try std.testing.expectEqualStrings("org.mozilla.firefox", extractBundleId("org.mozilla.firefox").?);
    try std.testing.expectEqualStrings("com.example.app", extractBundleId("com.example.app.savedState").?);
    try std.testing.expectEqualStrings("com.example.app", extractBundleId("com.example.app.plist").?);
    try std.testing.expectEqual(@as(?[]const u8, null), extractBundleId("Safari"));
}
