package monitors

import (
	"testing"
)

func TestMonitorValidation(t *testing.T) {
	tests := []struct {
		name      string
		monitor   Monitor
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid http monitor",
			monitor: Monitor{
				Name: "test-http",
				Type: "http",
				URL:  "http://example.com",
			},
			wantError: false,
		},
		{
			name: "http monitor missing url",
			monitor: Monitor{
				Name: "test-http",
				Type: "http",
			},
			wantError: true,
			errorMsg:  "url is required",
		},
		{
			name: "valid port monitor",
			monitor: Monitor{
				Name: "test-port",
				Type: "port",
				Port: 8080,
			},
			wantError: false,
		},
		{
			name: "port monitor missing port",
			monitor: Monitor{
				Name: "test-port",
				Type: "port",
			},
			wantError: true,
			errorMsg:  "port is required",
		},
		{
			name: "valid systemd monitor",
			monitor: Monitor{
				Name:   "test-systemd",
				Type:   "systemd",
				Target: "nginx",
			},
			wantError: false,
		},
		{
			name: "systemd monitor missing target",
			monitor: Monitor{
				Name: "test-systemd",
				Type: "systemd",
			},
			wantError: true,
			errorMsg:  "target is required",
		},
		{
			name: "invalid monitor type",
			monitor: Monitor{
				Name: "test-invalid",
				Type: "invalid",
			},
			wantError: true,
			errorMsg:  "invalid monitor type",
		},
		{
			name: "missing name",
			monitor: Monitor{
				Type: "http",
				URL:  "http://example.com",
			},
			wantError: true,
			errorMsg:  "name is required",
		},
		{
			name: "invalid priority",
			monitor: Monitor{
				Name:     "test",
				Type:     "http",
				URL:      "http://example.com",
				Priority: "super-critical",
			},
			wantError: true,
			errorMsg:  "invalid priority",
		},
		{
			name: "udp with non-localhost host",
			monitor: Monitor{
				Name:     "test-udp-remote",
				Type:     "port",
				Port:     53,
				Host:     "8.8.8.8",
				Protocol: "udp",
			},
			wantError: true,
			errorMsg:  "udp protocol only supports localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For valid monitors, apply defaults first (as real code does)
			if !tt.wantError {
				tt.monitor.ApplyDefaults()
			}

			err := tt.monitor.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		monitor  Monitor
		validate func(t *testing.T, m Monitor)
	}{
		{
			name: "http monitor defaults",
			monitor: Monitor{
				Name: "test",
				Type: "http",
				URL:  "http://example.com",
			},
			validate: func(t *testing.T, m Monitor) {
				if m.Priority != "medium" {
					t.Errorf("expected priority 'medium', got %q", m.Priority)
				}
				if m.Timeout != 10 {
					t.Errorf("expected timeout 10, got %d", m.Timeout)
				}
				if m.Retries != 1 {
					t.Errorf("expected retries 1, got %d", m.Retries)
				}
				if m.Method != "GET" {
					t.Errorf("expected method 'GET', got %q", m.Method)
				}
				if m.VerifyTLS == nil || !*m.VerifyTLS {
					t.Error("expected verify_tls true")
				}
				if m.FollowRedirects == nil || *m.FollowRedirects {
					t.Error("expected follow_redirects false")
				}
				if m.Validations == nil || len(m.Validations.StatusCodes) != 1 || m.Validations.StatusCodes[0] != 200 {
					t.Error("expected default status_codes [200]")
				}
			},
		},
		{
			name: "port monitor defaults",
			monitor: Monitor{
				Name: "test",
				Type: "port",
				Port: 8080,
			},
			validate: func(t *testing.T, m Monitor) {
				if m.Host != "localhost" {
					t.Errorf("expected host 'localhost', got %q", m.Host)
				}
				if m.Protocol != "tcp" {
					t.Errorf("expected protocol 'tcp', got %q", m.Protocol)
				}
			},
		},
		{
			name: "systemd monitor defaults",
			monitor: Monitor{
				Name:   "test",
				Type:   "systemd",
				Target: "nginx",
			},
			validate: func(t *testing.T, m Monitor) {
				if m.Validations == nil {
					t.Fatal("expected validations to be set")
				}
				if m.Validations.State != "active" {
					t.Errorf("expected state 'active', got %q", m.Validations.State)
				}
				if m.Validations.Enabled == nil || !*m.Validations.Enabled {
					t.Error("expected enabled true")
				}
			},
		},
		{
			name: "preserve explicit values",
			monitor: Monitor{
				Name:     "test",
				Type:     "http",
				URL:      "http://example.com",
				Priority: "critical",
				Timeout:  30,
				Retries:  5,
				Method:   "POST",
			},
			validate: func(t *testing.T, m Monitor) {
				if m.Priority != "critical" {
					t.Errorf("expected priority 'critical', got %q", m.Priority)
				}
				if m.Timeout != 30 {
					t.Errorf("expected timeout 30, got %d", m.Timeout)
				}
				if m.Retries != 5 {
					t.Errorf("expected retries 5, got %d", m.Retries)
				}
				if m.Method != "POST" {
					t.Errorf("expected method 'POST', got %q", m.Method)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.monitor.ApplyDefaults()
			tt.validate(t, tt.monitor)
		})
	}
}

func TestMonitorEndpoint(t *testing.T) {
	tests := []struct {
		baseURL  string
		expected string
	}{
		{
			baseURL:  "http://example.com",
			expected: "http://example.com/monitoring",
		},
		{
			baseURL:  "http://example.com/",
			expected: "http://example.com/monitoring",
		},
		{
			baseURL:  "https://api.example.com/v1",
			expected: "https://api.example.com/v1/monitoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			result := GetMonitorEndpoint(tt.baseURL)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
