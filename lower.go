package coesa

import "math"

// lowerAtmosphere implements eqs. (22), (23), and (33a-b). Computing layer
// bases recursively avoids discontinuities caused by rounded pressure tables.
func lowerAtmosphere(altitude float64) (temperature, pressure, molarMass float64) {
	heights := [...]float64{0, 11_000, 20_000, 32_000, 47_000, 51_000, 71_000}
	lapseRates := [...]float64{-0.0065, 0, 0.001, 0.0028, 0, -0.0028, -0.002}
	height := geopotential(altitude)
	temperature, pressure = seaLevelTemperature, seaLevelPressure

	for layer, base := range heights {
		top := height
		nextLayer := layer + 1
		if nextLayer < len(heights) {
			//nolint:gosec // G602: nextLayer is bounded by the preceding check
			top = math.Min(height, heights[nextLayer])
		}
		gradient := lapseRates[layer]
		nextTemperature := temperature + gradient*(top-base)
		if gradient == 0 {
			pressure *= math.Exp(-seaLevelGravity * seaLevelMolarMass * (top - base) /
				(universalGasConstant * temperature))
		} else {
			pressure *= math.Pow(temperature/nextTemperature,
				seaLevelGravity*seaLevelMolarMass/(universalGasConstant*gradient))
		}
		temperature = nextTemperature
		if top == height {
			break
		}
	}

	ratio := molecularWeightRatio(altitude)
	return temperature * ratio, pressure, seaLevelMolarMass * ratio
}

// Table 8 gives M/M0 at 500 m geometric intervals. Linear interpolation uses
// the more precise final ratio given in section 1.2.6, matching T(86 km).
func molecularWeightRatio(altitude float64) float64 {
	const (
		correctionBase = 80_000.0
		spacing        = 500.0
	)
	if altitude <= correctionBase {
		return 1
	}
	ratios := [...]float64{
		1, 0.999996, 0.999989, 0.999971, 0.999941, 0.999909, 0.999870,
		0.999829, 0.999786, 0.999741, 0.999694, 0.999641, 0.9995788,
	}
	position := (altitude - correctionBase) / spacing
	index := int(position)
	nextIndex := index + 1
	if nextIndex >= len(ratios) {
		return ratios[len(ratios)-1]
	}

	return ratios[index] + (position-float64(index))*(ratios[nextIndex]-ratios[index]) //nolint:gosec // G602: nextIndex is checked above; altitude > 80 km ensures index >= 0
}
