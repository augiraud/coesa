# coesa

A Go library for the [COESA 1976 standard atmosphere](https://ntrs.nasa.gov/citations/19770009539), covering −5 to 1,000 km. Input and output units are explicit. Requires Go 1.27+ and uses only the standard library.

```sh
go get github.com/augiraud/coesa
```

## Examples

### At

```go
package main

import (
	"fmt"
	"log"

	"github.com/augiraud/coesa"
)

func main() {
	air, err := coesa.At(10, coesa.Kilometre, coesa.SI())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%.2f K, %.0f Pa, %.5f kg/m³\n", air.Temperature, air.Pressure, air.Density)
	// Output: 223.25 K, 26500 Pa, 0.41351 kg/m³
}
```

### AtGeopotential

```go
air, err := coesa.AtGeopotential(11, coesa.Kilometre, coesa.SI())
fmt.Printf("%.2f K, %.0f Pa\n", air.Temperature, air.Pressure)
// Output: 216.65 K, 22632 Pa
```

### State.Transport

```go
air, err := coesa.At(0, coesa.Foot, coesa.Imperial())
transport, err := air.Transport() // Available up to 86 km.
fmt.Printf("%.2f %s\n", transport.SpeedOfSound, transport.Units.SpeedOfSound)
// Output: 1116.45 ft/s
```

### CompositionAt

```go
species, err := coesa.CompositionAt(500, coesa.Kilometre, coesa.PerCubicMetre) // From 86 km.
fmt.Printf("Hydrogen: %.1e particles/m³\n", species.H)
// Output: Hydrogen: 8.0e+10 particles/m³
```

### NumberDensities.Total

```go
species, err := coesa.CompositionAt(500, coesa.Kilometre, coesa.PerCubicCentimetre)
fmt.Printf("%.3e particles/cm³\n", species.Total())
// Output: 2.192e+07 particles/cm³
```

### GeopotentialAltitude

```go
height, err := coesa.GeopotentialAltitude(86, coesa.Kilometre, coesa.Kilometre)
fmt.Printf("%.3f km\n", height)
// Output: 84.852 km
```

### GeometricAltitude

```go
height, err := coesa.GeometricAltitude(11, coesa.Kilometre, coesa.Kilometre)
fmt.Printf("%.3f km\n", height)
// Output: 11.019 km
```

### AltitudeRange

```go
minimum, maximum, err := coesa.AltitudeRange(coesa.Kilometre)
fmt.Printf("%.0f to %.0f km\n", minimum, maximum)
// Output: -5 to 1000 km
```

### Optional evaluator

The functions above evaluate directly without shared state. For repeated upper-atmosphere queries, create a concurrent-safe evaluator with a bounded cache:

```go
evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: 1024})
air, err := evaluator.At(500, coesa.Kilometre, coesa.SI())
fmt.Printf("%.3e particles/m³\n", air.NumberDensity)
// Output: 2.192e+13 particles/m³
```

`At`, `AtGeopotential`, and `CompositionAt` share exact physical solutions across units. Calls at or below 86 km bypass the cache. `CacheStats()` reports reuse and eviction. `Options{}` or a zero-value `Evaluator` evaluates directly. See [cache behaviour and measurements](docs/performance.md#optional-evaluator).

## Units

Altitude arguments have an explicit unit. `SI()` and `Imperial()` select the output units below. Results record their units in `air.Units` or `species.Unit`; `Transport()` uses the state's selected units.

| Quantity | SI unit | Imperial unit |
| --- | --- | --- |
| Geometric and geopotential altitude | m | ft |
| Temperature | K | °F |
| Pressure | Pa | psi |
| Mass density | kg/m³ | slug/ft³ |
| Gravity | m/s² | ft/s² |
| Molar mass | kg/mol | lb/lbmol |
| Number density | particles/m³ | particles/ft³ |
| Sound speed | m/s | ft/s |
| Dynamic and kinematic viscosity | Pa s, m²/s | lbf s/ft², ft²/s |
| Thermal conductivity | W/(m K) | Btu_IT/(h ft °F) |

Select individual output units:

```go
units := coesa.SI()
units.Temperature = coesa.Celsius
units.Pressure = coesa.Hectopascal
air, err := coesa.At(10, coesa.Kilometre, units)
fmt.Printf("%.2f %s, %.2f %s\n", air.Temperature, air.Units.Temperature, air.Pressure, air.Units.Pressure)
// Output: -49.90 °C, 265.00 hPa
```

## Validation

Checked against 222 published reference rows for state, composition, and transport properties from the original NOAA/NASA/USAF report. At these reference heights, pressure agrees within 0.01%, density and individual species within 0.05%, and temperature within 0.006 K after the report's prescribed corrections. [Validation details](docs/validation.md) cover the sources, tolerances, integration refinement, and limitations. These figures describe agreement with the standard atmosphere, rather than weather prediction accuracy.

## Robustness

`State.Transport()` validates the physical fields it consumes in their recorded units. Invalid physical inputs and unrepresentable results return an error matching `ErrInvalidState`; error returns contain zero properties. Successful transport quantities are finite and positive. Use `errors.Is` to classify errors; see the [compatibility policy](docs/compatibility.md) for domains and error precedence.

Stateless functions and evaluators support concurrent use. CI runs correctness and race tests on Linux/macOS (amd64 and arm64) and Windows amd64. Statement coverage is 99.8%, and a 1.43-million-case transport fuzz campaign passed. See [design priorities](docs/qualities.md) for other properties and possible extensions.

## Performance

Representative serial benchmarks on an Intel Core i7-12700H, Linux amd64, Go 1.27.3:

| Query | Time per call |
| --- | --- |
| Direct evaluation at 10 km | ~130 ns |
| Direct evaluation at 500 km | ~1 ms |
| Warm evaluator cache hit at 500 km | ~77 ns |
| `State.Transport()` at 10 km, SI/imperial | ~145–148 ns |

Successful direct calls and warm cache hits allocate no heap memory. A cache holding 1,024 solutions retained about 196 KiB in profiling. Cache hit timings exclude construction and warmup; distinct heights still require calculation. See [benchmarks and profiling](docs/performance.md) for methodology, concurrent throughput, and miss costs.

## HTTP server

`cmd/coesa` serves evaluations over HTTP. `GET /v1/coesa?altitude_m=<metres>` returns the geometric state at that altitude in SI units, one unit per key. `altitude_m` is required and must lie within the supported range, otherwise the reply is `400` with a single `error` field. `-addr` (default `:8080`, or the `PORT` environment variable when it is set) selects the listen address and `-cache-entries` (default `0`) the bounded upper-atmosphere cache described above.

```sh
curl "http://127.0.0.1:8080/v1/coesa?altitude_m=10000"
# {"altitude_m":10000,"geopotential_altitude_m":9984.293438772525,"temperature_k":223.25209264797857,"pressure_pa":26499.89813925335,"density_kg_per_m3":0.4135104288988469,"gravity_m_per_s2":9.775868442887434,"molar_mass_kg_per_mol":0.0289644,"number_density_per_m3":8.597553155222755e+24}
```

## Container image

`Dockerfile` builds the command with cgo off and copies it into a distroless nonroot image. `scripts/build_docker.sh` produces `coesa:latest`; `docker-compose.yml` runs that image for local testing with the port published on loopback only.

```sh
./scripts/build_docker.sh
docker compose up    # add --build after changing the source
```

`EXPOSE 8080` documents the default `-addr` port. Overriding the flag means overriding the published ports as well.

`scripts/push_docker_image_to_gcloud.sh` re-tags `coesa:latest` and pushes it to a Google Artifact Registry Docker repository. The destination comes from `.env` at the repository root, which is not tracked by git; copy `.env.template` to `.env` and set:

| Variable | Meaning |
| --- | --- |
| `GOOGLE_CLOUD_PROJECT` | Google Cloud project id |
| `GOOGLE_CLOUD_REGION` | Region of the repository, for example `europe-west6`. The registry endpoint is regional, not zonal. |
| `ARTIFACT_REGISTRY_REPOSITORY` | Name of the Artifact Registry repository of type Docker |
| `CLOUD_RUN_SERVICE_ACCOUNT` | Name of the service account the service runs as, which needs no roles |

The pushed reference is `<GOOGLE_CLOUD_REGION>-docker.pkg.dev/<GOOGLE_CLOUD_PROJECT>/<ARTIFACT_REGISTRY_REPOSITORY>/coesa:latest`. Docker must already be authenticated to that endpoint, for example with `gcloud auth configure-docker <GOOGLE_CLOUD_REGION>-docker.pkg.dev`.

`scripts/deploy_cloud_run.sh` deploys that image to Cloud Run as a first-generation service sized for a demonstration: 0.08 vCPU, 256 MiB, concurrency 1, scaling between zero and one instance, a 30 second request timeout, and unauthenticated invocation. It reads its destination and runtime identity from the variables above, and assembles the image reference the same way the push script does. The image must be in the repository before deploying, and `gcloud` must be authenticated for the project. Scale-to-zero means an idle service accrues no instance charges, so raising the minimum instances is what makes a service like this cost money continuously.

The service runs as `<CLOUD_RUN_SERVICE_ACCOUNT>@<GOOGLE_CLOUD_PROJECT>.iam.gserviceaccount.com`, which needs no roles because the server calls no Google API. [Cloud Run setup](docs/cloud-run.md) lists the `gcloud` commands behind this: enabling the APIs, creating the repository, registering the credential helper, creating the account, and deploying.

MIT license. See [LICENSE](LICENSE).
