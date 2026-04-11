package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newAlertsCmd(flags *rootFlags) *cobra.Command {
	var flagState string
	var flagLat float64
	var flagLon float64

	cmd := &cobra.Command{
		Use:   "alerts [location]",
		Short: "View active NWS weather alerts for a location or state",
		Long:  "Fetch active weather alerts from the National Weather Service. Specify a location name, lat/lon, or filter by US state code.",
		Example: `  weather-goat-pp-cli alerts "San Francisco"
  weather-goat-pp-cli alerts --state CA
  weather-goat-pp-cli alerts --latitude 47.6 --longitude -122.3
  weather-goat-pp-cli alerts --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var alerts []map[string]any
			var err error

			if flagState != "" {
				alerts, err = nwsAlertsByState(flagState)
				if err != nil {
					return fmt.Errorf("fetching alerts for state %s: %w", flagState, err)
				}
			} else {
				cfg, cfgErr := config.Load(flags.configPath)
				if cfgErr != nil {
					return configErr(cfgErr)
				}

				lat, lon, locName, locErr := resolveLocation(cfg, flagLat, flagLon,
					cmd.Flags().Changed("latitude"), cmd.Flags().Changed("longitude"), args)
				if locErr != nil {
					return locErr
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Fetching alerts for %s...\n", locName)

				alerts, err = nwsAlerts(lat, lon)
				if err != nil {
					return fmt.Errorf("fetching alerts: %w", err)
				}
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(alerts), flags)
			}

			if len(alerts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active alerts.")
				return nil
			}

			w := cmd.OutOrStdout()
			for i, a := range alerts {
				if i > 0 {
					fmt.Fprintln(w)
				}
				event, _ := a["event"].(string)
				severity, _ := a["severity"].(string)
				headline, _ := a["headline"].(string)
				description, _ := a["description"].(string)
				expires, _ := a["expires"].(string)

				fmt.Fprintf(w, "%s\n", bold(event))
				if severity != "" {
					fmt.Fprintf(w, "  Severity: %s\n", severity)
				}
				if headline != "" {
					fmt.Fprintf(w, "  %s\n", headline)
				}
				if description != "" {
					desc := truncate(description, 200)
					fmt.Fprintf(w, "  %s\n", desc)
				}
				if expires != "" {
					fmt.Fprintf(w, "  Expires: %s\n", formatTimeShort(expires))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagState, "state", "", "Filter by US state code (e.g., CA, NY, TX)")
	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")

	return cmd
}

// Ensure json import is used
var _ = json.Marshal
