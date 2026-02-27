package collectors

import (
	"cartographer-go-agent/configuration"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// CollectorStatus represents the outcome of a collector's most recent execution
type CollectorStatus struct {
	Status   string  `json:"status"`             // "ok", "error", "skipped"
	Error    string  `json:"error,omitempty"`     // error message if status is "error"
	Duration float64 `json:"duration_ms"`         // execution time in milliseconds
	Cached   bool    `json:"cached,omitempty"`    // true if result was served from cache
	LastRun  string  `json:"last_run,omitempty"`  // timestamp of last successful collection
}

// Collector is a struct that represents a collector
type Collector struct {
	Name       string
	CollectFn  func(config *configuration.Config) (interface{}, error)
	data       interface{}
	lastUpdate time.Time
	ttl        time.Duration
	Config     *configuration.Config
	LastStatus CollectorStatus
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

// Collect collects the data from the collector, with panic recovery to prevent
// a single collector from crashing the entire agent.
func (c *Collector) Collect() (data interface{}, err error) {
	start := time.Now()

	// Recover from panics in collector functions
	defer func() {
		if r := recover(); r != nil {
			elapsed := time.Since(start)
			err = fmt.Errorf("collector panicked: %v", r)
			data = nil
			c.LastStatus = CollectorStatus{
				Status:   "error",
				Error:    err.Error(),
				Duration: float64(elapsed.Milliseconds()),
			}
			slog.Error("Collector panicked",
				slog.String("collector", c.Name),
				slog.String("error", err.Error()),
			)
		}
	}()

	if !c.lastUpdate.IsZero() && time.Since(c.lastUpdate) < c.ttl {
		slog.Debug("Using cached data for collector", slog.String("collector", c.Name))
		c.LastStatus = CollectorStatus{
			Status:  "ok",
			Cached:  true,
			LastRun: c.lastUpdate.UTC().Format(time.RFC3339),
		}
		return c.data, nil
	}

	slog.Debug("Running collection for collector", slog.String("collector", c.Name))
	collectedData, err := c.CollectFn(c.Config)
	elapsed := time.Since(start)

	if err != nil {
		if errors.Is(err, ErrCollectorSkipped) {
			c.LastStatus = CollectorStatus{
				Status:   "skipped",
				Duration: float64(elapsed.Milliseconds()),
			}
			return nil, err
		}
		c.LastStatus = CollectorStatus{
			Status:   "error",
			Error:    err.Error(),
			Duration: float64(elapsed.Milliseconds()),
		}
		return nil, err
	}

	// Update the cache
	c.data = collectedData
	c.lastUpdate = time.Now()

	c.LastStatus = CollectorStatus{
		Status:   "ok",
		Duration: float64(elapsed.Milliseconds()),
		LastRun:  c.lastUpdate.UTC().Format(time.RFC3339),
	}

	return c.data, nil
}
