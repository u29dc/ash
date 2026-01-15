const std = @import("std");
const builtin = @import("builtin");

/// Format bytes into human-readable string (e.g., "1.5 GB")
pub fn formatSize(size: u64) [32]u8 {
    var buf: [32]u8 = undefined;
    @memset(&buf, 0);

    const units = [_][]const u8{ "B", "KB", "MB", "GB", "TB" };
    var value: f64 = @floatFromInt(size);
    var unit_idx: usize = 0;

    while (value >= 1024.0 and unit_idx < units.len - 1) {
        value /= 1024.0;
        unit_idx += 1;
    }

    if (unit_idx == 0) {
        _ = std.fmt.bufPrint(&buf, "{d} {s}", .{ size, units[0] }) catch {};
    } else if (value < 10.0) {
        _ = std.fmt.bufPrint(&buf, "{d:.2} {s}", .{ value, units[unit_idx] }) catch {};
    } else if (value < 100.0) {
        _ = std.fmt.bufPrint(&buf, "{d:.1} {s}", .{ value, units[unit_idx] }) catch {};
    } else {
        _ = std.fmt.bufPrint(&buf, "{d:.0} {s}", .{ value, units[unit_idx] }) catch {};
    }

    return buf;
}

/// Get the home directory path
pub fn getHomeDir(allocator: std.mem.Allocator) ![]const u8 {
    if (std.posix.getenv("HOME")) |home| {
        return try allocator.dupe(u8, home);
    }
    return error.HomeNotFound;
}

/// Expand ~ in path to home directory
pub fn expandPath(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    if (path.len == 0) return try allocator.dupe(u8, path);

    if (std.mem.startsWith(u8, path, "~/")) {
        const home = try getHomeDir(allocator);
        defer allocator.free(home);
        return try std.fmt.allocPrint(allocator, "{s}{s}", .{ home, path[1..] });
    }

    return try allocator.dupe(u8, path);
}

/// Check if path starts with prefix (handles trailing slashes)
pub fn pathStartsWith(path: []const u8, prefix: []const u8) bool {
    const clean_prefix = std.mem.trimRight(u8, prefix, "/");
    if (path.len < clean_prefix.len) return false;
    if (!std.mem.startsWith(u8, path, clean_prefix)) return false;
    if (path.len == clean_prefix.len) return true;
    return path[clean_prefix.len] == '/';
}

/// Match a simple glob pattern (supports * and ?)
pub fn matchGlob(name: []const u8, pattern: []const u8) bool {
    var n_idx: usize = 0;
    var p_idx: usize = 0;
    var star_idx: ?usize = null;
    var match_idx: usize = 0;

    while (n_idx < name.len) {
        if (p_idx < pattern.len and (pattern[p_idx] == '?' or pattern[p_idx] == name[n_idx])) {
            n_idx += 1;
            p_idx += 1;
        } else if (p_idx < pattern.len and pattern[p_idx] == '*') {
            star_idx = p_idx;
            match_idx = n_idx;
            p_idx += 1;
        } else if (star_idx) |si| {
            p_idx = si + 1;
            match_idx += 1;
            n_idx = match_idx;
        } else {
            return false;
        }
    }

    while (p_idx < pattern.len and pattern[p_idx] == '*') {
        p_idx += 1;
    }

    return p_idx == pattern.len;
}

/// Get directory size recursively (sequential version)
pub fn getDirSize(allocator: std.mem.Allocator, path: []const u8) !u64 {
    var total: u64 = 0;

    var dir = std.fs.openDirAbsolute(path, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied, error.FileNotFound => return 0,
        else => return err,
    };
    defer dir.close();

    var walker = dir.walk(allocator) catch return 0;
    defer walker.deinit();

    while (walker.next() catch null) |entry| {
        if (entry.kind == .file) {
            const stat = entry.dir.statFile(entry.basename) catch continue;
            total += stat.size;
        }
    }

    return total;
}

/// Get directory size recursively with parallel subdirectory processing
pub fn getDirSizeParallel(allocator: std.mem.Allocator, path: []const u8, num_threads: usize) !u64 {
    var dir = std.fs.openDirAbsolute(path, .{ .iterate = true }) catch |err| switch (err) {
        error.AccessDenied, error.FileNotFound => return 0,
        else => return err,
    };
    defer dir.close();

    // Collect subdirectories and files
    var subdirs = std.ArrayList([]const u8){};
    defer {
        for (subdirs.items) |subdir| {
            allocator.free(subdir);
        }
        subdirs.deinit(allocator);
    }

    var file_total: u64 = 0;

    var iter = dir.iterate();
    while (iter.next() catch null) |entry| {
        if (entry.kind == .directory) {
            const subpath = std.fmt.allocPrint(allocator, "{s}/{s}", .{ path, entry.name }) catch continue;
            subdirs.append(allocator, subpath) catch {
                allocator.free(subpath);
                continue;
            };
        } else if (entry.kind == .file) {
            const stat = dir.statFile(entry.name) catch continue;
            file_total += stat.size;
        }
    }

    // If few subdirs, use sequential for less overhead
    if (subdirs.items.len <= 2) {
        var total = file_total;
        for (subdirs.items) |subdir| {
            total += getDirSize(allocator, subdir) catch 0;
        }
        return total;
    }

    // Use thread pool for parallel processing
    var pool: std.Thread.Pool = undefined;
    pool.init(.{ .allocator = allocator, .n_jobs = @min(num_threads, subdirs.items.len) }) catch {
        // Fallback to sequential if pool init fails
        var total = file_total;
        for (subdirs.items) |subdir| {
            total += getDirSize(allocator, subdir) catch 0;
        }
        return total;
    };
    defer pool.deinit();

    var total = std.atomic.Value(u64).init(file_total);
    var wg: std.Thread.WaitGroup = .{};

    for (subdirs.items) |subdir| {
        wg.start();
        pool.spawn(struct {
            fn work(t: *std.atomic.Value(u64), w: *std.Thread.WaitGroup, alloc: std.mem.Allocator, p: []const u8) void {
                defer w.finish();
                const size = getDirSize(alloc, p) catch 0;
                _ = t.fetchAdd(size, .monotonic);
            }
        }.work, .{ &total, &wg, allocator, subdir }) catch {
            wg.finish();
            continue;
        };
    }

    wg.wait();
    return total.load(.acquire);
}

/// Get current timestamp in nanoseconds
pub fn nowNs() i128 {
    return std.time.nanoTimestamp();
}

/// Sleep for specified milliseconds
pub fn sleepMs(ms: u64) void {
    std.time.sleep(ms * std.time.ns_per_ms);
}

/// Check if running on macOS
pub fn isMacOS() bool {
    return builtin.os.tag == .macos;
}

/// Truncate string to max length with ellipsis
pub fn truncate(str: []const u8, max_len: usize) []const u8 {
    if (str.len <= max_len) return str;
    if (max_len < 4) return str[0..max_len];
    return str[0 .. max_len - 3];
}

/// Get basename of a path
pub fn basename(path: []const u8) []const u8 {
    return std.fs.path.basename(path);
}

/// Get dirname of a path
pub fn dirname(path: []const u8) []const u8 {
    return std.fs.path.dirname(path) orelse "";
}

// =============================================================================
// Global Thread Pool - eliminates per-call pool creation overhead
// =============================================================================

var global_pool: ?std.Thread.Pool = null;
var global_pool_allocator: ?std.mem.Allocator = null;

/// Initialize global thread pool (call once at startup)
pub fn initGlobalPool(allocator: std.mem.Allocator) !void {
    if (global_pool != null) return;

    global_pool_allocator = allocator;
    const cpu_count = std.Thread.getCpuCount() catch 4;

    // Initialize pool in-place
    var pool: std.Thread.Pool = undefined;
    try pool.init(.{
        .allocator = allocator,
        .n_jobs = cpu_count,
    });
    global_pool = pool;
}

/// Deinitialize global thread pool (call at shutdown)
pub fn deinitGlobalPool() void {
    if (global_pool) |*pool| {
        pool.deinit();
        global_pool = null;
        global_pool_allocator = null;
    }
}

/// Get the global thread pool
pub fn getGlobalPool() ?*std.Thread.Pool {
    if (global_pool) |*pool| return pool;
    return null;
}

// =============================================================================
// Fast Directory Size - work-stealing queue with recursive parallelism
// =============================================================================

/// Thread-safe work queue for parallel directory traversal
const WorkQueue = struct {
    paths: std.ArrayList([]const u8),
    total: std.atomic.Value(u64),
    pending: std.atomic.Value(usize),
    allocator: std.mem.Allocator,
    mutex: std.Thread.Mutex,

    fn init(alloc: std.mem.Allocator) WorkQueue {
        return .{
            .paths = .{},
            .total = std.atomic.Value(u64).init(0),
            .pending = std.atomic.Value(usize).init(0),
            .allocator = alloc,
            .mutex = .{},
        };
    }

    fn deinit(self: *WorkQueue) void {
        // Free any remaining paths
        for (self.paths.items) |p| {
            self.allocator.free(p);
        }
        self.paths.deinit(self.allocator);
    }

    fn push(self: *WorkQueue, path: []const u8) void {
        self.mutex.lock();
        defer self.mutex.unlock();
        _ = self.pending.fetchAdd(1, .release);
        self.paths.append(self.allocator, path) catch {
            self.allocator.free(path);
            _ = self.pending.fetchSub(1, .release);
        };
    }

    fn pop(self: *WorkQueue) ?[]const u8 {
        self.mutex.lock();
        defer self.mutex.unlock();
        if (self.paths.items.len == 0) return null;
        return self.paths.pop();
    }

    fn markDone(self: *WorkQueue, path: []const u8) void {
        self.allocator.free(path);
        // Decrement pending counter - isDone() checks when this reaches 0
        _ = self.pending.fetchSub(1, .release);
    }

    fn isDone(self: *WorkQueue) bool {
        // Check pending directly - done when no work is pending
        // Use acquire ordering to synchronize with release in markDone/push
        return self.pending.load(.acquire) == 0;
    }
};

/// Worker function for parallel directory traversal
fn dirWorker(queue: *WorkQueue) void {
    var spin_count: usize = 0;
    const max_spins: usize = 1000;

    while (true) {
        const path = queue.pop() orelse {
            // Check if all work is done
            if (queue.isDone()) break;

            // Spin a bit before yielding
            spin_count += 1;
            if (spin_count > max_spins) {
                std.Thread.yield() catch {};
                spin_count = 0;
            }
            continue;
        };

        spin_count = 0;

        // Process this directory
        var dir = std.fs.openDirAbsolute(path, .{ .iterate = true }) catch {
            queue.markDone(path);
            continue;
        };
        defer dir.close();

        var iter = dir.iterate();
        while (iter.next() catch null) |entry| {
            if (entry.kind == .directory) {
                // Push subdirectory for parallel processing
                const subpath = std.fmt.allocPrint(queue.allocator, "{s}/{s}", .{ path, entry.name }) catch continue;
                queue.push(subpath);
            } else if (entry.kind == .file) {
                const stat = dir.statFile(entry.name) catch continue;
                _ = queue.total.fetchAdd(stat.size, .monotonic);
            }
        }

        queue.markDone(path);
    }
}

/// Fast parallel directory size calculation using work-stealing queue
pub fn getDirSizeFast(allocator: std.mem.Allocator, path: []const u8) !u64 {
    // Check if path exists
    std.fs.accessAbsolute(path, .{}) catch return 0;

    var queue = WorkQueue.init(allocator);
    defer queue.deinit();

    // Seed with initial path
    const initial = try allocator.dupe(u8, path);
    queue.push(initial);

    // CRITICAL: Use LOCAL pool to avoid deadlock with global pool
    // (category scanners run on global pool and call this function)
    const cpu_count = std.Thread.getCpuCount() catch 4;
    var pool: std.Thread.Pool = undefined;
    pool.init(.{ .allocator = allocator, .n_jobs = @min(cpu_count, 8) }) catch {
        // Fallback to sequential if pool init fails
        return getDirSize(allocator, path);
    };
    defer pool.deinit();

    // Spawn workers
    var wg: std.Thread.WaitGroup = .{};

    for (0..pool.threads.len) |_| {
        wg.start();
        pool.spawn(struct {
            fn work(w: *std.Thread.WaitGroup, q: *WorkQueue) void {
                defer w.finish();
                dirWorker(q);
            }
        }.work, .{ &wg, &queue }) catch {
            wg.finish();
        };
    }

    wg.wait();
    return queue.total.load(.acquire);
}

// Tests
test "formatSize" {
    const result1 = formatSize(0);
    try std.testing.expectEqualStrings("0 B", std.mem.sliceTo(&result1, 0));

    const result2 = formatSize(1024);
    try std.testing.expectEqualStrings("1.00 KB", std.mem.sliceTo(&result2, 0));

    const result3 = formatSize(1024 * 1024);
    try std.testing.expectEqualStrings("1.00 MB", std.mem.sliceTo(&result3, 0));

    const result4 = formatSize(1024 * 1024 * 1024);
    try std.testing.expectEqualStrings("1.00 GB", std.mem.sliceTo(&result4, 0));
}

test "matchGlob" {
    try std.testing.expect(matchGlob("test.log", "*.log"));
    try std.testing.expect(matchGlob("test.log", "test.*"));
    try std.testing.expect(matchGlob("test.log", "*"));
    try std.testing.expect(matchGlob("test.log", "t?st.log"));
    try std.testing.expect(!matchGlob("test.log", "*.txt"));
    try std.testing.expect(!matchGlob("test.log", "foo.*"));
}

test "pathStartsWith" {
    try std.testing.expect(pathStartsWith("/Users/test/Library", "/Users/test"));
    try std.testing.expect(pathStartsWith("/Users/test/Library", "/Users/test/"));
    try std.testing.expect(!pathStartsWith("/Users/test", "/Users/testing"));
    try std.testing.expect(pathStartsWith("/Users/test", "/Users/test"));
}
