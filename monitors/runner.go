package monitors

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// MonitorStatus represents the status of a monitor check
type MonitorStatus string

// Status constants follow Nagios/Icinga conventions
const (
	StatusOK       MonitorStatus = "ok"
	StatusWarning  MonitorStatus = "warning"
	StatusCritical MonitorStatus = "critical"
	StatusUnknown  MonitorStatus = "unknown"
)

// MonitorResult represents the result of a single monitor execution
type MonitorResult struct {
	// Metadata - WHO/WHAT this monitor is
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"`

	// Result data
	Status     MonitorStatus `json:"status"`
	Message    string        `json:"message"`
	Timestamp  string        `json:"timestamp"`
	DurationMs int64         `json:"duration_ms"`

	// Technical config - HOW it works
	Config MonitorConfig `json:"config"`
}

// MonitorConfig contains technical implementation details only
type MonitorConfig struct {
	// Common technical fields
	Timeout    int `json:"timeout"`
	Retries    int `json:"retries"`
	RetryDelay int `json:"retry_delay"`

	// HTTP-specific
	URL             string            `json:"url,omitempty"`
	Method          string            `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	VerifyTLS       *bool             `json:"verify_tls,omitempty"`
	FollowRedirects *bool             `json:"follow_redirects,omitempty"`
	Validations     *Validations      `json:"validations,omitempty"`

	// Port-specific
	Port     int    `json:"port,omitempty"`
	Host     string `json:"host,omitempty"`
	Protocol string `json:"protocol,omitempty"`

	// Systemd-specific
	Target string `json:"target,omitempty"`

	// Command-specific
	Command    string `json:"command,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
}

// MonitorReport represents the full report sent to cartographer
type MonitorReport struct {
	FQDN     string          `json:"fqdn"`
	Monitors []MonitorResult `json:"monitors"`
}

// Run is the main entry point for the monitoring system
func Run(config configuration.Config, version string) {
	slog.Info("Starting monitoring system",
		slog.String("monitors_dir", config.MonitorsDir),
		slog.String("version", version),
	)

	// Get FQDN for reporting
	fqdn := getFQDN(config)

	// Main monitoring loop - run every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	runMonitoringCycle(config, fqdn)

	// Then run every minute
	for range ticker.C {
		runMonitoringCycle(config, fqdn)
	}
}

// runMonitoringCycle executes one complete monitoring cycle
func runMonitoringCycle(config configuration.Config, fqdn string) {
	slog.Debug("Starting monitoring cycle")

	// Load monitor configurations
	monitors, loadErrors := LoadMonitors(config.MonitorsDir)

	// Log any load errors but continue with valid monitors
	for _, err := range loadErrors {
		slog.Error("Monitor configuration error", slog.String("error", err.Error()))
	}

	if len(monitors) == 0 {
		slog.Debug("No monitors configured, skipping cycle")
		return
	}

	slog.Info("Executing monitors", slog.Int("count", len(monitors)))

	// Execute all monitors and collect results
	var results []MonitorResult
	for _, monitor := range monitors {
		result := executeMonitor(monitor)
		results = append(results, result)

		// Log the result
		slog.Info("Monitor executed",
			slog.String("name", monitor.Name),
			slog.String("type", monitor.Type),
			slog.String("status", string(result.Status)),
			slog.String("message", result.Message),
			slog.Int64("duration_ms", result.DurationMs),
		)
	}

	// Add any load errors as UNKNOWN status monitors
	for _, err := range loadErrors {
		results = append(results, MonitorResult{
			Name:       "config_error",
			Type:       "config",
			Status:     StatusUnknown,
			Message:    err.Error(),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMs: 0,
		})
	}

	// Create report
	report := MonitorReport{
		FQDN:     fqdn,
		Monitors: results,
	}

	// Send report to cartographer
	if err := sendReport(config, report); err != nil {
		slog.Error("Failed to send monitoring report", slog.String("error", err.Error()))
	} else {
		slog.Debug("Monitoring report sent successfully")
	}
}

// executeMonitor runs a single monitor with retry logic
func executeMonitor(monitor Monitor) MonitorResult {
	var lastResult MonitorResult

	// Initial attempt (always runs once)
	lastResult = runMonitorCheck(monitor)

	// If OK or no retries configured, return immediately
	if lastResult.Status == StatusOK || monitor.Retries == 0 {
		return lastResult
	}

	// Retry logic
	for attempt := 1; attempt <= monitor.Retries; attempt++ {
		slog.Debug("Retrying monitor",
			slog.String("name", monitor.Name),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", monitor.Retries),
		)

		if monitor.RetryDelay > 0 {
			time.Sleep(time.Duration(monitor.RetryDelay) * time.Second)
		}

		// Execute the appropriate monitor type
		lastResult = runMonitorCheck(monitor)

		// If OK, no need to continue retrying
		if lastResult.Status == StatusOK {
			break
		}
	}

	return lastResult
}

func runMonitorCheck(monitor Monitor) MonitorResult {
	start := time.Now()

	var status MonitorStatus
	var message string

	switch monitor.Type {
	case "http":
		status, message = checkHTTP(monitor)
	case "port":
		status, message = checkPort(monitor)
	case "systemd":
		status, message = checkSystemd(monitor)
	case "command":
		status, message = checkCommand(monitor)
	default:
		status = StatusUnknown
		message = fmt.Sprintf("Unknown monitor type: %s", monitor.Type)
	}

	duration := time.Since(start).Milliseconds()

	// Build technical config (no metadata)
	config := MonitorConfig{
		Timeout:         monitor.Timeout,
		Retries:         monitor.Retries,
		RetryDelay:      monitor.RetryDelay,
		URL:             monitor.URL,
		Method:          monitor.Method,
		Headers:         monitor.Headers,
		Body:            monitor.Body,
		VerifyTLS:       monitor.VerifyTLS,
		FollowRedirects: monitor.FollowRedirects,
		Validations:     monitor.Validations,
		Port:            monitor.Port,
		Host:            monitor.Host,
		Protocol:        monitor.Protocol,
		Target:          monitor.Target,
		Command:         monitor.Command,
		WorkingDir:      monitor.WorkingDir,
	}

	return MonitorResult{
		// Metadata
		Name:        monitor.Name,
		Type:        monitor.Type,
		Priority:    monitor.Priority,
		Environment: monitor.Environment,
		Tags:        monitor.Tags,
		Description: monitor.Description,

		// Result
		Status:     status,
		Message:    message,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		DurationMs: duration,

		// Technical config
		Config: config,
	}
}

// sendReport posts the monitor report to cartographer
func sendReport(config configuration.Config, report MonitorReport) error {
	if config.DRYRUN {
		jsonData, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println("DRYRUN - Would send monitoring report:")
		fmt.Println(string(jsonData))
		return nil
	}

	endpoint := GetMonitorEndpoint(config.URL)

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Use common PostReport function
	_, err = common.PostReport(endpoint, jsonData, config.Token, config.Gzip)
	if err != nil {
		return fmt.Errorf("failed to post monitoring report: %w", err)
	}

	return nil
}

// getFQDN returns the FQDN for this agent, using config override if set
func getFQDN(config configuration.Config) string {
	if config.FQDN != "" {
		return config.FQDN
	}

	hostname, err := os.Hostname()
	if err != nil {
		slog.Warn("Failed to get hostname", slog.String("error", err.Error()))
		return "unknown"
	}

	return hostname
}
