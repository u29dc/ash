const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const keybinds = @import("../tui/components/keybinds.zig");
const utils = @import("../utils.zig");

pub const ConfirmView = struct {
    item_count: usize = 0,
    total_size: u64 = 0,
    cursor: usize = 0, // 0 = Cancel, 1 = Confirm
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init(item_count: usize, total_size: u64) ConfirmView {
        return .{
            .item_count = item_count,
            .total_size = total_size,
        };
    }

    pub fn moveLeft(self: *ConfirmView) void {
        if (self.cursor > 0) self.cursor -= 1;
    }

    pub fn moveRight(self: *ConfirmView) void {
        if (self.cursor < 1) self.cursor += 1;
    }

    pub fn toggle(self: *ConfirmView) void {
        self.cursor = 1 - self.cursor;
    }

    pub fn isConfirmed(self: ConfirmView) bool {
        return self.cursor == 1;
    }

    pub fn render(self: ConfirmView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        // Calculate dialog dimensions
        const dialog_width: u16 = 50;
        const dialog_height: u16 = 10;
        const start_col = if (width > dialog_width) (width - dialog_width) / 2 else 1;

        // Draw dialog box
        try writer.writeAll("\n\n");

        // Top border
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.top_left);
        for (0..dialog_width - 2) |_| try writer.writeAll(ansi.box.horizontal);
        try writer.writeAll(ansi.box.top_right);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Title line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try self.style.bold(writer);
        try self.style.primary(writer);
        const title = " Confirm Cleanup ";
        const title_padding = (dialog_width - 2 - title.len) / 2;
        for (0..title_padding) |_| try writer.writeAll(" ");
        try writer.writeAll(title);
        for (0..dialog_width - 2 - title_padding - title.len) |_| try writer.writeAll(" ");
        try self.style.reset(writer);
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Empty line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        for (0..dialog_width - 2) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Message line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);

        const size_str = utils.formatSize(self.total_size);
        var msg_buf: [64]u8 = undefined;
        const msg = std.fmt.bufPrint(&msg_buf, " Move {d} items ({s}) to Trash? ", .{
            self.item_count,
            std.mem.sliceTo(&size_str, 0),
        }) catch "Move items to Trash?";

        try self.style.secondary(writer);
        const msg_padding = if (dialog_width > msg.len + 2) (dialog_width - 2 - msg.len) / 2 else 0;
        for (0..msg_padding) |_| try writer.writeAll(" ");
        try writer.writeAll(msg);
        for (0..dialog_width - 2 - msg_padding - msg.len) |_| try writer.writeAll(" ");
        try self.style.reset(writer);
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Note line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try self.style.muted(writer);
        const note = " Items can be recovered from Trash. ";
        const note_padding = if (dialog_width > note.len + 2) (dialog_width - 2 - note.len) / 2 else 0;
        for (0..note_padding) |_| try writer.writeAll(" ");
        try writer.writeAll(note);
        for (0..dialog_width - 2 - note_padding - note.len) |_| try writer.writeAll(" ");
        try self.style.reset(writer);
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Empty line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        for (0..dialog_width - 2) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Buttons line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);

        // Cancel button
        const cancel_str = " Cancel ";
        const clean_str = " Clean ";
        const buttons_len = cancel_str.len + clean_str.len + 4;
        const btn_padding = if (dialog_width > buttons_len + 2) (dialog_width - 2 - buttons_len) / 2 else 0;

        for (0..btn_padding) |_| try writer.writeAll(" ");

        if (self.cursor == 0) {
            try self.style.bold(writer);
            try self.style.primary(writer);
            try writer.writeAll("[");
            try writer.writeAll(cancel_str);
            try writer.writeAll("]");
            try self.style.reset(writer);
        } else {
            try self.style.muted(writer);
            try writer.writeAll(" ");
            try writer.writeAll(cancel_str);
            try writer.writeAll(" ");
            try self.style.reset(writer);
        }

        try writer.writeAll("  ");

        if (self.cursor == 1) {
            try self.style.bold(writer);
            try self.style.danger(writer);
            try writer.writeAll("[");
            try writer.writeAll(clean_str);
            try writer.writeAll("]");
            try self.style.reset(writer);
        } else {
            try self.style.muted(writer);
            try writer.writeAll(" ");
            try writer.writeAll(clean_str);
            try writer.writeAll(" ");
            try self.style.reset(writer);
        }

        for (0..dialog_width - 2 - btn_padding - buttons_len) |_| try writer.writeAll(" ");

        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Empty line
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        for (0..dialog_width - 2) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.vertical);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Bottom border
        for (0..start_col) |_| try writer.writeAll(" ");
        try self.style.warning(writer);
        try writer.writeAll(ansi.box.bottom_left);
        for (0..dialog_width - 2) |_| try writer.writeAll(ansi.box.horizontal);
        try writer.writeAll(ansi.box.bottom_right);
        try self.style.reset(writer);
        try writer.writeAll("\n");

        // Key hints
        try writer.writeAll("\n");
        try keybinds.renderFooter(writer, &keybinds.confirm_bindings, dialog_width, self.style);
        _ = dialog_height;
    }
};

// Tests
test "ConfirmView" {
    var view = ConfirmView.init(5, 1024 * 1024);

    try std.testing.expectEqual(@as(usize, 0), view.cursor);
    try std.testing.expect(!view.isConfirmed());

    view.moveRight();
    try std.testing.expect(view.isConfirmed());

    view.toggle();
    try std.testing.expect(!view.isConfirmed());
}
