//go:build linux

package desktop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type native struct{}

func New() Adapter { return native{} }
func (native) Info() map[string]any {
	session := os.Getenv("XDG_SESSION_TYPE")
	wayland := session == "wayland" || os.Getenv("WAYLAND_DISPLAY") != ""
	x11 := session == "x11" || (session == "" && os.Getenv("DISPLAY") != "")
	if session == "" {
		switch {
		case wayland:
			session = "wayland"
		case x11:
			session = "x11"
		default:
			session = "unknown"
		}
	}
	_, inputErr := exec.LookPath("xdotool")
	_, windowErr := exec.LookPath("wmctrl")
	_, gnomeShotErr := exec.LookPath("gnome-screenshot")
	_, scrotErr := exec.LookPath("scrot")
	_, importErr := exec.LookPath("import")
	screenshotAvailable := gnomeShotErr == nil || scrotErr == nil || importErr == nil
	note := "桌面控制主要支持 X11；截图和输入仍取决于已安装工具及当前桌面会话。"
	if wayland {
		note = "检测到 Wayland：Nexora 的键鼠控制和窗口枚举不完整；部分截图工具可能仍可用。"
	} else if !x11 {
		note = "未检测到 X11 图形会话；桌面截图和输入控制可能不可用。请在已登录的图形桌面中运行 Nexora。"
	}
	return map[string]any{
		"os": "Linux", "desktop_supported": x11, "session_type": session,
		"input_ready": x11 && inputErr == nil, "window_list_ready": x11 && windowErr == nil,
		"screenshot_tool_available": screenshotAvailable, "note": note,
	}
}
func (native) Screenshot() ([]byte, error) {
	p, cleanup, err := screenshotPath()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	candidates := [][]string{{"gnome-screenshot", "-f", p}, {"scrot", p}, {"import", "-window", "root", p}}
	var last error
	for _, a := range candidates {
		if _, e := exec.LookPath(a[0]); e != nil {
			last = e
			continue
		}
		if out, e := exec.Command(a[0], a[1:]...).CombinedOutput(); e == nil {
			return readScreenshot(p)
		} else {
			last = errors.New(strings.TrimSpace(string(out)))
		}
	}
	if last == nil {
		return nil, errors.New("未找到截图工具。请安装 gnome-screenshot、scrot 或 ImageMagick；Wayland 下还需确保桌面截图门户可用。")
	}
	return nil, fmt.Errorf("Linux 截图失败：%v。请确认已登录图形桌面会话；安装 gnome-screenshot、scrot 或 ImageMagick，并检查桌面环境的屏幕捕获权限。", last)
}
func (native) Windows() ([]map[string]any, error) {
	out, e := exec.Command("wmctrl", "-l").Output()
	if e != nil {
		return nil, errors.New("window listing requires wmctrl on X11")
	}
	rows := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 {
			rows = append(rows, map[string]any{"id": f[0], "title": strings.Join(f[3:], " ")})
		}
	}
	return rows, nil
}
func (native) Pointer(x, y, button int) error {
	args := []string{"mousemove", strconv.Itoa(x), strconv.Itoa(y)}
	if button > 0 {
		args = append(args, "click", strconv.Itoa(button))
	}
	return exec.Command("xdotool", args...).Run()
}
func (native) Key(keys string) error { return exec.Command("xdotool", "key", keys).Run() }
func (native) Type(text string) error {
	return exec.Command("xdotool", "type", "--clearmodifiers", "--delay", "1", text).Run()
}
