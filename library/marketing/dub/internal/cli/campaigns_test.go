// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestTagNamesFrom(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want []string
	}{
		{
			name: "tagNames array",
			in:   map[string]any{"tagNames": []any{"a", "b"}},
			want: []string{"a", "b"},
		},
		{
			name: "tags structured array",
			in: map[string]any{"tags": []any{
				map[string]any{"id": "1", "name": "alpha"},
				map[string]any{"id": "2", "name": "beta"},
			}},
			want: []string{"alpha", "beta"},
		},
		{
			name: "both, deduplicated",
			in: map[string]any{
				"tagNames": []any{"alpha"},
				"tags":     []any{map[string]any{"name": "alpha"}, map[string]any{"name": "gamma"}},
			},
			want: []string{"alpha", "gamma"},
		},
		{
			name: "empty",
			in:   map[string]any{},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tagNamesFrom(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("idx %d got %q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSortCampaigns(t *testing.T) {
	rows := []campaignRow{
		{Tag: "a", TotalClicks: 100, TotalLeads: 5, TotalSales: 1, TotalLinks: 3},
		{Tag: "b", TotalClicks: 50, TotalLeads: 30, TotalSales: 10, TotalLinks: 2},
		{Tag: "c", TotalClicks: 200, TotalLeads: 1, TotalSales: 0, TotalLinks: 1},
	}
	sortCampaigns(rows, "clicks")
	if rows[0].Tag != "c" {
		t.Errorf("clicks sort: top tag = %s, want c", rows[0].Tag)
	}
	sortCampaigns(rows, "leads")
	if rows[0].Tag != "b" {
		t.Errorf("leads sort: top tag = %s, want b", rows[0].Tag)
	}
	sortCampaigns(rows, "sales")
	if rows[0].Tag != "b" {
		t.Errorf("sales sort: top tag = %s, want b", rows[0].Tag)
	}
}
