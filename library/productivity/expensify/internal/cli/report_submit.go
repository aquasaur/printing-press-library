// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `report submit` — submits a report; optional --wait polls until status leaves SUBMITTED.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newReportSubmitCmd(flags *rootFlags) *cobra.Command {
	var bodyReportID string
	var bodyNote string
	var stdinBody bool
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:     "submit [report-id]",
		Short:   "Submit a report for approval (optionally wait for status change)",
		Example: "  expensify-pp-cli report submit REPORT_ID_HERE\n  expensify-pp-cli report submit REPORT_ID_HERE --wait --timeout 1h\n  expensify-pp-cli report submit --report-id REPORT_ID_HERE --note \"April expenses\"",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Accept report ID as either positional arg or --report-id flag.
			if len(args) > 0 && bodyReportID == "" {
				bodyReportID = args[0]
			}
			if !stdinBody {
				if bodyReportID == "" && !flags.dryRun {
					return fmt.Errorf("report ID required: pass as positional arg or --report-id")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/SubmitReport"
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				if id, ok := body["reportID"].(string); ok {
					bodyReportID = id
				}
			} else {
				body = map[string]any{}
				if bodyReportID != "" {
					body["reportID"] = bodyReportID
				}
				if bodyNote != "" {
					body["note"] = bodyNote
				}
			}
			data, statusCode, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err)
			}

			w := cmd.OutOrStdout()
			if wait && !flags.dryRun {
				if bodyReportID == "" {
					fmt.Fprintln(os.Stderr, "warning: --wait requires --report-id; skipping poll")
				} else {
					final, werr := waitForSubmitExit(c, bodyReportID, timeout, w)
					if werr != nil {
						return werr
					}
					fmt.Fprintf(w, "Report %s transitioned to status %s\n", bodyReportID, final)
					if flags.asJSON {
						envelope := map[string]any{
							"reportID":   bodyReportID,
							"status":     statusCode,
							"finalState": final,
						}
						return flags.printJSON(cmd, envelope)
					}
					return nil
				}
			}

			// Default path: standard envelope output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if flags.quiet {
					return nil
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "report",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(data) > 0 {
					var parsed any
					if err := json.Unmarshal(data, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				return flags.printJSON(cmd, envelope)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyReportID, "report-id", "", "Report ID to submit")
	cmd.Flags().StringVar(&bodyNote, "note", "", "Note to approver")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	cmd.Flags().BoolVar(&wait, "wait", false, "After submit, poll the report until status leaves SUBMITTED")
	cmd.Flags().DurationVar(&timeout, "timeout", time.Hour, "Timeout for --wait (e.g. 1h, 30m)")
	return cmd
}

// waitForSubmitExit polls /OpenReport every 30s until the report's status is
// something other than SUBMITTED, or the timeout expires.
func waitForSubmitExit(c interface {
	Post(path string, body any) (json.RawMessage, int, error)
}, reportID string, timeout time.Duration, w io.Writer) (string, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		if time.Now().After(deadline) {
			return last, fmt.Errorf("timed out waiting for report %s to leave SUBMITTED (last status: %s)", reportID, last)
		}
		data, status, err := c.Post("/OpenReport", map[string]any{"reportID": reportID})
		if err != nil {
			return last, classifyAPIError(err)
		}
		if status < 200 || status >= 300 {
			return last, apiErr(fmt.Errorf("OpenReport returned HTTP %d", status))
		}
		curr := extractReportStatus(data)
		last = curr
		upper := strings.ToUpper(curr)
		if upper != "" && upper != "SUBMITTED" && upper != "PROCESSING" {
			return curr, nil
		}
		fmt.Fprintf(w, "report %s status: %s — polling again in 30s...\n", reportID, curr)
		time.Sleep(30 * time.Second)
	}
}

func extractReportStatus(data json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	for _, k := range []string{"status", "state", "stateNum", "statusNum"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
		if v, ok := m[k].(float64); ok && v != 0 {
			return fmt.Sprintf("%d", int64(v))
		}
	}
	if r, ok := m["report"].(map[string]any); ok {
		for _, k := range []string{"status", "state", "stateNum"} {
			if v, ok := r[k].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}
