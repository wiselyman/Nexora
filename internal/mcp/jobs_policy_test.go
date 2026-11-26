package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
)

func TestCommandPermissionRevocationWhileAwaitingApproval(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(configHome, ".config"))
	t.Setenv("APPDATA", filepath.Join(configHome, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(configHome, "AppData", "Local"))

	for _, tc := range []struct {
		name          string
		admin         bool
		allowCommands bool
		allowAdmin    bool
		expectCancel  bool
	}{
		{name: "command permission", admin: false, allowCommands: false, allowAdmin: true, expectCancel: true},
		{name: "administrator permission", admin: true, allowCommands: true, allowAdmin: false, expectCancel: true},
		{name: "ordinary command while only administrator permission is revoked", admin: false, allowCommands: true, allowAdmin: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := app.New(config.Config{AllowCommands: true, AllowAdmin: true}, config.Secrets{})
			s := NewServer(service, desktop.New(), "token")
			jobCtx, cancel := context.WithCancel(context.Background())
			job := newTestCommandJob("echo must-not-run")
			job.Cancel = cancel
			go s.runCommandJob(jobCtx, job, job.Command, "", tc.admin)

			approval := waitForCommandApproval(t, service)
			if err := service.Update(config.Config{Name: "Nexora", TunnelClient: "tunnel-client", AllowCommands: tc.allowCommands, AllowAdmin: tc.allowAdmin}, ""); err != nil {
				cancel()
				t.Fatalf("revoke command permissions: %v", err)
			}
			if tc.expectCancel {
				if pending := service.PendingCommandApprovals(); len(pending) != 0 {
					cancel()
					t.Fatalf("pending approvals after permission revocation = %d, want 0", len(pending))
				}
				if service.DecideCommandApproval(approval.ID, true) {
					cancel()
					t.Fatal("revoked approval accepted a late decision")
				}
			} else {
				pending := service.PendingCommandApprovals()
				if len(pending) != 1 || pending[0].ID != approval.ID {
					cancel()
					t.Fatalf("ordinary approval was cancelled with admin permission: %+v", pending)
				}
				if !service.DecideCommandApproval(approval.ID, false) {
					cancel()
					t.Fatal("ordinary approval could not be denied")
				}
			}
			waitForPolicyJob(t, job)
			job.mu.Lock()
			state, output := job.State, job.Output
			job.mu.Unlock()
			if state != "approval_failed" {
				t.Fatalf("job state = %q, want approval_failed; output=%q", state, output)
			}
			if tc.expectCancel && (!strings.Contains(output, "permission") || !strings.Contains(output, "not run")) {
				t.Fatalf("job output does not explain revocation: %q", output)
			}
			if !tc.expectCancel && !strings.Contains(output, "denied by the local user") {
				t.Fatalf("ordinary command was not resolved by the local decision: %q", output)
			}
		})
	}
}

func waitForCommandApproval(t *testing.T, service *app.Service) app.CommandApproval {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pending := service.PendingCommandApprovals(); len(pending) > 0 {
			return pending[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("command approval did not become pending")
	return app.CommandApproval{}
}

func waitForPolicyJob(t *testing.T, job *Job) {
	t.Helper()
	select {
	case <-job.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("command job did not finish after permission revocation")
	}
}
