package internal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
)

func TestArchVariant(t *testing.T) {
	tests := []struct {
		name     string
		goarch   string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name:   "amd64 passes through",
			goarch: "amd64",
			settings: []debug.BuildSetting{
				{Key: "GOARCH", Value: "amd64"},
			},
			want: "amd64",
		},
		{
			name:   "arm64 passes through",
			goarch: "arm64",
			want:   "arm64",
		},
		{
			name:   "arm with GOARM 6",
			goarch: "arm",
			settings: []debug.BuildSetting{
				{Key: "GOARCH", Value: "arm"},
				{Key: "GOARM", Value: "6"},
			},
			want: "arm6",
		},
		{
			name:   "arm with GOARM 7",
			goarch: "arm",
			settings: []debug.BuildSetting{
				{Key: "GOARM", Value: "7"},
			},
			want: "arm7",
		},
		{
			name:   "arm with float-ABI suffix",
			goarch: "arm",
			settings: []debug.BuildSetting{
				{Key: "GOARM", Value: "7,hardfloat"},
			},
			want: "arm7",
		},
		{
			name:   "arm without GOARM setting falls back to plain arch",
			goarch: "arm",
			want:   "arm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := archVariant(tt.goarch, tt.settings)
			if got != tt.want {
				t.Errorf("archVariant(%q) = %q, want %q", tt.goarch, got, tt.want)
			}
		})
	}
}

func TestBuildDownloadURL(t *testing.T) {
	templateURL := "https://example.com/releases/v{{.version}}/cartographer-agent_{{.osType}}-{{.arch}}"

	got, err := buildDownloadURL("1.5.0", templateURL)
	if err != nil {
		t.Fatal(err)
	}

	// On non-arm test hosts the arch is just runtime.GOARCH
	want := fmt.Sprintf("https://example.com/releases/v1.5.0/cartographer-agent_%s-%s", runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Errorf("buildDownloadURL() = %q, want %q", got, want)
	}
}

func TestBuildDownloadURLBadTemplate(t *testing.T) {
	if _, err := buildDownloadURL("1.5.0", "https://example.com/{{.version"); err == nil {
		t.Error("expected error for malformed template, got nil")
	}
}

func TestChecksumFor(t *testing.T) {
	checksums := []byte(
		"aaa111  cartographer-agent_linux-amd64\n" +
			"bbb222  cartographer-agent_darwin-arm64\n")

	if got, err := checksumFor(checksums, "cartographer-agent_darwin-arm64"); err != nil || got != "bbb222" {
		t.Errorf("checksumFor() = (%q, %v), want (%q, nil)", got, err, "bbb222")
	}
	if _, err := checksumFor(checksums, "cartographer-agent_linux-arm6"); err == nil {
		t.Error("expected error for missing entry, got nil")
	}
}

func TestVerifyChecksum(t *testing.T) {
	// The binary under test, with its real SHA-256
	// (printf 'binary contents' | shasum -a 256).
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "cartographer-agent")
	if err := os.WriteFile(binaryPath, []byte("binary contents"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodSum := "58dd882b7907e7d10da755323a848544f42119b2e599801d794a32d2c23e4051"

	serve := func(checksums string, status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1.5.0/checksums.txt" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(status)
			fmt.Fprint(w, checksums)
		}))
	}

	t.Run("matching checksum passes", func(t *testing.T) {
		srv := serve(goodSum+"  cartographer-agent_linux-amd64\n", http.StatusOK)
		defer srv.Close()
		if err := verifyChecksum(binaryPath, srv.URL+"/v1.5.0/cartographer-agent_linux-amd64"); err != nil {
			t.Errorf("expected pass, got %v", err)
		}
	})

	t.Run("mismatch fails", func(t *testing.T) {
		srv := serve("deadbeef  cartographer-agent_linux-amd64\n", http.StatusOK)
		defer srv.Close()
		if err := verifyChecksum(binaryPath, srv.URL+"/v1.5.0/cartographer-agent_linux-amd64"); err == nil {
			t.Error("expected checksum mismatch error, got nil")
		}
	})

	t.Run("missing entry fails", func(t *testing.T) {
		srv := serve(goodSum+"  some-other-asset\n", http.StatusOK)
		defer srv.Close()
		if err := verifyChecksum(binaryPath, srv.URL+"/v1.5.0/cartographer-agent_linux-amd64"); err == nil {
			t.Error("expected missing-entry error, got nil")
		}
	})

	t.Run("404 checksums.txt skips verification (pre-goreleaser releases)", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		defer srv.Close()
		if err := verifyChecksum(binaryPath, srv.URL+"/v1.0.0/cartographer-agent_linux-amd64"); err != nil {
			t.Errorf("expected 404 to be tolerated, got %v", err)
		}
	})
}
