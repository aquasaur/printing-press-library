// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// CLI doctor command — checks config, auth, connectivity.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/credentials"

	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health (config, auth, connectivity)",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				report["config"] = fmt.Sprintf("error: %s", err)
			} else {
				report["config"] = "ok"
				report["config_path"] = cfg.Path
				report["base_url"] = cfg.BaseURL
			}

			if cfg != nil {
				if cfg.HasSessionAuth() {
					report["session_auth"] = fmt.Sprintf("Session token: present (%d chars)", len(cfg.ExpensifyAuthToken))
				} else {
					report["session_auth"] = "not configured"
				}
				if cfg.HasPartnerAuth() {
					report["partner_auth"] = "Partner credentials: set"
				} else {
					report["partner_auth"] = "not configured"
				}
				if !cfg.HasSessionAuth() && !cfg.HasPartnerAuth() {
					report["auth"] = "not configured"
					report["auth_hint"] = "run `expensify-pp-cli auth login`"
				} else {
					report["auth"] = "configured"
					if cfg.AuthSource != "" {
						report["auth_source"] = cfg.AuthSource
					}
				}
			}

			// Session staleness: derived purely from LastLoginAt + threshold,
			// no network call. Computed BEFORE the credentials probe so the
			// report map carries both values; if the probe later flags 407
			// we suppress the staleness line to avoid double-reporting.
			credsConfigured := false
			if cfg != nil && cfg.ExpensifyEmail != "" && credentials.Has(cfg.ExpensifyEmail) {
				credsConfigured = true
			}
			threshold := stalenessThreshold()
			now := time.Now().UTC()
			if cfg != nil && cfg.HasSessionAuth() {
				bucket := classifyStaleness(cfg.LastLoginAt, now, threshold)
				switch bucket {
				case staleUnknown:
					report["staleness"] = "Token age: unknown (no login timestamp recorded — token set via auth set-token?)"
					report["staleness_level"] = "INFO"
				case staleFresh:
					age := now.Sub(cfg.LastLoginAt)
					report["staleness"] = fmt.Sprintf("Token age: %s (fresh)", humanizeDuration(age))
					report["staleness_level"] = "OK"
				case staleStale:
					age := now.Sub(cfg.LastLoginAt)
					hint := stalenessHint(credsConfigured)
					report["staleness"] = fmt.Sprintf("Token age: %s (>= %s stale). %s", humanizeDuration(age), humanizeDuration(threshold), hint)
					report["staleness_level"] = "WARN"
				case stalePossiblyExpired:
					age := now.Sub(cfg.LastLoginAt)
					hint := stalenessHint(credsConfigured)
					report["staleness"] = fmt.Sprintf("Token age: %s (>= %s possibly expired). %s", humanizeDuration(age), humanizeDuration(2*threshold), hint)
					report["staleness_level"] = "ERROR"
				}
			}

			// Session validation: try OpenPublicProfile if a session token is configured.
			sessionExpired := false
			if cfg != nil && cfg.HasSessionAuth() {
				c, cerr := flags.newClient()
				if cerr != nil {
					report["credentials"] = fmt.Sprintf("skipped: %v", cerr)
				} else {
					data, status, perr := c.Post("/OpenInitialSettingsPage", map[string]any{})
					// Expensify's dispatcher returns HTTP 200 with jsonCode:407
					// on session expiry — peek at the body to detect that even
					// when auto-retry is disabled and no error surfaced.
					bodyExpired := perr == nil && hasExpiredJSONCode(data)
					errMsg := ""
					if perr != nil {
						errMsg = perr.Error()
					}
					switch {
					case bodyExpired || status == 403 || status == 407 || strings.Contains(errMsg, "HTTP 403") || strings.Contains(errMsg, "jsonCode 407") || strings.Contains(errMsg, "auto-retry exhausted"):
						sessionExpired = true
						if credsConfigured {
							report["credentials"] = "Session expired — run `auth login --headless` (stored credentials available) or `auth login`"
						} else {
							report["credentials"] = "Session expired — run `auth login --headless` after `auth store-credentials`, or `auth login` (headed)"
						}
					case perr == nil && status >= 200 && status < 300:
						// Try to extract email/displayName from response
						email := extractEmail(data)
						if email != "" {
							report["credentials"] = fmt.Sprintf("Session valid — logged in as %s", email)
						} else {
							report["credentials"] = "Session valid"
						}
					case perr != nil:
						report["credentials"] = fmt.Sprintf("API error: %v", perr)
					default:
						report["credentials"] = fmt.Sprintf("HTTP %d", status)
					}
				}
			}

			// If the live probe already flagged the session as expired, drop
			// the staleness line so the user sees one canonical ERROR rather
			// than an ERROR + a redundant WARN pointing at the same issue.
			if sessionExpired {
				delete(report, "staleness")
				delete(report, "staleness_level")
			}

			report["version"] = version

			if flags.asJSON {
				return flags.printJSON(cmd, report)
			}

			w := cmd.OutOrStdout()
			keys := []struct{ key, label string }{
				{"config", "Config"},
				{"auth", "Auth"},
				{"session_auth", "Session"},
				{"partner_auth", "Partner"},
				{"staleness", "Staleness"},
				{"credentials", "Credentials"},
			}
			for _, k := range keys {
				v, ok := report[k.key]
				if !ok {
					continue
				}
				s := fmt.Sprintf("%v", v)
				indicator := green("OK")
				// Staleness carries an explicit level key so the human
				// rendering doesn't have to parse the message string.
				if k.key == "staleness" {
					switch report["staleness_level"] {
					case "ERROR":
						indicator = red("FAIL")
					case "WARN":
						indicator = yellow("WARN")
					case "INFO":
						indicator = yellow("INFO")
					default:
						indicator = green("OK")
					}
				} else {
					switch {
					case strings.Contains(s, "error") || strings.Contains(s, "expired") || strings.Contains(s, "unreachable"):
						indicator = red("FAIL")
					case strings.Contains(s, "not configured") || strings.Contains(s, "skipped"):
						indicator = yellow("WARN")
					}
				}
				fmt.Fprintf(w, "  %s %s: %s\n", indicator, k.label, s)
			}
			for _, infoKey := range []string{"config_path", "base_url", "auth_source", "version"} {
				if v, ok := report[infoKey]; ok {
					fmt.Fprintf(w, "  %s: %v\n", infoKey, v)
				}
			}
			if hint, ok := report["auth_hint"]; ok {
				fmt.Fprintf(w, "  hint: %v\n", hint)
			}
			_ = os.Stderr
			return nil
		},
	}
}

// hasExpiredJSONCode peeks at a doctor-probe response body for the
// dispatcher's session-expired envelope. Expensify returns HTTP 200 with
// `{"jsonCode":407}` when the token is stale; we need to catch that here
// because auto-retry may be disabled (CI, --no-auto-retry, or keychain-less
// environments) and the client.do() call returns nil error in that case.
func hasExpiredJSONCode(data json.RawMessage) bool {
	if len(data) == 0 {
		return false
	}
	var env struct {
		JSONCode int `json:"jsonCode"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return false
	}
	return env.JSONCode == 407
}

// stalenessHint picks the remediation sentence for the Staleness line based on
// whether the user has headless credentials configured. The branch is the
// single difference between the two WARN messages in the plan, so keeping it
// here avoids two near-duplicate format strings.
func stalenessHint(credsConfigured bool) string {
	if credsConfigured {
		return "Run `auth login --headless` or let auto-retry refresh on next call."
	}
	return "Run `auth login` (headed) or configure headless via `auth store-credentials` + `auth login --headless`."
}

// extractEmail digs through an OpenPublicProfile response for the logged-in
// user's email. Expensify uses several different field names depending on
// the endpoint, so we look at the likely ones.
func extractEmail(data json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"email", "login", "displayName", "userEmail"} {
		if v, ok := obj[key].(string); ok && v != "" {
			return v
		}
	}
	// Nested: onyxData.session.email etc.
	if inner, ok := obj["onyxData"].(map[string]any); ok {
		for _, key := range []string{"session", "user", "personalDetails"} {
			if sub, ok := inner[key].(map[string]any); ok {
				for _, k := range []string{"email", "login", "displayName"} {
					if v, ok := sub[k].(string); ok && v != "" {
						return v
					}
				}
			}
		}
	}
	return ""
}
