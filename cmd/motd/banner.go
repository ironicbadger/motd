package main

import (
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"
)

const fieldSeparator = "  ·  "

type bannerField struct{ label, value string }

func show(out io.Writer, local cache, now time.Time, cfg config, host string, system operatingSystem, stats systemStats, color, graphics bool) {
	fields := bannerFields(local, now, cfg, host, system, stats)
	rows, width := artwork(out, cfg, system, color, graphics)
	start := max(0, len(rows)-len(fields)) / 2
	labelWidth := 0
	for _, field := range fields {
		labelWidth = max(labelWidth, len(field.label))
	}
	padding := strings.Repeat(" ", width)
	for i := range max(len(rows), start+len(fields)) {
		if i < len(rows) {
			fmt.Fprint(out, rows[i])
		} else {
			fmt.Fprint(out, padding)
		}
		line := i - start
		if line >= 0 && line < len(fields) {
			field := fields[line]
			if width > 0 {
				fmt.Fprint(out, "  ")
			}
			fmt.Fprintf(out, "%-*s  ", labelWidth, field.label)
			if field.label == "OS:" && cfg.Artwork == "both" && graphics && system.logo != "" {
				placeImage(out, system.logo, 2, 1)
				fmt.Fprint(out, "   ")
			}
			fmt.Fprint(out, field.value)
		}
		fmt.Fprintln(out)
	}
}

func bannerFields(local cache, now time.Time, cfg config, host string, system operatingSystem, stats systemStats) []bannerField {
	var fields []bannerField
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, bannerField{label + ":", value})
		}
	}
	if !cfg.HideOS {
		add("OS", system.label)
	}
	if !cfg.HideUpdates {
		add("Updates", local.Updates.display(now))
	}
	if !cfg.HideKernel {
		add("Kernel", stats.kernel)
	}
	if !cfg.HideHostname {
		add("Hostname", host)
	}
	var place []string
	if !cfg.HideLocation && cfg.Location != "" {
		place = append(place, cfg.Location)
	}
	if endpoint := cfg.weatherEndpoint(); endpoint != "" && endpoint == local.WeatherSource {
		if weather := local.Weather.display(now, 6*time.Hour); weather != "" {
			place = append(place, weather)
		}
	}
	add("Location", strings.Join(place, fieldSeparator))

	var load string
	if !cfg.HideLoad {
		load = stats.load
	}
	if !cfg.HideCPUTemp && stats.cpuTemp != "" {
		if load != "" {
			load += fieldSeparator + "CPU " + stats.cpuTemp
		} else {
			add("CPU temp", stats.cpuTemp)
		}
	}
	add("Load average", load)
	if !cfg.HideUptime {
		add("Uptime", stats.uptime)
	}
	if cfg.PublicIPv4 != "hidden" {
		ip := local.IPv4
		if cfg.PublicIPv4 == "masked" {
			ip.Text = maskIPv4(ip.Text)
		}
		add("Public IPv4", ip.display(now, 24*time.Hour))
	}

	return fields
}

func maskIPv4(address string) string {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() {
		return ""
	}
	bytes := ip.As4()
	return fmt.Sprintf("%d.%d.x.x", bytes[0], bytes[1])
}
