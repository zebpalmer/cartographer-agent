package internal

import (
	"bytes"
	"cartographer-go-agent/configuration"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
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

	// Step 2: Download the new binary next to the current one. Staging it in the same
	// directory keeps the final rename on one filesystem: a /tmp download could not be
	// renamed over /usr/local/bin on hosts where /tmp is a tmpfs (Debian 13 and Ubuntu
	// 26.04 by default), failing every update with "invalid cross-device link". It also
	// avoids a fixed, predictable file name in a world-writable directory.
	currentBinaryPath, err := currentExecutable()
	if err != nil {
		slog.Error("Failed to get current binary path", slog.String("error", err.Error()))
		return err
	}
	newBinaryPath, err := stagingPath(currentBinaryPath)
	if err != nil {
		slog.Error("Failed to create staging file for the new binary", slog.String("error", err.Error()))
		return err
	}
	replaced := false
	defer func() {
		if !replaced {
			_ = os.Remove(newBinaryPath)
		}
	}()

	err = downloadFile(newBinaryPath, downloadURL)
	if err != nil {
		slog.Error("Failed to download new binary", slog.String("error", err.Error()))
		return err
	}

	// Step 3: Verify the download against the release's checksums.txt
	if err := verifyChecksum(newBinaryPath, downloadURL); err != nil {
		slog.Error("Checksum verification failed", slog.String("error", err.Error()))
		return err
	}

	// Step 4: Make the downloaded file executable
	err = os.Chmod(newBinaryPath, 0755)
	if err != nil {
		slog.Error("Failed to make the downloaded binary executable", slog.String("error", err.Error()))
		return err
	}

	// Step 5: Verify that the new binary runs with "--version"
	if err := verifyBinary(newBinaryPath); err != nil {
		slog.Error("Downloaded binary failed verification", slog.String("error", err.Error()))
		return err
	}

	// Step 6: Replace the current binary (same directory, so an atomic rename)
	err = os.Rename(newBinaryPath, currentBinaryPath)
	if err != nil {
		slog.Error("Failed to replace binary", slog.String("error", err.Error()))
		return err
	}
	replaced = true
	slog.Info("Replaced the binary with the new version")

	// Step 7: Restart the daemon using systemd
	err = restartAgentViaSystemd()
	if err != nil {
		slog.Error("Failed to restart agent via systemd", slog.String("error", err.Error()))
		return err
	}
	slog.Info("Agent restarted successfully")
	return nil
}

// verifyChecksum fetches the release's checksums.txt (published alongside
// the binary) and verifies the downloaded file's SHA-256 against the
// entry for this asset. A release without a checksums.txt (published
// before the move to GoReleaser) logs a warning and skips verification;
// a fetch error, missing entry, or mismatch fails the update.
func verifyChecksum(binaryPath, downloadURL string) error {
	slash := strings.LastIndex(downloadURL, "/")
	if slash < 0 {
		return fmt.Errorf("cannot derive checksums.txt location from URL: %s", downloadURL)
	}
	checksumsURL := downloadURL[:slash+1] + "checksums.txt"
	assetName := downloadURL[slash+1:]

	resp, err := http.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("fetching checksums.txt: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode == http.StatusNotFound {
		slog.Warn("Release has no checksums.txt; skipping checksum verification", slog.String("url", checksumsURL))
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching checksums.txt: %s, status code: %d", checksumsURL, resp.StatusCode)
	}
	checksums, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading checksums.txt: %w", err)
	}

	expected, err := checksumFor(checksums, assetName)
	if err != nil {
		return err
	}
	actual, err := fileSHA256(binaryPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(expected, actual) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetName, expected, actual)
	}
	slog.Info("Checksum verified", slog.String("asset", assetName))
	return nil
}

// checksumFor finds the SHA-256 entry for asset in checksums.txt content
// (sha256sum format: "<hex>  <filename>" per line).
func checksumFor(checksums []byte, asset string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s in checksums.txt", asset)
}

// fileSHA256 returns the hex-encoded SHA-256 of the file at path.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

// archVariant returns the architecture string used in release asset names.
// Release binaries for 32-bit ARM are GOARM-specific (arm6, arm7), while
// runtime.GOARCH is just "arm" — append the GOARM level recorded in the
// binary's build settings so the updater requests the right asset.
func archVariant(goarch string, settings []debug.BuildSetting) string {
	if goarch != "arm" {
		return goarch
	}
	for _, s := range settings {
		if s.Key == "GOARM" {
			// GOARM can carry a float-ABI suffix (e.g. "7,hardfloat");
			// only the level is part of the asset name
			return goarch + strings.SplitN(s.Value, ",", 2)[0]
		}
	}
	return goarch
}

func buildDownloadURL(version string, templateURL string) (string, error) {
	arch := runtime.GOARCH
	if info, ok := debug.ReadBuildInfo(); ok {
		arch = archVariant(runtime.GOARCH, info.Settings)
	}

	// Create a map with the values to substitute
	data := map[string]string{
		"version": version,
		"osType":  runtime.GOOS,
		"arch":    arch,
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

// currentExecutable returns the resolved path of the running binary.
func currentExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// stagingPath creates an empty, uniquely named file in the same directory as
// currentBinaryPath and returns its path, so the new binary can later be renamed
// over the current one without crossing a filesystem boundary.
func stagingPath(currentBinaryPath string) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(currentBinaryPath), "."+filepath.Base(currentBinaryPath)+".update-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
