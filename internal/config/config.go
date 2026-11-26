package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
)

type Config struct {
	Name                    string   `json:"name"`
	TunnelID                string   `json:"tunnel_id"`
	TunnelClient            string   `json:"tunnel_client"`
	FileRoots               []string `json:"file_roots"`
	AllowCommands           bool     `json:"allow_commands"`
	AllowFiles              bool     `json:"allow_files"`
	AllowDesktop            bool     `json:"allow_desktop"`
	AllowBrowser            bool     `json:"allow_browser"`
	AllowAdmin              bool     `json:"allow_admin"`
	StartOnLaunch           bool     `json:"start_on_launch"`
	ChatGPTGuideSeen        bool     `json:"chatgpt_guide_seen"`
	PermissionPolicyVersion int      `json:"permission_policy_version"`
}

const CurrentPermissionPolicyVersion = 2

type Secrets struct {
	RuntimeAPIKey string `json:"runtime_api_key"`
	MCPToken      string `json:"mcp_token"`
	Storage       string `json:"-"`
}

func NewMCPToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

var mu sync.Mutex

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Nexora"), nil
}

func legacyDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "5060"), nil
}

func Load() (Config, error) {
	dir, err := Dir()
	if err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		oldDir, oldErr := legacyDir()
		if oldErr != nil {
			return Config{}, oldErr
		}
		b, err = os.ReadFile(filepath.Join(oldDir, "config.json"))
		if errors.Is(err, os.ErrNotExist) {
			return Config{Name: "Nexora", TunnelClient: "tunnel-client", PermissionPolicyVersion: CurrentPermissionPolicyVersion}, nil
		}
		if err != nil {
			return Config{}, err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return Config{}, err
		}
		if err := atomicWrite(path, b, 0600); err != nil {
			return Config{}, err
		}
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if c.Name == "" {
		c.Name = "Nexora"
	}
	c.Name = "Nexora"
	if c.TunnelClient == "" {
		c.TunnelClient = "tunnel-client"
	}
	if c.PermissionPolicyVersion < 1 {
		// Older builds enabled these capabilities by default. Revoke that implicit
		// grant once and require the user to opt in again under the safer policy.
		c.AllowCommands = false
		c.AllowFiles = false
		c.AllowDesktop = false
		c.AllowAdmin = false
	}
	if c.PermissionPolicyVersion < 2 {
		// Opening a URL in the user's browser can reuse signed-in sessions and
		// trigger downloads, so this new capability always requires explicit opt-in.
		c.AllowBrowser = false
	}
	if c.PermissionPolicyVersion < CurrentPermissionPolicyVersion {
		c.PermissionPolicyVersion = CurrentPermissionPolicyVersion
	}
	return c, nil
}

func (c Config) Save() error {
	mu.Lock()
	defer mu.Unlock()
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := migrateLegacySecretsLocked(dir); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "config.json"), b, 0600)
}

func LoadSecrets() (Secrets, error) {
	mu.Lock()
	defer mu.Unlock()
	var s Secrets
	newRuntimeKey, newRuntimeErr := keyring.Get("Nexora", "openai-tunnel-runtime-key")
	legacyRuntimeKey := newRuntimeErr != nil
	legacyRuntimeSecret, legacyRuntimeErr := keyring.Get("5060", "openai-tunnel-runtime-key")
	if secret := first(newRuntimeKey, legacyRuntimeSecret); secret != "" {
		s.RuntimeAPIKey = secret
		if legacyRuntimeKey && legacyRuntimeErr == nil {
			if err := keyring.Set("Nexora", "openai-tunnel-runtime-key", secret); err == nil {
				_ = keyring.Delete("5060", "openai-tunnel-runtime-key")
			}
		}
	}
	newMCPToken, newMCPTokenErr := keyring.Get("Nexora", "local-mcp-token")
	legacyMCPToken := newMCPTokenErr != nil
	legacyToken, legacyTokenErr := keyring.Get("5060", "local-mcp-token")
	if token := first(newMCPToken, legacyToken); token != "" {
		s.MCPToken = token
		if legacyMCPToken && legacyTokenErr == nil {
			if err := keyring.Set("Nexora", "local-mcp-token", token); err == nil {
				_ = keyring.Delete("5060", "local-mcp-token")
			}
		}
	}
	dir, err := Dir()
	if err != nil {
		return Secrets{}, err
	}
	if err := migrateLegacySecretsLocked(dir); err != nil {
		return Secrets{}, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "secrets.json"))
	if errors.Is(err, os.ErrNotExist) {
		if s.RuntimeAPIKey != "" || s.MCPToken != "" {
			s.Storage = "system credential store"
			return s, nil
		}
		return Secrets{Storage: "not configured"}, nil
	}
	if err != nil {
		return Secrets{}, err
	}
	var fallback Secrets
	if err := json.Unmarshal(b, &fallback); err != nil {
		return Secrets{}, err
	}
	if s.RuntimeAPIKey == "" {
		s.RuntimeAPIKey = fallback.RuntimeAPIKey
	}
	if s.MCPToken == "" {
		s.MCPToken = fallback.MCPToken
	}
	if s.RuntimeAPIKey == "" && s.MCPToken == "" {
		oldDir, oldErr := legacyDir()
		if oldErr != nil {
			return Secrets{}, oldErr
		}
		legacyBytes, oldReadErr := os.ReadFile(filepath.Join(oldDir, "secrets.json"))
		if oldReadErr == nil {
			if jsonErr := json.Unmarshal(legacyBytes, &fallback); jsonErr != nil {
				return Secrets{}, jsonErr
			}
			if writeErr := atomicWrite(filepath.Join(dir, "secrets.json"), legacyBytes, 0600); writeErr != nil {
				return Secrets{}, writeErr
			}
			s.RuntimeAPIKey, s.MCPToken = fallback.RuntimeAPIKey, fallback.MCPToken
		} else if !errors.Is(oldReadErr, os.ErrNotExist) {
			return Secrets{}, oldReadErr
		}
	}
	if s.RuntimeAPIKey != "" || s.MCPToken != "" {
		s.Storage = "private config file fallback"
	} else {
		s.Storage = "not configured"
	}
	return s, nil
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func migrateLegacySecretsLocked(dir string) error {
	legacy, err := legacyDir()
	if err != nil {
		return err
	}
	oldPath, newPath := filepath.Join(legacy, "secrets.json"), filepath.Join(dir, "secrets.json")
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(oldPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := atomicWrite(newPath, data, 0600); err != nil {
		return err
	}
	return nil
}

func SaveSecrets(s Secrets) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	return saveSecretsLocked(s, false)
}

// ClearRuntimeAPIKey removes the runtime key from the system credential store
// and persists the remaining secrets without retaining a fallback copy.
func ClearRuntimeAPIKey(s Secrets) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	return saveSecretsLocked(s, true)
}

// PurgeUserData removes 5060's credentials from the OS credential store and
// deletes its user configuration directory. It is intended for an explicit
// uninstall choice while the app is not running.
func PurgeUserData() error {
	mu.Lock()
	defer mu.Unlock()
	var deleteErrors []error
	for _, service := range []string{"Nexora", "5060"} {
		for _, account := range []string{"openai-tunnel-runtime-key", "local-mcp-token"} {
			if err := keyring.Delete(service, account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
				deleteErrors = append(deleteErrors, fmt.Errorf("remove %s/%s from the system credential store: %w", service, account, err))
			}
		}
	}
	if err := errors.Join(deleteErrors...); err != nil {
		return err
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove Nexora configuration directory %s: %w", dir, err)
	}
	legacyDir, err := legacyDir()
	if err == nil {
		_ = os.RemoveAll(legacyDir)
	}
	return nil
}

func saveSecretsLocked(s Secrets, clearRuntimeAPIKey bool) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := migrateLegacySecretsLocked(dir); err != nil {
		return "", err
	}
	if clearRuntimeAPIKey {
		if err := keyring.Delete("Nexora", "openai-tunnel-runtime-key"); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("remove runtime API key from system credential store: %w", err)
		}
		if err := keyring.Delete("5060", "openai-tunnel-runtime-key"); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("remove legacy runtime API key: %w", err)
		}
		s.RuntimeAPIKey = ""
	}
	keyringOK := true
	if s.RuntimeAPIKey != "" && keyring.Set("Nexora", "openai-tunnel-runtime-key", s.RuntimeAPIKey) != nil {
		keyringOK = false
	}
	if s.MCPToken != "" && keyring.Set("Nexora", "local-mcp-token", s.MCPToken) != nil {
		keyringOK = false
	}
	if keyringOK {
		for _, account := range []string{"openai-tunnel-runtime-key", "local-mcp-token"} {
			if err := keyring.Delete("5060", account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
				keyringOK = false
			}
		}
	}
	if keyringOK {
		fallbackPath := filepath.Join(dir, "secrets.json")
		if err := os.Remove(fallbackPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			if clearRuntimeAPIKey {
				b, marshalErr := json.Marshal(s)
				if marshalErr != nil {
					return "", fmt.Errorf("cannot clear fallback secrets after keyring deletion: %w", marshalErr)
				}
				if writeErr := atomicWrite(fallbackPath, b, 0600); writeErr != nil {
					return "", fmt.Errorf("cannot remove or clear fallback secrets after keyring deletion: %w; overwrite failed: %v", err, writeErr)
				}
				return "private config file fallback", nil
			}
			return "", fmt.Errorf("secrets were saved to the system credential store, but the fallback secrets file could not be removed: %w", err)
		}
		return "system credential store", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	if err := atomicWrite(filepath.Join(dir, "secrets.json"), b, 0600); err != nil {
		return "", err
	}
	return "private config file fallback", nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmp, mode)
	}
	return os.Rename(tmp, path)
}

func ValidateTunnelID(id string) bool {
	if !strings.HasPrefix(id, "tunnel_") {
		return false
	}
	parts := strings.Split(id, "_")
	validPart := func(v string) bool {
		for _, r := range v {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return false
			}
		}
		return true
	}
	if len(parts) == 2 {
		return len(parts[1]) == 32 && validPart(parts[1])
	}
	if len(parts) == 3 {
		return len(parts[1]) == 4 && validPart(parts[1]) && len(parts[2]) == 32 && validPart(parts[2])
	}
	return false
}
