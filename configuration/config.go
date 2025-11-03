package configuration

import (
	"cartographer-go-agent/common"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ConfigYamlFile represents a YAML file to be sent to the server
type ConfigYamlFile struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// JSONCommand represents a JSON command to be sent to the server
type JSONCommand struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout"`
}

// Config represents the configuration for the agent
type Config struct {
	URL              string           `yaml:"url"`
	IntervalMinutes  int              `yaml:"interval_minutes"`
	JitterSeconds    int              `yaml:"jitter_seconds"`
	Daemonize        bool             `yaml:"daemonize"`
	FQDN             string           `yaml:"fqdn"`
	Token            string           `yaml:"token"`
	YamlFiles        []ConfigYamlFile `yaml:"yaml_files"`
	JSONCommands     []JSONCommand    `yaml:"json_commands"`
	Gzip             bool             `yaml:"gzip"`
	LogLevel         string           `yaml:"log_level"`
	ReleaseURL       string           `yaml:"release_url"`
	EnableMonitoring *bool            `yaml:"enable_monitoring"`
	MonitorsDir      string           `yaml:"monitors_dir"`
	DRYRUN           bool
}

// GetConfig reads the configuration from the provided path
func GetConfig(configPath string) (Config, error) {
	config := &Config{}

	if err := common.ValidateFile(configPath); err != nil {
		return Config{}, err
	}

	file, err := os.Open(configPath)
	if err != nil {
		return Config{}, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
		}
	}(file)

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return Config{}, err
	}

	// Set default log level to "info" if none is provided
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	// Set monitoring defaults
	if config.MonitorsDir == "" {
		config.MonitorsDir = "/etc/cartographer/monitors.d"
	}

	if config.EnableMonitoring == nil {
		defaultValue := true
		config.EnableMonitoring = &defaultValue
	}

	return *config, nil
}

// ValidateConfig checks that the config has required fields and valid values
func ValidateConfig(config Config) error {
	if config.DRYRUN {
		return nil
	}

	if config.URL == "" {
		return errors.New("URL must be provided unless running in dry-run mode")
	}
	if config.Token == "" {
		return errors.New("token must be provided")
	}

	if config.Daemonize && config.IntervalMinutes < 1 {
		return fmt.Errorf("interval_minutes must be greater than 0 when daemonize is enabled")
	}

	validLogLevels := map[string]bool{"info": true, "debug": true, "warn": true, "error": true}
	if config.LogLevel != "" && !validLogLevels[config.LogLevel] {
		return fmt.Errorf("log_level must be one of 'info', 'debug', 'warn', or 'error'")
	}

	return nil
}

// IsMonitoringEnabled returns true if monitoring is enabled in the config
func (c *Config) IsMonitoringEnabled() bool {
	return c.EnableMonitoring != nil && *c.EnableMonitoring
}
