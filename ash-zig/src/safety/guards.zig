const std = @import("std");
const utils = @import("../utils.zig");

/// Paths that must never be deleted
const never_delete = [_][]const u8{
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

/// Check if a path is safe to delete
pub fn isSafePath(allocator: std.mem.Allocator, path: []const u8) bool {
    // Expand the path
    const expanded = utils.expandPath(allocator, path) catch return false;
    defer allocator.free(expanded);

    // Check never-delete directories
    for (never_delete) |blocked| {
        const blocked_expanded = utils.expandPath(allocator, blocked) catch continue;
        defer allocator.free(blocked_expanded);

        if (utils.pathStartsWith(expanded, blocked_expanded)) {
            return false;
        }
    }

    // Check never-delete patterns
    const base = std.fs.path.basename(path);
    for (never_delete_patterns) |pattern| {
        if (utils.matchGlob(base, pattern)) {
            return false;
        }
    }

    // Check for .git anywhere in path
    if (std.mem.indexOf(u8, path, "/.git/") != null or
        std.mem.indexOf(u8, path, "/.git") != null)
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
