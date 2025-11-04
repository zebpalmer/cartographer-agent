//go:build !linux

package collectors

import (
	"cartographer-go-agent/configuration"
	"time"
)

// SysInfoCollector collects system information
func SysInfoCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("sys_info", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		return nil, ErrCollectorSkipped
	})
}
