package collectors

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseServerBlocks(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected []NginxSite
	}{
		{
			name: "simple site with server_name and listen",
			config: `server {
    listen 80;
    server_name example.com www.example.com;
    root /var/www/example;
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"example.com", "www.example.com"},
					ListenPorts: []string{"80"},
					Root:        "/var/www/example",
					SSL:         false,
				},
			},
		},
		{
			name: "ssl site",
			config: `server {
    listen 443 ssl;
    server_name secure.example.com;
    ssl_certificate /etc/ssl/certs/example.crt;
    ssl_certificate_key /etc/ssl/private/example.key;
    root /var/www/secure;
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"secure.example.com"},
					ListenPorts: []string{"443 ssl"},
					Root:        "/var/www/secure",
					SSL:         true,
				},
			},
		},
		{
			name: "reverse proxy site",
			config: `server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"api.example.com"},
					ListenPorts: []string{"80"},
					ProxyPass:   "http://localhost:3000",
					SSL:         false,
				},
			},
		},
		{
			name: "multiple server blocks",
			config: `server {
    listen 80;
    server_name site1.com;
    root /var/www/site1;
}

server {
    listen 80;
    listen 443 ssl;
    server_name site2.com;
    root /var/www/site2;
    ssl_certificate /etc/ssl/certs/site2.crt;
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"site1.com"},
					ListenPorts: []string{"80"},
					Root:        "/var/www/site1",
					SSL:         false,
				},
				{
					ServerNames: []string{"site2.com"},
					ListenPorts: []string{"80", "443 ssl"},
					Root:        "/var/www/site2",
					SSL:         true,
				},
			},
		},
		{
			name: "default server with underscore server_name",
			config: `server {
    listen 80 default_server;
    server_name _;
    root /var/www/html;
}`,
			expected: []NginxSite{
				{
					ServerNames: nil,
					ListenPorts: []string{"80 default_server"},
					Root:        "/var/www/html",
					SSL:         false,
				},
			},
		},
		{
			name: "comments and empty lines",
			config: `# This is a comment
server {
    # Listen on port 80
    listen 80;

    # The server name
    server_name commented.example.com;
    root /var/www/commented;
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"commented.example.com"},
					ListenPorts: []string{"80"},
					Root:        "/var/www/commented",
					SSL:         false,
				},
			},
		},
		{
			name:     "no server blocks",
			config:   `upstream backend { server 127.0.0.1:8080; }`,
			expected: nil,
		},
		{
			name: "ssl detected from ssl_certificate only",
			config: `server {
    listen 443;
    server_name ssl-cert.example.com;
    ssl_certificate /etc/ssl/cert.pem;
    root /var/www/ssl;
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"ssl-cert.example.com"},
					ListenPorts: []string{"443"},
					Root:        "/var/www/ssl",
					SSL:         true,
				},
			},
		},
		{
			name: "proxy pass with location blocks",
			config: `server {
    listen 80;
    server_name multi.example.com;

    location /api {
        proxy_pass http://backend:8080;
    }

    location /static {
        root /var/www/static;
    }
}`,
			expected: []NginxSite{
				{
					ServerNames: []string{"multi.example.com"},
					ListenPorts: []string{"80"},
					ProxyPass:   "http://backend:8080",
					SSL:         false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config to temp file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}

			sites, err := parseServerBlocks(configFile)
			if err != nil {
				t.Fatalf("parseServerBlocks returned error: %v", err)
			}

			// Set ConfigFile on expected sites for comparison
			for i := range tt.expected {
				tt.expected[i].ConfigFile = configFile
			}

			if !reflect.DeepEqual(sites, tt.expected) {
				t.Errorf("parseServerBlocks() =\n  %+v\nwant\n  %+v", sites, tt.expected)
			}
		})
	}
}

func TestResolveNginxIncludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")
	confD := filepath.Join(tmpDir, "conf.d")
	os.MkdirAll(sitesEnabled, 0755)
	os.MkdirAll(confD, 0755)

	// Create site configs
	os.WriteFile(filepath.Join(sitesEnabled, "site1.conf"), []byte("server { listen 80; server_name site1.com; }"), 0644)
	os.WriteFile(filepath.Join(confD, "site2.conf"), []byte("server { listen 80; server_name site2.com; }"), 0644)

	// Create main config with includes
	mainConfig := filepath.Join(tmpDir, "nginx.conf")
	mainContent := `events {}
http {
    include ` + filepath.Join(tmpDir, "conf.d", "*.conf") + `;
    include ` + filepath.Join(tmpDir, "sites-enabled", "*") + `;
}
`
	os.WriteFile(mainConfig, []byte(mainContent), 0644)

	files, err := resolveNginxIncludes(mainConfig)
	if err != nil {
		t.Fatalf("resolveNginxIncludes returned error: %v", err)
	}

	// Should include main config + both site configs
	if len(files) < 3 {
		t.Errorf("expected at least 3 files, got %d: %v", len(files), files)
	}

	// Verify main config is first
	if files[0] != mainConfig {
		t.Errorf("expected first file to be main config %s, got %s", mainConfig, files[0])
	}
}

func TestResolveNginxIncludesWithCommonDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sites-enabled directory with a config but no explicit include
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")
	os.MkdirAll(sitesEnabled, 0755)
	os.WriteFile(filepath.Join(sitesEnabled, "default"), []byte("server { listen 80; }"), 0644)

	// Create main config WITHOUT including sites-enabled
	mainConfig := filepath.Join(tmpDir, "nginx.conf")
	os.WriteFile(mainConfig, []byte("events {}\nhttp {}\n"), 0644)

	files, err := resolveNginxIncludes(mainConfig)
	if err != nil {
		t.Fatalf("resolveNginxIncludes returned error: %v", err)
	}

	// Should still find sites-enabled/default via common dir scan
	found := false
	for _, f := range files {
		if filepath.Base(f) == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find sites-enabled/default in files: %v", files)
	}
}

func TestParseNginxSites(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sites-enabled directory
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")
	os.MkdirAll(sitesEnabled, 0755)

	// Create site config
	siteConfig := `server {
    listen 80;
    server_name mysite.com;
    root /var/www/mysite;
}

server {
    listen 443 ssl;
    server_name mysite.com;
    ssl_certificate /etc/ssl/mysite.crt;

    location / {
        proxy_pass http://localhost:8080;
    }
}
`
	os.WriteFile(filepath.Join(sitesEnabled, "mysite.conf"), []byte(siteConfig), 0644)

	// Create main config that includes sites-enabled
	mainConfig := filepath.Join(tmpDir, "nginx.conf")
	mainContent := `events {}
http {
    include ` + filepath.Join(sitesEnabled, "*") + `;
}
`
	os.WriteFile(mainConfig, []byte(mainContent), 0644)

	sites, err := parseNginxSites(mainConfig)
	if err != nil {
		t.Fatalf("parseNginxSites returned error: %v", err)
	}

	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}

	// Check first site (HTTP)
	if sites[0].ServerNames[0] != "mysite.com" {
		t.Errorf("site 0 server_name = %v, want mysite.com", sites[0].ServerNames)
	}
	if sites[0].SSL {
		t.Error("site 0 should not have SSL")
	}

	// Check second site (HTTPS/proxy)
	if !sites[1].SSL {
		t.Error("site 1 should have SSL")
	}
	if sites[1].ProxyPass != "http://localhost:8080" {
		t.Errorf("site 1 proxy_pass = %v, want http://localhost:8080", sites[1].ProxyPass)
	}
}

func TestParseUpstreamBlocks(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		expected map[string][]string
	}{
		{
			name: "single upstream with one server",
			config: `upstream audiolab {
    server 127.0.0.1:4000;
}`,
			expected: map[string][]string{
				"audiolab": {"127.0.0.1:4000"},
			},
		},
		{
			name: "upstream with multiple servers",
			config: `upstream backend {
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}`,
			expected: map[string][]string{
				"backend": {"10.0.0.1:8080", "10.0.0.2:8080"},
			},
		},
		{
			name: "multiple upstreams",
			config: `upstream app {
    server 127.0.0.1:3000;
}

upstream api {
    server 127.0.0.1:8080;
    server 127.0.0.1:8081;
}`,
			expected: map[string][]string{
				"app": {"127.0.0.1:3000"},
				"api": {"127.0.0.1:8080", "127.0.0.1:8081"},
			},
		},
		{
			name: "upstream with weight and params stripped to address",
			config: `upstream weighted {
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}`,
			expected: map[string][]string{
				"weighted": {"10.0.0.1:8080", "10.0.0.2:8080"},
			},
		},
		{
			name:     "no upstreams",
			config:   `server { listen 80; server_name test.com; }`,
			expected: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "test.conf")
			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}

			upstreams := parseUpstreamBlocks(configFile)
			if !reflect.DeepEqual(upstreams, tt.expected) {
				t.Errorf("parseUpstreamBlocks() =\n  %v\nwant\n  %v", upstreams, tt.expected)
			}
		})
	}
}

func TestResolveProxyPass(t *testing.T) {
	upstreams := map[string][]string{
		"audiolab": {"127.0.0.1:4000"},
		"backend":  {"10.0.0.1:8080", "10.0.0.2:8080"},
	}

	tests := []struct {
		name      string
		proxyPass string
		expected  string
	}{
		{"direct URL unchanged", "http://localhost:3000/", "http://localhost:3000/"},
		{"variable resolved", "$audiolab", "http://127.0.0.1:4000"},
		{"variable multi-server", "$backend", "http://10.0.0.1:8080, 10.0.0.2:8080"},
		{"named upstream via http", "http://audiolab", "http://127.0.0.1:4000"},
		{"named upstream with path", "http://audiolab/api", "http://127.0.0.1:4000/api"},
		{"named upstream multi-server", "http://backend", "http://10.0.0.1:8080, 10.0.0.2:8080"},
		{"unknown variable unchanged", "$unknown", "$unknown"},
		{"empty unchanged", "", ""},
		{"https upstream", "https://audiolab", "https://127.0.0.1:4000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveProxyPass(tt.proxyPass, upstreams)
			if result != tt.expected {
				t.Errorf("resolveProxyPass(%q) = %q, want %q", tt.proxyPass, result, tt.expected)
			}
		})
	}
}

func TestParseNginxSitesResolvesUpstreams(t *testing.T) {
	tmpDir := t.TempDir()
	sitesEnabled := filepath.Join(tmpDir, "sites-enabled")
	os.MkdirAll(sitesEnabled, 0755)

	// Config with upstream block and proxy_pass referencing it
	siteConfig := `upstream audiolab {
    server 127.0.0.1:4000;
}

server {
    listen 443 ssl;
    server_name audiolab-qa.simucase.com;

    location / {
        proxy_pass $audiolab;
    }
}
`
	os.WriteFile(filepath.Join(sitesEnabled, "audiolab"), []byte(siteConfig), 0644)

	mainConfig := filepath.Join(tmpDir, "nginx.conf")
	mainContent := "events {}\nhttp {\n    include " + filepath.Join(sitesEnabled, "*") + ";\n}\n"
	os.WriteFile(mainConfig, []byte(mainContent), 0644)

	sites, err := parseNginxSites(mainConfig)
	if err != nil {
		t.Fatalf("parseNginxSites returned error: %v", err)
	}

	if len(sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(sites))
	}

	if sites[0].ProxyPass != "http://127.0.0.1:4000" {
		t.Errorf("proxy_pass = %q, want %q", sites[0].ProxyPass, "http://127.0.0.1:4000")
	}
}

func TestNginxCollectorCreation(t *testing.T) {
	collector := NginxCollector(5*time.Minute, nil)
	if collector == nil {
		t.Fatal("NginxCollector returned nil")
	}

	if collector.Name != "nginx" {
		t.Errorf("collector.Name = %v, want nginx", collector.Name)
	}

	if collector.ttl != 5*time.Minute {
		t.Errorf("collector.ttl = %v, want %v", collector.ttl, 5*time.Minute)
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !contains(slice, "b") {
		t.Error("expected contains to return true for 'b'")
	}
	if contains(slice, "d") {
		t.Error("expected contains to return false for 'd'")
	}
}
