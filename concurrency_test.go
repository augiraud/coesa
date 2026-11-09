package coesa_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/augiraud/coesa"
)

// Shared inputs and read-only returned values exercise all evaluation paths.
// Each goroutine owns its results; no computation depends on goroutine order.
func TestConcurrentEvaluation(t *testing.T) {
	altitudes := []float64{-5000, 0, 10000, 85000, 86000, 100000, 150000, 500000, 1000000}
	units := coesa.Imperial()
	states := make([]concurrentSnapshot, len(altitudes))
	for index, altitude := range altitudes {
		state, err := snapshotAt(altitude, units)
		if err != nil {
			t.Fatal(err)
		}
		states[index] = state
	}
	const workers = 8
	start := make(chan struct{})
	failures := make(chan error, workers)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			<-start
			failures <- compareConcurrentEvaluation(altitudes, units, states)
		})
	}
	close(start)
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Error(err)
		}
	}
}

type concurrentSnapshot struct {
	state, geopotentialState coesa.State
	transport                coesa.TransportProperties
	composition              coesa.NumberDensities
	minimum, maximum, total  float64
}

func compareConcurrentEvaluation(altitudes []float64, units coesa.Units, states []concurrentSnapshot) error {
	for index, altitude := range altitudes {
		state, err := snapshotAt(altitude, units)
		if err != nil {
			return err
		}
		if state != states[index] {
			return fmt.Errorf("evaluation at %g m: concurrent result changed", altitude)
		}
		if altitude <= 86000 {
			// Also exercise concurrent reads of a single shared State value.
			transport, transportErr := states[index].state.Transport()
			if transportErr != nil {
				return fmt.Errorf("shared Transport: %w", transportErr)
			}
			if transport != states[index].transport {
				return fmt.Errorf("shared Transport(%g): result changed", altitude)
			}
		}
	}
	return nil
}

func snapshotAt(altitude float64, units coesa.Units) (concurrentSnapshot, error) {
	var result concurrentSnapshot
	var err error
	result.state, err = coesa.At(altitude, coesa.Metre, units)
	if err != nil {
		return result, fmt.Errorf("At: %w", err)
	}
	height, err := coesa.GeopotentialAltitude(altitude, coesa.Metre, coesa.Metre)
	if err != nil {
		return result, fmt.Errorf("GeopotentialAltitude: %w", err)
	}
	result.geopotentialState, err = coesa.AtGeopotential(height, coesa.Metre, units)
	if err != nil {
		return result, fmt.Errorf("AtGeopotential: %w", err)
	}
	result.minimum, result.maximum, err = coesa.AltitudeRange(coesa.Foot)
	if err != nil {
		return result, fmt.Errorf("AltitudeRange: %w", err)
	}
	if altitude <= 86000 {
		result.transport, err = result.state.Transport()
		if err != nil {
			return result, fmt.Errorf("Transport: %w", err)
		}
	}
	if altitude >= 86000 {
		result.composition, err = coesa.CompositionAt(altitude, coesa.Metre, coesa.PerCubicFoot)
		if err != nil {
			return result, fmt.Errorf("CompositionAt: %w", err)
		}
		result.total = result.composition.Total()
	}
	return result, nil
}
