package monitors

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// checkPort performs a port connectivity check
func checkPort(monitor Monitor) (MonitorStatus, string) {
	if monitor.Protocol == "tcp" {
		return checkTCPPort(monitor)
	} else if monitor.Protocol == "udp" {
		return checkUDPPort(monitor)
	}

	return StatusUnknown, fmt.Sprintf("Unknown protocol: %s", monitor.Protocol)
}

// checkTCPPort checks if a TCP port is open and accepting connections
func checkTCPPort(monitor Monitor) (MonitorStatus, string) {
	// Use net.JoinHostPort to properly handle IPv6 addresses
	address := net.JoinHostPort(monitor.Host, fmt.Sprintf("%d", monitor.Port))
	timeout := time.Duration(monitor.Timeout) * time.Second

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return StatusCritical, fmt.Sprintf("TCP connection failed: %v", err)
	}
	defer conn.Close()

	return StatusOK, fmt.Sprintf("TCP port %d open on %s", monitor.Port, monitor.Host)
}

// checkUDPPort checks if a UDP port is bound on localhost using netstat
func checkUDPPort(monitor Monitor) (MonitorStatus, string) {
	// Try ss first (newer, faster), fall back to netstat
	if isCommandAvailable("ss") {
		return checkUDPPortWithSS(monitor)
	}
	return checkUDPPortWithNetstat(monitor)
}

// checkUDPPortWithSS uses ss to check UDP port binding
func checkUDPPortWithSS(monitor Monitor) (MonitorStatus, string) {
	// ss -ulnH | grep :PORT
	cmd := exec.Command("ss", "-ulnH")
	output, err := cmd.Output()
	if err != nil {
		return StatusUnknown, fmt.Sprintf("Failed to execute ss: %v", err)
	}

	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if matchesPort(line, monitor.Port) {
			return StatusOK, fmt.Sprintf("UDP port %d is bound on localhost", monitor.Port)
		}
	}

	return StatusCritical, fmt.Sprintf("UDP port %d is not bound on localhost", monitor.Port)
}

// checkUDPPortWithNetstat uses netstat to check UDP port binding
func checkUDPPortWithNetstat(monitor Monitor) (MonitorStatus, string) {
	// netstat -uln | grep :PORT
	cmd := exec.Command("netstat", "-uln")
	output, err := cmd.Output()
	if err != nil {
		return StatusUnknown, fmt.Sprintf("Failed to execute netstat: %v", err)
	}

	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if matchesPort(line, monitor.Port) {
			return StatusOK, fmt.Sprintf("UDP port %d is bound on localhost", monitor.Port)
		}
	}

	return StatusCritical, fmt.Sprintf("UDP port %d is not bound on localhost", monitor.Port)
}

// matchesPort checks if a line contains the specified port in a valid format.
// It looks for :PORT followed by a non-digit character (space, tab, etc.) or end of string.
// This handles various output formats including IPv4, IPv6, and different delimiters.
func matchesPort(line string, port int) bool {
	portPattern := fmt.Sprintf(`:%d(\D|$)`, port)
	matched, err := regexp.MatchString(portPattern, line)
	if err != nil {
		return false
	}
	return matched
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
