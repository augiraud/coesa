package coesa_test

import (
	"strconv"
	"testing"

	"github.com/augiraud/coesa"
)

func BenchmarkEvaluator(b *testing.B) { benchmarkEvaluatorCases(b, false) }

func BenchmarkEvaluatorParallel(b *testing.B) { benchmarkEvaluatorCases(b, true) }

func benchmarkEvaluatorCases(b *testing.B, parallel bool) {
	b.Helper()
	for _, altitude := range []float64{10000, 500000} {
		for _, mode := range []string{"direct", "zero", "disabled", "cached"} {
			b.Run(strconv.FormatFloat(altitude, 'f', 0, 64)+"/"+mode, func(b *testing.B) {
				evaluate := benchmarkEvaluatorMode(b, mode)
				if _, err := evaluate(altitude); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				benchmarkRepeated(b, evaluate, altitude, parallel)
			})
		}
	}
}

func benchmarkEvaluatorMode(b *testing.B, mode string) benchmarkEvaluator {
	b.Helper()
	if mode == "direct" {
		return benchmarkAtMetres
	}
	var evaluator coesa.Evaluator
	instance := &evaluator
	if mode != "zero" {
		capacity := 0
		if mode == "cached" {
			capacity = 64
		}
		var err error
		instance, err = coesa.NewEvaluator(coesa.Options{CacheEntries: capacity})
		if err != nil {
			b.Fatal(err)
		}
	}
	return func(z float64) (coesa.State, error) { return instance.At(z, coesa.Metre, coesa.SI()) }
}

func BenchmarkEvaluatorScan(b *testing.B) {
	for _, base := range []float64{10000, 500000} {
		for _, mode := range []string{"direct", "cached"} {
			b.Run(strconv.FormatFloat(base, 'f', 0, 64)+"/"+mode, func(b *testing.B) {
				evaluate := benchmarkEvaluatorMode(b, mode)
				warmEvaluatorScan(b, evaluate, base)
				index := 0
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					state, err := evaluate(base + float64(index%1024))
					if err != nil || state.Pressure <= 0 {
						b.Fatal("invalid evaluation", err)
					}
					index++
				}
			})
		}
	}
}

func warmEvaluatorScan(b *testing.B, evaluate benchmarkEvaluator, base float64) {
	b.Helper()
	// Fill reusable nodes with keys outside the scan before timing misses.
	for i := range 64 {
		if _, err := evaluate(base - 1 - float64(i)); err != nil {
			b.Fatal(err)
		}
	}
}
