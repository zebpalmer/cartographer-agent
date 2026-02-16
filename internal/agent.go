package internal

import (
	"cartographer-go-agent/collectors"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
)

// agentCommand represents a command received via NATS
type agentCommand struct {
	Action        string `json:"action"`
	TargetVersion string `json:"target_version,omitempty"`
}

// RunAgent is the main entry point for the agent
func RunAgent(config configuration.Config, collectorsList []*collectors.Collector, version string, nc *nats.Conn) {
	// Create a new scheduler
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		slog.Error("Error creating scheduler", slog.String("error", err.Error()))
		return
	}

	// Subscribe to commands for this agent
	if nc != nil {
		fqdn := getFQDN(config)
		commandSubject := "agent.commands." + common.ReverseFQDN(fqdn)
		_, err := nc.Subscribe(commandSubject, func(msg *nats.Msg) {
			handleCommand(msg, config, version)
		})
		if err != nil {
			slog.Error("Failed to subscribe to commands",
				slog.String("subject", commandSubject),
				slog.String("error", err.Error()),
			)
		} else {
			slog.Info("Subscribed to commands", slog.String("subject", commandSubject))
		}
	}

	// Send a heartbeat immediately
	HeartbeatTask(config, version, nc)
	// skew the first ReportTask by random time between 0 and 60 seconds
	common.RandomSleep(0, 60)
	ReportTask(config, collectorsList, version, nc)

	if !config.Daemonize {
		slog.Warn("Non-daemon mode, exiting after sending report")
		os.Exit(0)
	} else {
		slog.Info("Starting agent in daemon mode")
	}

	// Schedule the heartbeat task
	_, err = scheduler.NewJob(
		gocron.DurationRandomJob(50*time.Second, 55*time.Second),
		gocron.NewTask(func() {
			HeartbeatTask(config, version, nc)
		}),
	)
	if err != nil {
		slog.Error("Error scheduling heartbeat job", slog.String("error", err.Error()))
	}

	// Get the report intervals
	minInterval, maxInterval := getReportIntervals(config)

	// Schedule the report task using DurationRandomJob for jitter
	_, err = scheduler.NewJob(
		gocron.DurationRandomJob(minInterval, maxInterval),
		gocron.NewTask(func() {
			ReportTask(config, collectorsList, version, nc)
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		slog.Error("Error scheduling report job", slog.String("error", err.Error()))
	}

	// Start the scheduler asynchronously
	scheduler.Start()

	// Keep the agent running indefinitely
	select {}
}

func handleCommand(msg *nats.Msg, config configuration.Config, version string) {
	var cmd agentCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		slog.Error("Failed to parse command", slog.String("error", err.Error()))
		return
	}

	slog.Info("Received command", slog.String("action", cmd.Action))

	switch cmd.Action {
	case "update":
		if cmd.TargetVersion == "" {
			slog.Warn("Update command missing target_version")
			return
		}
		if cmd.TargetVersion == version {
			slog.Debug("Already running target version", slog.String("version", version))
			return
		}
		slog.Info("Triggering self-update", slog.String("target_version", cmd.TargetVersion))
		if err := SelfUpdate(cmd.TargetVersion, config); err != nil {
			slog.Error("Self-update failed", slog.String("error", err.Error()))
		}
	default:
		slog.Warn("Unknown command action", slog.String("action", cmd.Action))
	}
}

func getFQDN(config configuration.Config) string {
	if config.FQDN != "" {
		return config.FQDN
	}
	fqdn, _ := collectors.GetFQDN()
	return fqdn
}

func getReportIntervals(config configuration.Config) (time.Duration, time.Duration) {
	baseInterval := time.Duration(config.IntervalMinutes) * time.Minute
	jitter := time.Duration(config.JitterSeconds) * time.Second
	minInterval := baseInterval - jitter
	maxInterval := baseInterval + jitter

	slog.Debug("Interval values calculated",
		slog.Duration("base_interval", baseInterval),
		slog.Duration("jitter", jitter),
		slog.Duration("min_interval", minInterval),
		slog.Duration("max_interval", maxInterval),
	)

	if minInterval < 1*time.Minute {
		slog.Warn("Jitter is too high, setting minimum interval to 1 minute")
		minInterval = 1 * time.Minute
	}

	if maxInterval < 1*time.Minute {
		slog.Warn("Interval too low, setting maximum interval to 1 minute")
		maxInterval = 1 * time.Minute
	}

	return minInterval, maxInterval
}
