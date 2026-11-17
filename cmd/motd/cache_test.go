package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedCacheRefreshMigratesPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "motd")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := cacheFile{path: filepath.Join(dir, "cache.json"), shared: true}
	legacy := []byte(`{"ipv4":{"text":"203.0.113.42","fetched_at":"2026-11-17T00:00:00Z"}}`)
	if err := os.WriteFile(target.path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	if got := target.read().IPv4; got != (reading{}) {
		t.Fatalf("legacy shared cache exposed IPv4: %+v", got)
	}

	for range 2 {
		if err := refresh(target, config{PublicIPv4: "full", HideUpdates: true}); err != nil {
			t.Fatal(err)
		}
		assertFileMode(t, dir, 0755)
		assertFileMode(t, target.path, 0644)
		data, err := os.ReadFile(target.path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(`"ipv4"`)) {
			t.Fatalf("shared cache contains IPv4: %s", data)
		}
	}
}

func TestPrivateCacheKeepsIPv4AndPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "motd")
	target := newCacheFile(filepath.Join(dir, "cache.json"))
	value := reading{Text: "203.0.113.42"}
	if err := target.write(cache{IPv4: value}); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, dir, 0700)
	assertFileMode(t, target.path, 0600)
	if got := target.read().IPv4; got != value {
		t.Fatalf("got %+v, want %+v", got, value)
	}
}

func TestNewSharedCacheWritesWeatherAndOmitsIPv4(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "motd")
	target := cacheFile{path: filepath.Join(dir, "cache.json"), shared: true}
	weather := reading{Text: "4°C"}
	if err := target.write(cache{Weather: weather, IPv4: reading{Text: "203.0.113.42"}}); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, dir, 0755)
	assertFileMode(t, target.path, 0644)
	data, err := os.ReadFile(target.path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"ipv4"`)) {
		t.Fatalf("shared cache contains IPv4: %s", data)
	}
	if got := target.read().Weather; got != weather {
		t.Fatalf("got %+v, want %+v", got, weather)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode of %s is %04o, want %04o", path, got, want)
	}
}
