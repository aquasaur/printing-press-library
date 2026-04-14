package recipes

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newCDXServer returns an httptest server that responds to the Wayback
// CDX Search API with the caller-supplied JSON body. Used in place of real
// archive.org so tests are hermetic.
func newCDXServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// cdxResponse builds a CDX JSON response body with a single data row
// pointing at snapURL. The CDX JSON schema is [[headers...],[values...]].
func cdxResponse(snapURL, timestamp string) string {
	// The CDX `original` column stores the target URL, and the snapshot
	// URL is assembled as https://web.archive.org/web/<timestamp>/<original>.
	// For tests we want the final reconstructed URL to land on the
	// httptest snapshot server, so we set `original` to the bare
	// snapshot host/path and rely on the test to route via the
	// rewriteTransport.
	return fmt.Sprintf(`[["urlkey","timestamp","original","mimetype","statuscode","digest","length"],`+
		`["com,example)/page","%s","%s","text/html","200","ABC123","12345"]]`, timestamp, snapURL)
}

// --- awaitArchiveSlot ---

func TestAwaitArchiveSlot_FirstCallImmediate(t *testing.T) {
	resetArchivePacing()
	start := time.Now()
	if err := awaitArchiveSlot(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Errorf("first call should return immediately, took %v", elapsed)
	}
}

func TestAwaitArchiveSlot_SecondCallPaced(t *testing.T) {
	resetArchivePacing()
	_ = awaitArchiveSlot(context.Background())
	start := time.Now()
	if err := awaitArchiveSlot(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < archiveMinGap-50*time.Millisecond {
		t.Errorf("second call should wait ~%v, took %v", archiveMinGap, elapsed)
	}
	// Sanity upper bound — allow 2x margin for scheduling slop but catch
	// anything wildly over.
	if elapsed > 2*archiveMinGap {
		t.Errorf("second call took suspiciously long: %v", elapsed)
	}
}

func TestAwaitArchiveSlot_ContextCancellation(t *testing.T) {
	resetArchivePacing()
	_ = awaitArchiveSlot(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := awaitArchiveSlot(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	// Should return near the deadline (~100ms), not wait the full archiveMinGap
	if elapsed > 300*time.Millisecond {
		t.Errorf("ctx-cancel should return near deadline, took %v", elapsed)
	}
}

// --- fetchArchiveFallback end-to-end (via httptest) ---

// setupFallbackServers wires up mock CDX + snapshot servers and rewrites
// the archive URL to point at them. Returns a client and the snapshot
// handler's hit counter.
//
// Mechanics: the CDX server returns a response whose `original` column is
// the TLS snapshot server's URL. The fallback code reconstructs the
// snapshot URL as `https://web.archive.org/web/<ts>/<original>`; the
// rewriteTransport rewrites that `https://web.archive.org` prefix to the
// snapshot server's base.
func setupFallbackServers(t *testing.T, snapBody string, snapStatus int) (*http.Client, *httptest.Server, *int64) {
	t.Helper()
	resetArchivePacing()

	var snapHits int64
	// Snapshot server uses TLS (matches real web.archive.org and the HTTPS
	// upgrade in the fallback code).
	snapSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&snapHits, 1)
		w.WriteHeader(snapStatus)
		_, _ = w.Write([]byte(snapBody))
	}))
	t.Cleanup(snapSrv.Close)

	// CDX response points `original` at the snapshot server's base URL so
	// the reconstructed snapshot URL (https://web.archive.org/web/TS/ORIGINAL)
	// has ORIGINAL already pointing at the test server. The
	// rewriteTransport handles the `https://web.archive.org` prefix.
	cdxBody := cdxResponse(snapSrv.URL+"/page", "20260314204319")
	cdxSrv := newCDXServer(t, cdxBody, http.StatusOK)
	t.Cleanup(cdxSrv.Close)

	// Two rewrites are needed:
	//   1. CDX lookups (archiveCDXURL → cdxSrv)
	//   2. Snapshot fetches (https://web.archive.org/web/... → snapSrv)
	client := &http.Client{
		Transport: &multiRewriteTransport{
			rewrites: []urlRewrite{
				{from: archiveCDXURL, to: cdxSrv.URL},
				{from: "https://web.archive.org", to: snapSrv.URL},
			},
			tls: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}
	return client, snapSrv, &snapHits
}

// urlRewrite is a single from→to URL prefix substitution.
type urlRewrite struct {
	from string
	to   string
}

// multiRewriteTransport rewrites outbound requests matching any of the
// `rewrites` prefixes. Preserves path + query. Only used in tests.
//
// The `tls` field, when non-nil, configures cert verification. Needed
// because httptest.NewTLSServer uses a self-signed cert.
type multiRewriteTransport struct {
	rewrites []urlRewrite
	tls      *tls.Config
}

func (r *multiRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	if r.tls != nil {
		base.TLSClientConfig = r.tls
	}
	reqURL := req.URL.String()
	for _, rw := range r.rewrites {
		if strings.HasPrefix(reqURL, rw.from) {
			newURLStr := rw.to + strings.TrimPrefix(reqURL, rw.from)
			newReq := req.Clone(req.Context())
			parsedURL, err := url.Parse(newURLStr)
			if err != nil {
				return nil, err
			}
			newReq.URL = parsedURL
			newReq.Host = parsedURL.Host
			return base.RoundTrip(newReq)
		}
	}
	return base.RoundTrip(req)
}

// --- test matrix ---

func TestFetchArchiveFallback_Success(t *testing.T) {
	client, _, hits := setupFallbackServers(t, "<html><body>archived content</body></html>", http.StatusOK)
	body, err := fetchArchiveFallback(context.Background(), client, "https://www.seriouseats.com/recipe")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(string(body), "archived content") {
		t.Errorf("body mismatch: %q", body)
	}
	if *hits != 1 {
		t.Errorf("expected 1 snapshot fetch, got %d", *hits)
	}
}

func TestFetchArchiveFallback_NoSnapshot(t *testing.T) {
	resetArchivePacing()
	// CDX returns only the header row — no data rows = no snapshot.
	cdxSrv := newCDXServer(t, `[["urlkey","timestamp","original","mimetype","statuscode","digest","length"]]`, http.StatusOK)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://unknown.example/page")
	if err == nil {
		t.Fatal("expected error when no snapshot exists, got nil")
	}
	if !strings.Contains(err.Error(), "no wayback snapshot") {
		t.Errorf("expected 'no wayback snapshot' error, got %v", err)
	}
}

func TestFetchArchiveFallback_NoSnapshotEmptyBody(t *testing.T) {
	// Some CDX responses come back with just whitespace or empty body
	// when there are no matches. This should be treated as "no snapshot",
	// not a parse error.
	resetArchivePacing()
	cdxSrv := newCDXServer(t, "", http.StatusOK)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://unknown.example/page")
	if err == nil {
		t.Fatal("expected error when response empty, got nil")
	}
	if !strings.Contains(err.Error(), "no wayback snapshot") {
		t.Errorf("expected 'no wayback snapshot' error, got %v", err)
	}
}

func TestFetchArchiveFallback_CDXAPIDown(t *testing.T) {
	resetArchivePacing()
	cdxSrv := newCDXServer(t, "", http.StatusInternalServerError)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error when CDX API returns 500")
	}
	if !strings.Contains(err.Error(), "CDX API") && !strings.Contains(err.Error(), "querying wayback") {
		t.Errorf("expected CDX API error, got %v", err)
	}
}

func TestFetchArchiveFallback_CDXRateLimited(t *testing.T) {
	// CDX API returns 429 when the caller has been throttled. This should
	// surface as an error so the original ErrBlocked is returned to the
	// user rather than silently returning empty content.
	resetArchivePacing()
	cdxSrv := newCDXServer(t, "", http.StatusTooManyRequests)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate-limit error, got %v", err)
	}
}

func TestFetchArchiveFallback_SnapshotFetchFails(t *testing.T) {
	client, _, _ := setupFallbackServers(t, "", http.StatusNotFound)
	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error when snapshot returns 404")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected 'HTTP 404' error, got %v", err)
	}
}

func TestFetchArchiveFallback_MalformedCDXJSON(t *testing.T) {
	resetArchivePacing()
	cdxSrv := newCDXServer(t, "not json at all", http.StatusOK)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestFetchArchiveFallback_CDXMissingColumns(t *testing.T) {
	// Defensive: if the CDX schema ever changes and `timestamp` or
	// `original` are missing from the header, we should error rather than
	// silently return a malformed URL.
	resetArchivePacing()
	cdxSrv := newCDXServer(t,
		`[["urlkey","mimetype","statuscode"],["com,example)/page","text/html","200"]]`,
		http.StatusOK)
	defer cdxSrv.Close()

	client := &http.Client{
		Transport: &multiRewriteTransport{rewrites: []urlRewrite{{from: archiveCDXURL, to: cdxSrv.URL}}},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error when CDX columns missing")
	}
	if !strings.Contains(err.Error(), "missing timestamp or original") {
		t.Errorf("expected missing-columns error, got %v", err)
	}
}

// --- upgradeToHTTPS ---

func TestUpgradeToHTTPS(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"http://web.archive.org/web/2020/https://example.com/", "https://web.archive.org/web/2020/https://example.com/"},
		{"https://already-secure.example/", "https://already-secure.example/"},
		{"", ""},
		{"ftp://weird.example/", "ftp://weird.example/"}, // non-http/https left alone
	}
	for _, tc := range cases {
		if got := upgradeToHTTPS(tc.in); got != tc.want {
			t.Errorf("upgradeToHTTPS(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
