// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.sqlite")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{Path: filepath.Join(dir, "config.toml")}
}

// TestIngestReconnectApp_PersonalDetails verifies that a canned response with
// a personalDetailsList entry upserts the expected Person rows.
func TestIngestReconnectApp_PersonalDetails(t *testing.T) {
	st := openTestStore(t)
	cfg := newTestConfig(t)

	payload := map[string]any{
		"onyxData": []any{
			map[string]any{
				"key": "personalDetailsList",
				"value": map[string]any{
					"20647491": map[string]any{
						"displayName": "Myk Melez",
						"login":       "myk@example.com",
					},
					"20631946": map[string]any{
						"displayName": "mvh",
						"login":       "mvh@example.com",
					},
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	_, _, _, nPeople := ingestReconnectApp(st, raw, "", "", cfg)
	if nPeople != 2 {
		t.Fatalf("nPeople = %d, want 2", nPeople)
	}

	p, err := st.GetPersonByAccountID(20647491)
	if err != nil {
		t.Fatalf("GetPersonByAccountID(20647491): %v", err)
	}
	if p == nil || p.DisplayName != "Myk Melez" || p.Login != "myk@example.com" {
		t.Fatalf("got %+v, want Myk Melez / myk@example.com", p)
	}
	p2, err := st.GetPersonByAccountID(20631946)
	if err != nil {
		t.Fatalf("GetPersonByAccountID(20631946): %v", err)
	}
	if p2 == nil || p2.DisplayName != "mvh" {
		t.Fatalf("got %+v, want mvh", p2)
	}
}

// TestIngestReconnectApp_SessionAccountID verifies that a session blob with
// accountID populates an empty config.ExpensifyAccountID.
func TestIngestReconnectApp_SessionAccountID(t *testing.T) {
	st := openTestStore(t)
	cfg := newTestConfig(t)

	payload := map[string]any{
		"onyxData": []any{
			map[string]any{
				"key": "session",
				"value": map[string]any{
					"accountID": float64(20631946),
					"authToken": "abc",
				},
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ingestReconnectApp(st, raw, "", "", cfg)
	if cfg.ExpensifyAccountID != 20631946 {
		t.Fatalf("cfg.ExpensifyAccountID = %d, want 20631946", cfg.ExpensifyAccountID)
	}
}

// TestIngestReconnectApp_SessionAccountID_AlreadySet verifies that a session
// blob does NOT overwrite a pre-existing config.ExpensifyAccountID.
func TestIngestReconnectApp_SessionAccountID_AlreadySet(t *testing.T) {
	st := openTestStore(t)
	cfg := newTestConfig(t)
	cfg.ExpensifyAccountID = 99

	payload := map[string]any{
		"onyxData": []any{
			map[string]any{
				"key": "session",
				"value": map[string]any{
					"accountID": float64(20631946),
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)

	ingestReconnectApp(st, raw, "", "", cfg)
	if cfg.ExpensifyAccountID != 99 {
		t.Fatalf("cfg.ExpensifyAccountID = %d, want 99 (pre-existing)", cfg.ExpensifyAccountID)
	}
}

// TestIngestReconnectApp_SessionStringAccountID verifies that a string-typed
// accountID (some session blobs stringify numbers) still populates the config.
func TestIngestReconnectApp_SessionStringAccountID(t *testing.T) {
	st := openTestStore(t)
	cfg := newTestConfig(t)

	payload := map[string]any{
		"onyxData": []any{
			map[string]any{
				"key": "session",
				"value": map[string]any{
					"accountID": "20631946",
				},
			},
		},
	}
	raw, _ := json.Marshal(payload)

	ingestReconnectApp(st, raw, "", "", cfg)
	if cfg.ExpensifyAccountID != 20631946 {
		t.Fatalf("cfg.ExpensifyAccountID = %d, want 20631946 (parsed from string)", cfg.ExpensifyAccountID)
	}
}

// TestIngestReconnectApp_TopLevelPersonalDetails verifies that a top-level
// personalDetailsList (some responses include it at the root, not inside
// onyxData) is still ingested.
func TestIngestReconnectApp_TopLevelPersonalDetails(t *testing.T) {
	st := openTestStore(t)
	cfg := newTestConfig(t)
	payload := map[string]any{
		"personalDetailsList": map[string]any{
			"1001": map[string]any{
				"displayName": "Solo Person",
				"login":       "solo@example.com",
			},
		},
	}
	raw, _ := json.Marshal(payload)
	_, _, _, nPeople := ingestReconnectApp(st, raw, "", "", cfg)
	if nPeople != 1 {
		t.Fatalf("nPeople = %d, want 1", nPeople)
	}
	p, err := st.GetPersonByAccountID(1001)
	if err != nil {
		t.Fatalf("GetPersonByAccountID(1001): %v", err)
	}
	if p == nil || p.DisplayName != "Solo Person" {
		t.Fatalf("got %+v, want Solo Person", p)
	}
}
