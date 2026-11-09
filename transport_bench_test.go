package coesa_test

import (
	"testing"

	"github.com/augiraud/coesa"
)

func BenchmarkTransport(b *testing.B) {
	for _, row := range []struct {
		name  string
		units coesa.Units
	}{{"SI", coesa.SI()}, {"imperial", coesa.Imperial()}} {
		b.Run(row.name, func(b *testing.B) {
			state, err := coesa.At(10, coesa.Kilometre, row.units)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, transportErr := state.Transport()
				if transportErr != nil || result.SpeedOfSound <= 0 {
					b.Fatal("invalid transport result", transportErr)
				}
			}
		})
	}
}
