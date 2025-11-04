package internal

import (
	"cartographer-go-agent/collectors"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"log/slog"
)

type heartbeat struct {
	Version string `json:"version"`
	FQDN    string `json:"fqdn"`
}

// HeartbeatTask sends a heartbeat to the server and checks for pending updates
func HeartbeatTask(config configuration.Config, version string) {
	url := config.URL + "/heartbeat"
	// Get the FQDN (prefer config, fallback to dynamic)
	var fqdn string
	if config.FQDN != "" {
		fqdn = config.FQDN
	} else {
		fqdn, _ = collectors.GetFQDN()
	}

	hb := heartbeat{
		Version: version,
		FQDN:    fqdn,
	}
	jsonValue, err := json.Marshal(hb)
	if err != nil {
		slog.Error("Failed to marshal heartbeat JSON", slog.String("error", err.Error()))
		return
	}

	slog.Info("Sending heartbeat...")
	resp, err := common.PostReport(url, jsonValue, config.Token, config.Gzip)
	if err != nil {
		slog.Error("Failed to post heartbeat to server",
			slog.String("error", err.Error()),
		)
		return
	}

	// Log the JSON response from the server
	slog.Debug("Heartbeat response", slog.Any("response", resp))

	// Check if the response contains an "agent" key
	agentInfo, agentExists := resp["agent"].(map[string]interface{})
	if !agentExists {
		slog.Debug("No agent update information in the response")
		return
	}

	// Check if the "agent" key contains a "target_version"
	targetVersion, versionExists := agentInfo["target_version"].(string)
	if !versionExists {
		slog.Debug("No target_version key found in the agent information")
		return
	}

	// Compare the current version with the target version
	if targetVersion != version {
		slog.Info("New agent version available", slog.String("target_version", targetVersion))

		// Trigger self-update
		err := SelfUpdate(targetVersion, config)
		if err != nil {
			slog.Error("Self-update failed", slog.String("error", err.Error()))
		}
	} else {
		slog.Debug("Agent is already up-to-date")
	}
}
