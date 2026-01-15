const std = @import("std");
const utils = @import("utils.zig");
const scanner = @import("scanner/scanner.zig");

pub const Config = struct {
    // Scan options
    min_size: u64 = 0,
    include_hidden: bool = false,
    categories: []const scanner.Category = &default_categories,

    // Clean options
    dry_run: bool = false,
    use_trash: bool = true,

    // UI options
    show_sizes: bool = true,
    sort_by: SortOrder = .size,
    parallelism: usize = 4,
};

pub const SortOrder = enum {
    size,
    name,
    date,
};

pub const default_categories = [_]scanner.Category{
    .caches,
    .logs,
    .xcode,
    .homebrew,
    .browsers,
};

pub const default_config = Config{};

/// Load configuration from file
pub fn load(allocator: std.mem.Allocator) !Config {
    const path = try configPath(allocator);
    defer allocator.free(path);

    const file = std.fs.openFileAbsolute(path, .{}) catch return default_config;
    defer file.close();

    const content = file.readToEndAlloc(allocator, 1024 * 64) catch return default_config;
    defer allocator.free(content);

    // Simple JSON parsing - just return defaults for now
    // Full JSON parsing would require more complex implementation
    _ = content;
    return default_config;
}

/// Save configuration to file
pub fn save(allocator: std.mem.Allocator, config: Config) !void {
    const dir = try configDir(allocator);
    defer allocator.free(dir);

    // Create config directory
    std.fs.makeDirAbsolute(dir) catch |err| switch (err) {
        error.PathAlreadyExists => {},
        else => return err,
    };

    const path = try configPath(allocator);
    defer allocator.free(path);

    const file = try std.fs.createFileAbsolute(path, .{});
    defer file.close();

    // Write simple JSON
    try file.writer().print(
        \\{{
        \\  "min_size": {d},
        \\  "include_hidden": {s},
        \\  "dry_run": {s},
        \\  "use_trash": {s},
        \\  "show_sizes": {s},
        \\  "parallelism": {d}
        \\}}
    , .{
        config.min_size,
        if (config.include_hidden) "true" else "false",
        if (config.dry_run) "true" else "false",
        if (config.use_trash) "true" else "false",
        if (config.show_sizes) "true" else "false",
        config.parallelism,
    });
}

/// Get config file path
pub fn configPath(allocator: std.mem.Allocator) ![]const u8 {
    const home = try utils.getHomeDir(allocator);
    defer allocator.free(home);
    return try std.fmt.allocPrint(allocator, "{s}/.config/ash/config.json", .{home});
}

/// Get config directory path
pub fn configDir(allocator: std.mem.Allocator) ![]const u8 {
    const home = try utils.getHomeDir(allocator);
    defer allocator.free(home);
    return try std.fmt.allocPrint(allocator, "{s}/.config/ash", .{home});
}

/// Default values
pub fn defaultMinSize() u64 {
    return 0;
}

pub fn defaultParallelism() usize {
    return 4;
}

pub fn validSortOrders() [3]SortOrder {
    return .{ .size, .name, .date };
}

/// Size threshold for large items that require confirmation (1 GB)
pub fn sizeLimitLarge() u64 {
    return 1024 * 1024 * 1024;
}

// Tests
test "default config" {
    const config = default_config;
    try std.testing.expectEqual(@as(u64, 0), config.min_size);
    try std.testing.expect(config.use_trash);
    try std.testing.expect(!config.dry_run);
}
