package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	IPv4          reading       `json:"ipv4,omitzero"`
}

type cacheFile struct {
	path   string
	shared bool
}

func newCacheFile(path string) cacheFile {
	return cacheFile{path: path, shared: runtime.GOOS == "linux" && filepath.Clean(path) == systemCache}
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

func (c cacheFile) read() cache {
	var local cache
	file, err := os.Open(c.path)
	if err == nil {
		defer file.Close()
		if json.NewDecoder(io.LimitReader(file, 4096)).Decode(&local) == nil {
			if c.shared {
				local.IPv4 = reading{}
			}
			return local
		}
	}
	return cache{}
}

// write replaces the cache atomically so show can read it during refresh.
func (c cacheFile) write(local cache) error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".motd-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if c.shared {
		local.IPv4 = reading{}
	}
	err = json.NewEncoder(file).Encode(local)
	if err == nil && c.shared {
		err = file.Chmod(0644)
	}
	if err := errors.Join(err, file.Close()); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), c.path); err != nil || !c.shared {
		return err
	}

	// Open directories created by earlier releases only after publishing sanitized data.
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.IsDir() && info.Mode().Perm() == 0700 {
		return os.Chmod(dir, 0755)
	}
	return nil
}
