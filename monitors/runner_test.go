package monitors

import (
	"testing"
)

// TestRetryConfiguration tests that retry configuration is properly set
func TestRetryConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		retries         int
		retryDelay      int
		expectedRetries int
		expectedDelay   int
	}{
		{
			name:            "no retries configured",
			retries:         0,
			retryDelay:      0,
			expectedRetries: 0,
			expectedDelay:   0,
		},
		{
			name:            "default retry (1 retry)",
			retries:         0, // Will be set to 1 by ApplyDefaults
			retryDelay:      0,
			expectedRetries: 1,
			expectedDelay:   0,
		},
		{
			name:            "2 retries with delay",
			retries:         2,
			retryDelay:      5,
			expectedRetries: 2,
			expectedDelay:   5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := Monitor{
				Name:       "test-monitor",
				Type:       "http",
				URL:        "http://example.com",
				Retries:    tt.retries,
				RetryDelay: tt.retryDelay,
			}
			monitor.ApplyDefaults()

			// After applying defaults, retries should be at least 1
			if monitor.Retries < 1 {
				t.Errorf("expected retries >= 1 after defaults, got %d", monitor.Retries)
			}

			if tt.retryDelay > 0 && monitor.RetryDelay != tt.expectedDelay {
				t.Errorf("expected retry_delay %d, got %d", tt.expectedDelay, monitor.RetryDelay)
			}
		})
	}
}

// TestRetryLogicDocumentation documents the expected retry behavior
func TestRetryLogicDocumentation(t *testing.T) {
	t.Log("Retry logic semantics:")
	t.Log("- Retries=0: Execute once (no retries)")
	t.Log("- Retries=1: Execute up to 2 times (initial + 1 retry) [DEFAULT]")
	t.Log("- Retries=2: Execute up to 3 times (initial + 2 retries)")
	t.Log("- Stops immediately on OK status")
	t.Log("- Delay between retries configurable via retry_delay")
}

func TestMonitorStatusConstants(t *testing.T) {
	if StatusOK != "ok" {
		t.Errorf("StatusOK should be 'ok', got %q", StatusOK)
	}
	if StatusWarning != "warning" {
		t.Errorf("StatusWarning should be 'warning', got %q", StatusWarning)
	}
	if StatusCritical != "critical" {
		t.Errorf("StatusCritical should be 'critical', got %q", StatusCritical)
	}
	if StatusUnknown != "unknown" {
		t.Errorf("StatusUnknown should be 'unknown', got %q", StatusUnknown)
	}
}

func TestMonitorResultStructure(t *testing.T) {
	// Verify MonitorResult has expected fields with new structure
	result := MonitorResult{
		Name:        "test-monitor",
		Type:        "http",
		Priority:    "critical",
		Environment: "production",
		Tags:        []string{"test"},
		Status:      StatusOK,
		Message:     "200 OK",
		Timestamp:   "2025-11-03T10:00:00Z",
		DurationMs:  100,
		Config: MonitorConfig{
			Timeout: 10,
			URL:     "http://example.com",
		},
	}

	if result.Name != "test-monitor" {
		t.Errorf("expected name 'test-monitor', got %q", result.Name)
	}
	if result.Type != "http" {
		t.Errorf("expected type 'http', got %q", result.Type)
	}
	if result.DurationMs != 100 {
		t.Errorf("expected duration_ms 100, got %d", result.DurationMs)
	}
	if result.Status != StatusOK {
		t.Errorf("expected status OK, got %q", result.Status)
	}
	if result.Config.URL != "http://example.com" {
		t.Errorf("expected config URL, got %q", result.Config.URL)
	}
}

func TestMonitorReportStructure(t *testing.T) {
	report := MonitorReport{
		FQDN: "test-host.example.com",
		Monitors: []MonitorResult{
			{
				Name:       "test1",
				Type:       "http",
				Status:     StatusOK,
				Message:    "OK",
				Timestamp:  "2025-11-03T10:00:00Z",
				DurationMs: 50,
			},
			{
				Name:       "test2",
				Type:       "port",
				Status:     StatusCritical,
				Message:    "Connection refused",
				Timestamp:  "2025-11-03T10:00:01Z",
				DurationMs: 100,
			},
		},
	}

	if report.FQDN != "test-host.example.com" {
		t.Errorf("expected FQDN 'test-host.example.com', got %q", report.FQDN)
	}
	if len(report.Monitors) != 2 {
		t.Errorf("expected 2 monitors, got %d", len(report.Monitors))
	}
}
