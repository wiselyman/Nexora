package app

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"html"
	"net/http"
	"strings"

	"nexora/internal/config"
	"nexora/internal/desktop"
	"nexora/internal/folderpicker"
)

type HTTP struct {
	service  *Service
	ui       embed.FS
	uiToken  string
	shutdown func()
}

func NewHTTP(s *Service, ui embed.FS, uiToken string, shutdown func()) *HTTP {
	return &HTTP{service: s, ui: ui, uiToken: uiToken, shutdown: shutdown}
}
func (h *HTTP) Register(m *http.ServeMux) {
	m.HandleFunc("/", h.index)
	m.HandleFunc("/privacy", h.privacy)
	m.HandleFunc("/assets/5060.png", h.icon)
	m.HandleFunc("/assets/approval-text.js", func(w http.ResponseWriter, r *http.Request) {
		h.asset(w, r, "assets/approval-text.js", "text/javascript; charset=utf-8")
	})
	m.HandleFunc("/assets/ui-session.js", func(w http.ResponseWriter, r *http.Request) {
		h.asset(w, r, "assets/ui-session.js", "text/javascript; charset=utf-8")
	})
	m.HandleFunc("/assets/app.css", func(w http.ResponseWriter, r *http.Request) {
		h.asset(w, r, "assets/app.css", "text/css; charset=utf-8")
	})
	m.HandleFunc("/assets/app.js", func(w http.ResponseWriter, r *http.Request) {
		h.asset(w, r, "assets/app.js", "text/javascript; charset=utf-8")
	})
	m.Handle("/api/status", h.requireUIAuth(http.HandlerFunc(h.status)))
	m.Handle("/api/config", h.requireUIAuth(http.HandlerFunc(h.config)))
	m.Handle("/api/runtime-key/clear", h.requireUIAuth(http.HandlerFunc(h.clearRuntimeAPIKey)))
	m.Handle("/api/file-roots/select", h.requireUIAuth(http.HandlerFunc(h.selectFileRoot)))
	m.Handle("/api/tunnel-client/select", h.requireUIAuth(http.HandlerFunc(h.selectTunnelClient)))
	m.Handle("/api/command-approvals", h.requireUIAuth(http.HandlerFunc(h.commandApprovals)))
	m.Handle("/api/command-approval", h.requireUIAuth(http.HandlerFunc(h.commandApproval)))
	m.Handle("/api/start", h.requireUIAuth(http.HandlerFunc(h.start)))
	m.Handle("/api/stop", h.requireUIAuth(http.HandlerFunc(h.stop)))
	m.Handle("/api/quit", h.requireUIAuth(http.HandlerFunc(h.quit)))
	m.Handle("/api/logs", h.requireUIAuth(http.HandlerFunc(h.logs)))
	m.Handle("/api/diagnostics", h.requireUIAuth(http.HandlerFunc(h.diagnostics)))
	m.Handle("/api/health", h.requireUIAuth(http.HandlerFunc(h.health)))
}

func (h *HTTP) requireUIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Nexora-UI-Session")
		if h.uiToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(h.uiToken)) != 1 {
			http.Error(w, "本机配置页面访问凭据无效，请重新打开 Nexora。", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (h *HTTP) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := h.ui.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	page := strings.Replace(string(b), "__NEXORA_UI_SESSION__", html.EscapeString(h.uiToken), 1)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; object-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}
func (h *HTTP) privacy(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/privacy" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	b, err := h.ui.ReadFile("privacy.html")
	if err != nil {
		http.Error(w, "privacy information unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; object-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; form-action 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodGet {
		_, _ = w.Write(b)
	}
}
func (h *HTTP) asset(w http.ResponseWriter, r *http.Request, name, contentType string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	b, err := h.ui.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	// These URLs are stable across releases, so clients must fetch the current
	// embedded UI assets after 5060 is upgraded.
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		_, _ = w.Write(b)
	}
}
func (h *HTTP) icon(w http.ResponseWriter, r *http.Request) {
	b, err := h.ui.ReadFile("assets/5060.png")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(b)
}
func (h *HTTP) status(w http.ResponseWriter, r *http.Request) {
	status := h.service.Status()
	status["desktop_environment"] = desktop.New().Info()
	jsonOut(w, status)
}
func (h *HTTP) health(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"ok": true, "service": "Nexora"})
}
func (h *HTTP) quit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.shutdown == nil {
		http.Error(w, "shutdown unavailable", http.StatusServiceUnavailable)
		return
	}
	h.shutdown()
	jsonOut(w, map[string]bool{"ok": true})
}
func (h *HTTP) clearRuntimeAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := h.service.ClearRuntimeAPIKey(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (h *HTTP) logs(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"lines": h.service.LogLines()})
}
func (h *HTTP) diagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	jsonOut(w, h.service.DiagnosticBundle())
}
func (h *HTTP) commandApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	jsonOut(w, map[string]any{"approvals": h.service.PendingCommandApprovals()})
}
func (h *HTTP) commandApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	defer r.Body.Close()
	var body struct {
		ID       string `json:"id"`
		Approved *bool  `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" || body.Approved == nil {
		http.Error(w, "invalid approval request", http.StatusBadRequest)
		return
	}
	if !h.service.DecideCommandApproval(body.ID, *body.Approved) {
		http.Error(w, "approval request expired or no longer exists", http.StatusGone)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (h *HTTP) selectFileRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	path, err := folderpicker.Select(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOut(w, map[string]string{"path": path})
}

func (h *HTTP) selectTunnelClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	path, err := folderpicker.SelectExecutable(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	jsonOut(w, map[string]string{"path": path})
}
func (h *HTTP) config(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.service.mu.Lock()
		c := h.service.Config
		h.service.mu.Unlock()
		jsonOut(w, c)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "origin check failed", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	var body struct {
		Config struct {
			Name             string   `json:"name"`
			TunnelID         string   `json:"tunnel_id"`
			TunnelClient     string   `json:"tunnel_client"`
			FileRoots        []string `json:"file_roots"`
			AllowCommands    bool     `json:"allow_commands"`
			AllowFiles       bool     `json:"allow_files"`
			AllowDesktop     bool     `json:"allow_desktop"`
			AllowBrowser     bool     `json:"allow_browser"`
			AllowAdmin       bool     `json:"allow_admin"`
			StartOnLaunch    bool     `json:"start_on_launch"`
			ChatGPTGuideSeen bool     `json:"chatgpt_guide_seen"`
		} `json:"config"`
		RuntimeAPIKey string `json:"runtime_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if err := h.service.Update(ConfigFromRequest(body.Config), body.RuntimeAPIKey); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func ConfigFromRequest(c struct {
	Name             string   `json:"name"`
	TunnelID         string   `json:"tunnel_id"`
	TunnelClient     string   `json:"tunnel_client"`
	FileRoots        []string `json:"file_roots"`
	AllowCommands    bool     `json:"allow_commands"`
	AllowFiles       bool     `json:"allow_files"`
	AllowDesktop     bool     `json:"allow_desktop"`
	AllowBrowser     bool     `json:"allow_browser"`
	AllowAdmin       bool     `json:"allow_admin"`
	StartOnLaunch    bool     `json:"start_on_launch"`
	ChatGPTGuideSeen bool     `json:"chatgpt_guide_seen"`
}) config.Config {
	return config.Config{Name: c.Name, TunnelID: c.TunnelID, TunnelClient: c.TunnelClient, FileRoots: c.FileRoots, AllowCommands: c.AllowCommands, AllowFiles: c.AllowFiles, AllowDesktop: c.AllowDesktop, AllowBrowser: c.AllowBrowser, AllowAdmin: c.AllowAdmin, StartOnLaunch: c.StartOnLaunch, ChatGPTGuideSeen: c.ChatGPTGuideSeen}
}
func (h *HTTP) start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	if err := h.service.StartTunnel(); err != nil {
		h.service.Log(err.Error())
		http.Error(w, err.Error(), 400)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (h *HTTP) stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !sameOrigin(r) {
		http.Error(w, "forbidden", 403)
		return
	}
	h.service.StopTunnel()
	jsonOut(w, map[string]bool{"ok": true})
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if origin == "http://"+r.Host {
		return true
	}
	// The browser UI is served from a dedicated high port because some
	// browsers block the MCP client's conventional port 5060 as unsafe.
	// Mutations across the two loopback ports remain limited to these exact
	// origins; authentication still requires the per-launch UI session.
	return (r.Host == "127.0.0.1:5060" && origin == "http://127.0.0.1:18080") ||
		(r.Host == "127.0.0.1:18080" && origin == "http://127.0.0.1:5060")
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
