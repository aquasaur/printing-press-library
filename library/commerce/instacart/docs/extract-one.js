// Per-order extractor. Run this after navigating to a
// /store/orders/<id> page. Reads the OrderManagerOrderDelivery cache
// entry (populated after Apollo finishes loading the page) and appends
// the order to localStorage['__ic_dumped'].
//
// After iterating through all orders, run the exporter snippet below
// to download a JSONL file for `instacart history import`.

(async () => {
  await new Promise(r => setTimeout(r, 3500));
  const v = window.__APOLLO_CLIENT__.cache.extract().OrderManagerOrderDelivery;
  const k = v && Object.keys(v).find(k => k.includes('"includeOrderItems":true'));
  if (!k) return JSON.stringify({ skipped: true, reason: 'no items-bearing cache entry yet' });
  const od = v[k].orderDelivery;
  const orderId = JSON.parse(k).orderId;
  const items = (od.orderItems || []).map(oi => ({
    item_id: oi?.currentItem?.legacyId
      ? 'items_' + (od.retailer?.id || '?') + '-' + oi.currentItem.legacyId.replace(/^item_/, '')
      : null,
    product_id: oi?.currentItem?.basketProduct?.item?.productId,
    name: oi?.currentItem?.name,
    quantity: oi?.selectedQuantityValue,
    quantity_type: oi?.selectedQuantityType,
  }));
  const rec = {
    order_id: orderId,
    retailer_id: od.retailer?.id,
    retailer_slug: od.retailer?.slug,
    retailer_name: od.retailer?.name,
    delivered_at: od.deliveredAt,
    item_count: items.length,
    items,
  };
  const existing = JSON.parse(localStorage.getItem('__ic_dumped') || '[]');
  if (!existing.find(r => r.order_id === orderId)) existing.push(rec);
  localStorage.setItem('__ic_dumped', JSON.stringify(existing));
  return JSON.stringify({ order_id: orderId, retailer: od.retailer?.slug, items: items.length, total: existing.length });
})();

// ---- Exporter (run after all per-order extracts complete) ----
// Triggers a browser download of instacart-orders.jsonl. Drop it somewhere
// the CLI can find, then run:
//     instacart history import ~/Downloads/instacart-orders.jsonl

/*
(() => {
  const data = JSON.parse(localStorage.getItem('__ic_dumped') || '[]');
  const jsonl = data.map(r => JSON.stringify(r)).join('\n');
  const blob = new Blob([jsonl], { type: 'application/x-ndjson' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'instacart-orders.jsonl';
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  return JSON.stringify({ downloaded: true, bytes: jsonl.length, orders: data.length });
})();
*/
