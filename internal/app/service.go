package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexora/internal/config"
	"nexora/internal/fileaccess"
	"nexora/internal/startup"
)

type Service struct {
	mu                  sync.Mutex
	updateMu            sync.Mutex
	Config              config.Config
	Secrets             config.Secrets
	Tunnel              *exec.Cmd
	tunnelDone          chan struct{}
	TunnelRunning       bool
	TunnelStarting      bool
	TunnelStopping      bool
	TunnelReady         bool
	TunnelClientVersion string
	LastSuccessfulCheck time.Time
	StartedAt           time.Time
	LastError           string
	Logs                []string
	commandApprovals    map[string]*commandApproval
	activeCommands      map[string]activeCommandExecution
}

type CommandApproval struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	CWD       string    `json:"cwd,omitempty"`
	Admin     bool      `json:"admin"`
	ExpiresAt time.Time `json:"expires_at"`
}

type commandApproval struct {
	CommandApproval
	decision chan commandApprovalDecision
}

type commandApprovalDecision struct {
	approved bool
	reason   string
}

type activeCommandExecution struct {
	admin  bool
	cancel context.CancelFunc
}

func New(c config.Config, s config.Secrets) *Service {
	return &Service{
		Config:           c,
		Secrets:          s,
		commandApprovals: make(map[string]*commandApproval),
		activeCommands:   make(map[string]activeCommandExecution),
	}
}

// RequestCommandApproval blocks until the local user approves the exact command,
// rejects it, cancels the MCP request, or lets the short approval window expire.
func (s *Service) RequestCommandApproval(ctx context.Context, command, cwd string, admin bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(command) == "" {
		return errors.New("command is required")
	}
	if len(command) > 16<<10 {
		return errors.New("command exceeds 16 KiB and cannot be shown for approval")
	}
	idBytes := make([]byte, 18)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("cannot create command approval request: %w", err)
	}
	request := &commandApproval{
		CommandApproval: CommandApproval{
			ID:        base64.RawURLEncoding.EncodeToString(idBytes),
			Command:   command,
			CWD:       cwd,
			Admin:     admin,
			ExpiresAt: time.Now().Add(90 * time.Second),
		},
		decision: make(chan commandApprovalDecision, 1),
	}

	s.mu.Lock()
	if s.commandApprovals == nil {
		s.commandApprovals = make(map[string]*commandApproval)
	}
	if len(s.commandApprovals) >= 3 {
		s.mu.Unlock()
		return errors.New("too many commands are waiting for local approval; try again shortly")
	}
	s.commandApprovals[request.ID] = request
	s.mu.Unlock()
	s.Log("等待本机用户确认一条命令")
	defer s.removeCommandApproval(request.ID, request)

	timer := time.NewTimer(time.Until(request.ExpiresAt))
	defer timer.Stop()
	select {
	case decision := <-request.decision:
		return commandApprovalResult(decision)
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		select {
		case decision := <-request.decision:
			return commandApprovalResult(decision)
		default:
		}
		return errors.New("local approval timed out after 90 seconds; command was not run")
	}
}

func (s *Service) PendingCommandApprovals() []CommandApproval {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]CommandApproval, 0, len(s.commandApprovals))
	for _, request := range s.commandApprovals {
		items = append(items, request.CommandApproval)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ExpiresAt.Before(items[j].ExpiresAt) })
	return items
}

func (s *Service) DecideCommandApproval(id string, approve bool) bool {
	s.mu.Lock()
	request := s.commandApprovals[id]
	if request == nil || time.Now().After(request.ExpiresAt) {
		s.mu.Unlock()
		return false
	}
	delete(s.commandApprovals, id)
	request.decision <- commandApprovalDecision{approved: approve}
	s.mu.Unlock()
	if approve {
		s.Log("本机用户批准了一条命令")
	} else {
		s.Log("本机用户拒绝了一条命令")
	}
	return true
}

func commandApprovalResult(decision commandApprovalDecision) error {
	if decision.approved {
		return nil
	}
	if decision.reason != "" {
		return errors.New(decision.reason)
	}
	return errors.New("command was denied by the local user")
}

// StartCommandWithCurrentPermission serializes the final command permission
// check and child-process creation with configuration updates. The execution
// remains registered until FinishCommandExecution so revoking its capability
// can cancel the process tree.
func (s *Service) StartCommandWithCurrentPermission(id string, admin bool, cancel context.CancelFunc, start func() error) (bool, error) {
	if id == "" || cancel == nil || start == nil {
		return false, errors.New("command execution registration is incomplete")
	}
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.mu.Lock()
	allowed := s.Config.AllowCommands && (!admin || s.Config.AllowAdmin)
	if allowed {
		if s.activeCommands == nil {
			s.activeCommands = make(map[string]activeCommandExecution)
		}
		s.activeCommands[id] = activeCommandExecution{admin: admin, cancel: cancel}
	}
	s.mu.Unlock()
	if !allowed {
		return false, nil
	}
	if err := start(); err != nil {
		s.FinishCommandExecution(id)
		return true, err
	}
	return true, nil
}

func (s *Service) FinishCommandExecution(id string) {
	s.mu.Lock()
	delete(s.activeCommands, id)
	s.mu.Unlock()
}

func (s *Service) cancelDisallowedCommandApprovalsLocked() int {
	cancelled := 0
	for id, request := range s.commandApprovals {
		if s.Config.AllowCommands && (!request.Admin || s.Config.AllowAdmin) {
			continue
		}
		delete(s.commandApprovals, id)
		reason := "command permissions were disabled while awaiting approval; command was not run"
		if request.Admin && s.Config.AllowCommands && !s.Config.AllowAdmin {
			reason = "administrator permission was disabled while awaiting approval; command was not run"
		}
		request.decision <- commandApprovalDecision{
			reason: reason,
		}
		cancelled++
	}
	for id, execution := range s.activeCommands {
		if s.Config.AllowCommands && (!execution.admin || s.Config.AllowAdmin) {
			continue
		}
		delete(s.activeCommands, id)
		execution.cancel()
		cancelled++
	}
	return cancelled
}

func (s *Service) removeCommandApproval(id string, expected *commandApproval) {
	s.mu.Lock()
	if s.commandApprovals[id] == expected {
		delete(s.commandApprovals, id)
	}
	s.mu.Unlock()
}

func (s *Service) SnapshotConfig() config.Config { s.mu.Lock(); defer s.mu.Unlock(); return s.Config }

func (s *Service) Status() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	connected := s.TunnelRunning
	hostname, _ := os.Hostname()
	return map[string]any{"name": "Nexora", "version": Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "hostname": hostname, "tunnel_id": s.Config.TunnelID, "runtime_key_configured": s.Secrets.RuntimeAPIKey != "", "tunnel_configured": s.Config.TunnelID != "" && s.Secrets.RuntimeAPIKey != "", "tunnel_running": connected, "tunnel_starting": s.TunnelStarting, "tunnel_ready": s.TunnelReady, "tunnel_client_version": s.TunnelClientVersion, "last_successful_check": s.LastSuccessfulCheck, "credential_store": s.Secrets.Storage, "started_at": s.StartedAt, "last_error": redactSecrets(s.LastError, s.Secrets.RuntimeAPIKey, s.Secrets.MCPToken), "local_mcp": "http://127.0.0.1:5060/mcp", "permissions": map[string]bool{"commands": s.Config.AllowCommands, "files": s.Config.AllowFiles, "desktop": s.Config.AllowDesktop, "browser": s.Config.AllowBrowser, "admin": s.Config.AllowAdmin}}
}

func (s *Service) DiagnosticBundle() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs := make([]string, len(s.Logs))
	for i, line := range s.Logs {
		logs[i] = redactSecrets(line, s.Secrets.RuntimeAPIKey, s.Secrets.MCPToken)
	}
	return map[string]any{
		"generated_at":       time.Now().UTC().Format(time.RFC3339),
		"app":                map[string]any{"version": Version, "os": runtime.GOOS, "arch": runtime.GOARCH},
		"connection":         map[string]any{"configured": s.Config.TunnelID != "" && s.Secrets.RuntimeAPIKey != "", "running": s.TunnelRunning, "ready": s.TunnelReady, "client_version": s.TunnelClientVersion, "last_successful_check": s.LastSuccessfulCheck, "last_error": redactSecrets(s.LastError, s.Secrets.RuntimeAPIKey, s.Secrets.MCPToken)},
		"permissions":        map[string]bool{"commands": s.Config.AllowCommands, "files": s.Config.AllowFiles, "desktop": s.Config.AllowDesktop, "browser": s.Config.AllowBrowser, "admin": s.Config.AllowAdmin},
		"credential_storage": s.Secrets.Storage,
		"recent_logs":        logs,
	}
}

func (s *Service) Log(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line = redactSecrets(line, s.Secrets.RuntimeAPIKey, s.Secrets.MCPToken)
	s.Logs = append(s.Logs, time.Now().Format("15:04:05")+"  "+line)
	if len(s.Logs) > 100 {
		s.Logs = s.Logs[len(s.Logs)-100:]
	}
}

func (s *Service) LogLines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.Logs...)
	return out
}

type lineLogWriter struct {
	mu                  sync.Mutex
	service             *Service
	secrets             []string
	pending             []byte
	redactingPrivateKey bool
}

var credentialAssignmentPattern = regexp.MustCompile(`(?i)(^|[^a-z0-9_.-])(["']?(?:[a-z0-9_.-]*(?:api[_-]?key|(?:access|refresh|bearer)?[_-]?token|authorization|auth[_-]?(?:header|token)|secret|password|passwd|credential)[a-z0-9_.-]*)["']?\s*[:=]\s*["']?)([^"'\s,;}&]+)`)
var bearerCredentialPattern = regexp.MustCompile(`(?i)\b(bearer\s+)[a-z0-9._~+/-]+=*`)
var commonTokenPattern = regexp.MustCompile(`\b(?:sk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{12,}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{12,})\b`)
var urlPasswordPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^/@:\s]+:)[^/@\s]+(@)`)
var privateKeyBeginPattern = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
var privateKeyEndPattern = regexp.MustCompile(`-----END [A-Z0-9 ]*PRIVATE KEY-----`)

func (w *lineLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		i := strings.IndexByte(string(w.pending), '\n')
		if i < 0 {
			break
		}
		w.writeLine(w.pending[:i])
		w.pending = w.pending[i+1:]
	}
	for len(w.pending) > 4096 {
		w.writeLine(w.pending[:4096])
		w.pending = w.pending[4096:]
	}
	return len(p), nil
}

func (w *lineLogWriter) writeLine(data []byte) {
	line := strings.TrimSpace(string(data))
	if w.redactingPrivateKey {
		if privateKeyEndPattern.MatchString(line) {
			w.redactingPrivateKey = false
		}
		return
	}
	if privateKeyBeginPattern.MatchString(line) {
		w.redactingPrivateKey = !privateKeyEndPattern.MatchString(line)
		w.service.Log("tunnel-client: [redacted private key block]")
		return
	}
	line = redactSecrets(line, w.secrets...)
	if len(line) > 1000 {
		line = line[:1000]
	}
	if line != "" {
		w.service.Log("tunnel-client: " + line)
	}
}

func redactSecrets(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[redacted]")
		}
	}
	value = bearerCredentialPattern.ReplaceAllString(value, `${1}[redacted]`)
	value = credentialAssignmentPattern.ReplaceAllString(value, `${1}[redacted]`)
	value = commonTokenPattern.ReplaceAllString(value, "[redacted]")
	value = urlPasswordPattern.ReplaceAllString(value, `${1}[redacted]${2}`)
	value = redactPEMPrivateKeys(value)
	return value
}

func redactPEMPrivateKeys(value string) string {
	starts := privateKeyBeginPattern.FindAllStringIndex(value, -1)
	for i := len(starts) - 1; i >= 0; i-- {
		end := privateKeyEndPattern.FindStringIndex(value[starts[i][1]:])
		if end == nil {
			value = value[:starts[i][1]] + "[redacted private key]"
			continue
		}
		end[0] += starts[i][1]
		end[1] += starts[i][1]
		value = value[:starts[i][1]] + "[redacted private key]" + value[end[1]:]
	}
	return value
}

func (w *lineLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.writeLine(w.pending)
		w.pending = nil
	}
}

func (s *Service) Update(c config.Config, apiKey string) error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	if c.Name != "Nexora" {
		return errors.New("MCP 名称固定为 Nexora")
	}
	if strings.TrimSpace(c.TunnelID) != "" && !config.ValidateTunnelID(strings.TrimSpace(c.TunnelID)) {
		return errors.New("Tunnel ID 格式无效")
	}
	c.TunnelID = strings.TrimSpace(c.TunnelID)
	c.PermissionPolicyVersion = config.CurrentPermissionPolicyVersion
	if strings.ContainsAny(c.TunnelClient, "\r\n\x00") {
		return errors.New("Tunnel 客户端路径无效")
	}
	roots, err := fileaccess.NormalizeRoots(c.FileRoots)
	if err != nil {
		return err
	}
	if c.AllowFiles && len(roots) == 0 {
		return errors.New("启用文件访问前，请至少选择一个允许访问的文件夹")
	}
	c.FileRoots = roots
	s.mu.Lock()
	previousConfig := s.Config
	previousSecrets := s.Secrets
	if (s.TunnelRunning || s.TunnelStarting) && (c.TunnelID != s.Config.TunnelID || c.TunnelClient != s.Config.TunnelClient || apiKey != "") {
		s.mu.Unlock()
		return errors.New("修改 Tunnel ID、客户端路径或运行密钥前，请先停止 Tunnel")
	}
	s.mu.Unlock()
	startupChanged := c.StartOnLaunch != previousConfig.StartOnLaunch
	if startupChanged {
		if err := startup.Apply(c.StartOnLaunch); err != nil {
			return fmt.Errorf("无法更新登录启动设置：%w", err)
		}
	}
	if err := c.Save(); err != nil {
		if startupChanged {
			if rollbackErr := startup.Apply(previousConfig.StartOnLaunch); rollbackErr != nil {
				return fmt.Errorf("保存配置失败：%w；恢复登录启动设置也失败：%v", err, rollbackErr)
			}
		}
		return err
	}
	secret := previousSecrets
	if apiKey != "" {
		secret.RuntimeAPIKey = apiKey
	}
	storage, err := config.SaveSecrets(secret)
	if err != nil {
		rollbackErrors := []error{}
		if _, rollbackErr := config.SaveSecrets(previousSecrets); rollbackErr != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复旧密钥失败: %w", rollbackErr))
		}
		if configErr := previousConfig.Save(); configErr != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复旧配置失败: %w", configErr))
		}
		if startupChanged {
			if startupErr := startup.Apply(previousConfig.StartOnLaunch); startupErr != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复登录启动设置失败: %w", startupErr))
			}
		}
		if rollbackErr := errors.Join(rollbackErrors...); rollbackErr != nil {
			return fmt.Errorf("保存密钥失败：%w；部分状态回滚失败：%v", err, rollbackErr)
		}
		return err
	}
	s.mu.Lock()
	s.Config = c
	s.Secrets = secret
	s.Secrets.Storage = storage
	cancelledApprovals := s.cancelDisallowedCommandApprovalsLocked()
	s.mu.Unlock()
	if cancelledApprovals > 0 {
		s.Log("权限设置已更改，已取消不再允许的待审批和运行中命令")
	}
	s.Log("配置已保存")
	return nil
}

// ClearRuntimeAPIKey removes the saved OpenAI Tunnel credential while retaining
// the independent local MCP token and all non-secret settings.
func (s *Service) ClearRuntimeAPIKey() error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.mu.Lock()
	previous := s.Secrets
	if s.TunnelRunning || s.TunnelStarting {
		s.mu.Unlock()
		return errors.New("请先停止 Tunnel 再清除运行时 API Key")
	}
	s.mu.Unlock()
	if previous.RuntimeAPIKey == "" {
		return nil
	}
	cleared := previous
	cleared.RuntimeAPIKey = ""
	storage, err := config.ClearRuntimeAPIKey(cleared)
	if err != nil {
		message := "无法确认本机运行时 API Key 已清除；现有密钥保持不变，请检查权限后重试"
		if strings.Contains(err.Error(), "credential store") {
			message = "系统凭据库拒绝删除运行时 API Key；请在系统凭据管理器中检查 5060 的密钥条目后重试"
		} else if strings.Contains(err.Error(), "fallback secrets") {
			message = "无法清理本机回退密钥文件；现有密钥保持不变，请检查配置目录权限后重试"
		}
		if _, rollbackErr := config.SaveSecrets(previous); rollbackErr != nil {
			return fmt.Errorf("%s：%v；恢复原密钥也失败：%v", message, err, rollbackErr)
		}
		return fmt.Errorf("%s：%w", message, err)
	}
	cleared.Storage = storage
	s.mu.Lock()
	s.Secrets = cleared
	s.mu.Unlock()
	s.Log("已清除本机保存的 Tunnel 运行时 API Key")
	return nil
}

func (s *Service) StartTunnel() error {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.mu.Lock()
	if s.TunnelRunning || s.TunnelStarting {
		s.mu.Unlock()
		return errors.New("Tunnel 客户端已经在运行")
	}
	s.TunnelStarting = true
	s.TunnelClientVersion = ""
	c, secret := s.Config, s.Secrets
	s.mu.Unlock()
	fail := func(err error) error {
		s.mu.Lock()
		s.TunnelStarting = false
		s.LastError = err.Error()
		s.mu.Unlock()
		return err
	}
	if c.TunnelID == "" || secret.RuntimeAPIKey == "" {
		return fail(errors.New("请先配置 Tunnel ID 和运行时 API Key"))
	}
	client, err := exec.LookPath(c.TunnelClient)
	if err != nil {
		return fail(fmt.Errorf("找不到 tunnel-client：%s。请先安装 OpenAI 官方客户端并填写其路径", c.TunnelClient))
	}
	clientVersion, err := inspectTunnelClient(client)
	if err != nil {
		return fail(err)
	}
	s.mu.Lock()
	s.TunnelClientVersion = clientVersion
	s.mu.Unlock()
	healthDir, err := os.MkdirTemp("", "5060-tunnel-health-")
	if err != nil {
		return fail(fmt.Errorf("无法创建 Tunnel 健康检查目录：%w", err))
	}
	healthURLFile := healthDir + string(os.PathSeparator) + "url"
	cmd := exec.Command(client, "run", "--log.format=struct-text", "--health.listen-addr=127.0.0.1:0", "--health.url-file="+healthURLFile)
	cmd.Env = tunnelClientEnv(os.Environ(), map[string]string{
		"CONTROL_PLANE_API_KEY":   secret.RuntimeAPIKey,
		"CONTROL_PLANE_TUNNEL_ID": c.TunnelID,
		"MCP_SERVER_URL":          "http://127.0.0.1:5060/mcp",
		"MCP_EXTRA_HEADERS":       "X-5060-Token: " + secret.MCPToken,
	})
	redactions := []string{secret.RuntimeAPIKey, secret.MCPToken}
	stdoutLog := &lineLogWriter{service: s, secrets: redactions}
	stderrLog := &lineLogWriter{service: s, secrets: redactions}
	cmd.Stdout, cmd.Stderr = stdoutLog, stderrLog
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(healthDir)
		return fail(err)
	}
	tunnelDone := make(chan struct{})
	s.mu.Lock()
	s.Tunnel = cmd
	s.tunnelDone = tunnelDone
	s.TunnelRunning = true
	s.TunnelStarting = false
	s.TunnelStopping = false
	s.TunnelReady = false
	s.StartedAt = time.Now()
	s.LastError = ""
	s.mu.Unlock()
	s.Log("Tunnel 客户端已启动")
	go func() {
		defer os.RemoveAll(healthDir)
		defer close(tunnelDone)
		healthDone := make(chan struct{})
		healthFinished := make(chan struct{})
		go func() {
			defer close(healthFinished)
			s.pollHealth(healthDone, healthURLFile)
		}()
		err := cmd.Wait()
		stdoutLog.Flush()
		stderrLog.Flush()
		close(healthDone)
		<-healthFinished
		s.finishTunnelProcess(err)
	}()
	return nil
}

func (s *Service) finishTunnelProcess(err error) {
	s.mu.Lock()
	s.Tunnel = nil
	s.TunnelRunning = false
	s.TunnelReady = false
	stoppedByUser := s.TunnelStopping
	s.TunnelStopping = false
	if stoppedByUser {
		s.LastError = ""
	} else if err != nil {
		s.LastError = err.Error()
	} else {
		s.LastError = "Tunnel 客户端意外退出"
	}
	s.mu.Unlock()

	if stoppedByUser {
		s.Log("Tunnel 客户端已停止")
	} else if err != nil {
		s.Log("Tunnel 客户端已退出: " + err.Error())
	} else {
		s.Log("Tunnel 客户端意外退出，退出代码为 0")
	}
}

func (s *Service) pollHealth(done <-chan struct{}, healthURLFile string) {
	client := tunnelHealthHTTPClient()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			endpoint := tunnelHealthEndpoint(healthURLFile)
			ready := false
			if endpoint != "" {
				resp, err := client.Get(endpoint)
				ready = err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
				if resp != nil {
					_ = resp.Body.Close()
				}
			}
			s.mu.Lock()
			select {
			case <-done:
				s.mu.Unlock()
				return
			default:
			}
			s.TunnelReady = ready
			if ready {
				s.LastSuccessfulCheck = time.Now()
			}
			s.mu.Unlock()
		}
	}
}

func tunnelHealthEndpoint(healthURLFile string) string {
	file, err := os.Open(healthURLFile)
	if err != nil {
		return ""
	}
	defer file.Close()
	const maxHealthURLFileSize = 4 << 10
	data, err := io.ReadAll(io.LimitReader(file, maxHealthURLFileSize+1))
	if err != nil || len(data) > maxHealthURLFileSize {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(string(data)))
	if err != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return ""
	}
	host, port, err := net.SplitHostPort(base.Host)
	if err != nil || host != "127.0.0.1" || port == "" {
		return ""
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return ""
	}
	return "http://" + net.JoinHostPort(host, port) + "/readyz"
}

func (s *Service) pollHealthAt(done <-chan struct{}, endpoint string, interval time.Duration) {
	client := tunnelHealthHTTPClient()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			resp, err := client.Get(endpoint)
			ready := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
			if resp != nil {
				_ = resp.Body.Close()
			}
			s.mu.Lock()
			select {
			case <-done:
				s.mu.Unlock()
				return
			default:
			}
			s.TunnelReady = ready
			if ready {
				s.LastSuccessfulCheck = time.Now()
			}
			s.mu.Unlock()
		}
	}
}

func tunnelHealthHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   1500 * time.Millisecond,
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func tunnelClientEnv(env []string, values map[string]string) []string {
	// Pass only operating-system basics and documented network settings. A full
	// os.Environ() can contain unrelated cloud, source-control, or app secrets.
	allowed := map[string]bool{
		"path": true, "home": true, "user": true, "username": true, "logname": true,
		"tmp": true, "temp": true, "tmpdir": true, "systemroot": true, "windir": true,
		"userprofile": true, "homedrive": true, "homepath": true, "appdata": true,
		"localappdata": true, "programdata": true, "lang": true,
		"http_proxy": true, "https_proxy": true, "no_proxy": true,
		"tunnel_client_http_proxy": true, "control_plane_http_proxy": true,
		"mcp_http_proxy": true, "harpoon_http_proxy": true,
		"proxy_check_interval": true, "ca_bundle": true,
		"enterprise_ca_bundle": true, "ssl_cert_file": true, "ssl_cert_dir": true,
		"control_plane_client_cert": true, "control_plane_client_key": true,
	}
	filtered := make([]string, 0, len(allowed)+len(values))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		lower := strings.ToLower(key)
		if !allowed[lower] && !strings.HasPrefix(lower, "lc_") {
			continue
		}
		if _, replace := values[key]; replace {
			continue
		}
		filtered = append(filtered, entry)
	}
	for key, value := range values {
		filtered = append(filtered, key+"="+value)
	}
	return filtered
}

func (s *Service) StopTunnel() {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	s.mu.Lock()
	cmd := s.Tunnel
	done := s.tunnelDone
	if cmd != nil {
		s.TunnelStopping = true
	}
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if done != nil {
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			s.Log("等待 Tunnel 客户端退出超时")
		}
	}
}
