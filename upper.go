package coesa

import (
	"math"
)

const (
	nitrogen = iota
	oxygen
	molecularOxygen
	argon
	helium
	hydrogen
)

const (
	upperBaseTemperature = 186.8673
	hydrogenBoundary     = 500_000.0
	hydrogenBaseDensity  = 8.0e10
	hydrogenStart        = 150_000.0
)

// NumberDensities describes the six species used in the upper atmosphere.
// Each field is a number density in Unit. Hydrogen is neglected below
// 150 km, as specified by the standard.
type NumberDensities struct {
	N2   float64 // Molecular nitrogen.
	O    float64 // Atomic oxygen.
	O2   float64 // Molecular oxygen.
	Ar   float64 // Argon.
	He   float64 // Helium.
	H    float64 // Atomic hydrogen.
	Unit NumberDensityUnit
}

// CompositionAt evaluates an altitude in inputUnit and returns species number
// densities in outputUnit from 86 to 1,000 km.
// Lower altitudes return ErrCompositionUnavailable; invalid model altitudes
// return ErrAltitudeOutOfRange.
func CompositionAt(altitude float64, inputUnit LengthUnit, outputUnit NumberDensityUnit) (NumberDensities, error) {
	altitude, err := altitudeMetres(altitude, inputUnit)
	if err != nil {
		return NumberDensities{}, err
	}
	output, err := outputUnit.conversion()
	if err != nil {
		return NumberDensities{}, err
	}
	if altitude < upperBoundary {
		return NumberDensities{}, ErrCompositionUnavailable
	}

	numbers := upperAtmosphere(altitude)
	return numbers.fromSI(outputUnit, output), nil
}

func (n NumberDensities) fromSI(unit NumberDensityUnit, output conversion) NumberDensities {
	return NumberDensities{
		N2: output.fromSI(n.N2), O: output.fromSI(n.O), O2: output.fromSI(n.O2),
		Ar: output.fromSI(n.Ar), He: output.fromSI(n.He), H: output.fromSI(n.H),
		Unit: unit,
	}
}

// Total returns the total number density in n.Unit
func (n NumberDensities) Total() float64 {
	return n.N2 + n.O + n.O2 + n.Ar + n.He + n.H
}

func (n NumberDensities) molarMass(totalNumberDensity float64) float64 {
	masses := speciesMasses()
	return (n.N2*masses[nitrogen] + n.O*masses[oxygen] + n.O2*masses[molecularOxygen] +
		n.Ar*masses[argon] + n.He*masses[helium] + n.H*masses[hydrogen]) / totalNumberDensity
}

func speciesMasses() [6]float64 {
	return [6]float64{0.0280134, 0.0159994, 0.0319988, 0.039948, 0.0040026, 0.00100797}
}

// upperTemperature gives T and dT/dZ in K and K/m (eqs. 25-32)
func upperTemperature(altitude float64) (temperature, gradient float64) {
	const (
		ellipseBase     = 91_000.0
		linearBase      = 110_000.0
		exponentialBase = 120_000.0
		ellipseCenter   = 263.1905
		ellipseHeight   = 76.3232
		ellipseWidth    = 19_942.9
		linearGradient  = 0.012
		exosphere       = 1000.0
		lambda          = 0.00001875
	)
	switch {
	case altitude <= ellipseBase:
		return upperBaseTemperature, 0
	case altitude < linearBase:
		x := (altitude - ellipseBase) / ellipseWidth
		root := math.Sqrt(1 - x*x)
		return ellipseCenter - ellipseHeight*root, ellipseHeight * x / (ellipseWidth * root)
	case altitude < exponentialBase:
		return 240 + linearGradient*(altitude-linearBase), linearGradient
	default:
		factor := (earthRadius + exponentialBase) / (earthRadius + altitude)
		difference := (exosphere - 360) * math.Exp(-lambda*(altitude-exponentialBase)*factor)
		return exosphere - difference, lambda * difference * factor * factor
	}
}

// The first five entries integrate log(n_i*T/(n_i(86 km)*T(86 km))).
// The last two integrate hydrogen's reduced height (tau) and escape flux.
// This formulation keeps the large dynamic range away from the ODE solver.
type upperIntegrals [7]float64

func upperAtmosphere(altitude float64) NumberDensities {
	return upperAtmosphereWithSteps(altitude, 100, 1000)
}

// The explicit step sizes allow numerical refinement without changing the
// defining equations or their breakpoints. Production always uses the defaults
func upperAtmosphereWithSteps(altitude, lowerStepSize, upperStepSize float64) NumberDensities {
	var integrals, atTarget, atHydrogenBoundary upperIntegrals
	end := altitude
	if altitude >= hydrogenStart {
		end = math.Max(end, hydrogenBoundary)
	}
	// Split at every change of formula, the requested altitude, and H's
	// reference altitude. Endpoint evaluations use the interior of each step.
	boundaries := [...]float64{91_000, 95_000, 97_000, 100_000, 110_000, 115_000, 120_000,
		hydrogenStart, hydrogenBoundary, maxAltitude}
	position := upperBoundary
	for position < end {
		step := lowerStepSize
		if position >= hydrogenStart {
			step = upperStepSize
		}
		next := math.Min(position+step, end)
		for _, boundary := range boundaries {
			if boundary > position {
				next = math.Min(next, boundary)
				break
			}
		}
		if position < altitude {
			next = math.Min(next, altitude)
		}
		integrals = upperStep(position, next, integrals)
		position = next
		if position == altitude {
			atTarget = integrals
		}
		if position == hydrogenBoundary {
			atHydrogenBoundary = integrals
		}
	}

	temperature, _ := upperTemperature(altitude)
	n := upperNumbers(temperature, atTarget)
	if altitude >= hydrogenStart {
		referenceTemperature, _ := upperTemperature(hydrogenBoundary)
		reference := hydrogenBaseDensity * math.Pow(referenceTemperature, 0.75) * math.Exp(atHydrogenBoundary[5])
		n[hydrogen] = (reference + atHydrogenBoundary[6] - atTarget[6]) /
			(math.Pow(temperature, 0.75) * math.Exp(atTarget[5]))
	}

	return NumberDensities{N2: n[nitrogen], O: n[oxygen], O2: n[molecularOxygen],
		Ar: n[argon], He: n[helium], H: n[hydrogen]}
}

func upperNumbers(temperature float64, integrals upperIntegrals) [6]float64 {
	// Table 9. Helium's printed value is 7.5817e14 (not 7.6817e14).
	numbers := [6]float64{1.129794e20, 8.6e16, 3.030898e19, 1.351400e18, 7.5817e14, 0}
	for species := range hydrogen {
		numbers[species] *= upperBaseTemperature / temperature * math.Exp(integrals[species])
	}

	return numbers
}

// upperDerivative implements eqs (35)-(40) in meters and kg/mol
func upperDerivative(altitude float64, integrals upperIntegrals) upperIntegrals {
	temperature, gradient := upperTemperature(altitude)
	gravity := seaLevelGravity * square(earthRadius/(earthRadius+altitude))
	n := upperNumbers(temperature, integrals)
	masses := speciesMasses()
	eddy := eddyDiffusion(altitude)
	backgroundMass := seaLevelMolarMass
	if altitude > 100_000 {
		backgroundMass = masses[nitrogen]
	}
	var derivative upperIntegrals
	derivative[nitrogen] = -backgroundMass * gravity / (universalGasConstant * temperature)
	coefficients := [...]float64{0, 6.986e20, 4.863e20, 4.487e20, 1.700e21}
	exponents := [...]float64{0, 0.750, 0.750, 0.870, 0.691}
	for species := oxygen; species <= helium; species++ {
		backgroundNumber := n[nitrogen]
		meanMass := backgroundMass
		if species >= argon {
			backgroundNumber += n[oxygen] + n[molecularOxygen]
			if altitude > 100_000 {
				meanMass = (n[nitrogen]*masses[nitrogen] + n[oxygen]*masses[oxygen] +
					n[molecularOxygen]*masses[molecularOxygen]) / backgroundNumber
			}
		}
		diffusion := coefficients[species] / backgroundNumber * math.Pow(temperature/273.15, exponents[species])
		fraction := diffusion / (diffusion + eddy)
		rate := gravity / (universalGasConstant * temperature) *
			(masses[species]*fraction + meanMass*(1-fraction))
		if species == helium {
			rate -= 0.4 * gradient / temperature * fraction
		}
		derivative[species] = -rate - verticalTransport(altitude, species)
	}
	derivative[5] = gravity * masses[hydrogen] / (universalGasConstant * temperature)
	if altitude >= hydrogenStart && altitude < hydrogenBoundary {
		total := n[nitrogen] + n[oxygen] + n[molecularOxygen] + n[argon] + n[helium]
		diffusion := 3.305e21 / total * math.Sqrt(temperature/273.15)
		derivative[6] = 7.2e11 / diffusion * math.Pow(temperature, 0.75) * math.Exp(integrals[5])
	}

	return derivative
}

// Eq. (7): the computational transition starts at 95 km, as specified in
// (7a-b), despite the earlier descriptive paragraph mentioning 91 km
func eddyDiffusion(altitude float64) float64 {
	const coefficient = 120.0
	switch {
	case altitude <= 95_000:
		return coefficient
	case altitude < 115_000:
		distance := (altitude - 95_000) / 1000
		return coefficient * math.Exp(1-400/(400-distance*distance))
	default:
		return 0
	}
}

// Eq. (37), table 7. Its coefficients use kilometers; convert the resulting
// inverse kilometers to inverse meters before integrating.
func verticalTransport(altitude float64, species int) float64 {
	q := [...]float64{0, -5.809644e-4, 1.366212e-4, 9.434079e-5, -2.457369e-4}
	u := [...]float64{0, 56.90311, 86, 86, 86}
	w := [...]float64{0, 2.706240e-5, 8.333333e-5, 8.333333e-5, 6.666667e-4}
	z := altitude / 1000
	distance := z - u[species]
	flux := q[species] * distance * distance * math.Exp(-w[species]*distance*distance*distance)
	if species == oxygen && z < 97 {
		distance = 97 - z
		flux -= 3.416248e-3 * distance * distance * math.Exp(-5.008765e-4*distance*distance*distance)
	}

	return flux / 1000
}

// Classical fourth-order Runge-Kutta with one-sided boundary evaluations
func upperStep(start, end float64, values upperIntegrals) upperIntegrals {
	step := end - start
	k1 := upperDerivative(math.Nextafter(start, end), values)
	k2 := upperDerivative(start+step/2, addIntegrals(values, k1, step/2))
	k3 := upperDerivative(start+step/2, addIntegrals(values, k2, step/2))
	k4 := upperDerivative(math.Nextafter(end, start), addIntegrals(values, k3, step))
	for index := range values {
		values[index] += step / 6 * (k1[index] + 2*k2[index] + 2*k3[index] + k4[index])
	}

	return values
}

func addIntegrals(values, derivative upperIntegrals, step float64) upperIntegrals {
	for index := range values {
		values[index] += step * derivative[index]
	}

	return values
}
