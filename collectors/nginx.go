package collectors

import (
	"bufio"
	"cartographer-go-agent/common"
	"cartographer-go-agent/configuration"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// NginxSite represents a single server block parsed from nginx config
type NginxSite struct {
	ServerNames []string `json:"server_names"`
	ListenPorts []string `json:"listen_ports"`
	Root        string   `json:"root,omitempty"`
	SSL         bool     `json:"ssl"`
	ProxyPass   string   `json:"proxy_pass,omitempty"`
	ConfigFile  string   `json:"config_file"`
}

// NginxInfo represents the collected nginx information
type NginxInfo struct {
	Installed   bool        `json:"installed"`
	Running     bool        `json:"running"`
	Version     string      `json:"version,omitempty"`
	BinaryPath  string      `json:"binary_path,omitempty"`
	ConfigPath  string      `json:"config_path,omitempty"`
	Sites       []NginxSite `json:"sites,omitempty"`
	CollectedAt string      `json:"collected_at"`
}

// NginxCollector returns a collector that gathers nginx configuration and site info
func NginxCollector(ttl time.Duration, config *configuration.Config) *Collector {
	return NewCollector("nginx", ttl, config, func(cfg *configuration.Config) (interface{}, error) {
		if runtime.GOOS != "linux" {
			return nil, ErrCollectorSkipped
		}

		info := collectNginx()
		if !info.Installed {
			return nil, ErrCollectorSkipped
		}

		return info, nil
	})
}

func collectNginx() *NginxInfo {
	info := &NginxInfo{
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Find nginx binary
	nginxPath, err := exec.LookPath("nginx")
	if err != nil {
		return info
	}
	info.Installed = true
	info.BinaryPath = nginxPath

	// Get version
	if version, err := getNginxVersion(nginxPath); err == nil {
		info.Version = version
	}

	// Check if running
	info.Running = isNginxRunning()
	if !info.Running {
		return info
	}

	// Get config path from nginx -V
	info.ConfigPath = getNginxConfigPath(nginxPath)

	// Parse sites from config
	if info.ConfigPath != "" {
		sites, err := parseNginxSites(info.ConfigPath)
		if err != nil {
			slog.Warn("Failed to parse nginx sites", slog.String("error", err.Error()))
		} else {
			info.Sites = sites
		}
	}

	return info
}

func getNginxVersion(nginxPath string) (string, error) {
	stdout, stderr, exitCode, err := common.RunCommand(nginxPath+" -v", &common.CommandOptions{
		Timeout: 5,
	})
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to get nginx version: %w", err)
	}
	// nginx -v outputs to stderr in format: "nginx version: nginx/1.18.0"
	combined := stdout + " " + stderr
	re := regexp.MustCompile(`nginx/(\S+)`)
	if matches := re.FindStringSubmatch(combined); len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not parse nginx version")
}

func isNginxRunning() bool {
	_, _, exitCode, err := common.RunCommand("pgrep -x nginx", &common.CommandOptions{
		Timeout: 5,
	})
	return err == nil && exitCode == 0
}

func getNginxConfigPath(nginxPath string) string {
	stdout, stderr, exitCode, err := common.RunCommand(nginxPath+" -V", &common.CommandOptions{
		Timeout: 5,
	})
	if err != nil || exitCode != 0 {
		// Fall back to common default
		if _, err := os.Stat("/etc/nginx/nginx.conf"); err == nil {
			return "/etc/nginx/nginx.conf"
		}
		return ""
	}
	// nginx -V outputs to stderr
	combined := stdout + " " + stderr
	re := regexp.MustCompile(`--conf-path=(\S+)`)
	if matches := re.FindStringSubmatch(combined); len(matches) > 1 {
		return matches[1]
	}
	// Fall back to common default
	if _, err := os.Stat("/etc/nginx/nginx.conf"); err == nil {
		return "/etc/nginx/nginx.conf"
	}
	return ""
}

// parseNginxSites reads the main nginx config and any included site configs to extract server blocks
func parseNginxSites(configPath string) ([]NginxSite, error) {
	// Collect all config files to parse (main config + includes)
	configFiles, err := resolveNginxIncludes(configPath)
	if err != nil {
		return nil, err
	}

	var sites []NginxSite
	for _, cfgFile := range configFiles {
		fileSites, err := parseServerBlocks(cfgFile)
		if err != nil {
			slog.Debug("Failed to parse nginx config file",
				slog.String("file", cfgFile),
				slog.String("error", err.Error()),
			)
			continue
		}
		sites = append(sites, fileSites...)
	}

	return sites, nil
}

// resolveNginxIncludes reads the main config and finds include directives to build
// a list of all config files that may contain server blocks
func resolveNginxIncludes(mainConfig string) ([]string, error) {
	configDir := filepath.Dir(mainConfig)
	files := []string{mainConfig}

	content, err := os.ReadFile(mainConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", mainConfig, err)
	}

	includeRe := regexp.MustCompile(`(?m)^\s*include\s+([^;]+);`)
	matches := includeRe.FindAllStringSubmatch(string(content), -1)
	for _, match := range matches {
		pattern := strings.TrimSpace(match[1])
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(configDir, pattern)
		}
		globbed, err := filepath.Glob(pattern)
		if err != nil {
			slog.Debug("Failed to glob nginx include pattern",
				slog.String("pattern", pattern),
				slog.String("error", err.Error()),
			)
			continue
		}
		files = append(files, globbed...)
	}

	// Also check common site directories even if not explicitly included
	// (they might be included from a nested config)
	commonDirs := []string{
		filepath.Join(configDir, "sites-enabled"),
		filepath.Join(configDir, "conf.d"),
	}
	for _, dir := range commonDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			globbed, _ := filepath.Glob(filepath.Join(dir, "*"))
			for _, f := range globbed {
				if !contains(files, f) {
					files = append(files, f)
				}
			}
		}
	}

	return files, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseServerBlocks extracts server blocks from a single nginx config file
func parseServerBlocks(filePath string) ([]NginxSite, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sites []NginxSite
	scanner := bufio.NewScanner(file)

	var inServer bool
	var braceDepth int
	var current *NginxSite

	listenRe := regexp.MustCompile(`^\s*listen\s+(.+);`)
	serverNameRe := regexp.MustCompile(`^\s*server_name\s+(.+);`)
	rootRe := regexp.MustCompile(`^\s*root\s+(.+);`)
	proxyPassRe := regexp.MustCompile(`^\s*proxy_pass\s+(.+);`)
	sslCertRe := regexp.MustCompile(`^\s*ssl_certificate\s+`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Track brace depth
		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		// Detect server block start
		if !inServer && strings.HasPrefix(trimmed, "server") && strings.Contains(trimmed, "{") {
			inServer = true
			braceDepth = 1
			current = &NginxSite{
				ConfigFile: filePath,
			}
			// Account for any extra braces on the same line
			braceDepth += openBraces - 1 // -1 because we already counted the opening brace
			braceDepth -= closeBraces
			if braceDepth <= 0 {
				// Single-line server block (unusual but handle it)
				inServer = false
				sites = append(sites, *current)
				current = nil
			}
			continue
		}

		if inServer {
			braceDepth += openBraces
			braceDepth -= closeBraces

			if current != nil {
				// Parse listen directive
				if matches := listenRe.FindStringSubmatch(line); matches != nil {
					listen := strings.TrimSpace(matches[1])
					current.ListenPorts = append(current.ListenPorts, listen)
					if strings.Contains(listen, "ssl") {
						current.SSL = true
					}
				}

				// Parse server_name directive
				if matches := serverNameRe.FindStringSubmatch(line); matches != nil {
					names := strings.Fields(strings.TrimSpace(matches[1]))
					for _, name := range names {
						if name != "_" && name != "" {
							current.ServerNames = append(current.ServerNames, name)
						}
					}
				}

				// Parse root directive (only at server level, depth 1)
				if braceDepth == 1 {
					if matches := rootRe.FindStringSubmatch(line); matches != nil {
						current.Root = strings.TrimSpace(matches[1])
					}
				}

				// Parse proxy_pass directive
				if matches := proxyPassRe.FindStringSubmatch(line); matches != nil {
					if current.ProxyPass == "" {
						current.ProxyPass = strings.TrimSpace(matches[1])
					}
				}

				// Detect SSL from ssl_certificate directive
				if sslCertRe.MatchString(line) {
					current.SSL = true
				}
			}

			if braceDepth <= 0 {
				inServer = false
				if current != nil {
					// Only include sites that have at least a listen or server_name
					if len(current.ListenPorts) > 0 || len(current.ServerNames) > 0 {
						sites = append(sites, *current)
					}
				}
				current = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return sites, err
	}

	return sites, nil
}
