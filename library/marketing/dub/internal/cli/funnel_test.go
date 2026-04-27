// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestParseAnalyticsCount(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		event string
		want  int
	}{
		{"object with named field", `{"clicks": 123}`, "clicks", 123},
		{"object with count", `{"count": 42}`, "clicks", 42},
		{"array with count", `[{"count": 99}]`, "clicks", 99},
		{"array with event field", `[{"clicks": 7}]`, "clicks", 7},
		{"empty array", `[]`, "clicks", 0},
		{"empty object", `{}`, "clicks", 0},
		{"floats", `{"clicks": 12.0}`, "clicks", 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAnalyticsCount(json.RawMessage(tc.raw), tc.event)
			if got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestPctOf(t *testing.T) {
	if pctOf(0, 0) != 0 {
		t.Errorf("0/0 should be 0")
	}
	if pctOf(50, 100) != 50 {
		t.Errorf("50/100 = %v, want 50", pctOf(50, 100))
	}
	if pctOf(0, 100) != 0 {
		t.Errorf("0/100 = %v, want 0", pctOf(0, 100))
	}
}
