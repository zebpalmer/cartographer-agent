package collectors

import (
	"bufio"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"fmt"
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

// runAptCommand runs apt commands; a variable so tests can substitute it.
var runAptCommand = common.RunCommand

// AptUpdatesCollector returns a collector for available APT updates on Ubuntu systems
func AptUpdatesCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("apt", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		if runtime.GOOS != "linux" {
			return nil, ErrCollectorSkipped
		}

		// Get the list of upgradable packages
		results, _, exitCode, err := runAptCommand("apt list --upgradable", &common.CommandOptions{
			Timeout: 10,
		})
		if err != nil {
			return nil, err
		}
		if exitCode != 0 {
			return nil, fmt.Errorf("apt list --upgradable exited with code %d", exitCode)
		}

		// Parse the apt output
		updates, err := parseAptUpdates(results)
		if err != nil {
			return nil, err
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

	// <package>/<archive>[,<archive>...] <candidate> <arch> [upgradable from: <installed>]
	// The archive list names where the candidate version comes from, e.g.
	// "noble-updates,noble-security" or "trixie-security".
	re := regexp.MustCompile(`(?m)^(\S+)/(\S+)\s+(\S+)\s+\S+\s+\[upgradable from: (\S+)\]`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) == 5 {
			updates = append(updates, AptUpdateInfo{
				PackageName:      matches[1],
				CurrentVersion:   matches[4],
				CandidateVersion: matches[3],
				IsSecurityUpdate: fromSecurityArchive(matches[2]),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return updates, nil
}

// fromSecurityArchive reports whether the candidate version is published in a security
// archive: Debian "trixie-security", Ubuntu "noble-security", Ubuntu Pro
// "noble-infra-security" / "noble-apps-security". Earlier versions grepped the whole
// `apt-cache policy` output for "security", which also matched packages whose candidate
// comes from the regular archive whenever any version in the table had a security origin,
// overcounting by more than half on a Debian host (118 flagged vs 72 actual).
func fromSecurityArchive(archives string) bool {
	for _, archive := range strings.Split(archives, ",") {
		if strings.HasSuffix(archive, "-security") {
			return true
		}
	}
	return false
}
