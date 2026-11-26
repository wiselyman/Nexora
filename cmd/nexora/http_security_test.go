package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowedLocalHost(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		{host: "127.0.0.1:5060", want: true},
		{host: "LOCALHOST:5060", want: true},
		{host: "[::1]:5060", want: true},
		{host: "127.0.0.1.evil:5060"},
		{host: "localhost.evil:5060"},
		{host: "127.0.0.1:80"},
		{host: "localhost"},
	} {
		t.Run(tc.host, func(t *testing.T) {
			if got := allowedLocalHost(tc.host); got != tc.want {
				t.Fatalf("allowedLocalHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestLoopbackOnlyRejectsHostAndRemoteAddressAttacksBeforeRouting(t *testing.T) {
	routed := false
	handler := loopbackOnly(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		routed = true
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		name       string
		host       string
		remoteAddr string
		wantStatus int
		wantRoute  bool
	}{
		{name: "attacker host with attacker origin", host: "attacker.example:5060", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusForbidden},
		{name: "lookalike attacker host", host: "127.0.0.1.evil:5060", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusForbidden},
		{name: "remote nonloopback client", host: "127.0.0.1:5060", remoteAddr: "192.0.2.1:1234", wantStatus: http.StatusForbidden},
		{name: "valid local request", host: "127.0.0.1:5060", remoteAddr: "127.0.0.1:1234", wantStatus: http.StatusNoContent, wantRoute: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routed = false
			req := httptest.NewRequest(http.MethodGet, "http://"+tc.host+"/api/health", nil)
			req.Host = tc.host
			req.RemoteAddr = tc.remoteAddr
			if tc.name == "attacker host with attacker origin" {
				req.Header.Set("Origin", "http://attacker.example:5060")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus || routed != tc.wantRoute {
				t.Fatalf("status/routed = %d/%v, want %d/%v", rec.Code, routed, tc.wantStatus, tc.wantRoute)
			}
		})
	}
}
