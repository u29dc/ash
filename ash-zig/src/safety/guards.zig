const std = @import("std");
const utils = @import("../utils.zig");

/// Paths that must never be deleted (raw, unexpanded)
const never_delete_raw = [_][]const u8{
    "~/Library/Keychains",
    "~/.ssh",
    "~/.gnupg",
    "~/.config",
    "~/.local/share",
    "/System",
    "/usr",
    "/bin",
    "/sbin",
    "/private/var/vm",
    "/private/var/db",
    "/Applications",
    "/Library/Keychains",
    "/Network",
    "/cores",
};

/// File patterns that must never be deleted
const never_delete_patterns = [_][]const u8{
    "*.keychain*",
    "*.pem",
    "*.key",
    ".git",
    ".gitignore",
    "id_rsa*",
    "id_ed25519*",
    "*.p12",
    "*.pfx",
    "*.cer",
    "*.crt",
    "known_hosts",
    "authorized_keys",
};

/// Bundle ID prefixes for protected apps
const protected_bundle_ids = [_][]const u8{
    "com.apple.",
    "com.microsoft.",
};

/// Size threshold (1 GB) that requires confirmation
pub const size_confirmation_threshold: u64 = 1024 * 1024 * 1024;

/// Cached expanded blocked paths (initialized once per process)
var cached_blocked_paths: ?[]const []const u8 = null;
var cache_allocator: ?std.mem.Allocator = null;

/// Initialize the blocked paths cache
pub fn initBlockedPathsCache(allocator: std.mem.Allocator) !void {
    if (cached_blocked_paths != null) return; // Already initialized

    var paths = std.ArrayList([]const u8).init(allocator);
    errdefer {
        for (paths.items) |p| allocator.free(p);
        paths.deinit();
    }

    for (never_delete_raw) |blocked| {
        const expanded = utils.expandPath(allocator, blocked) catch continue;
        try paths.append(expanded);
    }

    cached_blocked_paths = try paths.toOwnedSlice();
    cache_allocator = allocator;
}

/// Deinitialize the blocked paths cache
pub fn deinitBlockedPathsCache() void {
    if (cached_blocked_paths) |paths| {
        if (cache_allocator) |allocator| {
            for (paths) |p| allocator.free(p);
            allocator.free(paths);
        }
    }
    cached_blocked_paths = null;
    cache_allocator = null;
}

/// Resolve symlinks in a path to get the real path
fn resolveSymlinks(allocator: std.mem.Allocator, path: []const u8) ?[]const u8 {
    var buf: [std.fs.max_path_bytes]u8 = undefined;
    const resolved = std.fs.cwd().realpath(path, &buf) catch return null;
    return allocator.dupe(u8, resolved) catch null;
}

/// Check if a path is safe to delete
pub fn isSafePath(allocator: std.mem.Allocator, path: []const u8) bool {
    // Expand the path (handle ~)
    const expanded = utils.expandPath(allocator, path) catch return false;
    defer allocator.free(expanded);

    // Resolve symlinks to get the real path - prevents symlink attacks
    // If resolution fails, continue with expanded path (file might not exist yet)
    const resolved = resolveSymlinks(allocator, expanded) orelse expanded;
    const should_free_resolved = (resolved.ptr != expanded.ptr);
    defer if (should_free_resolved) allocator.free(resolved);

    // Use cached blocked paths if available, otherwise expand on each call
    const blocked_paths = cached_blocked_paths orelse blk: {
        // Fallback: expand paths on each call (less efficient)
        var temp_paths: [never_delete_raw.len][]const u8 = undefined;
        var count: usize = 0;
        for (never_delete_raw) |blocked| {
            temp_paths[count] = utils.expandPath(allocator, blocked) catch continue;
            count += 1;
        }
        // Check and clean up temp paths
        for (temp_paths[0..count]) |blocked_expanded| {
            if (utils.pathStartsWith(resolved, blocked_expanded)) {
                // Clean up allocated paths before returning
                for (temp_paths[0..count]) |p| allocator.free(p);
                return false;
            }
        }
        for (temp_paths[0..count]) |p| allocator.free(p);
        break :blk null;
    };

    // Check against cached blocked paths
    if (blocked_paths) |paths| {
        for (paths) |blocked_expanded| {
            if (utils.pathStartsWith(resolved, blocked_expanded)) {
                return false;
            }
        }
    }

    // Check never-delete patterns against basename
    const base = std.fs.path.basename(path);
    for (never_delete_patterns) |pattern| {
        if (utils.matchGlob(base, pattern)) {
            return false;
        }
    }

    // Check for .git anywhere in path (use resolved path for symlink safety)
    if (std.mem.indexOf(u8, resolved, "/.git/") != null or
        std.mem.indexOf(u8, resolved, "/.git") != null)
    {
        return false;
    }

    return true;
}

/// Check if a bundle ID is protected
pub fn isProtectedApp(bundle_id: []const u8) bool {
    for (protected_bundle_ids) |prefix| {
        if (std.mem.startsWith(u8, bundle_id, prefix)) {
            return true;
        }
    }
    return false;
}

/// Check if a path requires confirmation before deletion
pub fn requiresConfirmation(allocator: std.mem.Allocator, path: []const u8, size: u64) bool {
    // Large files always require confirmation
    if (size >= size_confirmation_threshold) {
        return true;
    }

    const expanded = utils.expandPath(allocator, path) catch return true;
    defer allocator.free(expanded);

    // iOS backups
    if (std.mem.indexOf(u8, expanded, "MobileSync/Backup") != null) {
        return true;
    }

    // Xcode Archives
    if (std.mem.indexOf(u8, expanded, "Xcode/Archives") != null) {
        return true;
    }

    // Application Support directories
    if (std.mem.indexOf(u8, expanded, "Application Support") != null) {
        return true;
    }

    return false;
}

/// Validate a list of paths, returning (safe_paths, blocked_paths)
pub fn validatePaths(
    allocator: std.mem.Allocator,
    paths: []const []const u8,
) !struct { safe: std.ArrayList([]const u8), blocked: std.ArrayList([]const u8) } {
    var safe = std.ArrayList([]const u8).init(allocator);
    var blocked = std.ArrayList([]const u8).init(allocator);

    for (paths) |path| {
        if (isSafePath(allocator, path)) {
            try safe.append(path);
        } else {
            try blocked.append(path);
        }
    }

    return .{ .safe = safe, .blocked = blocked };
}

/// Sanitize a path by removing dangerous characters
pub fn sanitizePath(allocator: std.mem.Allocator, path: []const u8) ![]const u8 {
    var result = std.ArrayList(u8).init(allocator);
    errdefer result.deinit();

    for (path) |c| {
        // Skip null bytes and other control characters
        if (c == 0 or (c < 32 and c != '\t')) continue;
        try result.append(c);
    }

    return result.toOwnedSlice();
}

// Tests
test "isSafePath" {
    const allocator = std.testing.allocator;

    // Safe paths
    try std.testing.expect(isSafePath(allocator, "~/Library/Caches/com.example.app"));
    try std.testing.expect(isSafePath(allocator, "~/Library/Logs/app.log"));
    try std.testing.expect(isSafePath(allocator, "~/Library/Developer/Xcode/DerivedData/Project"));

    // Dangerous paths - must return false
    try std.testing.expect(!isSafePath(allocator, "~/Library/Keychains/login.keychain-db"));
    try std.testing.expect(!isSafePath(allocator, "~/.ssh/id_ed25519"));
    try std.testing.expect(!isSafePath(allocator, "~/.gnupg/private-keys"));
    try std.testing.expect(!isSafePath(allocator, "/System/Library/CoreServices"));
    try std.testing.expect(!isSafePath(allocator, "/usr/bin/bash"));
    try std.testing.expect(!isSafePath(allocator, "~/projects/app/.git/objects"));
}

test "isProtectedApp" {
    try std.testing.expect(isProtectedApp("com.apple.Safari"));
    try std.testing.expect(isProtectedApp("com.apple.finder"));
    try std.testing.expect(isProtectedApp("com.microsoft.VSCode"));
    try std.testing.expect(!isProtectedApp("com.spotify.client"));
    try std.testing.expect(!isProtectedApp("com.homebrew.cask"));
}

test "requiresConfirmation" {
    const allocator = std.testing.allocator;

    // Large files
    try std.testing.expect(requiresConfirmation(allocator, "/tmp/large", 2 * 1024 * 1024 * 1024));

    // Small files
    try std.testing.expect(!requiresConfirmation(allocator, "/tmp/small", 1024));
}

// Edge case tests for safety guards
test "isSafePath - nested protected paths" {
    const allocator = std.testing.allocator;

    // Deeply nested paths under protected directories should be blocked
    try std.testing.expect(!isSafePath(allocator, "~/.ssh/keys/backup/id_rsa"));
    try std.testing.expect(!isSafePath(allocator, "~/.gnupg/private-keys-v1.d/key.gpg"));
    try std.testing.expect(!isSafePath(allocator, "/System/Library/Frameworks/Foundation.framework"));
    try std.testing.expect(!isSafePath(allocator, "~/Library/Keychains/backup/old.keychain"));

    // Git directories at any depth
    try std.testing.expect(!isSafePath(allocator, "/home/user/projects/myapp/.git"));
    try std.testing.expect(!isSafePath(allocator, "/home/user/projects/myapp/.git/config"));
    try std.testing.expect(!isSafePath(allocator, "/home/user/projects/myapp/.git/objects/pack"));
}

test "isSafePath - pattern matching" {
    const allocator = std.testing.allocator;

    // Certificate/key patterns should be blocked
    try std.testing.expect(!isSafePath(allocator, "/tmp/server.pem"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/private.key"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/cert.p12"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/id_rsa"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/id_rsa.pub"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/id_ed25519"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/known_hosts"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/authorized_keys"));

    // Keychain patterns
    try std.testing.expect(!isSafePath(allocator, "/tmp/login.keychain-db"));
    try std.testing.expect(!isSafePath(allocator, "/tmp/backup.keychain"));
}

test "isSafePath - unicode paths" {
    const allocator = std.testing.allocator;

    // Safe unicode paths should work
    try std.testing.expect(isSafePath(allocator, "/tmp/cache-\xc3\xa9\xc3\xa8\xc3\xa0")); // cache-eea with accents
    try std.testing.expect(isSafePath(allocator, "/tmp/\xe4\xb8\xad\xe6\x96\x87")); // Chinese characters

    // But protected patterns should still be blocked with unicode in path
    try std.testing.expect(!isSafePath(allocator, "/tmp/\xe4\xb8\xad\xe6\x96\x87/.git"));
}

test "isSafePath - empty and edge cases" {
    const allocator = std.testing.allocator;

    // Empty path expands to "" which is safe (doesn't match any blocked patterns)
    try std.testing.expect(isSafePath(allocator, ""));

    // Just ~ should expand to home and be safe
    try std.testing.expect(isSafePath(allocator, "~"));

    // Paths with trailing slashes
    try std.testing.expect(isSafePath(allocator, "/tmp/cache/"));
    try std.testing.expect(!isSafePath(allocator, "~/.ssh/"));
}

test "validatePaths" {
    const allocator = std.testing.allocator;

    const paths = [_][]const u8{
        "/tmp/safe1",
        "~/.ssh/id_rsa",
        "/tmp/safe2",
        "/System/Library",
        "/tmp/safe3",
    };

    const result = try validatePaths(allocator, &paths);
    defer result.safe.deinit();
    defer result.blocked.deinit();

    // Should have 3 safe paths
    try std.testing.expectEqual(@as(usize, 3), result.safe.items.len);
    // Should have 2 blocked paths
    try std.testing.expectEqual(@as(usize, 2), result.blocked.items.len);
}

test "sanitizePath" {
    const allocator = std.testing.allocator;

    // Normal path should pass through
    const normal = try sanitizePath(allocator, "/tmp/test");
    defer allocator.free(normal);
    try std.testing.expectEqualStrings("/tmp/test", normal);

    // Path with null bytes should be cleaned
    const with_null = try sanitizePath(allocator, "/tmp/te\x00st");
    defer allocator.free(with_null);
    try std.testing.expectEqualStrings("/tmp/test", with_null);

    // Path with control characters should be cleaned
    const with_ctrl = try sanitizePath(allocator, "/tmp/te\x01\x02st");
    defer allocator.free(with_ctrl);
    try std.testing.expectEqualStrings("/tmp/test", with_ctrl);

    // Tab should be preserved
    const with_tab = try sanitizePath(allocator, "/tmp/te\tst");
    defer allocator.free(with_tab);
    try std.testing.expectEqualStrings("/tmp/te\tst", with_tab);
}
