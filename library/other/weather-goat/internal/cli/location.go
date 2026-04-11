package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"
)

// resolveLocation determines the lat/lon/name from args or config.
// Priority: explicit lat/lon flags > location name arg > saved config.
func resolveLocation(cfg *config.Config, latFlag, lonFlag float64, latChanged, lonChanged bool, args []string) (float64, float64, string, error) {
	// 1. Explicit lat/lon flags
	if latChanged && lonChanged {
		name := cfg.LocationName
		if name == "" {
			name = fmt.Sprintf("%.4f, %.4f", latFlag, lonFlag)
		}
		return latFlag, lonFlag, name, nil
	}

	// 2. Location name argument — resolve via geocoding
	if len(args) > 0 && args[0] != "" {
		lat, lon, name, err := geocodeLookup(args[0])
		if err != nil {
			return 0, 0, "", fmt.Errorf("resolving location %q: %w", args[0], err)
		}
		return lat, lon, name, nil
	}

	// 3. Saved config
	if cfg.Latitude != 0 || cfg.Longitude != 0 {
		return cfg.Latitude, cfg.Longitude, cfg.LocationName, nil
	}

	return 0, 0, "", fmt.Errorf("no location specified.\nSet your location: weather-goat-pp-cli config set-location \"City Name\"\nOr pass a location: weather-goat-pp-cli <command> \"City Name\"")
}

// geocodeLookup resolves a location name to lat/lon via Open-Meteo geocoding API.
func geocodeLookup(name string) (float64, float64, string, error) {
	u := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en", url.QueryEscape(name))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return 0, 0, "", fmt.Errorf("geocoding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", fmt.Errorf("reading geocoding response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return 0, 0, "", fmt.Errorf("geocoding API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, "", fmt.Errorf("parsing geocoding response: %w", err)
	}

	if len(result.Results) == 0 {
		return 0, 0, "", fmt.Errorf("no results found for %q", name)
	}

	r := result.Results[0]
	displayName := r.Name
	if r.Admin1 != "" {
		displayName += ", " + r.Admin1
	}
	if r.Country != "" {
		displayName += ", " + r.Country
	}

	return r.Latitude, r.Longitude, displayName, nil
}
