package spatial

import (
	"math"
	"testing"

	"github.com/dharmab/skyeye/pkg/bearings"
	"github.com/martinlindhe/unit"
	"github.com/paulmach/orb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectionRoundTrip(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		center orb.Point
		point  orb.Point
	}{
		{
			name:   "near center",
			center: orb.Point{-115.0, 36.0}, // Nevada
			point:  orb.Point{-115.5, 36.5},
		},
		{
			name:   "far from center",
			center: orb.Point{-115.0, 36.0},
			point:  orb.Point{-116.0, 37.0},
		},
		{
			name:   "high latitude",
			center: orb.Point{33.0, 69.0}, // Kola
			point:  orb.Point{34.0, 70.0},
		},
		{
			name:   "southern hemisphere",
			center: orb.Point{-59.0, -51.0}, // South Atlantic
			point:  orb.Point{-58.0, -50.0},
		},
		{
			name:   "same as center",
			center: orb.Point{37.0, 45.0},
			point:  orb.Point{37.0, 45.0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			proj := NewProjection(tc.center)

			// Forward projection
			projected := proj.ToProjected(tc.point)

			// Inverse projection
			recovered := proj.ToWGS84(projected)

			// Should recover original point within tolerance
			// Allow 1e-8 degree tolerance (~1mm at Earth surface)
			assert.InDelta(t, tc.point.Lon(), recovered.Lon(), 1e-8, "longitude mismatch")
			assert.InDelta(t, tc.point.Lat(), recovered.Lat(), 1e-8, "latitude mismatch")
		})
	}
}

func TestProjectedDistance(t *testing.T) {
	t.Parallel()
	// Test that projected distance is close to spherical distance for short distances
	center := orb.Point{37.0, 45.0} // Caucasus region
	proj := NewProjection(center)

	a := orb.Point{37.0, 45.0}
	b := orb.Point{37.1, 45.1}

	sphericalDist := Distance(a, b)
	projectedDist := Distance(a, b, WithProjection(proj))

	// For short distances, projected and spherical should be close
	// Allow 1% difference
	diff := math.Abs(sphericalDist.Meters()-projectedDist.Meters()) / sphericalDist.Meters()
	assert.Less(t, diff, 0.01, "distance difference should be less than 1%%")
}

func TestProjectedBearing(t *testing.T) {
	t.Parallel()
	center := orb.Point{37.0, 45.0}
	proj := NewProjection(center)

	testCases := []struct {
		name            string
		from            orb.Point
		to              orb.Point
		expectedBearing float64 // degrees
	}{
		{
			name:            "due north",
			from:            orb.Point{37.0, 45.0},
			to:              orb.Point{37.0, 46.0},
			expectedBearing: 0,
		},
		{
			name:            "due east",
			from:            orb.Point{37.0, 45.0},
			to:              orb.Point{38.0, 45.0},
			expectedBearing: 90,
		},
		{
			name:            "due south",
			from:            orb.Point{37.0, 45.0},
			to:              orb.Point{37.0, 44.0},
			expectedBearing: 180,
		},
		{
			name:            "due west",
			from:            orb.Point{37.0, 45.0},
			to:              orb.Point{36.0, 45.0},
			expectedBearing: 270,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bearing := TrueBearing(tc.from, tc.to, WithProjection(proj))

			// Normalize to 0-360
			actual := bearing.Degrees()
			if actual < 0 {
				actual += 360
			}

			// Allow 5 degree tolerance due to projection distortion
			diff := math.Abs(actual - tc.expectedBearing)
			if diff > 180 {
				diff = 360 - diff
			}
			assert.Less(t, diff, 5.0, "bearing %v should be close to %v", actual, tc.expectedBearing)
		})
	}
}

func TestProjectedPointAtBearingAndDistance(t *testing.T) {
	t.Parallel()
	center := orb.Point{37.0, 45.0}
	proj := NewProjection(center)
	origin := orb.Point{37.0, 45.0}

	testCases := []struct {
		name     string
		bearing  float64 // degrees
		distance unit.Length
	}{
		{
			name:     "10nm north",
			bearing:  0,
			distance: 10 * unit.NauticalMile,
		},
		{
			name:     "10nm east",
			bearing:  90,
			distance: 10 * unit.NauticalMile,
		},
		{
			name:     "50nm northeast",
			bearing:  45,
			distance: 50 * unit.NauticalMile,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bearing := bearings.NewTrueBearing(unit.Angle(tc.bearing) * unit.Degree)

			// Calculate destination point
			dest := PointAtBearingAndDistance(origin, bearing, tc.distance, WithProjection(proj))

			// Verify by calculating distance and bearing back
			calculatedDist := Distance(origin, dest, WithProjection(proj))
			calculatedBearing := TrueBearing(origin, dest, WithProjection(proj))

			// Distance should match within 1%
			distDiff := math.Abs(calculatedDist.Meters()-tc.distance.Meters()) / tc.distance.Meters()
			assert.Less(t, distDiff, 0.01, "distance should match within 1%%")

			// Bearing should match within 1 degree
			bearingDiff := math.Abs(calculatedBearing.Degrees() - tc.bearing)
			if bearingDiff > 180 {
				bearingDiff = 360 - bearingDiff
			}
			assert.Less(t, bearingDiff, 1.0, "bearing should match within 1 degree")
		})
	}
}

func TestWithTransverseMercatorProjection(t *testing.T) {
	t.Parallel()
	center := orb.Point{37.0, 45.0}
	a := orb.Point{37.0, 45.0}
	b := orb.Point{37.1, 45.1}

	// Using WithProjection with NewProjection
	proj := NewProjection(center)
	dist1 := Distance(a, b, WithProjection(proj))

	// Using WithTransverseMercatorProjection directly
	dist2 := Distance(a, b, WithTransverseMercatorProjection(center))

	// Should produce identical results
	assert.InDelta(t, dist1.Meters(), dist2.Meters(), 1e-9)
}

func TestProjectionCenter(t *testing.T) {
	t.Parallel()
	// When projecting the center point, it should be at (0, 0)
	center := orb.Point{37.0, 45.0}
	proj := NewProjection(center)

	projected := proj.ToProjected(center)

	require.InDelta(t, 0, projected[0], 1e-6, "easting of center should be 0")
	require.InDelta(t, 0, projected[1], 1e-6, "northing of center should be 0")
}
