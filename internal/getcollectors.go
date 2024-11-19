package internal

import (
	"cartographer-go-agent/configuration"
	"cartographer-go-agent/internal/collectors"
	"time"
)

// GetCollectors returns a list of collectors based on the configuration
func GetCollectors(config configuration.Config) []*collectors.Collector {
	// set TTLs to a reasonable minimum value, rather than desired update frequency
	//    otherwise actual update freq could be as high as ttl + agent report interval

	collectorsList := []*collectors.Collector{
		collectors.FQDNCollector(1*time.Minute, &config),
		collectors.UsersCollector(5*time.Minute, &config),
		collectors.SysInfoCollector(5*time.Minute, &config),
		collectors.SSHLoginEventsCollector(5*time.Minute, &config),
		collectors.AptUpdatesCollector(15*time.Minute, &config),
		collectors.DiskUsageCollector(20*time.Minute, &config),
		collectors.UUIDCollector(30*time.Minute, &config),
		collectors.NessusCollector(15*time.Minute, &config),
	}

	// loop over any desired YAML file sources in the configuration and create a collector for them
	for _, y := range config.YamlFiles {
		yamlCollector := collectors.YamlFileCollector(y.Name, y.Path, 5*time.Minute, &config)
		collectorsList = append(collectorsList, yamlCollector)
	}

	// Loop through each JSON command in the configuration and create a collector for it
	for _, jc := range config.JSONCommands {
		jsonCollector := collectors.JSONCommandCollector(jc.Name, jc.Command, jc.Timeout, 10*time.Minute, &config)
		collectorsList = append(collectorsList, jsonCollector)
	}
	return collectorsList
}
