package collectors

import (
	"bufio"
	"cartographer-go-agent/configuration"
	"fmt"
	"io"
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

	return parseSSHLoginEvents(file)
}

// parseSyslogTimestamp parses the leading timestamp of a syslog line. It
// supports both the traditional BSD format ("Jan _2 15:04:05"), which has no
// year, and the RFC 3339 high-precision format used by modern rsyslog
// ("2026-07-10T07:30:22.434548+00:00").
func parseSyslogTimestamp(line string, now time.Time, location *time.Location) (time.Time, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return time.Time{}, fmt.Errorf("empty log line")
	}

	// RFC 3339 format: the timestamp is a single field
	if timestamp, err := time.Parse(time.RFC3339, fields[0]); err == nil {
		return timestamp, nil
	}

	// Traditional BSD format: "Jan _2 15:04:05" across three fields
	if len(fields) < 3 {
		return time.Time{}, fmt.Errorf("unrecognized timestamp in line: %q", line)
	}
	timestamp, err := time.ParseInLocation("Jan _2 15:04:05", strings.Join(fields[0:3], " "), location)
	if err != nil {
		return time.Time{}, err
	}

	// The BSD format omits the year; assume the current year
	timestamp = timestamp.AddDate(now.Year(), 0, 0)

	// Handle New Year's edge case (if timestamp is in the future)
	if timestamp.After(now) {
		timestamp = timestamp.AddDate(-1, 0, 0)
	}

	return timestamp, nil
}

// parseSSHLoginEvents scans syslog-formatted lines and returns the most
// recent SSH login within the last 24 hours for each (username, IP) pair
func parseSSHLoginEvents(r io.Reader) ([]SSHLoginEvent, error) {
	var events []SSHLoginEvent

	// Regular expression to match the SSH login pattern
	regex := regexp.MustCompile(`Accepted publickey for (\w+) from ([\d\.]+) port \d+ ssh2`)

	scanner := bufio.NewScanner(r)
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
			timestamp, err := parseSyslogTimestamp(line, now, location)
			if err != nil {
				slog.Error("Error parsing timestamp", slog.Any("error", err))
				continue
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
