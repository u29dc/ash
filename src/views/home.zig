const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const header = @import("../tui/components/header.zig");
const keybinds = @import("../tui/components/keybinds.zig");

pub const MenuItem = struct {
    title: []const u8,
    description: []const u8,
    action: Action,
};

pub const Action = enum {
    scan,
    maintenance,
    quit,
};

pub const menu_items = [_]MenuItem{
    .{
        .title = "Scan for Junk",
        .description = "Find caches, logs, and developer files to clean",
        .action = .scan,
    },
    .{
        .title = "Maintenance",
        .description = "Run system maintenance commands",
        .action = .maintenance,
    },
    .{
        .title = "Quit",
        .description = "Exit the application",
        .action = .quit,
    },
};

pub const HomeView = struct {
    cursor: usize = 0,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init() HomeView {
        return .{};
    }

    pub fn moveUp(self: *HomeView) void {
        if (self.cursor > 0) {
            self.cursor -= 1;
        }
    }

    pub fn moveDown(self: *HomeView) void {
        if (self.cursor < menu_items.len - 1) {
            self.cursor += 1;
        }
    }

    pub fn selectedAction(self: HomeView) Action {
        return menu_items[self.cursor].action;
    }

    pub fn render(self: HomeView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        // Logo
        try writer.writeAll("\n");
        try header.renderLogo(writer, self.style);
        try writer.writeAll("\n");

        // Tagline
        try self.style.muted(writer);
        try writer.writeAll("  What remains after burning away the unnecessary\n\n");
        try self.style.reset(writer);

        // Menu items
        for (menu_items, 0..) |item, i| {
            const is_current = i == self.cursor;

            if (is_current) {
                try self.style.primary(writer);
                try writer.writeAll("  > ");
            } else {
                try writer.writeAll("    ");
            }

            try self.style.primary(writer);
            if (is_current) {
                try writer.writeAll(ansi.style.bold);
            }
            try writer.writeAll(item.title);
            try self.style.reset(writer);
            try writer.writeAll("\n");

            try writer.writeAll("    ");
            try self.style.muted(writer);
            try writer.writeAll(item.description);
            try self.style.reset(writer);
            try writer.writeAll("\n\n");
        }

        // Footer
        try writer.writeAll("\n");
        try keybinds.renderFooter(writer, &keybinds.home_bindings, width, self.style);
    }
};

// Tests
test "HomeView navigation" {
    var view = HomeView.init();

    try std.testing.expectEqual(@as(usize, 0), view.cursor);
    try std.testing.expectEqual(Action.scan, view.selectedAction());

    view.moveDown();
    try std.testing.expectEqual(@as(usize, 1), view.cursor);
    try std.testing.expectEqual(Action.maintenance, view.selectedAction());

    view.moveUp();
    try std.testing.expectEqual(@as(usize, 0), view.cursor);
}
