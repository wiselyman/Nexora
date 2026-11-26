package app

import (
	"encoding/json"
	"strings"
	"testing"

	"nexora/internal/config"
)

func TestRedactSecretsRemovesCredentialsFromDiagnosticLogs(t *testing.T) {
	privateKey := "-----BEGIN PRIVATE KEY-----\nprivate-key-material\n-----END PRIVATE KEY-----"
	input := strings.Join([]string{
		"configured runtime-secret-value and local-mcp-secret-value",
		"Authorization: Bearer bearer-secret-value",
		"api_key=assigned-secret-value token=assigned-token-value",
		"database URL https://operator:url-secret-value@example.test/db",
		"OpenAI key sk-proj-0123456789abcdefghijklmnopqrstuv",
		privateKey,
	}, "\n")
	redacted := redactSecrets(input, "runtime-secret-value", "local-mcp-secret-value")
	for _, secret := range []string{
		"runtime-secret-value",
		"local-mcp-secret-value",
		"bearer-secret-value",
		"assigned-secret-value",
		"assigned-token-value",
		"url-secret-value",
		"sk-proj-0123456789abcdefghijklmnopqrstuv",
		"private-key-material",
	} {
		if strings.Contains(redacted, secret) {
			t.Errorf("redacted output still contains %q: %s", secret, redacted)
		}
	}
}

func TestDiagnosticBundleOmitsDirectIdentifiersAndRedactsLogs(t *testing.T) {
	service := New(config.Config{TunnelID: "tunnel_0123456789abcdef0123456789abcdef", FileRoots: []string{"/private/customer-data"}}, config.Secrets{
		RuntimeAPIKey: "runtime-secret-value",
		MCPToken:      "local-mcp-secret-value",
		Storage:       "system credential store",
	})
	service.LastError = "Authorization: Bearer bearer-secret-value"
	service.Log("runtime-secret-value seen while connecting")

	bundle := service.DiagnosticBundle()
	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	for _, value := range []string{
		"runtime-secret-value",
		"local-mcp-secret-value",
		"bearer-secret-value",
		"tunnel_0123456789abcdef0123456789abcdef",
		"/private/customer-data",
	} {
		if strings.Contains(output, value) {
			t.Errorf("diagnostic bundle unexpectedly contains %q: %s", value, output)
		}
	}
	for _, field := range []string{"generated_at", "app", "connection", "permissions", "credential_storage", "recent_logs"} {
		if _, ok := bundle[field]; !ok {
			t.Errorf("diagnostic bundle is missing expected field %q", field)
		}
	}
}
