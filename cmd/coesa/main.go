// Command coesa serves U.S. Standard Atmosphere, 1976, evaluations over HTTP.
//
// It exposes GET /v1/coesa?altitude_m=<metres>, which answers with the
// geometric state at that altitude in SI units. Errors return a JSON object with
// a single "error" field.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	altitudeParameter  = "altitude_m"
	defaultListenPort  = "8080"
	portParameter      = "PORT"
	readHeaderTimeout  = 5 * time.Second
	shutdownTimeout    = 5 * time.Second
	jsonContentType    = "application/json; charset=utf-8"
	invalidAltitudeMsg = altitudeParameter + " must be a number of metres between -5000 and 1000000"
)

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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
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

// newServeMux registers the HTTP routes of the service
func newServeMux(logger *slog.Logger, evaluator *coesa.Evaluator) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/coesa", handleAtmosphere(logger, evaluator))
	return mux
}

// handleAtmosphere evaluates the altitude_m query parameter and replies in SI units
func handleAtmosphere(logger *slog.Logger, evaluator *coesa.Evaluator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := evaluate(r, evaluator)
		if err != nil {
			writeError(r.Context(), logger, w, err)
			return
		}
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
		return atmosphereResponse{}, &requestError{status: http.StatusBadRequest, message: invalidAltitudeMsg}
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
		return &requestError{status: http.StatusBadRequest, message: invalidAltitudeMsg}
	}
	return &requestError{status: http.StatusInternalServerError, message: "atmosphere evaluation failed"}
}

func writeError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, err error) {
	if rejected, ok := errors.AsType[*requestError](err); ok {
		logger.WarnContext(ctx, "request rejected", "status", rejected.status, "error", rejected.message)
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
