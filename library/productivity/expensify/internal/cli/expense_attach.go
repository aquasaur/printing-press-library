// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense attach` — reads a local receipt file and attaches it to an expense
// by sending the file bytes as a form field.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newExpenseAttachCmd(flags *rootFlags) *cobra.Command {
	var transactionID, receiptPath string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:     "attach",
		Short:   "Attach or replace a receipt on an expense",
		Example: "  expensify-pp-cli expense attach --transaction-id 12345 --receipt-path ./receipt.jpg",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stdinBody {
				if !cmd.Flags().Changed("transaction-id") && !flags.dryRun {
					return fmt.Errorf("required flag %q not set", "transaction-id")
				}
				if !cmd.Flags().Changed("receipt-path") && !flags.dryRun {
					return fmt.Errorf("required flag %q not set", "receipt-path")
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ReplaceReceipt"
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				if err := json.Unmarshal(stdinData, &body); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
			} else {
				body = map[string]any{}
				if transactionID != "" {
					body["transactionID"] = transactionID
				}
				if receiptPath != "" {
					// Resolve path and read the file. Expensify's ReplaceReceipt
					// expects the receipt bytes — the client's form-encoder will
					// pass strings as-is, so we base64-encode the bytes and also
					// send the filename. This keeps the binary safe through the
					// form-encoding pipeline.
					abs, err := filepath.Abs(receiptPath)
					if err != nil {
						return fmt.Errorf("resolving receipt path %q: %w", receiptPath, err)
					}
					data, err := os.ReadFile(abs)
					if err != nil {
						return fmt.Errorf("reading receipt file %q: %w", abs, err)
					}
					body["filename"] = filepath.Base(abs)
					body["receipt"] = base64.StdEncoding.EncodeToString(data)
					body["receipt_path"] = abs
				}
			}
			data, statusCode, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err)
			}
			w := cmd.OutOrStdout()
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"action":   "post",
					"resource": "expense",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
					"data":     json.RawMessage(data),
				})
			}
			fmt.Fprintf(w, "Attached %s to expense %s (HTTP %d)\n", receiptPath, transactionID, statusCode)
			return nil
		},
	}
	cmd.Flags().StringVar(&transactionID, "transaction-id", "", "Expense transaction ID")
	cmd.Flags().StringVar(&receiptPath, "receipt-path", "", "Local path to a receipt image (jpg, png, pdf)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")
	return cmd
}
