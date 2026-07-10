package collectors

import (
	"fmt"
	"reflect"
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

	// Mix of traditional BSD syslog timestamps and the RFC 3339
	// high-precision format used by modern rsyslog
	mockLogData := fmt.Sprintf(`
%s localhost sshd[12345]: Accepted publickey for user5 from 10.10.176.144 port 22 ssh2
%s localhost sshd[67890]: Accepted publickey for user1 from 192.168.146.48 port 22 ssh2
%s prod-web-app-3 sshd[3785270]: Accepted publickey for user7 from 10.20.30.40 port 22 ssh2
%s prod-web-app-3 sshd[3785271]: Accepted publickey for user8 from 10.20.30.41 port 22 ssh2
`,
		now.Add(-2*time.Hour).Format("Jan _2 15:04:05"),
		now.Add(-3*time.Hour).Format("Jan _2 15:04:05"),
		now.Add(-1*time.Hour).UTC().Format("2006-01-02T15:04:05.000000+00:00"),
		now.Add(-30*time.Hour).UTC().Format("2006-01-02T15:04:05.000000+00:00"), // older than 24h, should be excluded
	)

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
		{
			Username: "user7",
			Time:     now.Add(-1 * time.Hour).UTC().Format(time.RFC3339), // Expected value in UTC
			SourceIP: "10.20.30.40",
		},
	}

	events, err := parseSSHLoginEvents(strings.NewReader(mockLogData))
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

func TestParseSyslogTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		line    string
		want    time.Time
		wantErr bool
	}{
		{
			name: "RFC 3339 with microseconds and offset",
			line: "2026-07-10T07:30:22.434548+00:00 prod-web-app-3 sshd[3785270]: Accepted publickey for user7 from 10.20.30.40 port 22 ssh2",
			want: time.Date(2026, 7, 10, 7, 30, 22, 434548000, time.UTC),
		},
		{
			name: "traditional BSD format",
			line: "Jul 10 07:30:22 localhost sshd[12345]: Accepted publickey for user5 from 10.10.176.144 port 22 ssh2",
			want: time.Date(2026, 7, 10, 7, 30, 22, 0, time.UTC),
		},
		{
			name: "BSD format in the future rolls back a year",
			line: "Dec 31 23:59:59 localhost sshd[12345]: Accepted publickey for user5 from 10.10.176.144 port 22 ssh2",
			want: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "garbage",
			line:    "not a timestamp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSyslogTimestamp(tt.line, now, time.UTC)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
