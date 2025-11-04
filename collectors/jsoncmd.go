package collectors

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"encoding/json"
	"time"
)

// JSONCommandCollector creates a collector that runs a given command and returns JSON data
func JSONCommandCollector(name string, command string, timeout int, ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector(name, ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		// Run the command
		results, err := common.RunCommandLegacy(command, timeout)
		if err != nil {
			return nil, err
		}

		// Parse the JSON output from the command
		var jsonData interface{}
		err = json.Unmarshal([]byte(results), &jsonData)
		if err != nil {
			return nil, err
		}

		return jsonData, nil
	})
}
