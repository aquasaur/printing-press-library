package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
)

type Store struct {
	db   *sql.DB
	path string
}

func Open() (*Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "instacart.db")
	return openAt(path)
}

func openAt(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Path() string { return s.path }

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS persisted_ops (
			operation_name TEXT PRIMARY KEY,
			sha256_hash TEXT NOT NULL,
			captured_at INTEGER NOT NULL,
			sample_variables TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS retailers (
			slug TEXT PRIMARY KEY,
			retailer_id TEXT,
			shop_id TEXT,
			zone_id TEXT,
			name TEXT,
			location_id TEXT,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			item_id TEXT PRIMARY KEY,
			product_id TEXT,
			retailer_slug TEXT,
			name TEXT,
			brand TEXT,
			size TEXT,
			price_cents INTEGER,
			currency TEXT,
			in_stock INTEGER,
			updated_at INTEGER
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS products_fts USING fts5(
			item_id UNINDEXED,
			retailer_slug UNINDEXED,
			name,
			brand,
			size,
			tokenize = 'porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS carts (
			cart_id TEXT PRIMARY KEY,
			retailer_slug TEXT,
			shop_id TEXT,
			item_count INTEGER,
			subtotal_cents INTEGER,
			currency TEXT,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS cart_items (
			cart_id TEXT NOT NULL,
			item_id TEXT NOT NULL,
			quantity REAL,
			quantity_type TEXT,
			name TEXT,
			price_cents INTEGER,
			PRIMARY KEY (cart_id, item_id)
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_tokens (
			retailer_slug TEXT PRIMARY KEY,
			token TEXT NOT NULL,
			location_id TEXT,
			shop_id TEXT,
			zone_id TEXT,
			fetched_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate %q: %w", shortStmt(stmt), err)
		}
	}
	return nil
}

func shortStmt(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

type Op struct {
	OperationName string
	Sha256Hash    string
	SampleVars    string
	CapturedAt    time.Time
}

func (s *Store) UpsertOp(op Op) error {
	_, err := s.db.Exec(
		`INSERT INTO persisted_ops(operation_name, sha256_hash, captured_at, sample_variables)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(operation_name) DO UPDATE SET
			sha256_hash=excluded.sha256_hash,
			captured_at=excluded.captured_at,
			sample_variables=COALESCE(excluded.sample_variables, sample_variables)`,
		op.OperationName, op.Sha256Hash, time.Now().Unix(), op.SampleVars,
	)
	return err
}

func (s *Store) LookupOp(name string) (string, error) {
	var hash string
	err := s.db.QueryRow(`SELECT sha256_hash FROM persisted_ops WHERE operation_name = ?`, name).Scan(&hash)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (s *Store) CountOps() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM persisted_ops`).Scan(&n)
	return n, err
}

func (s *Store) ListOps() ([]Op, error) {
	rows, err := s.db.Query(`SELECT operation_name, sha256_hash, captured_at FROM persisted_ops ORDER BY operation_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Op
	for rows.Next() {
		var o Op
		var ts int64
		if err := rows.Scan(&o.OperationName, &o.Sha256Hash, &ts); err != nil {
			return nil, err
		}
		o.CapturedAt = time.Unix(ts, 0)
		out = append(out, o)
	}
	return out, rows.Err()
}

type Retailer struct {
	Slug       string
	RetailerID string
	ShopID     string
	ZoneID     string
	Name       string
	LocationID string
}

func (s *Store) UpsertRetailer(r Retailer) error {
	_, err := s.db.Exec(
		`INSERT INTO retailers(slug, retailer_id, shop_id, zone_id, name, location_id, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
			retailer_id=excluded.retailer_id,
			shop_id=excluded.shop_id,
			zone_id=excluded.zone_id,
			name=excluded.name,
			location_id=excluded.location_id,
			updated_at=excluded.updated_at`,
		r.Slug, r.RetailerID, r.ShopID, r.ZoneID, r.Name, r.LocationID, time.Now().Unix(),
	)
	return err
}

func (s *Store) GetRetailer(slug string) (*Retailer, error) {
	var r Retailer
	err := s.db.QueryRow(
		`SELECT slug, retailer_id, shop_id, zone_id, name, location_id FROM retailers WHERE slug = ?`,
		slug,
	).Scan(&r.Slug, &r.RetailerID, &r.ShopID, &r.ZoneID, &r.Name, &r.LocationID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListRetailers() ([]Retailer, error) {
	rows, err := s.db.Query(`SELECT slug, retailer_id, shop_id, zone_id, name, location_id FROM retailers ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Retailer
	for rows.Next() {
		var r Retailer
		if err := rows.Scan(&r.Slug, &r.RetailerID, &r.ShopID, &r.ZoneID, &r.Name, &r.LocationID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type InventoryToken struct {
	RetailerSlug string
	Token        string
	LocationID   string
	ShopID       string
	ZoneID       string
	FetchedAt    time.Time
	ExpiresAt    time.Time
}

// UpsertInventoryToken saves an inventory session token for a retailer with a TTL.
func (s *Store) UpsertInventoryToken(t InventoryToken) error {
	_, err := s.db.Exec(
		`INSERT INTO inventory_tokens(retailer_slug, token, location_id, shop_id, zone_id, fetched_at, expires_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(retailer_slug) DO UPDATE SET
			token=excluded.token,
			location_id=excluded.location_id,
			shop_id=excluded.shop_id,
			zone_id=excluded.zone_id,
			fetched_at=excluded.fetched_at,
			expires_at=excluded.expires_at`,
		t.RetailerSlug, t.Token, t.LocationID, t.ShopID, t.ZoneID,
		t.FetchedAt.Unix(), t.ExpiresAt.Unix(),
	)
	return err
}

// GetInventoryToken returns a cached token if present and unexpired.
// Returns (nil, nil) when no cached token exists or the stored one has expired.
func (s *Store) GetInventoryToken(slug string) (*InventoryToken, error) {
	var t InventoryToken
	var fetchedAt, expiresAt int64
	err := s.db.QueryRow(
		`SELECT retailer_slug, token, location_id, shop_id, zone_id, fetched_at, expires_at
		 FROM inventory_tokens WHERE retailer_slug = ?`,
		slug,
	).Scan(&t.RetailerSlug, &t.Token, &t.LocationID, &t.ShopID, &t.ZoneID, &fetchedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.FetchedAt = time.Unix(fetchedAt, 0)
	t.ExpiresAt = time.Unix(expiresAt, 0)
	if time.Now().After(t.ExpiresAt) {
		return nil, nil
	}
	return &t, nil
}

// ClearInventoryToken invalidates the cached token for a retailer, forcing
// the next search to re-bootstrap via ShopCollectionScoped.
func (s *Store) ClearInventoryToken(slug string) error {
	_, err := s.db.Exec(`DELETE FROM inventory_tokens WHERE retailer_slug = ?`, slug)
	return err
}

type Product struct {
	ItemID       string
	ProductID    string
	RetailerSlug string
	Name         string
	Brand        string
	Size         string
	PriceCents   int64
	Currency     string
	InStock      bool
}

// UpsertProduct stores or updates a resolved product in both products and products_fts.
func (s *Store) UpsertProduct(p Product) error {
	inStock := 0
	if p.InStock {
		inStock = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO products(item_id, product_id, retailer_slug, name, brand, size, price_cents, currency, in_stock, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET
			product_id=excluded.product_id,
			retailer_slug=excluded.retailer_slug,
			name=excluded.name,
			brand=excluded.brand,
			size=excluded.size,
			price_cents=excluded.price_cents,
			currency=excluded.currency,
			in_stock=excluded.in_stock,
			updated_at=excluded.updated_at`,
		p.ItemID, p.ProductID, p.RetailerSlug, p.Name, p.Brand, p.Size,
		p.PriceCents, p.Currency, inStock, time.Now().Unix(),
	)
	if err != nil {
		return err
	}

	// FTS table: delete + insert (FTS5 upsert dance).
	_, err = tx.Exec(`DELETE FROM products_fts WHERE item_id = ?`, p.ItemID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO products_fts(item_id, retailer_slug, name, brand, size) VALUES(?, ?, ?, ?, ?)`,
		p.ItemID, p.RetailerSlug, p.Name, p.Brand, p.Size,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// GetProduct returns a cached product by item_id, or nil if not found.
func (s *Store) GetProduct(itemID string) (*Product, error) {
	var p Product
	var inStock int
	err := s.db.QueryRow(
		`SELECT item_id, product_id, retailer_slug, name, brand, size, price_cents, currency, in_stock
		 FROM products WHERE item_id = ?`,
		itemID,
	).Scan(&p.ItemID, &p.ProductID, &p.RetailerSlug, &p.Name, &p.Brand, &p.Size,
		&p.PriceCents, &p.Currency, &inStock)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.InStock = inStock == 1
	return &p, nil
}
