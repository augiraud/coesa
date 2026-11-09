package coesa_test

import (
	"fmt"

	"github.com/augiraud/coesa"
)

func ExampleEvaluator_At() {
	evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: 1024})
	if err != nil {
		fmt.Println(err)
		return
	}
	air, err := evaluator.At(500, coesa.Kilometre, coesa.SI())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.3e particles/m³\n", air.NumberDensity)
	// Output: 2.192e+13 particles/m³
}

func ExampleEvaluator_AtGeopotential() {
	// The zero value needs no constructor and evaluates without caching
	var evaluator coesa.Evaluator
	air, err := evaluator.AtGeopotential(11, coesa.Kilometre, coesa.SI())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%.2f K, %.0f Pa\n", air.Temperature, air.Pressure)
	// Output: 216.65 K, 22632 Pa
}

func ExampleEvaluator_CompositionAt() {
	evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: 64})
	if err != nil {
		fmt.Println(err)
		return
	}
	species, err := evaluator.CompositionAt(500, coesa.Kilometre, coesa.PerCubicCentimetre)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Hydrogen: %.1e particles/cm³\n", species.H)
	// Output: Hydrogen: 8.0e+04 particles/cm³
}

func ExampleEvaluator_CacheStats() {
	evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: 64})
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err = evaluator.At(500, coesa.Kilometre, coesa.Imperial()); err != nil {
		fmt.Println(err)
		return
	}
	// Composition reuses the physical solution despite different output units.
	if _, err = evaluator.CompositionAt(500000, coesa.Metre, coesa.PerCubicMetre); err != nil {
		fmt.Println(err)
		return
	}
	stats := evaluator.CacheStats()
	fmt.Printf("%d hit, %d miss, %d entry\n", stats.Hits, stats.Misses, stats.Entries)
	// Output: 1 hit, 1 miss, 1 entry
}
