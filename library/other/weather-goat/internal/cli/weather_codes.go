package cli

// weatherCodeDescription maps WMO weather codes (0-99) to human-readable descriptions.
var weatherCodeDescription = map[int]string{
	0:  "Clear",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Foggy",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	56: "Light freezing drizzle",
	57: "Dense freezing drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	66: "Light freezing rain",
	67: "Heavy freezing rain",
	71: "Slight snow",
	73: "Moderate snow",
	75: "Heavy snow",
	77: "Snow grains",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers",
	86: "Heavy snow showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

func describeWeatherCode(code int) string {
	if desc, ok := weatherCodeDescription[code]; ok {
		return desc
	}
	return "Unknown"
}

// isActiveRain returns true if the weather code indicates active precipitation.
func isActiveRain(code int) bool {
	return (code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95
}

// isSnow returns true if the weather code indicates snow or freezing precipitation.
func isSnow(code int) bool {
	return (code >= 71 && code <= 77) || (code >= 85 && code <= 86) || code == 56 || code == 57 || code == 66 || code == 67
}

// isThunderstorm returns true if the weather code indicates a thunderstorm.
func isThunderstorm(code int) bool {
	return code >= 95
}

// isFog returns true if the weather code indicates fog.
func isFog(code int) bool {
	return code == 45 || code == 48
}

// isLowVisibility returns true for weather codes that reduce visibility.
func isLowVisibility(code int) bool {
	return isFog(code) || code == 65 || code == 67 || code == 75 || code == 82 || code == 86
}
