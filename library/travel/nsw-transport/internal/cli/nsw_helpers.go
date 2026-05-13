package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/geo"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/nsw"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/realtime"
)

const (
	tpBase    = "https://api.transport.nsw.gov.au/v1/tp"
	liveBase  = "https://api.transport.nsw.gov.au/v1/live"
	cliDBName = "nsw-transport-pp-cli"
)

// hazardCategories are the categories the location-intelligence commands scan
// for "hazards near X". regional-lga-participation is intentionally excluded —
// it lists councils, not hazards.
var hazardCategories = []string{"incident", "fire", "flood", "alpine", "roadwork", "majorevent", "regional-lga-incident"}

func nswOpenDataKey() (string, error) {
	cfg, err := config.Load("")
	if err != nil {
		return "", configErr(err)
	}
	if cfg.NswOpendataApiKey == "" {
		return "", authErr(fmt.Errorf("no NSW Open Data Hub API key configured; set NSW_OPENDATA_API_KEY (get one at https://opendata.transport.nsw.gov.au/data/user/register)"))
	}
	return cfg.NswOpendataApiKey, nil
}

func nswRealtimeClient(flags *rootFlags) (*realtime.Client, error) {
	key, err := nswOpenDataKey()
	if err != nil {
		return nil, err
	}
	return realtime.New(key, flags.timeout), nil
}

func nswFuelClient(flags *rootFlags) (*fuelcheck.Client, error) {
	// FuelCheck creds resolve from config file or the NSW_FUELCHECK_API_KEY /
	// NSW_FUELCHECK_API_SECRET env vars (env wins; config.Load already merges).
	key, secret := "", ""
	if cfg, cerr := config.Load(flags.configPath); cerr == nil {
		key, secret = cfg.FuelcheckApiKey, cfg.FuelcheckApiSecret
	}
	c, err := fuelcheck.NewWithCreds(key, secret, flags.timeout)
	if err != nil {
		if _, ok := err.(fuelcheck.MissingCredsError); ok {
			return nil, configErr(err)
		}
		return nil, err
	}
	return c, nil
}

func nswOpenStore() (*nsw.Store, error) {
	st, err := nsw.Open(defaultDBPath(cliDBName))
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store: %w", err))
	}
	return st, nil
}

// classifyNSWError maps source-package errors to the CLI's typed exit codes.
func classifyNSWError(err error, flags *rootFlags) error {
	if err == nil {
		return nil
	}
	var rl *cliutil.RateLimitError
	if As(err, &rl) {
		return rateLimitErr(err)
	}
	if _, ok := err.(fuelcheck.MissingCredsError); ok {
		return configErr(err)
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "HTTP 401"), strings.Contains(msg, "HTTP 403"), strings.Contains(msg, "rejected the credentials"), strings.Contains(msg, "no NSW Open Data Hub API key"):
		return authErr(err)
	case strings.Contains(msg, "HTTP 404"):
		return notFoundErr(err)
	default:
		return apiErr(err)
	}
}

// --- Trip Planner (rapidJSON) helpers ---

func tpGet(c *client.Client, path string, params map[string]string) (json.RawMessage, error) {
	full := tpBase + path
	if params == nil {
		params = map[string]string{}
	}
	params["outputFormat"] = "rapidJSON"
	params["coordOutputFormat"] = "EPSG:4326"
	return c.Get(full, params)
}

type tpLocation struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Coord        []float64 `json:"coord"`
	Distance     float64   `json:"distance,omitempty"`
	MatchQuality int       `json:"matchQuality,omitempty"`
	IsBest       bool      `json:"isBest,omitempty"`
}

func (l tpLocation) latLng() (lat, lng float64, ok bool) {
	return coordLatLng(l.Coord)
}

// transitStopType reports whether a stop_finder result is an actual public-
// transport stop/platform (as opposed to a street, address, or POI).
func transitStopType(t string) bool {
	switch strings.ToLower(t) {
	case "stop", "platform":
		return true
	}
	return false
}

// pickBestLocation chooses the most relevant stop_finder result for query:
// an exact ID match wins; otherwise prefer a transit stop/platform, then the
// API's isBest flag, then the highest matchQuality, then the first result.
func pickBestLocation(locs []tpLocation, query string) tpLocation {
	for _, l := range locs {
		if l.ID == query {
			return l
		}
	}
	best := locs[0]
	for _, l := range locs[1:] {
		if betterLocation(l, best) {
			best = l
		}
	}
	return best
}

func betterLocation(a, b tpLocation) bool {
	if transitStopType(a.Type) != transitStopType(b.Type) {
		return transitStopType(a.Type)
	}
	if a.IsBest != b.IsBest {
		return a.IsBest
	}
	return a.MatchQuality > b.MatchQuality
}

// coordLatLng extracts (lat, lng) from a 2-element coordinate slice regardless
// of order: NSW longitudes are far outside the [-90,90] latitude band, so the
// element in that band is the latitude.
func coordLatLng(c []float64) (lat, lng float64, ok bool) {
	if len(c) < 2 {
		return 0, 0, false
	}
	a, b := c[0], c[1]
	if a >= -90 && a <= 90 && (b < -90 || b > 90) {
		return a, b, true
	}
	if b >= -90 && b <= 90 {
		return b, a, true
	}
	// Both look like latitudes (unlikely for NSW) — assume [lat, lng].
	return a, b, true
}

// resolveStopCoord looks up a stop by ID (or name) via stop_finder and returns
// its display name and coordinates.
func resolveStopCoord(c *client.Client, idOrName string) (name string, lat, lng float64, err error) {
	raw, err := tpGet(c, "/stop_finder", map[string]string{
		"type_sf": "any",
		"name_sf": idOrName,
		"TfNSWSF": "true",
	})
	if err != nil {
		return "", 0, 0, err
	}
	var resp struct {
		Locations []tpLocation `json:"locations"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", 0, 0, fmt.Errorf("decoding stop_finder response: %w", err)
	}
	if len(resp.Locations) == 0 {
		return "", 0, 0, notFoundErr(fmt.Errorf("no stop or place found matching %q", idOrName))
	}
	pick := pickBestLocation(resp.Locations, idOrName)
	la, ln, okc := pick.latLng()
	if !okc {
		return pick.Name, 0, 0, fmt.Errorf("stop %q has no coordinates", idOrName)
	}
	return pick.Name, la, ln, nil
}

// --- Live Traffic helpers ---

type hazardFeature struct {
	ID           json.Number `json:"id,omitempty"`
	Headline     string      `json:"headline,omitempty"`
	DisplayName  string      `json:"displayName,omitempty"`
	MainCategory string      `json:"mainCategory,omitempty"`
	SubCategory  string      `json:"subCategory,omitempty"`
	Roads        []struct {
		Region   string `json:"region,omitempty"`
		RoadName string `json:"roadName,omitempty"`
		Suburb   string `json:"suburb,omitempty"`
	} `json:"roads,omitempty"`
	Created     string  `json:"created,omitempty"`
	LastUpdated string  `json:"lastUpdated,omitempty"`
	Start       string  `json:"start,omitempty"`
	End         string  `json:"end,omitempty"`
	IsMajor     bool    `json:"isMajor,omitempty"`
	IsEnded     bool    `json:"isEnded,omitempty"`
	OtherAdvice string  `json:"otherAdvice,omitempty"`
	WebURL      string  `json:"webUrl,omitempty"`
	Category    string  `json:"category"` // path category we fetched it from
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	DistanceKm  float64 `json:"distance_km,omitempty"` // populated by proximity filters
}

type geoFeatureCollection struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties json.RawMessage `json:"properties"`
	} `json:"features"`
}

// fetchHazards fetches one category/status combination and flattens the GeoJSON.
func fetchHazards(c *client.Client, category, status string) ([]hazardFeature, error) {
	if status == "" {
		status = "all"
	}
	raw, err := c.Get(fmt.Sprintf("%s/hazards/%s/%s", liveBase, category, status), nil)
	if err != nil {
		return nil, err
	}
	var fc geoFeatureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("decoding hazards GeoJSON: %w", err)
	}
	out := make([]hazardFeature, 0, len(fc.Features))
	for _, f := range fc.Features {
		var h hazardFeature
		_ = json.Unmarshal(f.Properties, &h)
		// RMS advice fields arrive as HTML fragments (<p>...</p>, <br/>).
		// Flatten to plain text so JSON/--agent consumers don't get HTML
		// soup inside a string field.
		h.OtherAdvice = htmlToPlainText(h.OtherAdvice)
		h.Headline = htmlToPlainText(h.Headline)
		h.Category = category
		if lat, lng, ok := coordLatLng(f.Geometry.Coordinates); ok {
			h.Latitude, h.Longitude = lat, lng
		}
		out = append(out, h)
	}
	return out, nil
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

// htmlToPlainText flattens a small HTML fragment (the shape RMS uses for
// hazard advice: <p>, <br/>, <a>) into readable plain text: block tags
// become spaces, the rest are stripped, entities are decoded, and runs of
// whitespace collapse to a single space.
func htmlToPlainText(s string) string {
	if s == "" || !strings.ContainsRune(s, '<') {
		return s
	}
	s = htmlTagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// fetchAllOpenHazards fetches the "open" set across every hazard category.
func fetchAllOpenHazards(c *client.Client) ([]hazardFeature, error) {
	var all []hazardFeature
	var firstErr error
	for _, cat := range hazardCategories {
		hs, err := fetchHazards(c, cat, "open")
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		all = append(all, hs...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

type cameraFeature struct {
	ID         string  `json:"id,omitempty"`
	Region     string  `json:"region,omitempty"`
	Title      string  `json:"title,omitempty"`
	View       string  `json:"view,omitempty"`
	Direction  string  `json:"direction,omitempty"`
	Href       string  `json:"href,omitempty"`
	Latitude   float64 `json:"latitude,omitempty"`
	Longitude  float64 `json:"longitude,omitempty"`
	DistanceKm float64 `json:"distance_km,omitempty"`
}

func fetchCameras(c *client.Client) ([]cameraFeature, error) {
	raw, err := c.Get(liveBase+"/cameras", nil)
	if err != nil {
		return nil, err
	}
	var fc geoFeatureCollection
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, fmt.Errorf("decoding cameras GeoJSON: %w", err)
	}
	out := make([]cameraFeature, 0, len(fc.Features))
	for _, f := range fc.Features {
		var cf cameraFeature
		_ = json.Unmarshal(f.Properties, &cf)
		if lat, lng, ok := coordLatLng(f.Geometry.Coordinates); ok {
			cf.Latitude, cf.Longitude = lat, lng
		}
		out = append(out, cf)
	}
	return out, nil
}

// camerasWithin returns cameras within radiusKm of (lat,lng), nearest first.
func camerasWithin(cams []cameraFeature, lat, lng, radiusKm float64) []cameraFeature {
	var out []cameraFeature
	for _, cf := range cams {
		if cf.Latitude == 0 && cf.Longitude == 0 {
			continue
		}
		d := geo.Haversine(lat, lng, cf.Latitude, cf.Longitude)
		if d <= radiusKm {
			cf.DistanceKm = round2(d)
			out = append(out, cf)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
	return out
}

// hazardsWithin returns hazards within radiusKm of (lat,lng), nearest first.
func hazardsWithin(hs []hazardFeature, lat, lng, radiusKm float64) []hazardFeature {
	var out []hazardFeature
	for _, h := range hs {
		if h.Latitude == 0 && h.Longitude == 0 {
			continue
		}
		d := geo.Haversine(lat, lng, h.Latitude, h.Longitude)
		if d <= radiusKm {
			h.DistanceKm = round2(d)
			out = append(out, h)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsMajor != out[j].IsMajor {
			return out[i].IsMajor
		}
		return out[i].DistanceKm < out[j].DistanceKm
	})
	return out
}

func round2(v float64) float64 {
	if v < 0 {
		return float64(int64(v*100-0.5)) / 100
	}
	return float64(int64(v*100+0.5)) / 100
}

// emitJSON is the standard output path for hand-written commands that build a
// typed value: routes through printJSONFiltered so --json/--select/--compact/
// --csv/--quiet all behave like the generated commands.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// ctx returns the command context, never nil.
func ctxOf(cmd *cobra.Command) context.Context {
	if c := cmd.Context(); c != nil {
		return c
	}
	return context.Background()
}
