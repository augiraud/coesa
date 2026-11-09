# Library qualities and priorities

The useful common core is a small deterministic implementation with explicit units, traceable validation, defined domains, predictable errors, and copied results. Those properties benefit occasional calls, scientific work, and intensive services alike. Caching, prepared integrations, and approximations have workload-dependent costs and belong behind optional APIs.

## Current properties

| Quality | What is provided | Practical limit |
| --- | --- | --- |
| Scientific traceability | Original report and equation references, versioned offline fixtures, measured deviations, integration refinement checks | Finite reference samples do not prove a global error bound or real-weather accuracy |
| Explicit physical meaning | Unit parameters, unit-labelled results, geometric/geopotential distinction, separate transport and composition domains | COESA is one fixed atmosphere, with no location, date, humidity, or solar-activity inputs |
| Reproducible computation | Adopted 1976 constants, deterministic independent calls within a build, no external state | Floating-point results can vary slightly across builds and architectures |
| Concurrent use | Stateless package functions; evaluators synchronize mutable caches and share pending calculations | Callers synchronize writes to values they own |
| Predictable memory | Zero-allocation successful direct calls and warm hits; opt-in bounded retained solutions | Cache capacity does not bound distinct calculations in flight or all bookkeeping memory |
| Integration simplicity | Pure Go, standard library only, no C toolchain or runtime services | CI covers five OS/architecture combinations, rather than every Go target |
| Observability | Evaluator hit, miss, coalescing, eviction, and occupancy snapshots | Cache statistics do not replace measuring application hit rates or latency |

## Priorities that benefit most callers

1. **Preserve an explicit numerical contract.** Keep the reference fixtures and tolerances attached to releases. Treat changes to constants, boundary handling, and numerical methods as observable behaviour, even when the Go signatures do not change. Publish before/after accuracy and performance when changing the solver. The published-reference and refinement tests in [validation](validation.md) are the basis for that policy.
2. **Define supported platforms and API stability.** The CI matrix covers Linux amd64/arm64, macOS amd64/arm64, and Windows amd64. [Compatibility policy](compatibility.md) defines the Go baseline, supported inputs, error identities, release expectations, and numerical contract. Add model/source metadata if consumers need to record which atmosphere produced stored results.
3. **Make invalid physical values fail clearly.** Altitude and unit parameters reject invalid values. `State.Transport` validates its consumed physical inputs after interpreting their units, and rejects nonfinite or nonpositive calculated outputs. Zero and corrupted states return an error wrapping `ErrInvalidState` instead of successful NaNs. It accepts hand-built states that supply only the required fields.

These properties serve a broad range of callers without requiring every caller to choose a strategy or build a heavyweight evaluator.

## Optional capabilities, driven by a concrete workload

| Capability | Main consumers | Suggested shape and tradeoff |
| --- | --- | --- |
| Batch evaluation | Trajectories, tables, vectorized applications | An evaluator method with caller-owned output storage, defined ordering and partial-error behaviour; amortizes overhead without changing equations. Decide whether combined state and species output is useful before fixing the result type. |
| Prepared integration checkpoints | Repeated distinct upper-atmosphere heights | Immutable data behind an explicit evaluator option; promising when whole-solution caching misses. Validate changed step placement and compare memory/startup costs with the direct solver. |
| Pressure altitude inversion | Flight simulation, altimetry | A bracketed monotone solve with explicit input/output units, bounds, and termination tolerances. Keep geometric and geopotential answers distinct. |
| Gradients | Integrators, optimization, estimation | Specify analytic versus numerical derivatives and one-sided behaviour at layer boundaries. Do not imply every species is smooth: hydrogen is omitted below 150 km and enabled above it. |
| Approximate tables | Plotting, large parameter sweeps | A separately requested approximation with a declared interpolation/error policy, explicit boundary handling, and a distinct accuracy contract. Preserve exact evaluation as the default. |
| Bulk cancellation and concurrency limits | Services, large batches | Apply limits and context cancellation at batch or queue boundaries. Single direct evaluations are short; adding scheduling machinery to every scalar call has little benefit. |
| Structured export | Scientific pipelines and reproducible datasets | Include units, model identity, and library version. Prefer a small companion layer over coupling the numerical core to file formats or plotting frameworks. |

The core already provides defensive state validation, a platform test matrix, and an explicit compatibility policy. For new computation APIs, start with a real batch workload; benchmark it before choosing checkpoints, preloading, or interpolation. More atmospheric models should be separate, clearly identified configurations rather than silently changing COESA 1976 or invalidating existing cache keys.
