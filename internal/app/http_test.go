package app

import (
	"bytes"
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"nexora/internal/config"
)

//go:embed http_test.go
var testUI embed.FS

func TestUIAPIRequiresPerLaunchSessionToken(t *testing.T) {
	service := New(config.Config{}, config.Secrets{})
	mux := http.NewServeMux()
	NewHTTP(service, testUI, "launch-secret", nil).Register(mux)

	for _, tc := range []struct {
		name   string
		token  string
		status int
	}{
		{name: "missing token", status: http.StatusUnauthorized},
		{name: "wrong token", token: "attacker-token", status: http.StatusUnauthorized},
		{name: "valid token", token: "launch-secret", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			if tc.token != "" {
				req.Header.Set("X-Nexora-UI-Session", tc.token)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestConfigMutationRequiresSameOriginPost(t *testing.T) {
	service := New(config.Config{}, config.Secrets{})
	mux := http.NewServeMux()
	NewHTTP(service, testUI, "launch-secret", nil).Register(mux)

	for _, tc := range []struct {
		name   string
		method string
		host   string
		origin string
		status int
	}{
		{name: "missing origin", method: http.MethodPost, host: "127.0.0.1:5060", status: http.StatusForbidden},
		{name: "attacker origin", method: http.MethodPost, host: "127.0.0.1:5060", origin: "http://attacker.example", status: http.StatusForbidden},
		{name: "origin matching attacker host", method: http.MethodPost, host: "attacker.example", origin: "http://attacker.example", status: http.StatusBadRequest},
		{name: "wrong method", method: http.MethodOptions, host: "127.0.0.1:5060", origin: "http://127.0.0.1:5060", status: http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/config", bytes.NewReader(nil))
			req.Host = tc.host
			req.Header.Set("X-Nexora-UI-Session", "launch-secret")
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestQuitRejectsMissingOriginAndAcceptsLocalOrigin(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	NewHTTP(New(config.Config{}, config.Secrets{}), testUI, "launch-secret", func() { called = true }).Register(mux)

	for _, tc := range []struct {
		name   string
		origin string
		status int
		called bool
	}{
		{name: "missing origin", status: http.StatusForbidden},
		{name: "forged origin", origin: "http://127.0.0.1.evil:5060", status: http.StatusForbidden},
		{name: "valid local origin", origin: "http://127.0.0.1:5060", status: http.StatusOK, called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodPost, "/api/quit", nil)
			req.Host = "127.0.0.1:5060"
			req.Header.Set("X-Nexora-UI-Session", "launch-secret")
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status || called != tc.called {
				t.Fatalf("status/callback = %d/%v, want %d/%v", rec.Code, called, tc.status, tc.called)
			}
		})
	}
}
