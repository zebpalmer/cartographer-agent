package collectors

import (
	"bufio"
	"cartographer-go-agent/configuration"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SSHLoginEvent represents an SSH login event
type SSHLoginEvent struct {
	Username string `json:"username"`
	Time     string `json:"time"`
	SourceIP string `json:"source_ip"`
}

// parseSSHLogins parses the SSH logins from the given log file
func parseSSHLogins(logFile string) ([]SSHLoginEvent, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			slog.Error("Error closing file", slog.Any("error", err))
		}
	}(file)

	var events []SSHLoginEvent

	// Regular expression to match the SSH login pattern
	regex := regexp.MustCompile(`Accepted publickey for (\w+) from ([\d\.]+) port \d+ ssh2`)

	scanner := bufio.NewScanner(file)
	now := time.Now()

	// Get the local timezone of the system
	location, err := time.LoadLocation("Local")
	if err != nil {
		slog.Error("Error loading system timezone", slog.Any("error", err))
		location = time.UTC // Fallback to UTC if unable to load local timezone
	}

	// Map to track the most recent login per (username, IP) pair
	mostRecentLogins := make(map[string]SSHLoginEvent)

	for scanner.Scan() {
		line := scanner.Text()
		matches := regex.FindStringSubmatch(line)
		if matches != nil {
			// Extract the timestamp from the log entry
			timestampStr := strings.Fields(line)[0:3]
			timestamp, err := time.ParseInLocation("Jan _2 15:04:05", strings.Join(timestampStr, " "), location)
			if err != nil {
				slog.Error("Error parsing timestamp", slog.Any("error", err))
				continue
			}

			// Add the current year to the timestamp
			timestamp = timestamp.AddDate(now.Year(), 0, 0)

			// Handle New Year's edge case (if timestamp is in the future)
			if timestamp.After(now) {
				timestamp = timestamp.AddDate(-1, 0, 0)
			}

			// Only include events from the last 24 hours
			if now.Sub(timestamp) > 24*time.Hour {
				continue
			}

			// Convert timestamp to UTC
			timestampUTC := timestamp.UTC()

			// Create a unique key based on username and source IP
			userIPKey := matches[1] + "_" + matches[2]

			// Store the login event if it's the most recent for this user-IP pair
			if existingEvent, exists := mostRecentLogins[userIPKey]; !exists || timestampUTC.After(parseEventTime(existingEvent)) {
				mostRecentLogins[userIPKey] = SSHLoginEvent{
					Username: matches[1],
					Time:     timestampUTC.Format(time.RFC3339),
					SourceIP: matches[2],
				}
			}
		}
	}

	// Convert the map to a slice of SSHLoginEvent
	for _, event := range mostRecentLogins {
		events = append(events, event)
	}

	return events, scanner.Err()
}

// parseEventTime parses the time from an SSHLoginEvent for comparison purposes
func parseEventTime(event SSHLoginEvent) time.Time {
	t, err := time.Parse(time.RFC3339, event.Time)
	if err != nil {
		slog.Error("Error parsing event time", slog.Any("error", err))
		return time.Time{}
	}
	return t
}

// SSHLoginEventsCollector returns a collector that gathers SSH login events, only on Linux systems
func SSHLoginEventsCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("ssh_login_events", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		if runtime.GOOS != "linux" {
			return nil, ErrCollectorSkipped
		}

		events, err := parseSSHLogins("/var/log/auth.log")
		if err != nil {
			return nil, err
		}

		return events, nil
	})
}
