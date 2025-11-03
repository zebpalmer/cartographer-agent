package collectors

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"fmt"
	"time"
)

// UUIDCollector collects a unique UUID for the system
func UUIDCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("agent_uuid", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		// Retrieve the UUID from the file or create a new one if it doesn't exist
		uuid, err := common.GetOrCreateUUID()
		if err != nil {
			return nil, fmt.Errorf("error when getting or creating UUID: %v", err)
		}

		// Return the UUID directly
		return uuid, nil
	})
}
