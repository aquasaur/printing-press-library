// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// CLI doctor command — checks config, auth, connectivity.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/config"

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

			// Session validation: try OpenPublicProfile if a session token is configured.
			if cfg != nil && cfg.HasSessionAuth() {
				c, cerr := flags.newClient()
				if cerr != nil {
					report["credentials"] = fmt.Sprintf("skipped: %v", cerr)
				} else {
					data, status, perr := c.Post("/OpenInitialSettingsPage", map[string]any{})
					switch {
					case perr == nil && status >= 200 && status < 300:
						// Try to extract email/displayName from response
						email := extractEmail(data)
						if email != "" {
							report["credentials"] = fmt.Sprintf("Session valid — logged in as %s", email)
						} else {
							report["credentials"] = "Session valid"
						}
					case status == 403 || status == 407 || strings.Contains(fmt.Sprintf("%v", perr), "HTTP 403"):
						report["credentials"] = "Session expired — run `auth login` again"
					case perr != nil:
						report["credentials"] = fmt.Sprintf("API error: %v", perr)
					default:
						report["credentials"] = fmt.Sprintf("HTTP %d", status)
					}
				}
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
				{"credentials", "Credentials"},
			}
			for _, k := range keys {
				v, ok := report[k.key]
				if !ok {
					continue
				}
				s := fmt.Sprintf("%v", v)
				indicator := green("OK")
				switch {
				case strings.Contains(s, "error") || strings.Contains(s, "expired") || strings.Contains(s, "unreachable"):
					indicator = red("FAIL")
				case strings.Contains(s, "not configured") || strings.Contains(s, "skipped"):
					indicator = yellow("WARN")
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
