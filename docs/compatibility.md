# Compatibility and supported inputs

## Go and platforms

The minimum Go version is the version declared in `go.mod`, currently Go 1.27. CI uses the latest available patch release in that Go minor line. Later Go releases are expected to work under Go's compatibility promise, but support claims refer to versions actually exercised in CI. Raising the minimum version requires a documented release change; the toolchain declaration is not changed as incidental housekeeping.

The platform checks cover Linux amd64 and arm64, macOS amd64 and arm64, and Windows amd64. The full lint, vulnerability, coverage, and release-configuration checks run on Linux amd64. Additional platforms execute the complete test suite both without cgo and with the race detector. The release job requires all platform jobs to pass. The library itself has no C or external module dependency; the race detector's C-toolchain requirements apply to testing. The workflow uses [standard GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), and [Go documents the race detector's supported platforms](https://go.dev/doc/articles/race_detector#Requirements).

This is a test matrix, not a promise to support every operating system and architecture Go can target. Floating-point tolerance checks are shared between platforms; values need not be bit-identical across compilers or architectures. A platform failure must be investigated before broadening a numerical tolerance. Tests are not silently skipped to advertise support.

## Public API and errors

The public package lives at the module root and is imported as `github.com/augiraud/coesa`.

Stable releases preserve source compatibility within a major version. Incompatible public API changes require a new major version; additive APIs belong in minor releases, and compatible fixes belong in patch releases. Fixes that reject previously invalid inputs can be patch releases, with the changed behaviour documented.

Input and output units remain explicit. Zero-value `Units` selects SI and a zero-value `Evaluator` evaluates directly without caching. Package-level functions remain independent of evaluators. Successful results are returned by value; callers synchronize changes to values they own. New caching or approximation strategies must be requested explicitly.

Classify errors with `errors.Is`; diagnostic text is not a stable parsing interface. Existing unit, altitude, transport-domain, composition-domain, and evaluator-option error identities are retained. Error returns provide zero-valued results rather than partial physical quantities.

`State.Transport()` uses `State.Units` to interpret altitude, temperature, density, and molar mass. Unit validation runs first. Nonfinite altitudes or heights below −5 km return an error wrapping `ErrInvalidState`; finite heights above 86 km retain `ErrTransportUnavailable`. At available heights, temperature must convert to a finite value above absolute zero, and density and molar mass to finite positive values. Invalid input conversions and transport results that overflow or underflow to zero in the selected output units also wrap `ErrInvalidState`. Every successful transport quantity is finite and strictly positive.

Only the four consumed physical inputs are validated. A hand-built state can supply those fields and valid `Units` without populating pressure, gravity, geopotential height, or number density. This validation checks representability and necessary physical signs; it does not establish that an arbitrary hand-built state matches the standard atmosphere or that the empirical relations describe other gases or extreme temperatures.

## Numerical contract

The defining model is COESA 1976, with its adopted constants, geometric/geopotential distinction, domains, and prescribed corrections. Weather, humidity, and solar-activity models must have distinct, explicit identities rather than silently changing this model. Default calls perform the defining calculation without interpolation or altitude quantization. Optional caching preserves that calculation and its exact input keys.

The published-reference fixtures, units, source provenance, and enforced tolerances in [validation](validation.md) are part of the release review. Changes to constants, layer boundaries, integration steps, or mathematical formulations require reference tests, refinement checks where relevant, and a before/after comparison of results and performance. Release notes must identify any intentional numerical changes and their measured size. Improving a numerical algorithm may change final floating-point bits while preserving the documented accuracy contract; bitwise identity between releases is not guaranteed.

Reference-point agreement is not a proven bound at all altitudes or an estimate of real atmospheric variability. Performance figures are measurements on named hardware and toolchains rather than latency guarantees. Successful direct evaluation and warm cache hits have allocation regression checks; compiler or platform changes that break those checks require investigation.
