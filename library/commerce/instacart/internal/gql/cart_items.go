package gql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// CartLineItem is one row of a shopper's cart with a resolved product name.
type CartLineItem struct {
	Name         string  `json:"name"`
	ItemID       string  `json:"item_id"`
	ProductID    string  `json:"product_id"`
	Quantity     float64 `json:"quantity"`
	QuantityType string  `json:"quantity_type,omitempty"`
	Retailer     string  `json:"retailer,omitempty"`
}

// FetchCartItems chains three graphql operations to resolve the full contents
// of a cart with real product names:
//
//  1. CartData(cartId) -> list of {basketProduct.id (items_LOC-PROD), quantity}
//  2. ensureInventoryToken(retailerSlug) -> shopId + zoneId context for Items
//  3. Items(ids, shopId, zoneId, postalCode) -> item names
//
// Cached products (from earlier searches or cart-shows) short-circuit the
// Items call: if every line item is already in the local products table,
// we skip the third round trip entirely.
func FetchCartItems(ctx context.Context, sess *auth.Session, cfg *config.Config, st *store.Store, cartID, retailerSlug string) ([]CartLineItem, error) {
	if cartID == "" {
		return nil, fmt.Errorf("cart id is required")
	}

	client := NewClient(sess, cfg, st)

	// Step 1: fetch cart data.
	resp, err := client.Query(ctx, "CartData", map[string]any{"id": cartID})
	if err != nil {
		return nil, fmt.Errorf("CartData: %w", err)
	}

	rawLines, err := parseCartData(resp.RawBody)
	if err != nil {
		return nil, err
	}
	if len(rawLines) == 0 {
		return []CartLineItem{}, nil
	}

	// Step 2: separate cache hits from misses.
	var misses []string
	lines := make([]CartLineItem, 0, len(rawLines))
	for _, raw := range rawLines {
		line := CartLineItem{
			ItemID:       raw.itemID,
			Quantity:     raw.quantity,
			QuantityType: raw.quantityType,
			Retailer:     retailerSlug,
		}
		if idx := strings.LastIndex(raw.itemID, "-"); idx > 0 {
			line.ProductID = raw.itemID[idx+1:]
		}
		if cached, _ := st.GetProduct(raw.itemID); cached != nil && cached.Name != "" {
			line.Name = cached.Name
		} else {
			misses = append(misses, raw.itemID)
		}
		lines = append(lines, line)
	}

	if len(misses) == 0 {
		return lines, nil
	}

	// Step 3: look up misses via Items. Need shopId/zoneId/postalCode.
	// For cart resolution we bootstrap via ShopCollectionScoped, same as search.
	tok, err := ensureInventoryToken(ctx, client, st, cfg, retailerSlug)
	if err != nil {
		return lines, fmt.Errorf("bootstrap retailer context for %s: %w", retailerSlug, err)
	}

	itemsVars := map[string]any{
		"ids":        misses,
		"shopId":     tok.ShopID,
		"zoneId":     tok.ZoneID,
		"postalCode": cfg.PostalCode,
	}
	resp, err = client.Query(ctx, "Items", itemsVars)
	// Instacart returns partial data with a "Cross Shop Load" or "Missing
	// arguments for item price" error when the cart contains items whose
	// location prefix doesn't match the shop we bootstrapped. The `name`
	// field still comes through in that partial response, which is all
	// cart-show needs. Tolerate the error when RawBody still has names.
	var resolved []SearchResult
	if resp != nil && len(resp.RawBody) > 0 {
		resolved = parseItemsResponse(resp.RawBody, retailerSlug, 0)
	}
	if err != nil && len(resolved) == 0 {
		return lines, fmt.Errorf("Items lookup for cart misses: %w", err)
	}
	nameByID := make(map[string]string, len(resolved))
	for _, r := range resolved {
		nameByID[r.ItemID] = r.Name
		_ = st.UpsertProduct(store.Product{
			ItemID:       r.ItemID,
			ProductID:    r.ProductID,
			RetailerSlug: retailerSlug,
			Name:         r.Name,
		})
	}

	for i := range lines {
		if lines[i].Name == "" {
			if name, ok := nameByID[lines[i].ItemID]; ok {
				lines[i].Name = name
			}
		}
	}
	return lines, nil
}

type rawCartLine struct {
	itemID       string
	quantity     float64
	quantityType string
}

// parseCartData walks a CartData response and extracts cart items.
// The response shape is {data: {userCart: {cartItemCollection: {cartItems: [{quantity, basketProduct: {id}}]}}}}.
func parseCartData(raw []byte) ([]rawCartLine, error) {
	var envelope struct {
		Data struct {
			UserCart struct {
				CartItemCollection struct {
					CartItems []struct {
						Quantity      float64 `json:"quantity"`
						QuantityType  string  `json:"quantityType"`
						BasketProduct struct {
							ID string `json:"id"`
						} `json:"basketProduct"`
					} `json:"cartItems"`
				} `json:"cartItemCollection"`
			} `json:"userCart"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse CartData: %w", err)
	}
	var out []rawCartLine
	for _, it := range envelope.Data.UserCart.CartItemCollection.CartItems {
		if it.BasketProduct.ID == "" {
			continue
		}
		out = append(out, rawCartLine{
			itemID:       it.BasketProduct.ID,
			quantity:     it.Quantity,
			quantityType: it.QuantityType,
		})
	}
	return out, nil
}
