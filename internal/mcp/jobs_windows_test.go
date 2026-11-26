//go:build windows

package mcp

import (
	"context"
	"testing"
	"time"

	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
)

func TestWindowsCommandJobCancellationAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
		want    string
	}{
		{name: "cancel", timeout: time.Minute, cancel: true, want: "cancelled"},
		{name: "timeout", timeout: 100 * time.Millisecond, want: "timed_out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := app.New(config.Config{AllowCommands: true}, config.Secrets{})
			s := NewServer(service, desktop.New(), "token")
			parentCtx, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			jobCtx, cancelJob := context.WithCancel(parentCtx)
			job := &Job{ID: randomID(), Command: "ping 127.0.0.1 -n 30 > nul", Started: time.Now(), State: "waiting_approval", ExitCode: -1, Cancel: cancelJob, finished: make(chan struct{})}
			go s.runCommandJobWithTimeout(jobCtx, job, job.Command, "", false, tc.timeout)
			approveWindowsTestCommand(t, service)
			waitForWindowsJobState(t, job, "running")
			if tc.cancel {
				job.Cancel()
			}
			waitForWindowsJobDone(t, job)
			job.mu.Lock()
			state := job.State
			job.mu.Unlock()
			if state != tc.want {
				t.Fatalf("job state = %q, want %q", state, tc.want)
			}
		})
	}
}

func approveWindowsTestCommand(t *testing.T, service *app.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pending := service.PendingCommandApprovals()
		if len(pending) > 0 {
			if !service.DecideCommandApproval(pending[0].ID, true) {
				t.Fatal("could not approve test command")
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("command approval did not become pending")
}

func waitForWindowsJobState(t *testing.T, job *Job, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job.mu.Lock()
		got := job.State
		job.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job did not reach state %q", want)
}

func waitForWindowsJobDone(t *testing.T, job *Job) {
	t.Helper()
	select {
	case <-job.finished:
	case <-time.After(10 * time.Second):
		t.Fatal("command job did not finish after cancellation/timeout")
	}
}
