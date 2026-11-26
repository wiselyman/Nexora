//go:build darwin || linux

package mcp

import (
	"context"
	"testing"
	"time"

	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
)

func TestCommandJobCancellationReapsProcess(t *testing.T) {
	service := app.New(config.Config{AllowCommands: true}, config.Secrets{})
	s := NewServer(service, desktop.New(), "token")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job := newTestCommandJob("exec sleep 30")
	jobCtx, stopJob := context.WithCancel(ctx)
	job.Cancel = stopJob
	go s.runCommandJob(jobCtx, job, job.Command, "", false)
	approveTestCommand(t, service)
	waitForJobState(t, job, "running")

	started := time.Now()
	job.Cancel()
	waitForJobDone(t, job)
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("cancelled command took %s to finish, expected process-group termination", elapsed)
	}
	job.mu.Lock()
	state := job.State
	job.mu.Unlock()
	if state != "cancelled" {
		t.Fatalf("job state = %q, want cancelled", state)
	}
}

func TestCommandJobTimeoutReapsProcess(t *testing.T) {
	service := app.New(config.Config{AllowCommands: true}, config.Secrets{})
	s := NewServer(service, desktop.New(), "token")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job := newTestCommandJob("exec sleep 30")
	jobCtx, stopJob := context.WithCancel(ctx)
	job.Cancel = stopJob
	go s.runCommandJobWithTimeout(jobCtx, job, job.Command, "", false, 100*time.Millisecond)
	approveTestCommand(t, service)
	waitForJobDone(t, job)
	job.mu.Lock()
	state := job.State
	job.mu.Unlock()
	if state != "timed_out" {
		t.Fatalf("job state = %q, want timed_out", state)
	}
}

func approveTestCommand(t *testing.T, service *app.Service) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pending := service.PendingCommandApprovals()
		if len(pending) != 0 {
			if !service.DecideCommandApproval(pending[0].ID, true) {
				t.Fatal("could not approve test command")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("command approval did not become pending")
}

func waitForJobState(t *testing.T, job *Job, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job.mu.Lock()
		got := job.State
		job.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job did not reach state %q", want)
}

func waitForJobDone(t *testing.T, job *Job) {
	t.Helper()
	select {
	case <-job.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("command job did not finish after cancellation/timeout")
	}
}
