const std = @import("std");
const ansi = @import("../ansi.zig");
const theme = @import("../theme.zig");

pub const SpinnerType = enum {
    dots,
    line,
    circle,
    block,
    arrows,
};

pub const Spinner = struct {
    frame: usize = 0,
    spinner_type: SpinnerType = .dots,
    label: []const u8 = "",

    pub fn init(spinner_type: SpinnerType) Spinner {
        return .{ .spinner_type = spinner_type };
    }

    pub fn tick(self: *Spinner) void {
        const frames = self.getFrames();
        self.frame = (self.frame + 1) % frames.len;
    }

    pub fn setLabel(self: *Spinner, label: []const u8) void {
        self.label = label;
    }

    fn getFrames(self: Spinner) []const []const u8 {
        return switch (self.spinner_type) {
            .dots => &ansi.spinner_frames.dots,
            .line => &ansi.spinner_frames.line,
            .circle => &ansi.spinner_frames.circle,
            .block => &ansi.spinner_frames.block,
            .arrows => &ansi.spinner_frames.arrows,
        };
    }

    pub fn currentFrame(self: Spinner) []const u8 {
        const frames = self.getFrames();
        return frames[self.frame % frames.len];
    }

    pub fn render(self: Spinner, writer: anytype) !void {
        try writer.writeAll(self.currentFrame());
        if (self.label.len > 0) {
            try writer.writeAll(" ");
            try writer.writeAll(self.label);
        }
    }

    pub fn renderWithStyle(self: Spinner, writer: anytype, style: theme.Style) !void {
        try style.primary(writer);
        try writer.writeAll(self.currentFrame());
        try style.reset(writer);
        if (self.label.len > 0) {
            try writer.writeAll(" ");
            try style.secondary(writer);
            try writer.writeAll(self.label);
            try style.reset(writer);
        }
    }
};

// Tests
test "Spinner" {
    var spinner = Spinner.init(.dots);
    spinner.setLabel("Loading...");

    try std.testing.expectEqualStrings(ansi.spinner_frames.dots[0], spinner.currentFrame());

    spinner.tick();
    try std.testing.expectEqualStrings(ansi.spinner_frames.dots[1], spinner.currentFrame());
}
