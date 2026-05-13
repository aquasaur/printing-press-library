// Package realtime fetches and decodes the TfNSW GTFS-Realtime protobuf feeds
// (vehicle positions, trip updates, service alerts) and downloads GTFS static
// timetable ZIPs. These feeds are not JSON, so they cannot be expressed in the
// generator's spec; this package is the hand-built client for the printed
// CLI's `realtime ...` commands.
package realtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	gtfs "github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/cliutil"
)

const baseURL = "https://api.transport.nsw.gov.au"

// Feed identifies which GTFS-Realtime feed to fetch.
type Feed string

const (
	FeedVehiclePositions Feed = "vehiclepos"
	FeedTripUpdates      Feed = "realtime"
	FeedAlerts           Feed = "alerts"
)

// Modes that ship a v2 feed; everything else still reads v1.
var v2Modes = map[string]bool{"sydneytrains": true, "metro": true, "lightrail": true}

// ValidModes lists the transport modes accepted on the `realtime ...` commands.
// Some modes accept an `/operator` suffix (e.g. `buses/SBSC008`); the bare mode
// bundles all operators where TfNSW supports it.
var ValidModes = []string{"sydneytrains", "metro", "lightrail", "buses", "ferries", "nswtrains", "regionbuses"}

// alertsAllModes are the modes the alerts feed supports plus "all".
var alertsModes = append([]string{"all"}, ValidModes...)

// DefaultVersion returns "v2" for modes that have a v2 feed, "v1" otherwise.
func DefaultVersion(mode string) string {
	if v2Modes[strings.ToLower(strings.SplitN(mode, "/", 2)[0])] {
		return "v2"
	}
	return "v1"
}

func validateMode(feed Feed, mode string) error {
	root := strings.ToLower(strings.SplitN(mode, "/", 2)[0])
	valid := ValidModes
	if feed == FeedAlerts {
		valid = alertsModes
	}
	for _, m := range valid {
		if m == root {
			return nil
		}
	}
	return fmt.Errorf("unknown mode %q for the %s feed; valid modes: %s", mode, feed, strings.Join(valid, ", "))
}

// FeedURL builds the GTFS-Realtime endpoint URL. version is "v1" or "v2";
// mode may include an `/operator` suffix.
func FeedURL(version string, feed Feed, mode string) string {
	if version == "" {
		version = DefaultVersion(mode)
	}
	return fmt.Sprintf("%s/%s/gtfs/%s/%s", baseURL, version, feed, mode)
}

// ScheduleURL builds the GTFS static timetable ZIP endpoint URL.
func ScheduleURL(mode string) string {
	return fmt.Sprintf("%s/v1/gtfs/schedule/%s", baseURL, mode)
}

// Client wraps an HTTP client with the Open Data Hub API key and a shared
// rate limiter. The TfNSW feeds refresh every 10-15s and throttle aggressive
// polling with HTTP 503, so the limiter floors at one request every ~3s.
type Client struct {
	apiKey  string
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func New(apiKey string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		apiKey:  apiKey,
		http:    &http.Client{Timeout: timeout},
		limiter: cliutil.NewAdaptiveLimiter(0.3),
	}
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("no NSW Open Data Hub API key configured; set NSW_OPENDATA_API_KEY")
	}
	c.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "apikey "+c.apiKey)
	req.Header.Set("Accept", "application/x-google-protobuf")
	req.Header.Set("User-Agent", "nsw-transport-pp-cli")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusOK:
		c.limiter.OnSuccess()
		return body, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp), Body: truncate(body)}
	default:
		return nil, fmt.Errorf("GTFS-Realtime request to %s failed: HTTP %d: %s", url, resp.StatusCode, truncate(body))
	}
}

func (c *Client) feedMessage(ctx context.Context, url string) (*gtfs.FeedMessage, error) {
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var msg gtfs.FeedMessage
	if err := proto.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("decoding GTFS-Realtime protobuf from %s: %w", url, err)
	}
	return &msg, nil
}

// Vehicle is the flattened vehicle-position record returned to the CLI.
type Vehicle struct {
	VehicleID string   `json:"vehicle_id,omitempty"`
	Label     string   `json:"label,omitempty"`
	TripID    string   `json:"trip_id,omitempty"`
	RouteID   string   `json:"route_id,omitempty"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Bearing   *float32 `json:"bearing,omitempty"`
	Speed     *float32 `json:"speed,omitempty"`
	StopID    string   `json:"stop_id,omitempty"`
	Status    string   `json:"current_status,omitempty"`
	Occupancy string   `json:"occupancy,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
}

// Vehicles fetches and flattens the vehicle-position feed for a mode.
func (c *Client) Vehicles(ctx context.Context, version, mode string) ([]Vehicle, error) {
	if err := validateMode(FeedVehiclePositions, mode); err != nil {
		return nil, err
	}
	msg, err := c.feedMessage(ctx, FeedURL(version, FeedVehiclePositions, mode))
	if err != nil {
		return nil, err
	}
	var out []Vehicle
	for _, e := range msg.GetEntity() {
		vp := e.GetVehicle()
		if vp == nil {
			continue
		}
		pos := vp.GetPosition()
		v := Vehicle{
			VehicleID: vp.GetVehicle().GetId(),
			Label:     vp.GetVehicle().GetLabel(),
			TripID:    vp.GetTrip().GetTripId(),
			RouteID:   vp.GetTrip().GetRouteId(),
			StopID:    vp.GetStopId(),
			Status:    vp.GetCurrentStatus().String(),
			Occupancy: vp.GetOccupancyStatus().String(),
		}
		if pos != nil {
			v.Latitude = float64(pos.GetLatitude())
			v.Longitude = float64(pos.GetLongitude())
			if pos.Bearing != nil {
				b := pos.GetBearing()
				v.Bearing = &b
			}
			if pos.Speed != nil {
				s := pos.GetSpeed()
				v.Speed = &s
			}
		}
		if ts := vp.GetTimestamp(); ts > 0 {
			v.Timestamp = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RouteID < out[j].RouteID })
	return out, nil
}

// TripUpdate is a flattened trip-update record.
type TripUpdate struct {
	TripID         string             `json:"trip_id,omitempty"`
	RouteID        string             `json:"route_id,omitempty"`
	ScheduleRel    string             `json:"schedule_relationship,omitempty"`
	VehicleID      string             `json:"vehicle_id,omitempty"`
	Timestamp      string             `json:"timestamp,omitempty"`
	MaxDelaySec    *int32             `json:"max_delay_seconds,omitempty"`
	StopTimeCount  int                `json:"stop_time_update_count"`
	NextStopUpdate *StopTimeUpdateRec `json:"next_stop_update,omitempty"`
}

type StopTimeUpdateRec struct {
	StopID         string `json:"stop_id,omitempty"`
	StopSequence   uint32 `json:"stop_sequence,omitempty"`
	ArrivalDelay   *int32 `json:"arrival_delay_seconds,omitempty"`
	DepartureDelay *int32 `json:"departure_delay_seconds,omitempty"`
}

// TripUpdates fetches and flattens the trip-update feed for a mode.
func (c *Client) TripUpdates(ctx context.Context, version, mode string) ([]TripUpdate, error) {
	if err := validateMode(FeedTripUpdates, mode); err != nil {
		return nil, err
	}
	msg, err := c.feedMessage(ctx, FeedURL(version, FeedTripUpdates, mode))
	if err != nil {
		return nil, err
	}
	var out []TripUpdate
	for _, e := range msg.GetEntity() {
		tu := e.GetTripUpdate()
		if tu == nil {
			continue
		}
		rec := TripUpdate{
			TripID:        tu.GetTrip().GetTripId(),
			RouteID:       tu.GetTrip().GetRouteId(),
			ScheduleRel:   tu.GetTrip().GetScheduleRelationship().String(),
			VehicleID:     tu.GetVehicle().GetId(),
			StopTimeCount: len(tu.GetStopTimeUpdate()),
		}
		if ts := tu.GetTimestamp(); ts > 0 {
			rec.Timestamp = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
		}
		var maxDelay *int32
		for _, stu := range tu.GetStopTimeUpdate() {
			for _, d := range []*int32{ptrDelay(stu.GetArrival()), ptrDelay(stu.GetDeparture())} {
				if d != nil && (maxDelay == nil || abs32(*d) > abs32(*maxDelay)) {
					maxDelay = d
				}
			}
		}
		rec.MaxDelaySec = maxDelay
		if stus := tu.GetStopTimeUpdate(); len(stus) > 0 {
			s := stus[0]
			rec.NextStopUpdate = &StopTimeUpdateRec{
				StopID:         s.GetStopId(),
				StopSequence:   s.GetStopSequence(),
				ArrivalDelay:   ptrDelay(s.GetArrival()),
				DepartureDelay: ptrDelay(s.GetDeparture()),
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

// Alert is a flattened service-alert record.
type Alert struct {
	ID             string   `json:"id,omitempty"`
	Cause          string   `json:"cause,omitempty"`
	Effect         string   `json:"effect,omitempty"`
	Header         string   `json:"header,omitempty"`
	Description    string   `json:"description,omitempty"`
	URL            string   `json:"url,omitempty"`
	AffectedRoutes []string `json:"affected_routes,omitempty"`
	AffectedStops  []string `json:"affected_stops,omitempty"`
	ActiveFrom     string   `json:"active_from,omitempty"`
	ActiveTo       string   `json:"active_to,omitempty"`
}

// Alerts fetches and flattens the service-alerts feed for a mode ("all" allowed).
func (c *Client) Alerts(ctx context.Context, mode string) ([]Alert, error) {
	if err := validateMode(FeedAlerts, mode); err != nil {
		return nil, err
	}
	// The alerts feed only ships under v2.
	msg, err := c.feedMessage(ctx, FeedURL("v2", FeedAlerts, mode))
	if err != nil {
		// Fall back to v1 for older mode paths.
		msg, err = c.feedMessage(ctx, FeedURL("v1", FeedAlerts, mode))
		if err != nil {
			return nil, err
		}
	}
	var out []Alert
	for _, e := range msg.GetEntity() {
		a := e.GetAlert()
		if a == nil {
			continue
		}
		rec := Alert{
			ID:          e.GetId(),
			Cause:       a.GetCause().String(),
			Effect:      a.GetEffect().String(),
			Header:      firstTranslation(a.GetHeaderText()),
			Description: firstTranslation(a.GetDescriptionText()),
			URL:         firstTranslation(a.GetUrl()),
		}
		for _, ie := range a.GetInformedEntity() {
			if r := ie.GetRouteId(); r != "" {
				rec.AffectedRoutes = append(rec.AffectedRoutes, r)
			}
			if s := ie.GetStopId(); s != "" {
				rec.AffectedStops = append(rec.AffectedStops, s)
			}
		}
		if periods := a.GetActivePeriod(); len(periods) > 0 {
			if s := periods[0].GetStart(); s > 0 {
				rec.ActiveFrom = time.Unix(int64(s), 0).UTC().Format(time.RFC3339)
			}
			if e := periods[0].GetEnd(); e > 0 {
				rec.ActiveTo = time.Unix(int64(e), 0).UTC().Format(time.RFC3339)
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

// DownloadSchedule fetches the GTFS static timetable ZIP for a mode and writes
// it to outPath. Returns the number of bytes written.
func (c *Client) DownloadSchedule(ctx context.Context, mode, outPath string) (int64, error) {
	body, err := c.get(ctx, ScheduleURL(mode))
	if err != nil {
		return 0, err
	}
	return writeFile(outPath, body)
}

func firstTranslation(ts *gtfs.TranslatedString) string {
	if ts == nil {
		return ""
	}
	for _, t := range ts.GetTranslation() {
		if t.GetText() != "" {
			return strings.TrimSpace(t.GetText())
		}
	}
	return ""
}

func ptrDelay(ev *gtfs.TripUpdate_StopTimeEvent) *int32 {
	if ev == nil || ev.Delay == nil {
		return nil
	}
	d := ev.GetDelay()
	return &d
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func truncate(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
