package geo

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		wantKm                 float64
		tol                    float64
	}{
		{"identical point", -33.8688, 151.2093, -33.8688, 151.2093, 0, 0.001},
		// Sydney Central (~ -33.8830, 151.2069) to Town Hall (~ -33.8731, 151.2071): ~1.1 km
		{"central to town hall", -33.8830, 151.2069, -33.8731, 151.2071, 1.1, 0.3},
		// Sydney to Parramatta: roughly 23 km as the crow flies.
		{"sydney to parramatta", -33.8688, 151.2093, -33.8150, 151.0011, 23, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Haversine(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(got-tt.wantKm) > tt.tol {
				t.Fatalf("Haversine = %.3f km, want %.3f ± %.3f", got, tt.wantKm, tt.tol)
			}
		})
	}
}

func TestHaversineSymmetry(t *testing.T) {
	a := Haversine(-33.87, 151.21, -33.81, 151.00)
	b := Haversine(-33.81, 151.00, -33.87, 151.21)
	if math.Abs(a-b) > 1e-9 {
		t.Fatalf("Haversine not symmetric: %v vs %v", a, b)
	}
}

func TestHaversineMonotonic(t *testing.T) {
	near := Haversine(-33.87, 151.21, -33.88, 151.22)
	far := Haversine(-33.87, 151.21, -34.00, 151.00)
	if near >= far {
		t.Fatalf("nearer point should be closer: near=%v far=%v", near, far)
	}
}
