// Package coesa implements the U.S. Standard Atmosphere, 1976 (COESA 1976).
//
// At and AtGeopotential require an input altitude unit and output Units.
// SI and Imperial provide presets; Units also selects each output independently.
// State records the output units, which also apply to State.Transport.
// CompositionAt and the altitude conversions take explicit output units.
// AltitudeRange reports the model's geometric bounds in a selected unit.
// The model retains the constants adopted in 1976.
//
// Below 86 km, the model uses the seven hydrostatic layers and the molecular
// weight correction from 80 to 86 km. Above 86 km, it numerically integrates
// the defining diffusion and transport equations for nitrogen, oxygen, argon,
// helium, and hydrogen.
//
// Evaluation functions are safe for concurrent use: all calculation state is
// local to each call. Unit configurations and results are copied by value.
// Callers own the returned values and synchronize any concurrent writes to
// their own variables. Successful package-function evaluations do not allocate
// heap memory.
//
// NewEvaluator optionally adds a bounded, concurrent-safe cache for repeated
// upper-atmosphere queries. It shares exact physical solutions across query
// methods and units. Direct evaluation remains the default; the zero-value
// Evaluator is usable without construction. Warm cache hits allocate nothing;
// cold cache misses allocate bookkeeping storage.
//
// Reference: NOAA/NASA/USAF, U.S. Standard Atmosphere, 1976,
// NASA-TM-X-74335, Part 1, https://ntrs.nasa.gov/citations/19770009539.
package coesa
