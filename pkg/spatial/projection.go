package spatial

import (
	"math"

	"github.com/paulmach/orb"
)

// WGS84 ellipsoid parameters.
const (
	// wgs84SemiMajorAxis is the semi-major axis of the WGS84 ellipsoid in meters.
	wgs84SemiMajorAxis = 6378137.0
	// wgs84Flattening is the flattening of the WGS84 ellipsoid.
	wgs84Flattening = 1.0 / 298.257223563
)

// Derived WGS84 parameters.
var (
	wgs84SemiMinorAxis             = wgs84SemiMajorAxis * (1 - wgs84Flattening)
	wgs84EccentricitySquared       = (wgs84SemiMajorAxis*wgs84SemiMajorAxis - wgs84SemiMinorAxis*wgs84SemiMinorAxis) / (wgs84SemiMajorAxis * wgs84SemiMajorAxis)
	wgs84SecondEccentricitySquared = (wgs84SemiMajorAxis*wgs84SemiMajorAxis - wgs84SemiMinorAxis*wgs84SemiMinorAxis) / (wgs84SemiMinorAxis * wgs84SemiMinorAxis)
)

// Projection provides coordinate transformation between WGS84 geographic
// coordinates and a Transverse Mercator projected coordinate system.
//
// DCS World uses Transverse Mercator projections for its terrain coordinate
// systems. Each terrain has a different projection center. Using this
// projection for spatial calculations improves accuracy compared to spherical
// calculations, especially at extreme latitudes.
type Projection struct {
	// centralMeridian is the longitude of the projection center in radians.
	centralMeridian float64
	// originLatitude is the latitude of the projection center in radians.
	originLatitude float64
	// scaleFactor is the scale factor at the central meridian.
	scaleFactor float64
	// falseEasting is added to all X coordinates.
	falseEasting float64
	// falseNorthing is added to all Y coordinates.
	falseNorthing float64
}

// NewProjection creates a Transverse Mercator projection centered on the given point.
// The point should be specified as orb.Point{longitude, latitude} in degrees.
// Deprecated: Use NewProjectionFromParams for accurate DCS terrain projections.
func NewProjection(center orb.Point) *Projection {
	return &Projection{
		centralMeridian: center.Lon() * math.Pi / 180.0,
		originLatitude:  center.Lat() * math.Pi / 180.0,
		scaleFactor:     1.0,
		falseEasting:    0.0,
		falseNorthing:   0.0,
	}
}

// NewTransverseMercator creates a Transverse Mercator projection with explicit parameters.
// This matches the projection parameters used by DCS World terrains.
// - centralMeridian: longitude of the central meridian in degrees
// - scaleFactor: scale factor at the central meridian (DCS uses 0.9996)
// - falseEasting: offset added to X coordinates in meters
// - falseNorthing: offset added to Y coordinates in meters
// The origin latitude is always 0 (equator) for DCS projections.
func NewTransverseMercator(centralMeridian, scaleFactor, falseEasting, falseNorthing float64) *Projection {
	return &Projection{
		centralMeridian: centralMeridian * math.Pi / 180.0,
		originLatitude:  0, // DCS always uses equator as origin
		scaleFactor:     scaleFactor,
		falseEasting:    falseEasting,
		falseNorthing:   falseNorthing,
	}
}

// ToProjected converts WGS84 geographic coordinates to projected (X, Y) in meters.
// The input point should be orb.Point{longitude, latitude} in degrees.
// Returns orb.Point{easting, northing} in meters.
func (p *Projection) ToProjected(point orb.Point) orb.Point {
	// Convert to radians
	lon := point.Lon() * math.Pi / 180.0
	lat := point.Lat() * math.Pi / 180.0

	// Compute parameters
	e2 := wgs84EccentricitySquared
	ep2 := wgs84SecondEccentricitySquared
	a := wgs84SemiMajorAxis
	k0 := p.scaleFactor

	// Difference in longitude from central meridian
	deltaLon := lon - p.centralMeridian

	// Trigonometric values
	sinLat := math.Sin(lat)
	cosLat := math.Cos(lat)
	tanLat := math.Tan(lat)

	// Radius of curvature in the prime vertical
	N := a / math.Sqrt(1-e2*sinLat*sinLat)

	// Compute T, C, A, M
	T := tanLat * tanLat
	C := ep2 * cosLat * cosLat
	A := deltaLon * cosLat

	// Meridional arc
	M := meridionalArc(lat)
	M0 := meridionalArc(p.originLatitude)

	// Compute easting (X)
	A2 := A * A
	A3 := A2 * A
	A4 := A3 * A
	A5 := A4 * A
	A6 := A5 * A

	easting := k0 * N * (A +
		(1-T+C)*A3/6 +
		(5-18*T+T*T+72*C-58*ep2)*A5/120)

	// Compute northing (Y)
	northing := k0 * ((M - M0) +
		N*tanLat*(A2/2+
			(5-T+9*C+4*C*C)*A4/24+
			(61-58*T+T*T+600*C-330*ep2)*A6/720))

	return orb.Point{
		easting + p.falseEasting,
		northing + p.falseNorthing,
	}
}

// ToWGS84 converts projected coordinates back to WGS84 geographic coordinates.
// The input point should be orb.Point{easting, northing} in meters.
// Returns orb.Point{longitude, latitude} in degrees.
func (p *Projection) ToWGS84(projected orb.Point) orb.Point {
	// Remove false easting/northing
	easting := projected[0] - p.falseEasting
	northing := projected[1] - p.falseNorthing

	// Parameters
	e2 := wgs84EccentricitySquared
	ep2 := wgs84SecondEccentricitySquared
	a := wgs84SemiMajorAxis
	k0 := p.scaleFactor

	// Meridional arc at origin
	M0 := meridionalArc(p.originLatitude)

	// Find footprint latitude using Newton-Raphson iteration
	M := M0 + northing/k0
	mu := M / (a * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256))

	// Compute footprint latitude
	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))
	phi1 := mu +
		(3*e1/2-27*e1*e1*e1/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*e1*e1*e1*e1/32)*math.Sin(4*mu) +
		(151*e1*e1*e1/96)*math.Sin(6*mu) +
		(1097*e1*e1*e1*e1/512)*math.Sin(8*mu)

	// Compute parameters at footprint latitude
	sinPhi1 := math.Sin(phi1)
	cosPhi1 := math.Cos(phi1)
	tanPhi1 := math.Tan(phi1)

	N1 := a / math.Sqrt(1-e2*sinPhi1*sinPhi1)
	R1 := a * (1 - e2) / math.Pow(1-e2*sinPhi1*sinPhi1, 1.5)
	T1 := tanPhi1 * tanPhi1
	C1 := ep2 * cosPhi1 * cosPhi1
	D := easting / (N1 * k0)

	// Compute latitude
	D2 := D * D
	D3 := D2 * D
	D4 := D3 * D
	D5 := D4 * D
	D6 := D5 * D

	lat := phi1 - (N1*tanPhi1/R1)*(D2/2-
		(5+3*T1+10*C1-4*C1*C1-9*ep2)*D4/24+
		(61+90*T1+298*C1+45*T1*T1-252*ep2-3*C1*C1)*D6/720)

	// Compute longitude
	lon := p.centralMeridian + (D-
		(1+2*T1+C1)*D3/6+
		(5-2*C1+28*T1-3*C1*C1+8*ep2+24*T1*T1)*D5/120)/cosPhi1

	// Convert to degrees
	return orb.Point{
		lon * 180.0 / math.Pi,
		lat * 180.0 / math.Pi,
	}
}

// meridionalArc computes the meridional arc distance from the equator to
// the given latitude (in radians).
func meridionalArc(lat float64) float64 {
	e2 := wgs84EccentricitySquared
	a := wgs84SemiMajorAxis

	// Series expansion coefficients
	e4 := e2 * e2
	e6 := e4 * e2

	A0 := 1 - e2/4 - 3*e4/64 - 5*e6/256
	A2 := 3.0 / 8.0 * (e2 + e4/4 + 15*e6/128)
	A4 := 15.0 / 256.0 * (e4 + 3*e6/4)
	A6 := 35 * e6 / 3072

	return a * (A0*lat -
		A2*math.Sin(2*lat) +
		A4*math.Sin(4*lat) -
		A6*math.Sin(6*lat))
}
