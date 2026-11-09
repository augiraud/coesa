package coesa_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/augiraud/coesa"
)

func TestAltitudeBounds(t *testing.T) {
	for _, unit := range []coesa.LengthUnit{coesa.Metre, coesa.Kilometre, coesa.Foot} {
		t.Run(unit.String(), func(t *testing.T) {
			minimum, maximum, err := coesa.AltitudeRange(unit)
			if err != nil {
				t.Fatal(err)
			}
			for _, altitude := range []float64{minimum, maximum} {
				checkAltitudeBound(t, altitude, unit)
			}
			for _, altitude := range []float64{math.Nextafter(minimum, math.Inf(-1)), math.Nextafter(maximum, math.Inf(1))} {
				if _, err = coesa.At(altitude, unit, coesa.SI()); !errors.Is(err, coesa.ErrAltitudeOutOfRange) {
					t.Fatalf("At(%g, %s) error = %v", altitude, unit, err)
				}
			}
		})
	}
}

func checkAltitudeBound(t *testing.T, altitude float64, unit coesa.LengthUnit) {
	t.Helper()
	output := coesa.Units{Altitude: unit}
	state, err := coesa.At(altitude, unit, output)
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "altitude at bound", state.Altitude, altitude, 1e-12)
	if state.Units != output {
		t.Fatal("output units not recorded")
	}
	geopotential, err := coesa.GeopotentialAltitude(altitude, unit, unit)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := coesa.AtGeopotential(geopotential, unit, output)
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "converted bound pressure", converted.Pressure, state.Pressure, 1e-10)
}

func TestAltitudeUnits(t *testing.T) {
	rows := []struct {
		altitude float64
		unit     coesa.LengthUnit
	}{
		{10000, coesa.Metre}, {10, coesa.Kilometre}, {32808.39895013123, coesa.Foot},
	}
	want := at(t, 10000)
	for _, row := range rows {
		state, err := coesa.At(row.altitude, row.unit, coesa.SI())
		if err != nil {
			t.Fatal(err)
		}
		closeRelative(t, "pressure with input unit", state.Pressure, want.Pressure, 1e-12)
		closeRelative(t, "temperature with input unit", state.Temperature, want.Temperature, 1e-12)
	}
	feet, err := coesa.GeometricAltitude(11, coesa.Kilometre, coesa.Foot)
	if err != nil {
		t.Fatal(err)
	}
	height, err := coesa.GeopotentialAltitude(feet, coesa.Foot, coesa.Kilometre)
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "mixed altitude unit round trip", height, 11, 1e-12)
	state, err := coesa.At(86, coesa.Kilometre, coesa.Units{Altitude: coesa.Kilometre})
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "output altitude in km", state.Altitude, 86, 1e-12)
	closeRelative(t, "output geopotential in km", state.GeopotentialAltitude, 84.8520, 1e-6)
}

func TestOutputUnits(t *testing.T) {
	// Fixed sea-level expectations and conversion factors from NIST SP 811,
	// Appendix B. Density and number density allow the report's rounding.
	temperature := func(s coesa.State) float64 { return s.Temperature }
	pressure := func(s coesa.State) float64 { return s.Pressure }
	density := func(s coesa.State) float64 { return s.Density }
	gravity := func(s coesa.State) float64 { return s.Gravity }
	molarMass := func(s coesa.State) float64 { return s.MolarMass }
	numberDensity := func(s coesa.State) float64 { return s.NumberDensity }
	rows := []struct {
		name  string
		units coesa.Units
		value func(coesa.State) float64
		want  float64
	}{
		{"kelvin", coesa.Units{Temperature: coesa.Kelvin}, temperature, 288.15},
		{"celsius", coesa.Units{Temperature: coesa.Celsius}, temperature, 15},
		{"fahrenheit", coesa.Units{Temperature: coesa.Fahrenheit}, temperature, 59},
		{"rankine", coesa.Units{Temperature: coesa.Rankine}, temperature, 518.67},
		{"pascal", coesa.Units{Pressure: coesa.Pascal}, pressure, 101325},
		{"hectopascal", coesa.Units{Pressure: coesa.Hectopascal}, pressure, 1013.25},
		{"kilopascal", coesa.Units{Pressure: coesa.Kilopascal}, pressure, 101.325},
		{"bar", coesa.Units{Pressure: coesa.Bar}, pressure, 1.01325},
		{"psi", coesa.Units{Pressure: coesa.PoundForcePerSquareInch}, pressure, 14.695948775513452},
		{"psf", coesa.Units{Pressure: coesa.PoundForcePerSquareFoot}, pressure, 2116.2166236739367},
		{"kg/m3", coesa.Units{Density: coesa.KilogramPerCubicMetre}, density, 1.2250},
		{"g/cm3", coesa.Units{Density: coesa.GramPerCubicCentimetre}, density, 0.0012250},
		{"lb/ft3", coesa.Units{Density: coesa.PoundPerCubicFoot}, density, 0.07647425170577717},
		{"slug/ft3", coesa.Units{Density: coesa.SlugPerCubicFoot}, density, 0.0023768924066751526},
		{"m/s2", coesa.Units{Gravity: coesa.MetrePerSecondSquared}, gravity, 9.80665},
		{"ft/s2", coesa.Units{Gravity: coesa.FootPerSecondSquared}, gravity, 32.17404855643044},
		{"kg/mol", coesa.Units{MolarMass: coesa.KilogramPerMole}, molarMass, 0.0289644},
		{"g/mol", coesa.Units{MolarMass: coesa.GramPerMole}, molarMass, 28.9644},
		{"kg/kmol", coesa.Units{MolarMass: coesa.KilogramPerKilomole}, molarMass, 28.9644},
		{"lb/lbmol", coesa.Units{MolarMass: coesa.PoundPerPoundMole}, molarMass, 28.9644},
		{"1/m3", coesa.Units{NumberDensity: coesa.PerCubicMetre}, numberDensity, 2.54697212e25},
		{"1/cm3", coesa.Units{NumberDensity: coesa.PerCubicCentimetre}, numberDensity, 2.54697212e19},
		{"1/ft3", coesa.Units{NumberDensity: coesa.PerCubicFoot}, numberDensity, 7.212221879614103e23},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			state, err := coesa.At(0, coesa.Metre, row.units)
			if err != nil {
				t.Fatal(err)
			}
			closeRelative(t, row.name, row.value(state), row.want, 1e-6)
			if state.Units != row.units {
				t.Fatal("output units not recorded")
			}
		})
	}
}

func TestImperialPreset(t *testing.T) {
	state, err := coesa.At(0, coesa.Foot, coesa.Imperial())
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "Fahrenheit", state.Temperature, 59, 1e-12)
	closeRelative(t, "psi", state.Pressure, 14.695948775513452, 1e-12)
	closeRelative(t, "slug/ft3", state.Density, 0.0023768924066751526, 1e-6)
	closeRelative(t, "ft/s2", state.Gravity, 32.17404855643044, 1e-12)
	closeRelative(t, "lb/lbmol", state.MolarMass, 28.9644, 1e-12)
	closeRelative(t, "1/ft3", state.NumberDensity, 7.212221879614103e23, 1e-8)
	if state.Units != coesa.Imperial() {
		t.Fatal("imperial units not recorded")
	}
}

func TestTransportUnits(t *testing.T) {
	baseline, err := at(t, 10000).Transport()
	if err != nil {
		t.Fatal(err)
	}
	for _, temperature := range []coesa.TemperatureUnit{coesa.Kelvin, coesa.Celsius, coesa.Fahrenheit, coesa.Rankine} {
		units := coesa.Imperial()
		units.Temperature = temperature
		state, evalErr := coesa.At(10, coesa.Kilometre, units)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		properties, evalErr := state.Transport()
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		closeRelative(t, "sound speed ft/s", properties.SpeedOfSound, baseline.SpeedOfSound/0.3048, 1e-12)
		closeRelative(
			t,
			"viscosity lbf s/ft2",
			properties.DynamicViscosity,
			baseline.DynamicViscosity/47.88025898033584,
			1e-12,
		)
		closeRelative(
			t,
			"viscosity ft2/s",
			properties.KinematicViscosity,
			baseline.KinematicViscosity/0.09290304,
			1e-12,
		)
		closeRelative(
			t,
			"conductivity Btu/(h ft F)",
			properties.ThermalConductivity,
			baseline.ThermalConductivity/1.730734666371391,
			1e-12,
		)
		if properties.Units != units {
			t.Fatal("transport units not recorded")
		}
	}
	units := coesa.Units{SpeedOfSound: coesa.Knot, DynamicViscosity: coesa.MicropascalSecond,
		KinematicViscosity: coesa.SquareMillimetrePerSecond, ThermalConductivity: coesa.MilliwattPerMetreKelvin}
	state, err := coesa.At(10, coesa.Kilometre, units)
	if err != nil {
		t.Fatal(err)
	}
	properties, err := state.Transport()
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "knots", properties.SpeedOfSound, baseline.SpeedOfSound/0.5144444444444444, 1e-12)
	closeRelative(t, "micropascal seconds", properties.DynamicViscosity, baseline.DynamicViscosity*1e6, 1e-12)
	closeRelative(t, "mm2/s", properties.KinematicViscosity, baseline.KinematicViscosity*1e6, 1e-12)
	closeRelative(t, "mW/(m K)", properties.ThermalConductivity, baseline.ThermalConductivity*1000, 1e-12)
	for _, altitude := range []float64{86, 86.001} {
		state, err = coesa.At(altitude, coesa.Kilometre, coesa.Imperial())
		if err != nil {
			t.Fatal(err)
		}
		_, err = state.Transport()
		if altitude == 86 && err != nil {
			t.Fatal(err)
		}
		if altitude > 86 && !errors.Is(err, coesa.ErrTransportUnavailable) {
			t.Fatal("transport limit ignored")
		}
	}
}

func TestCompositionUnits(t *testing.T) {
	baseline, err := coesa.CompositionAt(200, coesa.Kilometre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		unit  coesa.NumberDensityUnit
		scale float64
	}{
		{coesa.PerCubicCentimetre, 1e-6}, {coesa.PerCubicFoot, 0.028316846592},
	} {
		converted, evalErr := coesa.CompositionAt(200000, coesa.Metre, row.unit)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		want := []float64{baseline.N2, baseline.O, baseline.O2, baseline.Ar, baseline.He, baseline.H, baseline.Total()}
		got := []float64{
			converted.N2,
			converted.O,
			converted.O2,
			converted.Ar,
			converted.He,
			converted.H,
			converted.Total(),
		}
		for index := range want {
			closeRelative(t, "species unit conversion", got[index], want[index]*row.scale, 1e-12)
		}
		if converted.Unit != row.unit {
			t.Fatal("composition unit not recorded")
		}
	}
}

func TestInvalidOutputUnits(t *testing.T) {
	rows := []coesa.Units{
		{Altitude: 255}, {Temperature: 255}, {Pressure: 255}, {Density: 255}, {Gravity: 255},
		{MolarMass: 255}, {NumberDensity: 255}, {SpeedOfSound: 255}, {DynamicViscosity: 255},
		{KinematicViscosity: 255}, {ThermalConductivity: 255},
	}
	for _, units := range rows {
		state, err := coesa.At(0, coesa.Metre, units)
		if !errors.Is(err, coesa.ErrInvalidUnit) || state != (coesa.State{}) {
			t.Fatalf("invalid units: At returned %+v, %v", state, err)
		}
		if _, err = coesa.AtGeopotential(0, coesa.Metre, units); !errors.Is(err, coesa.ErrInvalidUnit) {
			t.Fatal(err)
		}
		state = at(t, 0)
		state.Units = units
		properties, err := state.Transport()
		if !errors.Is(err, coesa.ErrInvalidUnit) || properties != (coesa.TransportProperties{}) {
			t.Fatalf("invalid units: Transport returned %+v, %v", properties, err)
		}
	}
}

func TestInvalidInputUnits(t *testing.T) {
	if state, err := coesa.At(
		0,
		coesa.LengthUnit(255),
		coesa.SI(),
	); !errors.Is(err, coesa.ErrInvalidUnit) ||
		state != (coesa.State{}) {
		t.Fatalf("invalid input unit: %+v, %v", state, err)
	}
	if minimum, maximum, err := coesa.AltitudeRange(
		coesa.LengthUnit(255),
	); !errors.Is(err, coesa.ErrInvalidUnit) || minimum != 0 ||
		maximum != 0 {
		t.Fatalf("invalid range unit: %v", err)
	}
}

func TestInvalidAltitudeConversionUnits(t *testing.T) {
	for _, convert := range []func(float64, coesa.LengthUnit, coesa.LengthUnit) (float64, error){
		coesa.GeometricAltitude, coesa.GeopotentialAltitude,
	} {
		for _, units := range [][2]coesa.LengthUnit{{255, coesa.Metre}, {coesa.Metre, 255}} {
			if value, err := convert(0, units[0], units[1]); !errors.Is(err, coesa.ErrInvalidUnit) || value != 0 {
				t.Fatalf("invalid conversion units: %g, %v", value, err)
			}
		}
	}
}

func TestInvalidCompositionUnits(t *testing.T) {
	for _, call := range []struct {
		input  coesa.LengthUnit
		output coesa.NumberDensityUnit
	}{{255, coesa.PerCubicMetre}, {coesa.Metre, 255}} {
		if numbers, err := coesa.CompositionAt(
			100000,
			call.input,
			call.output,
		); !errors.Is(err, coesa.ErrInvalidUnit) ||
			numbers != (coesa.NumberDensities{}) {
			t.Fatalf("invalid composition units: %v", err)
		}
	}
}

func TestUnitSymbols(t *testing.T) {
	rows := []struct {
		unit fmt.Stringer
		want string
	}{
		{coesa.Metre, "m"}, {coesa.Kilometre, "km"}, {coesa.Foot, "ft"},
		{coesa.Celsius, "°C"}, {coesa.PoundForcePerSquareInch, "psi"},
		{coesa.SlugPerCubicFoot, "slug/ft³"}, {coesa.FootPerSecondSquared, "ft/s²"},
		{coesa.GramPerMole, "g/mol"}, {coesa.PerCubicCentimetre, "1/cm³"}, {coesa.Knot, "kn"},
		{coesa.MicropascalSecond, "µPa s"}, {coesa.SquareMillimetrePerSecond, "mm²/s"},
		{coesa.BTUPerHourFootFahrenheit, "Btu_IT/(h ft °F)"}, {coesa.LengthUnit(255), "?"},
	}
	for _, row := range rows {
		if got := row.unit.String(); got != row.want {
			t.Errorf("unit symbol = %q, want %q", got, row.want)
		}
	}
}
