package geo

import "math"

const (
	// RadiusFeet is the pickup/delivery location accuracy threshold in feet.
	RadiusFeet = 100.0
	// FeetPerMile is the conversion factor from feet to miles.
	FeetPerMile = 5280.0
	// EarthRadiusMiles is Earth's radius in miles for Haversine calculation.
	EarthRadiusMiles = 3958.7613
)

// FeetToMiles converts feet to miles.
func FeetToMiles(f float64) float64 {
	return f / FeetPerMile
}

// HasersineMiles calculates the great-circle distance between two points
// on Earth in miles using the Haversine formula.
func HaversineMiles(lat1, lng1, lat2, lng2 float64) float64 {
	const degToRad = math.Pi / 180
	dLat := (lat2 - lat1) * degToRad
	dLng := (lng2 - lng1) * degToRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*degToRad)*math.Cos(lat2*degToRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return EarthRadiusMiles * c
}

// IsWithinRadius checks if two coordinates are within the specified radius (in feet).
func IsWithinRadius(lat1, lng1, lat2, lng2 float64, radiusFeet float64) bool {
	distanceMiles := HaversineMiles(lat1, lng1, lat2, lng2)
	distanceFeet := distanceMiles * FeetPerMile
	return distanceFeet <= radiusFeet
}
