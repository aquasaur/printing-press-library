// Instacart order-history dumper. Paste this into DevTools console on a
// logged-in Instacart tab (starting from https://www.instacart.com/store/account/orders)
// to extract your order history as JSONL, then feed the download to
// `instacart history import <file>`.
//
// This works because Instacart's Apollo cache exposes the full order detail
// (retailer + items + quantities) under the OrderManagerOrderDelivery key
// once each order's detail page has been loaded. Plain HTTP scraping does
// NOT work on multi-profile accounts -- the user's selected-profile session
// is required, and only the live browser carries it.
//
// See docs/patterns/authenticated-session-scraping.md for why Chrome MCP /
// interactive DevTools is the right tier for this type of scraping.

(async () => {
  // -- Step 1: collect every order ID from the Orders page via infinite scroll
  //    + "Load more" button clicks.
  const ids = await (async () => {
    const seen = new Set();
    const collect = () =>
      Array.from(document.querySelectorAll('a[href^="/store/orders/"]')).forEach(a => {
        const id = (a.getAttribute('href').match(/\/store\/orders\/(\d+)/) || [])[1];
        if (id) seen.add(id);
      });
    collect();
    let stable = 0;
    for (let i = 0; i < 30 && stable < 3; i++) {
      window.scrollTo(0, document.body.scrollHeight);
      await new Promise(r => setTimeout(r, 1400));
      const before = seen.size;
      collect();
      if (seen.size === before) stable++;
      else stable = 0;
    }
    // Click "Load more" button until it is gone, with a cap for safety.
    for (let i = 0; i < 30; i++) {
      const btn = Array.from(document.querySelectorAll('button')).find(b =>
        /^load more/i.test((b.innerText || '').trim())
      );
      if (!btn) break;
      btn.scrollIntoView({ block: 'center' });
      await new Promise(r => setTimeout(r, 400));
      btn.click();
      await new Promise(r => setTimeout(r, 2200));
      const before = seen.size;
      collect();
      if (seen.size === before) break;
    }
    return Array.from(seen);
  })();

  console.log(`[dumper] found ${ids.length} order IDs`);

  // -- Step 2: for each order, navigate to its detail page via window.location,
  //    wait for Apollo cache to populate, and pull the OrderManagerOrderDelivery
  //    entry. Results persist in localStorage across navigations.
  //
  //    NOTE: This function relies on being RE-RUN after each navigation because
  //    changing window.location tears down the JS context. In practice you run
  //    the per-order extract snippet separately for each order (the companion
  //    extract-one.js). This block is for discovery of order IDs; iterating
  //    through them is done via Chrome MCP driver or a DevTools macro.

  const result = { order_ids: ids, collected_at: new Date().toISOString() };
  window.__ic_order_ids = ids;
  return JSON.stringify(result, null, 2);
})();
