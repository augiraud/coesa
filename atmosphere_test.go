package coesa_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/augiraud/coesa"
)

func TestPublishedGeometricValues(t *testing.T) {
	rows := referenceRows(t, "geometric.csv",
		[]string{"altitude_m", "temperature_K", "pressure_Pa", "density_kg_m3", "source_page"}, 157)
	ratios := publishedMassRatios(t)
	observed := make(referenceDeviations)
	for _, row := range rows {
		t.Run(strconv.FormatFloat(row[0], 'f', -1, 64), func(t *testing.T) {
			state := at(t, row[0])
			temperature, tolerance := row[1], 0.006
			if row[0] < 86000 {
				tolerance = 0.0006
				if row[0] >= 80000 {
					// Part 1, p. 9 explicitly corrects the unadjusted main tables
					ratio, ok := ratios[row[0]]
					if !ok {
						t.Fatal("reference needs a Table 8 ratio")
					}
					temperature *= ratio
				}
			}
			observed.check(t, "temperature", row[0], state.Temperature, temperature, tolerance, 0)
			observed.check(t, "pressure", row[0], state.Pressure, row[2], 0, 1e-4)
			observed.check(t, "density", row[0], state.Density, row[3], 0, 5e-4)
		})
	}
	observed.report(t)
}

func TestPublishedGeopotentialLayers(t *testing.T) {
	// Part 1, table 4 and Part 4, table I (geopotential rows).
	rows := []struct{ height, temperature, pressure, density float64 }{
		{0, 288.15, 101325, 1.2250},
		{11000, 216.65, 22632, 0.36392},
		{20000, 216.65, 5474.9, 0.088035},
		{32000, 228.65, 868.02, 0.013225},
		{47000, 270.65, 110.91, 0.0014275},
		{51000, 270.65, 66.939, 0.00086160},
		{71000, 214.65, 3.9564, 0.000064211},
	}
	for _, row := range rows {
		state, err := coesa.AtGeopotential(row.height, coesa.Metre, coesa.SI())
		if err != nil {
			t.Fatal(err)
		}
		closeRelative(t, "temperature", state.Temperature, row.temperature, 1e-12)
		closeRelative(t, "pressure", state.Pressure, row.pressure, 5e-5)
		closeRelative(t, "density", state.Density, row.density, 5e-5)
	}
}

func TestMolecularWeightCorrection(t *testing.T) {
	// Part 1, table 8 and the precise final ratio from section 1.2.6.
	ratios := publishedMassRatios(t)
	for altitude, ratio := range ratios {
		tolerance := 1e-12
		if altitude == 86000 {
			// The upper boundary derives its mean mass from rounded Table 9 species
			tolerance = 1e-6
		}
		state := at(t, altitude)
		closeRelative(t, "molecular weight ratio", state.MolarMass/0.0289644, ratio, tolerance)
	}
	closeRelative(t, "interpolated molecular weight ratio", at(t, 85750).MolarMass/0.0289644,
		(ratios[85500]+ratios[86000])/2, 1e-12)
	state := at(t, 86000)
	closeRelative(t, "86 km temperature", state.Temperature, 186.8673, 1e-12)
	closeRelative(t, "86 km molar mass", state.MolarMass, 0.0289522, 1e-6)
}

func TestSeaLevelTransport(t *testing.T) {
	properties, err := at(t, 0).Transport()
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "sound speed", properties.SpeedOfSound, 340.294, 1e-6)
	closeRelative(t, "dynamic viscosity", properties.DynamicViscosity, 1.7894e-5, 2e-5)
	closeRelative(t, "kinematic viscosity", properties.KinematicViscosity, 1.4607e-5, 3e-5)
	closeRelative(t, "thermal conductivity", properties.ThermalConductivity, 2.5326e-2, 1e-5)
	if _, err = at(t, 86000).Transport(); err != nil {
		t.Fatal(err)
	}
	if _, err = at(t, 86000.001).Transport(); !errors.Is(err, coesa.ErrTransportUnavailable) {
		t.Fatalf("transport error = %v, want ErrTransportUnavailable", err)
	}
}

func TestAltitudeConversions(t *testing.T) {
	for _, altitude := range []float64{-5_000.0, -1000, 0, 11000, 80000, 86000, 500000, 1_000_000.0} {
		height, err := coesa.GeopotentialAltitude(altitude, coesa.Metre, coesa.Metre)
		if err != nil {
			t.Fatal(err)
		}
		geometric, err := coesa.GeometricAltitude(height, coesa.Metre, coesa.Metre)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(geometric-altitude) > 1e-8 {
			t.Errorf("round trip %g -> %g -> %g", altitude, height, geometric)
		}
		converted, err := coesa.AtGeopotential(height, coesa.Metre, coesa.SI())
		if err != nil {
			t.Fatal(err)
		}
		closeRelative(t, "converted pressure", converted.Pressure, at(t, altitude).Pressure, 1e-10)
	}
	// Published conversion: 86 km geometric is 84.8520 km geopotential.
	height, err := coesa.GeopotentialAltitude(86000, coesa.Metre, coesa.Metre)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(height-84852.0) > 0.05 {
		t.Errorf("86 km geopotential = %g m", height)
	}
}

func TestInvalidAltitudes(t *testing.T) {
	for _, altitude := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -5000.001, 1000000.001} {
		state, err := coesa.At(altitude, coesa.Metre, coesa.SI())
		if !errors.Is(err, coesa.ErrAltitudeOutOfRange) || state != (coesa.State{}) {
			t.Errorf("At(%g) = %+v, %v; want zero State and ErrAltitudeOutOfRange", altitude, state, err)
		}
		if _, err = coesa.GeopotentialAltitude(
			altitude,
			coesa.Metre,
			coesa.Metre,
		); !errors.Is(
			err,
			coesa.ErrAltitudeOutOfRange,
		) {
			t.Errorf("GeopotentialAltitude(%g) error = %v", altitude, err)
		}
		if _, err = coesa.CompositionAt(
			altitude,
			coesa.Metre,
			coesa.PerCubicMetre,
		); !errors.Is(
			err,
			coesa.ErrAltitudeOutOfRange,
		) {
			t.Errorf("CompositionAt(%g) error = %v", altitude, err)
		}
	}
	for _, altitude := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -5100, 900000, 6_356_766.0} {
		if _, err := coesa.GeometricAltitude(
			altitude,
			coesa.Metre,
			coesa.Metre,
		); !errors.Is(
			err,
			coesa.ErrAltitudeOutOfRange,
		) {
			t.Errorf("GeometricAltitude(%g) error = %v", altitude, err)
		}
		if _, err := coesa.AtGeopotential(
			altitude,
			coesa.Metre,
			coesa.SI(),
		); !errors.Is(
			err,
			coesa.ErrAltitudeOutOfRange,
		) {
			t.Errorf("AtGeopotential(%g) error = %v", altitude, err)
		}
	}
}

func TestBoundaryContinuity(t *testing.T) {
	boundaries := []float64{80000, 86000, 91000, 95000, 97000, 100000, 110000, 115000, 120000, 150000, 500000}
	for _, height := range []float64{11000, 20000, 32000, 47000, 51000, 71000} {
		altitude, err := coesa.GeometricAltitude(height, coesa.Metre, coesa.Metre)
		if err != nil {
			t.Fatal(err)
		}
		boundaries = append(boundaries, altitude)
	}
	for _, altitude := range boundaries {
		below, above := at(t, altitude-0.001), at(t, altitude+0.001)
		closeRelative(t, "boundary temperature", below.Temperature, above.Temperature, 1e-5)
		closeRelative(t, "boundary pressure", below.Pressure, above.Pressure, 2e-5)
		closeRelative(t, "boundary density", below.Density, above.Density, 2e-5)
	}
}

func FuzzAt(f *testing.F) {
	for _, altitude := range []float64{
		-5000, 0, 11000, 85000, 86000, 100000, 150000, 500000, 1000000,
		-5000.001, 1_000_000.001, math.Inf(-1), math.Inf(1), math.NaN(),
	} {
		f.Add(altitude)
	}
	f.Fuzz(func(t *testing.T, altitude float64) {
		state, err := coesa.At(altitude, coesa.Metre, coesa.SI())
		if math.IsNaN(altitude) || altitude < -5_000.0 || altitude > 1_000_000.0 {
			if !errors.Is(err, coesa.ErrAltitudeOutOfRange) {
				t.Fatalf("invalid altitude %g: %v", altitude, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []float64{state.Temperature, state.Pressure, state.Density, state.Gravity,
			state.MolarMass, state.NumberDensity} {
			if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
				t.Fatalf("nonphysical result at %g m: %+v", altitude, state)
			}
		}
	})
}

func BenchmarkAt(b *testing.B) {
	for _, altitude := range []float64{10000, 100000, 500000} {
		b.Run(strconv.FormatFloat(altitude, 'f', 0, 64), func(b *testing.B) {
			for b.Loop() {
				if _, err := coesa.At(altitude, coesa.Metre, coesa.SI()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func at(t *testing.T, altitude float64) coesa.State {
	t.Helper()
	state, err := coesa.At(altitude, coesa.Metre, coesa.SI())
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func closeRelative(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance*math.Abs(want) {
		t.Errorf("%s = %.10g, want %.10g (relative tolerance %g)", name, got, want, tolerance)
	}
}
