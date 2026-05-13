package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/realtime"
)

func newRealtimeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realtime",
		Short: "Live GTFS-Realtime feeds (vehicle positions, trip updates, alerts) and GTFS static downloads",
		Long: "Decode the TfNSW GTFS-Realtime protobuf feeds and download GTFS static timetables.\n\n" +
			"Valid modes: " + strings.Join(realtime.ValidModes, ", ") + " (some accept a `/operator` suffix, e.g. buses/SBSC008).",
	}
	cmd.AddCommand(newRealtimeVehiclesCmd(flags))
	cmd.AddCommand(newRealtimeTripsCmd(flags))
	cmd.AddCommand(newRealtimeAlertsCmd(flags))
	cmd.AddCommand(newRealtimeScheduleCmd(flags))
	return cmd
}

func newRealtimeVehiclesCmd(flags *rootFlags) *cobra.Command {
	var version, route string
	cmd := &cobra.Command{
		Use:         "vehicles <mode>",
		Short:       "Live vehicle positions for a transport mode",
		Long:        "Fetch and decode the GTFS-Realtime vehicle-positions feed for a mode. Optionally filter to one route.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli realtime vehicles sydneytrains --json\n  nsw-transport-pp-cli realtime vehicles buses --route 333\n  nsw-transport-pp-cli realtime vehicles metro --version v2 --json --select route_id,latitude,longitude", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			vs, err := c.Vehicles(ctxOf(cmd), version, args[0])
			if err != nil {
				return classifyNSWError(err, flags)
			}
			if route != "" {
				vs = filterVehiclesByRoute(vs, route)
			}
			return emitJSON(cmd, flags, vs)
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Feed version: v1 or v2 (default: v2 for sydneytrains/metro/lightrail, v1 otherwise)")
	cmd.Flags().StringVar(&route, "route", "", "Only vehicles on this route ID (matched case-insensitively, prefix-aware)")
	return cmd
}

func newRealtimeTripsCmd(flags *rootFlags) *cobra.Command {
	var version, route string
	cmd := &cobra.Command{
		Use:         "trips <mode>",
		Short:       "Live trip updates (delays/cancellations) for a transport mode",
		Long:        "Fetch and decode the GTFS-Realtime trip-updates feed for a mode.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli realtime trips sydneytrains --json\n  nsw-transport-pp-cli realtime trips sydneytrains --route T1 --json --select trip_id,max_delay_seconds", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			tus, err := c.TripUpdates(ctxOf(cmd), version, args[0])
			if err != nil {
				return classifyNSWError(err, flags)
			}
			if route != "" {
				tus = filterTripsByRoute(tus, route)
			}
			return emitJSON(cmd, flags, tus)
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Feed version: v1 or v2 (default depends on mode)")
	cmd.Flags().StringVar(&route, "route", "", "Only trip updates on this route ID")
	return cmd
}

func newRealtimeAlertsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "alerts <mode>",
		Short:       "Live GTFS-Realtime service alerts for a transport mode (use \"all\" for every mode)",
		Long:        "Fetch and decode the GTFS-Realtime service-alerts feed. Pass \"all\" for the combined feed.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli realtime alerts all --json\n  nsw-transport-pp-cli realtime alerts sydneytrains --json --select header,effect,affected_routes", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			as, err := c.Alerts(ctxOf(cmd), args[0])
			if err != nil {
				return classifyNSWError(err, flags)
			}
			return emitJSON(cmd, flags, as)
		},
	}
	return cmd
}

func newRealtimeScheduleCmd(flags *rootFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "schedule <mode>",
		Short: "Download the GTFS static timetable ZIP for a transport mode",
		Long: "Download the GTFS static timetable archive for a mode. By default prints the URL that would be downloaded; pass --output <file.zip> to write it.\n\n" +
			"Valid modes include sydneytrains, buses, buses/<operator>, ferries/<operator>, lightrail/<operator>, nswtrains, regionbuses/<operator>, metro.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli realtime schedule sydneytrains\n  nsw-transport-pp-cli realtime schedule sydneytrains --output ./sydneytrains-gtfs.zip", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			mode := args[0]
			if !validScheduleModeRoot(mode) {
				return usageErr(fmt.Errorf("unknown mode %q; valid modes (some accept an /operator suffix): %s", mode, strings.Join(realtime.ValidModes, ", ")))
			}
			url := realtime.ScheduleURL(mode)
			if output == "" || cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would download: %s\n(pass --output <file.zip> to write it)\n", url)
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would download: %s -> %s\n", url, output)
				return nil
			}
			c, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			n, err := c.DownloadSchedule(ctxOf(cmd), mode, output)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes) from %s\n", output, n, url)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Write the GTFS ZIP to this file (default: print the URL only)")
	return cmd
}

func validScheduleModeRoot(mode string) bool {
	root := strings.ToLower(strings.SplitN(mode, "/", 2)[0])
	for _, m := range realtime.ValidModes {
		if m == root {
			return true
		}
	}
	return false
}

func routeMatches(routeID, want string) bool {
	r := strings.ToLower(routeID)
	w := strings.ToLower(want)
	return r == w || strings.HasPrefix(r, w+"_") || strings.HasSuffix(r, "_"+w) || strings.Contains(r, "_"+w+"_")
}

func filterVehiclesByRoute(vs []realtime.Vehicle, route string) []realtime.Vehicle {
	out := vs[:0]
	for _, v := range vs {
		if routeMatches(v.RouteID, route) {
			out = append(out, v)
		}
	}
	return out
}

func filterTripsByRoute(ts []realtime.TripUpdate, route string) []realtime.TripUpdate {
	out := ts[:0]
	for _, t := range ts {
		if routeMatches(t.RouteID, route) {
			out = append(out, t)
		}
	}
	return out
}
