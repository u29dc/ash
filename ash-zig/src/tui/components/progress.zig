const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");

pub const ProgressBar = struct {
    progress: f32 = 0.0,
    width: u16 = 40,
    label: []const u8 = "",
    show_percent: bool = true,

    pub fn init(width: u16) ProgressBar {
        return .{ .width = width };
    }

    pub fn setProgress(self: *ProgressBar, value: f32) void {
        self.progress = std.math.clamp(value, 0.0, 1.0);
    }

    pub fn setLabel(self: *ProgressBar, label: []const u8) void {
        self.label = label;
    }

    pub fn setWidth(self: *ProgressBar, width: u16) void {
        self.width = width;
    }

    pub fn render(self: ProgressBar, writer: anytype) !void {
        // Calculate filled portion
        const bar_width = if (self.show_percent) self.width - 5 else self.width;
        const filled = @as(u16, @intFromFloat(self.progress * @as(f32, @floatFromInt(bar_width))));

        // Draw bar
        try writer.writeAll("[");
        for (0..filled) |_| {
            try writer.writeAll(ansi.progress.full);
        }
        for (0..bar_width - filled) |_| {
            try writer.writeAll(ansi.progress.empty);
        }
        try writer.writeAll("]");

        // Draw percentage
        if (self.show_percent) {
            try writer.print(" {d:>3.0}%", .{self.progress * 100});
        }
    }

    pub fn renderWithStyle(self: ProgressBar, writer: anytype, style: theme.Style) !void {
        const bar_width = if (self.show_percent) self.width - 5 else self.width;
        const filled = @as(u16, @intFromFloat(self.progress * @as(f32, @floatFromInt(bar_width))));

        try style.muted(writer);
        try writer.writeAll("[");

        try style.success(writer);
        for (0..filled) |_| {
            try writer.writeAll(ansi.progress.full);
        }

        try style.muted(writer);
        for (0..bar_width - filled) |_| {
            try writer.writeAll(ansi.progress.empty);
        }
        try writer.writeAll("]");

        if (self.show_percent) {
            try style.secondary(writer);
            try writer.print(" {d:>3.0}%", .{self.progress * 100});
        }

        try style.reset(writer);
    }

    pub fn renderIndeterminate(self: ProgressBar, writer: anytype, frame: usize) !void {
        const frames = ansi.spinner_frames.dots;
        try writer.writeAll(frames[frame % frames.len]);
        try writer.writeAll(" ");
        try writer.writeAll(self.label);
    }
};

/// Simple ASCII progress bar
pub fn simpleBar(progress: f32, width: usize) [80]u8 {
    var buf: [80]u8 = undefined;
    @memset(&buf, 0);

    const bar_width = @min(width, 76);
    const filled = @as(usize, @intFromFloat(progress * @as(f32, @floatFromInt(bar_width))));

    var i: usize = 0;
    buf[i] = '[';
    i += 1;

    for (0..filled) |_| {
        if (i < buf.len - 1) buf[i] = '#';
        i += 1;
    }
    for (0..bar_width - filled) |_| {
        if (i < buf.len - 1) buf[i] = '-';
        i += 1;
    }

    if (i < buf.len) buf[i] = ']';

    return buf;
}

// Tests
test "ProgressBar" {
    var bar = ProgressBar.init(20);
    bar.setProgress(0.5);

    var buf: [256]u8 = undefined;
    var stream = std.io.fixedBufferStream(&buf);
    try bar.render(stream.writer());

    const output = stream.getWritten();
    try std.testing.expect(output.len > 0);
}

test "simpleBar" {
    const bar = simpleBar(0.5, 20);
    try std.testing.expectEqual(@as(u8, '['), bar[0]);
}
