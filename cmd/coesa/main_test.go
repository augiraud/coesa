package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/augiraud/coesa"
)

const (
	// Published values in testdata/geometric.csv and testdata/secondary.csv are rounded,
	// so the tolerances below match the agreement documented in docs/validation.md.
	temperatureTolerance = 0.01
	pressureTolerance    = 1e-4
	quantityTolerance    = 5e-4
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	evaluator, err := coesa.NewEvaluator(coesa.Options{})
	if err != nil {
		t.Fatalf("coesa.NewEvaluator: %v", err)
	}
	return newServeMux(slog.New(slog.DiscardHandler), evaluator)
}

func getAtmosphere(t *testing.T, handler http.Handler, target string) (atmosphereResponse, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))

	if contentType := recorder.Header().Get("Content-Type"); contentType != jsonContentType {
		t.Errorf("Content-Type = %q, want %q", contentType, jsonContentType)
	}
	if control := recorder.Header().Get("Cache-Control"); control != cacheControl {
		t.Errorf("Cache-Control = %q, want %q", control, cacheControl)
	}
	var response atmosphereResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding body %q: %v", recorder.Body.String(), err)
	}
	return response, recorder
}

func TestCoesaMatchesReferenceValues(t *testing.T) {
	handler := newTestHandler(t)

	tests := []struct {
		name               string
		altitude           string
		wantAltitudeM      float64
		wantTemperatureK   float64
		wantPressurePa     float64
		wantDensityKgPerM3 float64
	}{
		{name: "sea level", altitude: "0", wantTemperatureK: 288.150, wantPressurePa: 101_325,
			wantDensityKgPerM3: 1.2250},
		{name: "ten kilometres", altitude: "10000", wantAltitudeM: 10_000, wantTemperatureK: 223.252,
			wantPressurePa: 26_499, wantDensityKgPerM3: 0.41351},
		{name: "tropopause", altitude: "11000", wantAltitudeM: 11_000, wantTemperatureK: 216.774,
			wantPressurePa: 22_699, wantDensityKgPerM3: 0.36480},
		{name: "stratosphere", altitude: "20000", wantAltitudeM: 20_000, wantTemperatureK: 216.650,
			wantPressurePa: 5_529.3, wantDensityKgPerM3: 0.088910},
		{name: "below sea level", altitude: "-1000", wantAltitudeM: -1_000, wantTemperatureK: 294.651,
			wantPressurePa: 113_930, wantDensityKgPerM3: 1.3470},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, recorder := getAtmosphere(t, handler, "/v1/coesa?altitude_m="+test.altitude)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if response.AltitudeM != test.wantAltitudeM {
				t.Errorf("altitude_m = %v, want %v", response.AltitudeM, test.wantAltitudeM)
			}
			checkClose(t, "temperature_k", response.TemperatureK, test.wantTemperatureK, temperatureTolerance)
			checkClose(t, "pressure_pa", response.PressurePa, test.wantPressurePa,
				test.wantPressurePa*pressureTolerance)
			checkClose(t, "density_kg_per_m3", response.DensityKgPerM3, test.wantDensityKgPerM3,
				test.wantDensityKgPerM3*quantityTolerance)
		})
	}
}

// TestCoesaReportsSecondaryQuantitiesInSI pins the remaining response fields
// to the published row for 10 km, which is where a wrong unit or field mapping shows up.
func TestCoesaReportsSecondaryQuantitiesInSI(t *testing.T) {
	response, recorder := getAtmosphere(t, newTestHandler(t), "/v1/coesa?altitude_m=10000")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	checkClose(t, "geopotential_altitude_m", response.GeopotentialAltitudeM, 9_984, 0.5)
	checkClose(t, "gravity_m_per_s2", response.GravityMPerS2, 9.7759, 9.7759*quantityTolerance)
	checkClose(t, "number_density_per_m3", response.NumberDensityPerM3, 8.5976e24, 8.5976e24*quantityTolerance)
	checkClose(t, "molar_mass_kg_per_mol", response.MolarMassKgPerMol, 0.028964, 0.028964*quantityTolerance)
}

func TestCoesaAcceptsModelBounds(t *testing.T) {
	handler := newTestHandler(t)

	for _, altitude := range []string{"-5000", "86000", "1000000"} {
		t.Run(altitude, func(t *testing.T) {
			response, recorder := getAtmosphere(t, handler, "/v1/coesa?altitude_m="+altitude)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if response.TemperatureK <= 0 || response.PressurePa <= 0 || response.DensityKgPerM3 <= 0 {
				t.Errorf("non-positive state at %s m: %+v", altitude, response)
			}
			if response.GeopotentialAltitudeM >= response.AltitudeM {
				t.Errorf("geopotential_altitude_m = %v, want below altitude_m %v",
					response.GeopotentialAltitudeM, response.AltitudeM)
			}
		})
	}
}

func TestCoesaRejectsRequests(t *testing.T) {
	handler := newTestHandler(t)

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantError  string
		wantAllow  string
	}{
		{name: "missing parameter", method: http.MethodGet, target: "/v1/coesa",
			wantStatus: http.StatusBadRequest, wantError: "missing query parameter altitude_m"},
		{name: "empty parameter", method: http.MethodGet, target: "/v1/coesa?altitude_m=",
			wantStatus: http.StatusBadRequest, wantError: "missing query parameter altitude_m"},
		{name: "not a number", method: http.MethodGet, target: "/v1/coesa?altitude_m=cruise",
			wantStatus: http.StatusBadRequest, wantError: invalidAltitudeMessage()},
		{name: "above the model", method: http.MethodGet, target: "/v1/coesa?altitude_m=1000001",
			wantStatus: http.StatusBadRequest, wantError: invalidAltitudeMessage()},
		{name: "below the model", method: http.MethodGet, target: "/v1/coesa?altitude_m=-5001",
			wantStatus: http.StatusBadRequest, wantError: invalidAltitudeMessage()},
		{name: "NaN altitude", method: http.MethodGet, target: "/v1/coesa?altitude_m=NaN",
			wantStatus: http.StatusBadRequest, wantError: invalidAltitudeMessage()},
		{name: "wrong method", method: http.MethodPost, target: "/v1/coesa?altitude_m=0",
			wantStatus: http.StatusMethodNotAllowed, wantError: methodNotAllowed, wantAllow: allowedMethods},
		{name: "unknown path", method: http.MethodGet, target: "/v1/weather",
			wantStatus: http.StatusNotFound, wantError: unknownPath},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.target, nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.wantAllow != "" {
				if allow := recorder.Header().Get("Allow"); allow != test.wantAllow {
					t.Errorf("Allow = %q, want %q", allow, test.wantAllow)
				}
			}
			if test.wantError == "" {
				return
			}
			var body errorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decoding body %q: %v", recorder.Body.String(), err)
			}
			if body.Error != test.wantError {
				t.Errorf("error = %q, want %q", body.Error, test.wantError)
			}
		})
	}
}

// TestDefaultListenAddrFollowsPort pins the port a platform injects through the
// environment, which is how Cloud Run tells the container where to listen.
func TestDefaultListenAddrFollowsPort(t *testing.T) {
	tests := []struct {
		name string
		port string
		want string
	}{
		{name: "injected port", port: "9090", want: ":9090"},
		{name: "empty port falls back", port: "", want: ":" + defaultListenPort},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(portParameter, test.port)
			if got := defaultListenAddr(); got != test.want {
				t.Errorf("defaultListenAddr() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestCoesaAcceptsHead covers the second supported method, which the route pattern used to
// provide before the method check moved into the handler to keep rejections in JSON.
func TestCoesaAcceptsHead(t *testing.T) {
	recorder := httptest.NewRecorder()
	newTestHandler(t).ServeHTTP(recorder, httptest.NewRequest(http.MethodHead, "/v1/coesa?altitude_m=0", nil))
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

// TestLogRecordsUseCloudLoggingFieldNames pins the record shape Cloud Logging selects on:
// severity, message and timestamp rather than slog's level, msg and time.
func TestLogRecordsUseCloudLoggingFieldNames(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{level: slog.LevelInfo, want: "INFO"},
		{level: slog.LevelWarn, want: "WARNING"},
		{level: slog.LevelError, want: "ERROR"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			record := new(bytes.Buffer)
			newLogger(record).Log(context.Background(), test.level, "careful", "status", http.StatusBadRequest)
			assertCloudLoggingRecord(t, record, test.want)
		})
	}
}

// assertCloudLoggingRecord checks one emitted record for the Cloud Logging field names and for
// the absence of the slog names they replaced.
func assertCloudLoggingRecord(t *testing.T, record *bytes.Buffer, wantSeverity string) {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(record.Bytes(), &entry); err != nil {
		t.Fatalf("decoding record %q: %v", record.String(), err)
	}
	severity, ok := entry["severity"].(string)
	if !ok || severity != wantSeverity {
		t.Errorf("severity = %v, want %q in %q", entry["severity"], wantSeverity, record.String())
	}
	message, ok := entry["message"].(string)
	if !ok || message != "careful" {
		t.Errorf("message = %v, want %q in %q", entry["message"], "careful", record.String())
	}
	timestamp, ok := entry["timestamp"].(string)
	if !ok || timestamp == "" {
		t.Errorf("timestamp = %v, want a time in %q", entry["timestamp"], record.String())
	}
	status, ok := entry["status"].(float64)
	if !ok || status != http.StatusBadRequest {
		t.Errorf("status = %v, want %d in %q", entry["status"], http.StatusBadRequest, record.String())
	}
	for _, stale := range []string{"level", "msg", "time"} {
		if _, found := entry[stale]; found {
			t.Errorf("record %q still carries the slog key %q", record.String(), stale)
		}
	}
}

// TestLogRecordsBelowInfoAreNotWritten states the level the logger starts at, which is why a
// rejected request is not logged at all rather than logged at debug.
func TestLogRecordsBelowInfoAreNotWritten(t *testing.T) {
	records := new(bytes.Buffer)
	newLogger(records).Debug("careful")
	if records.Len() != 0 {
		t.Errorf("debug record %q was written, want none", records.String())
	}
}

// TestInvalidAltitudeMessageStatesThePublishedBounds pins the metres the rejection names. The
// message is built from coesa.AltitudeRange, so this fails when the library range changes rather
// than letting the two drift apart.
func TestInvalidAltitudeMessageStatesThePublishedBounds(t *testing.T) {
	message := invalidAltitudeMessage()
	for _, bound := range []string{"-5000", "1000000"} {
		if !strings.Contains(message, bound) {
			t.Errorf("invalidAltitudeMessage() = %q, want it to name %s metres", message, bound)
		}
	}
}

func checkClose(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %v, want %v within %v", name, got, want, tolerance)
	}
}
