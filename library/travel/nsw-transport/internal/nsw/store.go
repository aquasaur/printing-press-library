// Package nsw holds the printed CLI's local SQLite store for the
// location-intelligence commands. The generator's own store covers the
// spec-driven resources; this store adds the cross-source tables those
// commands need — most importantly fuel-price snapshots over time, which
// `fuel-drift` diffs and which no FuelCheck endpoint exposes directly.
package nsw

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/travel/nsw-transport/internal/source/fuelcheck"
)

// Store wraps the SQLite connection used by the location-intelligence commands.
type Store struct {
	db   *sql.DB
	path string
}

const schema = `
CREATE TABLE IF NOT EXISTS nsw_stations (
	stationcode TEXT PRIMARY KEY,
	brand TEXT,
	name TEXT,
	address TEXT,
	state TEXT,
	latitude REAL,
	longitude REAL,
	updated_at INTEGER
);
CREATE TABLE IF NOT EXISTS nsw_fuel_snapshots (
	stationcode TEXT,
	fueltype TEXT,
	price REAL,
	last_updated TEXT,
	snapshot_at INTEGER,
	PRIMARY KEY (stationcode, fueltype, snapshot_at)
);
CREATE INDEX IF NOT EXISTS idx_snap_type_time ON nsw_fuel_snapshots(fueltype, snapshot_at);
`

// Open opens (creating if needed) the SQLite store and applies the schema.
// dbPath is the same data.db the generated store uses; the nsw_* tables live
// alongside the generated tables in that file.
func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("applying nsw schema: %w", err)
	}
	return &Store{db: db, path: dbPath}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying connection for ad-hoc queries.
func (s *Store) DB() *sql.DB { return s.db }

// SnapshotFuel records a fresh price snapshot for every (station, fueltype)
// pair in the FuelCheck response and upserts the station metadata. Returns the
// snapshot timestamp and the number of price rows written.
func (s *Store) SnapshotFuel(pr *fuelcheck.PricesResponse) (time.Time, int, error) {
	now := time.Now().UTC()
	ts := now.Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return now, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmtStation, err := tx.Prepare(`INSERT INTO nsw_stations(stationcode,brand,name,address,state,latitude,longitude,updated_at)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(stationcode) DO UPDATE SET brand=excluded.brand,name=excluded.name,address=excluded.address,state=excluded.state,latitude=excluded.latitude,longitude=excluded.longitude,updated_at=excluded.updated_at`)
	if err != nil {
		return now, 0, err
	}
	defer stmtStation.Close()
	for _, st := range pr.Stations {
		if _, err := stmtStation.Exec(st.StationCode.String(), st.Brand, st.Name, st.Address, st.State, st.Location.Latitude.Float(), st.Location.Longitude.Float(), ts); err != nil {
			return now, 0, err
		}
	}

	stmtPrice, err := tx.Prepare(`INSERT OR REPLACE INTO nsw_fuel_snapshots(stationcode,fueltype,price,last_updated,snapshot_at) VALUES(?,?,?,?,?)`)
	if err != nil {
		return now, 0, err
	}
	defer stmtPrice.Close()
	n := 0
	for _, p := range pr.Prices {
		if _, err := stmtPrice.Exec(p.StationCode.String(), p.FuelType, p.Price.Float(), p.LastUpdated, ts); err != nil {
			return now, 0, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return now, 0, err
	}
	return now, n, nil
}

// DriftRow describes a station's price movement for one fuel type between the
// two most recent snapshots.
type DriftRow struct {
	StationCode string  `json:"stationcode"`
	Brand       string  `json:"brand,omitempty"`
	Name        string  `json:"name,omitempty"`
	Address     string  `json:"address,omitempty"`
	FuelType    string  `json:"fueltype"`
	PreviousC   float64 `json:"previous_price"`
	CurrentC    float64 `json:"current_price"`
	ChangeC     float64 `json:"change"`
	PreviousAt  string  `json:"previous_snapshot_at"`
	CurrentAt   string  `json:"current_snapshot_at"`
}

// snapshotTimes returns the distinct snapshot timestamps for a fuel type,
// newest first.
func (s *Store) snapshotTimes(fueltype string) ([]int64, error) {
	rows, err := s.db.Query(`SELECT DISTINCT snapshot_at FROM nsw_fuel_snapshots WHERE fueltype=? ORDER BY snapshot_at DESC`, fueltype)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FuelDrift compares the two most recent snapshots for a fuel type and returns
// the per-station change. When brand is non-empty only that brand is returned.
// onlyChanged drops rows whose price did not move. Returns ErrNoHistory when
// there are fewer than two snapshots.
func (s *Store) FuelDrift(fueltype, brand string, onlyChanged bool) ([]DriftRow, error) {
	times, err := s.snapshotTimes(fueltype)
	if err != nil {
		return nil, err
	}
	if len(times) < 2 {
		return nil, ErrNoHistory
	}
	cur, prev := times[0], times[1]

	q := `SELECT c.stationcode, COALESCE(s.brand,''), COALESCE(s.name,''), COALESCE(s.address,''),
		p.price AS prev_price, c.price AS cur_price
		FROM nsw_fuel_snapshots c
		JOIN nsw_fuel_snapshots p ON p.stationcode=c.stationcode AND p.fueltype=c.fueltype AND p.snapshot_at=?
		LEFT JOIN nsw_stations s ON s.stationcode=c.stationcode
		WHERE c.fueltype=? AND c.snapshot_at=?`
	args := []any{prev, fueltype, cur}
	if brand != "" {
		q += ` AND s.brand=?`
		args = append(args, brand)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DriftRow
	for rows.Next() {
		var r DriftRow
		if err := rows.Scan(&r.StationCode, &r.Brand, &r.Name, &r.Address, &r.PreviousC, &r.CurrentC); err != nil {
			return nil, err
		}
		r.FuelType = fueltype
		r.ChangeC = round2(r.CurrentC - r.PreviousC)
		r.PreviousAt = time.Unix(prev, 0).UTC().Format(time.RFC3339)
		r.CurrentAt = time.Unix(cur, 0).UTC().Format(time.RFC3339)
		if onlyChanged && r.ChangeC == 0 {
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ChangeC < out[j].ChangeC })
	return out, nil
}

// ErrNoHistory is returned when there are not enough snapshots to compute drift.
var ErrNoHistory = fmt.Errorf("not enough fuel-price history: run `nsw-transport-pp-cli refresh` (or `sync`) at least twice to build snapshots to compare")

func round2(v float64) float64 {
	return float64(int64(v*100+sign(v)*0.5)) / 100
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
