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

// ProxyPassEntry represents a proxy_pass directive with its location path
type ProxyPassEntry struct {
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
}

// NginxSite represents a single server block parsed from nginx config
type NginxSite struct {
	ServerNames []string         `json:"server_names"`
	ListenPorts []string         `json:"listen_ports"`
	Root        string           `json:"root,omitempty"`
	SSL         bool             `json:"ssl"`
	ProxyPasses []ProxyPassEntry `json:"proxy_passes,omitempty"`
	ConfigFile  string           `json:"config_file"`
	setVars     map[string]string // unexported: used during parsing for $variable resolution
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
	nginxPath := findNginxBinary()
	if nginxPath == "" {
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

func findNginxBinary() string {
	if path, err := exec.LookPath("nginx"); err == nil {
		return path
	}
	for _, path := range []string{
		"/usr/sbin/nginx",
		"/usr/local/sbin/nginx",
		"/usr/local/nginx/sbin/nginx",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
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
	nginxConfigDir := filepath.Dir(configPath)

	// Collect all config files to parse (main config + includes)
	configFiles, err := resolveNginxIncludes(configPath)
	if err != nil {
		return nil, err
	}

	// First pass: collect all upstream block definitions
	upstreams := make(map[string][]string)
	for _, cfgFile := range configFiles {
		for name, servers := range parseUpstreamBlocks(cfgFile) {
			upstreams[name] = servers
		}
	}

	// Second pass: parse server blocks (with include inlining for nested includes)
	var sites []NginxSite
	for _, cfgFile := range configFiles {
		fileSites, err := parseServerBlocks(cfgFile, nginxConfigDir)
		if err != nil {
			slog.Debug("Failed to parse nginx config file",
				slog.String("file", cfgFile),
				slog.String("error", err.Error()),
			)
			continue
		}
		sites = append(sites, fileSites...)
	}

	// Resolve proxy_pass upstream references and set variables
	for i := range sites {
		for j := range sites[i].ProxyPasses {
			upstream := sites[i].ProxyPasses[j].Upstream
			upstream = resolveSetVars(upstream, sites[i].setVars)
			upstream = resolveProxyPass(upstream, upstreams)
			sites[i].ProxyPasses[j].Upstream = upstream
		}
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

// parseUpstreamBlocks extracts upstream block definitions from a config file.
// Returns a map of upstream name → list of server addresses.
func parseUpstreamBlocks(filePath string) map[string][]string {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	upstreams := make(map[string][]string)
	scanner := bufio.NewScanner(file)

	upstreamStartRe := regexp.MustCompile(`^\s*upstream\s+(\S+)\s*\{`)
	serverRe := regexp.MustCompile(`^\s*server\s+(\S+)`)

	var inUpstream bool
	var currentName string
	var braceDepth int

	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !inUpstream {
			if matches := upstreamStartRe.FindStringSubmatch(trimmed); matches != nil {
				inUpstream = true
				currentName = matches[1]
				braceDepth = 1
				continue
			}
		} else {
			braceDepth += strings.Count(trimmed, "{")
			braceDepth -= strings.Count(trimmed, "}")

			if matches := serverRe.FindStringSubmatch(trimmed); matches != nil {
				addr := strings.TrimRight(matches[1], ";")
				upstreams[currentName] = append(upstreams[currentName], addr)
			}

			if braceDepth <= 0 {
				inUpstream = false
				currentName = ""
			}
		}
	}

	return upstreams
}

// resolveProxyPass resolves a proxy_pass value against known upstream blocks.
// Handles: $variable, http://upstream_name, http://upstream_name/path
func resolveProxyPass(proxyPass string, upstreams map[string][]string) string {
	if proxyPass == "" || len(upstreams) == 0 {
		return proxyPass
	}

	// Handle $variable references
	if strings.HasPrefix(proxyPass, "$") {
		name := strings.TrimPrefix(proxyPass, "$")
		if servers, ok := upstreams[name]; ok && len(servers) > 0 {
			return "http://" + strings.Join(servers, ", ")
		}
		return proxyPass
	}

	// Handle http(s)://upstream_name or http(s)://upstream_name/path
	for _, scheme := range []string{"http://", "https://"} {
		if !strings.HasPrefix(proxyPass, scheme) {
			continue
		}
		rest := strings.TrimPrefix(proxyPass, scheme)
		parts := strings.SplitN(rest, "/", 2)
		hostPart := parts[0]

		if servers, ok := upstreams[hostPart]; ok && len(servers) > 0 {
			path := ""
			if len(parts) > 1 {
				path = "/" + parts[1]
			}
			if len(servers) == 1 {
				return scheme + servers[0] + path
			}
			return scheme + strings.Join(servers, ", ") + path
		}
	}

	return proxyPass
}

// resolveSetVars replaces $variable references in a string using set directive values.
// Longer variable names are matched first to avoid prefix overlap (e.g. $backend_host vs $backend).
func resolveSetVars(value string, vars map[string]string) string {
	if len(vars) == 0 || !strings.Contains(value, "$") {
		return value
	}
	// Sort keys by length descending so longer names match first
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		value = strings.ReplaceAll(value, "$"+k, vars[k])
	}
	return value
}

// resolveIncludeToLines reads all files matching a glob pattern and returns their lines.
// Relative patterns are resolved against nginxConfigDir (the nginx prefix directory,
// e.g. /etc/nginx/), not the including file's directory.
func resolveIncludeToLines(pattern string, nginxConfigDir string) []string {
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(nginxConfigDir, pattern)
	}
	globbed, err := filepath.Glob(pattern)
	if err != nil || len(globbed) == 0 {
		return nil
	}
	var lines []string
	for _, f := range globbed {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines = append(lines, strings.Split(string(content), "\n")...)
	}
	return lines
}

// parseServerBlocks extracts server blocks from a single nginx config file.
// nginxConfigDir is the nginx prefix directory (e.g. /etc/nginx/) used to resolve
// relative include paths found inside server blocks.
func parseServerBlocks(filePath string, nginxConfigDir string) ([]NginxSite, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var sites []NginxSite

	var inServer bool
	var braceDepth int
	var current *NginxSite

	// locationStack tracks nested location paths with their brace depths
	type locEntry struct {
		path  string
		depth int
	}
	var locationStack []locEntry

	listenRe := regexp.MustCompile(`^\s*listen\s+(.+);`)
	serverNameRe := regexp.MustCompile(`^\s*server_name\s+(.+);`)
	rootRe := regexp.MustCompile(`^\s*root\s+(.+);`)
	proxyPassRe := regexp.MustCompile(`^\s*proxy_pass\s+(.+);`)
	sslCertRe := regexp.MustCompile(`^\s*ssl_certificate\s+`)
	locationRe := regexp.MustCompile(`^\s*location\s+(?:=\s+|\^~\s+)?(/\S+)\s*\{`)
	includeRe := regexp.MustCompile(`^\s*include\s+([^;]+);`)
	setRe := regexp.MustCompile(`^\s*set\s+\$(\w+)\s+"?([^";]+)"?\s*;`)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip comments and empty lines
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		// Detect server block start
		if !inServer && strings.HasPrefix(trimmed, "server") && strings.Contains(trimmed, "{") {
			inServer = true
			braceDepth = 1
			current = &NginxSite{
				ConfigFile: filePath,
				setVars:    make(map[string]string),
			}
			locationStack = nil
			braceDepth += openBraces - 1
			braceDepth -= closeBraces
			if braceDepth <= 0 {
				inServer = false
				sites = append(sites, *current)
				current = nil
			}
			continue
		}

		if !inServer {
			continue
		}

		// Inline includes found inside server blocks
		if current != nil {
			if matches := includeRe.FindStringSubmatch(trimmed); matches != nil {
				pattern := strings.TrimSpace(matches[1])
				included := resolveIncludeToLines(pattern, nginxConfigDir)
				if len(included) > 0 {
					// Insert included lines at the current position
					tail := make([]string, len(lines)-i-1)
					copy(tail, lines[i+1:])
					lines = append(lines[:i+1], included...)
					lines = append(lines, tail...)
				}
				continue
			}
		}

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

			// Track set directives for variable resolution
			if matches := setRe.FindStringSubmatch(trimmed); matches != nil {
				current.setVars[matches[1]] = matches[2]
			}

			// Track location blocks
			if matches := locationRe.FindStringSubmatch(trimmed); matches != nil {
				locationStack = append(locationStack, locEntry{
					path:  matches[1],
					depth: braceDepth,
				})
			}

			// Pop location stack on closing braces
			for len(locationStack) > 0 && braceDepth < locationStack[len(locationStack)-1].depth {
				locationStack = locationStack[:len(locationStack)-1]
			}

			// Parse proxy_pass with location context
			if matches := proxyPassRe.FindStringSubmatch(line); matches != nil {
				path := "/"
				if len(locationStack) > 0 {
					path = locationStack[len(locationStack)-1].path
				}
				current.ProxyPasses = append(current.ProxyPasses, ProxyPassEntry{
					Path:     path,
					Upstream: strings.TrimSpace(matches[1]),
				})
			}

			// Detect SSL from ssl_certificate directive
			if sslCertRe.MatchString(line) {
				current.SSL = true
			}
		}

		if braceDepth <= 0 {
			inServer = false
			if current != nil {
				if len(current.ListenPorts) > 0 || len(current.ServerNames) > 0 {
					sites = append(sites, *current)
				}
			}
			current = nil
			locationStack = nil
		}
	}

	return sites, nil
}
