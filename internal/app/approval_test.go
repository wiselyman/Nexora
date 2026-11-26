package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"nexora/internal/config"
)

func TestRequestCommandApprovalAcceptedAndDenied(t *testing.T) {
	for _, tc := range []struct {
		name        string
		approve     bool
		wantErrPart string
	}{
		{name: "approved", approve: true},
		{name: "denied", approve: false, wantErrPart: "denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := New(config.Config{}, config.Secrets{})
			result := make(chan error, 1)
			go func() { result <- service.RequestCommandApproval(context.Background(), "echo hello", "/tmp", false) }()
			request := waitForApproval(t, service)
			if request.Command != "echo hello" || request.CWD != "/tmp" || request.Admin {
				t.Fatalf("approval details = %+v", request)
			}
			if !service.DecideCommandApproval(request.ID, tc.approve) {
				t.Fatal("decision was rejected while request was pending")
			}
			if err := <-result; tc.wantErrPart == "" && err != nil {
				t.Fatalf("approved command returned error: %v", err)
			} else if tc.wantErrPart != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrPart)) {
				t.Fatalf("denied command error = %v, want substring %q", err, tc.wantErrPart)
			}
			if service.DecideCommandApproval(request.ID, true) {
				t.Fatal("completed approval request accepted a second decision")
			}
		})
	}
}

func TestRequestCommandApprovalCancellationAndInputBounds(t *testing.T) {
	service := New(config.Config{}, config.Secrets{})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- service.RequestCommandApproval(ctx, "echo wait", "", false) }()
	request := waitForApproval(t, service)
	cancel()
	if err := <-result; err != context.Canceled {
		t.Fatalf("cancelled request error = %v, want context.Canceled", err)
	}
	if len(service.PendingCommandApprovals()) != 0 {
		t.Fatal("cancelled approval remained pending")
	}
	if service.DecideCommandApproval(request.ID, true) {
		t.Fatal("cancelled approval accepted a late decision")
	}

	if err := service.RequestCommandApproval(context.Background(), " \t ", "", false); err == nil {
		t.Fatal("empty command was accepted")
	}
	if err := service.RequestCommandApproval(context.Background(), strings.Repeat("x", 16<<10+1), "", false); err == nil {
		t.Fatal("command over 16 KiB was accepted")
	}
}

func TestCommandApprovalPendingLimit(t *testing.T) {
	service := New(config.Config{}, config.Secrets{})
	results := make([]chan error, 3)
	for i := range results {
		results[i] = make(chan error, 1)
		go func(result chan error) {
			result <- service.RequestCommandApproval(context.Background(), "echo waiting", "", false)
		}(results[i])
	}
	deadline := time.Now().Add(time.Second)
	for len(service.PendingCommandApprovals()) != 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := len(service.PendingCommandApprovals()); got != 3 {
		t.Fatalf("pending approvals = %d, want 3", got)
	}
	if err := service.RequestCommandApproval(context.Background(), "echo fourth", "", false); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("fourth pending request error = %v, want queue-limit error", err)
	}
	for _, request := range service.PendingCommandApprovals() {
		if !service.DecideCommandApproval(request.ID, false) {
			t.Fatalf("could not resolve pending approval %s", request.ID)
		}
	}
	for _, result := range results {
		if err := <-result; err == nil {
			t.Fatal("denied pending request unexpectedly succeeded")
		}
	}
}

func waitForApproval(t *testing.T, service *Service) CommandApproval {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pending := service.PendingCommandApprovals(); len(pending) > 0 {
			return pending[0]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("command approval did not become pending")
	return CommandApproval{}
}
