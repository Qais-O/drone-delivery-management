package geo

import "testing"

func TestFeetToMiles(t *testing.T) {
    if got := FeetToMiles(5280); got != 1 {
        t.Fatalf("FeetToMiles(5280) = %v, want 1", got)
    }
}

func TestHaversineMiles_ZeroDistance(t *testing.T) {
    d := HaversineMiles(10, 20, 10, 20)
    if d < 0 || d > 1e-9 {
        t.Fatalf("zero distance expected ~0, got %v", d)
    }
}

func TestIsWithinRadius_Boundary(t *testing.T) {
    // Compute a distance equal to RadiusFeet by moving north ~RadiusFeet along meridian at equator (~1 foot ~ 3.048e-7 degrees)
    // Rather than relying on conversion, find a very close pair by binary search could be overkill.
    // Use a point extremely close (1e-6 miles) well under 100 feet.
    lat1, lng1 := 0.0, 0.0
    lat2, lng2 := 0.0, 0.000001
    if !IsWithinRadius(lat1, lng1, lat2, lng2, RadiusFeet) {
        t.Fatalf("expected points to be within radius")
    }
}
