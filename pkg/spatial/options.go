package spatial

import "github.com/paulmach/orb"

// options holds configuration for spatial calculations.
type options struct {
	projection *Projection
}

// Option configures spatial calculations.
type Option func(*options)

// WithProjection uses the given Transverse Mercator projection for calculations.
// When a projection is provided, distance and bearing calculations use planar
// geometry on the projected coordinate system instead of spherical geometry.
//
// This improves accuracy when working with DCS World coordinates, which use
// Transverse Mercator projections internally.
func WithProjection(p *Projection) Option {
	return func(o *options) {
		o.projection = p
	}
}

// WithTransverseMercatorProjection creates a Transverse Mercator projection
// centered on the given point and uses it for calculations.
//
// This is a convenience function that combines NewProjection and WithProjection.
func WithTransverseMercatorProjection(center orb.Point) Option {
	return func(o *options) {
		o.projection = NewProjection(center)
	}
}

// applyOptions applies all options to a new options struct.
func applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
