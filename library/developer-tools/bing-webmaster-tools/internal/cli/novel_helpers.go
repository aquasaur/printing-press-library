// Hand-authored (not generated): shared helpers for the novel transcendence
// commands (sites health, sites triage, traffic ctr-gaps, keywords
// cannibalization, submit smart, crawl triage).
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster-tools/internal/client"
)

// jsonNumber coerces a JSON value (number or numeric string) to float64.
func jsonNum(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(n), "%g", &f)
		return f
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

// getNum reads a numeric field from a decoded JSON object, tolerating numbers
// encoded as strings.
func getNum(m map[string]any, key string) float64 { return jsonNum(m[key]) }

// getInt reads an integer field from a decoded JSON object.
func getInt(m map[string]any, key string) int { return int(jsonNum(m[key])) }

// getStr reads a string field from a decoded JSON object.
func getStr(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	if m[key] == nil {
		return ""
	}
	return fmt.Sprintf("%v", m[key])
}

// firstStr returns the first non-empty string field from the given keys.
func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := getStr(m, k); s != "" {
			return s
		}
	}
	return ""
}

// firstNum returns the first present numeric field from the given keys.
func firstNum(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return getNum(m, k)
		}
	}
	return 0
}

// getList fetches a Bing Webmaster GET endpoint and decodes the (envelope-
// unwrapped) response as a list of JSON objects. A response that is a single
// object or null is normalized to a zero/one-element slice.
func getList(c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	raw, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	return decodeList(raw)
}

func decodeList(raw json.RawMessage) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	switch trimmed[0] {
	case '[':
		var items []map[string]any
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		return items, nil
	case '{':
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		return []map[string]any{obj}, nil
	}
	return nil, fmt.Errorf("unexpected response shape: %.40s", trimmed)
}

// getObject fetches a Bing Webmaster GET endpoint and decodes the response as
// a single JSON object.
func getObject(c *client.Client, path string, params map[string]string) (map[string]any, error) {
	raw, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	items, err := decodeList(raw)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return map[string]any{}, nil
	}
	return items[0], nil
}

// crawlIssueCategories decodes a Bing Webmaster Tools CrawlIssueType bitflag
// integer into human-readable category names. The bit→name mapping mirrors the
// documented CrawlIssueType flags enum; bits without a known name are reported
// as "flag_<bit>".
//
//nolint:gocyclo // a flat switch over known flag bits is the clearest form here
func crawlIssueCategories(flags int) []string {
	if flags <= 0 {
		return nil
	}
	known := []struct {
		bit  int
		name string
	}{
		{1, "http_400_404_not_found"},
		{2, "robots_txt_blocked"},
		{4, "important_url_blocked_by_robots_txt"},
		{8, "not_indexed"},
		{16, "outdated_content"},
		{32, "content_too_large"},
		{64, "redirect_chain_or_loop"},
		{128, "http_500_server_error"},
		{256, "blocked_by_meta_robots_noindex"},
		{512, "malware_or_security_issue"},
		{1024, "dns_or_connectivity_error"},
		{2048, "duplicate_content"},
	}
	var out []string
	matched := 0
	for _, k := range known {
		if flags&k.bit != 0 {
			out = append(out, k.name)
			matched |= k.bit
		}
	}
	for bit := 1; bit <= flags; bit <<= 1 {
		if flags&bit != 0 && matched&bit == 0 {
			out = append(out, fmt.Sprintf("flag_%d", bit))
		}
	}
	return out
}

// sortByIntDesc sorts a slice of rows by a numeric key descending; ties keep
// stable order.
func sortRowsDesc[T any](rows []T, key func(T) float64) {
	sort.SliceStable(rows, func(i, j int) bool { return key(rows[i]) > key(rows[j]) })
}

// readURLLines splits newline-delimited text into a deduplicated, trimmed,
// order-preserving list of URLs, skipping blanks and # comments.
func readURLLines(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		u := strings.TrimSpace(line)
		if u == "" || strings.HasPrefix(u, "#") {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}
