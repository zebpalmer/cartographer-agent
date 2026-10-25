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

func TestParseAptUpdatesSecurityArchives(t *testing.T) {
	// Line shapes taken from Debian 13 and Ubuntu 22.04/24.04 hosts.
	out := `Listing... Done
bsdutils/stable-updates 1:2.41.5-0+deb13u1 amd64 [upgradable from: 1:2.41-5]
openssl/trixie-security 3.5.4-1~deb13u1 amd64 [upgradable from: 3.5.1-1]
cloud-init/noble-updates,noble-security 25.2-0ubuntu1~24.04.1 all [upgradable from: 25.1.4-0ubuntu0~24.04.1]
libkrb5-3/jammy-updates,jammy-security 1.19.2-2ubuntu0.7 amd64 [upgradable from: 1.19.2-2ubuntu0.6]
curl/noble-infra-security 8.5.0-2ubuntu10.7 amd64 [upgradable from: 8.5.0-2ubuntu10.6]
ceph-common/stable 19.2.3-pve2 amd64 [upgradable from: 19.2.3-pve1]`

	updates, err := parseAptUpdates(out)
	if err != nil {
		t.Fatalf("Unexpected error during parsing: %v", err)
	}

	want := map[string]bool{
		"bsdutils":    false, // regular point-release update, not a security fix
		"openssl":     true,
		"cloud-init":  true, // published in both -updates and -security
		"libkrb5-3":   true,
		"curl":        true, // Ubuntu Pro ESM security
		"ceph-common": false,
	}
	if len(updates) != len(want) {
		t.Fatalf("expected %d updates, got %d: %+v", len(want), len(updates), updates)
	}
	for _, u := range updates {
		if got, ok := want[u.PackageName]; !ok || got != u.IsSecurityUpdate {
			t.Errorf("%s: IsSecurityUpdate=%v, want %v", u.PackageName, u.IsSecurityUpdate, want[u.PackageName])
		}
	}
	if updates[1].CandidateVersion != "3.5.4-1~deb13u1" || updates[1].CurrentVersion != "3.5.1-1" {
		t.Errorf("openssl versions parsed wrong: %+v", updates[1])
	}
}

func TestFromSecurityArchive(t *testing.T) {
	cases := map[string]bool{
		"trixie-security":              true,
		"jammy-updates,jammy-security": true,
		"noble-apps-security":          true,
		"stable":                       false,
		"focal":                        false,
		"stable-updates":               false,
		"security-tools":               false, // a third-party archive name, not a -security pocket
	}
	for archives, want := range cases {
		if got := fromSecurityArchive(archives); got != want {
			t.Errorf("fromSecurityArchive(%q) = %v, want %v", archives, got, want)
		}
	}
}
