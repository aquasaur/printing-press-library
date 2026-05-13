package fuelcheck

import (
	"encoding/json"
	"regexp"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewTransactionID(t *testing.T) {
	id := newTransactionID()
	if !uuidRe.MatchString(id) {
		t.Fatalf("transaction id %q is not a v4 UUID", id)
	}
	if newTransactionID() == id {
		t.Fatalf("two consecutive transaction ids collided")
	}
}

func TestParseSeconds(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{float64(43199), 43199},
		{"43199", 43199},
		{json.Number("3600"), 3600},
		{nil, 0},
		{"not-a-number", 0},
	}
	for _, c := range cases {
		if got := parseSeconds(c.in); got != c.want {
			t.Errorf("parseSeconds(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestMissingCredsError(t *testing.T) {
	var err error = MissingCredsError{}
	if err.Error() == "" {
		t.Fatal("MissingCredsError should have a message")
	}
}
