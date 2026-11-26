//go:build darwin

package desktop

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type native struct{}

func New() Adapter { return native{} }
func (native) Info() map[string]any {
	_, pointerErr := exec.LookPath("cliclick")
	return map[string]any{
		"os": "macOS", "desktop_supported": true, "pointer_helper_installed": pointerErr == nil,
		"screen_recording_permission": "需要在系统设置中授予；无法在调用前可靠预检",
		"accessibility_permission":    "键鼠与窗口工具需要辅助功能授权；首次调用时系统可能提示",
		"note":                        "截图需要屏幕录制权限；窗口和键鼠控制需要辅助功能权限。鼠标控制还需安装 cliclick。",
	}
}
func (native) Screenshot() ([]byte, error) {
	p, cleanup, err := screenshotPath()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if out, e := exec.Command("screencapture", "-x", p).CombinedOutput(); e != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = e.Error()
		}
		return nil, fmt.Errorf("macOS 无法截取屏幕：%s。请在“系统设置 > 隐私与安全性 > 屏幕录制”允许 Nexora，然后退出并重新打开 Nexora。", detail)
	}
	return readScreenshot(p)
}
func (native) Windows() ([]map[string]any, error) {
	out, e := exec.Command("osascript", "-e", `tell application "System Events" to get name of every process whose background only is false`).Output()
	if e != nil {
		return nil, e
	}
	var rows []map[string]any
	for i, n := range strings.Split(strings.TrimSpace(string(out)), ", ") {
		rows = append(rows, map[string]any{"id": i + 1, "title": n})
	}
	return rows, nil
}
func (native) Pointer(x, y, button int) error {
	_, e := exec.LookPath("cliclick")
	if e != nil {
		return errors.New("macOS 鼠标控制需要安装 cliclick 并授予辅助功能权限")
	}
	action := "m:" + itoa(x) + "," + itoa(y)
	if button != 0 {
		click := "c:"
		if button == 2 {
			click = "mc:"
		}
		if button == 3 {
			click = "rc:"
		}
		action = click + itoa(x) + "," + itoa(y)
	}
	return exec.Command("cliclick", action).Run()
}
func (native) Key(keys string) error {
	parts := strings.Split(keys, "+")
	modifiers := []string{}
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(part) {
		case "ctrl", "control":
			modifiers = append(modifiers, "control down")
		case "alt", "option":
			modifiers = append(modifiers, "option down")
		case "shift":
			modifiers = append(modifiers, "shift down")
		case "cmd", "command", "super", "meta":
			modifiers = append(modifiers, "command down")
		}
	}
	key := parts[len(parts)-1]
	codes := map[string]string{"return": "36", "enter": "36", "tab": "48", "space": "49", "delete": "51", "escape": "53", "esc": "53", "left": "123", "right": "124", "down": "125", "up": "126", "home": "115", "end": "119", "pageup": "116", "pagedown": "121"}
	using := ""
	if len(modifiers) > 0 {
		using = " using {" + strings.Join(modifiers, ", ") + "}"
	}
	if code, ok := codes[strings.ToLower(key)]; ok {
		return exec.Command("osascript", "-e", `tell application "System Events" to key code `+code+using).Run()
	}
	return exec.Command("osascript", "-e", `tell application "System Events" to keystroke `+quoteApple(key)+using).Run()
}
func (native) Type(text string) error {
	script := `tell application "System Events" to keystroke ` + quoteApple(text)
	return exec.Command("osascript", "-e", script).Run()
}
func quoteApple(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
func itoa(i int) string { return strconv.Itoa(i) }
