package internal

import (
	collectors2 "cartographer-go-agent/collectors"
	"cartographer-go-agent/configuration"
	"time"
)

// GetCollectors returns a list of collectors based on the configuration
func GetCollectors(config configuration.Config) []*collectors2.Collector {
	// set TTLs to a reasonable minimum value, rather than desired update frequency
	//    otherwise actual update freq could be as high as ttl + agent report interval

	collectorsList := []*collectors2.Collector{
		collectors2.FQDNCollector(1*time.Minute, &config),
		collectors2.UsersCollector(5*time.Minute, &config),
		collectors2.SysInfoCollector(5*time.Minute, &config),
		collectors2.SSHLoginEventsCollector(5*time.Minute, &config),
		collectors2.AptUpdatesCollector(15*time.Minute, &config),
		collectors2.DiskUsageCollector(20*time.Minute, &config),
		collectors2.UUIDCollector(30*time.Minute, &config),
		collectors2.NessusCollector(15*time.Minute, &config),
	}

	// loop over any desired YAML file sources in the configuration and create a collector for them
	for _, y := range config.YamlFiles {
		yamlCollector := collectors2.YamlFileCollector(y.Name, y.Path, 5*time.Minute, &config)
		collectorsList = append(collectorsList, yamlCollector)
	}

	// Loop through each JSON command in the configuration and create a collector for it
	for _, jc := range config.JSONCommands {
		jsonCollector := collectors2.JSONCommandCollector(jc.Name, jc.Command, jc.Timeout, 10*time.Minute, &config)
		collectorsList = append(collectorsList, jsonCollector)
	}
	return collectorsList
}
