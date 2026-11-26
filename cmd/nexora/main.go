package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"nexora/internal/app"
	"nexora/internal/config"
	"nexora/internal/desktop"
	"nexora/internal/mcp"
	"nexora/internal/startup"
	"nexora/internal/ui"
)

func main() {
	noOpen := flag.Bool("no-open", false, "do not open the configuration UI in a browser")
	showVersion := flag.Bool("version", false, "print the Nexora version and exit")
	purgeUserData := flag.Bool("purge-user-data", false, "remove Nexora credentials and settings")
	flag.Parse()
	if *showVersion {
		fmt.Println(app.Version)
		return
	}
	if *purgeUserData {
		lock, err := net.Listen("tcp", "127.0.0.1:5060")
		if err != nil {
			log.Fatalf("请先退出正在运行的 Nexora，再清理本机数据：%v", err)
		}
		defer lock.Close()
		if err := config.PurgeUserData(); err != nil {
			log.Fatalf("无法完整清理 Nexora 本机数据：%v", err)
		}
		fmt.Println("已删除 Nexora 系统凭据和本机配置数据。")
		return
	}

	// Reserve the compatibility MCP endpoint before reading or changing user data.
	addr := "127.0.0.1:5060"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			log.Fatalf("Nexora 无法启动：本机 MCP 端口 %s 已被占用。若 Nexora 已在运行，请使用启动终端输出的设置链接；否则关闭占用端口的程序后重试。详情：%v", addr, err)
		}
		log.Fatalf("Nexora 无法监听本机 MCP 地址 %s：%v", addr, err)
	}
	uiListeners := make([]net.Listener, 0, 2)
	var uiListenErrs []error
	for _, uiAddr := range []string{"127.0.0.1:18080", "[::1]:18080"} {
		uiListener, listenErr := net.Listen("tcp", uiAddr)
		if listenErr != nil {
			uiListenErrs = append(uiListenErrs, fmt.Errorf("%s: %w", uiAddr, listenErr))
			continue
		}
		uiListeners = append(uiListeners, uiListener)
	}
	if len(uiListeners) == 0 {
		_ = listener.Close()
		log.Fatalf("Nexora 无法启动网页设置服务：本机端口 18080 的回环地址均不可用：%v", errors.Join(uiListenErrs...))
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		log.Fatal(err)
	}
	secrets, err := config.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}
	if secrets.MCPToken == "" {
		secrets.MCPToken, err = config.NewMCPToken()
		if err != nil {
			log.Fatal(err)
		}
		secrets.Storage, err = config.SaveSecrets(secrets)
		if err != nil {
			log.Fatal(err)
		}
	}
	uiToken, err := config.NewMCPToken()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	service := app.New(cfg, secrets)
	if err := startup.Apply(cfg.StartOnLaunch); err != nil {
		log.Printf("登录启动设置未能同步：%v", err)
		service.Log("登录启动设置未能同步：" + err.Error())
	}
	mux := http.NewServeMux()
	api := app.NewHTTP(service, ui.FS, uiToken, stop)
	api.Register(mux)
	mcpServer := mcp.NewServer(service, desktop.New(), secrets.MCPToken)
	mux.Handle("/mcp", mcpServer)

	server := &http.Server{Handler: loopbackOnly(mux), ReadHeaderTimeout: 5 * time.Second}
	uiMux := http.NewServeMux()
	blockMCP := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
	uiMux.HandleFunc("/mcp", blockMCP)
	uiMux.HandleFunc("/mcp/", blockMCP)
	uiMux.Handle("/", mux)
	uiServer := &http.Server{Handler: loopbackOnlyForPorts(uiMux, 18080), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Nexora local service ready (MCP compatibility endpoint: http://%s/mcp)", addr)
	uiURL := configurationURL()
	if !*noOpen {
		if err := openBrowser(uiURL); err != nil {
			fmt.Printf("Nexora 配置页面（本次运行有效；无桌面环境时可通过 SSH 本地端口转发访问）：%s\n", uiURL)
			log.Printf("未能自动打开浏览器：%v", err)
		}
	} else {
		fmt.Printf("Nexora 配置页面（本次运行有效）：%s\n", uiURL)
	}

	shutdownComplete := make(chan struct{})
	go func() {
		defer close(shutdownComplete)
		<-ctx.Done()
		jobsCtx, jobsCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if !mcpServer.StopJobs(jobsCtx) {
			log.Printf("命令任务未能在关闭时限内结束；仍有命令或子进程可能存活")
		}
		jobsCancel()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
		_ = uiServer.Shutdown(shutdown)
		service.StopTunnel()
	}()
	serveErrCh := make(chan error, 1+len(uiListeners))
	go func() { serveErrCh <- server.Serve(listener) }()
	for _, uiListener := range uiListeners {
		go func(listener net.Listener) { serveErrCh <- uiServer.Serve(listener) }(uiListener)
	}
	serveErrDone := make(chan struct{})
	var serviceFailure error
	go func() {
		defer close(serveErrDone)
		for range 1 + len(uiListeners) {
			serveErr := <-serveErrCh
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				serviceFailure = serveErr
				log.Printf("Nexora 本机服务异常退出，正在清理命令和 Tunnel：%v", serveErr)
				stop()
			}
		}
	}()
	<-ctx.Done()
	<-shutdownComplete
	<-serveErrDone
	if serviceFailure != nil {
		log.Fatalf("Nexora 已完成关闭清理，因本机服务错误退出：%v", serviceFailure)
	}
}

func openBrowser(url string) error {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("当前系统没有图形桌面会话")
	}
	if runtime.GOOS == "darwin" && os.Getenv("SSH_CONNECTION") != "" {
		return fmt.Errorf("当前通过 SSH 运行")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func configurationURL() string {
	return "http://localhost:18080"
}

func loopbackOnly(next http.Handler) http.Handler {
	return loopbackOnlyForPorts(next, 5060)
}

func loopbackOnlyForPorts(next http.Handler, ports ...int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The Host header is attacker-controlled during DNS rebinding. Restrict it
		// to the local origins used by the UI and tunnel client before routing.
		if !allowedLocalHostPorts(r.Host, ports...) {
			http.Error(w, "local host names only", http.StatusForbidden)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid local connection", http.StatusForbidden)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.Error(w, "local connections only", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func allowedLocalHost(host string) bool {
	return allowedLocalHostPorts(host, 5060)
}

func allowedLocalHostPorts(host string, ports ...int) bool {
	name, port, err := net.SplitHostPort(strings.ToLower(host))
	if err != nil || (name != "127.0.0.1" && name != "localhost" && name != "::1") {
		return false
	}
	for _, allowed := range ports {
		if port == fmt.Sprint(allowed) {
			return true
		}
	}
	return false
}
