package coesa_test

import (
	"fmt"

	"github.com/augiraud/coesa"
)

func ExampleAt() {
	state, err := coesa.At(10, coesa.Kilometre, coesa.SI())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("T = %.2f K, p = %.0f Pa, rho = %.5f kg/m³\n", state.Temperature, state.Pressure, state.Density)
	// Output: T = 223.25 K, p = 26500 Pa, rho = 0.41351 kg/m³
}

func ExampleAt_outputUnits() {
	units := coesa.SI()
	units.Temperature = coesa.Celsius
	units.Pressure = coesa.Hectopascal
	air, err := coesa.At(10, coesa.Kilometre, units)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.2f %s, %.2f %s\n", air.Temperature, air.Units.Temperature, air.Pressure, air.Units.Pressure)
	// Output: -49.90 °C, 265.00 hPa
}

func ExampleImperial() {
	air, err := coesa.At(0, coesa.Foot, coesa.Imperial())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.2f %s, %.2f %s\n", air.Temperature, air.Units.Temperature, air.Pressure, air.Units.Pressure)
	// Output: 59.00 °F, 14.70 psi
}

func ExampleGeopotentialAltitude() {
	height, err := coesa.GeopotentialAltitude(86, coesa.Kilometre, coesa.Kilometre)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.3f km\n", height)
	// Output: 84.852 km
}

func ExampleAtGeopotential() {
	state, err := coesa.AtGeopotential(11, coesa.Kilometre, coesa.SI())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("T = %.2f K, p = %.0f Pa\n", state.Temperature, state.Pressure)
	// Output: T = 216.65 K, p = 22632 Pa
}

func ExampleState_Transport() {
	state, err := coesa.At(0, coesa.Metre, coesa.SI())
	if err != nil {
		fmt.Println(err)
		return
	}
	properties, err := state.Transport()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Sound speed = %.2f m/s\n", properties.SpeedOfSound)
	// Output: Sound speed = 340.29 m/s
}

func ExampleCompositionAt() {
	numbers, err := coesa.CompositionAt(500, coesa.Kilometre, coesa.PerCubicMetre)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Hydrogen = %.1e particles/m³\n", numbers.H)
	// Output: Hydrogen = 8.0e+10 particles/m³
}
