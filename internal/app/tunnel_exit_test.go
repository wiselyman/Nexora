package app

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"nexora/internal/config"
)

func TestFinishTunnelProcessDistinguishesUnexpectedCleanExit(t *testing.T) {
	service := New(config.Config{}, config.Secrets{})
	service.Tunnel = &exec.Cmd{}
	service.TunnelRunning = true
	service.TunnelReady = true
	service.LastError = ""

	service.finishTunnelProcess(nil)

	if service.Tunnel != nil || service.TunnelRunning || service.TunnelReady || service.TunnelStopping {
		t.Fatalf("clean exit left stale tunnel state: tunnel=%v running=%v ready=%v stopping=%v", service.Tunnel, service.TunnelRunning, service.TunnelReady, service.TunnelStopping)
	}
	if !strings.Contains(service.LastError, "意外退出") {
		t.Fatalf("unexpected clean exit error = %q", service.LastError)
	}
	if lines := service.LogLines(); len(lines) == 0 || !strings.Contains(lines[len(lines)-1], "意外退出") {
		t.Fatalf("unexpected clean exit was not logged: %v", lines)
	}
}

func TestFinishTunnelProcessPreservesFailureAndClearsUserStop(t *testing.T) {
	t.Run("failed exit", func(t *testing.T) {
		service := New(config.Config{}, config.Secrets{})
		failure := errors.New("exit status 7")
		service.TunnelRunning = true
		service.TunnelStopping = false
		service.finishTunnelProcess(failure)
		if service.LastError != failure.Error() {
			t.Fatalf("failure error = %q, want %q", service.LastError, failure.Error())
		}
	})

	t.Run("user initiated stop", func(t *testing.T) {
		service := New(config.Config{}, config.Secrets{})
		service.TunnelRunning = true
		service.TunnelStopping = true
		service.LastError = "stale prior error"
		service.finishTunnelProcess(nil)
		if service.LastError != "" {
			t.Fatalf("user stop retained an error: %q", service.LastError)
		}
		if got := service.LogLines(); len(got) == 0 || !strings.Contains(got[len(got)-1], "已停止") {
			t.Fatalf("user stop log = %v", got)
		}
	})
}
