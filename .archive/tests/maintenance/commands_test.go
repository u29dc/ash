package maintenance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"ash/internal/maintenance"
)

func TestGetCommands(t *testing.T) {
	commands := maintenance.GetCommands()

	assert.NotEmpty(t, commands)

	// Should have these commands
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Name
	}

	assert.Contains(t, names, "Flush DNS Cache")
	assert.Contains(t, names, "Restart mDNSResponder")
	assert.Contains(t, names, "Rebuild Spotlight Index")
	assert.Contains(t, names, "Rebuild Launch Services")
	assert.Contains(t, names, "Clear Font Cache")
	assert.Contains(t, names, "Purge RAM")
}

func TestCommand_Properties(t *testing.T) {
	commands := maintenance.GetCommands()

	for _, cmd := range commands {
		t.Run(cmd.Name, func(t *testing.T) {
			assert.NotEmpty(t, cmd.Name, "Command should have a name")
			assert.NotEmpty(t, cmd.Description, "Command should have a description")
			assert.NotEmpty(t, cmd.Cmd, "Command should have a cmd")
			// Args can be empty
		})
	}
}

func TestCommand_SudoRequirements(t *testing.T) {
	commands := maintenance.GetCommands()

	// Find specific commands and verify sudo requirements
	for _, cmd := range commands {
		switch cmd.Name {
		case "Flush DNS Cache":
			assert.False(t, cmd.RequiresSudo, "DNS flush shouldn't require sudo")
		case "Restart mDNSResponder":
			assert.True(t, cmd.RequiresSudo, "mDNSResponder restart requires sudo")
		case "Rebuild Spotlight Index":
			assert.True(t, cmd.RequiresSudo, "Spotlight rebuild requires sudo")
		case "Rebuild Launch Services":
			assert.False(t, cmd.RequiresSudo, "Launch Services rebuild doesn't require sudo")
		case "Clear Font Cache":
			assert.True(t, cmd.RequiresSudo, "Font cache clear requires sudo")
		case "Purge RAM":
			assert.True(t, cmd.RequiresSudo, "RAM purge requires sudo")
		}
	}
}

func TestCommand_UsefulFlag(t *testing.T) {
	commands := maintenance.GetCommands()

	// Purge RAM should be marked as not useful
	for _, cmd := range commands {
		if cmd.Name == "Purge RAM" {
			assert.False(t, cmd.Useful, "Purge RAM should be marked as not useful")
		} else {
			assert.True(t, cmd.Useful, "%s should be marked as useful", cmd.Name)
		}
	}
}

func TestCommandResult_Success(t *testing.T) {
	result := &maintenance.CommandResult{
		Command: &maintenance.Command{Name: "Test"},
		Success: true,
		Output:  "OK",
		Error:   nil,
	}

	assert.True(t, result.Success)
	assert.Nil(t, result.Error)
	assert.Equal(t, "OK", result.Output)
}

func TestCommandResult_Failure(t *testing.T) {
	result := &maintenance.CommandResult{
		Command: &maintenance.Command{Name: "Test"},
		Success: false,
		Output:  "",
		Error:   assert.AnError,
	}

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}
