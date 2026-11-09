package coesa_test

import (
	"errors"
	"testing"

	"github.com/augiraud/coesa"
)

func TestPublishedSpecies(t *testing.T) {
	rows := referenceRows(t, "species.csv",
		[]string{"altitude_m", "N2_m-3", "O_m-3", "O2_m-3", "Ar_m-3", "He_m-3", "H_m-3", "source_page"}, 38)
	observed := make(referenceDeviations)
	for _, row := range rows {
		numbers, err := coesa.CompositionAt(row[0], coesa.Metre, coesa.PerCubicMetre)
		if err != nil {
			t.Fatal(err)
		}
		for i, species := range []struct {
			name   string
			number float64
		}{{"N2", numbers.N2}, {"O", numbers.O}, {"O2", numbers.O2},
			{"Ar", numbers.Ar}, {"He", numbers.He}, {"H", numbers.H}} {
			observed.check(t, species.name, row[0], species.number, row[i+1], 0, 5e-4)
		}
		closeRelative(t, "total number density", numbers.Total(), at(t, row[0]).NumberDensity, 1e-12)
	}
	observed.report(t)
}

func TestPublishedSpeciesBoundary(t *testing.T) {
	// Preserve the more precise 86 km boundary values from Part 1, Table 9
	numbers, err := coesa.CompositionAt(86000, coesa.Metre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	want := [6]float64{1.129794e20, 8.6e16, 3.030898e19, 1.351400e18, 7.5817e14, 0}
	for index, actual := range speciesValues(numbers) {
		closeRelative(t, "boundary species", actual, want[index], 1e-12)
	}
}

func TestHydrogenBoundary(t *testing.T) {
	numbers, err := coesa.CompositionAt(500000, coesa.Metre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	closeRelative(t, "hydrogen reference density", numbers.H, 8e10, 1e-12)
	numbers, err = coesa.CompositionAt(149999, coesa.Metre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	if numbers.H != 0 {
		t.Fatalf("hydrogen below 150 km = %g, want 0", numbers.H)
	}
	if _, err = coesa.CompositionAt(
		85999,
		coesa.Metre,
		coesa.PerCubicMetre,
	); !errors.Is(
		err,
		coesa.ErrCompositionUnavailable,
	) {
		t.Fatalf("composition error = %v, want ErrCompositionUnavailable", err)
	}
}
