const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");

pub const ToastType = enum {
    info,
    success,
    warning,
    err,
};

pub const Toast = struct {
    message: []const u8 = "",
    toast_type: ToastType = .info,
    visible: bool = false,
    expires_at: i64 = 0,
    duration_ms: i64 = 3000,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init() Toast {
        return .{};
    }

    pub fn show(self: *Toast, message: []const u8, toast_type: ToastType) void {
        self.message = message;
        self.toast_type = toast_type;
        self.visible = true;
        self.expires_at = std.time.milliTimestamp() + self.duration_ms;
    }

    pub fn showInfo(self: *Toast, message: []const u8) void {
        self.show(message, .info);
    }

    pub fn showSuccess(self: *Toast, message: []const u8) void {
        self.show(message, .success);
    }

    pub fn showWarning(self: *Toast, message: []const u8) void {
        self.show(message, .warning);
    }

    pub fn showError(self: *Toast, message: []const u8) void {
        self.show(message, .err);
    }

    pub fn hide(self: *Toast) void {
        self.visible = false;
    }

    pub fn isVisible(self: Toast) bool {
        if (!self.visible) return false;
        return std.time.milliTimestamp() < self.expires_at;
    }

    pub fn update(self: *Toast) void {
        if (self.visible and std.time.milliTimestamp() >= self.expires_at) {
            self.visible = false;
        }
    }

    pub fn render(self: Toast, writer: anytype) !void {
        if (!self.isVisible()) return;

        // Icon based on type
        const icon = switch (self.toast_type) {
            .info => "i",
            .success => "+",
            .warning => "!",
            .err => "x",
        };

        // Style based on type
        switch (self.toast_type) {
            .info => try self.style.secondary(writer),
            .success => try self.style.success(writer),
            .warning => try self.style.warning(writer),
            .err => try self.style.danger(writer),
        }

        try writer.writeAll("[");
        try writer.writeAll(icon);
        try writer.writeAll("] ");
        try writer.writeAll(self.message);
        try self.style.reset(writer);
    }

    pub fn renderCentered(self: Toast, writer: anytype, width: u16) !void {
        if (!self.isVisible()) return;

        const msg_len = self.message.len + 4; // "[x] " prefix
        const padding = if (width > msg_len) (width - @as(u16, @intCast(msg_len))) / 2 else 0;

        for (0..padding) |_| {
            try writer.writeAll(" ");
        }
        try self.render(writer);
    }
};

// Tests
test "Toast" {
    var toast = Toast.init();
    toast.showSuccess("Test message");

    try std.testing.expect(toast.visible);
    try std.testing.expectEqual(ToastType.success, toast.toast_type);
    try std.testing.expectEqualStrings("Test message", toast.message);
}
