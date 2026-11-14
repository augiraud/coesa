// Command coesa serves U.S. Standard Atmosphere, 1976, evaluations over HTTP.
//
// It exposes GET /v1/coesa?altitude_m=<metres>, which answers with the
// geometric state at that altitude in SI units. Every reply is a JSON object with a single
// field: the state for a success, an "error" message for a rejection or an unknown route.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/augiraud/coesa"
)

const (
	altitudeParameter = "altitude_m"
	defaultListenPort = "8080"
	portParameter     = "PORT"
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
	jsonContentType   = "application/json; charset=utf-8"
	allowedMethods    = "GET, HEAD"
	methodNotAllowed  = "method not allowed"
	unknownPath       = "unknown path"
	// cacheControl is safe because the standard atmosphere is fixed: one altitude always
	// yields the same numbers.
	cacheControl = "public, max-age=86400"
)

// invalidAltitudeMessage words the supported altitude range as the library reports it, so the
// wording cannot drift from the bounds the evaluator enforces.
func invalidAltitudeMessage() string {
	minimum, maximum, err := coesa.AltitudeRange(coesa.Metre)
	if err != nil {
		return altitudeParameter + " must be a number of metres within the supported range"
	}
	return fmt.Sprintf("%s must be a number of metres between %s and %s", altitudeParameter,
		strconv.FormatFloat(minimum, 'f', -1, 64), strconv.FormatFloat(maximum, 'f', -1, 64))
}

// newLogger writes JSON records whose field names Cloud Logging selects on.
func newLogger(writer io.Writer) *slog.Logger {
	options := &slog.HandlerOptions{ReplaceAttr: cloudLoggingAttributes}
	return slog.New(slog.NewJSONHandler(writer, options))
}

// cloudLoggingAttributes renames slog's level, message and time keys onto the entry field
// names Cloud Logging uses, spelling the warning level as its severity enum does.
func cloudLoggingAttributes(groups []string, attribute slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return attribute
	}
	switch attribute.Key {
	case slog.LevelKey:
		level, ok := attribute.Value.Any().(slog.Level)
		if !ok {
			return attribute
		}
		severity := level.String()
		if severity == slog.LevelWarn.String() {
			severity = "WARNING"
		}
		return slog.String("severity", severity)
	case slog.MessageKey:
		attribute.Key = "message"
	case slog.TimeKey:
		attribute.Key = "timestamp"
	}
	return attribute
}

// atmosphereResponse is the geometric state at one altitude in SI units
type atmosphereResponse struct {
	AltitudeM             float64 `json:"altitude_m"`
	GeopotentialAltitudeM float64 `json:"geopotential_altitude_m"`
	TemperatureK          float64 `json:"temperature_k"`
	PressurePa            float64 `json:"pressure_pa"`
	DensityKgPerM3        float64 `json:"density_kg_per_m3"`
	GravityMPerS2         float64 `json:"gravity_m_per_s2"`
	MolarMassKgPerMol     float64 `json:"molar_mass_kg_per_mol"`
	NumberDensityPerM3    float64 `json:"number_density_per_m3"`
}

// errorResponse is the body of every non-2xx reply
type errorResponse struct {
	Error string `json:"error"`
}

// requestError carries the status and client-facing message for a rejected request
type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string { return e.message }

func main() {
	logger := newLogger(os.Stderr)
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	addr := flag.String("addr", defaultListenAddr(),
		"TCP address to listen on; defaults to the "+portParameter+" environment variable")
	cacheEntries := flag.Int("cache-entries", 0,
		"upper-atmosphere results to retain; zero evaluates every request")
	flag.Parse()

	evaluator, err := coesa.NewEvaluator(coesa.Options{CacheEntries: *cacheEntries})
	if err != nil {
		return fmt.Errorf("creating evaluator: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newServeMux(logger, evaluator),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "listening", "addr", *addr, "cache_entries", *cacheEntries)
		serveErrors <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.InfoContext(ctx, "shutting down", "addr", *addr)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return fmt.Errorf("shutting down: %w", shutdownErr)
		}
		return nil
	case serveErr := <-serveErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("listening on %s: %w", *addr, serveErr)
		}
		return nil
	}
}

// defaultListenAddr is the address the server listens on when -addr is not given.
// Platforms such as Cloud Run inject the port through the PORT environment variable.
func defaultListenAddr() string {
	if port := os.Getenv(portParameter); port != "" {
		return ":" + port
	}
	return ":" + defaultListenPort
}

// newServeMux registers the HTTP routes of the service. Unknown paths and methods are answered
// here rather than left to ServeMux, whose own replies are plain text, so that every response
// body is JSON.
func newServeMux(logger *slog.Logger, evaluator *coesa.Evaluator) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/coesa", handleCoesa(logger, evaluator))
	mux.HandleFunc("/", handleUnknownPath(logger))
	return mux
}

// handleCoesa serves the two methods the evaluation supports and rejects the rest in JSON.
func handleCoesa(logger *slog.Logger, evaluator *coesa.Evaluator) http.HandlerFunc {
	evaluate := handleAtmosphere(logger, evaluator)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", allowedMethods)
			writeJSON(r.Context(), logger, w, http.StatusMethodNotAllowed, errorResponse{Error: methodNotAllowed})
			return
		}
		evaluate(w, r)
	}
}

// handleUnknownPath answers paths the service does not serve
func handleUnknownPath(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(r.Context(), logger, w, http.StatusNotFound, errorResponse{Error: unknownPath})
	}
}

// handleAtmosphere evaluates the altitude_m query parameter and replies in SI units
func handleAtmosphere(logger *slog.Logger, evaluator *coesa.Evaluator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := evaluate(r, evaluator)
		if err != nil {
			writeError(r.Context(), logger, w, err)
			return
		}
		w.Header().Set("Cache-Control", cacheControl)
		writeJSON(r.Context(), logger, w, http.StatusOK, response)
	}
}

// evaluate reads the request parameters and evaluates the atmosphere in SI units.
// Rejected requests return an error matching [requestError].
func evaluate(r *http.Request, evaluator *coesa.Evaluator) (atmosphereResponse, error) {
	value := r.URL.Query().Get(altitudeParameter)
	if value == "" {
		return atmosphereResponse{}, &requestError{
			status:  http.StatusBadRequest,
			message: "missing query parameter " + altitudeParameter,
		}
	}
	metres, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return atmosphereResponse{}, &requestError{status: http.StatusBadRequest, message: invalidAltitudeMessage()}
	}
	state, err := evaluator.At(metres, coesa.Metre, coesa.SI())
	if err != nil {
		return atmosphereResponse{}, evaluationError(err)
	}
	return atmosphereResponse{
		AltitudeM:             state.Altitude,
		GeopotentialAltitudeM: state.GeopotentialAltitude,
		TemperatureK:          state.Temperature,
		PressurePa:            state.Pressure,
		DensityKgPerM3:        state.Density,
		GravityMPerS2:         state.Gravity,
		MolarMassKgPerMol:     state.MolarMass,
		NumberDensityPerM3:    state.NumberDensity,
	}, nil
}

// evaluationError maps an evaluation failure onto a client-facing error
func evaluationError(err error) error {
	if errors.Is(err, coesa.ErrAltitudeOutOfRange) {
		return &requestError{status: http.StatusBadRequest, message: invalidAltitudeMessage()}
	}
	return &requestError{status: http.StatusInternalServerError, message: "atmosphere evaluation failed"}
}

func writeError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) {
	if rejected, ok := errors.AsType[*requestError](err); ok {
		// Cloud Run records the status code of every request, so a rejected request adds
		// nothing to the logs and is not written here.
		writeJSON(ctx, logger, w, rejected.status, errorResponse{Error: rejected.message})
		return
	}
	logger.ErrorContext(ctx, "request failed", "error", err)
	writeJSON(ctx, logger, w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
}

func writeJSON(
	ctx context.Context, logger *slog.Logger, w http.ResponseWriter, status int, body any,
) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.ErrorContext(ctx, "encoding response", "status", status, "error", err)
	}
}
