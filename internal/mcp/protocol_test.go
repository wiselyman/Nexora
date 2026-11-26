package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
)

func TestStreamableHTTPWorksWithOfficialSDKClient(t *testing.T) {
	for _, version := range []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"} {
		t.Run(version, func(t *testing.T) {
			service := app.New(config.Config{}, config.Secrets{})
			server := NewServer(service, desktop.New(), "local-test-token")
			httpServer := httptest.NewServer(server)
			defer httpServer.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "5060-protocol-test", Version: "1.0.0"}, nil)
			session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{
				Endpoint:             httpServer.URL,
				HTTPClient:           &http.Client{Transport: mcpTokenRoundTripper{base: http.DefaultTransport, token: "local-test-token"}},
				DisableStandaloneSSE: true,
			}, &mcpsdk.ClientSessionOptions{ProtocolVersion: version})
			if err != nil {
				t.Fatalf("official SDK client failed to connect: %v", err)
			}
			defer session.Close()
			if got := session.InitializeResult().ProtocolVersion; got != version {
				t.Fatalf("negotiated protocol version = %q, want %q", got, version)
			}

			tools, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools failed: %v", err)
			}
			var systemInfo *mcpsdk.Tool
			for _, tool := range tools.Tools {
				if tool.Name == "system_info" {
					systemInfo = tool
				}
				if tool.Name == "command_start" {
					t.Fatal("command_start was exposed while command permission was disabled")
				}
			}
			if systemInfo == nil {
				t.Fatal("system_info tool missing from ListTools response")
			}
			if systemInfo.Annotations == nil || !systemInfo.Annotations.ReadOnlyHint {
				t.Fatal("system_info tool is missing its read-only annotation")
			}

			result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "system_info", Arguments: map[string]any{}})
			if err != nil {
				t.Fatalf("CallTool failed: %v", err)
			}
			if result.IsError || len(result.Content) == 0 {
				t.Fatalf("system_info returned an invalid result: %#v", result)
			}

			service.Config.AllowCommands = true
			tools, err = session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools after permission change failed: %v", err)
			}
			foundCommand := false
			for _, tool := range tools.Tools {
				foundCommand = foundCommand || tool.Name == "command_start"
			}
			if !foundCommand {
				t.Fatal("command_start did not appear after command permission was enabled")
			}
		})
	}
}

func TestMCPHandlerRejectsMissingOrIncorrectToken(t *testing.T) {
	httpServer := httptest.NewServer(NewServer(app.New(config.Config{}, config.Secrets{}), desktop.New(), "expected-token"))
	defer httpServer.Close()

	for _, token := range []string{"", "incorrect-token"} {
		request, err := http.NewRequest(http.MethodGet, httpServer.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			request.Header.Set("X-5060-Token", token)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("request with token %q failed: %v", token, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("request with token %q returned %d, want %d", token, response.StatusCode, http.StatusUnauthorized)
		}
	}
}

type mcpTokenRoundTripper struct {
	base  http.RoundTripper
	token string
}

func (rt mcpTokenRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("X-5060-Token", rt.token)
	return rt.base.RoundTrip(clone)
}
