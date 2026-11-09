package coesa_test

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"

	"github.com/augiraud/coesa"
)

// Run separately with full allocation sampling to measure live cache storage
func TestProfileEvaluatorRetained(t *testing.T) {
	directory := os.Getenv("COESA_PROFILE_DIR")
	if directory == "" {
		t.Skip("run scripts/profile.sh to capture the retained-evaluator profile")
	}
	evaluator := newEvaluator(t, 1024)
	for index := range 1024 {
		if _, err := evaluator.At(86001+float64(index), coesa.Metre, coesa.SI()); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Create(filepath.Join(directory, "evaluator-retained.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	runtime.GC()
	if err = pprof.WriteHeapProfile(file); err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(evaluator)
}
