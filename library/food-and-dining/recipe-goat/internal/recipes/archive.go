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
//
// Lookup uses the CDX Search API rather than the Availability API: the
// CDX API has a documented-in-practice 60 req/min ceiling vs availability's
// ~100/min, but it returns unambiguous empty-vs-throttled signals. The
// Availability API silently returns empty `archived_snapshots` when
// soft-throttled, which is indistinguishable from "no snapshot exists."
// CDX returns either a concrete row array or nothing at all.

// archiveCDXURL is the Wayback Machine CDX Search API. Unlike the
// Availability API, this endpoint gives deterministic empty responses
// (not silent throttle-looks-like-miss) and the IA team has publicly
// stated the ceiling as "60 requests per minute average."
const archiveCDXURL = "https://web.archive.org/cdx/search/cdx"

// archiveMinGap enforces a minimum interval between archive.org requests.
// 2s (30 req/min) is the community-proven safe rate — well under the 60/min
// CDX ceiling and comfortable margin for the snapshot fetches that share
// this bucket. Tightening further risks IP firewall blocks (1 hr minimum,
// doubling on repeat).
const archiveMinGap = 2000 * time.Millisecond

// archiveUA identifies this CLI to archive.org. The IA blog post "Let us
// serve you, but don't bring us down" (2023) requests that bulk clients
// identify themselves with a descriptive UA including a contact URL. IA
// staff have stated they'll preferentially allowlist well-identified,
// well-behaved clients; at minimum, an identifiable UA means they can
// reach out before blocking if we misbehave.
const archiveUA = "recipe-goat-pp-cli/1.0 (+https://github.com/mvanhorn/printing-press-library)"

// Package-level archive-request pacing. Successive fallback invocations
// serialize behind the minimum gap so parallel Fetch goroutines don't
// hammer archive.org.
var (
	archiveLastReq   time.Time
	archiveLastReqMu sync.Mutex
)

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

	// Enforce HTTPS on snapshot URLs. CDX's `original` column may be
	// http:// for sites that were archived under plain HTTP; the wayback
	// wrapper serves both, but we want the wrapper fetch itself on HTTPS.
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

// queryWaybackSnapshot asks the CDX Search API for the most recent
// successful snapshot of target. Returns an empty string with no error
// when no snapshot exists.
//
// Query shape: we ask for the single most recent 200-status capture
// (limit=-1 returns from end, filter=statuscode:200 excludes error
// captures that happened to be recorded). JSON output is a 2D array
// with a header row followed by zero or more data rows.
//
// Scheme stripping: archive.org's indexers canonicalize without scheme;
// querying "https://www.example.com/page" frequently misses snapshots
// stored under "www.example.com/page" (bare). We strip scheme before
// querying.
func queryWaybackSnapshot(ctx context.Context, client *http.Client, target string) (string, error) {
	bareURL := stripScheme(target)

	params := url.Values{}
	params.Set("url", bareURL)
	params.Set("limit", "-1")
	params.Set("output", "json")
	params.Set("filter", "statuscode:200")
	apiURL := archiveCDXURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", archiveUA)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Explicit throttle signal. Phase 2 will add exponential backoff
		// and session-level circuit breaker; for now, surface as error so
		// the caller falls back to the original ErrBlocked.
		return "", fmt.Errorf("CDX API rate limited (HTTP 429)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CDX API returned HTTP %d", resp.StatusCode)
	}

	// Cap at 256 KiB — a single-row CDX response is under 1 KiB, but the
	// cap gives headroom if IA returns more rows than expected while
	// keeping us safe against misbehaving servers.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", err
	}

	// Empty body on 200 status means no captures — CDX returns either a
	// populated array or nothing at all. Availability API's silent empty
	// behavior does not occur here.
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return "", nil
	}

	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return "", fmt.Errorf("parsing CDX response: %w", err)
	}

	// First row is column headers; data rows follow. Zero data rows means
	// no snapshot matches the filter.
	if len(rows) < 2 {
		return "", nil
	}

	header := rows[0]
	tsIdx, origIdx := -1, -1
	for i, col := range header {
		switch col {
		case "timestamp":
			tsIdx = i
		case "original":
			origIdx = i
		}
	}
	if tsIdx < 0 || origIdx < 0 {
		return "", fmt.Errorf("CDX response missing timestamp or original columns")
	}

	// Use the last data row. limit=-1 returns a single row (the most
	// recent), but defensively handle any shape.
	latest := rows[len(rows)-1]
	if len(latest) <= tsIdx || len(latest) <= origIdx {
		return "", fmt.Errorf("CDX row malformed: %v", latest)
	}

	// Construct the wayback snapshot URL from timestamp + original URL.
	return fmt.Sprintf("https://web.archive.org/web/%s/%s", latest[tsIdx], latest[origIdx]), nil
}

// fetchSnapshotBody GETs the wayback snapshot URL and returns the HTML
// body. Uses the identifying archive UA (same bucket as the CDX query) —
// this request counts against the same rate limit.
func fetchSnapshotBody(ctx context.Context, client *http.Client, snapURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", snapURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", archiveUA)
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

// upgradeToHTTPS rewrites a plain-http URL to https. CDX's `original`
// column may be http:// for sites archived under plain HTTP; we don't
// want cached archive content vulnerable to MITM.
func upgradeToHTTPS(raw string) string {
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return raw
}

// stripScheme returns the URL with any leading http:// or https:// prefix
// removed. Used for the CDX canonical lookup form.
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
