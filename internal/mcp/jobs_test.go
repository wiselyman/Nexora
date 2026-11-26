package mcp

import "time"

func newTestCommandJob(command string) *Job {
	return &Job{ID: randomID(), Command: command, Started: time.Now(), State: "waiting_approval", ExitCode: -1, finished: make(chan struct{})}
}
