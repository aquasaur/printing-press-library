package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

// runMorningBrief is called when weather-goat-pp-cli is invoked with no subcommand and no args.
func runMorningBrief(cmd *cobra.Command, flags *rootFlags) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}

	lat, lon, locName, err := resolveLocation(cfg, 0, 0, false, false, nil)
	if err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "No location configured.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Set your location: weather-goat-pp-cli config set-location \"City Name\"")
		return nil
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	// Fetch current conditions + daily forecast from Open-Meteo
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"current":          "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m,wind_direction_10m,wind_gusts_10m,uv_index",
		"daily":            "temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset,uv_index_max",
		"timezone":         "auto",
		"temperature_unit": "fahrenheit",
		"wind_speed_unit":  "mph",
		"forecast_days":    "1",
	}

	data, err := c.Get("/forecast", params)
	if err != nil {
		return classifyAPIError(err)
	}

	var forecast struct {
		Current struct {
			Temperature   float64 `json:"temperature_2m"`
			Humidity      float64 `json:"relative_humidity_2m"`
			ApparentTemp  float64 `json:"apparent_temperature"`
			Precipitation float64 `json:"precipitation"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed     float64 `json:"wind_speed_10m"`
			WindDirection float64 `json:"wind_direction_10m"`
			WindGusts     float64 `json:"wind_gusts_10m"`
			UVIndex       float64 `json:"uv_index"`
		} `json:"current"`
		Daily struct {
			TempMax    []float64 `json:"temperature_2m_max"`
			TempMin    []float64 `json:"temperature_2m_min"`
			PrecipProb []float64 `json:"precipitation_probability_max"`
			Sunrise    []string  `json:"sunrise"`
			Sunset     []string  `json:"sunset"`
			UVMax      []float64 `json:"uv_index_max"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(data, &forecast); err != nil {
		return fmt.Errorf("parsing forecast: %w", err)
	}

	// Fetch NWS alerts (best-effort — don't fail if NWS is unreachable)
	alerts, alertErr := nwsAlerts(lat, lon)

	if flags.asJSON {
		result := map[string]any{
			"location":  locName,
			"latitude":  lat,
			"longitude": lon,
			"current": map[string]any{
				"temperature":    forecast.Current.Temperature,
				"feels_like":     forecast.Current.ApparentTemp,
				"conditions":     describeWeatherCode(forecast.Current.WeatherCode),
				"weather_code":   forecast.Current.WeatherCode,
				"humidity":       forecast.Current.Humidity,
				"wind_speed":     forecast.Current.WindSpeed,
				"wind_direction": forecast.Current.WindDirection,
				"wind_gusts":     forecast.Current.WindGusts,
				"uv_index":       forecast.Current.UVIndex,
				"precipitation":  forecast.Current.Precipitation,
			},
		}
		if len(forecast.Daily.TempMax) > 0 {
			daily := map[string]any{
				"high": forecast.Daily.TempMax[0],
				"low":  forecast.Daily.TempMin[0],
			}
			if len(forecast.Daily.PrecipProb) > 0 {
				daily["precip_chance"] = forecast.Daily.PrecipProb[0]
			}
			if len(forecast.Daily.Sunrise) > 0 {
				daily["sunrise"] = forecast.Daily.Sunrise[0]
			}
			if len(forecast.Daily.Sunset) > 0 {
				daily["sunset"] = forecast.Daily.Sunset[0]
			}
			if len(forecast.Daily.UVMax) > 0 {
				daily["uv_max"] = forecast.Daily.UVMax[0]
			}
			result["today"] = daily
		}
		if alertErr == nil && len(alerts) > 0 {
			result["alerts"] = alerts
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	// Human-readable output
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "%s\n", bold(locName))
	fmt.Fprintf(w, "%s  %.0f°F (feels like %.0f°F)\n",
		describeWeatherCode(forecast.Current.WeatherCode),
		forecast.Current.Temperature,
		forecast.Current.ApparentTemp)
	fmt.Fprintf(w, "Wind: %.0f mph (gusts %.0f mph) %s\n",
		forecast.Current.WindSpeed,
		forecast.Current.WindGusts,
		windDirectionLabel(forecast.Current.WindDirection))
	fmt.Fprintf(w, "Humidity: %.0f%%  UV: %.0f\n",
		forecast.Current.Humidity,
		forecast.Current.UVIndex)

	if len(forecast.Daily.TempMax) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Today: High %.0f°F / Low %.0f°F\n",
			forecast.Daily.TempMax[0],
			forecast.Daily.TempMin[0])
		if len(forecast.Daily.PrecipProb) > 0 {
			fmt.Fprintf(w, "Precip chance: %.0f%%\n", forecast.Daily.PrecipProb[0])
		}
		if len(forecast.Daily.Sunrise) > 0 && len(forecast.Daily.Sunset) > 0 {
			sunrise := formatTimeShort(forecast.Daily.Sunrise[0])
			sunset := formatTimeShort(forecast.Daily.Sunset[0])
			fmt.Fprintf(w, "Sunrise: %s  Sunset: %s\n", sunrise, sunset)
		}
	}

	// NWS Alerts
	if alertErr == nil && len(alerts) > 0 {
		fmt.Fprintln(w)
		for _, a := range alerts {
			event, _ := a["event"].(string)
			severity, _ := a["severity"].(string)
			headline, _ := a["headline"].(string)
			if event == "" {
				continue
			}
			label := fmt.Sprintf("ALERT: %s", event)
			if severity != "" {
				label = fmt.Sprintf("ALERT [%s]: %s", strings.ToUpper(severity), event)
			}
			fmt.Fprintf(w, "%s\n", label)
			if headline != "" {
				fmt.Fprintf(w, "  %s\n", headline)
			}
		}
	} else if alertErr != nil {
		fmt.Fprintf(os.Stderr, "note: could not fetch NWS alerts: %v\n", alertErr)
	}

	return nil
}

// windDirectionLabel converts degrees to a compass direction.
func windDirectionLabel(deg float64) string {
	dirs := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	idx := int(math.Round(deg/22.5)) % 16
	return dirs[idx]
}

// formatTimeShort extracts HH:MM from an ISO datetime string.
func formatTimeShort(iso string) string {
	if len(iso) >= 16 {
		return iso[11:16]
	}
	return iso
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
