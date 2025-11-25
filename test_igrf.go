package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/proway2/go-igrf/igrf"
)

func main() {
	// Create a new IGRF instance
	igd := igrf.New()

	// Check if we have command line arguments
	if len(os.Args) != 5 {
		fmt.Println("Usage: test_igrf.go <latitude> <longitude> <altitude_km> <year>")
		fmt.Println("Example: test_igrf.go 40.0 -75.0 0.1 2023.5")
		fmt.Println("\nUsing default values for New York City...")
		testIGRF(igd, 40.7128, -74.0060, 0.1, 2023.5)
		return
	}

	// Parse command line arguments
	lat, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		log.Fatal("Invalid latitude:", err)
	}

	lon, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		log.Fatal("Invalid longitude:", err)
	}

	alt, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		log.Fatal("Invalid altitude:", err)
	}

	date, err := strconv.ParseFloat(os.Args[4], 64)
	if err != nil {
		log.Fatal("Invalid date:", err)
	}

	testIGRF(igd, lat, lon, alt, date)
}

func testIGRF(igd *igrf.IGRFdata, lat, lon, alt, date float64) {
	fmt.Printf("Testing IGRF with parameters:\n")
	fmt.Printf("  Latitude:  %.4f°\n", lat)
	fmt.Printf("  Longitude: %.4f°\n", lon)
	fmt.Printf("  Altitude:  %.1f km\n", alt)
	fmt.Printf("  Date:      %.1f\n", date)
	fmt.Println()

	// Validate input parameters
	if err := validateInputs(lat, lon, alt, date); err != nil {
		log.Fatal("Invalid input parameters:", err)
	}

	// Call the IGRF function
	results, err := igd.IGRF(lat, lon, alt, date)
	if err != nil {
		log.Fatal("Error calling IGRF:", err)
	}

	// Display results
	fmt.Println("IGRF Results:")
	fmt.Printf("  Declination:          %.2f°\n", results.Declination)
	fmt.Printf("  Declination SV:       %.2f'/yr\n", results.DeclinationSV)
	fmt.Printf("  Inclination:          %.2f°\n", results.Inclination)
	fmt.Printf("  Inclination SV:       %.2f'/yr\n", results.InclinationSV)
	fmt.Printf("  Horizontal Intensity: %.1f nT\n", results.HorizontalIntensity)
	fmt.Printf("  Horizontal SV:        %.1f nT/yr\n", results.HorizontalSV)
	fmt.Printf("  North Component:      %.1f nT\n", results.NorthComponent)
	fmt.Printf("  North SV:             %.1f nT/yr\n", results.NorthSV)
	fmt.Printf("  East Component:       %.1f nT\n", results.EastComponent)
	fmt.Printf("  East SV:              %.1f nT/yr\n", results.EastSV)
	fmt.Printf("  Vertical Component:   %.1f nT\n", results.VerticalComponent)
	fmt.Printf("  Vertical SV:          %.1f nT/yr\n", results.VerticalSV)
	fmt.Printf("  Total Intensity:      %.1f nT\n", results.TotalIntensity)
	fmt.Printf("  Total SV:             %.1f nT/yr\n", results.TotalSV)
}

func validateInputs(lat, lon, alt, date float64) error {
	if lat < -90.0 || lat > 90.0 {
		return fmt.Errorf("latitude %.4f is out of range (-90.0, 90.0)", lat)
	}
	if lon < -180.0 || lon > 180.0 {
		return fmt.Errorf("longitude %.4f is out of range (-180.0, 180.0)", lon)
	}
	if alt < -1.0 || alt > 600.0 {
		return fmt.Errorf("altitude %.1f km is out of range (-1.0, 600.0)", alt)
	}
	if date < 1900.0 || date > 2025.0 {
		return fmt.Errorf("date %.1f is out of range (1900.0, 2025.0)", date)
	}
	return nil
}
