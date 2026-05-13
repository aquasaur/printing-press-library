package cli

import (
	"reflect"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
)

func TestCoordLatLng(t *testing.T) {
	cases := []struct {
		name    string
		in      []float64
		wantLat float64
		wantLng float64
		wantOK  bool
	}{
		{"lng,lat order (rapidJSON Sydney)", []float64{151.206, -33.883}, -33.883, 151.206, true},
		{"lat,lng order", []float64{-33.883, 151.206}, -33.883, 151.206, true},
		{"too short", []float64{151.0}, 0, 0, false},
		{"empty", nil, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lat, lng, ok := coordLatLng(c.in)
			if ok != c.wantOK || (ok && (lat != c.wantLat || lng != c.wantLng)) {
				t.Fatalf("coordLatLng(%v) = (%v,%v,%v), want (%v,%v,%v)", c.in, lat, lng, ok, c.wantLat, c.wantLng, c.wantOK)
			}
		})
	}
}

func TestRouteMatches(t *testing.T) {
	yes := [][2]string{
		{"333", "333"},
		{"2441_333", "333"},
		{"333_x", "333"},
		{"x_333_y", "333"},
		{"T1", "t1"},
	}
	no := [][2]string{
		{"3333", "333"},
		{"433", "333"},
		{"T12", "T1"},
	}
	for _, c := range yes {
		if !routeMatches(c[0], c[1]) {
			t.Errorf("routeMatches(%q,%q) = false, want true", c[0], c[1])
		}
	}
	for _, c := range no {
		if routeMatches(c[0], c[1]) {
			t.Errorf("routeMatches(%q,%q) = true, want false", c[0], c[1])
		}
	}
}

func TestParseLatLng(t *testing.T) {
	cases := []struct {
		args    []string
		wantOK  bool
		wantLat float64
		wantLng float64
	}{
		{[]string{"-33.8688", "151.2093"}, true, -33.8688, 151.2093},
		{[]string{"-33.87,151.21"}, true, -33.87, 151.21},
		{[]string{"not a coord"}, false, 0, 0},
		{nil, false, 0, 0},
	}
	for _, c := range cases {
		lat, lng, ok := parseLatLng(c.args)
		if ok != c.wantOK || (ok && (lat != c.wantLat || lng != c.wantLng)) {
			t.Errorf("parseLatLng(%v) = (%v,%v,%v), want (%v,%v,%v)", c.args, lat, lng, ok, c.wantLat, c.wantLng, c.wantOK)
		}
	}
}

func TestJoinPrices(t *testing.T) {
	st1 := fuelcheck.Station{StationCode: "1", Brand: "BP", Name: "BP A"}
	st1.Location.Latitude, st1.Location.Longitude = -33.9, 151.2
	st2 := fuelcheck.Station{StationCode: "2", Brand: "7-Eleven", Name: "7-Eleven B"}
	pr := &fuelcheck.PricesResponse{
		Stations: []fuelcheck.Station{st1, st2},
		Prices: []fuelcheck.Price{
			{StationCode: "1", FuelType: "E10", Price: 189.9},
			{StationCode: "1", FuelType: "U91", Price: 199.9},
			{StationCode: "2", FuelType: "U91", Price: 197.5},
		},
	}
	// No filter: both stations, all prices.
	all := joinPrices(pr, "", "")
	if len(all) != 2 {
		t.Fatalf("joinPrices no filter = %d stations, want 2", len(all))
	}
	if all[0].Prices["E10"] != 189.9 || all[0].Prices["U91"] != 199.9 {
		t.Errorf("station 1 prices wrong: %+v", all[0].Prices)
	}
	// Fuel-type filter drops station 2 (it has no E10).
	e10 := joinPrices(pr, "E10", "")
	if len(e10) != 1 || e10[0].StationCode != "1" {
		t.Fatalf("joinPrices E10 = %+v, want only station 1", e10)
	}
	// Brand filter.
	bp := joinPrices(pr, "", "BP")
	if len(bp) != 1 || bp[0].Brand != "BP" {
		t.Fatalf("joinPrices brand BP = %+v", bp)
	}
}

func TestSortFuelViewsByPrice(t *testing.T) {
	vs := []fuelStationView{
		{StationCode: "a", Prices: map[string]float64{"E10": 195.0}},
		{StationCode: "b", Prices: map[string]float64{"E10": 189.9}},
		{StationCode: "c", Prices: map[string]float64{"E10": 192.0}},
	}
	sortFuelViews(vs, "price", "E10")
	got := []string{vs[0].StationCode, vs[1].StationCode, vs[2].StationCode}
	if !reflect.DeepEqual(got, []string{"b", "c", "a"}) {
		t.Fatalf("sortFuelViews by E10 price = %v, want [b c a]", got)
	}
}

func TestDedupeDisruptions(t *testing.T) {
	in := []disruption{
		{Source: "gtfs-realtime", Header: "Track work T1", AffectedLines: []string{"T1"}},
		{Source: "trip-planner", Header: "Track work T1", AffectedLines: []string{"T1"}},
		{Source: "trip-planner", Header: "Lift outage Central", AffectedLines: nil},
	}
	out := dedupeDisruptions(in)
	if len(out) != 2 {
		t.Fatalf("dedupeDisruptions = %d, want 2: %+v", len(out), out)
	}
}

func TestSeverityRank(t *testing.T) {
	if severityRank("HIGH") >= severityRank("normal") {
		t.Error("HIGH should rank before normal")
	}
	if severityRank("very_high") != 0 {
		t.Error("very_high should be rank 0")
	}
	if severityRank("anything else") != severityRank("normal") {
		t.Error("unknown severity should default to normal rank")
	}
}

func TestMatchLineAlerts(t *testing.T) {
	raw := []byte(`{"infos":{"current":[
		{"title":"T1 delays","content":"signal fault","priority":"high","affectedLines":[{"disassembledName":"T1"}]},
		{"title":"M1 metro ok","content":"","priority":"normal","affectedLines":[{"disassembledName":"M1"}]}
	]}}`)
	got := matchLineAlerts(raw, []string{"T1", "B61"})
	if len(got) != 1 || got[0].Title != "T1 delays" {
		t.Fatalf("matchLineAlerts = %+v, want only the T1 alert", got)
	}
	if got[0].Lines[0] != "T1" {
		t.Errorf("matched line = %v, want [T1]", got[0].Lines)
	}
	if matchLineAlerts(raw, nil) != nil {
		t.Error("matchLineAlerts with no lines should return nil")
	}
}

func TestParseFirstJourney(t *testing.T) {
	raw := []byte(`{"journeys":[{"legs":[
		{"duration":600,"transportation":{"disassembledName":"T3","destination":{"name":"Liverpool"}},"origin":{"name":"Central","departureTimePlanned":"2026-05-13T08:15:00Z"},"destination":{"name":"Sydenham","arrivalTimePlanned":"2026-05-13T08:25:00Z"}},
		{"duration":300,"transportation":{"disassembledName":"T4"},"origin":{"name":"Sydenham"},"destination":{"name":"Bondi Junction"}}
	]}]}`)
	legs, lines, mins := parseFirstJourney(raw)
	if len(legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(legs))
	}
	if mins != 15 {
		t.Errorf("mins = %d, want 15", mins)
	}
	if !reflect.DeepEqual(lines, []string{"T3", "T4"}) {
		t.Errorf("lines = %v, want [T3 T4]", lines)
	}
	if legs[0].Line != "T3" || legs[0].OriginName != "Central" {
		t.Errorf("leg[0] = %+v", legs[0])
	}
}
