package collectors

import (
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"fmt"
	"time"
)

// GetFQDN collects the Fully Qualified Domain Name of the system
func GetFQDN() (string, error) {
	// Use the internal RunCommandLegacy to get the FQDN
	output, err := common.RunCommandLegacy("/bin/hostname -f", 5) // Adjust timeout as needed
	if err != nil {
		return "", fmt.Errorf("error when getting FQDN: %v", err)
	}

	// Remove trailing newline if present
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}

	return output, nil
}

// FQDNCollector collects the Fully Qualified Domain Name of the system
func FQDNCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("fqdn", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		// Check if config has FQDN set
		if config.FQDN != "" {
			return config.FQDN, nil
		}

		// Otherwise, collect FQDN dynamically
		fqdn, err := GetFQDN()
		if err != nil {
			return nil, err
		}

		// Return the FQDN directly
		return fqdn, nil
	})
}
