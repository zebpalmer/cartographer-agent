package collectors

import (
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
