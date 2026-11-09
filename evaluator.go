package coesa

import (
	"errors"
	"fmt"
)

// ErrInvalidOptions identifies invalid evaluator configuration
var ErrInvalidOptions = errors.New("coesa: invalid evaluator options")

// Options configures an optional Evaluator. Its zero value disables caching.
type Options struct {
	// CacheEntries bounds retained upper-atmosphere solutions, evicting the least
	// recently used result when full. Zero disables caching; negatives are invalid.
	// Entries are allocated as needed.
	// Simultaneous distinct calculations are additional to this result limit.
	CacheEntries int
}

// Evaluator provides the same evaluations as the package functions with optional
// per-instance caching. The zero value evaluates directly without caching.
// Configuration is fixed at construction. Methods are safe for concurrent use;
// results are returned by value. Copies of an Evaluator share its cache.
type Evaluator struct {
	cache *solutionCache
}

// CacheStats is a consistent snapshot of an evaluator's cache. Lower-atmosphere
// calls and invalid requests bypass the cache and do not affect its counters.
type CacheStats struct {
	Hits      uint64 // Requests served from retained results
	Misses    uint64 // Requests without a retained result, including coalesced requests
	Coalesced uint64 // Misses that joined an existing calculation for the same height
	Evictions uint64 // Results displaced to respect capacity
	Entries   int    // Currently retained results
	Capacity  int    // Configured result limit; zero when disabled
	InFlight  int    // Distinct heights currently being calculated
}

// NewEvaluator constructs an optional evaluator. A negative CacheEntries returns
// an error wrapping ErrInvalidOptions.
// The package functions remain available independently of any evaluator.
func NewEvaluator(options Options) (*Evaluator, error) {
	if options.CacheEntries < 0 {
		return nil, fmt.Errorf("%w: CacheEntries must be nonnegative; got %d", ErrInvalidOptions, options.CacheEntries)
	}
	evaluator := &Evaluator{}
	if options.CacheEntries > 0 {
		evaluator.cache = &solutionCache{capacity: options.CacheEntries, solve: solveUpper}
	}
	return evaluator, nil
}

// At evaluates geometric altitude in inputUnit and returns quantities in
// outputUnits. Caching is exact and applies only above 86 km. Invalid requests
// return a zero State and the same errors as the package function At.
func (e *Evaluator) At(altitude float64, inputUnit LengthUnit, outputUnits Units) (State, error) {
	if e.cache == nil {
		return At(altitude, inputUnit, outputUnits)
	}
	metres, err := altitudeMetres(altitude, inputUnit)
	if err != nil {
		return State{}, err
	}
	conversions, err := outputUnits.conversions()
	if err != nil {
		return State{}, err
	}
	var state State
	if metres <= upperBoundary {
		state = atSI(metres)
	} else {
		state = e.cache.load(metres).state
	}
	return outputUnits.fromSI(state, conversions), nil
}

// AtGeopotential evaluates geopotential altitude in inputUnit and returns
// quantities in outputUnits, sharing the cache with At and CompositionAt
func (e *Evaluator) AtGeopotential(altitude float64, inputUnit LengthUnit, outputUnits Units) (State, error) {
	if e.cache == nil {
		return AtGeopotential(altitude, inputUnit, outputUnits)
	}
	geometric, err := GeometricAltitude(altitude, inputUnit, Metre)
	if err != nil {
		return State{}, err
	}
	return e.At(geometric, Metre, outputUnits)
}

// CompositionAt returns species number densities in outputUnit at a geometric
// altitude in inputUnit. It accepts 86 to 1,000 km and shares physical solutions
// with At and AtGeopotential, independently of the requested output units.
func (e *Evaluator) CompositionAt(
	altitude float64, inputUnit LengthUnit, outputUnit NumberDensityUnit,
) (NumberDensities, error) {
	if e.cache == nil {
		return CompositionAt(altitude, inputUnit, outputUnit)
	}
	metres, err := altitudeMetres(altitude, inputUnit)
	if err != nil {
		return NumberDensities{}, err
	}
	output, err := outputUnit.conversion()
	if err != nil {
		return NumberDensities{}, err
	}
	if metres < upperBoundary {
		return NumberDensities{}, ErrCompositionUnavailable
	}
	var numbers NumberDensities
	if metres == upperBoundary {
		numbers = upperAtmosphere(metres)
	} else {
		numbers = e.cache.load(metres).numbers
	}
	return numbers.fromSI(outputUnit, output), nil
}

// CacheStats reports cache activity without resetting counters. An evaluator
// with caching disabled returns the zero snapshot. Reading statistics does not
// change cache order or activity counters.
func (e *Evaluator) CacheStats() CacheStats {
	if e.cache == nil {
		return CacheStats{}
	}
	e.cache.mu.Lock()
	defer e.cache.mu.Unlock()
	stats := e.cache.stats
	stats.Entries = len(e.cache.entries)
	stats.Capacity = e.cache.capacity
	stats.InFlight = len(e.cache.flights)
	return stats
}
