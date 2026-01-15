const std = @import("std");
const ansi = @import("ansi.zig");

/// Grayscale theme colors
pub const Theme = struct {
    // Base colors - grayscale (256-color palette indices)
    background: u8 = 232, // #0a0a0a (near black)
    surface: u8 = 234, // #171717 (elevated surface)
    surface_hover: u8 = 236, // #262626 (hover state)
    border: u8 = 240, // #404040 (subtle borders)
    border_focus: u8 = 244, // #737373 (focused borders)

    // Text hierarchy
    text_primary: u8 = 255, // #fafafa (primary content)
    text_secondary: u8 = 248, // #a3a3a3 (secondary content)
    text_muted: u8 = 242, // #525252 (disabled/hints)

    // Semantic colors (RGB for accents)
    success: [3]u8 = .{ 34, 197, 94 }, // #22c55e green
    warning: [3]u8 = .{ 245, 158, 11 }, // #f59e0b amber
    danger: [3]u8 = .{ 239, 68, 68 }, // #ef4444 red

    // Selection
    selected: u8 = 236, // #262626 (selected row bg)
    cursor: u8 = 255, // #fafafa (cursor indicator)

    /// Get foreground escape code for text_primary
    pub fn fgPrimary(self: Theme) [12]u8 {
        return ansi.fg256(self.text_primary);
    }

    /// Get foreground escape code for text_secondary
    pub fn fgSecondary(self: Theme) [12]u8 {
        return ansi.fg256(self.text_secondary);
    }

    /// Get foreground escape code for text_muted
    pub fn fgMuted(self: Theme) [12]u8 {
        return ansi.fg256(self.text_muted);
    }

    /// Get foreground escape code for border
    pub fn fgBorder(self: Theme) [12]u8 {
        return ansi.fg256(self.border);
    }

    /// Get foreground escape code for success
    pub fn fgSuccess(self: Theme) [20]u8 {
        return ansi.fgRgb(self.success[0], self.success[1], self.success[2]);
    }

    /// Get foreground escape code for warning
    pub fn fgWarning(self: Theme) [20]u8 {
        return ansi.fgRgb(self.warning[0], self.warning[1], self.warning[2]);
    }

    /// Get foreground escape code for danger
    pub fn fgDanger(self: Theme) [20]u8 {
        return ansi.fgRgb(self.danger[0], self.danger[1], self.danger[2]);
    }

    /// Get background escape code for surface
    pub fn bgSurface(self: Theme) [12]u8 {
        return ansi.bg256(self.surface);
    }

    /// Get background escape code for selected
    pub fn bgSelected(self: Theme) [12]u8 {
        return ansi.bg256(self.selected);
    }
};

/// Default grayscale theme
pub const default_theme = Theme{};

/// Style helpers for common patterns
pub const Style = struct {
    theme: Theme,

    pub fn init(theme: Theme) Style {
        return .{ .theme = theme };
    }

    /// Write reset code
    pub fn reset(self: Style, writer: anytype) !void {
        _ = self;
        try writer.writeAll(ansi.style.reset);
    }

    /// Write bold style
    pub fn bold(self: Style, writer: anytype) !void {
        _ = self;
        try writer.writeAll(ansi.style.bold);
    }

    /// Write dim style
    pub fn dim(self: Style, writer: anytype) !void {
        _ = self;
        try writer.writeAll(ansi.style.dim);
    }

    /// Write primary text style
    pub fn primary(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgPrimary());
    }

    /// Write secondary text style
    pub fn secondary(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgSecondary());
    }

    /// Write muted text style
    pub fn muted(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgMuted());
    }

    /// Write success text style
    pub fn success(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgSuccess());
    }

    /// Write warning text style
    pub fn warning(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgWarning());
    }

    /// Write danger text style
    pub fn danger(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgDanger());
    }

    /// Write border style
    pub fn border(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.fgBorder());
    }

    /// Write selected background
    pub fn selectedBg(self: Style, writer: anytype) !void {
        try writer.writeAll(&self.theme.bgSelected());
    }

    /// Write styled text (convenience function)
    pub fn write(self: Style, writer: anytype, comptime fmt: []const u8, args: anytype) !void {
        try writer.print(fmt, args);
        try self.reset(writer);
    }
};

// Tests
test "Theme defaults" {
    const theme = default_theme;
    try std.testing.expectEqual(@as(u8, 232), theme.background);
    try std.testing.expectEqual(@as(u8, 255), theme.text_primary);
}

test "Style" {
    var buf: [256]u8 = undefined;
    var stream = std.io.fixedBufferStream(&buf);
    const writer = stream.writer();

    const style = Style.init(default_theme);
    try style.primary(writer);

    const written = stream.getWritten();
    try std.testing.expect(written.len > 0);
}
