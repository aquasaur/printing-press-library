package realtime

import "testing"

func TestDefaultVersion(t *testing.T) {
	cases := map[string]string{
		"sydneytrains":        "v2",
		"metro":               "v2",
		"lightrail":           "v2",
		"lightrail/innerwest": "v2",
		"buses":               "v1",
		"buses/SBSC008":       "v1",
		"ferries":             "v1",
		"nswtrains":           "v1",
		"regionbuses":         "v1",
	}
	for mode, want := range cases {
		if got := DefaultVersion(mode); got != want {
			t.Errorf("DefaultVersion(%q) = %q, want %q", mode, got, want)
		}
	}
}

func TestFeedURL(t *testing.T) {
	got := FeedURL("v2", FeedVehiclePositions, "sydneytrains")
	want := "https://api.transport.nsw.gov.au/v2/gtfs/vehiclepos/sydneytrains"
	if got != want {
		t.Errorf("FeedURL = %q, want %q", got, want)
	}
	// empty version falls back to the per-mode default.
	if got := FeedURL("", FeedTripUpdates, "buses"); got != "https://api.transport.nsw.gov.au/v1/gtfs/realtime/buses" {
		t.Errorf("FeedURL default version = %q", got)
	}
}

func TestScheduleURL(t *testing.T) {
	if got := ScheduleURL("buses/SBSC008"); got != "https://api.transport.nsw.gov.au/v1/gtfs/schedule/buses/SBSC008" {
		t.Errorf("ScheduleURL = %q", got)
	}
}

func TestValidateMode(t *testing.T) {
	if err := validateMode(FeedVehiclePositions, "sydneytrains"); err != nil {
		t.Errorf("expected sydneytrains valid for vehiclepos, got %v", err)
	}
	if err := validateMode(FeedVehiclePositions, "spaceships"); err == nil {
		t.Errorf("expected error for unknown mode")
	}
	if err := validateMode(FeedAlerts, "all"); err != nil {
		t.Errorf("expected 'all' valid for alerts, got %v", err)
	}
	if err := validateMode(FeedTripUpdates, "all"); err == nil {
		t.Errorf("expected 'all' invalid for trip updates")
	}
	// operator suffix is accepted via the root segment.
	if err := validateMode(FeedVehiclePositions, "buses/SBSC008"); err != nil {
		t.Errorf("expected buses/operator valid, got %v", err)
	}
}
