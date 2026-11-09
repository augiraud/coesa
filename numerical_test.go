package coesa_test

import (
	"math"
	"testing"

	"github.com/augiraud/coesa"
)

func TestIntegrationRefinement(t *testing.T) {
	// A 1 ppm agreement target is much smaller than the printed species' 0.05%
	// comparison budget. This checks step-size sensitivity, not equation accuracy
	altitudes := []float64{86000.1, 91000, 95000, 97000, 100000, 109999.5, 110000, 115000,
		120000, 149999.5, 150000, 200123.456, 499999.5, 500000, 1000000}
	observed := make(referenceDeviations)
	for _, altitude := range altitudes {
		numbers, err := coesa.CompositionAt(altitude, coesa.Metre, coesa.PerCubicMetre)
		if err != nil {
			t.Fatal(err)
		}
		half := coesa.RefinedCompositionForTest(altitude, 2)
		quarter := coesa.RefinedCompositionForTest(altitude, 4)
		for _, pair := range []struct {
			name  string
			value coesa.NumberDensities
		}{{"full vs quarter step", numbers}, {"half vs quarter step", half}} {
			actual, expected := speciesValues(pair.value), speciesValues(quarter)
			for index := range actual {
				observed.check(t, pair.name, altitude, actual[index], expected[index], 0, 1e-6)
			}
		}
	}
	observed.report(t)
}

func speciesValues(numbers coesa.NumberDensities) [6]float64 {
	return [6]float64{numbers.N2, numbers.O, numbers.O2, numbers.Ar, numbers.He, numbers.H}
}

func TestPhysicalProfiles(t *testing.T) {
	t.Parallel()
	previous := at(t, -5000)
	for altitude := -4000.; altitude <= 1000000; altitude += 1000 {
		state := at(t, altitude)
		for _, value := range []float64{state.Temperature, state.Pressure, state.Density,
			state.MolarMass, state.Gravity, state.NumberDensity} {
			if !(value > 0) || math.IsInf(value, 0) {
				t.Fatalf("nonpositive or nonfinite property at %g m: %+v", altitude, state)
			}
		}
		if state.Pressure >= previous.Pressure || state.Density >= previous.Density ||
			state.Gravity >= previous.Gravity {
			t.Fatalf("profile must decrease with height at %g m", altitude)
		}
		if altitude >= 86000 {
			checkPhysicalComposition(t, state)
		}
		previous = state
	}
}

func checkPhysicalComposition(t *testing.T, state coesa.State) {
	t.Helper()
	numbers, err := coesa.CompositionAt(state.Altitude, coesa.Metre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range speciesValues(numbers) {
		if !(value >= 0) || math.IsInf(value, 0) {
			t.Fatalf("negative or nonfinite species at %g m", state.Altitude)
		}
	}
	// Sum of independent species evaluations must agree with the state
	closeRelative(t, "species sum", numbers.Total(), state.NumberDensity, 1e-12)
	// The rounded 1976 constants R* and Na define k here; the separately rounded
	// printed Boltzmann constant is intentionally not substituted (Eq. 33c)
	closeRelative(t, "particle equation of state", state.Pressure/state.NumberDensity/state.Temperature,
		8.31432/6.022169e23, 1e-12)
}
