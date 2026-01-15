package maintenance

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Command represents a maintenance command.
type Command struct {
	Name        string
	Description string
	Cmd         string
	Args        []string
	RequiresSudo bool
	Useful      bool
}

// CommandResult represents the result of running a maintenance command.
type CommandResult struct {
	Command  *Command
	Success  bool
	Output   string
	Error    error
	Duration time.Duration
}

// GetCommands returns all available maintenance commands.
func GetCommands() []*Command {
	return []*Command{
		{
			Name:         "Flush DNS Cache",
			Description:  "Clear the DNS resolver cache",
			Cmd:          "dscacheutil",
			Args:         []string{"-flushcache"},
			RequiresSudo: false,
			Useful:       true,
		},
		{
			Name:         "Restart mDNSResponder",
			Description:  "Restart the DNS service",
			Cmd:          "killall",
			Args:         []string{"-HUP", "mDNSResponder"},
			RequiresSudo: true,
			Useful:       true,
		},
		{
			Name:         "Rebuild Spotlight Index",
			Description:  "Reindex Spotlight search database",
			Cmd:          "mdutil",
			Args:         []string{"-E", "/"},
			RequiresSudo: true,
			Useful:       true,
		},
		{
			Name:         "Rebuild Launch Services",
			Description:  "Rebuild the Launch Services database",
			Cmd:          "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
			Args:         []string{"-kill", "-r", "-domain", "local", "-domain", "user"},
			RequiresSudo: false,
			Useful:       true,
		},
		{
			Name:         "Clear Font Cache",
			Description:  "Remove cached font data",
			Cmd:          "atsutil",
			Args:         []string{"databases", "-remove"},
			RequiresSudo: true,
			Useful:       true,
		},
		{
			Name:         "Purge RAM",
			Description:  "Free up inactive memory",
			Cmd:          "purge",
			Args:         []string{},
			RequiresSudo: true,
			Useful:       false, // Limited benefit as noted in plan
		},
	}
}

// Run executes a maintenance command.
func Run(ctx context.Context, cmd *Command) *CommandResult {
	start := time.Now()
	result := &CommandResult{
		Command: cmd,
	}

	var execCmd *exec.Cmd
	if cmd.RequiresSudo {
		args := append([]string{cmd.Cmd}, cmd.Args...)
		execCmd = exec.CommandContext(ctx, "sudo", args...)
	} else {
		execCmd = exec.CommandContext(ctx, cmd.Cmd, cmd.Args...)
	}

	output, err := execCmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = strings.TrimSpace(string(output))

	if err != nil {
		result.Success = false
		result.Error = err
	} else {
		result.Success = true
	}

	return result
}

// RunAll executes all maintenance commands.
func RunAll(ctx context.Context, sudoOnly bool) []*CommandResult {
	commands := GetCommands()
	var results []*CommandResult

	for _, cmd := range commands {
		if sudoOnly && !cmd.RequiresSudo {
			continue
		}

		select {
		case <-ctx.Done():
			return results
		default:
		}

		result := Run(ctx, cmd)
		results = append(results, result)
	}

	return results
}

// FlushDNS flushes the DNS cache.
func FlushDNS(ctx context.Context) error {
	// First flush dscacheutil
	cmd1 := exec.CommandContext(ctx, "dscacheutil", "-flushcache")
	if err := cmd1.Run(); err != nil {
		return err
	}

	// Then restart mDNSResponder
	cmd2 := exec.CommandContext(ctx, "sudo", "killall", "-HUP", "mDNSResponder")
	return cmd2.Run()
}

// RebuildSpotlight rebuilds the Spotlight index.
func RebuildSpotlight(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "mdutil", "-E", "/")
	return cmd.Run()
}

// RebuildLaunchServices rebuilds the Launch Services database.
func RebuildLaunchServices(ctx context.Context) error {
	lsregister := "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	cmd := exec.CommandContext(ctx, lsregister, "-kill", "-r", "-domain", "local", "-domain", "user")
	return cmd.Run()
}

// ClearFontCache clears the font cache.
func ClearFontCache(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "atsutil", "databases", "-remove")
	return cmd.Run()
}

// PurgeRAM frees up inactive memory.
func PurgeRAM(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "sudo", "purge")
	return cmd.Run()
}

// CheckCommandAvailable checks if a command is available on the system.
func CheckCommandAvailable(cmdName string) bool {
	_, err := exec.LookPath(cmdName)
	return err == nil
}
