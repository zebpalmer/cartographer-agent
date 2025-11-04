package collectors

import (
	"bufio"
	"bytes"
	"cartographer-go-agent/configuration"
	"log/slog"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DiskUsageInfo struct to hold details about each disk mount point
type DiskUsageInfo struct {
	Filesystem      string `json:"filesystem"`
	Type            string `json:"type"`
	MountPoint      string `json:"mount_point"`
	Total           string `json:"total"`
	Used            string `json:"used"`
	Available       string `json:"available"`
	UsagePercentage int    `json:"usage_percentage"` // Store as an integer, no "%"
}

// DiskUsageCollector returns a collector that gathers information about disk usage
func DiskUsageCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("disk_usage", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		if runtime.GOOS != "linux" {
			return nil, ErrCollectorSkipped
		}

		// Run the `df -hT` command to gather disk usage information
		cmd := exec.Command("df", "-hT")
		var cmdOut bytes.Buffer
		cmd.Stdout = &cmdOut
		err := cmd.Run()
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(&cmdOut)
		var diskUsages []DiskUsageInfo

		// Skip the header line
		scanner.Scan()

		// Define filesystems to skip
		skipFileSystems := map[string]bool{
			"tmpfs":    true,
			"devtmpfs": true,
			"overlay":  true,
			"aufs":     true,
			"squashfs": true,
			"ramfs":    true,
		}

		// Parse each line of the df output
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) < 7 {
				continue // Ensure there are enough fields to avoid index errors
			}

			fsType := fields[1]
			if _, skip := skipFileSystems[fsType]; skip {
				continue
			}

			// Parse usage percentage as an integer
			usageStr := fields[5]
			usagePercentage, err := strconv.Atoi(strings.TrimSuffix(usageStr, "%"))
			if err != nil {
				slog.Error("Error parsing usage percentage", slog.Any("error", err))
				continue
			}

			// Create a DiskUsageInfo object for each entry
			diskUsage := DiskUsageInfo{
				Filesystem:      fields[0],
				Type:            fsType,
				Total:           fields[2],
				Used:            fields[3],
				Available:       fields[4],
				UsagePercentage: usagePercentage,
				MountPoint:      fields[6],
			}

			diskUsages = append(diskUsages, diskUsage)
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return diskUsages, nil
	})
}
