package collectors

import (
	"cartographer-go-agent/common"
	"errors"
	"fmt"
	"io/fs"
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

func TestSSHLoginsFromJournal(t *testing.T) {
	now := time.Now()
	// Shapes from real hosts: systemd 252 / OpenSSH 9.2 (offset without a colon, sshd) and
	// systemd 257 / OpenSSH 10 (RFC 3339 offset, sshd-session).
	out := fmt.Sprintf(`%s alluvium sshd[682058]: Accepted key ED25519 SHA256:abc found at /home/alice/.ssh/authorized_keys:1
%s alluvium sshd[682058]: Accepted publickey for alice from 100.64.1.2 port 59042 ssh2: ED25519 SHA256:abc
%s leeloo-n1 sshd-session[245274]: Accepted publickey for bob from 100.64.1.3 port 59060 ssh2: ED25519 SHA256:def
%s leeloo-n1 sshd-session[245275]: Accepted publickey for carol from 100.64.1.4 port 59061 ssh2: ED25519 SHA256:ghi`,
		now.Add(-1*time.Hour).Format("2006-01-02T15:04:05-0700"),
		now.Add(-1*time.Hour).Format("2006-01-02T15:04:05-0700"),
		now.Add(-2*time.Hour).Format(time.RFC3339),
		now.Add(-30*time.Hour).Format(time.RFC3339), // older than 24h, excluded
	)

	orig := runJournalCommand
	var gotCommand string
	runJournalCommand = func(cmd string, _ *common.CommandOptions) (string, string, int, error) {
		gotCommand = cmd
		return out, "", 0, nil
	}
	t.Cleanup(func() { runJournalCommand = orig })

	events, err := sshLoginsFromJournal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotCommand, "_COMM=sshd ") || !strings.Contains(gotCommand, "_COMM=sshd-session") {
		t.Errorf("journal query must match both sshd and sshd-session: %q", gotCommand)
	}

	sort.Sort(SSHLoginEventSorter(events))
	want := []SSHLoginEvent{
		{Username: "alice", SourceIP: "100.64.1.2", Time: now.Add(-1 * time.Hour).UTC().Format(time.RFC3339)},
		{Username: "bob", SourceIP: "100.64.1.3", Time: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339)},
	}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("events = %+v, want %+v", events, want)
	}
}

func TestSSHLoginsFromJournalCommandFails(t *testing.T) {
	orig := runJournalCommand
	runJournalCommand = func(string, *common.CommandOptions) (string, string, int, error) {
		return "", "No journal files were found.", 1, nil
	}
	t.Cleanup(func() { runJournalCommand = orig })

	if _, err := sshLoginsFromJournal(); err == nil || !strings.Contains(err.Error(), "exited with code 1") {
		t.Errorf("expected an exit-code error, got %v", err)
	}
}

func TestMissingAuthLogIsNotExist(t *testing.T) {
	// The collector falls back to journald only for a missing file, not for other errors.
	_, err := parseSSHLogins(t.TempDir() + "/no-such-auth.log")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}
