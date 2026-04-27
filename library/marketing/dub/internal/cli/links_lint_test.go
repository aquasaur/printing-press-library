// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestIsLookalike(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"launch", "launches", false}, // 2-char diff
		{"launch", "launchs", true},   // single trailing char
		{"hello", "hello", false},
		{"hello", "Hello", false}, // case differs, not trailing
		{"x", "xy", true},
		{"abc", "abcz", true},
		{"abc", "abc!", false}, // not alphanumeric
	}
	for _, tc := range cases {
		t.Run(tc.a+"-"+tc.b, func(t *testing.T) {
			got := isLookalike(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("isLookalike(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestLintSlugsReserved(t *testing.T) {
	in := []struct{ domain, slug string }{
		{"dub.sh", "admin"},
		{"dub.sh", "ok-link"},
		{"dub.sh", "API"}, // case-insensitive reserved match
		{"dub.sh", "x"},   // too short
	}
	got := lintSlugs(in)
	codes := make(map[string]int)
	for _, f := range got {
		codes[f.Code]++
	}
	if codes["reserved-slug"] < 2 { // admin and API
		t.Errorf("expected >=2 reserved-slug findings, got %d", codes["reserved-slug"])
	}
	if codes["slug-too-short"] != 1 {
		t.Errorf("expected 1 slug-too-short finding, got %d", codes["slug-too-short"])
	}
}

func TestLintSlugsCaseCollision(t *testing.T) {
	in := []struct{ domain, slug string }{
		{"dub.sh", "Launch"},
		{"dub.sh", "launch"},
		{"dub.sh", "LAUNCH"},
	}
	got := lintSlugs(in)
	hit := false
	for _, f := range got {
		if f.Code == "case-collision" {
			hit = true
			if len(f.Related) < 2 {
				t.Errorf("expected >=2 related variants, got %d", len(f.Related))
			}
		}
	}
	if !hit {
		t.Error("expected at least one case-collision finding")
	}
}

func TestLintSlugsLookalike(t *testing.T) {
	in := []struct{ domain, slug string }{
		{"dub.sh", "launch"},
		{"dub.sh", "launchs"},
		{"dub.sh", "unrelated"},
	}
	got := lintSlugs(in)
	hit := false
	for _, f := range got {
		if f.Code == "lookalike-slug" {
			hit = true
		}
	}
	if !hit {
		t.Error("expected at least one lookalike-slug finding for launch/launchs")
	}
}
