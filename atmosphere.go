package coesa

import (
	"errors"
	"fmt"
	"math"
)

// COESA 1976 constants, converted to SI units
const (
	minAltitude          = -5_000.0    // Minimum geometric altitude, m
	maxAltitude          = 1_000_000.0 // Maximum geometric altitude, m
	earthRadius          = 6_356_766.0 // Effective Earth radius, m
	seaLevelGravity      = 9.80665     // m/s²
	seaLevelTemperature  = 288.15      // K
	seaLevelPressure     = 101_325.0   // Pa
	seaLevelMolarMass    = 0.0289644   // kg/mol
	universalGasConstant = 8.31432     // J/(mol K)
	avogadroConstant     = 6.022169e23 // 1/mol

	upperBoundary = 86_000.0
)

// Errors can be identified with [errors.Is]
var (
	ErrAltitudeOutOfRange     = errors.New("coesa: altitude out of range")
	ErrTransportUnavailable   = errors.New("coesa: transport properties require altitude at or below 86 km")
	ErrCompositionUnavailable = errors.New("coesa: species composition requires altitude at or above 86 km")
	ErrInvalidState           = errors.New("coesa: invalid state")
)

// State describes the standard atmosphere at one altitude in the recorded Units
type State struct {
	Altitude             float64 // Geometric altitude
	GeopotentialAltitude float64 // Geopotential altitude
	Temperature          float64 // Kinetic temperature
	Pressure             float64
	Density              float64 // Mass density
	Gravity              float64 // Acceleration due to gravity
	MolarMass            float64 // Mean molar mass
	NumberDensity        float64 // Total neutral particle number density
	Units                Units
}

// TransportProperties contains the quantities defined for the lower atmosphere
type TransportProperties struct {
	SpeedOfSound        float64
	DynamicViscosity    float64
	KinematicViscosity  float64
	ThermalConductivity float64
	Units               Units
}

// At evaluates the atmosphere at a geometric altitude in inputUnit and returns
// quantities in outputUnits. The interval is -5 to 1,000 km; NaN and infinities
// return ErrAltitudeOutOfRange. Unsupported units return ErrInvalidUnit.
// On error, the returned State is zero.
func At(altitude float64, inputUnit LengthUnit, outputUnits Units) (State, error) {
	metres, err := altitudeMetres(altitude, inputUnit)
	if err != nil {
		return State{}, err
	}
	conversions, err := outputUnits.conversions()
	if err != nil {
		return State{}, err
	}
	return outputUnits.fromSI(atSI(metres), conversions), nil
}

// AltitudeRange returns the minimum and maximum geometric altitudes in unit
func AltitudeRange(unit LengthUnit) (minimum, maximum float64, err error) {
	conversion, err := unit.conversion()
	if err != nil {
		return 0, 0, err
	}
	return conversion.fromSI(minAltitude), conversion.fromSI(maxAltitude), nil
}

func altitudeMetres(altitude float64, unit LengthUnit) (float64, error) {
	conversion, err := unit.conversion()
	if err != nil {
		return 0, err
	}
	minimum, maximum := conversion.fromSI(minAltitude), conversion.fromSI(maxAltitude)
	if math.IsNaN(altitude) || altitude < minimum || altitude > maximum {
		return 0, fmt.Errorf("%w: geometric altitude %g %s; want [%g, %g] %s",
			ErrAltitudeOutOfRange, altitude, unit, minimum, maximum, unit)
	}
	return conversion.toSI(altitude), nil
}

func atSI(altitude float64) State {
	state := baseStateSI(altitude)
	if altitude < upperBoundary {
		state.Temperature, state.Pressure, state.MolarMass = lowerAtmosphere(altitude)
		state.NumberDensity = state.Pressure * avogadroConstant / (universalGasConstant * state.Temperature)
	} else {
		return upperStateSI(state, upperAtmosphere(altitude))
	}
	state.Density = state.Pressure * state.MolarMass / (universalGasConstant * state.Temperature)

	return state
}

func baseStateSI(altitude float64) State {
	return State{
		Altitude: altitude, GeopotentialAltitude: geopotential(altitude),
		Gravity: seaLevelGravity * square(earthRadius/(earthRadius+altitude)),
	}
}

func upperStateSI(state State, numbers NumberDensities) State {
	state.Temperature, _ = upperTemperature(state.Altitude)
	numberDensity := numbers.Total()
	state.NumberDensity = numberDensity
	state.MolarMass = numbers.molarMass(numberDensity)
	// Eq. (33c), using R*/Na so the equation of state remains consistent
	// with the report's independently rounded R*, k, and Na constants
	state.Pressure = state.NumberDensity * universalGasConstant * state.Temperature / avogadroConstant
	state.Density = state.Pressure * state.MolarMass / (universalGasConstant * state.Temperature)
	return state
}

// AtGeopotential evaluates a geopotential altitude in inputUnit, with results in
// outputUnits. Its bounds correspond to the geometric interval of -5 to 1,000 km
func AtGeopotential(altitude float64, inputUnit LengthUnit, outputUnits Units) (State, error) {
	geometric, err := GeometricAltitude(altitude, inputUnit, Metre)
	if err != nil {
		return State{}, err
	}

	return At(geometric, Metre, outputUnits)
}

// GeopotentialAltitude converts a geometric altitude in inputUnit to geopotential
// altitude in outputUnit using eq. (18), within the model's altitude range
func GeopotentialAltitude(altitude float64, inputUnit, outputUnit LengthUnit) (float64, error) {
	metres, err := altitudeMetres(altitude, inputUnit)
	if err != nil {
		return 0, err
	}
	output, err := outputUnit.conversion()
	if err != nil {
		return 0, err
	}
	return output.fromSI(geopotential(metres)), nil
}

// GeometricAltitude converts a geopotential altitude in inputUnit to geometric
// altitude in outputUnit using eq. (19), within the model's altitude range
func GeometricAltitude(altitude float64, inputUnit, outputUnit LengthUnit) (float64, error) {
	input, err := inputUnit.conversion()
	if err != nil {
		return 0, err
	}
	output, err := outputUnit.conversion()
	if err != nil {
		return 0, err
	}
	minimum, maximum := input.fromSI(geopotential(minAltitude)), input.fromSI(geopotential(maxAltitude))
	if math.IsNaN(altitude) || altitude < minimum || altitude > maximum {
		return 0, fmt.Errorf("%w: geopotential altitude %g %s", ErrAltitudeOutOfRange, altitude, inputUnit)
	}
	altitude = input.toSI(altitude)
	geometric := earthRadius * altitude / (earthRadius - altitude)
	// Roundoff at the endpoints must not invalidate an otherwise valid input.
	return output.fromSI(math.Max(minAltitude, math.Min(maxAltitude, geometric))), nil
}

// Transport returns sound speed, viscosity, and thermal conductivity for a State
// returned by At or AtGeopotential. These empirical relations are defined only
// at or below 86 km; higher altitudes return ErrTransportUnavailable. Results
// use s.Units, including its dedicated transport unit selections
//
// Manually constructed states need a finite altitude at or above the model's
// minimum, and finite positive temperature (in kelvins), density, and molar mass
// Invalid values or unrepresentable results return an error wrapping ErrInvalidState
// Other state quantities are not used or validated. Errors return a zero result
func (s State) Transport() (TransportProperties, error) {
	conversions, err := s.Units.conversions()
	if err != nil {
		return TransportProperties{}, err
	}
	if math.IsNaN(s.Altitude) || math.IsInf(s.Altitude, 0) ||
		s.Altitude < conversions.altitude.fromSI(minAltitude) {
		return TransportProperties{}, fmt.Errorf("%w: geometric altitude %g %s must be finite and at least %g %s",
			ErrInvalidState, s.Altitude, s.Units.Altitude,
			conversions.altitude.fromSI(minAltitude), s.Units.Altitude)
	}
	// Compare in the recorded unit so roundoff cannot reject a returned 86 km state
	if s.Altitude > conversions.altitude.fromSI(upperBoundary) {
		return TransportProperties{}, ErrTransportUnavailable
	}
	temperature := conversions.temperature.toSI(s.Temperature)
	if !positiveFinite(temperature) {
		return TransportProperties{}, fmt.Errorf("%w: temperature %g %s must be finite and above absolute zero",
			ErrInvalidState, s.Temperature, s.Units.Temperature)
	}
	molarMass := conversions.molarMass.toSI(s.MolarMass)
	if !positiveFinite(molarMass) {
		return TransportProperties{}, fmt.Errorf("%w: molar mass %g %s must convert to a finite positive value",
			ErrInvalidState, s.MolarMass, s.Units.MolarMass)
	}
	density := conversions.density.toSI(s.Density)
	if !positiveFinite(density) {
		return TransportProperties{}, fmt.Errorf("%w: density %g %s must convert to a finite positive value",
			ErrInvalidState, s.Density, s.Units.Density)
	}

	const (
		gamma      = 1.4
		beta       = 1.458e-6
		sutherland = 110.4 // Section 1.3.11 and the tables; table 2 rounds this to 110
	)
	viscosity := beta * math.Pow(temperature, 1.5) / (temperature + sutherland)

	result := TransportProperties{
		SpeedOfSound: conversions.speedOfSound.fromSI(
			math.Sqrt(gamma * universalGasConstant * temperature / molarMass),
		),
		DynamicViscosity:   conversions.dynamicViscosity.fromSI(viscosity),
		KinematicViscosity: conversions.kinematicViscosity.fromSI(viscosity / density),
		ThermalConductivity: conversions.thermalConductivity.fromSI(2.64638e-3 * math.Pow(temperature, 1.5) /
			(temperature + 245.4*math.Pow(10, -12/temperature))),
		Units: s.Units,
	}
	if !positiveFinite(result.SpeedOfSound) || !positiveFinite(result.DynamicViscosity) ||
		!positiveFinite(result.KinematicViscosity) || !positiveFinite(result.ThermalConductivity) {
		return TransportProperties{}, fmt.Errorf(
			"%w: transport properties cannot be represented as finite positive values in the requested units",
			ErrInvalidState)
	}
	return result, nil
}

func positiveFinite(value float64) bool { return value > 0 && value <= math.MaxFloat64 }

func geopotential(altitude float64) float64 {
	return earthRadius * altitude / (earthRadius + altitude)
}

func square(value float64) float64 { return value * value }
