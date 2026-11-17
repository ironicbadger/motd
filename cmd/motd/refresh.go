package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	weatherURL = "https://api.open-meteo.com/v1/forecast"
	ipv4URL    = "https://api.ipify.org?format=json"
)

func (w weather) endpoint() string {
	return fmt.Sprintf("%s?latitude=%g&longitude=%g&current=temperature_2m,weather_code", weatherURL, *w.Latitude, *w.Longitude)
}

func (cfg config) weatherEndpoint() string {
	if cfg.HideWeather || cfg.Weather == nil {
		return ""
	}
	return cfg.Weather.endpoint()
}

// refresh keeps the last successful reading on a fetch error and clears disabled readings.
func refresh(target cacheFile, cfg config) error {
	client := &http.Client{Timeout: 3 * time.Second}
	local := target.read()
	var weatherErr, ipErr error
	var requests sync.WaitGroup
	if cfg.HideUpdates {
		local.Updates = nil
	} else {
		requests.Go(func() {
			if status := readUpdates(); status != nil {
				local.Updates = status
			}
		})
	}
	endpoint := cfg.weatherEndpoint()
	if endpoint == "" || endpoint != local.WeatherSource {
		local.Weather = reading{}
	}
	local.WeatherSource = endpoint
	if endpoint != "" {
		requests.Go(func() {
			weatherErr = local.Weather.update(fetchWeather(client, endpoint))
		})
	}
	if target.shared || cfg.PublicIPv4 == "hidden" {
		local.IPv4 = reading{}
	} else {
		requests.Go(func() {
			ipErr = local.IPv4.update(fetchIPv4(client))
		})
	}
	requests.Wait()
	return errors.Join(target.write(local), weatherErr, ipErr)
}

func (r *reading) update(text string, err error) error {
	if err == nil {
		*r = reading{Text: text, FetchedAt: time.Now()}
	}
	return err
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
		temperature += fieldSeparator + conditions
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
