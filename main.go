package main

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"cartographer-go-agent/internal"
	"cartographer-go-agent/monitors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
)

var (
	// Version holds the current version of the agent
	Version = "dev" // Will be set during release
)

// BuildVersion returns the version information for the agent
func BuildVersion() string {
	return fmt.Sprintf("Cartographer-Agent: %s", Version)
}

// getLogLevelFromConfig maps the log level string from config to slog levels and normalizes the input to lowercase.
func getLogLevelFromConfig(logLevel string) slog.Level {
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
		return slog.LevelInfo
	}
}

// main is the entry point for the agent
func main() {
	configPath := flag.String("config", "", "Path to cartographer agent config file")
	natsURLFlag := flag.String("nats-url", "", "NATS server URL (e.g., tls://nats.example.com:4222)")
	natsNkeySeedFlag := flag.String("nats-nkey-seed", "", "NATS NKey seed for authentication")
	daemonize := flag.Bool("daemonize", false, "Continue running as a daemon")
	intervalMinutes := flag.Int("interval_minutes", 15, "How often reports should be sent in minutes")
	fqdn := flag.String("fqdn", "", "Override the FQDN sent to cartographer")
	showVersion := flag.Bool("version", false, "Show cartographer-agent version")
	dryrun := flag.Bool("dryrun", false, "Run once and print payload")
	validateConfig := flag.Bool("validate-config", false, "Validate the configuration file and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println(BuildVersion())
		os.Exit(0)
	}

	var config configuration.Config
	var err error

	if *configPath != "" {
		config, err = configuration.GetConfig(*configPath)
		if err != nil {
			fmt.Printf("Error loading configuration: %s\n", err.Error())
			os.Exit(1)
		}
	} else {
		config.NatsURL = *natsURLFlag
		config.NatsNkeySeed = *natsNkeySeedFlag
		config.Daemonize = *daemonize
		config.IntervalMinutes = *intervalMinutes
		config.FQDN = *fqdn
	}
	config.DRYRUN = *dryrun

	// Run validation before proceeding further
	if err := configuration.ValidateConfig(config); err != nil {
		fmt.Printf("Configuration validation error: %s\nSee --help for more information.\n", err.Error())
		os.Exit(1)
	}

	// Exit after validation if --validate-config is set
	if *validateConfig {
		fmt.Println("Configuration is valid.")
		os.Exit(0)
	}

	logLevel := getLogLevelFromConfig(config.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
	slog.Info("Configuration loaded successfully")

	collectorsList := internal.GetCollectors(config)

	// Establish NATS connection (skip in dry-run mode)
	var nc *nats.Conn
	if !config.DRYRUN {
		nc, err = common.ConnectNATS(config.NatsURL, config.NatsNkeySeed)
		if err != nil {
			slog.Error("Failed to connect to NATS", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer nc.Close()
	}

	// Start monitoring system in background (if enabled)
	if config.IsMonitoringEnabled() {
		go monitors.Run(config, Version, nc)
	} else {
		slog.Info("Monitoring is disabled")
	}

	// Start main agent (existing collectors, heartbeat, updates)
	internal.RunAgent(config, collectorsList, Version, nc)
}
