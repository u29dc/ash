const std = @import("std");
const ansi = @import("../tui/ansi.zig");
const theme = @import("../tui/theme.zig");
const filelist = @import("../tui/components/filelist.zig");
const keybinds = @import("../tui/components/keybinds.zig");
const scanner = @import("../scanner/scanner.zig");
const utils = @import("../utils.zig");

pub const ResultsView = struct {
    file_list: filelist.FileList,
    total_size: u64 = 0,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init(page_size: usize, width: u16) ResultsView {
        return .{
            .file_list = filelist.FileList.init(page_size, width),
        };
    }

    pub fn setEntries(self: *ResultsView, entries: []scanner.Entry) void {
        self.file_list.setEntries(entries);
        self.total_size = 0;
        for (entries) |entry| {
            self.total_size += entry.size;
        }
    }

    pub fn setPageSize(self: *ResultsView, size: usize) void {
        self.file_list.setPageSize(size);
    }

    pub fn setWidth(self: *ResultsView, width: u16) void {
        self.file_list.setWidth(width);
    }

    pub fn moveUp(self: *ResultsView) void {
        self.file_list.moveUp();
    }

    pub fn moveDown(self: *ResultsView) void {
        self.file_list.moveDown();
    }

    pub fn pageUp(self: *ResultsView) void {
        self.file_list.pageUp();
    }

    pub fn pageDown(self: *ResultsView) void {
        self.file_list.pageDown();
    }

    pub fn toggle(self: *ResultsView) void {
        self.file_list.toggle();
    }

    pub fn selectAll(self: *ResultsView) void {
        self.file_list.selectAll();
    }

    pub fn deselectAll(self: *ResultsView) void {
        self.file_list.deselectAll();
    }

    pub fn toggleAll(self: *ResultsView) void {
        self.file_list.toggleAll();
    }

    pub fn selectedCount(self: ResultsView) usize {
        return self.file_list.selectedCount();
    }

    pub fn selectedSize(self: ResultsView) u64 {
        return self.file_list.selectedSize();
    }

    pub fn hasSelection(self: ResultsView) bool {
        return self.selectedCount() > 0;
    }

    pub fn entries(self: *ResultsView) []scanner.Entry {
        return self.file_list.entries;
    }

    pub fn render(self: ResultsView, writer: anytype, width: u16, height: u16) !void {
        _ = height;

        // Title
        try writer.writeAll("\n");
        try self.style.bold(writer);
        try self.style.primary(writer);
        try writer.writeAll("  Scan Results\n\n");
        try self.style.reset(writer);

        // Summary line
        try writer.writeAll("  ");
        try self.style.secondary(writer);
        const total_size_str = utils.formatSize(self.total_size);
        try writer.print("Found {d} items ({s})", .{
            self.file_list.entries.len,
            std.mem.sliceTo(&total_size_str, 0),
        });

        if (self.selectedCount() > 0) {
            try writer.writeAll(" | ");
            try self.style.success(writer);
            const selected_size_str = utils.formatSize(self.selectedSize());
            try writer.print("Selected {d} items ({s})", .{
                self.selectedCount(),
                std.mem.sliceTo(&selected_size_str, 0),
            });
        }
        try self.style.reset(writer);
        try writer.writeAll("\n\n");

        // File list
        try self.file_list.render(writer);
        try writer.writeAll("\n\n");

        // Footer
        try keybinds.renderFooter(writer, &keybinds.results_bindings, width, self.style);
    }
};

// Tests
test "ResultsView" {
    var view = ResultsView.init(10, 80);

    var entries = [_]scanner.Entry{
        .{},
        .{},
    };
    entries[0].setName("file1");
    entries[0].size = 1000;
    entries[1].setName("file2");
    entries[1].size = 2000;

    view.setEntries(&entries);

    try std.testing.expectEqual(@as(u64, 3000), view.total_size);
    try std.testing.expectEqual(@as(usize, 0), view.selectedCount());

    view.toggle();
    try std.testing.expectEqual(@as(usize, 1), view.selectedCount());
}
