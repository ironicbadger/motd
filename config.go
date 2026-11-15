package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type config struct {
	Artwork      string   `json:"artwork"`
	Country      string   `json:"country"`
	Hostname     string   `json:"hostname"`
	HideHostname bool     `json:"hide_hostname"`
	HideOS       bool     `json:"hide_os"`
	Location     string   `json:"location"`
	PublicIPv4   string   `json:"public_ipv4"`
	Weather      *weather `json:"weather"`
}

type weather struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func loadConfig(path string, allowMissing bool) (config, error) {
	cfg := config{Artwork: "os", PublicIPv4: "hidden"}
	file, err := os.Open(path)
	if allowMissing && errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("config: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return cfg, errors.New("config: expected a single JSON object")
	}
	cfg.Country = strings.ToLower(cfg.Country)
	switch cfg.Artwork {
	case "os", "none":
	case "flag":
		if cfg.Country != "ca" && cfg.Country != "us" && cfg.Country != "gb" {
			return cfg, errors.New("config: artwork flag requires country ca, us, or gb")
		}
	default:
		return cfg, errors.New("config: artwork must be os, flag, or none")
	}
	switch cfg.PublicIPv4 {
	case "hidden", "masked", "full":
	default:
		return cfg, errors.New("config: public_ipv4 must be hidden, masked, or full")
	}
	if w := cfg.Weather; w != nil {
		if w.Latitude == nil || w.Longitude == nil || *w.Latitude < -90 || *w.Latitude > 90 || *w.Longitude < -180 || *w.Longitude > 180 {
			return cfg, errors.New("config: weather requires valid latitude and longitude")
		}
	}
	return cfg, nil
}
