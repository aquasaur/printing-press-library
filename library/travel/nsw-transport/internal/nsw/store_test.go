package nsw

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
)

func mkStation(code, brand, name string, _, _ float64) fuelcheck.Station {
	return fuelcheck.Station{StationCode: fuelcheck.FlexStr(code), Brand: brand, Name: name}
}

func TestFuelDrift(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Not enough history yet.
	if _, err := st.FuelDrift("E10", "", true); err != ErrNoHistory {
		t.Fatalf("expected ErrNoHistory with zero snapshots, got %v", err)
	}

	stations := []fuelcheck.Station{
		mkStation("100", "BP", "BP Newtown", -33.9, 151.2),
		mkStation("200", "7-Eleven", "7-Eleven Camperdown", -33.89, 151.18),
	}
	// First snapshot.
	if _, _, err := st.SnapshotFuel(&fuelcheck.PricesResponse{
		Stations: stations,
		Prices: []fuelcheck.Price{
			{StationCode: "100", FuelType: "E10", Price: 189.9},
			{StationCode: "200", FuelType: "E10", Price: 195.5},
		},
	}); err != nil {
		t.Fatalf("snapshot 1: %v", err)
	}
	// Still only one snapshot.
	if _, err := st.FuelDrift("E10", "", true); err != ErrNoHistory {
		t.Fatalf("expected ErrNoHistory with one snapshot, got %v", err)
	}

	// Force a distinct timestamp for the second snapshot by writing directly.
	if _, err := st.DB().Exec(`INSERT INTO nsw_fuel_snapshots(stationcode,fueltype,price,last_updated,snapshot_at) VALUES
		('100','E10',184.9,'',1000),('200','E10',196.0,'',1000)`); err != nil {
		t.Fatalf("manual snapshot insert: %v", err)
	}
	// And the original snapshot needs an earlier timestamp; rewrite it.
	if _, err := st.DB().Exec(`UPDATE nsw_fuel_snapshots SET snapshot_at=500 WHERE snapshot_at != 1000`); err != nil {
		t.Fatalf("rewrite ts: %v", err)
	}

	rows, err := st.FuelDrift("E10", "", true)
	if err != nil {
		t.Fatalf("FuelDrift: %v", err)
	}
	// Only stations whose price changed: 100 (189.9 -> 184.9 = -5.0) and 200 (195.5 -> 196.0 = +0.5).
	if len(rows) != 2 {
		t.Fatalf("expected 2 changed rows, got %d: %+v", len(rows), rows)
	}
	// Sorted ascending by change: the -5.0 drop comes first.
	if rows[0].StationCode != "100" || rows[0].ChangeC != -5.0 {
		t.Errorf("row[0] = %+v, want station 100 change -5.0", rows[0])
	}
	if rows[1].StationCode != "200" || rows[1].ChangeC != 0.5 {
		t.Errorf("row[1] = %+v, want station 200 change +0.5", rows[1])
	}
	if rows[0].Brand != "BP" {
		t.Errorf("row[0] brand = %q, want BP", rows[0].Brand)
	}

	// Brand filter.
	bp, err := st.FuelDrift("E10", "BP", true)
	if err != nil {
		t.Fatalf("FuelDrift brand: %v", err)
	}
	if len(bp) != 1 || bp[0].StationCode != "100" {
		t.Fatalf("brand filter = %+v, want only station 100", bp)
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		1.234:  1.23,
		1.236:  1.24,
		-5.001: -5.0,
		-5.008: -5.01,
		0:      0,
	}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}
