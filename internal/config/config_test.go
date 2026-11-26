package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFreshConfigStartsWithCapabilitiesDisabled(t *testing.T) {
	isolateUserConfig(t)

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.PermissionPolicyVersion != CurrentPermissionPolicyVersion {
		t.Fatalf("policy version = %d, want %d", c.PermissionPolicyVersion, CurrentPermissionPolicyVersion)
	}
	if c.AllowCommands || c.AllowFiles || c.AllowDesktop || c.AllowBrowser || c.AllowAdmin {
		t.Fatalf("fresh config unexpectedly enables capabilities: %+v", c)
	}
}

func TestLoadLegacyConfigRevokesImplicitCapabilitiesAndPreservesIndependentChoices(t *testing.T) {
	isolateUserConfig(t)
	dir, err := legacyDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"name":"My 5060","tunnel_id":"keep-me","allow_commands":true,"allow_files":true,"allow_desktop":true,"allow_browser":true,"allow_admin":true,"start_on_launch":true,"chatgpt_guide_seen":true}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AllowCommands || c.AllowFiles || c.AllowDesktop || c.AllowBrowser || c.AllowAdmin {
		t.Fatalf("legacy implicit capabilities were not revoked: %+v", c)
	}
	if c.Name != "Nexora" || c.TunnelID != "keep-me" || !c.StartOnLaunch || !c.ChatGPTGuideSeen {
		t.Fatalf("unrelated user choices were not preserved: %+v", c)
	}
	newDir, _ := Dir()
	if _, err := os.Stat(filepath.Join(newDir, "config.json")); err != nil {
		t.Fatalf("legacy configuration was not copied into Nexora directory: %v", err)
	}
	if c.PermissionPolicyVersion != CurrentPermissionPolicyVersion {
		t.Fatalf("policy version = %d, want %d", c.PermissionPolicyVersion, CurrentPermissionPolicyVersion)
	}
}

func TestLoadPolicyV1OnlyDisablesNewBrowserCapability(t *testing.T) {
	isolateUserConfig(t)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	prior := `{"permission_policy_version":1,"allow_commands":true,"allow_files":true,"allow_desktop":true,"allow_admin":true,"allow_browser":true}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(prior), 0600); err != nil {
		t.Fatal(err)
	}

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.AllowCommands || !c.AllowFiles || !c.AllowDesktop || !c.AllowAdmin {
		t.Fatalf("v1 explicit choices were unexpectedly revoked: %+v", c)
	}
	if c.AllowBrowser {
		t.Fatal("browser capability remained enabled after its opt-in migration")
	}
}

func isolateUserConfig(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("APPDATA", filepath.Join(root, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	dir, err := Dir()
	if err != nil {
		t.Fatalf("resolve isolated config directory: %v", err)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		t.Fatalf("config test is not isolated under its temporary home: %q (root %q)", dir, root)
	}
}

func TestValidateTunnelID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{id: "tunnel_0123456789abcdef0123456789abcdef", want: true},
		{id: "tunnel_ab12_0123456789abcdef0123456789abcdef", want: true},
		{id: "", want: false},
		{id: "tunnel_", want: false},
		{id: "tunnel_0123456789abcdef0123456789abcde", want: false},
		{id: "tunnel_0123456789abcdef0123456789abcdefg", want: false},
		{id: "tunnel_AB12_0123456789abcdef0123456789abcdef", want: false},
		{id: "tunnel_ab12_0123456789abcdef0123456789abc!", want: false},
		{id: "https://example.test/tunnel_0123456789abcdef0123456789abcdef", want: false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if got := ValidateTunnelID(tc.id); got != tc.want {
				t.Fatalf("ValidateTunnelID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
