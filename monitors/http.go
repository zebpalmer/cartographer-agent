package monitors

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// checkHTTP performs an HTTP/HTTPS check
func checkHTTP(monitor Monitor) (MonitorStatus, string) {
	// Create HTTP client with custom settings
	client := &http.Client{
		Timeout: time.Duration(monitor.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !*monitor.FollowRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// Configure TLS verification
	if strings.HasPrefix(strings.ToLower(monitor.URL), "https://") {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !*monitor.VerifyTLS,
			},
		}
		client.Transport = transport
	}

	// Create request
	req, err := http.NewRequest(monitor.Method, monitor.URL, strings.NewReader(monitor.Body))
	if err != nil {
		return StatusUnknown, fmt.Sprintf("Failed to create request: %v", err)
	}

	// Add custom headers
	for key, value := range monitor.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return StatusCritical, fmt.Sprintf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return StatusUnknown, fmt.Sprintf("Failed to read response body: %v", err)
	}
	body := string(bodyBytes)

	// Check status code
	validStatus := false
	for _, code := range monitor.Validations.StatusCodes {
		if resp.StatusCode == code {
			validStatus = true
			break
		}
	}

	if !validStatus {
		return StatusCritical, fmt.Sprintf("Unexpected status code: %d (expected %v)", resp.StatusCode, monitor.Validations.StatusCodes)
	}

	// Check body content if specified
	if monitor.Validations.BodyContains != "" {
		if !strings.Contains(body, monitor.Validations.BodyContains) {
			return StatusCritical, fmt.Sprintf("Response body does not contain expected string: '%s'", monitor.Validations.BodyContains)
		}
	}

	// Check body regex if specified
	if monitor.Validations.BodyRegex != "" {
		matched, err := regexp.MatchString(monitor.Validations.BodyRegex, body)
		if err != nil {
			return StatusUnknown, fmt.Sprintf("Invalid regex pattern: %v", err)
		}
		if !matched {
			return StatusCritical, fmt.Sprintf("Response body does not match regex: '%s'", monitor.Validations.BodyRegex)
		}
	}

	// Check certificate expiry if HTTPS and verification enabled
	if strings.HasPrefix(strings.ToLower(monitor.URL), "https://") && *monitor.VerifyTLS && monitor.Validations.CertExpiryDays > 0 {
		if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
			cert := resp.TLS.PeerCertificates[0]
			daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

			if daysUntilExpiry < 0 {
				return StatusCritical, fmt.Sprintf("Certificate expired %d days ago", -daysUntilExpiry)
			}

			if daysUntilExpiry <= monitor.Validations.CertExpiryDays {
				return StatusWarning, fmt.Sprintf("Certificate expires in %d days", daysUntilExpiry)
			}
		}
	}

	// All checks passed
	return StatusOK, fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
}
