//go:build linux

package monitors

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// checkSystemd performs a systemd service check
func checkSystemd(monitor Monitor) (MonitorStatus, string) {
	serviceName := monitor.Target

	// Check if service exists
	if !serviceExists(serviceName) {
		return StatusCritical, fmt.Sprintf("Service '%s' does not exist", serviceName)
	}

	// Get service state
	state, err := getServiceProperty(serviceName, "ActiveState")
	if err != nil {
		return StatusUnknown, fmt.Sprintf("Failed to get service state: %v", err)
	}

	// Check state matches expected
	expectedState := monitor.Validations.State
	if state != expectedState {
		return StatusCritical, fmt.Sprintf("Service state is '%s' (expected '%s')", state, expectedState)
	}

	// Check if service should be enabled
	if monitor.Validations.Enabled != nil && *monitor.Validations.Enabled {
		enabledState, err := getServiceProperty(serviceName, "UnitFileState")
		if err != nil {
			return StatusUnknown, fmt.Sprintf("Failed to get enabled state: %v", err)
		}

		// enabled, enabled-runtime, static, and indirect are all considered "enabled"
		validEnabledStates := map[string]bool{
			"enabled":         true,
			"enabled-runtime": true,
			"static":          true,
			"indirect":        true,
		}

		if !validEnabledStates[enabledState] {
			return StatusWarning, fmt.Sprintf("Service is %s but not enabled (state: %s)", state, enabledState)
		}
	}

	// Check restart count if specified
	if monitor.Validations.RestartCount != nil {
		restartCount, err := getServiceRestartCount(serviceName)
		if err != nil {
			return StatusUnknown, fmt.Sprintf("Failed to get restart count: %v", err)
		}

		maxRestarts := *monitor.Validations.RestartCount
		if restartCount > maxRestarts {
			return StatusWarning, fmt.Sprintf("Service has restarted %d times (threshold: %d)", restartCount, maxRestarts)
		}
	}

	return StatusOK, fmt.Sprintf("Service '%s' is %s", serviceName, state)
}

// serviceExists checks if a systemd service exists
func serviceExists(serviceName string) bool {
	cmd := exec.Command("systemctl", "list-unit-files", serviceName+".service")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If the service doesn't exist, output will only contain the header
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return len(lines) > 2 // More than just header lines means service exists
}

// getServiceProperty retrieves a property of a systemd service
func getServiceProperty(serviceName, property string) (string, error) {
	cmd := exec.Command("systemctl", "show", serviceName+".service", "--property="+property, "--value")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("systemctl show failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// getServiceRestartCount gets the number of times a service has been restarted
func getServiceRestartCount(serviceName string) (int, error) {
	// Get NRestarts property from systemctl
	restartStr, err := getServiceProperty(serviceName, "NRestarts")
	if err != nil {
		return 0, err
	}

	// Parse the restart count
	restartCount, err := strconv.Atoi(restartStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse restart count '%s': %w", restartStr, err)
	}

	return restartCount, nil
}
