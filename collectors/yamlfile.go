package collectors

import (
	"cartographer-go-agent/configuration"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// YamlFileCollector creates a collector that processes a given YAML file
func YamlFileCollector(name string, path string, ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector(name, ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		// Load the YAML file
		filename, _ := filepath.Abs(path)
		yamlFile, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		// Unmarshal the YAML file into a map
		parsedData := map[string]interface{}{}
		err = yaml.Unmarshal(yamlFile, &parsedData)
		if err != nil {
			return nil, err
		}

		return parsedData, nil
	})
}
