const std = @import("std");

/// ANSI escape code constants
pub const ESC = "\x1b";
pub const CSI = ESC ++ "[";

/// Cursor control
pub const cursor = struct {
    pub const hide = CSI ++ "?25l";
    pub const show = CSI ++ "?25h";
    pub const home = CSI ++ "H";
    pub const save = CSI ++ "s";
    pub const restore = CSI ++ "u";

    pub fn moveTo(row: u16, col: u16) [16]u8 {
        var buf: [16]u8 = undefined;
        _ = std.fmt.bufPrint(&buf, CSI ++ "{d};{d}H", .{ row, col }) catch {};
        return buf;
    }

    pub fn moveUp(n: u16) [12]u8 {
        var buf: [12]u8 = undefined;
        _ = std.fmt.bufPrint(&buf, CSI ++ "{d}A", .{n}) catch {};
        return buf;
    }

    pub fn moveDown(n: u16) [12]u8 {
        var buf: [12]u8 = undefined;
        _ = std.fmt.bufPrint(&buf, CSI ++ "{d}B", .{n}) catch {};
        return buf;
    }

    pub fn moveRight(n: u16) [12]u8 {
        var buf: [12]u8 = undefined;
        _ = std.fmt.bufPrint(&buf, CSI ++ "{d}C", .{n}) catch {};
        return buf;
    }

    pub fn moveLeft(n: u16) [12]u8 {
        var buf: [12]u8 = undefined;
        _ = std.fmt.bufPrint(&buf, CSI ++ "{d}D", .{n}) catch {};
        return buf;
    }
};

/// Screen control
pub const screen = struct {
    pub const clear = CSI ++ "2J";
    pub const clear_line = CSI ++ "2K";
    pub const clear_to_end = CSI ++ "J";
    pub const clear_to_line_end = CSI ++ "K";
    pub const alt_buffer_on = CSI ++ "?1049h";
    pub const alt_buffer_off = CSI ++ "?1049l";
};

/// Text styles
pub const style = struct {
    pub const reset = CSI ++ "0m";
    pub const bold = CSI ++ "1m";
    pub const dim = CSI ++ "2m";
    pub const italic = CSI ++ "3m";
    pub const underline = CSI ++ "4m";
    pub const blink = CSI ++ "5m";
    pub const reverse = CSI ++ "7m";
    pub const hidden = CSI ++ "8m";
    pub const strikethrough = CSI ++ "9m";

    pub const bold_off = CSI ++ "22m";
    pub const italic_off = CSI ++ "23m";
    pub const underline_off = CSI ++ "24m";
    pub const blink_off = CSI ++ "25m";
    pub const reverse_off = CSI ++ "27m";
    pub const hidden_off = CSI ++ "28m";
    pub const strikethrough_off = CSI ++ "29m";
};

/// 256-color foreground
pub fn fg256(color: u8) [12]u8 {
    var buf: [12]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "38;5;{d}m", .{color}) catch {};
    return buf;
}

/// 256-color background
pub fn bg256(color: u8) [12]u8 {
    var buf: [12]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "48;5;{d}m", .{color}) catch {};
    return buf;
}

/// True color (24-bit) foreground
pub fn fgRgb(r: u8, g: u8, b: u8) [20]u8 {
    var buf: [20]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "38;2;{d};{d};{d}m", .{ r, g, b }) catch {};
    return buf;
}

/// True color (24-bit) background
pub fn bgRgb(r: u8, g: u8, b: u8) [20]u8 {
    var buf: [20]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "48;2;{d};{d};{d}m", .{ r, g, b }) catch {};
    return buf;
}

/// Basic colors (0-7)
pub const Color = enum(u8) {
    black = 0,
    red = 1,
    green = 2,
    yellow = 3,
    blue = 4,
    magenta = 5,
    cyan = 6,
    white = 7,
    default = 9,
};

/// Basic foreground color
pub fn fg(color: Color) [8]u8 {
    var buf: [8]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "3{d}m", .{@intFromEnum(color)}) catch {};
    return buf;
}

/// Basic background color
pub fn bg(color: Color) [8]u8 {
    var buf: [8]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "4{d}m", .{@intFromEnum(color)}) catch {};
    return buf;
}

/// Bright foreground color
pub fn fgBright(color: Color) [8]u8 {
    var buf: [8]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "9{d}m", .{@intFromEnum(color)}) catch {};
    return buf;
}

/// Bright background color
pub fn bgBright(color: Color) [8]u8 {
    var buf: [8]u8 = undefined;
    @memset(&buf, 0);
    _ = std.fmt.bufPrint(&buf, CSI ++ "10{d}m", .{@intFromEnum(color)}) catch {};
    return buf;
}

/// Box drawing characters
pub const box = struct {
    pub const horizontal = "\u{2500}"; // ─
    pub const vertical = "\u{2502}"; // │
    pub const top_left = "\u{250C}"; // ┌
    pub const top_right = "\u{2510}"; // ┐
    pub const bottom_left = "\u{2514}"; // └
    pub const bottom_right = "\u{2518}"; // ┘
    pub const t_left = "\u{251C}"; // ├
    pub const t_right = "\u{2524}"; // ┤
    pub const t_top = "\u{252C}"; // ┬
    pub const t_bottom = "\u{2534}"; // ┴
    pub const cross = "\u{253C}"; // ┼

    // Double line variants
    pub const horizontal_double = "\u{2550}"; // ═
    pub const vertical_double = "\u{2551}"; // ║
    pub const top_left_double = "\u{2554}"; // ╔
    pub const top_right_double = "\u{2557}"; // ╗
    pub const bottom_left_double = "\u{255A}"; // ╚
    pub const bottom_right_double = "\u{255D}"; // ╝
};

/// Progress bar characters
pub const progress = struct {
    pub const full = "\u{2588}"; // █
    pub const seven_eighths = "\u{2589}"; // ▉
    pub const three_quarters = "\u{258A}"; // ▊
    pub const five_eighths = "\u{258B}"; // ▋
    pub const half = "\u{258C}"; // ▌
    pub const three_eighths = "\u{258D}"; // ▍
    pub const quarter = "\u{258E}"; // ▎
    pub const eighth = "\u{258F}"; // ▏
    pub const empty = "\u{2591}"; // ░
};

/// Spinner frames
pub const spinner_frames = struct {
    pub const dots = [_][]const u8{ "⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏" };
    pub const line = [_][]const u8{ "-", "\\", "|", "/" };
    pub const circle = [_][]const u8{ "○", "◔", "◑", "◕", "●" };
    pub const block = [_][]const u8{ "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█" };
    pub const arrows = [_][]const u8{ "←", "↖", "↑", "↗", "→", "↘", "↓", "↙" };
};

/// Checkbox characters
pub const checkbox = struct {
    pub const checked = "[x]";
    pub const unchecked = "[ ]";
    pub const checked_fancy = "[\u{2713}]"; // [✓]
    pub const unchecked_fancy = "[\u{2717}]"; // [✗]
};

/// Other symbols
pub const symbols = struct {
    pub const bullet = "\u{2022}"; // •
    pub const arrow_right = "\u{2192}"; // →
    pub const arrow_left = "\u{2190}"; // ←
    pub const arrow_up = "\u{2191}"; // ↑
    pub const arrow_down = "\u{2193}"; // ↓
    pub const check = "\u{2713}"; // ✓
    pub const cross = "\u{2717}"; // ✗
    pub const warning = "\u{26A0}"; // ⚠
    pub const info = "\u{2139}"; // ℹ
    pub const ellipsis = "\u{2026}"; // …
};

// Tests
test "fg256" {
    const result = fg256(255);
    try std.testing.expect(result[0] == '\x1b');
}

test "fgRgb" {
    const result = fgRgb(255, 128, 64);
    try std.testing.expect(result[0] == '\x1b');
}
