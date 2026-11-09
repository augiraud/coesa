package coesa_test

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

type referenceDeviation struct {
	absolute, relative, atAbsolute, atRelative float64
	count                                      int
}

type referenceDeviations map[string]referenceDeviation

func (observed referenceDeviations) check(
	t *testing.T, name string, altitude, actual, expected, absoluteTolerance, relativeTolerance float64,
) {
	t.Helper()
	if math.IsNaN(actual) || math.IsInf(actual, 0) {
		t.Errorf("%s at %g m is not finite: %g", name, altitude, actual)
		return
	}
	difference := math.Abs(actual - expected)
	if difference > absoluteTolerance+relativeTolerance*math.Abs(expected) {
		t.Errorf("%s at %g m = %.12g, reference %.12g (absolute tolerance %g, relative tolerance %g)",
			name, altitude, actual, expected, absoluteTolerance, relativeTolerance)
	}
	maximum := observed[name]
	maximum.count++
	if difference > maximum.absolute {
		maximum.absolute, maximum.atAbsolute = difference, altitude
	}
	if expected != 0 {
		relative := difference / math.Abs(expected)
		if relative > maximum.relative {
			maximum.relative, maximum.atRelative = relative, altitude
		}
	}
	observed[name] = maximum
}

func (observed referenceDeviations) report(t *testing.T) {
	t.Helper()
	keys := make([]string, 0, len(observed))
	for name := range observed {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		maximum := observed[name]
		t.Logf("%s (%d rows): max absolute %.9g at %g m; max relative %.9g%% at %g m",
			name, maximum.count, maximum.absolute, maximum.atAbsolute, maximum.relative*100, maximum.atRelative)
	}
}

func referenceRows(t *testing.T, filename string, header []string, count int) [][]float64 {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != count+1 || !slices.Equal(rows[0], header) {
		t.Fatalf("%s: expected header %v and %d data rows", filename, header, count)
	}
	values := make([][]float64, 0, count)
	for index, row := range rows[1:] {
		parsed := make([]float64, len(row))
		for column, value := range row {
			parsed[column], err = strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(parsed[column]) || math.IsInf(parsed[column], 0) {
				t.Fatalf("%s row %d column %s: invalid number %q", filename, index+2, header[column], value)
			}
		}
		if index > 0 && parsed[0] <= values[index-1][0] {
			t.Fatalf("%s: reference coordinates must increase strictly", filename)
		}
		values = append(values, parsed)
	}
	return values
}

func publishedMassRatios(t *testing.T) map[float64]float64 {
	t.Helper()
	rows := referenceRows(t, "molar_mass_ratios.csv", []string{"altitude_m", "molar_mass_ratio", "source_page"}, 13)
	ratios := make(map[float64]float64, len(rows))
	for _, row := range rows {
		ratios[row[0]] = row[1]
	}
	return ratios
}
