package main

import (
	"cartographer-go-agent/configuration"
	"cartographer-go-agent/internal"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var (
	// Version represents the current version of the application.
	Version = "dev"

	// CommitHash is the Git commit hash from which the binary was built.
	CommitHash = "dev"

	// BuildTimestamp is the timestamp of when the binary was built.
	BuildTimestamp = "n/a"
)

// BuildVersion returns the version information for the agent
func BuildVersion() string {
	return fmt.Sprintf("Cartographer-Agent: %s", Version)
}

// getLogLevelFromConfig maps the log level string from config to slog levels and normalizes the input to lowercase.
func getLogLevelFromConfig(logLevel string) slog.Level {
	// Normalize the log level input to lowercase
	switch strings.ToLower(logLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Default to info level if not recognized
		return slog.LevelInfo
	}
}

// main is the entry point for the agent
func main() {
	configPath := flag.String("config", "", "Path to cartographer agent config file")
	token := flag.String("token", "", "Agent Token")
	serverURL := flag.String("url", "", "URL of Cartographer Server")
	daemonize := flag.Bool("daemonize", false, "Continue running as a daemon")
	intervalMinutes := flag.Int("interval_minutes", 15, "How often reports should be sent in minutes")
	fqdn := flag.String("fqdn", "", "Override the FQDN sent to cartographer")
	showVersion := flag.Bool("version", false, "Show cartographer-agent version")
	dryrun := flag.Bool("dryrun", false, "Run once and print payload")
	validateConfig := flag.Bool("validate-config", false, "Validate the configuration file and exit")

	flag.Parse()

	if *showVersion {
		fmt.Printf(BuildVersion() + "\n")
		os.Exit(0)
	}

	var config configuration.Config
	var err error

	// Load configuration
	if *configPath != "" {
		config, err = configuration.GetConfig(*configPath)
		if err != nil {
			// Print error and exit if the configuration cannot be loaded
			slog.Error("Failed to load configuration", slog.String("path", *configPath), slog.String("error", err.Error()))
			os.Exit(1)
		}
	} else {
		// Fallback to command-line arguments if no config file provided
		config.URL = *serverURL
		config.Daemonize = *daemonize
		config.IntervalMinutes = *intervalMinutes
		config.FQDN = *fqdn
		config.Token = *token
	}
	config.DRYRUN = *dryrun

	// Ensure URL is provided unless it's a dry run
	if config.URL == "" && !config.DRYRUN {
		flag.Usage()
		fmt.Printf("\nMUST PROVIDE: url or request --dryrun\n")
		os.Exit(1)
	}

	// Always validate the loaded configuration
	if err := configuration.ValidateConfig(config); err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("Configuration validation failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// If the validate flag is set, exit after validation
	if *validateConfig {
		fmt.Println("Configuration is valid.")
		os.Exit(0)
	}

	// Set log level based on the config
	logLevel := getLogLevelFromConfig(config.LogLevel)

	// Initialize the logger with the appropriate level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	// assign this logger to the default for use in other functions
	slog.SetDefault(logger)

	// Log the successful config load
	slog.Info("Configuration loaded successfully")

	// Initialize collectors list
	slog.Debug("Initializing collectors")
	collectorsList := internal.GetCollectors(config)

	// Run the agent
	internal.RunAgent(config, collectorsList, Version)

}
