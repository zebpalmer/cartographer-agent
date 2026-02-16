package common

import (
	"log/slog"
	"net"
)

// GetOutboundIP returns the primary outbound IP address of this machine.
// It uses a UDP dial to 8.8.8.8:80 which does not send any traffic,
// but lets the OS select the appropriate source address.
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		slog.Warn("Failed to detect outbound IP", slog.String("error", err.Error()))
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
