#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ ${1:-} == -h || ${1:-} == --help || $# -gt 1 ]]; then
  echo "Usage: $0 [output-directory]"
  echo "Optional: COESA_BENCHTIME=200ms COESA_BENCHCOUNT=3 COESA_CPUS=1,8 COESA_PROFILE_TIME=3s"
  exit 0
fi

profile_directory=${1:-profiles}
mkdir -p "$profile_directory"
profile_directory=$(cd "$profile_directory" && pwd)
profile_binary="$profile_directory/coesa.test"
profile_benchtime=${COESA_BENCHTIME:-200ms}
profile_count=${COESA_BENCHCOUNT:-3}
profile_cpus=${COESA_CPUS:-1,8}
profile_duration=${COESA_PROFILE_TIME:-3s}

if go tool pprof -h >/dev/null 2>&1; then
  profile_pprof() { go tool pprof "$@"; }
elif command -v pprof >/dev/null 2>&1; then
  profile_pprof() { pprof "$@"; }
else
  # Some distributions omit the binary but ship the Go tool's source.
  go build -o "$profile_directory/pprof" cmd/pprof
  profile_pprof() { "$profile_directory/pprof" "$@"; }
fi

{
  go version
  go env GOOS GOARCH
  git rev-parse HEAD
  git status --short
} > "$profile_directory/environment.txt"
go test -c -o "$profile_binary" .

echo "Benchmarking evaluations and cache workloads..."
"$profile_binary" -test.run '^$' -test.bench '^(BenchmarkAt|BenchmarkAtmosphereLayers|BenchmarkTransport)$' \
  -test.benchtime="$profile_benchtime" -test.count="$profile_count" -test.cpu=1 -test.benchmem \
  > "$profile_directory/evaluations.txt"
"$profile_binary" -test.run '^$' -test.bench '^Benchmark(Cache(Reuse|Scan|Fill)|Evaluator(Parallel|Scan)?)$' \
  -test.benchtime="$profile_benchtime" -test.count="$profile_count" -test.cpu="$profile_cpus" -test.benchmem \
  > "$profile_directory/caches.txt"

for profile_case in lower:10000 upper:500000; do
  profile_label=${profile_case%%:*}
  profile_altitude=${profile_case##*:}
  profile_filter="^BenchmarkAt$/^${profile_altitude}\$"
  echo "Profiling $profile_label atmosphere..."
  # CPU and full allocation sampling run separately to avoid distorting CPU data.
  "$profile_binary" -test.run '^$' -test.bench "$profile_filter" -test.cpu=1 \
    -test.benchtime="$profile_duration" -test.cpuprofile="$profile_directory/$profile_label.cpu.pprof" \
    > "$profile_directory/$profile_label-cpu-benchmark.txt"
  "$profile_binary" -test.run '^$' -test.bench "$profile_filter" -test.cpu=1 \
    -test.benchtime=1s -test.memprofilerate=1 -test.memprofile="$profile_directory/$profile_label.heap.pprof" \
    > "$profile_directory/$profile_label-heap-benchmark.txt"
  profile_pprof -top -nodecount=20 "$profile_binary" "$profile_directory/$profile_label.cpu.pprof" \
    > "$profile_directory/$profile_label-cpu.txt"
  for profile_sample in alloc_space inuse_space; do
    profile_pprof -top -"$profile_sample" -focus='github\.com/augiraud/coesa\.' \
      "$profile_binary" "$profile_directory/$profile_label.heap.pprof" \
      > "$profile_directory/$profile_label-$profile_sample.txt"
  done
  if command -v dot >/dev/null 2>&1; then
    profile_pprof -svg "$profile_binary" "$profile_directory/$profile_label.cpu.pprof" \
      > "$profile_directory/$profile_label-cpu.svg"
  fi
done

echo "Profiling cache allocation and retained memory..."
"$profile_binary" -test.run '^$' -test.bench '^BenchmarkCacheFill$' -test.benchtime=100ms \
  -test.cpu=1 -test.memprofilerate=1 -test.memprofile="$profile_directory/cache-alloc.pprof" \
  > "$profile_directory/cache-heap-benchmark.txt"
COESA_PROFILE_DIR="$profile_directory" "$profile_binary" -test.run '^TestProfile(Cache|Evaluator)Retained$' \
  -test.memprofilerate=1 > "$profile_directory/cache-retained-test.txt"
profile_pprof -top -alloc_space -focus='benchmarkMemo|newBenchmarkMemo' \
  "$profile_binary" "$profile_directory/cache-alloc.pprof" > "$profile_directory/cache-alloc.txt"
profile_pprof -top -inuse_space -focus='benchmarkMemo|newBenchmarkMemo' \
  "$profile_binary" "$profile_directory/cache-retained.pprof" > "$profile_directory/cache-retained.txt"
profile_pprof -top -inuse_space -focus='solutionCache|NewEvaluator' \
  "$profile_binary" "$profile_directory/evaluator-retained.pprof" > "$profile_directory/evaluator-retained.txt"
echo "Profiles and benchmark results: $profile_directory"
