const std = @import("std");
const scanner = @import("scanner.zig");
const utils = @import("../utils.zig");

pub const Browser = struct {
    name: []const u8,
    bundle_id: []const u8,
    cache_paths: []const []const u8,
};

/// Known browsers and their cache locations
pub const known_browsers = [_]Browser{
    .{
        .name = "Safari",
        .bundle_id = "com.apple.Safari",
        .cache_paths = &.{
            "~/Library/Caches/com.apple.Safari",
            "~/Library/Caches/com.apple.Safari.SafeBrowsing",
        },
    },
    .{
        .name = "Google Chrome",
        .bundle_id = "com.google.Chrome",
        .cache_paths = &.{
            "~/Library/Caches/Google/Chrome",
            "~/Library/Caches/com.google.Chrome",
        },
    },
    .{
        .name = "Firefox",
        .bundle_id = "org.mozilla.firefox",
        .cache_paths = &.{
            "~/Library/Caches/Firefox",
            "~/Library/Caches/org.mozilla.firefox",
        },
    },
    .{
        .name = "Brave",
        .bundle_id = "com.brave.Browser",
        .cache_paths = &.{
            "~/Library/Caches/BraveSoftware/Brave-Browser",
            "~/Library/Caches/com.brave.Browser",
        },
    },
    .{
        .name = "Microsoft Edge",
        .bundle_id = "com.microsoft.edgemac",
        .cache_paths = &.{
            "~/Library/Caches/Microsoft Edge",
            "~/Library/Caches/com.microsoft.edgemac",
        },
    },
    .{
        .name = "Opera",
        .bundle_id = "com.operasoftware.Opera",
        .cache_paths = &.{
            "~/Library/Caches/com.operasoftware.Opera",
        },
    },
    .{
        .name = "Arc",
        .bundle_id = "company.thebrowser.Browser",
        .cache_paths = &.{
            "~/Library/Caches/company.thebrowser.Browser",
        },
    },
};

/// Scan browser cache directories
pub fn scan(allocator: std.mem.Allocator) !std.ArrayList(scanner.Entry) {
    var entries = std.ArrayList(scanner.Entry){};
    errdefer entries.deinit(allocator);

    for (known_browsers) |browser| {
        // Check if browser is installed
        if (!isBrowserInstalled(browser.bundle_id)) continue;

        for (browser.cache_paths) |cache_path| {
            const expanded = utils.expandPath(allocator, cache_path) catch continue;
            defer allocator.free(expanded);

            // Check if cache path exists
            std.fs.accessAbsolute(expanded, .{}) catch continue;

            var entry = scanner.Entry{
                .category = .browsers,
                .risk = .safe,
                .is_dir = true,
            };

            entry.setPath(expanded);
            entry.setName(browser.name);
            entry.setBundleId(browser.bundle_id);
            entry.size = utils.getDirSizeFast(allocator, expanded) catch 0;

            // Skip empty caches
            if (entry.size == 0) continue;

            try entries.append(allocator, entry);
            break; // Only add one entry per browser
        }
    }

    return entries;
}

/// Check if a browser is installed
pub fn isBrowserInstalled(bundle_id: []const u8) bool {
    // Bundle ID to app name mapping
    const app_names = getAppNameForBundle(bundle_id);

    // Check /Applications first
    for (app_names) |app_name| {
        var buf: [512]u8 = undefined;
        const path = std.fmt.bufPrint(&buf, "/Applications/{s}.app", .{app_name}) catch continue;
        std.fs.accessAbsolute(path, .{}) catch continue;
        return true;
    }

    // Check ~/Applications (expand ~ to HOME)
    if (std.posix.getenv("HOME")) |home| {
        for (app_names) |app_name| {
            var buf: [512]u8 = undefined;
            const path = std.fmt.bufPrint(&buf, "{s}/Applications/{s}.app", .{ home, app_name }) catch continue;
            std.fs.accessAbsolute(path, .{}) catch continue;
            return true;
        }
    }

    return false;
}

fn getAppNameForBundle(bundle_id: []const u8) []const []const u8 {
    if (std.mem.eql(u8, bundle_id, "com.apple.Safari")) return &.{"Safari"};
    if (std.mem.eql(u8, bundle_id, "com.google.Chrome")) return &.{ "Google Chrome", "Chrome" };
    if (std.mem.eql(u8, bundle_id, "org.mozilla.firefox")) return &.{ "Firefox", "Firefox Developer Edition" };
    if (std.mem.eql(u8, bundle_id, "com.brave.Browser")) return &.{"Brave Browser"};
    if (std.mem.eql(u8, bundle_id, "com.microsoft.edgemac")) return &.{"Microsoft Edge"};
    if (std.mem.eql(u8, bundle_id, "com.operasoftware.Opera")) return &.{"Opera"};
    if (std.mem.eql(u8, bundle_id, "company.thebrowser.Browser")) return &.{"Arc"};
    return &.{};
}

/// Get installed browsers
pub fn getInstalledBrowsers() [known_browsers.len]bool {
    var installed: [known_browsers.len]bool = undefined;
    for (known_browsers, 0..) |browser, i| {
        installed[i] = isBrowserInstalled(browser.bundle_id);
    }
    return installed;
}

/// Get total browser cache size
pub fn getTotalCacheSize(allocator: std.mem.Allocator) u64 {
    var total: u64 = 0;
    for (known_browsers) |browser| {
        for (browser.cache_paths) |cache_path| {
            const expanded = utils.expandPath(allocator, cache_path) catch continue;
            defer allocator.free(expanded);
            total += utils.getDirSize(allocator, expanded) catch 0;
        }
    }
    return total;
}

// Tests
test "getAppNameForBundle" {
    const names = getAppNameForBundle("com.apple.Safari");
    try std.testing.expectEqual(@as(usize, 1), names.len);
    try std.testing.expectEqualStrings("Safari", names[0]);
}
