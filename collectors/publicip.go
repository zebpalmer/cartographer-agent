package collectors

import (
	"cartographer-go-agent/configuration"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// PublicIPs holds the public IPv4 and IPv6 addresses of the machine.
type PublicIPs struct {
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

const (
	ipifyV4URL   = "https://api.ipify.org"
	ipifyV6URL   = "https://api6.ipify.org"
	ipifyTimeout = 10 * time.Second
)

// fetchPublicIPOnce makes a single HTTP GET request to the given URL and
// returns the response body as a trimmed string.
func fetchPublicIPOnce(url string) (string, error) {
	client := &http.Client{Timeout: ipifyTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request to %s returned status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", url, err)
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP address from %s: %q", url, ip)
	}

	return ip, nil
}

// fetchPublicIP attempts to fetch a public IP with up to 2 retries,
// waiting 3 seconds between attempts.
func fetchPublicIP(url string) (string, error) {
	const maxAttempts = 3
	const retryDelay = 3 * time.Second

	var lastErr error
	for attempt := range maxAttempts {
		ip, err := fetchPublicIPOnce(url)
		if err == nil {
			return ip, nil
		}
		lastErr = err
		if attempt < maxAttempts-1 {
			slog.Debug("Retrying public IP fetch",
				slog.String("url", url),
				slog.Int("attempt", attempt+1),
				slog.String("error", err.Error()),
			)
			time.Sleep(retryDelay)
		}
	}
	return "", lastErr
}

// collectPublicIPs fetches the public IPv4 and IPv6 addresses using ipify.org.
// Either address may be empty if the machine lacks public connectivity for that
// IP version. Returns an error only if both lookups fail.
func collectPublicIPs() (*PublicIPs, error) {
	result := &PublicIPs{}

	ipv4, err := fetchPublicIP(ipifyV4URL)
	if err != nil {
		slog.Debug("Could not determine public IPv4", slog.String("error", err.Error()))
	} else {
		result.IPv4 = ipv4
	}

	ipv6, err := fetchPublicIP(ipifyV6URL)
	if err != nil {
		slog.Debug("Could not determine public IPv6", slog.String("error", err.Error()))
	} else {
		result.IPv6 = ipv6
	}

	if result.IPv4 == "" && result.IPv6 == "" {
		return nil, fmt.Errorf("failed to determine any public IP address")
	}

	return result, nil
}

// PublicIPCollector returns a collector that fetches public IP addresses.
func PublicIPCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("public_ips", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		return collectPublicIPs()
	})
}
