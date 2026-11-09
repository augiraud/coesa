package coesa_test

import (
	"container/list"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/augiraud/coesa"
)

// These cache implementations are experiments, not part of the public API.
// Each evaluator has fixed output units and accepts geometric metres, so unit
// choices cannot collide in its keys. Results are returned by value.
type benchmarkEvaluator func(float64) (coesa.State, error)

type memoEntry struct {
	altitude float64
	state    coesa.State
}

type memoFlight struct {
	done  chan struct{}
	state coesa.State
	err   error
}

type benchmarkMemo struct {
	mu       sync.Mutex
	capacity int
	evaluate benchmarkEvaluator
	entries  map[float64]*list.Element
	flights  map[float64]*memoFlight
	order    list.List
}

func newBenchmarkMemo(capacity int, evaluate benchmarkEvaluator) *benchmarkMemo {
	return &benchmarkMemo{capacity: capacity, evaluate: evaluate,
		entries: make(map[float64]*list.Element, capacity), flights: make(map[float64]*memoFlight)}
}

func (c *benchmarkMemo) atMetres(altitude float64) (coesa.State, error) {
	// NaN is not equal to itself, so it must never enter a map of pending work.
	if math.IsNaN(altitude) {
		return c.evaluate(altitude)
	}
	c.mu.Lock()
	if element, ok := c.entries[altitude]; ok {
		c.order.MoveToFront(element)
		state := element.Value.(memoEntry).state
		c.mu.Unlock()
		return state, nil
	}
	if flight, ok := c.flights[altitude]; ok {
		c.mu.Unlock()
		<-flight.done
		return flight.state, flight.err
	}
	flight := &memoFlight{done: make(chan struct{})}
	c.flights[altitude] = flight
	c.mu.Unlock()

	state, err := c.evaluate(altitude)
	c.mu.Lock()
	if err == nil {
		element := c.order.PushFront(memoEntry{altitude: altitude, state: state})
		c.entries[altitude] = element
		if len(c.entries) > c.capacity {
			oldest := c.order.Back()
			delete(c.entries, oldest.Value.(memoEntry).altitude)
			c.order.Remove(oldest)
		}
	}
	flight.state, flight.err = state, err
	delete(c.flights, altitude)
	close(flight.done)
	c.mu.Unlock()
	return state, err
}

func benchmarkAtMetres(altitude float64) (coesa.State, error) {
	return coesa.At(altitude, coesa.Metre, coesa.SI())
}

func BenchmarkCacheReuse(b *testing.B) {
	for _, altitude := range []float64{10000, 500000} {
		for _, parallel := range []bool{false, true} {
			mode := "serial"
			if parallel {
				mode = "parallel"
			}
			for _, strategy := range []string{"direct", "bounded", "preloaded"} {
				b.Run(strconv.FormatFloat(altitude, 'f', 0, 64)+"/"+strategy+"/"+mode, func(b *testing.B) {
					evaluate := benchmarkStrategy(b, strategy, altitude)
					b.ReportAllocs()
					b.ResetTimer()
					benchmarkRepeated(b, evaluate, altitude, parallel)
				})
			}
		}
	}
}

func benchmarkStrategy(b *testing.B, strategy string, altitude float64) benchmarkEvaluator {
	b.Helper()
	state, err := benchmarkAtMetres(altitude)
	if err != nil {
		b.Fatal(err)
	}
	switch strategy {
	case "direct":
		return benchmarkAtMetres
	case "bounded":
		memo := newBenchmarkMemo(64, benchmarkAtMetres)
		if _, err = memo.atMetres(altitude); err != nil {
			b.Fatal(err)
		}
		return memo.atMetres
	case "preloaded":
		// Build once, then read without writes or locks.
		table := map[float64]coesa.State{altitude: state}
		return func(z float64) (coesa.State, error) {
			if value, ok := table[z]; ok {
				return value, nil
			}
			return benchmarkAtMetres(z)
		}
	default:
		b.Fatalf("unknown cache strategy %q", strategy)
		return nil
	}
}

func benchmarkRepeated(b *testing.B, evaluate benchmarkEvaluator, altitude float64, parallel bool) {
	b.Helper()
	if parallel {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				state, err := evaluate(altitude)
				if err != nil || state.Pressure <= 0 {
					b.Error("invalid evaluation", err)
					return
				}
			}
		})
		return
	}
	for b.Loop() {
		state, err := evaluate(altitude)
		if err != nil || state.Pressure <= 0 {
			b.Fatal("invalid evaluation", err)
		}
	}
}

func BenchmarkCacheScan(b *testing.B) {
	for _, base := range []float64{10000, 500000} {
		for _, cached := range []bool{false, true} {
			name := "direct"
			if cached {
				name = "bounded"
			}
			b.Run(strconv.FormatFloat(base, 'f', 0, 64)+"/"+name, func(b *testing.B) {
				var evaluations int
				evaluate := benchmarkEvaluator(func(z float64) (coesa.State, error) {
					evaluations++
					return benchmarkAtMetres(z)
				})
				if cached {
					evaluate = newBenchmarkMemo(64, evaluate).atMetres
				}
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
				b.ReportMetric(float64(evaluations)/float64(b.N), "evaluations/op")
			})
		}
	}
}

func BenchmarkCacheFill(b *testing.B) {
	const entries = 1024
	b.ReportAllocs()
	for b.Loop() {
		memo := newBenchmarkMemo(entries, benchmarkAtMetres)
		for altitude := range entries {
			if _, err := memo.atMetres(float64(altitude)); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(memo)
	}
	b.ReportMetric(entries, "entries/op")
}

func BenchmarkAtmosphereLayers(b *testing.B) {
	for _, altitude := range []float64{0, 80000, 86000, 110000, 149999, 150000, 200000, 1000000} {
		b.Run(strconv.FormatFloat(altitude, 'f', 0, 64), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := benchmarkAtMetres(altitude); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestMemoEvictionAndValues(t *testing.T) {
	var calls int
	memo := newBenchmarkMemo(2, func(z float64) (coesa.State, error) {
		calls++
		return benchmarkAtMetres(z)
	})
	for _, altitude := range []float64{10000, 11000, 10000, 12000, 11000} {
		state, err := memo.atMetres(altitude)
		if err != nil {
			t.Fatal(err)
		}
		if state != at(t, altitude) {
			t.Fatal("memoized result changed")
		}
	}
	if calls != 4 || len(memo.entries) != 2 {
		t.Fatalf("calls = %d, entries = %d", calls, len(memo.entries))
	}
	state, err := memo.atMetres(11000)
	if err != nil {
		t.Fatal(err)
	}
	state.Pressure = -1
	copyOfState, err := memo.atMetres(11000)
	if err != nil {
		t.Fatal(err)
	}
	if copyOfState.Pressure == state.Pressure {
		t.Fatal("cached result aliases caller's copy")
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), -6000} {
		before := calls
		for range 2 {
			if _, err = memo.atMetres(invalid); err == nil {
				t.Fatal("invalid altitude accepted")
			}
		}
		if calls-before != 2 || len(memo.flights) != 0 {
			t.Fatal("error retained in cache")
		}
	}
}

func TestMemoConcurrentMisses(t *testing.T) {
	var calls atomic.Int64
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	memo := newBenchmarkMemo(4, func(z float64) (coesa.State, error) {
		calls.Add(1)
		started <- struct{}{}
		<-release
		return benchmarkAtMetres(z)
	})
	const workers = 8
	results := make(chan coesa.State, workers)
	failures := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			state, err := memo.atMetres(500000)
			results <- state
			failures <- err
		})
	}
	<-started
	close(release)
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	want := at(t, 500000)
	for state := range results {
		if state != want {
			t.Fatal("concurrent cached result changed")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("same-key cold batch evaluated %d times", calls.Load())
	}
}

func TestMemoIndependentMissesOverlap(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	memo := newBenchmarkMemo(2, func(z float64) (coesa.State, error) {
		started <- struct{}{}
		<-release
		return benchmarkAtMetres(z)
	})
	failures := make(chan error, 2)
	for _, altitude := range []float64{10000, 11000} {
		go func() { _, err := memo.atMetres(altitude); failures <- err }()
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case <-started:
		case <-timer.C:
			t.Fatal("independent cache misses serialized during computation")
		}
	}
	unblock()
	for range 2 {
		if err := <-failures; err != nil {
			t.Fatal(err)
		}
	}
}

func TestSuccessfulEvaluationAllocations(t *testing.T) {
	state := at(t, 10000)
	rows := []struct {
		name     string
		evaluate func() error
	}{
		{"lower", func() error {
			_, err := benchmarkAtMetres(10000)
			return err
		}},
		{"upper", func() error {
			_, err := benchmarkAtMetres(500000)
			return err
		}},
		{"imperial", func() error {
			_, err := coesa.At(500, coesa.Kilometre, coesa.Imperial())
			return err
		}},
		{"transport", func() error {
			_, err := state.Transport()
			return err
		}},
		{"composition", func() error {
			_, err := coesa.CompositionAt(500, coesa.Kilometre, coesa.PerCubicMetre)
			return err
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(5, func() {
				if err := row.evaluate(); err != nil {
					t.Fatal(err)
				}
			})
			if allocations != 0 {
				t.Fatalf("%s: %g allocations per successful call", row.name, allocations)
			}
		})
	}
}

// Run explicitly from scripts/profile.sh so the snapshot includes a live cache,
// rather than a cache already collected after a benchmark returns
func TestProfileCacheRetained(t *testing.T) {
	directory := os.Getenv("COESA_PROFILE_DIR")
	if directory == "" {
		t.Skip("run scripts/profile.sh to capture the retained-cache profile")
	}
	memo := newBenchmarkMemo(1024, benchmarkAtMetres)
	for altitude := range 1024 {
		if _, err := memo.atMetres(float64(altitude)); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Create(filepath.Join(directory, "cache-retained.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	runtime.GC()
	if err = pprof.WriteHeapProfile(file); err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(memo)
}
