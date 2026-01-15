const std = @import("std");

pub const Command = struct {
    name: []const u8,
    description: []const u8,
    cmd: []const u8,
    args: []const []const u8,
    requires_sudo: bool,
    useful: bool,
};

pub const CommandResult = struct {
    command: *const Command,
    success: bool,
    output_buf: [4096]u8 = undefined,
    output_len: usize = 0,
    error_msg_buf: [256]u8 = undefined,
    error_msg_len: usize = 0,
    duration_ns: u64 = 0,

    pub fn output(self: *const CommandResult) []const u8 {
        return self.output_buf[0..self.output_len];
    }

    pub fn errorMsg(self: *const CommandResult) ?[]const u8 {
        if (self.error_msg_len == 0) return null;
        return self.error_msg_buf[0..self.error_msg_len];
    }
};

/// Available maintenance commands
pub const commands = [_]Command{
    .{
        .name = "Flush DNS Cache",
        .description = "Clear DNS resolver cache",
        .cmd = "dscacheutil",
        .args = &.{"-flushcache"},
        .requires_sudo = false,
        .useful = true,
    },
    .{
        .name = "Restart mDNSResponder",
        .description = "Restart multicast DNS service",
        .cmd = "killall",
        .args = &.{ "-HUP", "mDNSResponder" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Rebuild Spotlight Index",
        .description = "Reindex Spotlight search database",
        .cmd = "mdutil",
        .args = &.{ "-E", "/" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Rebuild Launch Services",
        .description = "Rebuild app registration database",
        .cmd = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
        .args = &.{ "-kill", "-r", "-domain", "local", "-domain", "user" },
        .requires_sudo = false,
        .useful = true,
    },
    .{
        .name = "Clear Font Cache",
        .description = "Remove cached font data",
        .cmd = "atsutil",
        .args = &.{ "databases", "-remove" },
        .requires_sudo = true,
        .useful = true,
    },
    .{
        .name = "Purge RAM",
        .description = "Free inactive memory (limited benefit)",
        .cmd = "purge",
        .args = &.{},
        .requires_sudo = true,
        .useful = false,
    },
};

/// Run a single maintenance command
pub fn run(allocator: std.mem.Allocator, cmd: *const Command) !CommandResult {
    const start = std.time.nanoTimestamp();

    var result = CommandResult{
        .command = cmd,
        .success = false,
    };

    // Build argv
    var argv = std.ArrayList([]const u8){};
    defer argv.deinit(allocator);

    if (cmd.requires_sudo) {
        try argv.append(allocator, "sudo");
    }
    try argv.append(allocator, cmd.cmd);
    for (cmd.args) |arg| {
        try argv.append(allocator, arg);
    }

    // Execute
    var child = std.process.Child.init(argv.items, allocator);
    child.stdout_behavior = .Pipe;
    child.stderr_behavior = .Pipe;

    try child.spawn();

    // Read output
    const stdout = child.stdout orelse return result;
    const output = stdout.readToEndAlloc(allocator, 4096) catch "";
    defer if (output.len > 0) allocator.free(output);

    const stderr = child.stderr orelse return result;
    const err_output = stderr.readToEndAlloc(allocator, 256) catch "";
    defer if (err_output.len > 0) allocator.free(err_output);

    const term = try child.wait();

    const end = std.time.nanoTimestamp();
    result.duration_ns = @intCast(end - start);

    // Copy output
    if (output.len > 0) {
        const len = @min(output.len, result.output_buf.len);
        @memcpy(result.output_buf[0..len], output[0..len]);
        result.output_len = len;
    }

    if (err_output.len > 0) {
        const len = @min(err_output.len, result.error_msg_buf.len);
        @memcpy(result.error_msg_buf[0..len], err_output[0..len]);
        result.error_msg_len = len;
    }

    result.success = term.Exited == 0;

    return result;
}

/// Run all useful commands
pub fn runAll(allocator: std.mem.Allocator, useful_only: bool) !std.ArrayList(CommandResult) {
    var results = std.ArrayList(CommandResult){};
    errdefer results.deinit(allocator);

    for (&commands) |*cmd| {
        if (useful_only and !cmd.useful) continue;
        const result = try run(allocator, cmd);
        try results.append(allocator, result);
    }

    return results;
}

/// Check if a command is available
pub fn isCommandAvailable(allocator: std.mem.Allocator, cmd_name: []const u8) bool {
    var child = std.process.Child.init(&.{ "which", cmd_name }, allocator);
    child.stdout_behavior = .Ignore;
    child.stderr_behavior = .Ignore;

    child.spawn() catch return false;
    const term = child.wait() catch return false;

    return term.Exited == 0;
}

// Tests
test "commands defined" {
    try std.testing.expect(commands.len > 0);
    try std.testing.expectEqualStrings("Flush DNS Cache", commands[0].name);
}
