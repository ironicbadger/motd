package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublicIPv4Modes(t *testing.T) {
	const address = "203.0.113.42"
	now := time.Now()
	local := cache{IPv4: reading{address, now}}
	for _, tc := range []struct{ mode, want string }{
		{"hidden", ""},
		{"masked", "203.0.x.x"},
		{"full", address},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			var out bytes.Buffer
			show(&out, local, now, config{PublicIPv4: tc.mode}, "gateway", operatingSystem{label: "Linux"}, false, false)
			got := out.String()
			if tc.want == "" && strings.Contains(got, "Public IPv4") {
				t.Fatalf("hidden address appeared in %q", got)
			}
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatalf("expected %q in %q", tc.want, got)
			}
			if tc.mode != "full" && strings.Contains(got, address) {
				t.Fatalf("raw address appeared in %q", got)
			}
		})
	}
}

func TestDefaultConfigClearsIPWithoutLookup(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.json")
	cfg, err := loadConfig(file, true)
	if err != nil || cfg.PublicIPv4 != "hidden" {
		t.Fatalf("unexpected defaults %+v, %v", cfg, err)
	}
	if err := writeCache(file, cache{IPv4: reading{"203.0.113.42", time.Now()}}); err != nil {
		t.Fatal(err)
	}
	if err := refresh(file, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil || bytes.Contains(data, []byte("203.0.113.42")) {
		t.Fatalf("IP persisted in the cache: %v", err)
	}
}

func TestOSRelease(t *testing.T) {
	for _, tc := range []struct{ content, label, logo string }{
		{"ID=fedora\nPRETTY_NAME=\"Fedora CoreOS 44.20261023.3.1 (uCore minimal)\"", "Fedora CoreOS 44.20261023.3.1 (uCore minimal)", "fedora"},
		{"ID=ubuntu\nNAME=Ubuntu\nVERSION_ID=\"24.04\"", "Ubuntu 24.04", "ubuntu"},
		{"ID=debian\nPRETTY_NAME=\"Debian GNU/Linux 13\"", "Debian GNU/Linux 13", ""},
	} {
		info := parseOSRelease(strings.NewReader(tc.content))
		if info.label != tc.label || info.logo != tc.logo {
			t.Errorf("got %+v, want %q and %q", info, tc.label, tc.logo)
		}
	}
}
