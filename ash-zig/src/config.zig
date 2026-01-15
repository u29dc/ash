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

    // Check if config file exists, otherwise return defaults
    std.fs.accessAbsolute(path, .{}) catch return default_config;

    // TODO: Implement actual JSON parsing when needed
    // For now, just return defaults
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

test "default values functions" {
    try std.testing.expectEqual(@as(u64, 0), defaultMinSize());
    try std.testing.expectEqual(@as(usize, 4), defaultParallelism());
    try std.testing.expectEqual(@as(u64, 1024 * 1024 * 1024), sizeLimitLarge());
}

test "valid sort orders" {
    const orders = validSortOrders();
    try std.testing.expectEqual(@as(usize, 3), orders.len);
    try std.testing.expectEqual(SortOrder.size, orders[0]);
    try std.testing.expectEqual(SortOrder.name, orders[1]);
    try std.testing.expectEqual(SortOrder.date, orders[2]);
}

test "config path generation" {
    const allocator = std.testing.allocator;

    const cfg_path = configPath(allocator) catch |err| {
        // If HOME is not set, this will fail - that's expected in some test environments
        if (err == error.HomeNotFound) return;
        return err;
    };
    defer allocator.free(cfg_path);

    // Path should end with /config.json
    try std.testing.expect(std.mem.endsWith(u8, cfg_path, "/config.json"));
    try std.testing.expect(std.mem.indexOf(u8, cfg_path, ".config/ash") != null);
}

test "config dir generation" {
    const allocator = std.testing.allocator;

    const cfg_dir = configDir(allocator) catch |err| {
        // If HOME is not set, this will fail - that's expected in some test environments
        if (err == error.HomeNotFound) return;
        return err;
    };
    defer allocator.free(cfg_dir);

    // Dir should end with /ash
    try std.testing.expect(std.mem.endsWith(u8, cfg_dir, "/.config/ash"));
}

test "load returns defaults for missing config" {
    const allocator = std.testing.allocator;

    // Load should return defaults when config file doesn't exist
    const config = load(allocator) catch |err| {
        // If HOME is not set, this will fail - that's expected
        if (err == error.HomeNotFound) return;
        return err;
    };

    // Should get default values
    try std.testing.expectEqual(@as(u64, 0), config.min_size);
    try std.testing.expect(config.use_trash);
}

test "default categories" {
    // Verify default categories include expected values
    try std.testing.expectEqual(@as(usize, 5), default_categories.len);
    try std.testing.expectEqual(scanner.Category.caches, default_categories[0]);
    try std.testing.expectEqual(scanner.Category.logs, default_categories[1]);
    try std.testing.expectEqual(scanner.Category.xcode, default_categories[2]);
    try std.testing.expectEqual(scanner.Category.homebrew, default_categories[3]);
    try std.testing.expectEqual(scanner.Category.browsers, default_categories[4]);
}
