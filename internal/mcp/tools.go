package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"nexora/internal/commandexec"
	"nexora/internal/fileaccess"
)

type Job struct {
	mu       sync.Mutex
	ID       string
	Command  string
	Started  time.Time
	State    string
	Done     bool
	ExitCode int
	Output   string
	Cancel   context.CancelFunc
	finished chan struct{}
}

const (
	maxActiveCommandJobs   = 5
	maxRetainedCommandJobs = 20
)

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"inputSchema"`
}

func (s *Server) toolDefinitions() []tool {
	cfg := s.app.SnapshotConfig()
	commandJobsAvailable := cfg.AllowCommands || s.hasCommandJobs()
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	definitions := []tool{
		{Name: "system_info", Description: "Read this computer's operating system, architecture, user and desktop availability.", Schema: obj(map[string]any{})},
		{Name: "command_start", Description: "Create a command job for an arbitrary shell command on this computer as the current user. The command can access or change anything that user can access, including files outside Nexora's file-folder allowlist. The job returns immediately and waits for the local user to approve the exact command in the Nexora service; approval expires after 90 seconds. Commands have a 5 minute time limit, at most five can run or await approval at once, and completed job history is capped at 20 entries. admin=true additionally requests OS administrator authorization and requires that permission to be enabled in Nexora settings.", Schema: obj(map[string]any{"command": str("Shell command to run; may read, modify, or delete data accessible to the current user. The exact command is shown for local approval before execution."), "cwd": str("Optional existing working directory; shown in the approval prompt"), "admin": map[string]any{"type": "boolean", "description": "Request the platform's administrator authorization prompt; requires administrator access to be enabled in Nexora settings and local approval of each command"}}, "command")},
		{Name: "command_status", Description: "Read the state and output of a command job. State is waiting_approval, running, completed, failed, cancelled, timed_out, or approval_failed.", Schema: obj(map[string]any{"job_id": str("Job id")}, "job_id")},
		{Name: "command_stop", Description: "Stop a running command job.", Schema: obj(map[string]any{"job_id": str("Job id")}, "job_id")},
		{Name: "file_list", Description: "List up to 1000 files and directories only within folders explicitly allowed in Nexora settings.", Schema: obj(map[string]any{"path": str("Directory path inside an allowed folder")}, "path")},
		{Name: "file_read", Description: "Read a UTF-8 text file (up to 1 MiB) only within folders explicitly allowed in Nexora settings.", Schema: obj(map[string]any{"path": str("File path inside an allowed folder")}, "path")},
		{Name: "file_write", Description: "Write UTF-8 text only within folders explicitly allowed in Nexora settings. A new file uses private permissions; an existing regular file keeps its permissions. New contents are fully written before replacement, so a failed write leaves the existing target unchanged.", Schema: obj(map[string]any{"path": str("File path inside an allowed folder"), "content": str("File content")}, "path", "content")},
		{Name: "desktop_screenshot", Description: "Capture the current desktop screenshot. Platform permissions and X11 tools may be required.", Schema: obj(map[string]any{})},
		{Name: "desktop_windows", Description: "List currently visible desktop windows.", Schema: obj(map[string]any{})},
		{Name: "desktop_pointer", Description: "Move the mouse pointer and optionally click (button: 0 move, 1 left, 2 middle, 3 right).", Schema: obj(map[string]any{"x": map[string]any{"type": "integer"}, "y": map[string]any{"type": "integer"}, "button": map[string]any{"type": "integer"}}, "x", "y")},
		{Name: "desktop_key", Description: "Press a key or key chord, for example Return or ctrl+l.", Schema: obj(map[string]any{"keys": str("Key or chord")}, "keys")},
		{Name: "desktop_type", Description: "Type text into the focused window.", Schema: obj(map[string]any{"text": str("Text to type")}, "text")},
		{Name: "browser_open", Description: "Open an HTTP or HTTPS URL in the default browser. Links containing an embedded username or password are rejected.", Schema: obj(map[string]any{"url": str("HTTP or HTTPS URL to open (maximum 8 KiB; no embedded username or password)")}, "url")},
	}
	visible := make([]tool, 0, len(definitions))
	for _, definition := range definitions {
		enabled := true
		switch definition.Name {
		case "command_start":
			enabled = cfg.AllowCommands
		case "command_status", "command_stop":
			enabled = commandJobsAvailable
		case "file_list", "file_read", "file_write":
			enabled = cfg.AllowFiles
		case "desktop_screenshot", "desktop_windows", "desktop_pointer", "desktop_key", "desktop_type":
			enabled = cfg.AllowDesktop
		case "browser_open":
			enabled = cfg.AllowBrowser
		}
		if !enabled {
			continue
		}
		if definition.Name == "command_start" && !cfg.AllowAdmin {
			properties := definition.Schema["properties"].(map[string]any)
			delete(properties, "admin")
			definition.Description += " Administrator execution is disabled in Nexora settings."
		}
		visible = append(visible, definition)
	}
	return visible
}

func (s *Server) hasCommandJobs() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs) > 0
}

func (s *Server) call(ctx context.Context, name string, a map[string]any) (any, error) {
	cfg := s.app.SnapshotConfig()
	switch name {
	case "system_info":
		return textResult(map[string]any{"hostname": hostname(), "os": runtime.GOOS, "architecture": runtime.GOARCH, "username": username(), "desktop": s.desktop.Info()}), nil
	case "command_start":
		if !cfg.AllowCommands {
			return nil, errors.New("command execution is disabled in Nexora settings")
		}
		command := stringArg(a, "command")
		if command == "" {
			return nil, errors.New("command is required")
		}
		if len(command) > 16<<10 {
			return nil, errors.New("command exceeds 16 KiB and cannot be shown for approval")
		}
		cwd := stringArg(a, "cwd")
		if cwd != "" {
			if st, err := os.Stat(cwd); err != nil || !st.IsDir() {
				return nil, errors.New("cwd must be an existing directory")
			}
		}
		admin, _ := a["admin"].(bool)
		if admin && !cfg.AllowAdmin {
			return nil, errors.New("administrator execution is disabled in Nexora settings")
		}
		jobCtx, cancel := context.WithCancel(context.Background())
		job := &Job{ID: randomID(), Command: command, Started: time.Now(), State: "waiting_approval", ExitCode: -1, Cancel: cancel, finished: make(chan struct{})}
		s.mu.Lock()
		if s.activeJobCountLocked() >= maxActiveCommandJobs {
			s.mu.Unlock()
			cancel()
			return nil, errors.New("maximum of five active command jobs reached")
		}
		s.jobs[job.ID] = job
		s.pruneJobsLocked()
		s.mu.Unlock()
		go s.runCommandJob(jobCtx, job, command, cwd, admin)
		return textResult(map[string]any{"job_id": job.ID, "state": "waiting_approval", "accepted": true, "approval_required": true}), nil
	case "command_status":
		job := s.getJob(stringArg(a, "job_id"))
		if job == nil {
			return nil, errors.New("job not found")
		}
		job.mu.Lock()
		defer job.mu.Unlock()
		return textResult(map[string]any{"job_id": job.ID, "state": job.State, "done": job.Done, "exit_code": job.ExitCode, "output": job.Output, "elapsed_ms": time.Since(job.Started).Milliseconds()}), nil
	case "command_stop":
		job := s.getJob(stringArg(a, "job_id"))
		if job == nil {
			return nil, errors.New("job not found")
		}
		job.mu.Lock()
		if job.Done {
			state := job.State
			job.mu.Unlock()
			return textResult(map[string]any{"stopping": false, "done": true, "state": state}), nil
		}
		job.mu.Unlock()
		job.Cancel()
		return textResult(map[string]any{"stopping": true, "done": false}), nil
	case "file_list":
		if !cfg.AllowFiles {
			return nil, errors.New("file access is disabled in Nexora settings")
		}
		root, p, err := fileaccess.OpenAuthorized(cfg.FileRoots, stringArg(a, "path"))
		if err != nil {
			return nil, err
		}
		defer root.Close()
		dir, e := root.Open(p)
		if e != nil {
			return nil, e
		}
		entries, e := dir.ReadDir(1001)
		_ = dir.Close()
		if e != nil && !errors.Is(e, io.EOF) {
			return nil, e
		}
		truncated := len(entries) > 1000
		if truncated {
			entries = entries[:1000]
		}
		rows := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			info, _ := entry.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			rows = append(rows, map[string]any{"name": entry.Name(), "directory": entry.IsDir(), "size": size})
		}
		if truncated {
			rows = append(rows, map[string]any{"name": "[listing truncated at 1000 entries; choose a narrower folder]", "directory": false, "size": 0})
		}
		return textResult(rows), nil
	case "file_read":
		if !cfg.AllowFiles {
			return nil, errors.New("file access is disabled in Nexora settings")
		}
		root, p, err := fileaccess.OpenAuthorized(cfg.FileRoots, stringArg(a, "path"))
		if err != nil {
			return nil, err
		}
		defer root.Close()
		f, err := root.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("file_read only supports regular files")
		}
		b, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if e != nil {
			return nil, e
		}
		if len(b) > 1<<20 {
			return nil, errors.New("file exceeds 1 MiB limit")
		}
		return textResult(string(b)), nil
	case "file_write":
		if !cfg.AllowFiles {
			return nil, errors.New("file access is disabled in Nexora settings")
		}
		p := stringArg(a, "path")
		if p == "" {
			return nil, errors.New("path is required")
		}
		if len(stringArg(a, "content")) > 1<<20 {
			return nil, errors.New("content exceeds 1 MiB limit")
		}
		resolved, e := fileaccess.Authorize(cfg.FileRoots, p, true)
		if e != nil {
			return nil, e
		}
		root, p, e := fileaccess.OpenAuthorized(cfg.FileRoots, resolved)
		if e != nil {
			return nil, e
		}
		defer root.Close()
		parent := "."
		if p != "." {
			parent = filepath.Dir(p)
		}
		if e := root.MkdirAll(parent, 0700); e != nil {
			return nil, e
		}
		if e := writeAuthorizedFile(root, p, []byte(stringArg(a, "content"))); e != nil {
			return nil, e
		}
		return textResult(map[string]any{"written": true, "path": stringArg(a, "path")}), nil
	case "desktop_screenshot":
		if !cfg.AllowDesktop {
			return nil, errors.New("desktop access is disabled in Nexora settings")
		}
		b, e := s.desktop.Screenshot()
		if e != nil {
			return nil, e
		}
		return map[string]any{"content": []any{map[string]any{"type": "image", "data": base64.StdEncoding.EncodeToString(b), "mimeType": "image/png"}}}, nil
	case "desktop_windows":
		if !cfg.AllowDesktop {
			return nil, errors.New("desktop access is disabled in Nexora settings")
		}
		v, e := s.desktop.Windows()
		if e != nil {
			return nil, e
		}
		return textResult(v), nil
	case "desktop_pointer":
		if !cfg.AllowDesktop {
			return nil, errors.New("desktop access is disabled in Nexora settings")
		}
		x, ok := intArg(a, "x")
		if !ok {
			return nil, errors.New("x must be an integer")
		}
		y, ok := intArg(a, "y")
		if !ok {
			return nil, errors.New("y must be an integer")
		}
		button, _ := intArg(a, "button")
		if button < 0 || button > 3 {
			return nil, errors.New("button must be between 0 and 3")
		}
		if e := s.desktop.Pointer(x, y, button); e != nil {
			return nil, e
		}
		return textResult(map[string]any{"ok": true}), nil
	case "desktop_key":
		if !cfg.AllowDesktop {
			return nil, errors.New("desktop access is disabled in Nexora settings")
		}
		v := stringArg(a, "keys")
		if len(v) > 100 {
			return nil, errors.New("key sequence too long")
		}
		if e := s.desktop.Key(v); e != nil {
			return nil, e
		}
		return textResult(map[string]any{"ok": true}), nil
	case "desktop_type":
		if !cfg.AllowDesktop {
			return nil, errors.New("desktop access is disabled in Nexora settings")
		}
		v := stringArg(a, "text")
		if len(v) > 10000 {
			return nil, errors.New("text too long")
		}
		if e := s.desktop.Type(v); e != nil {
			return nil, e
		}
		return textResult(map[string]any{"ok": true}), nil
	case "browser_open":
		if !cfg.AllowBrowser {
			return nil, errors.New("opening web pages is disabled in Nexora settings")
		}
		raw := stringArg(a, "url")
		if len(raw) > 8<<10 {
			return nil, errors.New("URL exceeds the 8 KiB limit")
		}
		u, e := url.Parse(raw)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, errors.New("only valid http and https URLs are allowed")
		}
		if u.User != nil {
			return nil, errors.New("URLs with embedded usernames or passwords are not allowed")
		}
		if e := openURL(raw); e != nil {
			return nil, e
		}
		return textResult(map[string]any{"opened": true, "url": raw}), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
func writeAuthorizedFile(root *os.Root, path string, contents []byte) error {
	if path == "" || path == "." {
		return errors.New("file_write requires a file path")
	}
	mode := os.FileMode(0600)
	info, err := root.Lstat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("file_write only supports regular files")
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	parent := filepath.Dir(path)
	if parent == "" {
		parent = "."
	}
	var stagedPath string
	var staged *os.File
	for attempt := 0; attempt < 10; attempt++ {
		stagedPath = filepath.Join(parent, ".5060-write-"+randomID())
		staged, err = root.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
	}
	if staged == nil {
		return errors.New("could not create a temporary file for the write")
	}
	defer func() {
		if stagedPath != "" {
			_ = root.Remove(stagedPath)
		}
	}()
	if err := root.Chmod(stagedPath, mode); err != nil {
		_ = staged.Close()
		return err
	}

	if _, err = staged.Write(contents); err != nil {
		_ = staged.Close()
		return err
	}
	if err = staged.Sync(); err != nil {
		_ = staged.Close()
		return err
	}
	if err = staged.Close(); err != nil {
		return err
	}
	if err := root.Rename(stagedPath, path); err != nil {
		return err
	}
	stagedPath = ""
	return nil
}

func (s *Server) getJob(id string) *Job { s.mu.Lock(); defer s.mu.Unlock(); return s.jobs[id] }

func (s *Server) runCommandJob(jobCtx context.Context, job *Job, command, cwd string, admin bool) {
	s.runCommandJobWithTimeout(jobCtx, job, command, cwd, admin, 5*time.Minute)
}

func (s *Server) runCommandJobWithTimeout(jobCtx context.Context, job *Job, command, cwd string, admin bool, timeout time.Duration) {
	defer s.app.FinishCommandExecution(job.ID)
	defer job.Cancel()
	if err := s.app.RequestCommandApproval(jobCtx, command, cwd, admin); err != nil {
		state := "approval_failed"
		if errors.Is(err, context.Canceled) {
			state = "cancelled"
		}
		finishCommandJob(job, state, err.Error(), -1)
		return
	}
	if err := jobCtx.Err(); err != nil {
		finishCommandJob(job, "cancelled", err.Error(), -1)
		return
	}

	cmdCtx, cancel := context.WithTimeout(jobCtx, timeout)
	defer cancel()
	cmd := commandexec.Build(cmdCtx, command, cwd, admin)
	stdoutCapture, stderrCapture := &boundedCapture{limit: 1 << 20}, &boundedCapture{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = stdoutCapture, stderrCapture
	started, err := s.app.StartCommandWithCurrentPermission(job.ID, admin, job.Cancel, cmd.Start)
	if !started {
		finishCommandJob(job, "approval_failed", "command permissions changed while waiting for approval; command was not run", -1)
		return
	}
	if err != nil {
		finishCommandJob(job, "failed", err.Error(), -1)
		return
	}
	job.mu.Lock()
	job.State = "running"
	job.mu.Unlock()
	err = cmd.Wait()
	output := stdoutCapture.String() + stderrCapture.String()
	if stdoutCapture.truncated || stderrCapture.truncated {
		output += "\n[output truncated at 1 MiB per stream]"
	}
	if err == nil {
		finishCommandJob(job, "completed", output, 0)
		return
	}
	var exitErr *exec.ExitError
	exitCode := -1
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	state := "failed"
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		state = "timed_out"
	} else if errors.Is(cmdCtx.Err(), context.Canceled) {
		state = "cancelled"
	}
	finishCommandJob(job, state, output+"\n"+err.Error(), exitCode)
}

func finishCommandJob(job *Job, state, output string, exitCode int) {
	job.mu.Lock()
	job.State = state
	job.Done = true
	job.Output = output
	job.ExitCode = exitCode
	close(job.finished)
	job.mu.Unlock()
}

func (s *Server) activeJobCountLocked() int {
	active := 0
	for _, job := range s.jobs {
		job.mu.Lock()
		if !job.Done {
			active++
		}
		job.mu.Unlock()
	}
	return active
}

func (s *Server) pruneJobsLocked() {
	for len(s.jobs) > maxRetainedCommandJobs {
		oldestID := ""
		var oldest time.Time
		for id, job := range s.jobs {
			job.mu.Lock()
			done, started := job.Done, job.Started
			job.mu.Unlock()
			if done && (oldestID == "" || started.Before(oldest)) {
				oldestID, oldest = id, started
			}
		}
		if oldestID == "" {
			return
		}
		delete(s.jobs, oldestID)
	}
}
func textResult(v any) map[string]any {
	var text string
	if s, ok := v.(string); ok {
		text = s
	} else {
		b, _ := json.MarshalIndent(v, "", "  ")
		text = string(b)
	}
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
}
func stringArg(a map[string]any, k string) string   { v, _ := a[k].(string); return v }
func intArg(a map[string]any, k string) (int, bool) { v, ok := a[k].(float64); return int(v), ok }
func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return "job-" + base64.RawURLEncoding.EncodeToString(b)
	}
	return fmt.Sprintf("job-%d", time.Now().UnixNano())
}

type boundedCapture struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedCapture) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if remaining > 0 {
		keep := n
		if keep > remaining {
			keep = remaining
		}
		_, _ = b.Buffer.Write(p[:keep])
		if keep < n {
			b.truncated = true
		}
	} else if n > 0 {
		b.truncated = true
	}
	return n, nil
}
func hostname() string { v, _ := os.Hostname(); return v }
func username() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERNAME")
	}
	return os.Getenv("USER")
}
func openURL(raw string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		cmd = exec.Command("xdg-open", raw)
	}
	return cmd.Start()
}
