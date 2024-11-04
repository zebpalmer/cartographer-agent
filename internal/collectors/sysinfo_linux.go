//go:build linux

package collectors

import (
	"cartographer-go-agent/configuration"
	"time"

	"github.com/zcalusic/sysinfo"
)

func SysInfoCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("sys_info", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		var si sysinfo.SysInfo
		si.GetSysInfo()
		return si, nil
	})
}
