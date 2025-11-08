package monitors

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const maxOutputLength = 200

// truncateOutput truncates a string to maxOutputLength and adds ellipsis if needed
func truncateOutput(s string) string {
	if len(s) > maxOutputLength {
		return s[:maxOutputLength] + "..."
	}
	return s
}

// checkCommand executes a custom command and validates the output
//
// SECURITY WARNING: This function executes arbitrary shell commands using 'sh -c'
// without sanitization. Monitor configurations should only be loaded from trusted
// sources (filesystem owned by root/admin). DO NOT load monitor configs from:
// - User-provided input
// - Untrusted network sources
// - World-writable directories
// - Any source that could be controlled by untrusted users
//
// The command string is passed directly to the shell, allowing:
// - Command injection via shell metacharacters
// - Arbitrary file system access with agent permissions
// - Network operations
// - Process execution
func checkCommand(monitor Monitor) (MonitorStatus, string) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(monitor.Timeout)*time.Second)
	defer cancel()

	// Create command with shell execution to support piping and shell features
	cmd := exec.CommandContext(ctx, "sh", "-c", monitor.Command)

	// Set working directory if specified
	if monitor.WorkingDir != "" {
		cmd.Dir = monitor.WorkingDir
	}

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	err := cmd.Run()
	exitCode := 0

	// Get stdout/stderr early for use in error messages
	stdoutStr := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())

	// Get exit code and handle execution errors
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			msg := fmt.Sprintf("Command timed out after %d seconds", monitor.Timeout)
			if stdoutStr != "" || stderrStr != "" {
				msg += fmt.Sprintf(". Output: %s", truncateOutput(stdoutStr))
				if stderrStr != "" {
					msg += fmt.Sprintf(", Stderr: %s", truncateOutput(stderrStr))
				}
			}
			return StatusCritical, msg
		} else {
			msg := fmt.Sprintf("Failed to execute command: %v", err)
			if stdoutStr != "" || stderrStr != "" {
				msg += fmt.Sprintf(". Output: %s", truncateOutput(stdoutStr))
				if stderrStr != "" {
					msg += fmt.Sprintf(", Stderr: %s", truncateOutput(stderrStr))
				}
			}
			return StatusUnknown, msg
		}
	}

	// Validate exit code
	expectedExitCode := 0
	if monitor.Validations != nil && monitor.Validations.ExitCode != nil {
		expectedExitCode = *monitor.Validations.ExitCode
	}

	if exitCode != expectedExitCode {
		msg := fmt.Sprintf("Exit code %d (expected %d). Output: %s", exitCode, expectedExitCode, truncateOutput(stdoutStr))
		if stderrStr != "" {
			msg += fmt.Sprintf(", Stderr: %s", truncateOutput(stderrStr))
		}
		return StatusCritical, msg
	}

	// Validate output contains expected string
	if monitor.Validations != nil && monitor.Validations.OutputContains != "" {
		if !strings.Contains(stdoutStr, monitor.Validations.OutputContains) {
			return StatusCritical, fmt.Sprintf("Output does not contain expected string: '%s'. Got: %s", monitor.Validations.OutputContains, truncateOutput(stdoutStr))
		}
	}

	// Validate output does NOT contain specified string
	if monitor.Validations != nil && monitor.Validations.OutputNotContains != "" {
		if strings.Contains(stdoutStr, monitor.Validations.OutputNotContains) {
			return StatusCritical, fmt.Sprintf("Output contains unexpected string: '%s'. Got: %s", monitor.Validations.OutputNotContains, truncateOutput(stdoutStr))
		}
	}

	// Validate output matches regex
	if monitor.Validations != nil && monitor.Validations.OutputRegex != "" {
		matched, err := regexp.MatchString(monitor.Validations.OutputRegex, stdoutStr)
		if err != nil {
			return StatusUnknown, fmt.Sprintf("Invalid regex pattern: %v", err)
		}
		if !matched {
			return StatusCritical, fmt.Sprintf("Output does not match regex: '%s'. Got: %s", monitor.Validations.OutputRegex, truncateOutput(stdoutStr))
		}
	}

	// Validate stderr contains expected string
	if monitor.Validations != nil && monitor.Validations.ErrorContains != "" {
		if !strings.Contains(stderrStr, monitor.Validations.ErrorContains) {
			return StatusCritical, fmt.Sprintf("Stderr does not contain expected string: '%s'. Got: %s", monitor.Validations.ErrorContains, truncateOutput(stderrStr))
		}
	}

	// Build success message
	message := fmt.Sprintf("Command executed successfully (exit code %d)", exitCode)
	if stdoutStr != "" {
		message = fmt.Sprintf("%s. Output: %s", message, truncateOutput(stdoutStr))
	}

	return StatusOK, message
}
