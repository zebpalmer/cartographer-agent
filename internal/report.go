package internal

import (
	collectors "cartographer-go-agent/collectors"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
)

func buildDataReport(config configuration.Config, collectorsList []*collectors.Collector, version string) map[string]interface{} {
	hostname, _ := os.Hostname()
	fqdn := getFQDN(config)

	data := map[string]interface{}{
		"sys_info":      map[string]interface{}{},
		"agent_version": version,
		"hostname":      hostname,
		"fqdn":          fqdn,
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

// ReportTask builds a report from collectors and publishes it via NATS.
func ReportTask(config configuration.Config, collectorsList []*collectors.Collector, version string, nc *nats.Conn) {
	slog.Info("Starting agent report task")
	data := buildDataReport(config, collectorsList, version)
	SendReport(config, data, nc)
}

// SendReport publishes the report to NATS
func SendReport(config configuration.Config, data map[string]interface{}, nc *nats.Conn) {
	if config.DRYRUN {
		jsonValue, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonValue))
		slog.Info("DRYRUN: Not sending report")
		return
	}

	if err := common.PublishJSON(nc, "agent.report", data, config.Gzip); err != nil {
		slog.Error("Error publishing report", slog.String("error", err.Error()))
	}
}
