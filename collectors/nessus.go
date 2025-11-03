package collectors

import (
	"cartographer-go-agent/configuration"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var nessusPaths = []string{
	"/opt/nessus_agent/sbin/nessuscli",
	"/opt/nessus/sbin/nessuscli",
}

// NessusStatus represents the status of the Nessus agent
type NessusStatus struct {
	Running               bool   `json:"running"`
	LinkedTo              string `json:"linked_to"`
	LinkStatus            string `json:"link_status"`
	LastSuccessConnection int    `json:"last_success_connection_secs"`
	Proxy                 string `json:"proxy"`
	PluginSet             string `json:"plugin_set"`
	Scanning              bool   `json:"scanning"`
	JobsPending           int    `json:"jobs_pending"`
	SmartScanConfigs      int    `json:"smart_scan_configs"`
	ScansCompleted        int    `json:"scans_completed"`
	ScansLimit            int    `json:"scans_limit"`
	LastScanned           int64  `json:"last_scanned"`
	LastConnect           int64  `json:"last_connect"`
	LastConnectionAttempt int64  `json:"last_connection_attempt"`
	AgentPath             string `json:"agent_path,omitempty"`
}

// fileSystem interface for mocking filesystem operations
type fileSystem interface {
	Stat(name string) (os.FileInfo, error)
	LookPath(file string) (string, error)
}

// realFileSystem implements actual filesystem operations
type realFileSystem struct{}

func (fs *realFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *realFileSystem) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// isExecutable but using the filesystem interface
func isExecutableWithFS(path string, fs fileSystem) bool {
	info, err := fs.Stat(path)
	if err != nil {
		return false
	}
	mode := info.Mode()
	return !mode.IsDir() && (mode.Perm()&0111) != 0
}

// findNessuscliWithFS is the testable version that accepts a filesystem interface
func findNessuscliWithFS(fs fileSystem) (string, error) {
	// First, check if it's in PATH
	if path, err := fs.LookPath("nessuscli"); err == nil {
		if isExecutableWithFS(path, fs) {
			return path, nil
		}
	}

	// Check common paths
	for _, path := range nessusPaths {
		if isExecutableWithFS(path, fs) {
			return path, nil
		}
	}

	return "", fmt.Errorf("nessuscli not found in PATH or common locations")
}

// The original findNessuscli function now just calls findNessuscliWithFS with the real implementation
func findNessuscli() (string, error) {
	return findNessuscliWithFS(&realFileSystem{})
}

// The original isExecutable can now just call isExecutableWithFS
func isExecutable(path string) bool {
	return isExecutableWithFS(path, &realFileSystem{})
}

func parseNessusOutput(output []byte, agentPath string) (*NessusStatus, error) {
	status := &NessusStatus{
		AgentPath: agentPath,
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Running":
			status.Running = value == "Yes"
		case "Linked to":
			status.LinkedTo = value
		case "Link status":
			status.LinkStatus = value
		case "Last successful connection with controller":
			if secs := strings.Split(value, " ")[0]; secs != "" {
				if i, err := strconv.Atoi(secs); err == nil {
					status.LastSuccessConnection = i
				}
			}
		case "Proxy":
			status.Proxy = value
		case "Plugin set":
			status.PluginSet = value
		case "Scanning":
			re := regexp.MustCompile(`No \((\d+) jobs pending, (\d+) smart scan configs\)`)
			if matches := re.FindStringSubmatch(value); matches != nil {
				status.Scanning = false
				status.JobsPending, _ = strconv.Atoi(matches[1])
				status.SmartScanConfigs, _ = strconv.Atoi(matches[2])
			} else {
				status.Scanning = value == "Yes"
			}
		case "Scans run today":
			re := regexp.MustCompile(`(\d+) of (\d+) limit`)
			if matches := re.FindStringSubmatch(value); matches != nil {
				status.ScansCompleted, _ = strconv.Atoi(matches[1])
				status.ScansLimit, _ = strconv.Atoi(matches[2])
			}
		case "Last scanned":
			if i, err := strconv.ParseInt(value, 10, 64); err == nil {
				status.LastScanned = i
			}
		case "Last connect":
			if i, err := strconv.ParseInt(value, 10, 64); err == nil {
				status.LastConnect = i
			}
		case "Last connection attempt":
			if i, err := strconv.ParseInt(value, 10, 64); err == nil {
				status.LastConnectionAttempt = i
			}
		}
	}

	return status, nil
}

// NessusCollector returns a collector that runs the nessuscli agent status command
func NessusCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("nessus_agent", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		// Find nessuscli executable
		nessusPath, err := findNessuscli()
		if err != nil {
			return nil, err
		}

		// Run nessuscli command
		cmd := exec.Command(nessusPath, "agent", "status", "--local")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to execute nessuscli: %w", err)
		}

		// Parse the output
		status, err := parseNessusOutput(output, nessusPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse nessus status: %w", err)
		}

		return status, nil
	})
}
