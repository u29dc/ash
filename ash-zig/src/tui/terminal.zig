const std = @import("std");
const ansi = @import("ansi.zig");

pub const TerminalError = error{
    NotATTY,
    SetupFailed,
};

/// Terminal state for raw mode
pub const TermState = struct {
    original_termios: std.posix.termios,
    width: u16,
    height: u16,
};

/// Input event types
pub const Event = union(enum) {
    key: Key,
    resize: struct { width: u16, height: u16 },
    none,
};

/// Key events
pub const Key = union(enum) {
    char: u8,
    ctrl: u8,
    alt: u8,
    up,
    down,
    left,
    right,
    enter,
    tab,
    backspace,
    delete,
    home,
    end,
    page_up,
    page_down,
    escape,
    f1,
    f2,
    f3,
    f4,
    f5,
    f6,
    f7,
    f8,
    f9,
    f10,
    f11,
    f12,
};

/// Get terminal size using ioctl
pub fn getSize() !struct { width: u16, height: u16 } {
    const stdout = std.fs.File.stdout();
    var ws: std.posix.winsize = undefined;

    const result = std.posix.system.ioctl(stdout.handle, std.posix.T.IOCGWINSZ, @intFromPtr(&ws));
    if (result != 0) {
        // Fallback to environment variables
        const cols = std.posix.getenv("COLUMNS") orelse "80";
        const lines = std.posix.getenv("LINES") orelse "24";
        return .{
            .width = std.fmt.parseInt(u16, cols, 10) catch 80,
            .height = std.fmt.parseInt(u16, lines, 10) catch 24,
        };
    }

    return .{
        .width = ws.col,
        .height = ws.row,
    };
}

/// Enable raw mode for direct input
pub fn enableRawMode() !TermState {
    const stdin = std.fs.File.stdin();

    // Check if stdin is a TTY
    if (!std.posix.isatty(stdin.handle)) {
        return TerminalError.NotATTY;
    }

    const original = try std.posix.tcgetattr(stdin.handle);
    var raw = original;

    // Input flags - disable various input processing
    raw.iflag.BRKINT = false;
    raw.iflag.ICRNL = false;
    raw.iflag.INPCK = false;
    raw.iflag.ISTRIP = false;
    raw.iflag.IXON = false;

    // Output flags - disable output processing
    raw.oflag.OPOST = false;

    // Control flags
    raw.cflag.CSIZE = .CS8;

    // Local flags - disable echo, canonical mode, signals
    raw.lflag.ECHO = false;
    raw.lflag.ICANON = false;
    raw.lflag.ISIG = false;
    raw.lflag.IEXTEN = false;

    // Control characters
    raw.cc[@intFromEnum(std.posix.V.MIN)] = 0;
    raw.cc[@intFromEnum(std.posix.V.TIME)] = 1;

    try std.posix.tcsetattr(stdin.handle, .FLUSH, raw);

    const size = try getSize();

    return TermState{
        .original_termios = original,
        .width = size.width,
        .height = size.height,
    };
}

/// Restore terminal to original state
pub fn disableRawMode(state: TermState) void {
    const stdin = std.fs.File.stdin();
    std.posix.tcsetattr(stdin.handle, .FLUSH, state.original_termios) catch {};
}

/// Enter alternate screen buffer
pub fn enterAltScreen(writer: anytype) !void {
    try writer.writeAll(ansi.screen.alt_buffer_on);
    try writer.writeAll(ansi.cursor.hide);
    try writer.writeAll(ansi.screen.clear);
    try writer.writeAll(ansi.cursor.home);
}

/// Leave alternate screen buffer
pub fn leaveAltScreen(writer: anytype) !void {
    try writer.writeAll(ansi.cursor.show);
    try writer.writeAll(ansi.screen.alt_buffer_off);
}

/// Clear screen and move cursor home
pub fn clearScreen(writer: anytype) !void {
    try writer.writeAll(ansi.screen.clear);
    try writer.writeAll(ansi.cursor.home);
}

/// Read a single key event (non-blocking)
pub fn readKey() ?Event {
    const stdin = std.fs.File.stdin();
    var buf: [16]u8 = undefined;

    const bytes_read = stdin.read(&buf) catch return null;
    if (bytes_read == 0) return .none;

    return parseInput(buf[0..bytes_read]);
}

/// Parse input bytes into an event
fn parseInput(bytes: []const u8) Event {
    if (bytes.len == 0) return .none;

    // Single byte - regular character or control
    if (bytes.len == 1) {
        const c = bytes[0];
        return switch (c) {
            '\t' => .{ .key = .tab }, // 0x09
            '\n', '\r' => .{ .key = .enter }, // 0x0A, 0x0D
            0x1b => .{ .key = .escape },
            0x7f => .{ .key = .backspace },
            0x00...0x08, 0x0b, 0x0c, 0x0e...0x1a => .{ .key = .{ .ctrl = c + 'a' - 1 } },
            else => .{ .key = .{ .char = c } },
        };
    }

    // Escape sequence
    if (bytes[0] == 0x1b) {
        if (bytes.len >= 2 and bytes[1] == '[') {
            return parseCSI(bytes[2..]);
        }
        if (bytes.len >= 2) {
            return .{ .key = .{ .alt = bytes[1] } };
        }
    }

    return .{ .key = .{ .char = bytes[0] } };
}

/// Parse CSI sequence (ESC [)
fn parseCSI(bytes: []const u8) Event {
    if (bytes.len == 0) return .none;

    // Arrow keys
    if (bytes.len == 1) {
        return switch (bytes[0]) {
            'A' => .{ .key = .up },
            'B' => .{ .key = .down },
            'C' => .{ .key = .right },
            'D' => .{ .key = .left },
            'H' => .{ .key = .home },
            'F' => .{ .key = .end },
            else => .none,
        };
    }

    // Extended sequences (e.g., ESC [ 1 ~)
    if (bytes.len >= 2 and bytes[bytes.len - 1] == '~') {
        const num = std.fmt.parseInt(u8, bytes[0 .. bytes.len - 1], 10) catch return .none;
        return switch (num) {
            1 => .{ .key = .home },
            3 => .{ .key = .delete },
            4 => .{ .key = .end },
            5 => .{ .key = .page_up },
            6 => .{ .key = .page_down },
            11 => .{ .key = .f1 },
            12 => .{ .key = .f2 },
            13 => .{ .key = .f3 },
            14 => .{ .key = .f4 },
            15 => .{ .key = .f5 },
            17 => .{ .key = .f6 },
            18 => .{ .key = .f7 },
            19 => .{ .key = .f8 },
            20 => .{ .key = .f9 },
            21 => .{ .key = .f10 },
            23 => .{ .key = .f11 },
            24 => .{ .key = .f12 },
            else => .none,
        };
    }

    return .none;
}

/// Poll for input with timeout (milliseconds)
pub fn pollInput(timeout_ms: i32) bool {
    const stdin = std.fs.File.stdin();
    var fds = [_]std.posix.pollfd{.{
        .fd = stdin.handle,
        .events = std.posix.POLL.IN,
        .revents = 0,
    }};

    const result = std.posix.poll(&fds, timeout_ms) catch return false;
    return result > 0 and (fds[0].revents & std.posix.POLL.IN) != 0;
}

// Tests
test "getSize" {
    // This test might fail in non-TTY environments
    _ = getSize() catch {};
}

test "parseInput" {
    const enter = parseInput(&[_]u8{'\r'});
    try std.testing.expectEqual(Event{ .key = .enter }, enter);

    const esc = parseInput(&[_]u8{0x1b});
    try std.testing.expectEqual(Event{ .key = .escape }, esc);
}
