// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
//
// Tests for the `doctor` command's Unit 4 staleness branch plus the existing
// Credentials probe. These exercise the render layer directly (by running
// the cobra command with a temp-config flag set) and stub the session probe
// via EXPENSIFY_BASE_URL + httptest so we never touch the real Expensify
// host. The keychain is mocked in auth_test.go via keyring.MockInit() which
// the package-level init runs before any test here.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/credentials"
)

// newDoctorTestFlags returns a rootFlags wired to a fresh temp config file,
// with auto-retry disabled so the 407 branch can be exercised deterministically
// without the client trying to re-mint a token through a (stub) authenticate
// endpoint. The timeout is short so tests don't hang when the stub server is
// misconfigured.
func newDoctorTestFlags(t *testing.T) *rootFlags {
	t.Helper()
	dir := t.TempDir()
	return &rootFlags{
		configPath:  filepath.Join(dir, "config.toml"),
		timeout:     2 * time.Second,
		noAutoRetry: true,
	}
}

// seedSessionConfig writes a config file with a session token and the given
// LastLoginAt value so doctor has something to classify. If email is non-empty,
// the email is persisted too (enabling the credsConfigured branch when a
// keychain entry is also seeded).
func seedSessionConfig(t *testing.T, path, email string, lastLogin time.Time) {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.ExpensifyAuthToken = "stub-token-for-doctor-tests"
	cfg.ExpensifyEmail = email
	cfg.LastLoginAt = lastLogin
	if err := cfg.SaveSessionToken(cfg.ExpensifyAuthToken, email); err != nil {
		t.Fatalf("SaveSessionToken: %v", err)
	}
	// SaveSessionToken resets LastLoginAt to now; rewrite the timestamp to
	// the caller's value so tests can simulate old sessions.
	cfg.LastLoginAt = lastLogin
	if err := cfg.SaveAccountID(cfg.ExpensifyAccountID); err != nil {
		t.Fatalf("SaveAccountID: %v", err)
	}
}

// runDoctor builds the doctor cobra command against the given flags, captures
// combined stdout/stderr, and returns the output + error.
func runDoctor(t *testing.T, flags *rootFlags) (string, error) {
	t.Helper()
	cmd := newDoctorCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	return buf.String(), err
}

// startStubAPI spins up an httptest server that answers /OpenInitialSettingsPage
// with the given handler. The test sets EXPENSIFY_BASE_URL to the server's
// root so client.buildNewExpensifyRequest routes requests to it.
func startStubAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("EXPENSIFY_BASE_URL", srv.URL)
	return srv
}

// TestDoctor_Fresh: a 10-minute-old LastLoginAt → Staleness line is OK/fresh.
func TestDoctor_Fresh(t *testing.T) {
	flags := newDoctorTestFlags(t)
	// Stub returns a happy session payload so the Credentials line is green,
	// which keeps the test focused on the staleness branch.
	startStubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonCode":200,"email":"test@example.com"}`)
	})

	seedSessionConfig(t, flags.configPath, "", time.Now().Add(-10*time.Minute).UTC())

	out, err := runDoctor(t, flags)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Staleness:") {
		t.Fatalf("output missing Staleness line; got:\n%s", out)
	}
	if !strings.Contains(out, "fresh") {
		t.Fatalf("output missing 'fresh' marker; got:\n%s", out)
	}
	if strings.Contains(out, "stale") {
		t.Fatalf("output should not contain 'stale' for a 10m-old token; got:\n%s", out)
	}
}

// TestDoctor_Stale_WithCredentials: 90-minute-old token + keychain credentials
// configured → WARN with headless hint.
func TestDoctor_Stale_WithCredentials(t *testing.T) {
	flags := newDoctorTestFlags(t)
	email := uniqueEmail(t)
	t.Cleanup(func() { _ = credentials.Delete(email) })
	if err := credentials.Set(email, "pw"); err != nil {
		t.Fatalf("credentials.Set: %v", err)
	}
	startStubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonCode":200,"email":"test@example.com"}`)
	})
	seedSessionConfig(t, flags.configPath, email, time.Now().Add(-90*time.Minute).UTC())

	out, err := runDoctor(t, flags)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Staleness:") {
		t.Fatalf("output missing Staleness line; got:\n%s", out)
	}
	if !strings.Contains(out, "auth login --headless") {
		t.Fatalf("output missing 'auth login --headless' hint; got:\n%s", out)
	}
	if !strings.Contains(out, "WARN") {
		t.Fatalf("output missing WARN level marker; got:\n%s", out)
	}
}

// TestDoctor_Stale_NoCredentials: 90-minute-old token, no keychain creds →
// WARN with store-credentials hint.
func TestDoctor_Stale_NoCredentials(t *testing.T) {
	flags := newDoctorTestFlags(t)
	startStubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonCode":200,"email":"test@example.com"}`)
	})
	// No email set → credsConfigured=false branch.
	seedSessionConfig(t, flags.configPath, "", time.Now().Add(-90*time.Minute).UTC())

	out, err := runDoctor(t, flags)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "auth store-credentials") {
		t.Fatalf("output missing 'auth store-credentials' hint; got:\n%s", out)
	}
	if !strings.Contains(out, "WARN") {
		t.Fatalf("output missing WARN level marker; got:\n%s", out)
	}
}

// TestDoctor_Expired_CredentialsWin: 150-minute-old token + session-validation
// returns jsonCode 407 → the 407 ERROR wins and the staleness line is
// suppressed to avoid double-reporting the same cause.
func TestDoctor_Expired_CredentialsWin(t *testing.T) {
	flags := newDoctorTestFlags(t)
	email := uniqueEmail(t)
	t.Cleanup(func() { _ = credentials.Delete(email) })
	if err := credentials.Set(email, "pw"); err != nil {
		t.Fatalf("credentials.Set: %v", err)
	}
	startStubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// HTTP 200 with jsonCode 407 — the dispatcher's "session expired" shape.
		fmt.Fprintln(w, `{"jsonCode":407,"message":"Session expired"}`)
	})
	seedSessionConfig(t, flags.configPath, email, time.Now().Add(-150*time.Minute).UTC())

	out, err := runDoctor(t, flags)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Credentials:") || !strings.Contains(out, "Session expired") {
		t.Fatalf("output missing expected Session expired line; got:\n%s", out)
	}
	if strings.Contains(out, "Staleness:") {
		t.Fatalf("staleness line should be suppressed when 407 is flagged; got:\n%s", out)
	}
}

// TestDoctor_UnknownAge: LastLoginAt zero-value → INFO line with "unknown".
func TestDoctor_UnknownAge(t *testing.T) {
	flags := newDoctorTestFlags(t)
	startStubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonCode":200,"email":"test@example.com"}`)
	})
	// Zero-value LastLoginAt: write the config directly so SaveSessionToken
	// doesn't stamp "now".
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.ExpensifyAuthToken = "stub-token"
	cfg.LastLoginAt = time.Time{}
	if err := cfg.SaveAccountID(0); err != nil {
		t.Fatalf("SaveAccountID: %v", err)
	}
	// Manually write token without touching LastLoginAt.
	// SaveSessionToken always stamps — avoid it here by re-saving the file
	// shape with a direct Save path: since we just loaded, write via the
	// exported method that doesn't touch LastLoginAt. There isn't one, so
	// we use save() via SaveEmail("") — which no-ops on email but calls save.
	if err := cfg.SaveEmail(""); err != nil {
		t.Fatalf("SaveEmail: %v", err)
	}

	out, err := runDoctor(t, flags)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Staleness:") {
		t.Fatalf("output missing Staleness line; got:\n%s", out)
	}
	if !strings.Contains(out, "unknown") {
		t.Fatalf("output missing 'unknown' marker; got:\n%s", out)
	}
}

// TestStalenessThreshold_Default verifies the env override is respected and
// the default lands at 60 minutes when no override is set.
func TestStalenessThreshold_Default(t *testing.T) {
	t.Setenv("EXPENSIFY_TOKEN_STALE_AFTER", "")
	if got := stalenessThreshold(); got != 60*time.Minute {
		t.Fatalf("default threshold = %v, want 60m", got)
	}
	t.Setenv("EXPENSIFY_TOKEN_STALE_AFTER", "30")
	if got := stalenessThreshold(); got != 30*time.Minute {
		t.Fatalf("overridden threshold = %v, want 30m", got)
	}
	t.Setenv("EXPENSIFY_TOKEN_STALE_AFTER", "0") // clamped to min=1
	if got := stalenessThreshold(); got != 1*time.Minute {
		t.Fatalf("clamped-min threshold = %v, want 1m", got)
	}
	t.Setenv("EXPENSIFY_TOKEN_STALE_AFTER", "99999") // clamped to max=1440
	if got := stalenessThreshold(); got != 1440*time.Minute {
		t.Fatalf("clamped-max threshold = %v, want 1440m", got)
	}
	t.Setenv("EXPENSIFY_TOKEN_STALE_AFTER", "garbage")
	if got := stalenessThreshold(); got != 60*time.Minute {
		t.Fatalf("garbage input threshold = %v, want 60m default", got)
	}
}

// TestHumanizeDuration spot-checks the formatter across the four bucket edges.
func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{10 * time.Minute, "10m"},
		{45 * time.Minute, "45m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{25 * time.Hour, "1d1h"},
		{72 * time.Hour, "3d"},
	}
	for _, c := range cases {
		if got := humanizeDuration(c.in); got != c.want {
			t.Errorf("humanizeDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
