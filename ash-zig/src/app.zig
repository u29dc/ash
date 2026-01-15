const std = @import("std");
const builtin = @import("builtin");

// TUI imports
const terminal = @import("tui/terminal.zig");
const ansi = @import("tui/ansi.zig");
const theme_mod = @import("tui/theme.zig");
const render = @import("tui/render.zig");

// View imports
const home_view = @import("views/home.zig");
const scanning_view = @import("views/scanning.zig");
const results_view = @import("views/results.zig");
const confirm_view = @import("views/confirm.zig");
const cleaning_view = @import("views/cleaning.zig");
const maintenance_view = @import("views/maintenance.zig");

// Core imports
const scanner = @import("scanner/scanner.zig");
const caches = @import("scanner/caches.zig");
const logs = @import("scanner/logs.zig");
const xcode = @import("scanner/xcode.zig");
const homebrew = @import("scanner/homebrew.zig");
const browsers = @import("scanner/browsers.zig");
const apps = @import("scanner/apps.zig");
const cleaner = @import("cleaner/cleaner.zig");
const maintenance = @import("maintenance.zig");
const utils = @import("utils.zig");

pub const View = enum {
    home,
    scanning,
    results,
    confirm,
    cleaning,
    maintenance,
};

pub const App = struct {
    allocator: std.mem.Allocator,

    // Terminal state
    term_state: ?terminal.TermState = null,
    width: u16 = 80,
    height: u16 = 24,

    // View state
    current_view: View = .home,
    previous_view: View = .home,

    // Views
    home: home_view.HomeView,
    scan: scanning_view.ScanView,
    results: results_view.ResultsView,
    confirm: confirm_view.ConfirmView,
    cleaning: cleaning_view.CleaningView,
    maint: maintenance_view.MaintenanceView,

    // Theme
    theme: theme_mod.Theme,
    style: theme_mod.Style,

    // Data
    scan_result: ?scanner.ScanResult = null,
    clean_stats: ?cleaner.CleanStats = null,

    // State
    running: bool = true,
    needs_render: bool = true,
    last_tick: i64 = 0,

    pub fn init(allocator: std.mem.Allocator) App {
        const t = theme_mod.default_theme;
        return App{
            .allocator = allocator,
            .theme = t,
            .style = theme_mod.Style.init(t),
            .home = home_view.HomeView.init(),
            .scan = scanning_view.ScanView.init(),
            .results = results_view.ResultsView.init(15, 80),
            .confirm = confirm_view.ConfirmView.init(0, 0),
            .cleaning = cleaning_view.CleaningView.init(),
            .maint = maintenance_view.MaintenanceView.init(),
        };
    }

    pub fn deinit(self: *App) void {
        if (self.scan_result) |*result| {
            result.deinit();
        }
        if (self.clean_stats) |*stats| {
            stats.deinit();
        }
    }

    pub fn run(self: *App) !void {
        // Check platform
        if (!utils.isMacOS()) {
            var err_buf: [256]u8 = undefined;
            var stderr_writer = std.fs.File.stderr().writer(&err_buf);
            const stderr = &stderr_writer.interface;
            try stderr.writeAll("ash only runs on macOS\n");
            try stderr.flush();
            return;
        }

        // Enter raw mode
        self.term_state = try terminal.enableRawMode();
        errdefer if (self.term_state) |ts| terminal.disableRawMode(ts);

        var stdout_buf: [8192]u8 = undefined;
        var stdout_writer = std.fs.File.stdout().writer(&stdout_buf);
        const stdout = &stdout_writer.interface;

        // Enter alternate screen
        try terminal.enterAltScreen(stdout);
        errdefer terminal.leaveAltScreen(stdout) catch {};

        // Update terminal size
        const size = try terminal.getSize();
        self.width = size.width;
        self.height = size.height;
        self.results.setPageSize(@as(usize, self.height) -| 12);
        self.results.setWidth(self.width);

        // Main loop
        while (self.running) {
            // Render if needed
            if (self.needs_render) {
                try self.render(stdout);
                try stdout.flush();
                self.needs_render = false;
            }

            // Poll for input
            if (terminal.pollInput(100)) {
                if (terminal.readKey()) |event| {
                    try self.handleEvent(event);
                }
            }

            // Tick animations
            const now = std.time.milliTimestamp();
            if (now - self.last_tick >= 100) {
                self.tick();
                self.last_tick = now;

                // Render for animation
                if (self.current_view == .scanning or self.current_view == .cleaning) {
                    self.needs_render = true;
                }
            }
        }

        // Cleanup
        try terminal.leaveAltScreen(stdout);
        if (self.term_state) |ts| terminal.disableRawMode(ts);
    }

    fn tick(self: *App) void {
        self.scan.tick();
        self.cleaning.tick();
    }

    fn handleEvent(self: *App, event: terminal.Event) !void {
        switch (event) {
            .key => |key| try self.handleKey(key),
            .resize => |size| {
                self.width = size.width;
                self.height = size.height;
                self.results.setPageSize(@as(usize, self.height) -| 12);
                self.results.setWidth(self.width);
                self.needs_render = true;
            },
            .none => {},
        }
    }

    fn handleKey(self: *App, key: terminal.Key) !void {
        // Global keys
        switch (key) {
            .char => |c| {
                if (c == 'q' or c == 'Q') {
                    self.running = false;
                    return;
                }
            },
            .escape => {
                try self.handleBack();
                return;
            },
            .ctrl => |c| {
                if (c == 'c') {
                    self.running = false;
                    return;
                }
            },
            else => {},
        }

        // View-specific handling
        switch (self.current_view) {
            .home => try self.handleHomeKey(key),
            .scanning => {},
            .results => try self.handleResultsKey(key),
            .confirm => try self.handleConfirmKey(key),
            .cleaning => {},
            .maintenance => try self.handleMaintenanceKey(key),
        }

        self.needs_render = true;
    }

    fn handleHomeKey(self: *App, key: terminal.Key) !void {
        switch (key) {
            .char => |c| switch (c) {
                'j', 'J' => self.home.moveDown(),
                'k', 'K' => self.home.moveUp(),
                else => {},
            },
            .down => self.home.moveDown(),
            .up => self.home.moveUp(),
            .enter => {
                switch (self.home.selectedAction()) {
                    .scan => try self.startScan(),
                    .maintenance => self.goToView(.maintenance),
                    .quit => self.running = false,
                }
            },
            else => {},
        }
    }

    fn handleResultsKey(self: *App, key: terminal.Key) !void {
        switch (key) {
            .char => |c| switch (c) {
                'j', 'J' => self.results.moveDown(),
                'k', 'K' => self.results.moveUp(),
                ' ' => self.results.toggle(),
                'a', 'A' => self.results.toggleAll(),
                else => {},
            },
            .down => self.results.moveDown(),
            .up => self.results.moveUp(),
            .page_down => self.results.pageDown(),
            .page_up => self.results.pageUp(),
            .enter => {
                if (self.results.hasSelection()) {
                    self.confirm = confirm_view.ConfirmView.init(
                        self.results.selectedCount(),
                        self.results.selectedSize(),
                    );
                    self.goToView(.confirm);
                }
            },
            else => {},
        }
    }

    fn handleConfirmKey(self: *App, key: terminal.Key) !void {
        switch (key) {
            .char => |c| switch (c) {
                'h', 'H' => self.confirm.moveLeft(),
                'l', 'L' => self.confirm.moveRight(),
                'y', 'Y' => {
                    self.confirm.cursor = 1;
                    try self.startClean();
                },
                'n', 'N' => self.goBack(),
                else => {},
            },
            .left => self.confirm.moveLeft(),
            .right => self.confirm.moveRight(),
            .tab => self.confirm.toggle(),
            .enter => {
                if (self.confirm.isConfirmed()) {
                    try self.startClean();
                } else {
                    self.goBack();
                }
            },
            else => {},
        }
    }

    fn handleMaintenanceKey(self: *App, key: terminal.Key) !void {
        switch (key) {
            .char => |c| switch (c) {
                'j', 'J' => self.maint.moveDown(),
                'k', 'K' => self.maint.moveUp(),
                else => {},
            },
            .down => self.maint.moveDown(),
            .up => self.maint.moveUp(),
            .enter => try self.runMaintenanceCommand(),
            else => {},
        }
    }

    fn handleBack(self: *App) !void {
        switch (self.current_view) {
            .home => self.running = false,
            .scanning => self.goToView(.home),
            .results => self.goToView(.home),
            .confirm => self.goToView(.results),
            .cleaning => {}, // Can't go back while cleaning
            .maintenance => self.goToView(.home),
        }
        self.needs_render = true;
    }

    fn goToView(self: *App, view: View) void {
        self.previous_view = self.current_view;
        self.current_view = view;
        self.needs_render = true;
    }

    fn goBack(self: *App) void {
        self.current_view = self.previous_view;
        self.needs_render = true;
    }

    fn startScan(self: *App) !void {
        self.goToView(.scanning);
        self.scan.setMessage("Scanning directories...");
        self.scan.setProgress(0.0);
        self.needs_render = true;

        // Perform scan
        var result = scanner.ScanResult.init(self.allocator);
        errdefer result.deinit();

        // Scan each category
        const categories = [_]struct {
            name: []const u8,
            scanFn: *const fn (std.mem.Allocator) anyerror!std.ArrayList(scanner.Entry),
        }{
            .{ .name = "caches", .scanFn = caches.scan },
            .{ .name = "logs", .scanFn = logs.scan },
            .{ .name = "xcode", .scanFn = xcode.scan },
            .{ .name = "homebrew", .scanFn = homebrew.scan },
            .{ .name = "browsers", .scanFn = browsers.scan },
        };

        for (categories, 0..) |cat, i| {
            self.scan.setMessage(cat.name);
            self.scan.setProgress(@as(f32, @floatFromInt(i)) / @as(f32, @floatFromInt(categories.len)));

            // Render progress
            var scan_stdout_buf: [8192]u8 = undefined;
            var scan_stdout_writer = std.fs.File.stdout().writer(&scan_stdout_buf);
            const scan_stdout = &scan_stdout_writer.interface;
            try self.render(scan_stdout);
            try scan_stdout.flush();

            var entries = cat.scanFn(self.allocator) catch continue;
            defer entries.deinit(self.allocator);

            for (entries.items) |*entry| {
                try result.addEntry(entry);
            }
        }

        // Sort by size
        result.sortBySize();

        // Store result
        if (self.scan_result) |*old| {
            old.deinit();
        }
        self.scan_result = result;

        // Update results view
        if (self.scan_result) |*sr| {
            self.results.setEntries(sr.entries.items);
        }

        self.goToView(.results);
    }

    fn startClean(self: *App) !void {
        self.goToView(.cleaning);
        self.needs_render = true;

        var clean = cleaner.Cleaner.init(self.allocator, .{
            .dry_run = false,
            .use_trash = true,
        });

        const entries = self.results.entries();
        const stats = try clean.clean(entries);

        if (self.clean_stats) |*old| {
            old.deinit();
        }
        self.clean_stats = stats;
        self.cleaning.setStats(stats);

        // Show completion
        var clean_stdout_buf: [8192]u8 = undefined;
        var clean_stdout_writer = std.fs.File.stdout().writer(&clean_stdout_buf);
        const clean_stdout = &clean_stdout_writer.interface;
        try terminal.clearScreen(clean_stdout);
        try self.cleaning.renderComplete(clean_stdout, self.width, self.height);
        try clean_stdout.flush();

        // Wait for key
        while (true) {
            if (terminal.pollInput(100)) {
                if (terminal.readKey()) |_| break;
            }
        }

        self.goToView(.home);
    }

    fn runMaintenanceCommand(self: *App) !void {
        const idx = self.maint.cursor;
        const cmd = self.maint.selectedCommand();

        self.maint.setRunning(idx);
        self.needs_render = true;

        // Render running state
        var maint_stdout_buf: [8192]u8 = undefined;
        var maint_stdout_writer = std.fs.File.stdout().writer(&maint_stdout_buf);
        const maint_stdout = &maint_stdout_writer.interface;
        try self.render(maint_stdout);
        try maint_stdout.flush();

        // Run command
        const result = maintenance.run(self.allocator, cmd) catch |err| {
            var error_result = maintenance.CommandResult{
                .command = cmd,
                .success = false,
            };
            const msg = @errorName(err);
            @memcpy(error_result.error_msg_buf[0..msg.len], msg);
            error_result.error_msg_len = msg.len;

            self.maint.setResult(idx, error_result);
            self.maint.setRunning(null);
            return;
        };

        self.maint.setResult(idx, result);
        self.maint.setRunning(null);
        self.needs_render = true;
    }

    fn render(self: *App, writer: anytype) !void {
        try terminal.clearScreen(writer);

        switch (self.current_view) {
            .home => try self.home.render(writer, self.width, self.height),
            .scanning => try self.scan.render(writer, self.width, self.height),
            .results => try self.results.render(writer, self.width, self.height),
            .confirm => try self.confirm.render(writer, self.width, self.height),
            .cleaning => try self.cleaning.render(writer, self.width, self.height),
            .maintenance => try self.maint.render(writer, self.width, self.height),
        }
    }
};

// Tests
test "App init" {
    const allocator = std.testing.allocator;
    var app = App.init(allocator);
    defer app.deinit();

    try std.testing.expectEqual(View.home, app.current_view);
}
