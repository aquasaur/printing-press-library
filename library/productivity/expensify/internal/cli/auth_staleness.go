// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
//
// Shared helpers for the token-staleness surface: threshold resolution,
// human-readable duration formatting, and a small state classifier used by
// both `auth status` and `doctor`. Kept in a dedicated file because helpers.go
// already carries a lot of output-format logic and the staleness concepts are
// specific to the auth subsystem.

package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// defaultStaleMinutes is the staleness threshold default, in minutes.
	// Picked so the WARN fires well before Expensify's observed 2-3h real
	// expiry, giving users and agents room to refresh proactively.
	defaultStaleMinutes = 60

	// minStaleMinutes and maxStaleMinutes clamp the override range. 1 minute
	// is the lower bound so a misconfigured `EXPENSIFY_TOKEN_STALE_AFTER=0`
	// doesn't flip every token to stale; 1440 minutes (24h) is generous but
	// still below Expensify's real expiry.
	minStaleMinutes = 1
	maxStaleMinutes = 1440
)

// stalenessThreshold returns the configured staleness window, consulting
// EXPENSIFY_TOKEN_STALE_AFTER (integer minutes) and clamping to
// [minStaleMinutes, maxStaleMinutes]. Non-integer or out-of-range values fall
// back to the default silently — this is a display knob, not a correctness
// lever, so noisy validation would cost more than it saves.
func stalenessThreshold() time.Duration {
	raw := os.Getenv("EXPENSIFY_TOKEN_STALE_AFTER")
	if raw == "" {
		return time.Duration(defaultStaleMinutes) * time.Minute
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(defaultStaleMinutes) * time.Minute
	}
	if n < minStaleMinutes {
		n = minStaleMinutes
	}
	if n > maxStaleMinutes {
		n = maxStaleMinutes
	}
	return time.Duration(n) * time.Minute
}

// humanizeDuration renders a duration for humans + log greppers:
//   - under 1 minute  → "30s"
//   - under 1 hour    → "45m"
//   - under 24 hours  → "2h15m" (minutes omitted when zero → "2h")
//   - 24 hours or more → "1d3h"  (hours omitted when zero → "1d")
//
// The output is always lowercase and unit-suffixed so it's unambiguous when
// embedded in a line like "Token age: 2h15m (stale)".
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		s := int(d.Seconds())
		return fmt.Sprintf("%ds", s)
	}
	if d < time.Hour {
		m := int(d.Minutes())
		return fmt.Sprintf("%dm", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) - h*60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours()) / 24
	h := int(d.Hours()) - days*24
	if h == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, h)
}

// tokenStaleness describes the age bucket of the current session token.
type tokenStaleness int

const (
	// staleUnknown means LastLoginAt is zero — the token was set via
	// `auth set-token` (or env var) and we have no timestamp to reason about.
	staleUnknown tokenStaleness = iota
	// staleFresh means age < threshold.
	staleFresh
	// staleStale means threshold <= age < 2*threshold.
	staleStale
	// stalePossiblyExpired means age >= 2*threshold. At this point Expensify
	// has almost certainly invalidated the token; auth_status labels it
	// "possibly expired" rather than "expired" because only a round-trip
	// can confirm it's really dead.
	stalePossiblyExpired
)

// classifyStaleness picks the bucket for a given LastLoginAt + now + threshold.
// If lastLogin is zero, the bucket is staleUnknown regardless of now.
func classifyStaleness(lastLogin, now time.Time, threshold time.Duration) tokenStaleness {
	if lastLogin.IsZero() {
		return staleUnknown
	}
	age := now.Sub(lastLogin)
	if age < threshold {
		return staleFresh
	}
	if age < 2*threshold {
		return staleStale
	}
	return stalePossiblyExpired
}
