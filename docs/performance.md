# Performance, concurrency, and caching

Measurements taken on 2026-11-04 with Go 1.27.3-X:nodwarf5, Linux amd64, and an Intel Core i7-12700H. These are synthetic workloads on one development machine. [Raw benchmark samples and profile summaries](benchmarks/2026-11-04.txt) include the pre-release baseline and the measurements after the allocation fix.

## CPU and allocations

The original unit-conversion setup allocated a 176-byte conversion structure on every successful evaluation. Both the allocation profile and compiler escape analysis identified `Units.conversions`: its interface-based conversion table held pointers into that structure. Replacing that table with direct value assignments eliminated the heap allocation without changing the atmospheric equations or unit conversions.

Medians of five 300 ms benchmark repetitions with one logical CPU:

| Geometric altitude | Before | After | Before allocation | After allocation |
| --- | --- | --- | --- | --- |
| 10 km | 178 ns | 126 ns | 176 B, 1 allocation | 0 B, 0 allocations |
| 100 km | 138 µs | 139 µs | 176 B, 1 allocation | 0 B, 0 allocations |
| 500 km | 997 µs | 1.02 ms | 176 B, 1 allocation | 0 B, 0 allocations |

The low-altitude improvement is about 30%. Upper-atmosphere timings overlap across repetitions and show no clear speed improvement from this change. Successful `At`, `CompositionAt`, and `State.Transport` calls have allocation regression coverage. Error construction can allocate.

Transport input and output validation was measured separately on 2026-11-06. At 10 km, medians of five 200 ms serial runs were 146 ns before versus 145 ns after validation in SI, and 146 ns before versus 148 ns after in imperial units. These small differences do not establish a performance change. Successful calls still allocate zero bytes. [Raw transport samples](benchmarks/2026-11-06-transport.txt) record the comparison with the pre-release implementation without validation; `BenchmarkTransport` is included in the profiling script.

In the optimized CPU profiles, `lowerAtmosphere` accounts for about 55% of the 10 km evaluation time and unit-conversion setup for about 19%, including their callees. At 500 km, `upperDerivative` accounts for about 94% and `upperStep` for about 99%. `math.archExp` alone accounts for about 45% of CPU samples; `math.Pow`, including its callees, accounts for about 53%. These cumulative percentages overlap.

High-altitude evaluations repeat integration from 86 km. From 150 km upward, a query also needs the hydrogen reference calculation at 500 km. The layer benchmarks show this cost: approximately 648 µs at 149.999 km, 1.03 ms at 150 km, and 1.44 ms at 1,000 km. This makes reuse of the physical solution the main opportunity for further improvement.

## Concurrent use

The package functions keep calculation state local to each call. Optional evaluators synchronize their caches and calculate different heights outside the cache lock. Unit configurations and results are copied by value. Callers can evaluate independently from multiple goroutines and share returned values for reading; they synchronize writes to their own variables.

The concurrency regression compares serial results with eight simultaneous callers across nine altitudes, both atmospheric regimes, geopotential conversions, composition, transport, and shared read-only states. Evaluator tests cover same-key miss coalescing, overlapping calculations for different keys, concurrent eviction and statistics reads, error handling, and independent returned copies. The concurrent tests passed ten repetitions at each of `GOMAXPROCS=1` and `8` under the [Go race detector](https://go.dev/doc/articles/race_detector). Race detection checks exercised execution paths; the implementation also keeps the numerical solver independent of shared mutable state.

## Optional evaluator

Package-level functions remain stateless and directly accessible. `NewEvaluator(Options{})` and a zero-value `Evaluator` also evaluate directly. Setting a positive `Options.CacheEntries` enables a per-instance least-recently-used cache; negative capacities return `ErrInvalidOptions`. Configuration is copied at construction and stays fixed. Copies of an evaluator share its cache; separately constructed evaluators are independent.

The cache retains canonical SI state and species together, so `At`, `AtGeopotential`, and `CompositionAt` share a solution across input and output units. Keys are exact geometric metres after the usual unit and geopotential conversion. Heights are never rounded or interpolated. Floating-point conversions can produce neighbouring values for mathematically equivalent heights; those remain separate keys. Values at or below exactly 86,000 geometric metres bypass the cache because their calculation is cheap. Transport properties remain calculated on demand from a returned state.

Simultaneous requests for the same uncached height share one calculation. Other heights calculate concurrently outside the cache lock. Invalid requests are rejected before cache access. Entries are allocated on demand and eviction recycles their storage. The capacity bounds retained solutions, rather than simultaneous calculations or temporary bookkeeping; services that need to limit concurrent work should apply their own concurrency limit. There is no global cache or time-based expiry: the underlying COESA model is fixed.

`CacheStats()` returns a consistent snapshot without resetting counters or changing recency. `Hits` counts retained-result lookups. `Misses` includes requests that join an existing calculation; `Coalesced` counts those joins. Thus `Misses - Coalesced` counts calculations started by the cache. `Evictions`, `Entries`, `Capacity`, and `InFlight` describe retained and pending work. Invalid requests and calls that bypass caching do not affect these counters.

Measurements of the public API on the machine described above, with a 64-entry cache and construction and warmup excluded. Values are medians of three 200 ms repetitions; parallel rows use eight logical CPUs and report aggregate wall time per operation.

| Workload | Package function | Zero-value evaluator | Cache disabled | Cache enabled |
| --- | --- | --- | --- | --- |
| 10 km, serial | 131 ns | 129 ns | 129 ns | 123 ns (bypass) |
| 500 km, serial | 1.00 ms | 957 µs | 987 µs | 77 ns (hit) |
| 10 km, eight parallel callers | 20.5 ns/op | 21.7 ns/op | 21.7 ns/op | 21.1 ns/op (bypass) |
| 500 km, eight parallel callers | 176 µs/op | 177 µs/op | 177 µs/op | 91.8 ns/op (hit) |

These success paths allocate nothing. Repeatedly requesting one warmed height establishes the benefit of exact reuse, rather than an application hit rate. The direct and disabled paths run the same solver; small timing differences are not evidence of different algorithms or accuracy. A separate comparison with the pre-release implementation before the evaluator was added retained zero allocations for package calls and found no clear upper-atmosphere performance regression. Across 1,911 state, composition, and transport outputs, the refactored core produced byte-identical JSON to that implementation in this build.

A cyclic scan of 1,024 distinct heights exceeds the 64-entry capacity. With cache nodes preallocated before timing, every upper-atmosphere query misses: medians of five 200 ms runs are 932 µs directly versus 939 µs through the evaluator around 500 km. Each such miss allocates 256 bytes in two temporary bookkeeping objects. Initial filling and map growth add allocations. Around 10 km the same scan bypasses caching and remains allocation-free. A full-sampling heap profile attributes about 196 KiB to an evaluator holding 1,024 physical solutions, including cache maps and entries, after pending calculations have completed. Memory costs vary with architecture and concurrent workload. [Raw evaluator benchmarks and retained-memory profile summary](benchmarks/2026-11-04-evaluator.txt) accompany these measurements.

## Earlier cache experiments

The implementations in [performance_test.go](../performance_test.go) are earlier benchmark prototypes. Their figures below exclude the public API's unit validation and conversion, and their memory footprint differs from the evaluator's combined physical solution. Each accepts geometric metres and binds one output configuration, avoiding unit collisions within an instance. They return exact calculated values by copy.

- The bounded cache uses a mutex, a map, and an LRU list. It retains at most 64 results in the throughput benchmarks. A per-key pending calculation lets simultaneous callers share a cold result; calculations for different keys run outside the cache lock. Errors are not cached, and NaN bypasses the maps.
- The preloaded table is an immutable map populated before use. Concurrent callers read it without a lock. Unknown keys fall back to direct evaluation. It performs exact lookup and does not interpolate.

Medians of three 200 ms repetitions, with cache construction and warmup excluded:

| Workload | Direct evaluation | Bounded cache hit | Preloaded exact lookup |
| --- | --- | --- | --- |
| 10 km, serial | 128 ns | 31 ns | 19 ns |
| 500 km, serial | 1.03 ms | 31 ns | 18 ns |
| 10 km, eight parallel callers | 22 ns/op | 73 ns/op | 2.0 ns/op |
| 500 km, eight parallel callers | 192 µs/op | 73 ns/op | 2.0 ns/op |

Parallel numbers are aggregate wall time per operation across eight logical CPUs, rather than individual request latency. These hit benchmarks repeatedly request one warmed altitude. They establish best-case reuse, not an application hit rate. Preloading requires calculation time and storage before those measurements begin.

For changing altitudes, a 1,024-point scan exceeds the 64-entry cache and produces a 100% miss rate. At around 10 km, direct evaluation takes 133 ns versus 395 ns through the cache. Around 500 km, direct evaluation takes 1.02 ms versus 1.09 ms through the cache. Each miss adds four allocations totalling 368 bytes; warm hits allocate nothing.

A separate full-sampling heap profile, taken while a 1,024-entry cache remains live, attributes about 180 KiB of retained memory to its entries, map, and LRU list. Building that cache allocates about 404 KiB cumulatively, including temporary pending calculations. These are different measurements: retained memory is the live footprint, while cumulative allocation includes objects later collected. The result limit does not cap the number of different requests currently in flight; a service would need a separate concurrency limit.

## Choosing a strategy

| Usage | Suitable approach | Tradeoff |
| --- | --- | --- |
| Occasional calls or mostly lower-atmosphere queries | Direct evaluation | Zero allocation, no retained results, scales without cache contention |
| Repeated upper-atmosphere queries in a service | `NewEvaluator(Options{CacheEntries: n})` | Strong benefit on hits; choose a capacity and measure actual reuse |
| A fixed set of requested heights | Immutable preloaded results | Fast concurrent reads; initial calculation and memory costs |
| A trajectory with mostly distinct high-altitude queries | Prepared integration checkpoints or a batch solver | Reuses integration work even without repeated keys; requires a separate implementation and accuracy validation |
| Plots or applications with an explicit error tolerance | Interpolated tables | Can cover arbitrary heights cheaply; changes numerical accuracy |

Direct evaluation remains the default. The optional evaluator bypasses cheap lower-atmosphere calculations and supports exact reuse above 86 km. Measure actual hit rates before choosing a cache capacity; mostly distinct heights gain little from retaining whole solutions.

For continuous trajectories, immutable checkpoints are the most promising next experiment. The existing integration grid has roughly 1,500 states of seven float64 integrals, suggesting about 0.1 MiB of checkpoint storage including coordinates. A query could resume from a nearby state and reuse the hydrogen reference calculation. This is a storage estimate, not an implemented performance result. Changed step placement needs verification against the defining equations and published reference tolerances before such an evaluator is offered.

## Other useful properties

| Consumer | Potential additions |
| --- | --- |
| Flight and orbit simulations | Batch evaluation, reusable solver checkpoints, pressure/density gradients, pressure altitude inversion with defined domains |
| Concurrent services | Cancellation and concurrency limits for bulk or queued work |
| Tables and visualizations | CSV/JSON export, reusable grids, interpolation with a declared error budget and explicit treatment of layer boundaries |
| Scientific workflows | Model/version metadata, cross-platform numerical tolerances, thermodynamic consistency checks, convergence studies, combined state and species results to avoid solving twice |

The core already offers explicit units, copied results, deterministic independent calls within a build, and [published-reference validation](validation.md). [Library qualities and priorities](qualities.md) evaluates portability, reproducibility, invalid physical states, API stability, and these optional extensions. New physical variations should be distinguished from the fixed COESA 1976 model so cached results remain associated with a well-defined model configuration.

## Reproduce

```sh
./scripts/profile.sh
# Or choose the output directory and sampling duration:
COESA_BENCHTIME=300ms COESA_BENCHCOUNT=5 COESA_CPUS=1,8 \
  COESA_PROFILE_TIME=5s ./scripts/profile.sh /tmp/coesa-profiles

go test -race . -run 'TestConcurrentEvaluation|TestEvaluator(Concurrent|Independent)|TestMemoConcurrentMisses|TestMemoIndependentMissesOverlap' -count=10 -cpu=1,8
```

The script builds the benchmark executable, records the environment, runs serial and concurrent benchmarks for the public evaluator and the earlier prototypes, and writes CPU and heap profiles with text summaries. CPU profiling and full allocation sampling run in separate processes because allocation stack collection can distort the fast path's CPU profile. A core-focused heap summary with no samples reflects the zero-allocation success path. Prototype cache allocation and live retention have separate profiles; `evaluator-retained.pprof` measures the public evaluator. Graphviz, when installed, also produces CPU call-graph SVGs. The script uses Go's `pprof` tool, an installed `pprof`, or builds `cmd/pprof` from the Go distribution's source when its binary is omitted. [Go profiling documentation](https://pkg.go.dev/runtime/pprof) describes the profile formats and interactive tools.

Outputs default to the ignored `profiles/` directory and include the matching executable needed to inspect them:

```sh
go tool pprof -http=127.0.0.1:8080 profiles/coesa.test profiles/upper.cpu.pprof
go tool pprof -alloc_space profiles/coesa.test profiles/cache-alloc.pprof
go tool pprof -inuse_space profiles/coesa.test profiles/cache-retained.pprof
go tool pprof -inuse_space profiles/coesa.test profiles/evaluator-retained.pprof
```

If this Go distribution omits the `pprof` binary, use `profiles/pprof` in place of `go tool pprof` after the script builds it.
