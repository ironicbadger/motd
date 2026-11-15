package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type reading struct {
	Text      string    `json:"text"`
	FetchedAt time.Time `json:"fetched_at"`
}

type cache struct {
	Updates       *updateStatus `json:"updates,omitempty"`
	Weather       reading       `json:"weather"`
	WeatherSource string        `json:"weather_source,omitempty"`
	IPv4          reading       `json:"ipv4"`
}

func (r reading) display(now time.Time, maxAge time.Duration) string {
	age := now.Sub(r.FetchedAt)
	if r.Text == "" || r.FetchedAt.IsZero() || age >= maxAge {
		return ""
	}
	if age >= time.Hour {
		return fmt.Sprintf("%s (%dh ago)", r.Text, int(age.Hours()))
	}
	return r.Text
}

func readCache(path string) cache {
	var local cache
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		if json.NewDecoder(io.LimitReader(file, 4096)).Decode(&local) == nil {
			return local
		}
	}
	return cache{}
}

// writeCache replaces the cache atomically so show can read it during refresh.
func writeCache(path string, local cache) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".motd-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writeErr := json.NewEncoder(file).Encode(local)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
