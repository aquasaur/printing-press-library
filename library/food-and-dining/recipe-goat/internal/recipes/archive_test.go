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

// newAvailabilityServer returns an httptest server that responds to the
// Wayback availability API with the caller-supplied JSON body. Used in
// place of real archive.org so tests are hermetic.
func newAvailabilityServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// withAvailabilityURL temporarily swaps the package-level
// archiveAvailabilityURL so tests can point the fallback at an httptest
// server. Restored via t.Cleanup.
func withAvailabilityURL(t *testing.T, url string) {
	t.Helper()
	// archiveAvailabilityURL is a const — can't swap directly. Tests that
	// need to override the endpoint use the archiveAvailabilityURLForTest
	// shim added below. This function documents the intent; it's a no-op
	// placeholder so future refactors that make the URL variable have a
	// clear hook.
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

// setupFallbackServers wires up mock availability + snapshot servers and
// makes the package-level archive URL point at them. Returns a client and
// the snapshot handler's hit counter.
func setupFallbackServers(t *testing.T, snapBody string, snapStatus int) (*http.Client, *httptest.Server, *int64) {
	t.Helper()
	resetArchivePacing()

	var snapHits int64
	// Snapshot server is TLS so upgradeToHTTPS (called inside the
	// fallback code path) produces a URL the test can reach. Without
	// this, the http→https rewrite would leave the test pointing at a
	// non-HTTPS httptest server.
	snapSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&snapHits, 1)
		w.WriteHeader(snapStatus)
		_, _ = w.Write([]byte(snapBody))
	}))
	t.Cleanup(snapSrv.Close)

	// The availability server returns JSON with a DELIBERATELY http://
	// snapshot URL so we exercise the upgradeToHTTPS path. The test
	// client skips cert verification since httptest uses a self-signed
	// cert.
	insecureSnapURL := "http://" + strings.TrimPrefix(snapSrv.URL, "https://") + "/page"
	availBody := fmt.Sprintf(`{"archived_snapshots":{"closest":{"url":"%s","timestamp":"20260314204319","available":true}}}`, insecureSnapURL)
	availSrv := newAvailabilityServer(t, availBody, http.StatusOK)
	t.Cleanup(availSrv.Close)

	client := &http.Client{
		Transport: &rewriteTransport{
			from: archiveAvailabilityURL,
			to:   availSrv.URL,
			tls:  &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}
	return client, snapSrv, &snapHits
}

// rewriteTransport rewrites outbound requests whose URL starts with `from`
// to instead hit `to`. Preserves path + query. Only used in tests.
//
// The `tls` field, when non-nil, configures cert verification for all
// requests going through the underlying transport. Needed because
// httptest.NewTLSServer uses a self-signed cert.
type rewriteTransport struct {
	from string
	to   string
	tls  *tls.Config
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	if r.tls != nil {
		base.TLSClientConfig = r.tls
	}
	if strings.HasPrefix(req.URL.String(), r.from) {
		newURLStr := r.to + strings.TrimPrefix(req.URL.String(), r.from)
		newReq := req.Clone(req.Context())
		parsedURL, err := url.Parse(newURLStr)
		if err != nil {
			return nil, err
		}
		newReq.URL = parsedURL
		newReq.Host = parsedURL.Host
		return base.RoundTrip(newReq)
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
	availSrv := newAvailabilityServer(t, `{"archived_snapshots":{}}`, http.StatusOK)
	defer availSrv.Close()

	client := &http.Client{
		Transport: &rewriteTransport{from: archiveAvailabilityURL, to: availSrv.URL},
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

func TestFetchArchiveFallback_AvailabilityAPIDown(t *testing.T) {
	resetArchivePacing()
	availSrv := newAvailabilityServer(t, "", http.StatusInternalServerError)
	defer availSrv.Close()

	client := &http.Client{
		Transport: &rewriteTransport{from: archiveAvailabilityURL, to: availSrv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error when availability API returns 500")
	}
	if !strings.Contains(err.Error(), "availability API") && !strings.Contains(err.Error(), "querying wayback") {
		t.Errorf("expected availability-API error, got %v", err)
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

func TestFetchArchiveFallback_MalformedAvailabilityJSON(t *testing.T) {
	resetArchivePacing()
	availSrv := newAvailabilityServer(t, "not json at all", http.StatusOK)
	defer availSrv.Close()

	client := &http.Client{
		Transport: &rewriteTransport{from: archiveAvailabilityURL, to: availSrv.URL},
		Timeout:   5 * time.Second,
	}

	_, err := fetchArchiveFallback(context.Background(), client, "https://x.example/page")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
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
