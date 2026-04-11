package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newGoCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLon float64

	cmd := &cobra.Command{
		Use:   "go <activity> [location]",
		Short: "Get activity-specific weather verdicts: walk, bike, hike, commute, drive",
		Long: `Check whether conditions are safe for an activity. Each mode applies
domain-specific thresholds and returns a verdict: GO, CAUTION, or STOP.

Activities:
  walk     - Preparation advice based on precip, temperature, UV
  bike     - GO/CAUTION/STOP based on wind, rain, temperature, AQI
  hike     - GO/CAUTION/STOP for thunderstorms, hypothermia risk, UV, wind
  commute  - Compare morning vs evening conditions with umbrella advice
  drive    - GO/CAUTION/STOP for visibility, ice, wind, and NWS warnings`,
		Example: `  weather-goat-pp-cli go walk
  weather-goat-pp-cli go bike "Portland"
  weather-goat-pp-cli go commute
  weather-goat-pp-cli go drive --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			activity := strings.ToLower(args[0])
			locationArgs := args[1:]

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			lat, lon, locName, err := resolveLocation(cfg, flagLat, flagLon,
				cmd.Flags().Changed("latitude"), cmd.Flags().Changed("longitude"), locationArgs)
			if err != nil {
				return err
			}

			c, clientErr := flags.newClient()
			if clientErr != nil {
				return clientErr
			}

			switch activity {
			case "walk":
				return runWalk(cmd, flags, c, lat, lon, locName)
			case "bike":
				return runBike(cmd, flags, c, lat, lon, locName)
			case "hike":
				return runHike(cmd, flags, c, lat, lon, locName)
			case "commute":
				return runCommute(cmd, flags, c, cfg, lat, lon, locName)
			case "drive":
				return runDrive(cmd, flags, c, lat, lon, locName)
			default:
				return usageErr(fmt.Errorf("unknown activity %q. Choose: walk, bike, hike, commute, drive", activity))
			}
		},
	}

	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")

	return cmd
}

type currentConditions struct {
	Temperature   float64
	FeelsLike     float64
	WeatherCode   int
	WindSpeed     float64
	WindGusts     float64
	Precipitation float64
	PrecipProb    float64
	UVIndex       float64
	Humidity      float64
	USAQI         float64
}

func fetchCurrentForActivity(c clientGetter, lat, lon float64) (*currentConditions, error) {
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"current":          "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m,wind_gusts_10m,uv_index",
		"hourly":           "precipitation_probability",
		"timezone":         "auto",
		"temperature_unit": "fahrenheit",
		"wind_speed_unit":  "mph",
		"forecast_hours":   "1",
	}

	data, err := c.Get("/forecast", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Current struct {
			Temperature   float64 `json:"temperature_2m"`
			Humidity      float64 `json:"relative_humidity_2m"`
			ApparentTemp  float64 `json:"apparent_temperature"`
			Precipitation float64 `json:"precipitation"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed     float64 `json:"wind_speed_10m"`
			WindGusts     float64 `json:"wind_gusts_10m"`
			UVIndex       float64 `json:"uv_index"`
		} `json:"current"`
		Hourly struct {
			PrecipProb []float64 `json:"precipitation_probability"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing forecast: %w", err)
	}

	cond := &currentConditions{
		Temperature:   resp.Current.Temperature,
		FeelsLike:     resp.Current.ApparentTemp,
		WeatherCode:   resp.Current.WeatherCode,
		WindSpeed:     resp.Current.WindSpeed,
		WindGusts:     resp.Current.WindGusts,
		Precipitation: resp.Current.Precipitation,
		UVIndex:       resp.Current.UVIndex,
		Humidity:      resp.Current.Humidity,
	}
	if len(resp.Hourly.PrecipProb) > 0 {
		cond.PrecipProb = resp.Hourly.PrecipProb[0]
	}

	return cond, nil
}

type clientGetter interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

func fetchAQI(c clientGetter, lat, lon float64) float64 {
	params := map[string]string{
		"latitude":  fmt.Sprintf("%f", lat),
		"longitude": fmt.Sprintf("%f", lon),
		"current":   "us_aqi",
		"timezone":  "auto",
	}
	data, err := c.Get("/air-quality", params)
	if err != nil {
		return 0
	}
	var resp struct {
		Current struct {
			USAQI float64 `json:"us_aqi"`
		} `json:"current"`
	}
	if json.Unmarshal(data, &resp) == nil {
		return resp.Current.USAQI
	}
	return 0
}

// --- Walk ---

func runWalk(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	var advice []string
	if cond.PrecipProb > 60 || isActiveRain(cond.WeatherCode) {
		advice = append(advice, "Take an umbrella")
	}
	if cond.FeelsLike < 40 {
		advice = append(advice, "Wear warm layers")
	}
	if cond.FeelsLike > 90 {
		advice = append(advice, "Stay hydrated")
	}
	if cond.UVIndex > 6 {
		advice = append(advice, "Wear sunscreen")
	}

	if flags.asJSON {
		result := map[string]any{
			"activity":    "walk",
			"location":    loc,
			"conditions":  describeWeatherCode(cond.WeatherCode),
			"temperature": cond.Temperature,
			"feels_like":  cond.FeelsLike,
			"precip_prob": cond.PrecipProb,
			"uv_index":    cond.UVIndex,
			"advice":      advice,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — Walk\n", bold(loc))
	fmt.Fprintf(w, "%.0f°F (feels like %.0f°F), %s\n", cond.Temperature, cond.FeelsLike, describeWeatherCode(cond.WeatherCode))
	if len(advice) == 0 {
		fmt.Fprintln(w, "Good conditions for a walk!")
	} else {
		for _, a := range advice {
			fmt.Fprintf(w, "  - %s\n", a)
		}
	}
	return nil
}

// --- Bike ---

func runBike(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}
	cond.USAQI = fetchAQI(c, lat, lon)

	verdict := "GO"
	var reasons []string

	// Wind
	if cond.WindSpeed > 30 {
		verdict = "STOP"
		reasons = append(reasons, fmt.Sprintf("Wind %.0f mph — dangerous crosswinds", cond.WindSpeed))
	} else if cond.WindSpeed > 20 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("Wind %.0f mph — strong headwinds possible", cond.WindSpeed))
	}

	// Precipitation
	if isActiveRain(cond.WeatherCode) {
		verdict = "STOP"
		reasons = append(reasons, "Active rain — slippery roads")
	} else if cond.PrecipProb > 60 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("%.0f%% chance of rain", cond.PrecipProb))
	}

	// Temperature
	if cond.Temperature < 20 {
		verdict = "STOP"
		reasons = append(reasons, fmt.Sprintf("%.0f°F — frostbite risk", cond.Temperature))
	} else if cond.Temperature < 32 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("%.0f°F — possible ice on roads", cond.Temperature))
	}

	// AQI
	if cond.USAQI > 150 {
		verdict = "STOP"
		reasons = append(reasons, fmt.Sprintf("AQI %.0f — unhealthy for all groups", cond.USAQI))
	} else if cond.USAQI > 100 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("AQI %.0f — unhealthy for sensitive groups", cond.USAQI))
	}

	return printVerdict(cmd, flags, "bike", loc, verdict, reasons, cond)
}

// --- Hike ---

func runHike(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	verdict := "GO"
	var reasons []string

	// Thunderstorm
	if isThunderstorm(cond.WeatherCode) {
		verdict = "STOP"
		reasons = append(reasons, "Thunderstorm/lightning — do not hike")
	}

	// Hypothermia risk
	if isActiveRain(cond.WeatherCode) && cond.Temperature < 40 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, "Rain + cold — hypothermia risk")
	}

	// UV
	if cond.UVIndex > 8 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("UV index %.0f — high altitude UV exposure", cond.UVIndex))
	}

	// Wind gusts
	if cond.WindGusts > 40 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("Wind gusts %.0f mph — exposed ridges dangerous", cond.WindGusts))
	}

	return printVerdict(cmd, flags, "hike", loc, verdict, reasons, cond)
}

// --- Commute ---

func runCommute(cmd *cobra.Command, flags *rootFlags, c clientGetter, cfg *config.Config, lat, lon float64, loc string) error {
	departTime := cfg.CommuteDepartTime
	if departTime == "" {
		departTime = "08:00"
	}
	returnTime := cfg.CommuteReturnTime
	if returnTime == "" {
		returnTime = "18:00"
	}

	// Fetch hourly forecast
	params := map[string]string{
		"latitude":         fmt.Sprintf("%f", lat),
		"longitude":        fmt.Sprintf("%f", lon),
		"hourly":           "temperature_2m,apparent_temperature,precipitation_probability,precipitation,weather_code,wind_speed_10m",
		"timezone":         "auto",
		"temperature_unit": "fahrenheit",
		"wind_speed_unit":  "mph",
		"forecast_hours":   "24",
	}

	data, err := c.Get("/forecast", params)
	if err != nil {
		return classifyAPIError(err)
	}

	var resp struct {
		Hourly struct {
			Time        []string  `json:"time"`
			Temp        []float64 `json:"temperature_2m"`
			FeelsLike   []float64 `json:"apparent_temperature"`
			PrecipProb  []float64 `json:"precipitation_probability"`
			Precip      []float64 `json:"precipitation"`
			WeatherCode []int     `json:"weather_code"`
			WindSpeed   []float64 `json:"wind_speed_10m"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing hourly forecast: %w", err)
	}

	// Find depart and return hour indices
	departIdx := findHourIndex(resp.Hourly.Time, departTime)
	returnIdx := findHourIndex(resp.Hourly.Time, returnTime)

	if departIdx < 0 || returnIdx < 0 || departIdx >= len(resp.Hourly.WeatherCode) || returnIdx >= len(resp.Hourly.WeatherCode) {
		return fmt.Errorf("could not find commute hours in forecast data")
	}

	departCond := describeWeatherCode(resp.Hourly.WeatherCode[departIdx])
	returnCond := describeWeatherCode(resp.Hourly.WeatherCode[returnIdx])
	departTemp := resp.Hourly.Temp[departIdx]
	returnTemp := resp.Hourly.Temp[returnIdx]
	departPrecipProb := resp.Hourly.PrecipProb[departIdx]
	returnPrecipProb := resp.Hourly.PrecipProb[returnIdx]

	var advice []string
	if isActiveRain(resp.Hourly.WeatherCode[departIdx]) {
		advice = append(advice, "Rain this morning — take umbrella for departure")
	}
	if isActiveRain(resp.Hourly.WeatherCode[returnIdx]) && !isActiveRain(resp.Hourly.WeatherCode[departIdx]) {
		advice = append(advice, fmt.Sprintf("%s by %s — take umbrella for the ride home", returnCond, returnTime))
	}
	if resp.Hourly.WindSpeed[departIdx] > 25 || resp.Hourly.WindSpeed[returnIdx] > 25 {
		advice = append(advice, "Strong winds expected — allow extra travel time")
	}

	if flags.asJSON {
		result := map[string]any{
			"activity": "commute",
			"location": loc,
			"depart": map[string]any{
				"time":        departTime,
				"temperature": departTemp,
				"conditions":  departCond,
				"precip_prob": departPrecipProb,
			},
			"return": map[string]any{
				"time":        returnTime,
				"temperature": returnTemp,
				"conditions":  returnCond,
				"precip_prob": returnPrecipProb,
			},
			"advice": advice,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — Commute\n", bold(loc))
	fmt.Fprintf(w, "Depart (%s): %.0f°F, %s (%.0f%% precip)\n", departTime, departTemp, departCond, departPrecipProb)
	fmt.Fprintf(w, "Return (%s): %.0f°F, %s (%.0f%% precip)\n", returnTime, returnTemp, returnCond, returnPrecipProb)
	if len(advice) > 0 {
		fmt.Fprintln(w)
		for _, a := range advice {
			fmt.Fprintf(w, "  - %s\n", a)
		}
	} else {
		fmt.Fprintln(w, "Clear commute expected.")
	}
	return nil
}

func findHourIndex(times []string, target string) int {
	for i, t := range times {
		// times are like "2026-04-11T08:00"
		if len(t) >= 16 && t[11:16] == target {
			return i
		}
	}
	return -1
}

// --- Drive ---

func runDrive(cmd *cobra.Command, flags *rootFlags, c clientGetter, lat, lon float64, loc string) error {
	cond, err := fetchCurrentForActivity(c, lat, lon)
	if err != nil {
		return classifyAPIError(err)
	}

	verdict := "GO"
	var reasons []string

	// Visibility
	if isLowVisibility(cond.WeatherCode) {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("Low visibility — %s", describeWeatherCode(cond.WeatherCode)))
	}

	// Snow / freezing rain
	if cond.WeatherCode == 66 || cond.WeatherCode == 67 {
		verdict = "STOP"
		reasons = append(reasons, "Freezing rain — extremely dangerous roads")
	} else if isSnow(cond.WeatherCode) {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, "Snow — reduced traction")
	}

	// Wind gusts
	if cond.WindGusts > 60 {
		verdict = "STOP"
		reasons = append(reasons, fmt.Sprintf("Wind gusts %.0f mph — dangerous for all vehicles", cond.WindGusts))
	} else if cond.WindGusts > 45 {
		verdict = maxVerdict(verdict, "CAUTION")
		reasons = append(reasons, fmt.Sprintf("Wind gusts %.0f mph — dangerous for high-profile vehicles", cond.WindGusts))
	}

	// NWS alerts (best-effort)
	alerts, alertErr := nwsAlerts(lat, lon)
	if alertErr == nil {
		for _, a := range alerts {
			event, _ := a["event"].(string)
			severity, _ := a["severity"].(string)
			if strings.Contains(strings.ToLower(event), "warning") || severity == "Extreme" || severity == "Severe" {
				verdict = maxVerdict(verdict, "CAUTION")
				reasons = append(reasons, fmt.Sprintf("NWS: %s", event))
			}
		}
	}

	return printVerdict(cmd, flags, "drive", loc, verdict, reasons, cond)
}

// --- Shared verdict helpers ---

func maxVerdict(current, proposed string) string {
	order := map[string]int{"GO": 0, "CAUTION": 1, "STOP": 2}
	if order[proposed] > order[current] {
		return proposed
	}
	return current
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func printVerdict(cmd *cobra.Command, flags *rootFlags, activity, loc, verdict string, reasons []string, cond *currentConditions) error {
	if flags.asJSON {
		result := map[string]any{
			"activity":    activity,
			"location":    loc,
			"verdict":     verdict,
			"conditions":  describeWeatherCode(cond.WeatherCode),
			"temperature": cond.Temperature,
			"feels_like":  cond.FeelsLike,
			"wind_speed":  cond.WindSpeed,
			"wind_gusts":  cond.WindGusts,
			"reasons":     reasons,
		}
		return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(result), flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %s: %s\n", bold(loc), titleCase(activity), verdict)
	fmt.Fprintf(w, "%.0f°F (feels like %.0f°F), %s, wind %.0f mph\n",
		cond.Temperature, cond.FeelsLike, describeWeatherCode(cond.WeatherCode), cond.WindSpeed)

	if len(reasons) == 0 {
		fmt.Fprintln(w, "No concerns. Enjoy!")
	} else {
		for _, r := range reasons {
			fmt.Fprintf(w, "  - %s\n", r)
		}
	}
	return nil
}
