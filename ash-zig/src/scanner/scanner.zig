const std = @import("std");
const utils = @import("../utils.zig");
const guards = @import("../safety/guards.zig");

pub const Category = enum {
    caches,
    logs,
    xcode,
    homebrew,
    browsers,
    app_data,
    other,

    pub fn description(self: Category) []const u8 {
        return switch (self) {
            .caches => "Application Caches",
            .logs => "Log Files",
            .xcode => "Xcode Data",
            .homebrew => "Homebrew Cache",
            .browsers => "Browser Caches",
            .app_data => "App Leftovers",
            .other => "Other",
        };
    }

    pub fn shortName(self: Category) []const u8 {
        return switch (self) {
            .caches => "Caches",
            .logs => "Logs",
            .xcode => "Xcode",
            .homebrew => "Homebrew",
            .browsers => "Browsers",
            .app_data => "Apps",
            .other => "Other",
        };
    }
};

pub const RiskLevel = enum {
    safe,
    caution,
    dangerous,

    pub fn symbol(self: RiskLevel) []const u8 {
        return switch (self) {
            .safe => "o",
            .caution => "!",
            .dangerous => "X",
        };
    }
};

pub const Entry = struct {
    path_buf: [std.fs.max_path_bytes]u8 = undefined,
    path_len: usize = 0,
    name_buf: [256]u8 = undefined,
    name_len: usize = 0,
    size: u64 = 0,
    mod_time: i128 = 0,
    category: Category = .other,
    risk: RiskLevel = .safe,
    selected: bool = false,
    bundle_id_buf: [256]u8 = undefined,
    bundle_id_len: usize = 0,
    is_dir: bool = false,

    pub fn path(self: *const Entry) []const u8 {
        return self.path_buf[0..self.path_len];
    }

    pub fn name(self: *const Entry) []const u8 {
        return self.name_buf[0..self.name_len];
    }

    pub fn bundleId(self: *const Entry) ?[]const u8 {
        if (self.bundle_id_len == 0) return null;
        return self.bundle_id_buf[0..self.bundle_id_len];
    }

    pub fn setPath(self: *Entry, p: []const u8) void {
        const len = @min(p.len, self.path_buf.len);
        @memcpy(self.path_buf[0..len], p[0..len]);
        self.path_len = len;
    }

    pub fn setName(self: *Entry, n: []const u8) void {
        const len = @min(n.len, self.name_buf.len);
        @memcpy(self.name_buf[0..len], n[0..len]);
        self.name_len = len;
    }

    pub fn setBundleId(self: *Entry, bid: []const u8) void {
        const len = @min(bid.len, self.bundle_id_buf.len);
        @memcpy(self.bundle_id_buf[0..len], bid[0..len]);
        self.bundle_id_len = len;
    }

    pub fn formattedSize(self: *const Entry) [32]u8 {
        return utils.formatSize(self.size);
    }
};

pub const ScanError = struct {
    path_buf: [std.fs.max_path_bytes]u8 = undefined,
    path_len: usize = 0,
    message_buf: [256]u8 = undefined,
    message_len: usize = 0,
    code: ErrorCode = .io_error,

    pub fn path(self: *const ScanError) []const u8 {
        return self.path_buf[0..self.path_len];
    }

    pub fn message(self: *const ScanError) []const u8 {
        return self.message_buf[0..self.message_len];
    }
};

pub const ErrorCode = enum {
    permission_denied,
    not_found,
    io_error,
};

pub const ScanOptions = struct {
    categories: []const Category = &.{},
    min_size: u64 = 0,
    include_hidden: bool = false,
    parallelism: usize = 4,
};

pub const ScanResult = struct {
    entries: std.ArrayList(Entry),
    total_size: u64,
    total_count: usize,
    duration_ns: u64,
    errors: std.ArrayList(ScanError),
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator) ScanResult {
        return .{
            .entries = std.ArrayList(Entry).init(allocator),
            .total_size = 0,
            .total_count = 0,
            .duration_ns = 0,
            .errors = std.ArrayList(ScanError).init(allocator),
            .allocator = allocator,
        };
    }

    pub fn deinit(self: *ScanResult) void {
        self.entries.deinit();
        self.errors.deinit();
    }

    pub fn addEntry(self: *ScanResult, entry: Entry) !void {
        try self.entries.append(entry);
        self.total_size += entry.size;
        self.total_count += 1;
    }

    pub fn addError(self: *ScanResult, err: ScanError) !void {
        try self.errors.append(err);
    }

    pub fn sortBySize(self: *ScanResult) void {
        std.mem.sort(Entry, self.entries.items, {}, struct {
            fn lessThan(_: void, a: Entry, b: Entry) bool {
                return a.size > b.size; // Descending
            }
        }.lessThan);
    }

    pub fn sortByName(self: *ScanResult) void {
        std.mem.sort(Entry, self.entries.items, {}, struct {
            fn lessThan(_: void, a: Entry, b: Entry) bool {
                return std.mem.lessThan(u8, a.name(), b.name());
            }
        }.lessThan);
    }

    pub fn selectedSize(self: *const ScanResult) u64 {
        var total: u64 = 0;
        for (self.entries.items) |entry| {
            if (entry.selected) total += entry.size;
        }
        return total;
    }

    pub fn selectedCount(self: *const ScanResult) usize {
        var count: usize = 0;
        for (self.entries.items) |entry| {
            if (entry.selected) count += 1;
        }
        return count;
    }

    pub fn selectAll(self: *ScanResult) void {
        for (self.entries.items) |*entry| {
            if (guards.isSafePath(self.allocator, entry.path())) {
                entry.selected = true;
            }
        }
    }

    pub fn deselectAll(self: *ScanResult) void {
        for (self.entries.items) |*entry| {
            entry.selected = false;
        }
    }

    pub fn toggleSelection(self: *ScanResult, index: usize) void {
        if (index < self.entries.items.len) {
            const entry = &self.entries.items[index];
            if (!entry.selected and !guards.isSafePath(self.allocator, entry.path())) {
                return; // Don't allow selecting unsafe paths
            }
            entry.selected = !entry.selected;
        }
    }
};

/// Scan a single directory and return entries
pub fn scanDirectory(
    allocator: std.mem.Allocator,
    base_path: []const u8,
    category: Category,
    risk: RiskLevel,
) !std.ArrayList(Entry) {
    var entries = std.ArrayList(Entry).init(allocator);
    errdefer entries.deinit();

    const expanded = try utils.expandPath(allocator, base_path);
    defer allocator.free(expanded);

    var dir = std.fs.openDirAbsolute(expanded, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied, error.FileNotFound => return entries,
        else => return err,
    };
    defer dir.close();

    var iter = dir.iterate();
    while (try iter.next()) |item| {
        // Skip hidden files unless they're large
        if (item.name[0] == '.') continue;

        var entry = Entry{
            .category = category,
            .risk = risk,
            .is_dir = item.kind == .directory,
        };

        // Build full path
        const full_path = try std.fmt.allocPrint(allocator, "{s}/{s}", .{ expanded, item.name });
        defer allocator.free(full_path);

        entry.setPath(full_path);
        entry.setName(item.name);

        // Get size
        if (item.kind == .directory) {
            entry.size = utils.getDirSize(allocator, full_path) catch 0;
        } else {
            const stat = dir.statFile(item.name) catch continue;
            entry.size = stat.size;
            entry.mod_time = stat.mtime;
        }

        // Skip empty entries
        if (entry.size == 0) continue;

        // Check safety
        if (!guards.isSafePath(allocator, entry.path())) {
            entry.risk = .dangerous;
        }

        try entries.append(entry);
    }

    return entries;
}

// Tests
test "Entry basic operations" {
    var entry = Entry{};
    entry.setPath("/test/path");
    entry.setName("test");
    entry.size = 1024;

    try std.testing.expectEqualStrings("/test/path", entry.path());
    try std.testing.expectEqualStrings("test", entry.name());
    try std.testing.expectEqual(@as(u64, 1024), entry.size);
}

test "ScanResult operations" {
    const allocator = std.testing.allocator;
    var result = ScanResult.init(allocator);
    defer result.deinit();

    var entry1 = Entry{};
    entry1.setPath("/test/a");
    entry1.setName("a");
    entry1.size = 1000;

    var entry2 = Entry{};
    entry2.setPath("/test/b");
    entry2.setName("b");
    entry2.size = 2000;

    try result.addEntry(entry1);
    try result.addEntry(entry2);

    try std.testing.expectEqual(@as(usize, 2), result.total_count);
    try std.testing.expectEqual(@as(u64, 3000), result.total_size);
}
