package collectors

import (
	"bufio"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"log/slog"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// AptUpdateInfo struct to represent the data for each package update
type AptUpdateInfo struct {
	PackageName      string `json:"package_name"`
	CurrentVersion   string `json:"current_version"`
	CandidateVersion string `json:"candidate_version"`
	IsSecurityUpdate bool   `json:"is_security_update"`
}

// AptUpdatesCollector returns a collector for available APT updates on Ubuntu systems
func AptUpdatesCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("apt", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		if runtime.GOOS != "linux" {
			return nil, ErrCollectorSkipped
		}

		// Get the list of upgradable packages
		results, _, exitCode, err := common.RunCommand("apt list --upgradable", &common.CommandOptions{
			Timeout: 10,
		})
		if err != nil || exitCode != 0 {
			return nil, err
		}

		// Parse the apt output
		updates, err := parseAptUpdates(results)
		if err != nil {
			return nil, err
		}

		// For each package, determine if it is a security update
		for i, update := range updates {
			updates[i].IsSecurityUpdate = isSecurityUpdate(update.PackageName)
		}

		// Create the final data structure to return
		return map[string]interface{}{
			"available_updates": updates,
			"collected_at":      time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func parseAptUpdates(output string) ([]AptUpdateInfo, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	var updates []AptUpdateInfo

	// Updated regex to correctly parse the apt output
	re := regexp.MustCompile(`(?m)^(\S+)/\S+\s+(\S+)\s+\S+\s+\[upgradable from: (\S+)\]`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			packageName := matches[1]
			candidateVersion := matches[2]
			currentVersion := matches[3]

			update := AptUpdateInfo{
				PackageName:      packageName,
				CurrentVersion:   currentVersion,
				CandidateVersion: candidateVersion,
				IsSecurityUpdate: false, // Default value, will be updated later
			}
			updates = append(updates, update)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return updates, nil
}

// isSecurityUpdate determines if a package is a security update
func isSecurityUpdate(packageName string) bool {
	// Run command to check if the package is in the security updates list
	output, _, exitCode, err := common.RunCommand("apt-cache policy "+packageName, &common.CommandOptions{
		Timeout: 30,
	})
	if err != nil || exitCode != 0 {
		slog.Error("Error checking security update status for package",
			slog.String("package_name", packageName),
			slog.String("error", err.Error()),
		)
		return false
	}
	return strings.Contains(output, "security")
}
