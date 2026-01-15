const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const spinner = @import("../tui/components/spinner.zig");
const progress = @import("../tui/components/progress.zig");
const cleaner = @import("../cleaner/cleaner.zig");
const utils = @import("../utils.zig");

pub const CleaningView = struct {
    spinner_inst: spinner.Spinner,
    progress_bar: progress.ProgressBar,
    current: usize = 0,
    total: usize = 0,
    current_path: []const u8 = "",
    stats: ?cleaner.CleanStats = null,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init() CleaningView {
        return .{
            .spinner_inst = spinner.Spinner.init(.dots),
            .progress_bar = progress.ProgressBar.init(50),
        };
    }

    pub fn setProgress(self: *CleaningView, current: usize, total: usize, path: []const u8) void {
        self.current = current;
        self.total = total;
        self.current_path = path;

        if (total > 0) {
            self.progress_bar.setProgress(@as(f32, @floatFromInt(current)) / @as(f32, @floatFromInt(total)));
        }
    }

    pub fn setStats(self: *CleaningView, stats: cleaner.CleanStats) void {
        self.stats = stats;
    }

    pub fn tick(self: *CleaningView) void {
        self.spinner_inst.tick();
    }

    pub fn render(self: CleaningView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        try writer.writeAll("\n");
        try self.style.bold(writer);
        try self.style.primary(writer);
        try writer.writeAll("  Cleaning...\n\n");
        try self.style.reset(writer);

        // Spinner
        try writer.writeAll("  ");
        try self.spinner_inst.renderWithStyle(writer, self.style);
        try writer.writeAll("\n\n");

        // Progress
        if (self.total > 0) {
            try writer.writeAll("  ");
            try self.progress_bar.renderWithStyle(writer, self.style);
            try writer.print(" ({d}/{d})", .{ self.current, self.total });
            try writer.writeAll("\n\n");
        }

        // Current path
        if (self.current_path.len > 0) {
            try self.style.muted(writer);
            try writer.writeAll("  ");
            const max_path = @as(usize, width) -| 4;
            if (self.current_path.len > max_path) {
                try writer.writeAll("...");
                try writer.writeAll(self.current_path[self.current_path.len - max_path + 3 ..]);
            } else {
                try writer.writeAll(self.current_path);
            }
            try self.style.reset(writer);
            try writer.writeAll("\n");
        }
    }

    pub fn renderComplete(self: CleaningView, writer: anytype, width: u16, height: u16) !void {
        _ = width;
        _ = height;

        try writer.writeAll("\n");
        try self.style.bold(writer);
        try self.style.success(writer);
        try writer.writeAll("  Cleanup Complete!\n\n");
        try self.style.reset(writer);

        if (self.stats) |stats| {
            // Summary
            const cleaned_size_str = utils.formatSize(stats.cleaned_size);
            try self.style.secondary(writer);
            try writer.print("  Cleaned {d} items ({s})\n", .{
                stats.success_count,
                std.mem.sliceTo(&cleaned_size_str, 0),
            });
            try self.style.reset(writer);

            if (stats.failed_count > 0) {
                try self.style.warning(writer);
                try writer.print("  {d} items failed\n", .{stats.failed_count});
                try self.style.reset(writer);
            }

            // Duration
            const duration_ms = stats.duration_ns / 1_000_000;
            try self.style.muted(writer);
            try writer.print("\n  Completed in {d}ms\n", .{duration_ms});
            try self.style.reset(writer);
        }

        try writer.writeAll("\n  Press any key to continue...\n");
    }
};

// Tests
test "CleaningView" {
    var view = CleaningView.init();
    view.setProgress(5, 10, "/test/path");

    try std.testing.expectEqual(@as(usize, 5), view.current);
    try std.testing.expectEqual(@as(usize, 10), view.total);
}
