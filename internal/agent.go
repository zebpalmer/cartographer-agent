package internal

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"cartographer-go-agent/internal/collectors"
	"log/slog"
	"os"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// RunAgent is the main entry point for the agent
func RunAgent(config configuration.Config, collectorsList []*collectors.Collector, version string) {
	// Create a new scheduler
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		slog.Error("Error creating scheduler", slog.String("error", err.Error()))
		return // Don't proceed if scheduler creation fails
	}

	// Send a heartbeat immediately
	HeartbeatTask(config, version)
	// skew the first ReportTask by random time between 0 and 60 seconds
	common.RandomSleep(0, 60)
	ReportTask(config, collectorsList, version)

	if !config.Daemonize {
		slog.Warn("Non-daemon mode, exiting after sending report")
		os.Exit(0)
	} else {
		slog.Info("Starting agent in daemon mode")
	}

	// Schedule the heartbeat task
	_, err = scheduler.NewJob(
		gocron.DurationRandomJob(55*time.Second, 65*time.Second), // Random time between 55s and 65s
		gocron.NewTask(func() {
			HeartbeatTask(config, version)
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
			ReportTask(config, collectorsList, version)
		}),
		gocron.WithSingletonMode(gocron.LimitModeReschedule), // never run more than one instance of the job
	)
	if err != nil {
		slog.Error("Error scheduling report job", slog.String("error", err.Error()))
	}

	// Start the scheduler asynchronously
	scheduler.Start()

	// Keep the agent running indefinitely
	select {}
}

func getReportIntervals(config configuration.Config) (time.Duration, time.Duration) {
	// Define the base interval, and set jitter as -1 minute to +1 minute around the base interval
	baseInterval := time.Duration(config.IntervalMinutes) * time.Minute
	jitter := time.Duration(config.JitterSeconds) * time.Second
	minInterval := baseInterval - jitter
	maxInterval := baseInterval + jitter

	// Log the variables directly as key-value pairs
	slog.Debug("Interval values calculated",
		slog.Duration("base_interval", baseInterval),
		slog.Duration("jitter", jitter),
		slog.Duration("min_interval", minInterval),
		slog.Duration("max_interval", maxInterval),
	)

	// Ensure minimum interval isn't too small
	if minInterval < 1*time.Minute {
		slog.Warn("Jitter is too high, setting minimum interval to 1 minute")
		minInterval = 1 * time.Minute // Corrected to set minimum to 1 minute
	}

	// Ensure maximum interval isn't too small
	if maxInterval < 1*time.Minute {
		slog.Warn("Interval too low, setting maximum interval to 1 minute")
		maxInterval = 1 * time.Minute
	}

	return minInterval, maxInterval
}
