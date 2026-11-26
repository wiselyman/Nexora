package app

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"nexora/internal/config"
)

func TestTunnelReadinessTracksRecoveryAndLastSuccess(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(status.Load()))
	}))
	defer server.Close()

	service := New(config.Config{}, config.Secrets{})
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		service.pollHealthAt(done, server.URL+"/readyz", 10*time.Millisecond)
	}()
	waitForReadiness(t, service, false)

	status.Store(http.StatusOK)
	waitForReadiness(t, service, true)
	service.mu.Lock()
	lastSuccess := service.LastSuccessfulCheck
	service.mu.Unlock()
	if lastSuccess.IsZero() {
		t.Fatal("successful readiness response did not record its timestamp")
	}

	status.Store(http.StatusServiceUnavailable)
	waitForReadiness(t, service, false)
	service.mu.Lock()
	retainedSuccess := service.LastSuccessfulCheck
	service.mu.Unlock()
	if !retainedSuccess.Equal(lastSuccess) {
		t.Fatalf("last successful check changed after an unavailable response: got %v want %v", retainedSuccess, lastSuccess)
	}

	close(done)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("readiness polling did not stop after tunnel shutdown")
	}
}

func TestReadinessPollCannotRestoreReadyAfterTunnelExit(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := New(config.Config{}, config.Secrets{})
	service.TunnelRunning = true
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		service.pollHealthAt(done, server.URL+"/readyz", time.Millisecond)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("readiness request did not start")
	}

	close(done)
	service.finishTunnelProcess(nil)
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("readiness poll did not stop after the in-flight request")
	}
	service.mu.Lock()
	ready, running := service.TunnelReady, service.TunnelRunning
	service.mu.Unlock()
	if ready || running {
		t.Fatalf("late readiness response restored exited tunnel state: ready=%v running=%v", ready, running)
	}
}

func waitForReadiness(t *testing.T, service *Service, want bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		got := service.TunnelReady
		service.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("TunnelReady did not become %v", want)
}
