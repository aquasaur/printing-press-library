// Copyright 2026 dinakar-sarbada. Licensed under Apache-2.0. See LICENSE.

package share

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExportImportRoundTrip exercises the happy path: Export writes a
// bundle, Import reads it back, and the resource label + rows survive
// the round-trip with their original shape.
func TestExportImportRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	rows := []map[string]any{
		{"user_id": "1", "handle": "alice", "direction": "followers"},
		{"user_id": "2", "handle": "bob", "direction": "following"},
	}

	path, err := Export(tmp, "follows", rows)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if filepath.Base(path) != "follows.share.jsonl" {
		t.Fatalf("expected follows.share.jsonl, got %s", filepath.Base(path))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("export file missing: %v", err)
	}

	resource, got, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if resource != "follows" {
		t.Errorf("resource=%q, want follows", resource)
	}
	if len(got) != len(rows) {
		t.Fatalf("rows=%d, want %d", len(got), len(rows))
	}
	for i, want := range rows {
		for k, v := range want {
			if got[i][k] != v {
				t.Errorf("row[%d][%q]=%v, want %v", i, k, got[i][k], v)
			}
		}
	}
}

// TestExportEmpty ensures Export on an empty rowset still writes a
// valid bundle (header only) that Import can read.
func TestExportEmpty(t *testing.T) {
	tmp := t.TempDir()
	path, err := Export(tmp, "users", nil)
	if err != nil {
		t.Fatalf("Export empty: %v", err)
	}
	resource, rows, err := Import(path)
	if err != nil {
		t.Fatalf("Import empty: %v", err)
	}
	if resource != "users" {
		t.Errorf("resource=%q, want users", resource)
	}
	if len(rows) != 0 {
		t.Errorf("rows=%d, want 0", len(rows))
	}
}

// TestImportMissingFile asserts Import returns a meaningful error
// rather than panicking when the bundle path doesn't exist.
func TestImportMissingFile(t *testing.T) {
	_, _, err := Import(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
