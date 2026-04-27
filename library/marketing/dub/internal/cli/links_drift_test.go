// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"1h30m", time.Hour + 30*time.Minute, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-1d", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseWindow(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestComputeDriftPct(t *testing.T) {
	cases := []struct {
		recent, prior int
		want          float64
	}{
		{100, 100, 0},
		{50, 100, -50},
		{200, 100, 100},
		{0, 100, -100},
		{0, 0, 0},
		{50, 0, 100},
		{75, 50, 50},
	}
	for _, tc := range cases {
		got := computeDriftPct(tc.recent, tc.prior)
		if got != tc.want {
			t.Errorf("computeDriftPct(%d, %d) = %v, want %v", tc.recent, tc.prior, got, tc.want)
		}
	}
}
