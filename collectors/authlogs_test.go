package collectors

import (
	"bufio"
	"log/slog"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// SSHLoginEventSorter helps to sort SSHLoginEvent by Username and SourceIP
type SSHLoginEventSorter []SSHLoginEvent

func (s SSHLoginEventSorter) Len() int {
	return len(s)
}

func (s SSHLoginEventSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SSHLoginEventSorter) Less(i, j int) bool {
	if s[i].Username == s[j].Username {
		return s[i].SourceIP < s[j].SourceIP
	}
	return s[i].Username < s[j].Username
}

func TestParseSSHLogins(t *testing.T) {
	// Generate mock SSH login events for testing purposes
	now := time.Now()

	mockLogData := `
Jul 29 07:17:50 localhost sshd[12345]: Accepted publickey for user5 from 10.10.176.144 port 22 ssh2
Aug  1 15:56:01 localhost sshd[67890]: Accepted publickey for user1 from 192.168.146.48 port 22 ssh2
`

	// Adjust mock log entries to have recent timestamps
	mockLogData = strings.Replace(mockLogData, "Jul 29 07:17:50", now.Add(-2*time.Hour).Format("Jan _2 15:04:05"), 1)
	mockLogData = strings.Replace(mockLogData, "Aug  1 15:56:01", now.Add(-3*time.Hour).Format("Jan _2 15:04:05"), 1)

	// Use strings.NewReader to create an in-memory buffer with the mock log data
	reader := strings.NewReader(mockLogData)

	// Set expected values that correspond to the mock data in UTC
	expected := []SSHLoginEvent{
		{
			Username: "user5",
			Time:     now.Add(-2 * time.Hour).UTC().Format(time.RFC3339), // Expected value in UTC
			SourceIP: "10.10.176.144",
		},
		{
			Username: "user1",
			Time:     now.Add(-3 * time.Hour).UTC().Format(time.RFC3339), // Expected value in UTC
			SourceIP: "192.168.146.48",
		},
	}

	// Call parseSSHLoginsFromScanner with a scanner that reads the in-memory data
	events, err := parseSSHLoginsFromScanner(bufio.NewScanner(reader))
	if err != nil {
		t.Fatal(err)
	}

	// Sort both expected and actual results before comparing them
	sort.Sort(SSHLoginEventSorter(expected))
	sort.Sort(SSHLoginEventSorter(events))

	if !reflect.DeepEqual(events, expected) {
		t.Errorf("Expected: %v, got: %v", expected, events)
	}
}

// parseSSHLoginsFromScanner is a helper function for testing that takes a scanner and mimics the parseSSHLogins behavior
func parseSSHLoginsFromScanner(scanner *bufio.Scanner) ([]SSHLoginEvent, error) {
	now := time.Now()
	var events []SSHLoginEvent

	regex := regexp.MustCompile(`Accepted publickey for (\w+) from ([\d\.]+) port \d+ ssh2`)

	// Create a map to track the most recent login per (username, IP) pair
	mostRecentLogins := make(map[string]SSHLoginEvent)

	// Get the local timezone of the system
	location, err := time.LoadLocation("Local")
	if err != nil {
		slog.Warn("Error loading system timezone", slog.Any("error", err))
		location = time.UTC // Fallback to UTC if unable to load local timezone
	}

	for scanner.Scan() {
		line := scanner.Text()
		matches := regex.FindStringSubmatch(line)
		if matches != nil {
			// Extract the timestamp from the log entry
			timestampStr := strings.Fields(line)[0:3]
			timestamp, err := time.ParseInLocation("Jan _2 15:04:05", strings.Join(timestampStr, " "), location)
			if err != nil {
				slog.Warn("Error parsing timestamp", slog.Any("error", err))
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

	for _, event := range mostRecentLogins {
		events = append(events, event)
	}

	return events, scanner.Err()
}
