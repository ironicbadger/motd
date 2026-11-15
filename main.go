package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const weatherURL = "https://api.open-meteo.com/v1/forecast"
const ipv4URL = "https://api.ipify.org?format=json"

type reading struct {
	Text      string    `json:"text"`
	FetchedAt time.Time `json:"fetched_at"`
}

type cache struct {
	Weather       reading `json:"weather"`
	WeatherSource string  `json:"weather_source,omitempty"`
	IPv4          reading `json:"ipv4"`
}

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: motd show | motd refresh | motd flags (use -h after a command for options)")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := flags.String("cache", filepath.Join(cacheDir, "motd", "cache.json"), "path to the local cache")
	configPath := flags.String("config", filepath.Join(configDir, "motd", "config.json"), "path to the display config")
	switch args[0] {
	case "show", "flags":
		host := flags.String("host", "", "override the displayed hostname")
		osName := flags.String("os", "", "override the displayed OS label")
		color := flags.String("color", "auto", "ANSI colors (auto, always, never)")
		ansi := flags.Bool("ansi", false, "use terminal characters even when inline images are supported")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		switch *color {
		case "auto", "always", "never":
		default:
			return fmt.Errorf("unknown color mode %q", *color)
		}
		info, _ := os.Stdout.Stat()
		isTerminal := info != nil && info.Mode()&os.ModeCharDevice != 0
		term := os.Getenv("TERM")
		useColor := *color == "always" || (*color == "auto" && isTerminal && term != "dumb" && os.Getenv("NO_COLOR") == "")
		useGraphics := useColor && isTerminal && !*ansi && (term == "xterm-ghostty" || term == "xterm-kitty")
		if args[0] == "flags" {
			showFlags(os.Stdout, useColor, useGraphics)
		} else {
			cfg, err := loadConfig(*configPath, !explicitFlag(flags, "config"))
			if err != nil {
				return err
			}
			hostname, _ := os.Hostname()
			if cfg.Hostname != "" {
				hostname = cfg.Hostname
			}
			if *host != "" {
				hostname = *host
			}
			system := detectOS()
			if *osName != "" {
				system.label = *osName
			}
			show(os.Stdout, readCache(*path), time.Now(), cfg, hostname, system, useColor, useGraphics)
		}
	case "refresh":
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		cfg, err := loadConfig(*configPath, !explicitFlag(flags, "config"))
		if err != nil {
			return err
		}
		return refresh(*path, cfg)
	default:
		return fmt.Errorf("unknown command %q, use show, refresh, or flags", args[0])
	}
	return nil
}

func explicitFlag(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		found = found || f.Name == name
	})
	return found
}

func show(out io.Writer, local cache, now time.Time, cfg config, host string, system operatingSystem, color, graphics bool) {
	var heading, place []string
	if !cfg.HideHostname && host != "" {
		heading = append(heading, host)
	}
	if !cfg.HideOS {
		heading = append(heading, system.label)
	}
	if cfg.Location != "" {
		place = append(place, cfg.Location)
	}
	if cfg.Weather != nil && local.WeatherSource == cfg.Weather.endpoint() {
		if weather := local.Weather.display(now, 6*time.Hour); weather != "" {
			place = append(place, weather)
		}
	}
	lines := []string{strings.Join(heading, "  ·  "), strings.Join(place, "  ·  ")}
	if cfg.PublicIPv4 != "hidden" {
		ip := local.IPv4
		if cfg.PublicIPv4 == "masked" {
			ip.Text = maskIPv4(ip.Text)
		}
		if value := ip.display(now, 24*time.Hour); value != "" {
			lines = append(lines, "Public IPv4  "+value)
		}
	}

	rows := artwork(out, cfg, system, color, graphics)
	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(rows) > 0 {
			fmt.Fprint(out, rows[0], "  ")
			rows = rows[1:]
		}
		fmt.Fprintln(out, line)
	}
	for _, row := range rows {
		fmt.Fprintln(out, row)
	}
}

func maskIPv4(address string) string {
	ip, err := netip.ParseAddr(address)
	if err != nil || !ip.Is4() {
		return ""
	}
	bytes := ip.As4()
	return fmt.Sprintf("%d.%d.x.x", bytes[0], bytes[1])
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

func (w weather) endpoint() string {
	return fmt.Sprintf("%s?latitude=%g&longitude=%g&current=temperature_2m,weather_code", weatherURL, *w.Latitude, *w.Longitude)
}

func refresh(path string, cfg config) error {
	client := &http.Client{Timeout: 3 * time.Second}
	local := readCache(path)
	var weatherErr, ipErr error
	var requests sync.WaitGroup
	if cfg.Weather == nil {
		local.Weather = reading{}
		local.WeatherSource = ""
	} else {
		endpoint := cfg.Weather.endpoint()
		if local.WeatherSource != endpoint {
			local.Weather = reading{}
		}
		local.WeatherSource = endpoint
		requests.Go(func() {
			var text string
			text, weatherErr = fetchWeather(client, endpoint)
			if weatherErr == nil {
				local.Weather = reading{text, time.Now()}
			}
		})
	}
	if cfg.PublicIPv4 == "hidden" {
		local.IPv4 = reading{}
	} else {
		requests.Go(func() {
			var text string
			text, ipErr = fetchIPv4(client)
			if ipErr == nil {
				local.IPv4 = reading{text, time.Now()}
			}
		})
	}
	requests.Wait()
	return errors.Join(writeCache(path, local), weatherErr, ipErr)
}

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

func fetchWeather(client *http.Client, endpoint string) (string, error) {
	var forecast struct {
		Current struct {
			Temperature *float64 `json:"temperature_2m"`
			Code        *int     `json:"weather_code"`
		} `json:"current"`
	}
	if err := fetchJSON(client, endpoint, &forecast); err != nil {
		return "", fmt.Errorf("weather: %w", err)
	}
	if forecast.Current.Temperature == nil || forecast.Current.Code == nil {
		return "", errors.New("weather: missing current conditions")
	}
	temperature := fmt.Sprintf("%.0f°C", *forecast.Current.Temperature)
	if conditions := weatherDescription(*forecast.Current.Code); conditions != "" {
		temperature += "  ·  " + conditions
	}
	return temperature, nil
}

func weatherDescription(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1, 2:
		return "Partly cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55:
		return "Drizzle"
	case 56, 57:
		return "Freezing drizzle"
	case 61, 63, 65:
		return "Rain"
	case 66, 67:
		return "Freezing rain"
	case 71, 73, 75, 77:
		return "Snow"
	case 80, 81, 82:
		return "Rain showers"
	case 85, 86:
		return "Snow showers"
	case 95, 96, 99:
		return "Thunderstorms"
	default:
		return ""
	}
}

func fetchIPv4(client *http.Client) (string, error) {
	var response struct {
		IP string `json:"ip"`
	}
	if err := fetchJSON(client, ipv4URL, &response); err != nil {
		return "", fmt.Errorf("public IPv4: %w", err)
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(response.IP))
	if err != nil || !ip.Is4() {
		return "", errors.New("public IPv4: invalid address")
	}
	return ip.String(), nil
}

func fetchJSON(client *http.Client, endpoint string, target any) error {
	response, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(target)
}
