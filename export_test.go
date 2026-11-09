package coesa

// NewObservedEvaluatorForTest installs an observer before the evaluator is
// published to other goroutines, allowing deterministic cold-miss tests
func NewObservedEvaluatorForTest(options Options, before func(float64)) (*Evaluator, error) {
	evaluator, err := NewEvaluator(options)
	if err != nil {
		return nil, err
	}
	if evaluator.cache != nil {
		evaluator.cache.solve = func(altitude float64) physicalSolution {
			before(altitude)
			return solveUpper(altitude)
		}
	}
	return evaluator, nil
}

// RefinedCompositionForTest keeps the equations and breakpoints fixed while
// dividing the production integration step sizes by refinement
func RefinedCompositionForTest(altitude, refinement float64) NumberDensities {
	return upperAtmosphereWithSteps(altitude, 100/refinement, 1000/refinement)
}
