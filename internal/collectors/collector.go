package collectors

import (
	"cartographer-go-agent/configuration"
	"errors"
	"log/slog"
	"time"
)

// Collector is a struct that represents a collector
type Collector struct {
	Name       string
	CollectFn  func(config *configuration.Config) (interface{}, error)
	data       interface{}
	lastUpdate time.Time
	ttl        time.Duration
	Config     *configuration.Config
}

// NewCollector creates a new collector with the given name, ttl, configuration, and collection function
func NewCollector(name string, ttl time.Duration, config *configuration.Config, collectFn func(config *configuration.Config) (interface{}, error)) *Collector {
	return &Collector{
		Name:      name,
		ttl:       ttl,
		CollectFn: collectFn,
		Config:    config,
		data:      nil,
	}
}

// Collect collects the data from the collector
func (c *Collector) Collect() (interface{}, error) {
	if !c.lastUpdate.IsZero() && time.Since(c.lastUpdate) < c.ttl {
		slog.Debug("Using cached data for collector", slog.String("collector", c.Name))
		return c.data, nil
	}

	slog.Debug("Running collection for collector", slog.String("collector", c.Name))
	collectedData, err := c.CollectFn(c.Config)
	if err != nil {
		if errors.Is(err, ErrCollectorSkipped) {
			return nil, err
		}
		return nil, err
	}

	// Update the cache
	c.data = collectedData
	c.lastUpdate = time.Now()

	return c.data, nil
}
