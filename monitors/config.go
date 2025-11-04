package monitors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MonitorFile represents the structure of a monitor configuration file
type MonitorFile struct {
	Monitors []Monitor `yaml:"monitors"`
}

// Monitor represents a single monitor configuration
type Monitor struct {
	// Common fields
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	Priority    string   `yaml:"priority" json:"priority,omitempty"`
	Environment string   `yaml:"environment" json:"environment,omitempty"`
	Tags        []string `yaml:"tags" json:"tags,omitempty"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Timeout     int      `yaml:"timeout" json:"timeout"`
	Retries     int      `yaml:"retries" json:"retries"`
	RetryDelay  int      `yaml:"retry_delay" json:"retry_delay"`

	// HTTP-specific fields
	URL             string            `yaml:"url" json:"url,omitempty"`
	Method          string            `yaml:"method" json:"method,omitempty"`
	Headers         map[string]string `yaml:"headers" json:"headers,omitempty"`
	Body            string            `yaml:"body" json:"body,omitempty"`
	VerifyTLS       *bool             `yaml:"verify_tls" json:"verify_tls,omitempty"`
	FollowRedirects *bool             `yaml:"follow_redirects" json:"follow_redirects,omitempty"`
	Validations     *Validations      `yaml:"validations,omitempty" json:"validations,omitempty"`

	// Port-specific fields
	Port     int    `yaml:"port" json:"port,omitempty"`
	Host     string `yaml:"host" json:"host,omitempty"`
	Protocol string `yaml:"protocol" json:"protocol,omitempty"`

	// Systemd-specific fields
	Target string `yaml:"target" json:"target,omitempty"`
}

// Validations represents validation rules for monitors (type-specific fields)
type Validations struct {
	// HTTP validations
	StatusCodes    []int  `yaml:"status_codes" json:"status_codes,omitempty"`
	BodyContains   string `yaml:"body_contains" json:"body_contains,omitempty"`
	BodyRegex      string `yaml:"body_regex" json:"body_regex,omitempty"`
	CertExpiryDays int    `yaml:"cert_expiry_days" json:"cert_expiry_days,omitempty"`

	// Systemd validations
	State        string `yaml:"state" json:"state,omitempty"`
	Enabled      *bool  `yaml:"enabled" json:"enabled,omitempty"`
	RestartCount *int   `yaml:"restart_count" json:"restart_count,omitempty"`
}

// ApplyDefaults applies default values to a monitor configuration
func (m *Monitor) ApplyDefaults() {
	// Common defaults
	if m.Priority == "" {
		m.Priority = "medium"
	}
	if m.Timeout == 0 {
		m.Timeout = 10
	}
	if m.Retries == 0 {
		m.Retries = 1
	}

	// HTTP defaults
	if m.Type == "http" {
		if m.Method == "" {
			m.Method = "GET"
		}
		if m.VerifyTLS == nil {
			defaultVerifyTLS := true
			m.VerifyTLS = &defaultVerifyTLS
		}
		if m.FollowRedirects == nil {
			defaultFollowRedirects := false
			m.FollowRedirects = &defaultFollowRedirects
		}
		if m.Validations == nil {
			m.Validations = &Validations{
				StatusCodes: []int{200},
			}
		} else if len(m.Validations.StatusCodes) == 0 {
			m.Validations.StatusCodes = []int{200}
		}
	}

	// Port defaults
	if m.Type == "port" {
		if m.Host == "" {
			m.Host = "localhost"
		}
		if m.Protocol == "" {
			m.Protocol = "tcp"
		}
	}

	// Systemd defaults
	if m.Type == "systemd" {
		if m.Validations == nil {
			defaultEnabled := true
			m.Validations = &Validations{
				State:   "active",
				Enabled: &defaultEnabled,
			}
		} else {
			if m.Validations.State == "" {
				m.Validations.State = "active"
			}
			if m.Validations.Enabled == nil {
				defaultEnabled := true
				m.Validations.Enabled = &defaultEnabled
			}
		}
	}
}

// Validate checks if a monitor configuration is valid
func (m *Monitor) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("monitor name is required")
	}
	if m.Type == "" {
		return fmt.Errorf("monitor type is required for '%s'", m.Name)
	}

	validTypes := map[string]bool{"http": true, "port": true, "systemd": true}
	if !validTypes[m.Type] {
		return fmt.Errorf("invalid monitor type '%s' for '%s', must be http, port, or systemd", m.Type, m.Name)
	}

	validPriorities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "info": true}
	if m.Priority != "" && !validPriorities[m.Priority] {
		return fmt.Errorf("invalid priority '%s' for '%s'", m.Priority, m.Name)
	}

	// Type-specific validation
	switch m.Type {
	case "http":
		if m.URL == "" {
			return fmt.Errorf("url is required for http monitor '%s'", m.Name)
		}
	case "port":
		if m.Port == 0 {
			return fmt.Errorf("port is required for port monitor '%s'", m.Name)
		}
		if m.Protocol != "tcp" && m.Protocol != "udp" {
			return fmt.Errorf("invalid protocol '%s' for port monitor '%s', must be tcp or udp", m.Protocol, m.Name)
		}
		// UDP only supports localhost
		if m.Protocol == "udp" && m.Host != "localhost" {
			return fmt.Errorf("udp protocol only supports localhost for monitor '%s'", m.Name)
		}
	case "systemd":
		if m.Target == "" {
			return fmt.Errorf("target is required for systemd monitor '%s'", m.Name)
		}
	}

	return nil
}

// LoadMonitors reads all monitor configurations from the specified directory
func LoadMonitors(monitorsDir string) ([]Monitor, []error) {
	var allMonitors []Monitor
	var errors []error

	// Check if directory exists
	if _, err := os.Stat(monitorsDir); os.IsNotExist(err) {
		return allMonitors, []error{fmt.Errorf("monitors directory does not exist: %s", monitorsDir)}
	}

	// Read all .yaml files in directory
	files, err := filepath.Glob(filepath.Join(monitorsDir, "*.yaml"))
	if err != nil {
		return allMonitors, []error{fmt.Errorf("error reading monitors directory: %w", err)}
	}

	// Also check for .yml extension
	ymlFiles, err := filepath.Glob(filepath.Join(monitorsDir, "*.yml"))
	if err != nil {
		return allMonitors, []error{fmt.Errorf("error reading monitors directory: %w", err)}
	}
	files = append(files, ymlFiles...)

	if len(files) == 0 {
		return allMonitors, nil // No files, no error
	}

	// Parse each file
	for _, file := range files {
		monitors, err := loadMonitorFile(file)
		if err != nil {
			errors = append(errors, fmt.Errorf("error loading %s: %w", filepath.Base(file), err))
			continue
		}

		// Validate and apply defaults to each monitor
		for i := range monitors {
			monitors[i].ApplyDefaults()
			if err := monitors[i].Validate(); err != nil {
				errors = append(errors, fmt.Errorf("invalid monitor in %s: %w", filepath.Base(file), err))
				continue
			}
			allMonitors = append(allMonitors, monitors[i])
		}
	}

	return allMonitors, errors
}

// loadMonitorFile reads a single monitor configuration file
func loadMonitorFile(path string) ([]Monitor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var monitorFile MonitorFile
	if err := yaml.Unmarshal(data, &monitorFile); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	return monitorFile.Monitors, nil
}

// GetMonitorEndpoint returns the full URL for posting monitor results
func GetMonitorEndpoint(baseURL string) string {
	return strings.TrimSuffix(baseURL, "/") + "/monitoring"
}
