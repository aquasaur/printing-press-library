// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		ignoreUTM bool
		want      string
	}{
		{"no flag returns raw", "https://x.com/a?utm_source=x", false, "https://x.com/a?utm_source=x"},
		{"strip utm_ params", "https://x.com/a?utm_source=fb&utm_campaign=spring", true, "https://x.com/a"},
		{"keep non-utm params", "https://x.com/a?id=42&utm_source=fb", true, "https://x.com/a?id=42"},
		{"trim trailing slash", "https://x.com/", true, "https://x.com"},
		{"sort remaining params for stability", "https://x.com/a?b=2&a=1&utm_source=x", true, "https://x.com/a?a=1&b=2"},
		{"no query string", "https://x.com/a", true, "https://x.com/a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeURL(tc.in, tc.ignoreUTM)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
