//go:build !linux

package monitors

// checkSystemd is not supported on non-Linux platforms
func checkSystemd(monitor Monitor) (MonitorStatus, string) {
	return StatusUnknown, "Systemd monitoring is only supported on Linux"
}
