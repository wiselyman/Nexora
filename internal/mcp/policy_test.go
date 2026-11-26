package mcp

import (
	"context"
	"strings"
	"testing"

	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
)

func TestToolDefinitionsReflectCommandAndAdminPermissions(t *testing.T) {
	service := app.New(config.Config{}, config.Secrets{})
	s := NewServer(service, desktop.New(), "token")
	if hasTool(s.toolDefinitions(), "command_start") {
		t.Fatal("command_start exposed while command permission is disabled")
	}

	service.Config.AllowCommands = true
	if !hasTool(s.toolDefinitions(), "command_start") {
		t.Fatal("command_start hidden after command permission was enabled")
	}
	definition := findTool(s.toolDefinitions(), "command_start")
	properties := definition.Schema["properties"].(map[string]any)
	if _, exists := properties["admin"]; exists {
		t.Fatal("admin option exposed while administrator permission is disabled")
	}

	service.Config.AllowAdmin = true
	definition = findTool(s.toolDefinitions(), "command_start")
	properties = definition.Schema["properties"].(map[string]any)
	if _, exists := properties["admin"]; !exists {
		t.Fatal("admin option hidden after administrator permission was enabled")
	}
}

func TestToolCallsEnforceDisabledCommandAndAdminPermissions(t *testing.T) {
	service := app.New(config.Config{}, config.Secrets{})
	s := NewServer(service, desktop.New(), "token")
	if _, err := s.call(context.Background(), "command_start", map[string]any{"command": "echo must-not-run"}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled command call error = %v", err)
	}

	service.Config.AllowCommands = true
	if _, err := s.call(context.Background(), "command_start", map[string]any{"command": "echo must-not-run", "admin": true}); err == nil || !strings.Contains(err.Error(), "administrator") {
		t.Fatalf("disabled admin call error = %v", err)
	}
}

func hasTool(definitions []tool, name string) bool { return findTool(definitions, name) != nil }

func findTool(definitions []tool, name string) *tool {
	for i := range definitions {
		if definitions[i].Name == name {
			return &definitions[i]
		}
	}
	return nil
}
