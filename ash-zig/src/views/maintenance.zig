const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const keybinds = @import("../tui/components/keybinds.zig");
const maintenance = @import("../maintenance.zig");

pub const MaintenanceView = struct {
    cursor: usize = 0,
    results: [maintenance.commands.len]?maintenance.CommandResult = .{null} ** maintenance.commands.len,
    running: ?usize = null,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init() MaintenanceView {
        return .{};
    }

    pub fn moveUp(self: *MaintenanceView) void {
        if (self.cursor > 0) {
            self.cursor -= 1;
        }
    }

    pub fn moveDown(self: *MaintenanceView) void {
        if (self.cursor < maintenance.commands.len - 1) {
            self.cursor += 1;
        }
    }

    pub fn selectedCommand(self: MaintenanceView) *const maintenance.Command {
        return &maintenance.commands[self.cursor];
    }

    pub fn setRunning(self: *MaintenanceView, index: ?usize) void {
        self.running = index;
    }

    pub fn setResult(self: *MaintenanceView, index: usize, result: maintenance.CommandResult) void {
        self.results[index] = result;
    }

    pub fn render(self: MaintenanceView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        try writer.writeAll("\n");
        try self.style.bold(writer);
        try self.style.primary(writer);
        try writer.writeAll("  System Maintenance\n\n");
        try self.style.reset(writer);

        // Command list
        for (maintenance.commands, 0..) |cmd, i| {
            const is_current = i == self.cursor;
            const is_running = self.running == i;
            const result = self.results[i];

            // Cursor
            if (is_current) {
                try self.style.primary(writer);
                try writer.writeAll("  > ");
            } else {
                try writer.writeAll("    ");
            }

            // Status indicator
            if (is_running) {
                try self.style.warning(writer);
                try writer.writeAll("[...] ");
            } else if (result) |r| {
                if (r.success) {
                    try self.style.success(writer);
                    try writer.writeAll("[OK]  ");
                } else {
                    try self.style.danger(writer);
                    try writer.writeAll("[ERR] ");
                }
            } else {
                try self.style.muted(writer);
                try writer.writeAll("      ");
            }

            // Command name
            if (is_current) {
                try self.style.bold(writer);
                try self.style.primary(writer);
            } else {
                try self.style.secondary(writer);
            }
            try writer.writeAll(cmd.name);

            // Sudo indicator
            if (cmd.requires_sudo) {
                try self.style.muted(writer);
                try writer.writeAll(" [sudo]");
            }
            try self.style.reset(writer);
            try writer.writeAll("\n");

            // Description
            try writer.writeAll("          ");
            try self.style.muted(writer);
            try writer.writeAll(cmd.description);
            try self.style.reset(writer);

            // Result details
            if (result) |r| {
                if (r.success) {
                    try self.style.success(writer);
                    const duration_ms = r.duration_ns / 1_000_000;
                    try writer.print(" (completed in {d}ms)", .{duration_ms});
                } else {
                    try self.style.danger(writer);
                    if (r.errorMsg()) |err| {
                        try writer.print(" - {s}", .{err});
                    }
                }
                try self.style.reset(writer);
            }

            try writer.writeAll("\n\n");
        }

        // Footer
        const bindings = [_]keybinds.KeyBind{
            .{ .key = "j/k", .description = "navigate" },
            .{ .key = "enter", .description = "run" },
            .{ .key = "esc", .description = "back" },
            .{ .key = "q", .description = "quit" },
        };
        try keybinds.renderFooter(writer, &bindings, width, self.style);
    }
};

// Tests
test "MaintenanceView" {
    var view = MaintenanceView.init();

    try std.testing.expectEqual(@as(usize, 0), view.cursor);

    view.moveDown();
    try std.testing.expectEqual(@as(usize, 1), view.cursor);

    view.moveUp();
    try std.testing.expectEqual(@as(usize, 0), view.cursor);
}
