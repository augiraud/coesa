package coesa_test

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/augiraud/coesa"
)

func newEvaluator(t *testing.T, capacity int) *coesa.Evaluator {
	t.Helper()
	evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: capacity})
	if err != nil {
		t.Fatal(err)
	}
	return evaluator
}

func TestEvaluatorOptions(t *testing.T) {
	if evaluator, err := coesa.NewEvaluator(
		coesa.Options{CacheEntries: -1},
	); evaluator != nil ||
		!errors.Is(err, coesa.ErrInvalidOptions) {
		t.Fatalf("negative capacity: %v, %v", evaluator, err)
	}
	for _, capacity := range []int{0, 1, 1_000_000} {
		evaluator := newEvaluator(t, capacity)
		if stats := evaluator.CacheStats(); stats != (coesa.CacheStats{Capacity: capacity}) {
			t.Fatalf("initial stats: %+v", stats)
		}
	}
	options := coesa.Options{CacheEntries: 2}
	evaluator, err := coesa.NewEvaluator(options)
	if err != nil {
		t.Fatal(err)
	}
	options.CacheEntries = 0
	if evaluator.CacheStats().Capacity != 2 || options.CacheEntries != 0 {
		t.Fatal("options were not copied")
	}
}

func TestEvaluatorResults(t *testing.T) {
	evaluators := []*coesa.Evaluator{{}, newEvaluator(t, 0), newEvaluator(t, 3)}
	altitudes := []float64{-5000, 0, 10000, 80000, 85999.999, 86000, 86000.001, 100000, 150000, 500000, 1000000}
	for _, evaluator := range evaluators {
		for _, units := range []coesa.Units{coesa.SI(), coesa.Imperial(), {Temperature: coesa.Celsius, Pressure: coesa.Hectopascal}} {
			for _, altitude := range altitudes {
				compareEvaluatorAt(t, evaluator, altitude, units)
			}
		}
	}
}

func compareEvaluatorAt(t *testing.T, evaluator *coesa.Evaluator, altitude float64, units coesa.Units) {
	t.Helper()
	want := at(t, altitude)
	expected, err := coesa.At(altitude, coesa.Metre, units)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := evaluator.At(altitude, coesa.Metre, units)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("At(%g): got %+v, want %+v", altitude, actual, expected)
	}
	height, err := coesa.GeopotentialAltitude(altitude, coesa.Metre, coesa.Metre)
	if err != nil {
		t.Fatal(err)
	}
	expected, err = coesa.AtGeopotential(height, coesa.Metre, units)
	if err != nil {
		t.Fatal(err)
	}
	actual, err = evaluator.AtGeopotential(height, coesa.Metre, units)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("AtGeopotential(%g): result changed", height)
	}
	if want.Altitude >= 86000 {
		for _, numberUnit := range []coesa.NumberDensityUnit{coesa.PerCubicMetre, coesa.PerCubicCentimetre, coesa.PerCubicFoot} {
			compareEvaluatorComposition(t, evaluator, altitude, numberUnit)
		}
	}
}

func compareEvaluatorComposition(
	t *testing.T,
	evaluator *coesa.Evaluator,
	altitude float64,
	unit coesa.NumberDensityUnit,
) {
	t.Helper()
	want, err := coesa.CompositionAt(altitude, coesa.Metre, unit)
	if err != nil {
		t.Fatal(err)
	}
	got, err := evaluator.CompositionAt(altitude, coesa.Metre, unit)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CompositionAt(%g, %s): result changed", altitude, unit)
	}
}

func TestEvaluatorCacheSharing(t *testing.T) {
	evaluator := newEvaluator(t, 2)
	if _, err := evaluator.At(500, coesa.Kilometre, coesa.SI()); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluator.At(500000, coesa.Metre, coesa.Imperial()); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluator.CompositionAt(500, coesa.Kilometre, coesa.PerCubicCentimetre); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluator.CompositionAt(500000, coesa.Metre, coesa.PerCubicFoot); err != nil {
		t.Fatal(err)
	}
	geometric, err := coesa.GeometricAltitude(100000, coesa.Metre, coesa.Metre)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = evaluator.At(geometric, coesa.Metre, coesa.SI()); err != nil {
		t.Fatal(err)
	}
	for _, units := range []coesa.Units{coesa.SI(), coesa.Imperial()} {
		if _, err = evaluator.AtGeopotential(100000, coesa.Metre, units); err != nil {
			t.Fatal(err)
		}
	}
	want := coesa.CacheStats{Hits: 5, Misses: 2, Entries: 2, Capacity: 2}
	if stats := evaluator.CacheStats(); stats != want {
		t.Fatalf("sharing stats: %+v, want %+v", stats, want)
	}
	// The stateless functions do not observe or populate evaluator caches.
	if _, err = coesa.At(500, coesa.Kilometre, coesa.SI()); err != nil {
		t.Fatal(err)
	}
	if evaluator.CacheStats() != want {
		t.Fatal("package call changed evaluator state")
	}
	other := newEvaluator(t, 2)
	if other.CacheStats().Entries != 0 {
		t.Fatal("evaluators share global storage")
	}
	// Composition can populate a solution before its State is requested
	if _, err = other.CompositionAt(200, coesa.Kilometre, coesa.PerCubicMetre); err != nil {
		t.Fatal(err)
	}
	state, err := other.At(200, coesa.Kilometre, coesa.Imperial())
	if err != nil {
		t.Fatal(err)
	}
	if state != atImperial(t, 200000) {
		t.Fatal("composition-first State changed")
	}
	if other.CacheStats().Misses != 1 {
		t.Fatal("composition and state solved independently")
	}
}

func TestEvaluatorLRU(t *testing.T) {
	evaluator := newEvaluator(t, 2)
	for _, altitude := range []float64{100000, 150000, 100000, 200000, 100000, 150000} {
		state, err := evaluator.At(altitude, coesa.Metre, coesa.SI())
		if err != nil {
			t.Fatal(err)
		}
		if state != at(t, altitude) {
			t.Fatal("eviction changed the physical result")
		}
		if evaluator.CacheStats().Entries > 2 {
			t.Fatal("capacity exceeded")
		}
	}
	want := coesa.CacheStats{Hits: 2, Misses: 4, Evictions: 2, Entries: 2, Capacity: 2}
	if stats := evaluator.CacheStats(); stats != want {
		t.Fatalf("LRU stats: %+v, want %+v", stats, want)
	}
	// Exercise recycling the only node, including both ends of the LRU list
	one := newEvaluator(t, 1)
	for _, altitude := range []float64{100000, 200000, 100000, 100000} {
		if _, err := one.At(altitude, coesa.Metre, coesa.SI()); err != nil {
			t.Fatal(err)
		}
	}
	if stats := one.CacheStats(); stats.Entries != 1 || stats.Evictions != 2 || stats.Hits != 1 {
		t.Fatalf("single-entry stats: %+v", stats)
	}
}

func TestEvaluatorLRUPromotion(t *testing.T) {
	evaluator := newEvaluator(t, 3)
	// Promote the middle of three retained results, then force two evictions
	for _, altitude := range []float64{100000, 110000, 120000, 110000, 130000, 120000, 110000, 100000} {
		state, err := evaluator.At(altitude, coesa.Metre, coesa.SI())
		if err != nil || state != at(t, altitude) {
			t.Fatalf("promotion changed the result at %g: %v", altitude, err)
		}
	}
	want := coesa.CacheStats{Hits: 3, Misses: 5, Evictions: 2, Entries: 3, Capacity: 3}
	if stats := evaluator.CacheStats(); stats != want {
		t.Fatalf("promotion stats: %+v, want %+v", stats, want)
	}
}

func TestEvaluatorOwnedResults(t *testing.T) {
	evaluator := newEvaluator(t, 1)
	state, err := evaluator.At(500, coesa.Kilometre, coesa.SI())
	if err != nil {
		t.Fatal(err)
	}
	state.Pressure = -1
	state.Units.Pressure = coesa.Bar
	got, err := evaluator.At(500, coesa.Kilometre, coesa.SI())
	if err != nil {
		t.Fatal(err)
	}
	if got != at(t, 500000) || got == state {
		t.Fatal("caller mutated a cached State")
	}
	numbers, err := evaluator.CompositionAt(500, coesa.Kilometre, coesa.PerCubicMetre)
	if err != nil {
		t.Fatal(err)
	}
	numbers.H = -1
	numbers.Unit = coesa.PerCubicFoot
	compareEvaluatorComposition(t, evaluator, 500000, coesa.PerCubicMetre)
	if numbers.H != -1 || numbers.Unit != coesa.PerCubicFoot {
		t.Fatal("caller copy changed")
	}
	stats := evaluator.CacheStats()
	stats.Capacity = -1
	if evaluator.CacheStats().Capacity != 1 || stats.Capacity != -1 {
		t.Fatal("statistics are not copied")
	}
}

func TestEvaluatorLowerBypass(t *testing.T) {
	evaluator := newEvaluator(t, 2)
	for _, altitude := range []float64{-5000, 0, 10000, 80000, 86000} {
		state, err := evaluator.At(altitude, coesa.Metre, coesa.Imperial())
		if err != nil {
			t.Fatal(err)
		}
		if state != atImperial(t, altitude) {
			t.Fatal("uncached lower State changed")
		}
	}
	if _, err := evaluator.AtGeopotential(11, coesa.Kilometre, coesa.SI()); err != nil {
		t.Fatal(err)
	}
	compareEvaluatorComposition(t, evaluator, 86000, coesa.PerCubicMetre)
	if got := evaluator.CacheStats(); got != (coesa.CacheStats{Capacity: 2}) {
		t.Fatalf("cheap calls touched cache: %+v", got)
	}
}

func TestEvaluatorInvalidRequests(t *testing.T) {
	for _, capacity := range []int{0, 2} {
		evaluator := newEvaluator(t, capacity)
		if _, err := evaluator.At(500, coesa.Kilometre, coesa.SI()); err != nil {
			t.Fatal(err)
		}
		before := evaluator.CacheStats()
		checkEvaluatorInvalidAltitudes(t, evaluator)
		checkEvaluatorInvalidUnits(t, evaluator)
		if numbers, err := evaluator.CompositionAt(
			85,
			coesa.Kilometre,
			coesa.PerCubicMetre,
		); numbers != (coesa.NumberDensities{}) ||
			!errors.Is(err, coesa.ErrCompositionUnavailable) {
			t.Fatalf("lower composition: %v", err)
		}
		if after := evaluator.CacheStats(); after != before {
			t.Fatalf("invalid inputs changed cache: %+v -> %+v", before, after)
		}
	}
}

func checkEvaluatorInvalidAltitudes(t *testing.T, evaluator *coesa.Evaluator) {
	t.Helper()
	for _, altitude := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -6000, 1000001} {
		state, err := evaluator.At(altitude, coesa.Metre, coesa.SI())
		if state != (coesa.State{}) || !errors.Is(err, coesa.ErrAltitudeOutOfRange) {
			t.Fatalf("invalid At(%g): %+v, %v", altitude, state, err)
		}
		state, err = evaluator.AtGeopotential(altitude, coesa.Metre, coesa.SI())
		if state != (coesa.State{}) || !errors.Is(err, coesa.ErrAltitudeOutOfRange) {
			t.Fatalf("invalid AtGeopotential(%g): %v", altitude, err)
		}
		numbers, err := evaluator.CompositionAt(altitude, coesa.Metre, coesa.PerCubicMetre)
		if numbers != (coesa.NumberDensities{}) || !errors.Is(err, coesa.ErrAltitudeOutOfRange) {
			t.Fatalf("invalid CompositionAt(%g): %v", altitude, err)
		}
	}
}

func checkEvaluatorInvalidUnits(t *testing.T, evaluator *coesa.Evaluator) {
	t.Helper()
	for _, units := range []coesa.Units{{Altitude: 255}, {Temperature: 255}, {Pressure: 255}, {Density: 255}, {Gravity: 255},
		{MolarMass: 255}, {NumberDensity: 255}, {SpeedOfSound: 255}, {DynamicViscosity: 255}, {KinematicViscosity: 255}, {ThermalConductivity: 255}} {
		state, err := evaluator.At(500, coesa.Kilometre, units)
		if state != (coesa.State{}) || !errors.Is(err, coesa.ErrInvalidUnit) {
			t.Fatalf("invalid output units: %v", err)
		}
		state, err = evaluator.AtGeopotential(100000, coesa.Metre, units)
		if state != (coesa.State{}) || !errors.Is(err, coesa.ErrInvalidUnit) {
			t.Fatalf("invalid geopotential output units: %v", err)
		}
	}
	if state, err := evaluator.At(
		0,
		coesa.LengthUnit(255),
		coesa.SI(),
	); state != (coesa.State{}) ||
		!errors.Is(err, coesa.ErrInvalidUnit) {
		t.Fatal("invalid input unit accepted", err)
	}
	if state, err := evaluator.AtGeopotential(
		0,
		coesa.LengthUnit(255),
		coesa.SI(),
	); state != (coesa.State{}) ||
		!errors.Is(err, coesa.ErrInvalidUnit) {
		t.Fatal("invalid geopotential input unit accepted", err)
	}
	for _, units := range [][2]uint8{{255, 0}, {0, 255}} {
		if numbers, err := evaluator.CompositionAt(
			500000,
			coesa.LengthUnit(units[0]),
			coesa.NumberDensityUnit(units[1]),
		); numbers != (coesa.NumberDensities{}) ||
			!errors.Is(err, coesa.ErrInvalidUnit) {
			t.Fatal("invalid composition unit accepted", err)
		}
	}
}

func TestEvaluatorConcurrentCoalescing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const workers = 8
		release := make(chan struct{})
		var calls atomic.Int64
		evaluator, err := coesa.NewObservedEvaluatorForTest(
			coesa.Options{CacheEntries: 1},
			func(float64) { calls.Add(1); <-release },
		)
		if err != nil {
			t.Fatal(err)
		}
		failures := make(chan error, workers)
		var group sync.WaitGroup
		for index := range workers {
			group.Go(func() {
				failures <- evaluatorConcurrentQuery(t, evaluator, index)
			})
		}
		synctest.Wait()
		stats := evaluator.CacheStats()
		if stats.Misses != workers || stats.Coalesced != workers-1 || stats.InFlight != 1 || calls.Load() != 1 {
			t.Errorf("pending coalescing: %+v, calculations=%d", stats, calls.Load())
		}
		close(release)
		group.Wait()
		close(failures)
		for failure := range failures {
			if failure != nil {
				t.Error(failure)
			}
		}
		if stats = evaluator.CacheStats(); stats.Entries != 1 || stats.InFlight != 0 {
			t.Errorf("completed coalescing: %+v", stats)
		}
	})
}

func evaluatorConcurrentQuery(t *testing.T, evaluator *coesa.Evaluator, index int) error {
	t.Helper()
	if index%2 == 0 {
		state, err := evaluator.At(500, coesa.Kilometre, coesa.Imperial())
		if err == nil && state != atImperial(t, 500000) {
			t.Error("coalesced State changed")
		}
		return err
	}
	numbers, err := evaluator.CompositionAt(500000, coesa.Metre, coesa.PerCubicMetre)
	if err == nil && numbers.H != 8e10 {
		t.Error("coalesced composition changed")
	}
	return err
}

func atImperial(t *testing.T, altitude float64) coesa.State {
	t.Helper()
	state, err := coesa.At(altitude, coesa.Metre, coesa.Imperial())
	if err != nil {
		t.Error(err)
	}
	return state
}

func TestEvaluatorIndependentMissesOverlap(t *testing.T) {
	started := make(chan float64, 2)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	evaluator, err := coesa.NewObservedEvaluatorForTest(
		coesa.Options{CacheEntries: 2},
		func(z float64) { started <- z; <-release },
	)
	if err != nil {
		t.Fatal(err)
	}
	failures := make(chan error, 2)
	for _, altitude := range []float64{100000, 200000} {
		go func() { _, evalErr := evaluator.At(altitude, coesa.Metre, coesa.SI()); failures <- evalErr }()
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case <-started:
		case <-timer.C:
			t.Fatal("different heights calculated under cache lock")
		}
	}
	if stats := evaluator.CacheStats(); stats.InFlight != 2 {
		t.Fatalf("pending stats: %+v", stats)
	}
	unblock()
	for range 2 {
		if failure := <-failures; failure != nil {
			t.Fatal(failure)
		}
	}
	if stats := evaluator.CacheStats(); stats.InFlight != 0 || stats.Entries != 2 {
		t.Fatalf("completed stats: %+v", stats)
	}
}

func TestEvaluatorExactKeys(t *testing.T) {
	evaluator := newEvaluator(t, 3)
	// Foot and metre inputs normalize to the same exactly representable height
	feet, err := evaluator.At(500000, coesa.Foot, coesa.SI())
	if err != nil {
		t.Fatal(err)
	}
	metres, err := evaluator.At(152400, coesa.Metre, coesa.SI())
	if err != nil || feet != metres {
		t.Fatalf("input-unit normalization: %v, %v", feet == metres, err)
	}
	// Adjacent floating-point heights must remain distinct; caching is exact.
	adjacent := math.Nextafter(152400, math.Inf(1))
	state, err := evaluator.At(adjacent, coesa.Metre, coesa.SI())
	if err != nil || state != at(t, adjacent) || state.Altitude == metres.Altitude {
		t.Fatalf("adjacent height was rounded: %+v, %v", state, err)
	}
	want := coesa.CacheStats{Hits: 1, Misses: 2, Entries: 2, Capacity: 3}
	if stats := evaluator.CacheStats(); stats != want {
		t.Fatalf("exact-key stats: %+v, want %+v", stats, want)
	}
	copyOfEvaluator := *evaluator
	if _, err = copyOfEvaluator.CompositionAt(152.4, coesa.Kilometre, coesa.PerCubicFoot); err != nil {
		t.Fatal(err)
	}
	if evaluator.CacheStats().Hits != 2 || copyOfEvaluator.CacheStats() != evaluator.CacheStats() {
		t.Fatal("copied evaluator did not share its cache")
	}
}

func TestEvaluatorConcurrentEviction(t *testing.T) {
	evaluator := newEvaluator(t, 2)
	altitudes := []float64{100000, 110000, 120000, 130000, 140000}
	states := make([]coesa.State, len(altitudes))
	numbers := make([]coesa.NumberDensities, len(altitudes))
	for i, altitude := range altitudes {
		states[i] = atImperial(t, altitude)
		var err error
		numbers[i], err = coesa.CompositionAt(altitude, coesa.Metre, coesa.PerCubicCentimetre)
		if err != nil {
			t.Fatal(err)
		}
	}
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for iteration := range 20 {
				i := (worker + iteration) % len(altitudes)
				checkEvaluatorConcurrentQuery(t, evaluator, altitudes[i], states[i], numbers[i])
			}
		})
	}
	workers.Wait()
	stats := evaluator.CacheStats()
	if stats.Entries != 2 || stats.InFlight != 0 || stats.Evictions == 0 || stats.Hits+stats.Misses != 320 {
		t.Fatalf("completed concurrent snapshot: %+v", stats)
	}
}

func checkEvaluatorConcurrentQuery(
	t *testing.T, evaluator *coesa.Evaluator, altitude float64, expected coesa.State, numbers coesa.NumberDensities,
) {
	t.Helper()
	state, err := evaluator.At(altitude/1000, coesa.Kilometre, coesa.Imperial())
	if err != nil || state != expected {
		t.Errorf("concurrent state changed at %g: %v", altitude, err)
	}
	composition, err := evaluator.CompositionAt(altitude, coesa.Metre, coesa.PerCubicCentimetre)
	if err != nil || composition != numbers {
		t.Errorf("concurrent composition changed at %g: %v", altitude, err)
	}
	if stats := evaluator.CacheStats(); stats.Entries > 2 || stats.Capacity != 2 || stats.InFlight < 0 {
		t.Errorf("invalid concurrent snapshot: %+v", stats)
	}
}

func TestEvaluatorAllocations(t *testing.T) {
	for name, evaluator := range map[string]*coesa.Evaluator{
		"zero": {}, "disabled": newEvaluator(t, 0), "cached": newEvaluator(t, 2),
	} {
		// Warm the exact coordinate used by AtGeopotential, then both query types.
		if _, err := evaluator.AtGeopotential(100, coesa.Kilometre, coesa.SI()); err != nil {
			t.Fatal(err)
		}
		if _, err := evaluator.At(500, coesa.Kilometre, coesa.Imperial()); err != nil {
			t.Fatal(err)
		}
		checkEvaluatorAllocations(t, name, evaluator)
		if name != "cached" && evaluator.CacheStats() != (coesa.CacheStats{}) {
			t.Fatal("direct evaluation retained cache state")
		}
	}
}

func checkEvaluatorAllocations(t *testing.T, name string, evaluator *coesa.Evaluator) {
	t.Helper()
	queries := map[string]func() error{
		"lower": func() error {
			_, err := evaluator.At(10, coesa.Kilometre, coesa.Imperial())
			return err
		},
		"upper": func() error {
			_, err := evaluator.At(500, coesa.Kilometre, coesa.Imperial())
			return err
		},
		"geopotential": func() error {
			_, err := evaluator.AtGeopotential(100, coesa.Kilometre, coesa.SI())
			return err
		},
		"composition": func() error {
			_, err := evaluator.CompositionAt(500000, coesa.Metre, coesa.PerCubicFoot)
			return err
		},
		"stats": func() error { _ = evaluator.CacheStats(); return nil },
	}
	for query, evaluate := range queries {
		t.Run(name+"/"+query, func(t *testing.T) {
			allocations := testing.AllocsPerRun(5, func() {
				if err := evaluate(); err != nil {
					t.Fatal(err)
				}
			})
			if allocations != 0 {
				t.Fatalf("%g allocations per successful call", allocations)
			}
		})
	}
}
