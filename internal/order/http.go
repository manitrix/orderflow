package order

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type API struct {
	service *Service
	ready   func(context.Context) error
	log     *slog.Logger
}

func NewHandler(service *Service, ready func(context.Context) error, log *slog.Logger) http.Handler {
	a := &API{service: service, ready: ready, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", a.create)
	mux.HandleFunc("GET /orders/{id}", a.get)
	mux.HandleFunc("GET /orders/{id}/status", a.status)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := a.ready(ctx); err != nil {
			respond(w, 503, map[string]string{"error": "database unavailable"})
			return
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get("X-Correlation-ID")
		if !validCorrelationID(id) {
			id = rand.Text()
		}
		w.Header().Set("X-Correlation-ID", id)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		recorder := &responseWriter{ResponseWriter: w, status: 200}
		request := r.WithContext(ctx)
		mux.ServeHTTP(recorder, request)
		log.Info("http request", "method", request.Method, "route", request.Pattern, "status", recorder.status, "duration_ms", time.Since(start).Milliseconds(), "correlation_id", id)
	})
}
func validCorrelationID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *API) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req CreateRequest
	if err := decoder.Decode(&req); err != nil {
		a.badJSON(w, err)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		a.badJSON(w, err)
		return
	}
	o, err := a.service.Create(r.Context(), req)
	if err != nil {
		var validation *ValidationError
		if errors.As(err, &validation) {
			respond(w, 400, map[string]string{"error": validation.Error()})
			return
		}
		a.internalError(w, err)
		return
	}
	w.Header().Set("Location", "/orders/"+o.ID)
	respond(w, 201, o)
}
func (a *API) badJSON(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		respond(w, 413, map[string]string{"error": "request body exceeds 64 KiB"})
		return
	}
	respond(w, 400, map[string]string{"error": "body must be one valid JSON object with known fields"})
}
func (a *API) load(w http.ResponseWriter, r *http.Request) (Order, bool) {
	o, err := a.service.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		respond(w, 404, map[string]string{"error": "order not found"})
		return Order{}, false
	}
	if err != nil {
		a.internalError(w, err)
		return Order{}, false
	}
	return o, true
}
func (a *API) get(w http.ResponseWriter, r *http.Request) {
	if o, ok := a.load(w, r); ok {
		respond(w, 200, o)
	}
}
func (a *API) status(w http.ResponseWriter, r *http.Request) {
	if o, ok := a.load(w, r); ok {
		respond(w, 200, map[string]any{"id": o.ID, "status": o.Status})
	}
}
func (a *API) internalError(w http.ResponseWriter, err error) {
	a.log.Error("request failed", "error", err, "correlation_id", w.Header().Get("X-Correlation-ID"))
	respond(w, 500, map[string]string{"error": "internal server error"})
}
