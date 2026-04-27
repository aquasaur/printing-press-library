// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestIsOlderThanDays(t *testing.T) {
	defer func(orig func() time.Time) { nowFunc = orig }(nowFunc)
	nowFunc = func() time.Time {
		return time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name string
		ts   string
		days int
		want bool
	}{
		{"recent", "2026-03-30T00:00:00Z", 14, false},
		{"exactly 14 days", "2026-03-18T00:00:00Z", 14, false},
		{"15 days old", "2026-03-17T00:00:00Z", 14, true},
		{"empty timestamp", "", 14, false},
		{"unparseable", "not-a-date", 14, false},
		{"30+ days old", "2025-12-01T00:00:00Z", 14, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isOlderThanDays(tc.ts, tc.days)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestParseTimestamp(t *testing.T) {
	good := []string{
		"2026-04-01T12:00:00Z",
		"2026-04-01T12:00:00.123Z",
		"2026-04-01",
	}
	for _, s := range good {
		if _, err := parseTimestamp(s); err != nil {
			t.Errorf("parseTimestamp(%q) failed: %v", s, err)
		}
	}
	if _, err := parseTimestamp("garbage"); err == nil {
		t.Error("expected parseTimestamp to fail on garbage input")
	}
}
