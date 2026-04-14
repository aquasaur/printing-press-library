package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Archive.org Wayback Machine fallback for sites that return structural
// blocks (402/403/429). The live fetch fails fast; this fallback asks
// archive.org whether there's a snapshot and fetches from there if so.
//
// This is Phase 1 of the scrape-mitigations plan (docs/plans/2026-04-13-002
// in the cli-printing-press repo): a minimal inline patch shipped to
// observe real-world archive.org hit rate before committing to a cliutil
// abstraction. Designed to be replaceable in Phase 3 by
// cliutil.ScrapeClient; deliberately self-contained so the cutover is
// localized.
//
// Usage: the live Fetch/FetchHTML functions call fetchArchiveFallback()
// when they'd otherwise return ErrBlocked. On success the fallback returns
// the snapshot body; on failure the caller surfaces the original block.

// archiveAvailabilityURL is the Wayback Machine availability API.
// Returns JSON with the closest snapshot URL for a given target.
const archiveAvailabilityURL = "https://archive.org/wayback/available"

// archiveMinGap enforces a minimum interval between archive.org requests
// (to both the availability API and the snapshot fetch). 1.5s is a
// generous good-citizen gap — we're a small client, not a scraper farm.
const archiveMinGap = 1500 * time.Millisecond

// Package-level archive-request pacing. Successive fallback invocations
// serialize behind the minimum gap so parallel Fetch goroutines don't
// hammer archive.org.
var (
	archiveLastReq   time.Time
	archiveLastReqMu sync.Mutex
)

// waybackResponse is the subset of the availability API response we parse.
// The full response has more fields (timestamp, status, etc.); we only
// need the snapshot URL.
type waybackResponse struct {
	ArchivedSnapshots struct {
		Closest *struct {
			URL       string `json:"url"`
			Timestamp string `json:"timestamp"`
			Available bool   `json:"available"`
		} `json:"closest"`
	} `json:"archived_snapshots"`
}

// fetchArchiveFallback queries archive.org for a snapshot of targetURL and
// returns the snapshot HTML body. Returns an error if no snapshot exists,
// the snapshot itself fails to fetch, or archive.org is unreachable —
// callers should surface their original ErrBlocked in those cases.
//
// Logs "warn: archive fallback: <url>" to stderr when engaged so users
// see when they're reading archived content instead of live.
func fetchArchiveFallback(ctx context.Context, client *http.Client, targetURL string) ([]byte, error) {
	if err := awaitArchiveSlot(ctx); err != nil {
		return nil, fmt.Errorf("archive rate limit wait: %w", err)
	}

	snapURL, err := queryWaybackSnapshot(ctx, client, targetURL)
	if err != nil {
		return nil, fmt.Errorf("querying wayback: %w", err)
	}
	if snapURL == "" {
		return nil, fmt.Errorf("no wayback snapshot for %s", targetURL)
	}

	// Enforce HTTPS on snapshot URLs. Wayback's availability API returns
	// http:// by default, but cached archive responses shouldn't be
	// vulnerable to MITM — especially with the 7d TTL we'd apply at
	// Phase 2.
	snapURL = upgradeToHTTPS(snapURL)

	fmt.Fprintf(os.Stderr, "warn: archive fallback: %s (wayback snapshot)\n", targetURL)

	// Second archive.org request — respect the rate limit again.
	if err := awaitArchiveSlot(ctx); err != nil {
		return nil, fmt.Errorf("archive rate limit wait: %w", err)
	}

	return fetchSnapshotBody(ctx, client, snapURL)
}

// awaitArchiveSlot blocks until it's safe to send another archive.org
// request, enforcing the package-level minimum gap. Context-aware.
//
// The "intended next time" is recorded under the lock before releasing so
// concurrent callers queue behind each other correctly rather than all
// racing to the same "now + gap" slot.
func awaitArchiveSlot(ctx context.Context) error {
	archiveLastReqMu.Lock()
	now := time.Now()
	var wait time.Duration
	if !archiveLastReq.IsZero() {
		elapsed := now.Sub(archiveLastReq)
		if elapsed < archiveMinGap {
			wait = archiveMinGap - elapsed
		}
	}
	archiveLastReq = now.Add(wait)
	archiveLastReqMu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// queryWaybackSnapshot asks the availability API for the closest snapshot
// URL for target. Returns an empty string with no error when no snapshot
// exists — caller distinguishes "no snapshot" from "availability API
// failed."
//
// Archive.org's Availability API is schema-sensitive: querying with
// "https://www.example.com/page" frequently misses snapshots that exist
// under "www.example.com/page" (bare). We strip scheme before querying
// so the lookup uses the canonical form archive.org indexes under.
func queryWaybackSnapshot(ctx context.Context, client *http.Client, target string) (string, error) {
	bareURL := stripScheme(target)
	apiURL := archiveAvailabilityURL + "?url=" + url.QueryEscape(bareURL)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("availability API returned HTTP %d", resp.StatusCode)
	}

	// Cap the availability response at 64 KiB — the JSON is tiny, anything
	// larger is a misbehaving server.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}

	var wb waybackResponse
	if err := json.Unmarshal(body, &wb); err != nil {
		return "", fmt.Errorf("parsing wayback response: %w", err)
	}

	if wb.ArchivedSnapshots.Closest == nil || !wb.ArchivedSnapshots.Closest.Available {
		return "", nil
	}
	return wb.ArchivedSnapshots.Closest.URL, nil
}

// fetchSnapshotBody GETs the wayback snapshot URL and returns the HTML
// body. Uses the same 5 MiB cap and browser-ish headers as the live fetch.
func fetchSnapshotBody(ctx context.Context, client *http.Client, snapURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", snapURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot fetch HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 5<<20))
}

// upgradeToHTTPS rewrites a plain-http URL to https. Wayback returns
// http://web.archive.org/web/TIMESTAMP/ORIGINAL by default; we don't want
// cached archive content vulnerable to MITM — especially given archive
// responses may be cached for days in Phase 2.
func upgradeToHTTPS(raw string) string {
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return raw
}

// stripScheme returns the URL with any leading http:// or https:// prefix
// removed. Used for the availability API's canonical lookup form.
func stripScheme(raw string) string {
	if idx := strings.Index(raw, "://"); idx >= 0 {
		return raw[idx+3:]
	}
	return raw
}

// resetArchivePacing zeroes the package-level pacing state. Test-only —
// real callers never need this, but tests that assert timing behavior
// would otherwise be affected by prior tests' pacing state.
func resetArchivePacing() {
	archiveLastReqMu.Lock()
	archiveLastReq = time.Time{}
	archiveLastReqMu.Unlock()
}
