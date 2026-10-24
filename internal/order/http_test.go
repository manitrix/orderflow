package order

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLog(t *testing.T) {
	var logs bytes.Buffer
	h := NewHandler(NewService(&memoryRepo{orders: make(map[string]Order)}), func(context.Context) error { return nil }, slog.New(slog.NewJSONHandler(&logs, nil)))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/orders/missing", nil)
	r.Header.Set("X-Correlation-ID", "workflow-1")
	h.ServeHTTP(w, r)
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["route"] != "GET /orders/{id}" || entry["correlation_id"] != "workflow-1" || entry["status"] != float64(404) {
		t.Fatalf("incorrect request log: %v", entry)
	}
}

func testHandler(repo *memoryRepo, readyErr error) http.Handler {
	return NewHandler(NewService(repo), func(context.Context) error { return readyErr }, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}
func TestHTTPRoundTrip(t *testing.T) {
	handler := testHandler(&memoryRepo{orders: make(map[string]Order)}, nil)
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(`{"customer_id":"c1","items":[{"product_id":"p1","quantity":2,"unit_price_cents":125}]}`))
	req.Header.Set("X-Correlation-ID", "test-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Correlation-ID") != "test-123" {
		t.Fatal("correlation ID lost")
	}
	var o Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if w.Header().Get("Location") != "/orders/"+o.ID {
		t.Fatal("missing location")
	}
	for _, path := range []string{"/orders/" + o.ID, "/orders/" + o.ID + "/status"} {
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"PENDING"`) {
			t.Fatalf("get: %d %s", w.Code, w.Body.String())
		}
	}
}
func TestHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		dbErr, readyErr          error
		want                     int
	}{
		{name: "malformed", method: "POST", path: "/orders", body: `{`, want: 400},
		{name: "unknown field", method: "POST", path: "/orders", body: `{"extra":1}`, want: 400},
		{name: "multiple JSON values", method: "POST", path: "/orders", body: `{} {}`, want: 400},
		{name: "null", method: "POST", path: "/orders", body: `null`, want: 400},
		{name: "empty items", method: "POST", path: "/orders", body: `{"customer_id":"c"}`, want: 400},
		{name: "large", method: "POST", path: "/orders", body: `{"customer_id":"` + strings.Repeat("x", 70<<10) + `"}`, want: 413},
		{name: "missing", method: "GET", path: "/orders/missing", want: 404},
		{name: "missing status", method: "GET", path: "/orders/missing/status", want: 404},
		{name: "method", method: "DELETE", path: "/orders/x", want: 405},
		{name: "database", method: "GET", path: "/orders/x", dbErr: errors.New("secret database detail"), want: 500},
		{name: "liveness", method: "GET", path: "/healthz", readyErr: errors.New("down"), want: 200},
		{name: "readiness", method: "GET", path: "/readyz", readyErr: errors.New("down"), want: 503},
		{name: "ready", method: "GET", path: "/readyz", want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := testHandler(&memoryRepo{orders: make(map[string]Order), err: tc.dbErr}, tc.readyErr)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret") {
				t.Fatal("internal error leaked")
			}
			if w.Header().Get("X-Correlation-ID") == "" {
				t.Fatal("missing correlation ID")
			}
		})
	}
}
