package common

import (
	"strings"
	"testing"
)

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		options    *CommandOptions
		wantErr    bool
		wantExit   int
		wantOutput string
	}{
		{
			name:       "Simple echo command",
			command:    "echo Hello World",
			options:    &CommandOptions{Timeout: 5},
			wantErr:    false,
			wantExit:   0,
			wantOutput: "Hello World",
		},
		{
			name:     "Non-existent command",
			command:  "false_command",
			options:  &CommandOptions{Timeout: 5},
			wantErr:  true,
			wantExit: 127, // Common exit code for command not found
		},
		{
			name:     "Timeout command",
			command:  "sleep 10",
			options:  &CommandOptions{Timeout: 1},
			wantErr:  true,
			wantExit: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, _, exitCode, err := RunCommand(tt.command, tt.options)

			// Trim spaces to ignore trailing newlines and leading/trailing spaces
			output = strings.TrimSpace(output)
			expectedOutput := strings.TrimSpace(tt.wantOutput)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if exitCode != tt.wantExit {
				t.Errorf("RunCommand() exitCode = %v, want %v", exitCode, tt.wantExit)
			}

			if output != expectedOutput {
				t.Errorf("RunCommand() output = %q, want %q", output, expectedOutput)
			}
		})
	}
}
