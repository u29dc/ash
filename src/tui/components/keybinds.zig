const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");

pub const KeyBind = struct {
    key: []const u8,
    description: []const u8,
};

pub const KeyBinds = struct {
    bindings: []const KeyBind = &.{},
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init(bindings: []const KeyBind) KeyBinds {
        return .{ .bindings = bindings };
    }

    pub fn render(self: KeyBinds, writer: anytype) !void {
        for (self.bindings, 0..) |binding, i| {
            if (i > 0) try writer.writeAll("  ");

            try self.style.primary(writer);
            try writer.writeAll(binding.key);
            try self.style.reset(writer);
            try writer.writeAll(":");
            try self.style.muted(writer);
            try writer.writeAll(binding.description);
            try self.style.reset(writer);
        }
    }

    pub fn renderCompact(self: KeyBinds, writer: anytype) !void {
        try self.style.muted(writer);
        for (self.bindings, 0..) |binding, i| {
            if (i > 0) try writer.writeAll(" | ");
            try writer.writeAll(binding.key);
            try writer.writeAll(" ");
            try writer.writeAll(binding.description);
        }
        try self.style.reset(writer);
    }

    pub fn renderVertical(self: KeyBinds, writer: anytype) !void {
        for (self.bindings) |binding| {
            try writer.writeAll("  ");
            try self.style.primary(writer);
            try writer.print("{s: <12}", .{binding.key});
            try self.style.reset(writer);
            try self.style.secondary(writer);
            try writer.writeAll(binding.description);
            try self.style.reset(writer);
            try writer.writeAll("\n");
        }
    }
};

// Common key binding sets
pub const common_bindings = [_]KeyBind{
    .{ .key = "q", .description = "quit" },
    .{ .key = "?", .description = "help" },
};

pub const navigation_bindings = [_]KeyBind{
    .{ .key = "j/k", .description = "navigate" },
    .{ .key = "enter", .description = "select" },
    .{ .key = "esc", .description = "back" },
};

pub const selection_bindings = [_]KeyBind{
    .{ .key = "space", .description = "toggle" },
    .{ .key = "a", .description = "select all" },
};

pub const home_bindings = [_]KeyBind{
    .{ .key = "j/k", .description = "navigate" },
    .{ .key = "enter", .description = "select" },
    .{ .key = "q", .description = "quit" },
};

pub const results_bindings = [_]KeyBind{
    .{ .key = "j/k", .description = "navigate" },
    .{ .key = "space", .description = "toggle" },
    .{ .key = "a", .description = "select all" },
    .{ .key = "enter", .description = "clean" },
    .{ .key = "esc", .description = "back" },
};

pub const confirm_bindings = [_]KeyBind{
    .{ .key = "y", .description = "confirm" },
    .{ .key = "n/esc", .description = "cancel" },
};

pub fn renderFooter(writer: anytype, bindings: []const KeyBind, width: u16, style: theme.Style) !void {
    // Draw separator
    try style.border(writer);
    for (0..width) |_| {
        try writer.writeAll(ansi.box.horizontal);
    }
    try writer.writeAll("\n");

    // Draw bindings
    const kb = KeyBinds.init(bindings);
    kb.style = style;
    try kb.renderCompact(writer);
    try writer.writeAll("\n");
}

// Tests
test "KeyBinds" {
    const bindings = [_]KeyBind{
        .{ .key = "q", .description = "quit" },
        .{ .key = "?", .description = "help" },
    };

    var buf: [256]u8 = undefined;
    var stream = std.io.fixedBufferStream(&buf);

    const kb = KeyBinds.init(&bindings);
    try kb.render(stream.writer());

    const output = stream.getWritten();
    try std.testing.expect(output.len > 0);
}
