package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
)

func newFuelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fuel",
		Short: "NSW FuelCheck — current fuel prices, nearby search, station lookup, trends",
		Long: "Query the NSW FuelCheck (FuelPriceCheck) API.\n\n" +
			"These commands need separate free credentials from the Open Data Hub key: set NSW_FUELCHECK_API_KEY and NSW_FUELCHECK_API_SECRET (register a FuelCheck app at https://api.nsw.gov.au/Product/Index/22). The CLI performs the OAuth2 handshake and caches the token.\n\n" +
			"Common fuel-type codes: E10, U91, P95, P98, DL (diesel), PDL (premium diesel), B20, LPG, CNG, EV. Run `fuel types` for the authoritative list.",
	}
	cmd.AddCommand(newFuelPricesCmd(flags))
	cmd.AddCommand(newFuelNearCmd(flags))
	cmd.AddCommand(newFuelStationCmd(flags))
	cmd.AddCommand(newFuelByLocationCmd(flags))
	cmd.AddCommand(newFuelTypesCmd(flags))
	cmd.AddCommand(newFuelBrandsCmd(flags))
	cmd.AddCommand(newFuelTrendsCmd(flags))
	return cmd
}

// fuelStationView pairs a station with its prices for human/JSON output.
type fuelStationView struct {
	StationCode string             `json:"stationcode"`
	Brand       string             `json:"brand,omitempty"`
	Name        string             `json:"name,omitempty"`
	Address     string             `json:"address,omitempty"`
	Latitude    float64            `json:"latitude,omitempty"`
	Longitude   float64            `json:"longitude,omitempty"`
	DistanceKm  float64            `json:"distance_km,omitempty"`
	Prices      map[string]float64 `json:"prices"`
	LastUpdated map[string]string  `json:"last_updated,omitempty"`
}

// joinPrices folds a PricesResponse (parallel station+price arrays) into one
// view per station, optionally filtering by fuel type and brand.
func joinPrices(pr *fuelcheck.PricesResponse, fuelType, brand string) []fuelStationView {
	byCode := map[string]*fuelStationView{}
	order := []string{}
	for _, st := range pr.Stations {
		if brand != "" && !strings.EqualFold(st.Brand, brand) {
			continue
		}
		v := &fuelStationView{
			StationCode: st.StationCode.String(), Brand: st.Brand, Name: st.Name, Address: st.Address,
			Latitude: st.Location.Latitude.Float(), Longitude: st.Location.Longitude.Float(), DistanceKm: round2(st.Location.Distance.Float()),
			Prices: map[string]float64{}, LastUpdated: map[string]string{},
		}
		byCode[st.StationCode.String()] = v
		order = append(order, st.StationCode.String())
	}
	for _, p := range pr.Prices {
		v := byCode[p.StationCode.String()]
		if v == nil {
			continue
		}
		if fuelType != "" && !strings.EqualFold(p.FuelType, fuelType) {
			continue
		}
		v.Prices[strings.ToUpper(p.FuelType)] = p.Price.Float()
		if p.LastUpdated != "" {
			v.LastUpdated[strings.ToUpper(p.FuelType)] = p.LastUpdated
		}
	}
	out := make([]fuelStationView, 0, len(order))
	for _, code := range order {
		v := byCode[code]
		if fuelType != "" && len(v.Prices) == 0 {
			continue // dropped: this station has no price for the requested type
		}
		out = append(out, *v)
	}
	return out
}

func sortFuelViews(vs []fuelStationView, sortBy, fuelType string) {
	switch strings.ToLower(sortBy) {
	case "distance":
		sort.SliceStable(vs, func(i, j int) bool { return vs[i].DistanceKm < vs[j].DistanceKm })
	case "price":
		ft := strings.ToUpper(fuelType)
		priceOf := func(v fuelStationView) float64 {
			if ft != "" {
				if p, ok := v.Prices[ft]; ok {
					return p
				}
				return 1e9
			}
			best := 1e9
			for _, p := range v.Prices {
				if p < best {
					best = p
				}
			}
			return best
		}
		sort.SliceStable(vs, func(i, j int) bool { return priceOf(vs[i]) < priceOf(vs[j]) })
	}
}

func newFuelPricesCmd(flags *rootFlags) *cobra.Command {
	var fuelType, brand string
	var snapshotOnly bool
	cmd := &cobra.Command{
		Use:         "prices",
		Short:       "All current NSW fuel prices, filterable by fuel type and brand",
		Long:        "Fetch every current fuel price in NSW. Always records a price snapshot in the local store so `fuel-drift` has history.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel prices --fueltype E10 --json\n  nsw-transport-pp-cli fuel prices --brand \"7-Eleven\" --fueltype U91 --csv", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			pr, err := c.Prices(ctxOf(cmd))
			if err != nil {
				return classifyNSWError(err, flags)
			}
			// Opportunistically snapshot the full price set for fuel-drift.
			if st, serr := nswOpenStore(); serr == nil {
				if _, n, werr := st.SnapshotFuel(pr); werr == nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "snapshot: recorded %d price points\n", n)
				}
				_ = st.Close()
			}
			if snapshotOnly {
				return nil
			}
			views := joinPrices(pr, fuelType, brand)
			sortFuelViews(views, "price", fuelType)
			return emitJSON(cmd, flags, views)
		},
	}
	cmd.Flags().StringVar(&fuelType, "fueltype", "", "Filter to one fuel-type code (E10, U91, P95, P98, DL, ...)")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter to one brand (e.g. \"BP\", \"7-Eleven\")")
	cmd.Flags().BoolVar(&snapshotOnly, "snapshot-only", false, "Record a price snapshot to the local store and print nothing")
	return cmd
}

func parseLatLng(args []string) (lat, lng float64, ok bool) {
	if len(args) >= 2 {
		la, e1 := strconv.ParseFloat(strings.TrimSpace(args[0]), 64)
		ln, e2 := strconv.ParseFloat(strings.TrimSpace(args[1]), 64)
		if e1 == nil && e2 == nil {
			return la, ln, true
		}
	}
	if len(args) == 1 {
		parts := strings.Split(args[0], ",")
		if len(parts) == 2 {
			la, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			ln, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if e1 == nil && e2 == nil {
				return la, ln, true
			}
		}
	}
	return 0, 0, false
}

func newFuelNearCmd(flags *rootFlags) *cobra.Command {
	var fuelType, brand, sortBy, coord string
	var radius float64
	cmd := &cobra.Command{
		Use:   "near --coord <lat,lng> --fueltype <code>",
		Short: "Fuel prices near a coordinate, within a radius, sorted by price or distance",
		Long: "Find fuel prices within a radius (km) of a latitude/longitude.\n\n" +
			"Pass the point via --coord <lat,lng> (negative NSW latitudes confuse positional parsing). Positional `<lat> <lng>` also works if you separate it with `--` (e.g. `fuel near -- -33.87 151.21`).",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel near --coord -33.8688,151.2093 --fueltype E10 --radius 5 --sort price\n  nsw-transport-pp-cli fuel near --coord -33.87,151.21 --fueltype DL --json --select stationcode,prices,distance_km", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,4,5,7,10"},
		Args:        cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if coord == "" && len(args) == 0 {
				return cmd.Help()
			}
			var lat, lng float64
			var ok bool
			if coord != "" {
				lat, lng, ok = parseLatLng([]string{coord})
			} else {
				lat, lng, ok = parseLatLng(args)
			}
			if !ok {
				return usageErr(fmt.Errorf("could not parse a latitude/longitude; pass --coord <lat,lng> (e.g. --coord -33.87,151.21)"))
			}
			if fuelType == "" {
				return usageErr(fmt.Errorf("--fueltype is required (e.g. --fueltype E10)"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			req := fuelcheck.NearbyRequest{
				FuelType: strings.ToUpper(fuelType), Latitude: lat, Longitude: lng, Radius: radius,
				SortBy: pickSortBy(sortBy), SortAscending: true,
			}
			if brand != "" {
				req.Brand = []string{brand}
			}
			pr, err := c.PricesNearby(ctxOf(cmd), req)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			views := joinPrices(pr, fuelType, "")
			sortFuelViews(views, pickSortBy(sortBy), fuelType)
			return emitJSON(cmd, flags, views)
		},
	}
	cmd.Flags().StringVar(&coord, "coord", "", "Coordinate as <lat,lng>, e.g. -33.8688,151.2093")
	cmd.Flags().StringVar(&fuelType, "fueltype", "", "Fuel-type code to price (required)")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter to one brand")
	cmd.Flags().StringVar(&sortBy, "sort", "price", "Sort results by: price | distance")
	cmd.Flags().Float64Var(&radius, "radius", 5, "Search radius in kilometres")
	return cmd
}

func pickSortBy(s string) string {
	if strings.EqualFold(s, "distance") {
		return "distance"
	}
	return "price"
}

func newFuelStationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "station <stationcode>",
		Short:       "Current fuel prices for one station by station code",
		Long:        "Look up current prices for a single FuelCheck station code (the numeric code shown on `fuel prices` / `fuel near`).",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel station 18070 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3,4,5,7,10"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			sr, err := c.PricesByStation(ctxOf(cmd), args[0])
			if err != nil {
				return classifyNSWError(err, flags)
			}
			code := sr.Station.StationCode.String()
			if code == "" {
				code = args[0] // some FuelCheck responses omit the code; echo the request value
			}
			v := fuelStationView{
				StationCode: code, Brand: sr.Station.Brand, Name: sr.Station.Name, Address: sr.Station.Address,
				Latitude: sr.Station.Location.Latitude.Float(), Longitude: sr.Station.Location.Longitude.Float(),
				Prices: map[string]float64{}, LastUpdated: map[string]string{},
			}
			for _, p := range sr.Prices {
				v.Prices[strings.ToUpper(p.FuelType)] = p.Price.Float()
				if p.LastUpdated != "" {
					v.LastUpdated[strings.ToUpper(p.FuelType)] = p.LastUpdated
				}
			}
			return emitJSON(cmd, flags, v)
		},
	}
	return cmd
}

func newFuelByLocationCmd(flags *rootFlags) *cobra.Command {
	var fuelType, suburb, postcode, brand, namedLocation, state string
	cmd := &cobra.Command{
		Use:         "by-location",
		Short:       "Fuel prices for a named location (suburb or postcode)",
		Long:        "Find fuel prices for a suburb, postcode, or named location.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel by-location --suburb Newtown --fueltype E10 --json\n  nsw-transport-pp-cli fuel by-location --postcode 2042 --fueltype U91", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,4,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fuelType == "" {
				return usageErr(fmt.Errorf("--fueltype is required"))
			}
			if suburb == "" && postcode == "" && namedLocation == "" {
				return usageErr(fmt.Errorf("provide one of --suburb, --postcode, or --location"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			// FuelCheck's /prices/location requires a `namedlocation` value;
			// fold whichever of --location/--suburb/--postcode the user gave
			// into it (suburb wins over postcode when both are set).
			loc := firstNonEmpty(namedLocation, suburb, postcode)
			req := fuelcheck.LocationRequest{
				FuelType: strings.ToUpper(fuelType), NamedLocation: loc, StateTerritory: state, Suburb: suburb, Postcode: postcode,
			}
			if brand != "" {
				req.Brands = []string{brand}
			}
			pr, err := c.PricesByLocation(ctxOf(cmd), req)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			views := joinPrices(pr, fuelType, "")
			sortFuelViews(views, "price", fuelType)
			return emitJSON(cmd, flags, views)
		},
	}
	cmd.Flags().StringVar(&fuelType, "fueltype", "", "Fuel-type code (required)")
	cmd.Flags().StringVar(&suburb, "suburb", "", "Suburb name")
	cmd.Flags().StringVar(&postcode, "postcode", "", "Postcode")
	cmd.Flags().StringVar(&namedLocation, "location", "", "Named location string")
	cmd.Flags().StringVar(&state, "state", "NSW", "State/territory (NSW or TAS via the v2 dataset)")
	cmd.Flags().StringVar(&brand, "brand", "", "Filter to one brand")
	return cmd
}

func newFuelTypesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "types",
		Short:       "List valid FuelCheck fuel-type codes",
		Long:        "Fetch the FuelCheck list-of-values and print the valid fuel-type codes.",
		Example:     "  nsw-transport-pp-cli fuel types --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			lr, err := c.Lovs(ctxOf(cmd))
			if err != nil {
				return classifyNSWError(err, flags)
			}
			return emitJSON(cmd, flags, lr.FuelTypes)
		},
	}
	return cmd
}

func newFuelBrandsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "brands",
		Short:       "List FuelCheck fuel brands",
		Long:        "Fetch the FuelCheck list-of-values and print the fuel brands.",
		Example:     "  nsw-transport-pp-cli fuel brands --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			lr, err := c.Lovs(ctxOf(cmd))
			if err != nil {
				return classifyNSWError(err, flags)
			}
			return emitJSON(cmd, flags, lr.Brands)
		},
	}
	return cmd
}

func newFuelTrendsCmd(flags *rootFlags) *cobra.Command {
	var fuelType, period, station string
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Statewide or per-station fuel price trends over a period",
		Long: "Fetch FuelCheck price-trend data. Statewide by default; pass --station <code> for one station.\n\n" +
			"Note: the trends endpoint is documented in the NSW API portal but not cross-validated by community clients; if the response shape differs the raw JSON is printed for you to inspect.",
		Example:     strings.Trim("\n  nsw-transport-pp-cli fuel trends --fueltype E10 --period MONTH --json\n  nsw-transport-pp-cli fuel trends --fueltype DL --station 11820 --period WEEK", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,4,5,7,10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if fuelType == "" {
				return usageErr(fmt.Errorf("--fueltype is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := nswFuelClient(flags) // pp:client-call
			if err != nil {
				return err
			}
			raw, err := c.Trends(ctxOf(cmd), strings.ToUpper(fuelType), period, station)
			if err != nil {
				return classifyNSWError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&fuelType, "fueltype", "", "Fuel-type code (required)")
	cmd.Flags().StringVar(&period, "period", "MONTH", "Trend period: DAY | WEEK | MONTH | YEAR")
	cmd.Flags().StringVar(&station, "station", "", "Station code for a per-station trend (default: statewide)")
	return cmd
}
