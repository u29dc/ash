const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const spinner = @import("../tui/components/spinner.zig");
const progress = @import("../tui/components/progress.zig");

pub const ScanView = struct {
    spinner_inst: spinner.Spinner,
    progress_bar: progress.ProgressBar,
    message: []const u8 = "Scanning...",
    current_path: []const u8 = "",
    progress_value: f32 = 0.0,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init() ScanView {
        return .{
            .spinner_inst = spinner.Spinner.init(.dots),
            .progress_bar = progress.ProgressBar.init(50),
        };
    }

    pub fn setMessage(self: *ScanView, message: []const u8) void {
        self.message = message;
        self.spinner_inst.setLabel(message);
    }

    pub fn setCurrent(self: *ScanView, path: []const u8) void {
        self.current_path = path;
    }

    pub fn setProgress(self: *ScanView, value: f32) void {
        self.progress_value = value;
        self.progress_bar.setProgress(value);
    }

    pub fn tick(self: *ScanView) void {
        self.spinner_inst.tick();
    }

    pub fn render(self: ScanView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        // Title
        try writer.writeAll("\n");
        try self.style.bold(writer);
        try self.style.primary(writer);
        try writer.writeAll("  Scanning for Junk Files\n\n");
        try self.style.reset(writer);

        // Spinner with message
        try writer.writeAll("  ");
        try self.spinner_inst.renderWithStyle(writer, self.style);
        try writer.writeAll("\n\n");

        // Progress bar
        try writer.writeAll("  ");
        try self.progress_bar.renderWithStyle(writer, self.style);
        try writer.writeAll("\n\n");

        // Current path
        if (self.current_path.len > 0) {
            try self.style.muted(writer);
            try writer.writeAll("  ");

            // Truncate path to fit width
            const max_path_len = @as(usize, width) -| 4;
            if (self.current_path.len > max_path_len) {
                try writer.writeAll("...");
                try writer.writeAll(self.current_path[self.current_path.len - max_path_len + 3 ..]);
            } else {
                try writer.writeAll(self.current_path);
            }
            try self.style.reset(writer);
        }

        try writer.writeAll("\n");
    }
};

// Tests
test "ScanView" {
    var view = ScanView.init();
    view.setMessage("Scanning caches...");
    view.setProgress(0.5);

    try std.testing.expectEqualStrings("Scanning caches...", view.message);
    try std.testing.expectEqual(@as(f32, 0.5), view.progress_value);
}
