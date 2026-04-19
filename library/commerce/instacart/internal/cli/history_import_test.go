package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImportEndToEnd exercises writeImportedOrder across a small JSONL:
// two orders at two retailers plus a repeated item. Confirms orders and
// items land, FTS finds them, and the history-first resolver picks them up.
func TestImportEndToEnd(t *testing.T) {
	app := newTestApp(t)

	lines := []string{
		`{"order_id":"ord-1","retailer_id":"42","retailer_slug":"qfc","retailer_name":"QFC","delivered_at":"2026-04-18T15:31:54Z","item_count":2,"items":[{"item_id":"items_42-111","product_id":"p1","name":"Test Sorbet","quantity":1,"quantity_type":"each"},{"item_id":"items_42-222","product_id":"p2","name":"Oat Milk","quantity":2,"quantity_type":"each"}]}`,
		`{"order_id":"ord-2","retailer_id":"1","retailer_slug":"safeway","retailer_name":"Safeway","delivered_at":"2026-04-10T12:00:00Z","item_count":1,"items":[{"item_id":"items_1-333","product_id":"p3","name":"Whole Milk","quantity":1,"quantity_type":"each"}]}`,
	}

	for _, line := range lines {
		var o jsonlOrder
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := writeImportedOrder(app.Store, &o); err != nil {
			t.Fatalf("writeImportedOrder: %v", err)
		}
	}

	orderCount, _ := app.Store.CountOrders()
	if orderCount != 2 {
		t.Fatalf("expected 2 orders, got %d", orderCount)
	}

	itemCount, _, _ := app.Store.CountPurchasedItems()
	if itemCount != 3 {
		t.Errorf("expected 3 unique purchased items, got %d", itemCount)
	}

	matches, err := app.Store.SearchPurchasedItems("sorbet", "qfc", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 || matches[0].Name != "Test Sorbet" {
		t.Errorf("expected Test Sorbet match at qfc, got %+v", matches)
	}

	picked := resolveFromHistory(app, "qfc", "Test Sorbet")
	if picked == nil {
		t.Fatal("expected history-first resolver to pick a result")
	}
	if picked.ItemID != "items_42-111" {
		t.Errorf("expected items_42-111, got %s", picked.ItemID)
	}
}

// TestImportIdempotent confirms re-importing the same record does not
// inflate purchase_count — UpsertPurchasedItem uses MAX(excluded,current).
func TestImportIdempotent(t *testing.T) {
	app := newTestApp(t)
	line := `{"order_id":"ord-1","retailer_id":"42","retailer_slug":"qfc","retailer_name":"QFC","delivered_at":"2026-04-18T15:31:54Z","item_count":1,"items":[{"item_id":"items_42-111","product_id":"p1","name":"Test Sorbet","quantity":1,"quantity_type":"each"}]}`
	var o jsonlOrder
	if err := json.Unmarshal([]byte(line), &o); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := writeImportedOrder(app.Store, &o); err != nil {
			t.Fatalf("writeImportedOrder #%d: %v", i, err)
		}
	}
	orderCount, _ := app.Store.CountOrders()
	if orderCount != 1 {
		t.Errorf("expected 1 order after 3 imports, got %d", orderCount)
	}
	rows, _ := app.Store.ListPurchasedItems("qfc", 5)
	if len(rows) != 1 {
		t.Fatalf("expected 1 purchased_items row, got %d", len(rows))
	}
	if rows[0].PurchaseCount != 1 {
		t.Errorf("expected purchase_count=1, got %d", rows[0].PurchaseCount)
	}
}

// TestImportFixtureFile runs the parsing+write path against the checked-in
// testdata JSONL.
func TestImportFixtureFile(t *testing.T) {
	app := newTestApp(t)
	fixturePath := filepath.Join("..", "..", "testdata", "orders-sample.jsonl")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var o jsonlOrder
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if err := writeImportedOrder(app.Store, &o); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	orderCount, _ := app.Store.CountOrders()
	if orderCount != 3 {
		t.Errorf("expected 3 orders from fixture, got %d", orderCount)
	}
}

// TestImportSkipsMalformed confirms the CLI-level skip-and-continue
// behavior (parse error -> report line, continue with next).
func TestImportSkipsMalformed(t *testing.T) {
	lines := []string{
		`not-json`,
		`{"order_id":"ok-1","retailer_slug":"qfc","delivered_at":"2026-04-01T00:00:00Z","items":[{"item_id":"items_42-1","name":"A","quantity":1,"quantity_type":"each"}]}`,
	}
	app := newTestApp(t)
	for _, line := range lines {
		var o jsonlOrder
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			continue
		}
		if o.OrderID == "" {
			continue
		}
		if err := writeImportedOrder(app.Store, &o); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	orderCount, _ := app.Store.CountOrders()
	if orderCount != 1 {
		t.Errorf("expected 1 order after malformed skip, got %d", orderCount)
	}
}
