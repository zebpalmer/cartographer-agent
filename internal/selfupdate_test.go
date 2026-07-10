package internal

import (
	"fmt"
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
