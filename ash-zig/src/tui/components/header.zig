const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");

pub const Header = struct {
    title: []const u8 = "ash",
    subtitle: []const u8 = "",
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init(title: []const u8) Header {
        return .{ .title = title };
    }

    pub fn setTitle(self: *Header, title: []const u8) void {
        self.title = title;
    }

    pub fn setSubtitle(self: *Header, subtitle: []const u8) void {
        self.subtitle = subtitle;
    }

    pub fn render(self: Header, writer: anytype, width: u16) !void {
        // Top border
        try self.style.border(writer);
        try writer.writeAll(ansi.box.top_left);
        for (0..width - 2) |_| {
            try writer.writeAll(ansi.box.horizontal);
        }
        try writer.writeAll(ansi.box.top_right);
        try writer.writeAll("\n");

        // Title line
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try self.style.bold(writer);
        try self.style.primary(writer);

        // Center title
        const title_len = self.title.len;
        const padding = if (width > title_len + 2) (width - @as(u16, @intCast(title_len)) - 2) / 2 else 0;

        for (0..padding) |_| {
            try writer.writeAll(" ");
        }
        try writer.writeAll(self.title);
        for (0..width - 2 - padding - @as(u16, @intCast(title_len))) |_| {
            try writer.writeAll(" ");
        }

        try self.style.reset(writer);
        try self.style.border(writer);
        try writer.writeAll(ansi.box.vertical);
        try writer.writeAll("\n");

        // Subtitle if present
        if (self.subtitle.len > 0) {
            try writer.writeAll(ansi.box.vertical);
            try self.style.reset(writer);
            try self.style.secondary(writer);

            const sub_len = self.subtitle.len;
            const sub_padding = if (width > sub_len + 2) (width - @as(u16, @intCast(sub_len)) - 2) / 2 else 0;

            for (0..sub_padding) |_| {
                try writer.writeAll(" ");
            }
            try writer.writeAll(self.subtitle);
            for (0..width - 2 - sub_padding - @as(u16, @intCast(sub_len))) |_| {
                try writer.writeAll(" ");
            }

            try self.style.reset(writer);
            try self.style.border(writer);
            try writer.writeAll(ansi.box.vertical);
            try writer.writeAll("\n");
        }

        // Bottom border
        try writer.writeAll(ansi.box.bottom_left);
        for (0..width - 2) |_| {
            try writer.writeAll(ansi.box.horizontal);
        }
        try writer.writeAll(ansi.box.bottom_right);
        try self.style.reset(writer);
        try writer.writeAll("\n");
    }

    pub fn renderSimple(self: Header, writer: anytype) !void {
        try self.style.bold(writer);
        try self.style.primary(writer);
        try writer.writeAll(self.title);
        try self.style.reset(writer);

        if (self.subtitle.len > 0) {
            try writer.writeAll(" - ");
            try self.style.secondary(writer);
            try writer.writeAll(self.subtitle);
            try self.style.reset(writer);
        }
        try writer.writeAll("\n");
    }
};

/// ASCII art logo for ash
pub const logo = [_][]const u8{
    "       __               ",
    "  __ _/ /  ___  ___ ___ ",
    " / _` \\_ \\/ _ \\/ _ Y _ \\",
    " \\__,_/__/_//_/_//_/_.__/",
    "                        ",
};

pub fn renderLogo(writer: anytype, style: theme.Style) !void {
    try style.primary(writer);
    for (logo) |line| {
        try writer.writeAll(line);
        try writer.writeAll("\n");
    }
    try style.reset(writer);
}

// Tests
test "Header" {
    var header = Header.init("Test");
    header.setSubtitle("Subtitle");

    var buf: [1024]u8 = undefined;
    var stream = std.io.fixedBufferStream(&buf);
    try header.renderSimple(stream.writer());

    const output = stream.getWritten();
    try std.testing.expect(output.len > 0);
}
