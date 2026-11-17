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
			show(&out, local, now, config{PublicIPv4: tc.mode}, "gateway", operatingSystem{label: "Linux"}, systemStats{}, false, false)
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
	if err != nil || cfg.PublicIPv4 != "hidden" || !cfg.HideKernel || cfg.HideUpdates {
		t.Fatalf("unexpected defaults %+v, %v", cfg, err)
	}
	target := newCacheFile(file)
	if err := target.write(cache{IPv4: reading{"203.0.113.42", time.Now()}}); err != nil {
		t.Fatal(err)
	}
	if err := refresh(target, cfg); err != nil {
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

func TestTOMLDisplayToggles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	const content = `
artwork = "both"
country = "ca"
location = "Toronto, CA"
hide_hostname = true
hide_os = true
hide_kernel = true
hide_cpu_temp = true
hide_location = true
hide_weather = true
hide_load = true
hide_uptime = true
[weather]
latitude = 43.65
longitude = -79.38
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	local := cache{Weather: reading{"4°C", now}, WeatherSource: cfg.Weather.endpoint(), IPv4: reading{"203.0.113.42", now}}
	stats := systemStats{load: "2.44 2.65 2.44", uptime: "20:13:46"}
	var out bytes.Buffer
	show(&out, local, now, cfg, "gateway", operatingSystem{label: "Fedora", logo: "fedora"}, stats, false, false)
	if out.Len() != 0 {
		t.Fatalf("hidden fields produced %q", out.String())
	}
	cfg.HideLoad, cfg.HideUptime = false, false
	show(&out, local, now, cfg, "gateway", operatingSystem{label: "Fedora", logo: "fedora"}, stats, false, false)
	const want = "\nLoad average:  2.44 2.65 2.44\nUptime:        20:13:46\n\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestCPUReadingAndDisplay(t *testing.T) {
	sysfs := t.TempDir()
	if got := linuxCPUTemperature(sysfs); got != "" {
		t.Fatalf("missing sensors produced %q", got)
	}
	for i, device := range []struct{ name, temp string }{{"coretemp", "46000"}, {"nvme", "62000"}} {
		dir := filepath.Join(sysfs, "class", "hwmon", []string{"hwmon0", "hwmon1"}[i])
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]string{"name": device.name, "temp1_input": device.temp} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	stats := systemStats{kernel: "6.15.0", load: "0.10 0.20 0.30", cpuTemp: linuxCPUTemperature(sysfs)}
	if stats.cpuTemp != "46°C" {
		t.Fatalf("unexpected CPU temperature %q", stats.cpuTemp)
	}
	var out bytes.Buffer
	show(&out, cache{}, time.Now(), config{Location: "Toronto, CA"}, "gateway", operatingSystem{label: "Fedora"}, stats, false, false)
	want := "\nOS:            Fedora\nKernel:        6.15.0\nHostname:      gateway\nLocation:      Toronto, CA\nLoad average:  0.10 0.20 0.30  ·  CPU 46°C\n\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestUpdateRow(t *testing.T) {
	now := time.Now()
	local := cache{Updates: &updateStatus{
		ReadAt: now, LastCheck: now.Add(-time.Hour), NextCheck: now.Add(time.Minute), CheckState: "success",
	}}
	cfg := config{HideKernel: true}
	var out bytes.Buffer
	show(&out, local, now, cfg, "gateway", operatingSystem{label: "Fedora"}, systemStats{kernel: "7.2.11"}, false, false)
	result := out.String()
	if !strings.Contains(result, "None staged · Checked ") || !strings.Contains(result, " · Next ") || strings.Contains(result, "Kernel:") {
		t.Fatalf("unexpected update row: %q", result)
	}
	local.Updates.Staged = "44.20261115.3.1"
	local.Updates.AutoReboot = true
	if got := local.Updates.display(now); got != "44.20261115.3.1 staged  ·  Auto-reboot pending" {
		t.Fatalf("unexpected staged status: %q", got)
	}
	if got := local.Updates.display(now.Add(2 * time.Hour)); got != "" {
		t.Fatalf("expired status rendered: %q", got)
	}
}

func TestSystemAndExplicitPaths(t *testing.T) {
	configFile, cacheFile, err := resolvePaths("/etc/motd.toml", "")
	if err != nil || configFile != "/etc/motd.toml" || cacheFile != "/var/cache/motd/cache.json" {
		t.Fatalf("unexpected system paths: %q, %q, %v", configFile, cacheFile, err)
	}
	configFile, cacheFile, err = resolvePaths("/tmp/custom.toml", "/tmp/custom.json")
	if err != nil || configFile != "/tmp/custom.toml" || cacheFile != "/tmp/custom.json" {
		t.Fatalf("unexpected explicit paths: %q, %q, %v", configFile, cacheFile, err)
	}
}

func TestBannerPaddingIncludesInlineArtwork(t *testing.T) {
	var out bytes.Buffer
	show(&out, cache{}, time.Now(), config{Artwork: "flag", Country: "ca"}, "gateway", operatingSystem{}, systemStats{}, true, true)
	got := out.String()
	if !strings.HasPrefix(got, "\n\x1b_Ga=T,") || !strings.HasSuffix(got, "\n\n") || strings.HasSuffix(got, "\n\n\n") {
		t.Fatal("expected one blank line before the inline flag and after the banner")
	}
}

func TestAPTUpdateDisplay(t *testing.T) {
	now := time.Now()
	zero, one, two := 0, 1, 2
	for _, tc := range []struct {
		available *int
		indexDate time.Time
		want      string
	}{
		{nil, time.Time{}, "APT status unavailable"},
		{&zero, now, "No updates"},
		{&one, now, "1 package update available"},
		{&two, now.Add(-72 * time.Hour), "2 package updates available · APT indexes 3d old"},
	} {
		status := updateStatus{ReadAt: now, APT: &aptUpdateStatus{tc.available, tc.indexDate}}
		if got := status.display(now); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
