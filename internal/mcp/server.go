package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"nexora/internal/app"
	"nexora/internal/desktop"
)

type Server struct {
	app             *app.Service
	desktop         desktop.Adapter
	token           string
	jobs            map[string]*Job
	mu              sync.Mutex
	toolMu          sync.Mutex
	activeTools     map[string]string
	protocolServer  *mcpsdk.Server
	protocolHandler http.Handler
}

func NewServer(service *app.Service, adapter desktop.Adapter, token string) *Server {
	s := &Server{app: service, desktop: adapter, token: token, jobs: map[string]*Job{}, activeTools: map[string]string{}}
	s.protocolServer = mcpsdk.NewServer(&mcpsdk.Implementation{Name: "Nexora", Version: app.Version}, &mcpsdk.ServerOptions{
		Capabilities:              &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{ListChanged: true}},
		SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"},
	})
	s.syncTools()
	s.protocolHandler = mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return s.protocolServer
	}, &mcpsdk.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		MaxRequestBodyBytes:          2 << 20,
		PropagateRequestCancellation: true,
	})
	return s
}

func (s *Server) syncTools() {
	s.toolMu.Lock()
	defer s.toolMu.Unlock()
	definitions := s.toolDefinitions()
	desired := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		serialized, err := json.Marshal(definition)
		if err != nil {
			continue
		}
		desired[definition.Name] = string(serialized)
	}
	var remove []string
	for name := range s.activeTools {
		if _, ok := desired[name]; !ok {
			remove = append(remove, name)
		}
	}
	if len(remove) > 0 {
		s.protocolServer.RemoveTools(remove...)
		for _, name := range remove {
			delete(s.activeTools, name)
		}
	}
	for _, definition := range definitions {
		fingerprint, ok := desired[definition.Name]
		if !ok || s.activeTools[definition.Name] == fingerprint {
			continue
		}
		definition := definition
		s.protocolServer.AddTool(&mcpsdk.Tool{
			Name:        definition.Name,
			Description: definition.Description,
			InputSchema: definition.Schema,
			Annotations: toolAnnotations(definition.Name),
		}, func(ctx context.Context, request *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			arguments := map[string]any{}
			if len(request.Params.Arguments) > 0 {
				if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
					return toolError(err), nil
				}
			}
			result, err := s.call(ctx, definition.Name, arguments)
			if err != nil {
				return toolError(err), nil
			}
			return toSDKToolResult(result), nil
		})
		s.activeTools[definition.Name] = fingerprint
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	provided := r.Header.Get("X-5060-Token")
	if s.token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.syncTools()
	s.protocolHandler.ServeHTTP(w, r)
}

// StopJobs cancels jobs that outlive their initiating MCP request and waits
// until their command runners finish or ctx expires.
func (s *Server) StopJobs(ctx context.Context) bool {
	s.mu.Lock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.Unlock()
	for _, job := range jobs {
		job.mu.Lock()
		done := job.Done
		job.mu.Unlock()
		if done {
			continue
		}
		if job.Cancel != nil {
			job.Cancel()
		}
	}
	for _, job := range jobs {
		job.mu.Lock()
		done := job.Done
		job.mu.Unlock()
		if done {
			continue
		}
		select {
		case <-job.finished:
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func toolAnnotations(name string) *mcpsdk.ToolAnnotations {
	readOnly := false
	destructive := false
	openWorld := false
	idempotent := false
	title := name
	switch name {
	case "system_info":
		title, readOnly = "Read system information", true
	case "command_start":
		title, destructive, openWorld = "Run an approved command", true, true
	case "command_status":
		title, readOnly = "Read command status", true
	case "command_stop":
		title, destructive, idempotent = "Stop a command", true, true
	case "file_list":
		title, readOnly = "List allowed files", true
	case "file_read":
		title, readOnly = "Read an allowed file", true
	case "file_write":
		title, destructive, idempotent = "Write an allowed file", true, true
	case "desktop_screenshot":
		title, readOnly = "Capture the desktop", true
	case "desktop_windows":
		title, readOnly = "List desktop windows", true
	case "desktop_pointer":
		title, destructive = "Control the pointer", true
	case "desktop_key":
		title, destructive = "Press keys", true
	case "desktop_type":
		title, destructive = "Type into the active window", true
	case "browser_open":
		title, openWorld = "Open a web page", true
	}
	return &mcpsdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  idempotent,
		OpenWorldHint:   &openWorld,
	}
}

func toolError(err error) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}}, IsError: true}
}

func toSDKToolResult(value any) *mcpsdk.CallToolResult {
	result, ok := value.(map[string]any)
	if !ok {
		encoded, _ := json.Marshal(value)
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(encoded)}}}
	}
	content, _ := result["content"].([]any)
	blocks := make([]mcpsdk.Content, 0, len(content))
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch block["type"] {
		case "text":
			text, _ := block["text"].(string)
			blocks = append(blocks, &mcpsdk.TextContent{Text: text})
		case "image":
			data, _ := block["data"].(string)
			mimeType, _ := block["mimeType"].(string)
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err == nil {
				blocks = append(blocks, &mcpsdk.ImageContent{Data: decoded, MIMEType: mimeType})
			}
		}
	}
	if len(blocks) == 0 {
		encoded, _ := json.Marshal(value)
		blocks = append(blocks, &mcpsdk.TextContent{Text: string(encoded)})
	}
	isError, _ := result["isError"].(bool)
	return &mcpsdk.CallToolResult{Content: blocks, IsError: isError}
}
