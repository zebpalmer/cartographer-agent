package collectors

import (
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

const sampleOutput = `Running: Yes
Linked to: cloud.tenable.com:443
Link status: Connected to cloud.tenable.com:443
Last successful connection with controller: 253 secs ago
Proxy: None
Plugin set: 202411181519
Scanning: No (0 jobs pending, 2 smart scan configs) 
Scans run today: 2 of 10 limit
Last scanned: 1732029688
Last connect: 1732041680
Last connection attempt: 1732041680`

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	executable bool
}

func (m *mockFileInfo) Name() string { return "nessuscli" }
func (m *mockFileInfo) Size() int64  { return 1000 }
func (m *mockFileInfo) Mode() os.FileMode {
	if m.executable {
		return 0755
	}
	return 0644
}
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// mockFileSystem implements fileSystem interface for testing
type mockFileSystem struct {
	exists     bool
	executable bool
}

func (fs *mockFileSystem) Stat(name string) (os.FileInfo, error) {
	if !fs.exists {
		return nil, os.ErrNotExist
	}
	return &mockFileInfo{executable: fs.executable}, nil
}

func (fs *mockFileSystem) LookPath(file string) (string, error) {
	if !fs.exists {
		return "", exec.ErrNotFound
	}
	return "/mock/path/nessuscli", nil
}

func TestFindNessuscli(t *testing.T) {
	tests := []struct {
		name       string
		exists     bool
		executable bool
		wantErr    bool
	}{
		{
			name:       "path exists and is executable",
			exists:     true,
			executable: true,
			wantErr:    false,
		},
		{
			name:       "path exists but not executable",
			exists:     true,
			executable: false,
			wantErr:    true,
		},
		{
			name:       "path does not exist",
			exists:     false,
			executable: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := &mockFileSystem{
				exists:     tt.exists,
				executable: tt.executable,
			}

			_, err := findNessuscliWithFS(mockFS)
			if (err != nil) != tt.wantErr {
				t.Errorf("findNessuscli() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseNessusOutput(t *testing.T) {
	output := []byte(sampleOutput)
	status, err := parseNessusOutput(output, "/test/path")

	if err != nil {
		t.Errorf("parseNessusOutput returned unexpected error: %v", err)
	}

	expected := &NessusStatus{
		Running:               true,
		LinkedTo:              "cloud.tenable.com:443",
		LinkStatus:            "Connected to cloud.tenable.com:443",
		LastSuccessConnection: 253,
		Proxy:                 "None",
		PluginSet:             "202411181519",
		Scanning:              false,
		JobsPending:           0,
		SmartScanConfigs:      2,
		ScansCompleted:        2,
		ScansLimit:            10,
		LastScanned:           1732029688,
		LastConnect:           1732041680,
		LastConnectionAttempt: 1732041680,
		AgentPath:             "/test/path",
	}

	if !reflect.DeepEqual(status, expected) {
		t.Errorf("parseNessusOutput = %+v, want %+v", status, expected)
	}
}

func TestParseNessusOutputVariations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *NessusStatus
		wantErr  bool
	}{
		{
			name: "scanning yes",
			input: `Running: Yes
Scanning: Yes`,
			expected: &NessusStatus{
				Running:  true,
				Scanning: true,
			},
			wantErr: false,
		},
		{
			name: "scanning with jobs",
			input: `Running: No
Scanning: No (5 jobs pending, 3 smart scan configs)`,
			expected: &NessusStatus{
				Running:          false,
				Scanning:         false,
				JobsPending:      5,
				SmartScanConfigs: 3,
			},
			wantErr: false,
		},
		{
			name: "different scan limits",
			input: `Running: Yes
Scans run today: 5 of 20 limit`,
			expected: &NessusStatus{
				Running:        true,
				ScansCompleted: 5,
				ScansLimit:     20,
			},
			wantErr: false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: &NessusStatus{},
			wantErr:  false,
		},
		{
			name: "malformed scanning line",
			input: `Running: Yes
Scanning: Invalid Format`,
			expected: &NessusStatus{
				Running: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := parseNessusOutput([]byte(tt.input), "/test/path")

			if (err != nil) != tt.wantErr {
				t.Errorf("parseNessusOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Only compare the fields we care about for each test
			if tt.expected.Running != status.Running {
				t.Errorf("Running = %v, want %v", status.Running, tt.expected.Running)
			}
			if tt.expected.Scanning != status.Scanning {
				t.Errorf("Scanning = %v, want %v", status.Scanning, tt.expected.Scanning)
			}
			if tt.expected.JobsPending != status.JobsPending {
				t.Errorf("JobsPending = %v, want %v", status.JobsPending, tt.expected.JobsPending)
			}
			if tt.expected.ScansCompleted != status.ScansCompleted {
				t.Errorf("ScansCompleted = %v, want %v", status.ScansCompleted, tt.expected.ScansCompleted)
			}
			if tt.expected.ScansLimit != status.ScansLimit {
				t.Errorf("ScansLimit = %v, want %v", status.ScansLimit, tt.expected.ScansLimit)
			}
		})
	}
}

func TestNessusCollector(t *testing.T) {
	collector := NessusCollector(5*time.Minute, nil)
	if collector == nil {
		t.Error("NessusCollector returned nil")
	}

	if collector.Name != "nessus_status" {
		t.Errorf("collector.Name = %v, want %v", collector.Name, "nessus_status")
	}

	if collector.ttl != 5*time.Minute {
		t.Errorf("collector.ttl = %v, want %v", collector.ttl, 5*time.Minute)
	}
}
