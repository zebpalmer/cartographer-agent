package collectors

import (
	"cartographer-go-agent/common"
	"errors"
	"reflect"
	"testing"
)

func TestParseAptUpdates(t *testing.T) {
	mockAptOutput := `Listing... Done
php-common/focal 2:95+ubuntu20.04.1+deb.sury.org+1 all [upgradable from: 2:94+ubuntu20.04.1+deb.sury.org+2]
php8.2-apcu/focal 5.1.24-1+ubuntu20.04.1+deb.sury.org+1 amd64 [upgradable from: 5.1.23-1+ubuntu20.04.1+deb.sury.org+1]
php8.2-bcmath/focal 8.2.24-1+ubuntu20.04.1+deb.sury.org+1 amd64 [upgradable from: 8.2.21-1+ubuntu20.04.1+deb.sury.org+1]
wazuh-agent/stable 4.9.1-1 amd64 [upgradable from: 4.9.0-1]`

	// Parse the mock APT output
	updates, err := parseAptUpdates(mockAptOutput)
	if err != nil {
		t.Fatalf("Unexpected error during parsing: %v", err)
	}

	// Define the expected parsed updates
	expected := []AptUpdateInfo{
		{
			PackageName:      "php-common",
			CurrentVersion:   "2:94+ubuntu20.04.1+deb.sury.org+2",
			CandidateVersion: "2:95+ubuntu20.04.1+deb.sury.org+1",
			IsSecurityUpdate: false,
		},
		{
			PackageName:      "php8.2-apcu",
			CurrentVersion:   "5.1.23-1+ubuntu20.04.1+deb.sury.org+1",
			CandidateVersion: "5.1.24-1+ubuntu20.04.1+deb.sury.org+1",
			IsSecurityUpdate: false,
		},
		{
			PackageName:      "php8.2-bcmath",
			CurrentVersion:   "8.2.21-1+ubuntu20.04.1+deb.sury.org+1",
			CandidateVersion: "8.2.24-1+ubuntu20.04.1+deb.sury.org+1",
			IsSecurityUpdate: false,
		},
		{
			PackageName:      "wazuh-agent",
			CurrentVersion:   "4.9.0-1",
			CandidateVersion: "4.9.1-1",
			IsSecurityUpdate: false,
		},
	}

	// Compare the expected and actual parsed output
	if !reflect.DeepEqual(updates, expected) {
		t.Errorf("Expected: %+v, got: %+v", expected, updates)
	}
}

// stubAptCommand replaces runAptCommand for one test.
func stubAptCommand(t *testing.T, stdout string, exitCode int, err error) {
	t.Helper()
	orig := runAptCommand
	runAptCommand = func(string, *common.CommandOptions) (string, string, int, error) {
		return stdout, "", exitCode, err
	}
	t.Cleanup(func() { runAptCommand = orig })
}

func TestIsSecurityUpdateNonZeroExitWithoutError(t *testing.T) {
	// Regression: a non-zero exit with a nil error used to panic on err.Error().
	stubAptCommand(t, "", 100, nil)
	if isSecurityUpdate("openssl") {
		t.Error("expected false when apt-cache policy fails")
	}
}

func TestIsSecurityUpdateCommandError(t *testing.T) {
	stubAptCommand(t, "", 1, errors.New("boom"))
	if isSecurityUpdate("openssl") {
		t.Error("expected false when apt-cache policy errors")
	}
}

func TestIsSecurityUpdateDetectsSecurityPocket(t *testing.T) {
	policy := `openssl:
  Installed: 3.0.13-0ubuntu3.4
  Candidate: 3.0.13-0ubuntu3.5
  Version table:
     3.0.13-0ubuntu3.5 500
        500 http://security.ubuntu.com/ubuntu noble-security/main amd64 Packages`
	stubAptCommand(t, policy, 0, nil)
	if !isSecurityUpdate("openssl") {
		t.Error("expected true for a candidate from the security pocket")
	}
}
