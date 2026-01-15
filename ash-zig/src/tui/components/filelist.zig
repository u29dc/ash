const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");
const scanner = @import("../../scanner/scanner.zig");
const utils = @import("../../utils.zig");

pub const FileList = struct {
    entries: []scanner.Entry = &.{},
    cursor: usize = 0,
    offset: usize = 0,
    page_size: usize = 20,
    width: u16 = 80,
    style: theme.Style = theme.Style.init(theme.default_theme),

    pub fn init(page_size: usize, width: u16) FileList {
        return .{
            .page_size = page_size,
            .width = width,
        };
    }

    pub fn setEntries(self: *FileList, entries: []scanner.Entry) void {
        self.entries = entries;
        self.cursor = 0;
        self.offset = 0;
    }

    pub fn setPageSize(self: *FileList, size: usize) void {
        self.page_size = size;
    }

    pub fn setWidth(self: *FileList, width: u16) void {
        self.width = width;
    }

    pub fn moveUp(self: *FileList) void {
        if (self.cursor > 0) {
            self.cursor -= 1;
            if (self.cursor < self.offset) {
                self.offset = self.cursor;
            }
        }
    }

    pub fn moveDown(self: *FileList) void {
        if (self.cursor < self.entries.len -| 1) {
            self.cursor += 1;
            if (self.cursor >= self.offset + self.page_size) {
                self.offset = self.cursor - self.page_size + 1;
            }
        }
    }

    pub fn pageUp(self: *FileList) void {
        if (self.cursor >= self.page_size) {
            self.cursor -= self.page_size;
            self.offset = if (self.offset >= self.page_size) self.offset - self.page_size else 0;
        } else {
            self.cursor = 0;
            self.offset = 0;
        }
    }

    pub fn pageDown(self: *FileList) void {
        const remaining = self.entries.len -| self.cursor;
        if (remaining > self.page_size) {
            self.cursor += self.page_size;
            self.offset += self.page_size;
        } else if (self.entries.len > 0) {
            self.cursor = self.entries.len - 1;
            self.offset = if (self.entries.len > self.page_size) self.entries.len - self.page_size else 0;
        }
    }

    pub fn toggle(self: *FileList) void {
        if (self.cursor < self.entries.len) {
            self.entries[self.cursor].selected = !self.entries[self.cursor].selected;
        }
    }

    pub fn selectAll(self: *FileList) void {
        for (self.entries) |*entry| {
            entry.selected = true;
        }
    }

    pub fn deselectAll(self: *FileList) void {
        for (self.entries) |*entry| {
            entry.selected = false;
        }
    }

    pub fn toggleAll(self: *FileList) void {
        // If all selected, deselect all; otherwise select all
        var all_selected = true;
        for (self.entries) |entry| {
            if (!entry.selected) {
                all_selected = false;
                break;
            }
        }

        if (all_selected) {
            self.deselectAll();
        } else {
            self.selectAll();
        }
    }

    pub fn selectedCount(self: FileList) usize {
        var count: usize = 0;
        for (self.entries) |entry| {
            if (entry.selected) count += 1;
        }
        return count;
    }

    pub fn selectedSize(self: FileList) u64 {
        var total: u64 = 0;
        for (self.entries) |entry| {
            if (entry.selected) total += entry.size;
        }
        return total;
    }

    pub fn currentEntry(self: FileList) ?*scanner.Entry {
        if (self.cursor < self.entries.len) {
            return &self.entries[self.cursor];
        }
        return null;
    }

    pub fn render(self: FileList, writer: anytype) !void {
        const end_idx = @min(self.offset + self.page_size, self.entries.len);

        for (self.offset..end_idx) |i| {
            const entry = &self.entries[i];
            const is_current = i == self.cursor;

            // Cursor indicator
            if (is_current) {
                try self.style.primary(writer);
                try writer.writeAll("> ");
            } else {
                try writer.writeAll("  ");
            }

            // Checkbox
            if (entry.selected) {
                try self.style.success(writer);
                try writer.writeAll("[x] ");
            } else {
                try self.style.muted(writer);
                try writer.writeAll("[ ] ");
            }

            // Risk indicator
            switch (entry.risk) {
                .safe => {
                    try self.style.success(writer);
                    try writer.writeAll("o ");
                },
                .caution => {
                    try self.style.warning(writer);
                    try writer.writeAll("! ");
                },
                .dangerous => {
                    try self.style.danger(writer);
                    try writer.writeAll("X ");
                },
            }

            // Name (truncated)
            if (is_current) {
                try self.style.primary(writer);
            } else {
                try self.style.secondary(writer);
            }

            const max_name_len = @as(usize, self.width) -| 24;
            const name = entry.name();
            if (name.len > max_name_len) {
                try writer.writeAll(name[0 .. max_name_len - 3]);
                try writer.writeAll("...");
            } else {
                try writer.writeAll(name);
                for (0..max_name_len - name.len) |_| {
                    try writer.writeAll(" ");
                }
            }

            // Size
            try self.style.muted(writer);
            const size_str = utils.formatSize(entry.size);
            try writer.print(" {s: >10}", .{std.mem.sliceTo(&size_str, 0)});

            try self.style.reset(writer);
            try writer.writeAll("\n");
        }

        // Scroll indicator
        if (self.entries.len > self.page_size) {
            try self.style.muted(writer);
            try writer.print("\n  [{d}-{d} of {d}]", .{ self.offset + 1, end_idx, self.entries.len });
            try self.style.reset(writer);
        }
    }
};

// Tests
test "FileList navigation" {
    var list = FileList.init(5, 80);

    var entries = [_]scanner.Entry{
        .{},
        .{},
        .{},
    };
    entries[0].setName("file1");
    entries[1].setName("file2");
    entries[2].setName("file3");

    list.setEntries(&entries);

    try std.testing.expectEqual(@as(usize, 0), list.cursor);

    list.moveDown();
    try std.testing.expectEqual(@as(usize, 1), list.cursor);

    list.moveUp();
    try std.testing.expectEqual(@as(usize, 0), list.cursor);
}

test "FileList selection" {
    var list = FileList.init(5, 80);

    var entries = [_]scanner.Entry{
        .{},
        .{},
    };
    entries[0].setName("file1");
    entries[0].size = 1000;
    entries[1].setName("file2");
    entries[1].size = 2000;

    list.setEntries(&entries);

    list.toggle();
    try std.testing.expectEqual(@as(usize, 1), list.selectedCount());
    try std.testing.expectEqual(@as(u64, 1000), list.selectedSize());

    list.selectAll();
    try std.testing.expectEqual(@as(usize, 2), list.selectedCount());
}
