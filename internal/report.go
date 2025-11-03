package internal

import (
	collectors "cartographer-go-agent/collectors"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

func buildDataReport(config configuration.Config, collectorsList []*collectors.Collector, version string) map[string]interface{} {
	hostname, _ := os.Hostname()

	data := map[string]interface{}{
		"sys_info":      map[string]interface{}{},
		"agent_version": version,
		"hostname":      hostname,
	}

	for _, collector := range collectorsList {
		collectorData, err := collector.Collect()
		if err != nil {
			if errors.Is(err, collectors.ErrCollectorSkipped) {
				slog.Info("Skipping collector due to unsupported OS",
					slog.String("collector_name", collector.Name),
				)
				continue
			}
			slog.Error("Error collecting data",
				slog.String("collector_name", collector.Name),
				slog.String("error", err.Error()),
			)
			continue
		}

		data[collector.Name] = collectorData
	}

	return data
}

// ReportTask This function will be scheduled by the RunAgent function.
func ReportTask(config configuration.Config, collectorsList []*collectors.Collector, version string) {
	slog.Info("Starting agent report task")
	// Build the report
	data := buildDataReport(config, collectorsList, version)

	// Send the report
	SendReport(config, data)
}

// SendReport sends the report to the server
func SendReport(config configuration.Config, data map[string]interface{}) {
	url := config.URL + "/report"
	jsonValue, err := common.ToJSON(data)
	if err != nil {
		slog.Error("Error marshalling report data",
			slog.String("error", err.Error()),
		)
		return
	}

	if config.DRYRUN {
		fmt.Print(string(jsonValue))
		slog.Info("DRYRUN: Not sending report")
		return
	}

	resp, err := common.PostReport(url, jsonValue, config.Token, config.Gzip)
	if err != nil {
		slog.Error("Error posting report", slog.String("error", err.Error()))
		return
	}

	if targets, ok := resp["replay_targets"]; ok {
		targetArray, ok := targets.([]interface{})
		if !ok {
			slog.Error("Error casting replay_targets to []interface{}")
			return
		}
		for _, target := range targetArray {
			replayURL, ok := target.(string)
			if !ok {
				slog.Error("Error casting replay target to string")
				continue
			}
			slog.Info("Replay Target", slog.String("replay_url", replayURL))
			_, err := common.PostReport(replayURL+"/agent-post", jsonValue, config.Token, config.Gzip)
			if err != nil {
				slog.Error("Error posting replay report",
					slog.String("replay_url", replayURL),
					slog.String("error", err.Error()),
				)
			}
		}
	}
}
