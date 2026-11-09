package coesa_test

import "testing"

func TestPublishedSecondaryProperties(t *testing.T) {
	rows := referenceRows(t, "secondary.csv", []string{
		"altitude_m",
		"geopotential_altitude_m",
		"gravity_m_s2",
		"number_density_m-3",
		"molar_mass_kg_mol",
		"source_page",
	}, 9)
	observed := make(referenceDeviations)
	for _, row := range rows {
		state := at(t, row[0])
		observed.check(t, "geopotential altitude", row[0], state.GeopotentialAltitude, row[1], 0.51, 0)
		observed.check(t, "gravity", row[0], state.Gravity, row[2], 0, 1e-5)
		observed.check(t, "number density", row[0], state.NumberDensity, row[3], 0, 5e-4)
		// Table II prints only three significant digits for molar mass at 1,000 km
		observed.check(t, "molar mass", row[0], state.MolarMass, row[4], 0, 2e-3)
	}
	observed.report(t)
}
