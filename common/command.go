package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunCommandLegacy runs a shell command with a timeout, capturing its output.
func RunCommandLegacy(command string, timeout int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CommandOptions contains options for running a shell command.
type CommandOptions struct {
	Timeout        int               // Timeout in seconds for the command, default if not specified.
	SuppressStderr bool              // Whether to suppress stderr output.
	Env            map[string]string // Custom environment variables for the command.
	WorkingDir     string            // Custom working directory for the command.
}

var (
	// ErrCommandTimeout and ErrCommandFailed are errors returned by RunCommand.
	ErrCommandTimeout = errors.New("command timed out")
	// ErrCommandFailed is an error returned by RunCommand when the command fails to run.
	ErrCommandFailed = errors.New("command failed to run")
)

// RunCommand runs a shell command with the given options, capturing its stdout, stderr, and exit code.
// It returns:
// - stdout: The command's standard output.
// - stderr: The command's standard error (unless suppressed).
// - exitCode: The exit code of the process.
// - error: An error if the command could not be executed or timed out.
//
// The command is run with a timeout specified by the CommandOptions.
// If no timeout is provided, it defaults to 30 seconds.
// Environment variables and working directory can also be customized via CommandOptions.
func RunCommand(command string, options *CommandOptions) (string, string, int, error) {
	// Set default options if not provided
	if options == nil {
		options = &CommandOptions{}
	}
	if options.Timeout == 0 {
		options.Timeout = 30 // Default to 30 seconds if no timeout is provided
	}

	// Prepare command context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(options.Timeout)*time.Second)
	defer cancel()

	// Prepare the command
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

	// Set working directory if provided
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// Set environment variables if provided
	if len(options.Env) > 0 {
		env := os.Environ()
		for k, v := range options.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Buffer to capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	if !options.SuppressStderr {
		cmd.Stderr = &stderr
	}

	// Run the command
	err := cmd.Run()

	// Capture exit code
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Check for specific error cases
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exitCode, fmt.Errorf("%w: %s", ErrCommandTimeout, command)
	}
	if err != nil {
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exitCode, fmt.Errorf("%w: %s, error: %v", ErrCommandFailed, command, err)
	}

	// Return the structured output
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exitCode, nil
}
