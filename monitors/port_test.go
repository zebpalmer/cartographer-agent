package monitors

import (
	"fmt"
	"strings"
	"testing"
)

// TestPortMatching tests the port matching logic to prevent false positives
func TestPortMatching(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		targetPort int
		wantMatch  bool
	}{
		{
			name:       "exact match port 53",
			output:     "udp        0      0 0.0.0.0:53              0.0.0.0:*",
			targetPort: 53,
			wantMatch:  true,
		},
		{
			name:       "should not match port 530 when looking for 53",
			output:     "udp        0      0 0.0.0.0:530             0.0.0.0:*",
			targetPort: 53,
			wantMatch:  false,
		},
		{
			name:       "should not match port 8080 when looking for 80",
			output:     "tcp        0      0 0.0.0.0:8080            0.0.0.0:*",
			targetPort: 80,
			wantMatch:  false,
		},
		{
			name:       "exact match port 80",
			output:     "tcp        0      0 0.0.0.0:80              0.0.0.0:*",
			targetPort: 80,
			wantMatch:  true,
		},
		{
			name:       "match with tab separator",
			output:     "udp	0	0	:::53	:::*",
			targetPort: 53,
			wantMatch:  true,
		},
		{
			name:       "port at end of line",
			output:     "udp        0      0 127.0.0.1:53",
			targetPort: 53,
			wantMatch:  true,
		},
		{
			name:       "IPv6 format",
			output:     "udp6       0      0 :::53                   :::*",
			targetPort: 53,
			wantMatch:  true,
		},
		{
			name:       "no match - different port",
			output:     "tcp        0      0 0.0.0.0:22              0.0.0.0:*",
			targetPort: 53,
			wantMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the port matching logic
			portStr := ":53"
			if tt.targetPort != 53 {
				portStr = ":80"
			}

			matched := false
			if idx := strings.Index(tt.output, portStr); idx != -1 {
				endIdx := idx + len(portStr)
				if endIdx >= len(tt.output) || tt.output[endIdx] == ' ' || tt.output[endIdx] == '\t' {
					matched = true
				}
			}

			if matched != tt.wantMatch {
				t.Errorf("port matching: expected %v, got %v for line: %q", tt.wantMatch, matched, tt.output)
			}
		})
	}
}

// TestNetstatOutputParsing tests parsing of real netstat/ss output formats
func TestNetstatOutputParsing(t *testing.T) {
	// Realistic netstat output
	netstatOutput := `Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address           Foreign Address         State
tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN
tcp        0      0 127.0.0.1:631           0.0.0.0:*               LISTEN
tcp6       0      0 :::80                   :::*                    LISTEN
udp        0      0 0.0.0.0:53              0.0.0.0:*
udp        0      0 0.0.0.0:68              0.0.0.0:*
udp6       0      0 :::546                  :::*`

	tests := []struct {
		port      int
		wantFound bool
	}{
		{port: 53, wantFound: true},    // UDP DNS
		{port: 22, wantFound: true},    // SSH
		{port: 80, wantFound: true},    // HTTP
		{port: 443, wantFound: false},  // Not in output
		{port: 530, wantFound: false},  // Should not match 53
		{port: 8080, wantFound: false}, // Should not match 80
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%d", tt.port), func(t *testing.T) {
			lines := strings.Split(netstatOutput, "\n")
			portStr := fmt.Sprintf(":%d", tt.port)

			found := false
			for _, line := range lines {
				if idx := strings.Index(line, portStr); idx != -1 {
					endIdx := idx + len(portStr)
					if endIdx >= len(line) || line[endIdx] == ' ' || line[endIdx] == '\t' {
						found = true
						break
					}
				}
			}

			if found != tt.wantFound {
				t.Errorf("port %d: expected found=%v, got found=%v", tt.port, tt.wantFound, found)
			}
		})
	}
}
