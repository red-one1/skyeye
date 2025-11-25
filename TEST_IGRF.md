# Test IGRF Function

This program allows you to test the IGRF function with custom parameters.

## Usage

```bash
# Run with default values (New York City)
go run test_igrf.go

# Run with custom values
go run test_igrf.go <latitude> <longitude> <altitude_km> <year>

# Examples:
go run test_igrf.go 40.7128 -74.0060 0.1 2023.5
go run test_igrf.go 37.7749 -122.4194 0.0 2023.0
go run test_igrf.go 51.5074 -0.1278 0.2 2020.0
```

## Parameter Limits

- Latitude: -90.0 to 90.0 degrees
- Longitude: -180.0 to 180.0 degrees
- Altitude: -1.0 to 600.0 km
- Date: 1900.0 to 2025.0 (decimal year)

## Output

The program displays the following geomagnetic field components:
- Declination and its secular variation (annual change)
- Inclination and its secular variation
- Horizontal intensity and its secular variation
- North and East components with their secular variations
- Vertical component and its secular variation
- Total intensity and its secular variation

All magnetic field values are in nanotesla (nT), and secular variations are in nT per year.
Declination and inclination are in degrees, with their secular variations in minutes of arc per year.