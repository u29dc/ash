const std = @import("std");
const ansi = @import("ansi.zig");
const theme = @import("theme.zig");

/// Screen buffer for efficient rendering
pub const Screen = struct {
    allocator: std.mem.Allocator,
    buffer: std.ArrayList(u8),
    width: u16,
    height: u16,
    style: theme.Style,

    pub fn init(allocator: std.mem.Allocator, width: u16, height: u16) Screen {
        return .{
            .allocator = allocator,
            .buffer = .{},
            .width = width,
            .height = height,
            .style = theme.Style.init(theme.default_theme),
        };
    }

    pub fn deinit(self: *Screen) void {
        self.buffer.deinit(self.allocator);
    }

    pub fn writer(self: *Screen) std.ArrayList(u8).Writer {
        return self.buffer.writer();
    }

    /// Clear the buffer
    pub fn clear(self: *Screen) void {
        self.buffer.clearRetainingCapacity();
    }

    /// Set cursor position
    pub fn moveTo(self: *Screen, row: u16, col: u16) !void {
        try self.buffer.writer().print(ansi.CSI ++ "{d};{d}H", .{ row, col });
    }

    /// Write text at position
    pub fn writeAt(self: *Screen, row: u16, col: u16, text: []const u8) !void {
        try self.moveTo(row, col);
        try self.buffer.appendSlice(text);
    }

    /// Write formatted text at position
    pub fn printAt(self: *Screen, row: u16, col: u16, comptime fmt: []const u8, args: anytype) !void {
        try self.moveTo(row, col);
        try self.buffer.writer().print(fmt, args);
    }

    /// Fill a line with a character
    pub fn fillLine(self: *Screen, row: u16, char: u8) !void {
        try self.moveTo(row, 1);
        for (0..self.width) |_| {
            try self.buffer.append(char);
        }
    }

    /// Draw a horizontal line
    pub fn horizontalLine(self: *Screen, row: u16, col: u16, length: u16) !void {
        try self.moveTo(row, col);
        for (0..length) |_| {
            try self.buffer.appendSlice(ansi.box.horizontal);
        }
    }

    /// Draw a vertical line
    pub fn verticalLine(self: *Screen, row: u16, col: u16, length: u16) !void {
        for (0..length) |i| {
            try self.moveTo(row + @as(u16, @intCast(i)), col);
            try self.buffer.appendSlice(ansi.box.vertical);
        }
    }

    /// Draw a box
    pub fn box(self: *Screen, row: u16, col: u16, width: u16, height: u16) !void {
        // Top border
        try self.moveTo(row, col);
        try self.buffer.appendSlice(ansi.box.top_left);
        for (0..width - 2) |_| {
            try self.buffer.appendSlice(ansi.box.horizontal);
        }
        try self.buffer.appendSlice(ansi.box.top_right);

        // Sides
        for (1..height - 1) |i| {
            try self.moveTo(row + @as(u16, @intCast(i)), col);
            try self.buffer.appendSlice(ansi.box.vertical);
            try self.moveTo(row + @as(u16, @intCast(i)), col + width - 1);
            try self.buffer.appendSlice(ansi.box.vertical);
        }

        // Bottom border
        try self.moveTo(row + height - 1, col);
        try self.buffer.appendSlice(ansi.box.bottom_left);
        for (0..width - 2) |_| {
            try self.buffer.appendSlice(ansi.box.horizontal);
        }
        try self.buffer.appendSlice(ansi.box.bottom_right);
    }

    /// Center text on a line
    pub fn centerText(self: *Screen, row: u16, text: []const u8) !void {
        const col = if (text.len < self.width) (self.width - @as(u16, @intCast(text.len))) / 2 else 1;
        try self.writeAt(row, col, text);
    }

    /// Right-align text on a line
    pub fn rightText(self: *Screen, row: u16, text: []const u8, margin: u16) !void {
        const col = if (text.len + margin < self.width) self.width - @as(u16, @intCast(text.len)) - margin else 1;
        try self.writeAt(row, col, text);
    }

    /// Write with truncation
    pub fn writeTruncated(self: *Screen, text: []const u8, max_len: usize) !void {
        if (text.len <= max_len) {
            try self.buffer.appendSlice(text);
        } else if (max_len > 3) {
            try self.buffer.appendSlice(text[0 .. max_len - 3]);
            try self.buffer.appendSlice("...");
        } else {
            try self.buffer.appendSlice(text[0..max_len]);
        }
    }

    /// Pad text to fixed width
    pub fn writePadded(self: *Screen, text: []const u8, width: usize, alignment: enum { left, right, center }) !void {
        const len = @min(text.len, width);
        const padding = if (width > len) width - len else 0;

        switch (alignment) {
            .left => {
                try self.buffer.appendSlice(text[0..len]);
                for (0..padding) |_| try self.buffer.append(' ');
            },
            .right => {
                for (0..padding) |_| try self.buffer.append(' ');
                try self.buffer.appendSlice(text[0..len]);
            },
            .center => {
                const left_pad = padding / 2;
                const right_pad = padding - left_pad;
                for (0..left_pad) |_| try self.buffer.append(' ');
                try self.buffer.appendSlice(text[0..len]);
                for (0..right_pad) |_| try self.buffer.append(' ');
            },
        }
    }

    /// Flush buffer to stdout
    pub fn flush(self: *Screen) !void {
        const stdout = std.fs.File.stdout();
        try stdout.writeAll(self.buffer.items);
        self.clear();
    }

    /// Set dimensions
    pub fn setSize(self: *Screen, width: u16, height: u16) void {
        self.width = width;
        self.height = height;
    }
};

/// Convenience function to create a progress bar string
pub fn progressBar(progress: f32, width: usize) [64]u8 {
    var buf: [64]u8 = undefined;
    @memset(&buf, 0);

    const bar_width = @min(width, 60);
    const filled = @as(usize, @intFromFloat(progress * @as(f32, @floatFromInt(bar_width))));

    var i: usize = 0;
    for (0..filled) |_| {
        if (i < buf.len) buf[i] = '#';
        i += 1;
    }
    for (0..bar_width - filled) |_| {
        if (i < buf.len) buf[i] = '-';
        i += 1;
    }

    return buf;
}

/// Format a percentage
pub fn formatPercent(value: f32) [8]u8 {
    var buf: [8]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, "{d:>3.0}%", .{value * 100}) catch {};
    return buf;
}

// Tests
test "Screen basic operations" {
    const allocator = std.testing.allocator;
    var screen = Screen.init(allocator, 80, 24);
    defer screen.deinit();

    try screen.writeAt(1, 1, "Hello");
    try std.testing.expect(screen.buffer.items.len > 0);
}

test "progressBar" {
    const bar = progressBar(0.5, 20);
    var filled: usize = 0;
    for (bar) |c| {
        if (c == '#') filled += 1;
    }
    try std.testing.expectEqual(@as(usize, 10), filled);
}
