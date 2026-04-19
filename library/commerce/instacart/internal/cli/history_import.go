package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// jsonlOrder mirrors one JSONL line produced by docs/extract-one.js. Fields
// not carried by the extractor (first_purchased_at, last_price_cents, etc.)
// are filled with sensible defaults during import.
type jsonlOrder struct {
	OrderID      string           `json:"order_id"`
	RetailerID   string           `json:"retailer_id"`
	RetailerSlug string           `json:"retailer_slug"`
	RetailerName string           `json:"retailer_name"`
	DeliveredAt  string           `json:"delivered_at"`
	ItemCount    int              `json:"item_count"`
	Items        []jsonlOrderItem `json:"items"`
}

type jsonlOrderItem struct {
	ItemID       string  `json:"item_id"`
	ProductID    string  `json:"product_id"`
	Name         string  `json:"name"`
	Brand        string  `json:"brand,omitempty"`
	Size         string  `json:"size,omitempty"`
	Category     string  `json:"category,omitempty"`
	Quantity     float64 `json:"quantity"`
	QuantityType string  `json:"quantity_type"`
	PriceCents   int64   `json:"price_cents,omitempty"`
}

// newHistoryImportCmd ingests a JSONL file produced by the browser-side
// dumper (docs/extract-one.js) into the local history tables.
//
// Designed to be complementary to `history sync`: sync is the eventual
// programmatic path; import is the pragmatic "I dumped via the browser,
// load it up" path. Both land in the same schema and feed the same
// history-first resolver in `add`.
//
// Idempotent: re-running the same JSONL does not inflate purchase_count
// because UpsertPurchasedItem uses MAX(excluded, current) for counts.
func newHistoryImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <path>",
		Short: "Import a JSONL order dump into the local history tables",
		Long: `Reads a JSONL file produced by the companion browser dumper
(docs/extract-one.js) and upserts orders, order_items, and purchased_items.

The JSONL shape is one order per line with this field set:
  order_id, retailer_id, retailer_slug, retailer_name, delivered_at,
  item_count, items[{item_id, product_id, name, quantity, quantity_type}]

Pass '-' for stdin. Idempotent -- re-importing the same file does not
double-count purchases.

After import, 'instacart add <retailer> "<query>" --dry-run --json' will
resolve via history for anything you have bought before at that retailer.`,
		Example: `  instacart history import ~/Downloads/instacart-orders.jsonl
  instacart history import -  # read from stdin
  instacart history import /tmp/dump.jsonl --dry-run
  instacart history import /tmp/dump.jsonl --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			path := args[0]
			reader, closeFn, err := openImport(path)
			if err != nil {
				return coded(ExitNotFound, "%v", err)
			}
			defer closeFn()

			summary := importSummary{PerRetailer: map[string]int{}}
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				raw := strings.TrimSpace(scanner.Text())
				if raw == "" {
					continue
				}
				var o jsonlOrder
				if err := json.Unmarshal([]byte(raw), &o); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: skip malformed: %v\n", lineNo, err)
					summary.SkippedLines++
					continue
				}
				if o.OrderID == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: skip missing order_id\n", lineNo)
					summary.SkippedLines++
					continue
				}
				if app.DryRun {
					summary.OrdersSeen++
					summary.ItemsSeen += len(o.Items)
					summary.PerRetailer[o.RetailerSlug]++
					continue
				}
				if err := writeImportedOrder(app.Store, &o); err != nil {
					return coded(ExitConflict, "line %d: write order %s: %v", lineNo, o.OrderID, err)
				}
				summary.OrdersWritten++
				summary.ItemsWritten += len(o.Items)
				summary.PerRetailer[o.RetailerSlug]++
			}
			if err := scanner.Err(); err != nil {
				return coded(ExitConflict, "read error: %v", err)
			}

			// Stamp per-retailer history_sync_meta so downstream queries (doctor,
			// history stats) recognise we have usable data.
			if !app.DryRun {
				for slug, orders := range summary.PerRetailer {
					if slug == "" {
						continue
					}
					_ = app.Store.UpsertHistorySyncMeta(store.HistorySyncMeta{
						RetailerSlug:       slug,
						LastSyncAt:         time.Now(),
						LastSyncStatus:     fmt.Sprintf("imported from %s", path),
						LastSyncOrderCount: orders,
						LastSyncItemCount:  summary.ItemsWritten,
					})
				}
			}

			return renderImportSummary(cmd, summary, app.JSON, app.DryRun)
		},
	}
	return cmd
}

type importSummary struct {
	OrdersSeen    int            `json:"orders_seen"`
	OrdersWritten int            `json:"orders_written"`
	ItemsSeen     int            `json:"items_seen"`
	ItemsWritten  int            `json:"items_written"`
	SkippedLines  int            `json:"skipped_lines"`
	PerRetailer   map[string]int `json:"per_retailer"`
}

func openImport(path string) (io.Reader, func() error, error) {
	if path == "-" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open %q: %w", path, err)
	}
	return f, f.Close, nil
}

// writeImportedOrder persists one JSONL record by calling the same store
// helpers `history sync` would call. Reuses UpsertPurchasedItem without
// incrementCount so purchase counts reflect real order-count, not repeat
// imports.
func writeImportedOrder(s *store.Store, o *jsonlOrder) error {
	placedAt, _ := time.Parse(time.RFC3339, o.DeliveredAt)
	if placedAt.IsZero() {
		placedAt = time.Now()
	}
	if err := s.UpsertOrder(store.Order{
		OrderID:      o.OrderID,
		RetailerSlug: o.RetailerSlug,
		PlacedAt:     placedAt,
		Status:       "imported",
		ItemCount:    len(o.Items),
	}); err != nil {
		return err
	}
	for _, it := range o.Items {
		if it.ItemID == "" {
			continue
		}
		if err := s.UpsertOrderItem(store.OrderItem{
			OrderID:      o.OrderID,
			ItemID:       it.ItemID,
			ProductID:    it.ProductID,
			Name:         it.Name,
			Quantity:     it.Quantity,
			QuantityType: it.QuantityType,
			PriceCents:   it.PriceCents,
		}); err != nil {
			return err
		}
		if err := s.UpsertPurchasedItem(store.PurchasedItem{
			ItemID:           it.ItemID,
			RetailerSlug:     o.RetailerSlug,
			ProductID:        it.ProductID,
			Name:             it.Name,
			Brand:            it.Brand,
			Size:             it.Size,
			Category:         it.Category,
			LastPurchasedAt:  placedAt,
			FirstPurchasedAt: placedAt,
			PurchaseCount:    1,
			LastPriceCents:   it.PriceCents,
			LastInStock:      true,
		}, false); err != nil {
			return err
		}
	}
	return nil
}

func renderImportSummary(cmd *cobra.Command, s importSummary, asJSON, dryRun bool) error {
	if asJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"dry_run":      dryRun,
			"orders":       pickCount(dryRun, s.OrdersSeen, s.OrdersWritten),
			"items":        pickCount(dryRun, s.ItemsSeen, s.ItemsWritten),
			"skipped":      s.SkippedLines,
			"per_retailer": s.PerRetailer,
		})
	}
	prefix := "imported"
	if dryRun {
		prefix = "would import"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d orders, %d items (skipped %d malformed lines)\n",
		prefix, pickCount(dryRun, s.OrdersSeen, s.OrdersWritten), pickCount(dryRun, s.ItemsSeen, s.ItemsWritten), s.SkippedLines)
	if len(s.PerRetailer) > 0 {
		keys := make([]string, 0, len(s.PerRetailer))
		for k := range s.PerRetailer {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(cmd.OutOrStdout(), "per retailer:")
		for _, k := range keys {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d orders\n", k, s.PerRetailer[k])
		}
	}
	return nil
}

func pickCount(dryRun bool, seen, written int) int {
	if dryRun {
		return seen
	}
	return written
}
