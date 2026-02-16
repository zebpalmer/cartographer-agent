package internal

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

type heartbeat struct {
	FQDN         string `json:"fqdn"`
	AgentVersion string `json:"agent_version"`
	Timestamp    string `json:"timestamp"`
	IP           string `json:"ip,omitempty"`
}

// HeartbeatTask publishes a heartbeat to NATS
func HeartbeatTask(config configuration.Config, version string, nc *nats.Conn) {
	fqdn := getFQDN(config)

	hb := heartbeat{
		FQDN:         fqdn,
		AgentVersion: version,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		IP:           common.GetOutboundIP(),
	}

	if config.DRYRUN {
		jsonValue, _ := json.MarshalIndent(hb, "", "  ")
		fmt.Println("DRYRUN heartbeat:")
		fmt.Println(string(jsonValue))
		return
	}

	slog.Info("Sending heartbeat...")
	if err := common.PublishJSON(nc, "agent.heartbeat", hb, false); err != nil {
		slog.Error("Failed to publish heartbeat", slog.String("error", err.Error()))
	}
}
