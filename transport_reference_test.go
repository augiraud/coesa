package coesa_test

import (
	"testing"

	"github.com/augiraud/coesa"
)

func TestPublishedTransport(t *testing.T) {
	rows := referenceRows(t, "transport.csv", []string{
		"altitude_m", "sound_speed_m_s", "dynamic_viscosity_Pa_s", "kinematic_viscosity_m2_s", "source_page",
	}, 18)
	observed := make(referenceDeviations)
	for _, row := range rows {
		transport, err := at(t, row[0]).Transport()
		if err != nil {
			t.Fatal(err)
		}
		observed.check(t, "sound speed", row[0], transport.SpeedOfSound, row[1], 0.006, 0)
		observed.check(t, "dynamic viscosity", row[0], transport.DynamicViscosity, row[2], 0, 1e-4)
		observed.check(t, "kinematic viscosity", row[0], transport.KinematicViscosity, row[3], 0, 1e-4)
	}
	observed.report(t)
}

func TestThermalConductivityEquation(t *testing.T) {
	// Decimal reference generation uses the defining Eq. (53). The report's
	// main conductivity tables contain errors described in testdata/README.md
	rows := referenceRows(t, "thermal_conductivity.csv",
		[]string{"temperature_K", "thermal_conductivity_W_m_K", "source_page"}, 10)
	for _, row := range rows {
		state := coesa.State{Temperature: row[0], MolarMass: 0.0289644, Density: 1, Units: coesa.SI()}
		transport, err := state.Transport()
		if err != nil {
			t.Fatal(err)
		}
		closeRelative(t, "thermal conductivity (Eq. 53)", transport.ThermalConductivity, row[1], 1e-14)
	}
}
