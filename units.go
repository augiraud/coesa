package coesa

import (
	"errors"
	"fmt"
)

// ErrInvalidUnit identifies unsupported unit values, including invalid enum casts
var ErrInvalidUnit = errors.New("coesa: invalid unit")

// LengthUnit selects the unit of an altitude
type LengthUnit uint8

// Supported altitude units
const (
	Metre LengthUnit = iota
	Kilometre
	Foot
)

// TemperatureUnit selects the unit of kinetic temperature
type TemperatureUnit uint8

// Supported temperature units
const (
	Kelvin TemperatureUnit = iota
	Celsius
	Fahrenheit
	Rankine
)

// PressureUnit selects the unit of pressure
type PressureUnit uint8

// Supported pressure units
const (
	Pascal PressureUnit = iota
	Hectopascal
	Kilopascal
	Bar
	PoundForcePerSquareInch
	PoundForcePerSquareFoot
)

// DensityUnit selects the unit of mass density
type DensityUnit uint8

// Supported mass density units
const (
	KilogramPerCubicMetre DensityUnit = iota
	GramPerCubicCentimetre
	PoundPerCubicFoot
	SlugPerCubicFoot
)

// AccelerationUnit selects the unit of gravity
type AccelerationUnit uint8

// Supported acceleration units
const (
	MetrePerSecondSquared AccelerationUnit = iota
	FootPerSecondSquared
)

// MolarMassUnit selects the unit of mean molar mass
type MolarMassUnit uint8

// Supported molar mass units. PoundPerPoundMole uses the conventional lbmol.
const (
	KilogramPerMole MolarMassUnit = iota
	GramPerMole
	KilogramPerKilomole
	PoundPerPoundMole
)

// NumberDensityUnit selects the unit of a particle number density
type NumberDensityUnit uint8

// Supported particle number density units
const (
	PerCubicMetre NumberDensityUnit = iota
	PerCubicCentimetre
	PerCubicFoot
)

// SpeedUnit selects the unit of sound speed
type SpeedUnit uint8

// Supported speed units
const (
	MetrePerSecond SpeedUnit = iota
	FootPerSecond
	Knot
)

// DynamicViscosityUnit selects the unit of dynamic viscosity
type DynamicViscosityUnit uint8

// Supported dynamic viscosity units
const (
	PascalSecond DynamicViscosityUnit = iota
	MicropascalSecond
	PoundForceSecondPerSquareFoot
)

// KinematicViscosityUnit selects the unit of kinematic viscosity
type KinematicViscosityUnit uint8

// Supported kinematic viscosity units
const (
	SquareMetrePerSecond KinematicViscosityUnit = iota
	SquareMillimetrePerSecond
	SquareFootPerSecond
)

// ThermalConductivityUnit selects the unit of thermal conductivity
type ThermalConductivityUnit uint8

// Supported thermal conductivity units. BTU means the International Table Btu.
const (
	WattPerMetreKelvin ThermalConductivityUnit = iota
	MilliwattPerMetreKelvin
	BTUPerHourFootFahrenheit
)

// Units selects each output quantity independently. Its zero value is SI.
// Both altitude fields use Altitude; transport quantities use their named fields.
type Units struct {
	Altitude            LengthUnit
	Temperature         TemperatureUnit
	Pressure            PressureUnit
	Density             DensityUnit
	Gravity             AccelerationUnit
	MolarMass           MolarMassUnit
	NumberDensity       NumberDensityUnit
	SpeedOfSound        SpeedUnit
	DynamicViscosity    DynamicViscosityUnit
	KinematicViscosity  KinematicViscosityUnit
	ThermalConductivity ThermalConductivityUnit
}

// SI returns metres, kelvin, pascals, and SI units for the remaining quantities
func SI() Units { return Units{} }

// Imperial returns feet, Fahrenheit, psi, slug/ft³, ft/s², lb/lbmol, particles/ft³,
// ft/s, lbf s/ft², ft²/s, and Btu_IT/(h ft °F)
func Imperial() Units {
	return Units{
		Altitude: Foot, Temperature: Fahrenheit, Pressure: PoundForcePerSquareInch,
		Density: SlugPerCubicFoot, Gravity: FootPerSecondSquared, MolarMass: PoundPerPoundMole,
		NumberDensity: PerCubicFoot, SpeedOfSound: FootPerSecond,
		DynamicViscosity: PoundForceSecondPerSquareFoot, KinematicViscosity: SquareFootPerSecond,
		ThermalConductivity: BTUPerHourFootFahrenheit,
	}
}

// Exact definitions from NIST SP 811, Appendix B:
// https://www.nist.gov/pml/special-publication-811/nist-guide-si-appendix-b-conversion-factors
const (
	metresPerFoot        = 0.3048
	kilogramsPerPound    = 0.45359237
	newtonsPerPoundForce = kilogramsPerPound * 9.80665
	metresPerInch        = metresPerFoot / 12
	joulesPerBTU         = 1055.05585262
)

// conversion maps a selected unit to SI as si = value*scale + offset
type conversion struct{ scale, offset float64 }

func (c conversion) toSI(value float64) float64   { return value*c.scale + c.offset }
func (c conversion) fromSI(value float64) float64 { return (value - c.offset) / c.scale }

func unitScale(name string, unit uint8, scales ...float64) (conversion, error) {
	for index, scale := range scales {
		if index == int(unit) {
			return conversion{scale: scale}, nil
		}
	}
	return conversion{}, fmt.Errorf("%w: %s unit %d", ErrInvalidUnit, name, unit)
}

func (u LengthUnit) conversion() (conversion, error) {
	return unitScale("length", uint8(u), 1, 1000, metresPerFoot)
}

func (u TemperatureUnit) conversion() (conversion, error) {
	switch u {
	case Kelvin:
		return conversion{scale: 1}, nil
	case Celsius:
		return conversion{scale: 1, offset: 273.15}, nil
	case Fahrenheit:
		return conversion{scale: 5.0 / 9, offset: 273.15 - 32*5.0/9}, nil
	case Rankine:
		return conversion{scale: 5.0 / 9}, nil
	default:
		return conversion{}, fmt.Errorf("%w: temperature unit %d", ErrInvalidUnit, u)
	}
}

func (u PressureUnit) conversion() (conversion, error) {
	return unitScale("pressure", uint8(u), 1, 100, 1000, 100_000,
		newtonsPerPoundForce/(metresPerInch*metresPerInch), newtonsPerPoundForce/(metresPerFoot*metresPerFoot))
}

func (u DensityUnit) conversion() (conversion, error) {
	cubicFoot := metresPerFoot * metresPerFoot * metresPerFoot
	return unitScale("density", uint8(u), 1, 1000, kilogramsPerPound/cubicFoot,
		newtonsPerPoundForce/metresPerFoot/cubicFoot)
}

func (u AccelerationUnit) conversion() (conversion, error) {
	return unitScale("acceleration", uint8(u), 1, metresPerFoot)
}

func (u MolarMassUnit) conversion() (conversion, error) {
	return unitScale("molar mass", uint8(u), 1, 0.001, 0.001, 0.001)
}

func (u NumberDensityUnit) conversion() (conversion, error) {
	return unitScale("number density", uint8(u), 1, 1e6, 1/(metresPerFoot*metresPerFoot*metresPerFoot))
}

func (u SpeedUnit) conversion() (conversion, error) {
	return unitScale("speed", uint8(u), 1, metresPerFoot, 1852.0/3600)
}

func (u DynamicViscosityUnit) conversion() (conversion, error) {
	return unitScale("dynamic viscosity", uint8(u), 1, 1e-6, newtonsPerPoundForce/(metresPerFoot*metresPerFoot))
}

func (u KinematicViscosityUnit) conversion() (conversion, error) {
	return unitScale("kinematic viscosity", uint8(u), 1, 1e-6, metresPerFoot*metresPerFoot)
}

func (u ThermalConductivityUnit) conversion() (conversion, error) {
	return unitScale("thermal conductivity", uint8(u), 1, 0.001, joulesPerBTU/3600/metresPerFoot*1.8)
}

type unitConversions struct {
	altitude, temperature, pressure, density, gravity, molarMass, numberDensity conversion
	speedOfSound, dynamicViscosity, kinematicViscosity, thermalConductivity     conversion
}

func (u Units) conversions() (unitConversions, error) {
	var result unitConversions
	var err error
	result.altitude, err = u.Altitude.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.temperature, err = u.Temperature.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.pressure, err = u.Pressure.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.density, err = u.Density.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.gravity, err = u.Gravity.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.molarMass, err = u.MolarMass.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.numberDensity, err = u.NumberDensity.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.speedOfSound, err = u.SpeedOfSound.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.dynamicViscosity, err = u.DynamicViscosity.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.kinematicViscosity, err = u.KinematicViscosity.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	result.thermalConductivity, err = u.ThermalConductivity.conversion()
	if err != nil {
		return unitConversions{}, err
	}
	return result, nil
}

func (u Units) fromSI(state State, c unitConversions) State {
	return State{
		Altitude:             c.altitude.fromSI(state.Altitude),
		GeopotentialAltitude: c.altitude.fromSI(state.GeopotentialAltitude),
		Temperature:          c.temperature.fromSI(state.Temperature),
		Pressure:             c.pressure.fromSI(state.Pressure), Density: c.density.fromSI(state.Density),
		Gravity: c.gravity.fromSI(state.Gravity), MolarMass: c.molarMass.fromSI(state.MolarMass),
		NumberDensity: c.numberDensity.fromSI(state.NumberDensity), Units: u,
	}
}

func unitSymbol(unit uint8, symbols ...string) string {
	for index, symbol := range symbols {
		if index == int(unit) {
			return symbol
		}
	}
	return "?"
}

func (u LengthUnit) String() string      { return unitSymbol(uint8(u), "m", "km", "ft") }
func (u TemperatureUnit) String() string { return unitSymbol(uint8(u), "K", "°C", "°F", "°R") }
func (u PressureUnit) String() string {
	return unitSymbol(uint8(u), "Pa", "hPa", "kPa", "bar", "psi", "psf")
}
func (u DensityUnit) String() string {
	return unitSymbol(uint8(u), "kg/m³", "g/cm³", "lb/ft³", "slug/ft³")
}
func (u AccelerationUnit) String() string { return unitSymbol(uint8(u), "m/s²", "ft/s²") }
func (u MolarMassUnit) String() string {
	return unitSymbol(uint8(u), "kg/mol", "g/mol", "kg/kmol", "lb/lbmol")
}
func (u NumberDensityUnit) String() string { return unitSymbol(uint8(u), "1/m³", "1/cm³", "1/ft³") }
func (u SpeedUnit) String() string         { return unitSymbol(uint8(u), "m/s", "ft/s", "kn") }
func (u DynamicViscosityUnit) String() string {
	return unitSymbol(uint8(u), "Pa s", "µPa s", "lbf s/ft²")
}
func (u KinematicViscosityUnit) String() string {
	return unitSymbol(uint8(u), "m²/s", "mm²/s", "ft²/s")
}
func (u ThermalConductivityUnit) String() string {
	return unitSymbol(uint8(u), "W/(m K)", "mW/(m K)", "Btu_IT/(h ft °F)")
}
