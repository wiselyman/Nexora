package app

import (
	"strings"
	"testing"

	"nexora/internal/config"
)

func TestStartTunnelReportsMissingSetupAndInvalidClient(t *testing.T) {
	for _, tc := range []struct {
		name    string
		config  config.Config
		secrets config.Secrets
		wantErr string
	}{
		{name: "missing tunnel id and key", config: config.Config{TunnelClient: "tunnel-client"}, wantErr: "Tunnel ID 和运行时 API Key"},
		{name: "missing runtime key", config: config.Config{TunnelID: "tunnel_0123456789abcdef0123456789abcdef", TunnelClient: "tunnel-client"}, wantErr: "Tunnel ID 和运行时 API Key"},
		{name: "missing client executable", config: config.Config{TunnelID: "tunnel_0123456789abcdef0123456789abcdef", TunnelClient: "/definitely/missing/5060-test-client"}, secrets: config.Secrets{RuntimeAPIKey: "test-only"}, wantErr: "找不到 tunnel-client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := New(tc.config, tc.secrets)
			err := service.StartTunnel()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("StartTunnel() error = %v, want substring %q", err, tc.wantErr)
			}
			service.mu.Lock()
			starting, running, lastError := service.TunnelStarting, service.TunnelRunning, service.LastError
			service.mu.Unlock()
			if starting || running {
				t.Fatalf("failed startup left running state: starting=%v running=%v", starting, running)
			}
			if !strings.Contains(lastError, tc.wantErr) {
				t.Fatalf("persistent error = %q, want substring %q", lastError, tc.wantErr)
			}
		})
	}
}
