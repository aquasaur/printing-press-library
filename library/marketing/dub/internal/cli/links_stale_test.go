// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestClassifyStale(t *testing.T) {
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name            string
		obj             map[string]any
		clicks          int
		archived        bool
		minClicks       int
		expiresAt       string
		createdAt       string
		includeArchived bool
		wantHit         bool
		wantReasonHas   string
	}{
		{
			name:            "active link with clicks is not stale",
			obj:             map[string]any{},
			clicks:          50,
			archived:        false,
			minClicks:       1,
			createdAt:       "2025-06-01T00:00:00Z",
			includeArchived: true,
			wantHit:         false,
		},
		{
			name:            "zero-click old link is stale",
			obj:             map[string]any{},
			clicks:          0,
			archived:        false,
			minClicks:       1,
			createdAt:       "2025-06-01T00:00:00Z",
			includeArchived: true,
			wantHit:         true,
			wantReasonHas:   "low-clicks-since-",
		},
		{
			name:            "archived but trafficked is flagged",
			obj:             map[string]any{},
			clicks:          200,
			archived:        true,
			minClicks:       1,
			createdAt:       "2025-06-01T00:00:00Z",
			includeArchived: true,
			wantHit:         true,
			wantReasonHas:   "archived-but-trafficked",
		},
		{
			name:            "archived excluded when flag false",
			obj:             map[string]any{},
			clicks:          0,
			archived:        true,
			minClicks:       1,
			createdAt:       "2025-06-01T00:00:00Z",
			includeArchived: false,
			wantHit:         false,
		},
		{
			name:            "expired link flagged",
			obj:             map[string]any{},
			clicks:          5,
			archived:        false,
			minClicks:       1,
			expiresAt:       "2025-01-01T00:00:00Z",
			createdAt:       "2024-06-01T00:00:00Z",
			includeArchived: true,
			wantHit:         true,
			wantReasonHas:   "expired",
		},
		{
			name:            "recent link with low clicks is not flagged for low-clicks",
			obj:             map[string]any{},
			clicks:          0,
			archived:        false,
			minClicks:       1,
			createdAt:       time.Now().Add(-time.Hour).Format(time.RFC3339),
			includeArchived: true,
			wantHit:         false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, hit := classifyStale(tc.obj, tc.clicks, tc.archived, tc.minClicks, tc.expiresAt, tc.createdAt, cutoff, tc.includeArchived)
			if hit != tc.wantHit {
				t.Fatalf("hit = %v, want %v (reason=%q)", hit, tc.wantHit, reason)
			}
			if tc.wantReasonHas != "" && !contains(reason, tc.wantReasonHas) {
				t.Fatalf("reason=%q, want substring %q", reason, tc.wantReasonHas)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
