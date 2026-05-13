package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/geo"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/nsw"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/realtime"
)

// ---------------------------------------------------------------------------
// commute — plan a journey, then surface line alerts + nearby road hazards
// ---------------------------------------------------------------------------

type tripLeg struct {
	Duration      int    `json:"duration_seconds,omitempty"`
	Line          string `json:"line,omitempty"`
	OriginName    string `json:"origin,omitempty"`
	DestName      string `json:"destination,omitempty"`
	DepartPlanned string `json:"depart_planned,omitempty"`
	ArrivePlanned string `json:"arrive_planned,omitempty"`
}

type lineAlert struct {
	Title    string   `json:"title,omitempty"`
	Content  string   `json:"content,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Lines    []string `json:"affected_lines,omitempty"`
}

type commuteResult struct {
	Origin        string          `json:"origin"`
	Destination   string          `json:"destination"`
	Verdict       string          `json:"verdict"`
	JourneyMins   int             `json:"journey_minutes,omitempty"`
	Legs          []tripLeg       `json:"legs,omitempty"`
	LineAlerts    []lineAlert     `json:"line_alerts"`
	NearbyHazards []hazardFeature `json:"nearby_hazards"`
}

func resolveEndpoint(c *client.Client, value string) (id, name string, lat, lng float64, isCoord bool, err error) {
	// "lng:lat:EPSG:4326" coord string -> pass through
	if strings.Contains(value, ":EPSG:") || strings.Count(value, ":") >= 2 {
		return value, value, 0, 0, true, nil
	}
	n, la, ln, e := resolveStopCoord(c, value)
	if e != nil {
		return "", "", 0, 0, false, e
	}
	// resolveStopCoord picked a location; recover its ID by re-querying is
	// wasteful, so re-run the finder once and capture the picked ID inline.
	id2, name2, la2, ln2, e2 := resolveStopFull(c, value)
	if e2 == nil {
		return id2, name2, la2, ln2, false, nil
	}
	return value, n, la, ln, false, nil
}

// resolveStopFull is resolveStopCoord but also returns the matched stop ID.
func resolveStopFull(c *client.Client, idOrName string) (id, name string, lat, lng float64, err error) {
	raw, err := tpGet(c, "/stop_finder", map[string]string{"type_sf": "any", "name_sf": idOrName, "TfNSWSF": "true"})
	if err != nil {
		return "", "", 0, 0, err
	}
	var resp struct {
		Locations []tpLocation `json:"locations"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", "", 0, 0, err
	}
	if len(resp.Locations) == 0 {
		return "", "", 0, 0, notFoundErr(fmt.Errorf("no stop or place found matching %q", idOrName))
	}
	pick := pickBestLocation(resp.Locations, idOrName)
	la, ln, _ := pick.latLng()
	return pick.ID, pick.Name, la, ln, nil
}

func newCommuteCmd(flags *rootFlags) *cobra.Command {
	var depart, arrive, date string
	var hazardRadius float64
	cmd := &cobra.Command{
		Use:   "commute <origin> <destination>",
		Short: "Plan a journey A→B and report line alerts plus open road hazards near the route",
		Long: "Resolve two stops or places, plan a journey between them, then report (1) Trip Planner service-status messages affecting any line the journey uses, and (2) open road hazards within --hazard-radius km of either endpoint. The verdict is \"clear\" only when both are empty.\n\n" +
			"Origin/destination may be stop IDs (from `trip stops`) or place names.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli commute \"Marrickville\" \"Central\" --depart 0815\n  nsw-transport-pp-cli commute 10101100 200060 --arrive 0900 --json\n  nsw-transport-pp-cli commute \"Parramatta\" \"Town Hall\" --json --select verdict,line_alerts,nearby_hazards", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,4,5,7"},
		Args:        cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("commute needs two arguments: an origin and a destination (stop IDs or place names)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			oid, oname, olat, olng, _, err := resolveEndpoint(c, args[0]) // pp:client-call
			if err != nil {
				return classifyNSWError(err, flags)
			}
			did, dname, dlat, dlng, _, err := resolveEndpoint(c, args[1])
			if err != nil {
				return classifyNSWError(err, flags)
			}
			tripParams := map[string]string{
				"type_origin": "any", "name_origin": oid,
				"type_destination": "any", "name_destination": did,
				"TfNSWTR": "true", "calcNumberOfTrips": "3",
			}
			switch {
			case arrive != "":
				tripParams["depArrMacro"] = "arr"
				tripParams["itdTime"] = arrive
			case depart != "":
				tripParams["depArrMacro"] = "dep"
				tripParams["itdTime"] = depart
			}
			if date != "" {
				tripParams["itdDate"] = date
			}
			rawTrip, err := tpGet(c, "/trip", tripParams)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			legs, lines, mins := parseFirstJourney(rawTrip)

			// Line alerts from add_info, filtered to the lines used.
			var matchedAlerts []lineAlert
			if rawInfo, ierr := tpGet(c, "/add_info", map[string]string{"filterPublicationStatus": "current"}); ierr == nil {
				matchedAlerts = matchLineAlerts(rawInfo, lines)
			}

			// Open hazards near either endpoint.
			var nearHaz []hazardFeature
			if hz, herr := fetchAllOpenHazards(c); herr == nil {
				seen := map[string]bool{}
				for _, src := range [][2]float64{{olat, olng}, {dlat, dlng}} {
					if src[0] == 0 && src[1] == 0 {
						continue
					}
					for _, h := range hazardsWithin(hz, src[0], src[1], hazardRadius) {
						key := string(h.ID) + h.Headline
						if seen[key] {
							continue
						}
						seen[key] = true
						nearHaz = append(nearHaz, h)
					}
				}
			}

			res := commuteResult{
				Origin: orFallback(oname, args[0]), Destination: orFallback(dname, args[1]),
				JourneyMins: mins, Legs: legs, LineAlerts: matchedAlerts, NearbyHazards: nearHaz,
			}
			if len(matchedAlerts) == 0 && len(nearHaz) == 0 {
				res.Verdict = "clear"
			} else {
				res.Verdict = fmt.Sprintf("check: %d line alert(s), %d nearby hazard(s)", len(matchedAlerts), len(nearHaz))
			}
			return emitJSON(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&depart, "depart", "", "Depart at this time HHMM (24h); default now")
	cmd.Flags().StringVar(&arrive, "arrive", "", "Arrive by this time HHMM (24h); overrides --depart")
	cmd.Flags().StringVar(&date, "date", "", "Travel date YYYYMMDD; default today")
	cmd.Flags().Float64Var(&hazardRadius, "hazard-radius", 3, "Report open hazards within this many km of either endpoint")
	return cmd
}

func parseFirstJourney(raw json.RawMessage) (legs []tripLeg, lines []string, mins int) {
	var resp struct {
		Journeys []struct {
			Legs []struct {
				Duration       int `json:"duration"`
				Transportation struct {
					DisassembledName string `json:"disassembledName"`
					Number           string `json:"number"`
					Name             string `json:"name"`
					Destination      struct {
						Name string `json:"name"`
					} `json:"destination"`
				} `json:"transportation"`
				Origin struct {
					Name                 string `json:"name"`
					DepartureTimePlanned string `json:"departureTimePlanned"`
				} `json:"origin"`
				Destination struct {
					Name               string `json:"name"`
					ArrivalTimePlanned string `json:"arrivalTimePlanned"`
				} `json:"destination"`
			} `json:"legs"`
		} `json:"journeys"`
	}
	if json.Unmarshal(raw, &resp) != nil || len(resp.Journeys) == 0 {
		return nil, nil, 0
	}
	j := resp.Journeys[0]
	total := 0
	seenLines := map[string]bool{}
	for _, l := range j.Legs {
		total += l.Duration
		line := firstNonEmpty(l.Transportation.DisassembledName, l.Transportation.Number, l.Transportation.Name)
		if line != "" && !seenLines[line] {
			seenLines[line] = true
			lines = append(lines, line)
		}
		legs = append(legs, tripLeg{
			Duration: l.Duration, Line: line,
			OriginName: l.Origin.Name, DestName: l.Destination.Name,
			DepartPlanned: l.Origin.DepartureTimePlanned, ArrivePlanned: l.Destination.ArrivalTimePlanned,
		})
	}
	return legs, lines, total / 60
}

func matchLineAlerts(raw json.RawMessage, lines []string) []lineAlert {
	if len(lines) == 0 {
		return nil
	}
	var resp struct {
		Infos struct {
			Current []struct {
				Title         string `json:"title"`
				Content       string `json:"content"`
				Priority      string `json:"priority"`
				AffectedLines []struct {
					Name             string `json:"name"`
					Number           string `json:"number"`
					DisassembledName string `json:"disassembledName"`
				} `json:"affectedLines"`
			} `json:"current"`
		} `json:"infos"`
	}
	if json.Unmarshal(raw, &resp) != nil {
		return nil
	}
	want := map[string]bool{}
	for _, l := range lines {
		want[strings.ToLower(strings.TrimSpace(l))] = true
	}
	var out []lineAlert
	for _, m := range resp.Infos.Current {
		var hit []string
		for _, al := range m.AffectedLines {
			for _, cand := range []string{al.DisassembledName, al.Number, al.Name} {
				cl := strings.ToLower(strings.TrimSpace(cand))
				if cl != "" && want[cl] {
					hit = append(hit, firstNonEmpty(al.DisassembledName, al.Number, al.Name))
				}
			}
		}
		if len(hit) > 0 {
			out = append(out, lineAlert{Title: m.Title, Content: m.Content, Priority: m.Priority, Lines: dedupeStrings(hit)})
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// fuel-stop — cheapest fuel near a named place or coordinate
// ---------------------------------------------------------------------------

func newFuelStopCmd(flags *rootFlags) *cobra.Command {
	var near, coord, brand, sortBy string
	var radius float64
	cmd := &cobra.Command{
		Use:   "fuel-stop <fueltype>",
		Short: "Cheapest fuel of a given type near a named stop/place or a coordinate",
		Long: "Resolve a place name to coordinates (via the Trip Planner stop finder), then return FuelCheck stations within --radius km of it, sorted by price.\n\n" +
			"Provide one of --near \"<place>\" or --coord <lat,lng>.\n" +
			"Needs the FuelCheck credentials (NSW_FUELCHECK_API_KEY / NSW_FUELCHECK_API_SECRET) plus an Open Data Hub key when using --near.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel-stop E10 --near \"Central\" --radius 5 --sort price\n  nsw-transport-pp-cli fuel-stop DL --coord -33.81,151.00 --radius 8 --json --select stationcode,prices,distance_km", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,3,4,5,7,10"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fuelType := strings.ToUpper(args[0])
			if near == "" && coord == "" {
				return usageErr(fmt.Errorf("provide --near \"<place>\" or --coord <lat,lng>"))
			}
			if dryRunOK(flags) {
				return nil
			}
			var lat, lng float64
			placeName := coord
			if coord != "" {
				la, ln, ok := parseLatLng([]string{coord})
				if !ok {
					return usageErr(fmt.Errorf("could not parse --coord %q as <lat,lng>", coord))
				}
				lat, lng = la, ln
			} else {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				name, la, ln, err := resolveStopCoord(c, near) // pp:client-call
				if err != nil {
					return classifyNSWError(err, flags)
				}
				lat, lng, placeName = la, ln, orFallback(name, near)
			}
			fc, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			req := fuelcheck.NearbyRequest{FuelType: fuelType, Latitude: lat, Longitude: lng, Radius: radius, SortBy: pickSortBy(sortBy), SortAscending: true}
			if brand != "" {
				req.Brand = []string{brand}
			}
			pr, err := fc.PricesNearby(ctxOf(cmd), req)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			views := joinPrices(pr, fuelType, "")
			sortFuelViews(views, pickSortBy(sortBy), fuelType)
			out := struct {
				Near     string            `json:"near"`
				FuelType string            `json:"fueltype"`
				RadiusKm float64           `json:"radius_km"`
				Stations []fuelStationView `json:"stations"`
			}{Near: placeName, FuelType: fuelType, RadiusKm: radius, Stations: views}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&near, "near", "", "Place or stop name to search near")
	cmd.Flags().StringVar(&coord, "coord", "", "Coordinate as <lat,lng> (alternative to --near)")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter to one brand")
	cmd.Flags().StringVar(&sortBy, "sort", "price", "Sort by: price | distance")
	cmd.Flags().Float64Var(&radius, "radius", 5, "Search radius in kilometres")
	return cmd
}

// ---------------------------------------------------------------------------
// whereis — live vehicles on a route, ranked by distance from a stop
// ---------------------------------------------------------------------------

func newWhereisCmd(flags *rootFlags) *cobra.Command {
	var mode, near, version string
	cmd := &cobra.Command{
		Use:   "whereis [route] <routeID>",
		Short: "Live vehicle positions on a route, ranked by distance from a stop",
		Long: "Decode the GTFS-Realtime vehicle-positions feed for --mode, keep only vehicles on <routeID>, and (with --near <stopID>) rank them by distance from that stop.\n\n" +
			"The optional literal word \"route\" before the ID is accepted for readability: `whereis route 333` == `whereis 333`.\n" +
			"Valid modes: " + strings.Join(realtime.ValidModes, ", ") + ".",
		Example:     strings.Trim("\n  nsw-transport-pp-cli whereis route 333 --near 203311 --mode buses\n  nsw-transport-pp-cli whereis T1 --mode sydneytrains --json --select route_id,latitude,longitude,distance_km", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,3,4,5,7"},
		Args:        cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			routeID := args[0]
			if strings.EqualFold(routeID, "route") {
				if len(args) < 2 {
					return usageErr(fmt.Errorf("expected a route ID after \"route\""))
				}
				routeID = args[1]
			}
			if dryRunOK(flags) {
				return nil
			}
			rc, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			vs, err := rc.Vehicles(ctxOf(cmd), version, mode)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			vs = filterVehiclesByRoute(vs, routeID)

			type rankedVehicle struct {
				realtime.Vehicle
				DistanceKm float64 `json:"distance_km,omitempty"`
			}
			ranked := make([]rankedVehicle, 0, len(vs))
			var stopName string
			if near != "" {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				name, slat, slng, rerr := resolveStopCoord(c, near) // pp:client-call
				if rerr != nil {
					return classifyNSWError(rerr, flags)
				}
				stopName = name
				for _, v := range vs {
					rv := rankedVehicle{Vehicle: v}
					if v.Latitude != 0 || v.Longitude != 0 {
						rv.DistanceKm = round2(geo.Haversine(slat, slng, v.Latitude, v.Longitude))
					}
					ranked = append(ranked, rv)
				}
				sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].DistanceKm < ranked[j].DistanceKm })
			} else {
				for _, v := range vs {
					ranked = append(ranked, rankedVehicle{Vehicle: v})
				}
			}
			out := struct {
				Route        string          `json:"route"`
				Mode         string          `json:"mode"`
				NearStop     string          `json:"near_stop,omitempty"`
				VehicleCount int             `json:"vehicle_count"`
				Vehicles     []rankedVehicle `json:"vehicles"`
			}{Route: routeID, Mode: mode, NearStop: orFallback(stopName, near), VehicleCount: len(ranked), Vehicles: ranked}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "buses", "Transport mode whose vehicle feed to read")
	cmd.Flags().StringVar(&near, "near", "", "Stop ID to rank vehicles by distance from")
	cmd.Flags().StringVar(&version, "version", "", "Feed version v1|v2 (default depends on mode)")
	return cmd
}

// ---------------------------------------------------------------------------
// station — everything about one stop
// ---------------------------------------------------------------------------

func newStationCmd(flags *rootFlags) *cobra.Command {
	var count int
	var cameraRadius, fuelRadius float64
	var fuelType string
	cmd := &cobra.Command{
		Use:   "station <stopID>",
		Short: "Everything about one stop: next departures, service status, nearby cameras and fuel",
		Long: "Resolve a stop, then gather in one report: the next --count departures, current service-status messages affecting lines at this stop, traffic cameras within --camera-radius km, and the cheapest fuel within --fuel-radius km (FuelCheck creds permitting).\n\n" +
			"Stop IDs come from `trip stops`.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli station 10101100 --json\n  nsw-transport-pp-cli station 2204135 --count 3 --json --select stop,departures,nearby_cameras", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sid, sname, slat, slng, err := resolveStopFull(c, args[0]) // pp:client-call
			if err != nil {
				return classifyNSWError(err, flags)
			}

			type departure struct {
				Line          string `json:"line,omitempty"`
				Destination   string `json:"destination,omitempty"`
				PlannedTime   string `json:"planned_time,omitempty"`
				EstimatedTime string `json:"estimated_time,omitempty"`
				Realtime      bool   `json:"realtime"`
			}
			var deps []departure
			depLines := map[string]bool{}
			if rawDM, derr := tpGet(c, "/departure_mon", map[string]string{
				"name_dm": sid, "type_dm": "stop", "mode": "direct", "departureMonitorMacro": "true", "TfNSWDM": "true",
			}); derr == nil {
				var resp struct {
					StopEvents []struct {
						DepartureTimePlanned   string `json:"departureTimePlanned"`
						DepartureTimeEstimated string `json:"departureTimeEstimated"`
						IsRealtimeControlled   bool   `json:"isRealtimeControlled"`
						Transportation         struct {
							Number           string `json:"number"`
							DisassembledName string `json:"disassembledName"`
							Destination      struct {
								Name string `json:"name"`
							} `json:"destination"`
						} `json:"transportation"`
					} `json:"stopEvents"`
				}
				if json.Unmarshal(rawDM, &resp) == nil {
					for i, se := range resp.StopEvents {
						if i >= count {
							break
						}
						line := firstNonEmpty(se.Transportation.DisassembledName, se.Transportation.Number)
						if line != "" {
							depLines[strings.ToLower(line)] = true
						}
						deps = append(deps, departure{
							Line: line, Destination: se.Transportation.Destination.Name,
							PlannedTime: se.DepartureTimePlanned, EstimatedTime: se.DepartureTimeEstimated, Realtime: se.IsRealtimeControlled,
						})
					}
				}
			}

			// Service status messages affecting any departing line.
			var svc []lineAlert
			if rawInfo, ierr := tpGet(c, "/add_info", map[string]string{"filterPublicationStatus": "current"}); ierr == nil {
				var lines []string
				for l := range depLines {
					lines = append(lines, l)
				}
				svc = matchLineAlerts(rawInfo, lines)
			}

			var nearCams []cameraFeature
			if cams, cerr := fetchCameras(c); cerr == nil && (slat != 0 || slng != 0) {
				nearCams = camerasWithin(cams, slat, slng, cameraRadius)
				if len(nearCams) > 3 {
					nearCams = nearCams[:3]
				}
			}

			var cheapestFuel []fuelStationView
			var fuelNote string
			if slat != 0 || slng != 0 {
				if fc, ferr := nswFuelClient(flags); ferr == nil { // pp:client-call
					if pr, perr := fc.PricesNearby(ctxOf(cmd), fuelcheck.NearbyRequest{FuelType: strings.ToUpper(fuelType), Latitude: slat, Longitude: slng, Radius: fuelRadius, SortBy: "price", SortAscending: true}); perr == nil {
						v := joinPrices(pr, fuelType, "")
						sortFuelViews(v, "price", fuelType)
						if len(v) > 3 {
							v = v[:3]
						}
						cheapestFuel = v
					}
				} else {
					fuelNote = "set NSW_FUELCHECK_API_KEY / NSW_FUELCHECK_API_SECRET to include nearby fuel prices"
				}
			}

			out := struct {
				Stop struct {
					ID        string  `json:"id"`
					Name      string  `json:"name"`
					Latitude  float64 `json:"latitude,omitempty"`
					Longitude float64 `json:"longitude,omitempty"`
				} `json:"stop"`
				Departures      []departure       `json:"departures"`
				ServiceStatus   []lineAlert       `json:"service_status"`
				NearbyCameras   []cameraFeature   `json:"nearby_cameras"`
				CheapestFuelE10 []fuelStationView `json:"cheapest_fuel_nearby"`
				FuelNote        string            `json:"fuel_note,omitempty"`
			}{}
			out.Stop.ID, out.Stop.Name, out.Stop.Latitude, out.Stop.Longitude = orFallback(sid, args[0]), sname, slat, slng
			out.Departures, out.ServiceStatus, out.NearbyCameras, out.CheapestFuelE10, out.FuelNote = deps, svc, nearCams, cheapestFuel, fuelNote
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&count, "count", 5, "Number of next departures to include")
	cmd.Flags().Float64Var(&cameraRadius, "camera-radius", 3, "Include traffic cameras within this many km")
	cmd.Flags().Float64Var(&fuelRadius, "fuel-radius", 3, "Include the cheapest fuel within this many km")
	cmd.Flags().StringVar(&fuelType, "fueltype", "E10", "Fuel type for the nearby-fuel lookup")
	return cmd
}

// ---------------------------------------------------------------------------
// cameras-near — traffic cameras within N km of a stop or coordinate
// ---------------------------------------------------------------------------

func newCamerasNearCmd(flags *rootFlags) *cobra.Command {
	var coord, save string
	var radius float64
	cmd := &cobra.Command{
		Use:   "cameras-near [stopID]",
		Short: "Traffic cameras within N km of a stop or coordinate, with optional image download",
		Long: "List Live Traffic cameras within --radius km of a stop (by ID) or a --coord <lat,lng>, nearest first. With --save <file> the nearest camera's current JPEG is downloaded.\n\n" +
			"Cameras refresh roughly every 15 seconds.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli cameras-near 203311 --radius 3 --json\n  nsw-transport-pp-cli cameras-near --coord -33.8688,151.2093 --radius 2 --save ./cam.jpg", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,3,4,5,7"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && coord == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var lat, lng float64
			var anchor string
			if coord != "" {
				la, ln, ok := parseLatLng([]string{coord})
				if !ok {
					return usageErr(fmt.Errorf("could not parse --coord %q", coord))
				}
				lat, lng, anchor = la, ln, coord
			} else {
				name, la, ln, rerr := resolveStopCoord(c, args[0]) // pp:client-call
				if rerr != nil {
					return classifyNSWError(rerr, flags)
				}
				lat, lng, anchor = la, ln, orFallback(name, args[0])
			}
			cams, cerr := fetchCameras(c)
			if cerr != nil {
				return classifyNSWError(cerr, flags)
			}
			near := camerasWithin(cams, lat, lng, radius)

			if save != "" {
				if cliutil.IsVerifyEnv() {
					if len(near) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "would download: %s -> %s\n", near[0].Href, save)
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "no camera within radius to download")
					}
					return nil
				}
				if len(near) == 0 {
					return notFoundErr(fmt.Errorf("no camera within %.1f km of %s", radius, anchor))
				}
				n, derr := downloadFile(ctxOf(cmd), near[0].Href, save)
				if derr != nil {
					return apiErr(fmt.Errorf("downloading camera image: %w", derr))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes) — camera %s (%s)\n", save, n, near[0].ID, near[0].Title)
				return nil
			}
			out := struct {
				Anchor   string          `json:"anchor"`
				RadiusKm float64         `json:"radius_km"`
				Count    int             `json:"count"`
				Cameras  []cameraFeature `json:"cameras"`
			}{Anchor: anchor, RadiusKm: radius, Count: len(near), Cameras: near}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&coord, "coord", "", "Coordinate as <lat,lng> (alternative to a stop ID)")
	cmd.Flags().Float64Var(&radius, "radius", 3, "Search radius in kilometres")
	cmd.Flags().StringVar(&save, "save", "", "Download the nearest camera's current JPEG to this file")
	return cmd
}

// ---------------------------------------------------------------------------
// disruptions — GTFS-RT alerts ∪ Trip Planner add_info, deduped
// ---------------------------------------------------------------------------

type disruption struct {
	Source        string   `json:"source"` // "gtfs-realtime" | "trip-planner"
	ID            string   `json:"id,omitempty"`
	Header        string   `json:"header"`
	Description   string   `json:"description,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	AffectedLines []string `json:"affected_lines,omitempty"`
	ActiveFrom    string   `json:"active_from,omitempty"`
	ActiveTo      string   `json:"active_to,omitempty"`
	URL           string   `json:"url,omitempty"`
}

func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "very_high", "extreme", "veryhigh":
		return 0
	case "high":
		return 1
	case "normal", "medium":
		return 2
	case "low":
		return 3
	default:
		return 2
	}
}

func newDisruptionsCmd(flags *rootFlags) *cobra.Command {
	var mode string
	var all bool
	cmd := &cobra.Command{
		Use:   "disruptions",
		Short: "Today's disruptions across modes — GTFS-RT alerts unioned with Trip Planner service status, deduped",
		Long: "Fetch the GTFS-Realtime service-alerts feed and the Trip Planner add_info messages, normalize both, dedupe by header, and sort by severity.\n\n" +
			"Use --all for every mode (the combined alerts feed), or --mode <mode> to scope to one.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli disruptions --all --json\n  nsw-transport-pp-cli disruptions --mode sydneytrains --json --select source,header,severity,affected_lines", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			feedMode := mode
			if all || feedMode == "" {
				feedMode = "all"
			}
			var out []disruption
			var firstErr error

			rc, err := nswRealtimeClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			if alerts, aerr := rc.Alerts(ctxOf(cmd), feedMode); aerr == nil {
				for _, a := range alerts {
					out = append(out, disruption{
						Source: "gtfs-realtime", ID: a.ID, Header: firstNonEmpty(a.Header, a.Effect), Description: a.Description,
						Severity: a.Effect, AffectedLines: a.AffectedRoutes, ActiveFrom: a.ActiveFrom, ActiveTo: a.ActiveTo, URL: a.URL,
					})
				}
			} else {
				firstErr = aerr
			}

			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			infoParams := map[string]string{"filterPublicationStatus": "current"}
			if !all && mode != "" {
				if mot := motTypeFor(mode); mot != "" {
					infoParams["filterMOTType"] = mot
				}
			}
			if rawInfo, ierr := tpGet(c, "/add_info", infoParams); ierr == nil {
				var resp struct {
					Infos struct {
						Current []struct {
							ID            string `json:"id"`
							Title         string `json:"title"`
							Content       string `json:"content"`
							Priority      string `json:"priority"`
							URL           string `json:"url"`
							AffectedLines []struct {
								Name             string `json:"name"`
								Number           string `json:"number"`
								DisassembledName string `json:"disassembledName"`
							} `json:"affectedLines"`
							ValidityPeriods []struct {
								From string `json:"from"`
								To   string `json:"to"`
							} `json:"validityPeriods"`
						} `json:"current"`
					} `json:"infos"`
				}
				if json.Unmarshal(rawInfo, &resp) == nil {
					for _, m := range resp.Infos.Current {
						var lines []string
						for _, al := range m.AffectedLines {
							if n := firstNonEmpty(al.DisassembledName, al.Number, al.Name); n != "" {
								lines = append(lines, n)
							}
						}
						d := disruption{Source: "trip-planner", ID: m.ID, Header: m.Title, Description: m.Content, Severity: m.Priority, AffectedLines: dedupeStrings(lines), URL: m.URL}
						if len(m.ValidityPeriods) > 0 {
							d.ActiveFrom, d.ActiveTo = m.ValidityPeriods[0].From, m.ValidityPeriods[0].To
						}
						out = append(out, d)
					}
				}
			} else if firstErr == nil {
				firstErr = ierr
			}

			if len(out) == 0 && firstErr != nil {
				return classifyNSWError(firstErr, flags)
			}
			out = dedupeDisruptions(out)
			sort.SliceStable(out, func(i, j int) bool { return severityRank(out[i].Severity) < severityRank(out[j].Severity) })
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "", "Scope to one mode (sydneytrains, metro, lightrail, buses, ferries, nswtrains, regionbuses)")
	cmd.Flags().BoolVar(&all, "all", false, "All modes (combined feed); the default when --mode is unset")
	return cmd
}

func motTypeFor(mode string) string {
	switch strings.ToLower(mode) {
	case "sydneytrains", "nswtrains":
		return "1"
	case "buses", "regionbuses":
		return "5"
	case "ferries":
		return "9"
	case "lightrail":
		return "4"
	case "metro":
		return "2"
	}
	return ""
}

func dedupeDisruptions(in []disruption) []disruption {
	seen := map[string]bool{}
	var out []disruption
	for _, d := range in {
		key := strings.ToLower(strings.TrimSpace(d.Header)) + "|" + strings.Join(d.AffectedLines, ",")
		if d.Header == "" {
			key = d.Source + "|" + d.ID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}

// ---------------------------------------------------------------------------
// fuel-drift — price change since the last snapshot
// ---------------------------------------------------------------------------

func newFuelDriftCmd(flags *rootFlags) *cobra.Command {
	var brand string
	var includeUnchanged bool
	cmd := &cobra.Command{
		Use:   "fuel-drift <fueltype>",
		Short: "Fuel price change per station since the last snapshot (needs ≥2 snapshots)",
		Long: "Compare the two most recent fuel-price snapshots in the local store for a fuel type and report each station's change (negative = price dropped).\n\n" +
			"Build snapshots by running `nsw-transport-pp-cli refresh` (or `fuel prices`) at least twice — they record a snapshot every time.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel-drift E10 --json\n  nsw-transport-pp-cli fuel-drift U91 --brand \"7-Eleven\" --json --select stationcode,change,current_price", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,10"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := nswOpenStore()
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.FuelDrift(strings.ToUpper(args[0]), brand, !includeUnchanged)
			if err != nil {
				if err == nsw.ErrNoHistory {
					return notFoundErr(err)
				}
				return apiErr(err)
			}
			out := struct {
				FuelType string         `json:"fueltype"`
				Brand    string         `json:"brand,omitempty"`
				Count    int            `json:"count"`
				Drift    []nsw.DriftRow `json:"drift"`
			}{FuelType: strings.ToUpper(args[0]), Brand: brand, Count: len(rows), Drift: rows}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Only this brand")
	cmd.Flags().BoolVar(&includeUnchanged, "include-unchanged", false, "Include stations whose price did not move")
	return cmd
}

// ---------------------------------------------------------------------------
// refresh — populate the local store (fuel-price snapshot)
// ---------------------------------------------------------------------------

func newRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the local location-intelligence store (records a fuel-price snapshot)",
		Long: "Populate the local SQLite store used by the cross-source commands. Currently records a fresh fuel-price snapshot (so `fuel-drift` has history); a no-op if the FuelCheck credentials are not configured.\n\n" +
			"This is separate from `sync`, which syncs the spec-driven Trip Planner / Live Traffic resources.",
		Example:     "  nsw-transport-pp-cli refresh",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fc, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				if _, ok := err.(*cliError); ok {
					fmt.Fprintln(cmd.OutOrStdout(), "refresh: FuelCheck credentials not configured — nothing to refresh (set NSW_FUELCHECK_API_KEY / NSW_FUELCHECK_API_SECRET to enable fuel snapshots)")
					return nil
				}
				return err
			}
			pr, err := fc.Prices(ctxOf(cmd))
			if err != nil {
				return classifyNSWError(err, flags)
			}
			st, serr := nswOpenStore()
			if serr != nil {
				return serr
			}
			defer st.Close()
			when, n, werr := st.SnapshotFuel(pr)
			if werr != nil {
				return apiErr(fmt.Errorf("writing snapshot: %w", werr))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "refresh: recorded %d fuel-price points across %d stations at %s\n", n, len(pr.Stations), when.Format("2006-01-02 15:04:05 UTC"))
			return nil
		},
	}
	return cmd
}

// --- small shared helpers ---

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func orFallback(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
