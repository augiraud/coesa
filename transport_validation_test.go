package coesa_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/augiraud/coesa"
)

func TestTransportInvalidState(t *testing.T) {
	checkTransportError(t, coesa.State{}, coesa.ErrInvalidState)
	baseline := at(t, 0)
	for _, field := range []struct {
		name string
		set  func(*coesa.State, float64)
	}{
		{"temperature", func(s *coesa.State, v float64) { s.Temperature = v }},
		{"density", func(s *coesa.State, v float64) { s.Density = v }},
		{"molar mass", func(s *coesa.State, v float64) { s.MolarMass = v }},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, value := range []float64{0, math.Copysign(0, -1), -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
				state := baseline
				field.set(&state, value)
				err := checkTransportError(t, state, coesa.ErrInvalidState)
				if !strings.Contains(err.Error(), field.name) {
					t.Errorf("error does not identify %s: %v", field.name, err)
				}
			}
		})
	}
}

func checkTransportError(t *testing.T, state coesa.State, expected error) error {
	t.Helper()
	result, err := state.Transport()
	if !errors.Is(err, expected) {
		t.Fatalf("Transport() error = %v, want %v; state %+v", err, expected, state)
	}
	if result != (coesa.TransportProperties{}) {
		t.Fatalf("Transport() returned partial properties on error: %+v", result)
	}
	return err
}

func TestTransportAbsoluteZero(t *testing.T) {
	for _, row := range []struct {
		unit coesa.TemperatureUnit
		zero float64
	}{
		{coesa.Kelvin, 0}, {coesa.Celsius, -273.15},
		{coesa.Fahrenheit, -459.67}, {coesa.Rankine, 0},
	} {
		for _, temperature := range []float64{row.zero, row.zero - 1} {
			state := at(t, 0)
			state.Units.Temperature, state.Temperature = row.unit, temperature
			checkTransportError(t, state, coesa.ErrInvalidState)
		}
	}
}

func TestTransportTemperatureOffsets(t *testing.T) {
	for _, row := range []struct {
		unit                coesa.TemperatureUnit
		temperature, kelvin float64
	}{
		{coesa.Celsius, 0, 273.15}, {coesa.Celsius, -10, 263.15},
		{coesa.Fahrenheit, 0, 255.3722222222222}, {coesa.Fahrenheit, -40, 233.15},
	} {
		state := at(t, 0)
		state.Temperature = row.kelvin
		expected, err := state.Transport()
		if err != nil {
			t.Fatal(err)
		}
		state.Units.Temperature, state.Temperature = row.unit, row.temperature
		actual, err := state.Transport()
		if err != nil {
			t.Fatal(err)
		}
		closeRelative(t, "offset temperature sound speed", actual.SpeedOfSound, expected.SpeedOfSound, 1e-12)
		closeRelative(t, "offset temperature viscosity", actual.DynamicViscosity, expected.DynamicViscosity, 1e-12)
		closeRelative(
			t,
			"offset temperature kinematic viscosity",
			actual.KinematicViscosity,
			expected.KinematicViscosity,
			1e-12,
		)
		closeRelative(
			t,
			"offset temperature conductivity",
			actual.ThermalConductivity,
			expected.ThermalConductivity,
			1e-12,
		)
	}
}

func TestTransportAltitudeValidation(t *testing.T) {
	for _, unit := range []coesa.LengthUnit{coesa.Metre, coesa.Kilometre, coesa.Foot} {
		checkTransportAltitudeBounds(t, unit)
	}
	state := at(t, 100000)
	state.Temperature = math.NaN()
	checkTransportError(t, state, coesa.ErrTransportUnavailable)
	state.Altitude = math.MaxFloat64
	checkTransportError(t, state, coesa.ErrTransportUnavailable)
	state.Units.Density = coesa.DensityUnit(255)
	checkTransportError(t, state, coesa.ErrInvalidUnit)
}

func checkTransportAltitudeBounds(t *testing.T, unit coesa.LengthUnit) {
	t.Helper()
	units := coesa.SI()
	units.Altitude = unit
	for _, altitude := range []float64{-5000, 86000} {
		state, err := coesa.At(altitude, coesa.Metre, units)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = state.Transport(); err != nil {
			t.Fatalf("valid boundary in %s rejected: %v", unit, err)
		}
		if altitude < 0 {
			state.Altitude = math.Nextafter(state.Altitude, math.Inf(-1))
			checkTransportError(t, state, coesa.ErrInvalidState)
		} else {
			state.Altitude = math.Nextafter(state.Altitude, math.Inf(1))
			checkTransportError(t, state, coesa.ErrTransportUnavailable)
		}
	}
	for _, altitude := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		state := at(t, 0)
		state.Units.Altitude, state.Altitude = unit, altitude
		checkTransportError(t, state, coesa.ErrInvalidState)
	}
}

func TestTransportUnrepresentableValues(t *testing.T) {
	for _, row := range []struct {
		name string
		set  func(*coesa.State)
	}{
		{"temperature overflow", func(s *coesa.State) { s.Temperature = math.MaxFloat64 }},
		{"temperature underflow", func(s *coesa.State) { s.Temperature = math.SmallestNonzeroFloat64 }},
		{"sound speed overflow", func(s *coesa.State) { s.MolarMass = math.SmallestNonzeroFloat64 }},
		{"kinematic overflow", func(s *coesa.State) { s.Density = math.SmallestNonzeroFloat64 }},
		{"density conversion overflow", func(s *coesa.State) {
			s.Units.Density, s.Density = coesa.GramPerCubicCentimetre, math.MaxFloat64
		}},
		{"mass conversion underflow", func(s *coesa.State) {
			s.Units.MolarMass, s.MolarMass = coesa.GramPerMole, math.SmallestNonzeroFloat64
		}},
		{"output conversion overflow", func(s *coesa.State) {
			s.Units.KinematicViscosity, s.Density = coesa.SquareMillimetrePerSecond, 1e-310
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			state := at(t, 0)
			row.set(&state)
			checkTransportError(t, state, coesa.ErrInvalidState)
		})
	}
}

func TestTransportPartialState(t *testing.T) {
	state := at(t, 0)
	expected, err := state.Transport()
	if err != nil {
		t.Fatal(err)
	}
	partial := coesa.State{Altitude: state.Altitude, Temperature: state.Temperature,
		Density: state.Density, MolarMass: state.MolarMass, Units: state.Units}
	// Values that the calculation does not consume do not invalidate its inputs
	partial.Pressure, partial.Gravity = math.NaN(), math.Inf(1)
	partial.GeopotentialAltitude, partial.NumberDensity = math.Inf(-1), -1
	actual, err := partial.Transport()
	if err != nil || actual != expected {
		t.Fatalf("partial state changed supported transport evaluation: %+v, %v", actual, err)
	}
}

func FuzzTransport(f *testing.F) {
	f.Add(0., 288.15, 1.225, 0.0289644, false)
	f.Add(0., 59., 0.00237689, 28.9644, true)
	f.Add(86000., 186.8673, 6.958e-6, 0.0289522, false)
	f.Add(math.NaN(), 288.15, 1.225, 0.0289644, false)
	f.Add(0., math.MaxFloat64, 1., 1., false)
	f.Add(0., 288.15, math.SmallestNonzeroFloat64, 0.0289644, false)
	f.Fuzz(func(t *testing.T, altitude, temperature, density, molarMass float64, imperial bool) {
		units := coesa.SI()
		if imperial {
			units = coesa.Imperial()
		}
		state := coesa.State{Altitude: altitude, Temperature: temperature,
			Density: density, MolarMass: molarMass, Units: units}
		result, err := state.Transport()
		if err != nil {
			if result != (coesa.TransportProperties{}) {
				t.Fatalf("partial transport result on error: %+v", result)
			}
			if !errors.Is(err, coesa.ErrInvalidState) && !errors.Is(err, coesa.ErrTransportUnavailable) {
				t.Fatalf("unexpected error class: %v", err)
			}
			return
		}
		if result.Units != units {
			t.Fatal("result units changed")
		}
		checkSuccessfulTransportInputs(t, state, imperial)
		for _, value := range []float64{result.SpeedOfSound, result.DynamicViscosity,
			result.KinematicViscosity, result.ThermalConductivity} {
			if !(value > 0) || math.IsInf(value, 0) {
				t.Fatalf("nonfinite or nonpositive successful result: %+v; state %+v", result, state)
			}
		}
	})
}

func checkSuccessfulTransportInputs(t *testing.T, state coesa.State, imperial bool) {
	t.Helper()
	minimum, maximum, absoluteZero := -5000., 86000., 0.
	if imperial {
		minimum, maximum, absoluteZero = minimum/0.3048, maximum/0.3048, -459.67
	}
	if math.IsNaN(state.Altitude) || state.Altitude < minimum || state.Altitude > maximum {
		t.Fatalf("invalid altitude succeeded: %+v", state)
	}
	if !(state.Temperature > absoluteZero) || !(state.Density > 0) || !(state.MolarMass > 0) {
		t.Fatalf("invalid physical signs succeeded: %+v", state)
	}
	for _, value := range []float64{state.Temperature, state.Density, state.MolarMass} {
		if math.IsInf(value, 0) {
			t.Fatalf("nonfinite input succeeded: %+v", state)
		}
	}
}
