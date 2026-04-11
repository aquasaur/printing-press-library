package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

// DefaultRegistryURL points at a community-maintained hash registry. When
// Instacart rolls a new web bundle and built-in hashes go stale, someone
// with printing-press installed re-sniffs and PRs an updated hashes.json
// to this URL. All CLI users then get fresh hashes by running
// `instacart capture --remote`. A user-hostile story on its face, but it
// scales: one maintainer per rotation serves every CLI installation.
const DefaultRegistryURL = "https://raw.githubusercontent.com/mvanhorn/instacart-pp-cli/main/hashes.json"

// registryPayload is the shape of the remote hashes.json file:
//
//	{
//	  "version": 1,
//	  "updated_at": "2026-04-11T00:00:00Z",
//	  "operations": {
//	    "CurrentUserFields": "d7d1050d...",
//	    "UpdateCartItemsMutation": "a33745461a..."
//	  }
//	}
type registryPayload struct {
	Version    int               `json:"version"`
	UpdatedAt  string            `json:"updated_at"`
	Operations map[string]string `json:"operations"`
}

func newCaptureCmd() *cobra.Command {
	var remoteFlag bool
	var registryURL string
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Refresh the local GraphQL operation hash cache",
		Long: `Instacart's web client uses Apollo persisted queries: the server only
accepts queries whose sha256 hash is in its allowlist. When Instacart ships
a new web bundle, old hashes can stop working.

Without flags, 'capture' re-seeds the cache from the hashes compiled into
this binary. Pass --remote to additionally fetch a community-maintained
registry of fresh hashes; this is how the CLI recovers from a bundle
rotation without rebuilding.

For now the remote path is a no-op unless you have a working registry URL
(the default URL may or may not exist yet -- override with --registry).`,
		Example: `  instacart capture
  instacart capture --remote
  instacart capture --remote --registry https://my.host/instacart-hashes.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			builtIn := 0
			for _, name := range instacart.OpNames() {
				seed := instacart.DefaultOps[name]
				if seed.Hash == "" {
					continue
				}
				if err := app.Store.UpsertOp(store.Op{OperationName: name, Sha256Hash: seed.Hash}); err != nil {
					return err
				}
				builtIn++
			}
			fmt.Fprintf(cmd.OutOrStdout(), "seeded %d built-in GraphQL operations\n", builtIn)

			if remoteFlag {
				if registryURL == "" {
					registryURL = DefaultRegistryURL
				}
				fetched, err := fetchRegistry(app.Ctx, registryURL)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "warning: remote registry fetch failed: %v\n", err)
					fmt.Fprintln(cmd.OutOrStderr(), "built-in hashes are still loaded; if they're stale, install browser-use and open an issue")
					return nil
				}
				updated := 0
				for name, hash := range fetched.Operations {
					if err := app.Store.UpsertOp(store.Op{OperationName: name, Sha256Hash: hash}); err != nil {
						return err
					}
					updated++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "merged %d operations from remote registry (updated %s)\n", updated, fetched.UpdatedAt)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "run `instacart doctor` to verify the session and API are healthy")
			return nil
		},
	}
	cmd.Flags().BoolVar(&remoteFlag, "remote", false, "Fetch the community hash registry from --registry URL and merge into the local cache")
	cmd.Flags().StringVar(&registryURL, "registry", "", "Override the default registry URL (see DefaultRegistryURL in source)")
	return cmd
}

// fetchRegistry GETs a hashes.json from a URL and parses it.
func fetchRegistry(ctx interface{}, url string) (*registryPayload, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "instacart-pp-cli/1.0 (+hash-registry-fetch)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		return nil, err
	}
	var out registryPayload
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("parse registry JSON: %w", err)
	}
	if out.Version != 1 {
		return nil, fmt.Errorf("unsupported registry version %d (CLI supports v1)", out.Version)
	}
	if len(out.Operations) == 0 {
		return nil, fmt.Errorf("registry has zero operations")
	}
	return &out, nil
}
