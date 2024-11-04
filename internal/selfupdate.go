package internal

import (
	"bytes"
	"cartographer-go-agent/configuration"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"
)

// SelfUpdate handles downloading a new version of the agent and restarting it.
func SelfUpdate(version string, config configuration.Config) error {
	// Step 1: Build the URL for the new release
	downloadURL, err := buildDownloadURL(version, config.ReleaseURL)
	if err != nil {
		slog.Error("Failed to build download URL", slog.String("error", err.Error()))
		return err
	}
	slog.Info("Downloading new version", slog.String("version", version), slog.String("url", downloadURL))

	// Step 2: Download the new binary
	newBinaryPath := "/tmp/cartographer-agent" // Temporary path to download new binary
	err = downloadFile(newBinaryPath, downloadURL)
	if err != nil {
		slog.Error("Failed to download new binary", slog.String("error", err.Error()))
		return err
	}

	// Step 3: Make the downloaded file executable
	err = os.Chmod(newBinaryPath, 0755)
	if err != nil {
		slog.Error("Failed to make the downloaded binary executable", slog.String("error", err.Error()))
		return err
	}

	// Step 4: Verify that the new binary runs with "--version"
	if err := verifyBinary(newBinaryPath); err != nil {
		slog.Error("Downloaded binary failed verification", slog.String("error", err.Error()))
		return err
	}

	// Step 5: Replace the current binary
	currentBinaryPath, err := os.Executable() // Path to the currently running binary
	if err != nil {
		slog.Error("Failed to get current binary path", slog.String("error", err.Error()))
		return err
	}
	err = os.Rename(newBinaryPath, currentBinaryPath) // Overwrite the current binary
	if err != nil {
		slog.Error("Failed to replace binary", slog.String("error", err.Error()))
		return err
	}
	slog.Info("Replaced the binary with the new version")

	// Step 6: Restart the daemon using systemd
	err = restartAgentViaSystemd()
	if err != nil {
		slog.Error("Failed to restart agent via systemd", slog.String("error", err.Error()))
		return err
	}
	slog.Info("Agent restarted successfully")
	return nil
}

// verifyBinary tests the downloaded binary by running it with "--version"
func verifyBinary(binaryPath string) error {
	slog.Info("Verifying downloaded binary", slog.String("binaryPath", binaryPath))

	// Run the binary with "--version" to check if it's functional
	cmd := exec.Command(binaryPath, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("Verification failed", slog.String("stdout", stdout.String()), slog.String("stderr", stderr.String()))
		return fmt.Errorf("binary verification failed: %w", err)
	}

	slog.Info("Binary verified successfully", slog.String("output", stdout.String()))
	return nil
}

func buildDownloadURL(version string, templateURL string) (string, error) {
	// Create a map with the values to substitute
	data := map[string]string{
		"version": version,
		"osType":  runtime.GOOS,
		"arch":    runtime.GOARCH,
	}

	// Parse and execute the template string
	tmpl, err := template.New("url").Parse(templateURL)
	if err != nil {
		return "", fmt.Errorf("error parsing template URL: %w", err)
	}

	var builder strings.Builder
	err = tmpl.Execute(&builder, data)
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return builder.String(), err
}

// downloadFile downloads a file from the given URL to the specified path.
func downloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {

		}
	}(out)

	// Get the data from the URL
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	// Check if the download was successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s, status code: %d", url, resp.StatusCode)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// restartAgentViaSystemd triggers a restart of the current agent via systemd.
func restartAgentViaSystemd() error {
	slog.Debug("Restarting agent via systemd")

	// Capture stdout and stderr for debugging purposes
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("systemctl", "restart", "cartographer") // Correct service name
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("Failed to restart agent via systemd",
			slog.String("error", err.Error()),
			slog.String("stdout", stdout.String()),
			slog.String("stderr", stderr.String()),
		)
		return fmt.Errorf("failed to restart agent via systemd: %w", err)
	}

	slog.Info("Agent restarted successfully via systemd")
	return nil
}
