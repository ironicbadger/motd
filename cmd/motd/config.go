package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type config struct {
	Artwork      string   `json:"artwork" toml:"artwork"`
	Country      string   `json:"country" toml:"country"`
	Hostname     string   `json:"hostname" toml:"hostname"`
	HideHostname bool     `json:"hide_hostname" toml:"hide_hostname"`
	HideUpdates  bool     `json:"hide_updates" toml:"hide_updates"`
	HideKernel   bool     `json:"hide_kernel" toml:"hide_kernel"`
	HideCPUTemp  bool     `json:"hide_cpu_temp" toml:"hide_cpu_temp"`
	HideOS       bool     `json:"hide_os" toml:"hide_os"`
	HideLocation bool     `json:"hide_location" toml:"hide_location"`
	HideWeather  bool     `json:"hide_weather" toml:"hide_weather"`
	HideLoad     bool     `json:"hide_load" toml:"hide_load"`
	HideUptime   bool     `json:"hide_uptime" toml:"hide_uptime"`
	Location     string   `json:"location" toml:"location"`
	PublicIPv4   string   `json:"public_ipv4" toml:"public_ipv4"`
	Weather      *weather `json:"weather" toml:"weather"`
}

type weather struct {
	Latitude  *float64 `json:"latitude" toml:"latitude"`
	Longitude *float64 `json:"longitude" toml:"longitude"`
}

// loadConfig overlays a file on the defaults. The implicit TOML path also supports legacy JSON.
func loadConfig(path string, allowMissing bool) (config, error) {
	cfg := config{Artwork: "os", PublicIPv4: "hidden", HideKernel: true}
	file, err := os.Open(path)
	if allowMissing && errors.Is(err, os.ErrNotExist) && filepath.Ext(path) == ".toml" {
		path = strings.TrimSuffix(path, ".toml") + ".json"
		file, err = os.Open(path)
	}
	if allowMissing && errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer file.Close()
	if err := decodeConfig(io.LimitReader(file, 64<<10), filepath.Ext(path), &cfg); err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	cfg.Country = strings.ToLower(cfg.Country)
	return cfg, cfg.validate()
}

func decodeConfig(input io.Reader, extension string, cfg *config) error {
	if extension == ".json" {
		decoder := json.NewDecoder(input)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(cfg); err != nil {
			return err
		}
		if decoder.Decode(new(any)) != io.EOF {
			return errors.New("expected a single JSON object")
		}
		return nil
	}
	metadata, err := toml.NewDecoder(input).Decode(cfg)
	if err != nil {
		return err
	}
	if keys := metadata.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("unknown keys %v", keys)
	}
	return nil
}

func (cfg config) validate() error {
	switch cfg.Artwork {
	case "os", "none":
	case "flag", "both":
		if !validCountry(cfg.Country) {
			return errors.New("config: flag artwork requires country ca, us, or gb")
		}
	default:
		return errors.New("config: artwork must be os, flag, both, or none")
	}
	switch cfg.PublicIPv4 {
	case "hidden", "masked", "full":
	default:
		return errors.New("config: public_ipv4 must be hidden, masked, or full")
	}
	if w := cfg.Weather; w != nil && (!validCoordinate(w.Latitude, 90) || !validCoordinate(w.Longitude, 180)) {
		return errors.New("config: weather requires valid latitude and longitude")
	}
	return nil
}

func validCoordinate(value *float64, limit float64) bool {
	return value != nil && *value >= -limit && *value <= limit
}
