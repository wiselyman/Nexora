//go:build windows

package desktop

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type native struct{}

func New() Adapter { return native{} }
func (native) Info() map[string]any {
	return map[string]any{"os": "Windows", "desktop_supported": true, "interactive_session_required": true, "note": "需要当前用户已登录且桌面已解锁；UAC 安全桌面和锁屏桌面不支持控制。"}
}
func (native) Screenshot() ([]byte, error) {
	p, cleanup, err := screenshotPath()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	script := `Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; $b=[System.Windows.Forms.SystemInformation]::VirtualScreen; $i=New-Object System.Drawing.Bitmap $b.Width,$b.Height; $g=[System.Drawing.Graphics]::FromImage($i); $g.CopyFromScreen($b.Location,[System.Drawing.Point]::Empty,$b.Size); $i.Save('` + strings.ReplaceAll(p, "'", "''") + `',[System.Drawing.Imaging.ImageFormat]::Png); $g.Dispose(); $i.Dispose()`
	if out, e := runPS(script); e != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			detail = e.Error()
		}
		return nil, fmt.Errorf("Windows 无法截取当前桌面：%s。请确认当前用户已登录且桌面已解锁；锁屏和 UAC 安全桌面无法截取。", detail)
	}
	return readScreenshot(p)
}
func (native) Windows() ([]map[string]any, error) {
	script := `Add-Type @'
using System; using System.Text; using System.Collections.Generic; using System.Runtime.InteropServices;
public class W { public delegate bool E(IntPtr h, IntPtr l); [DllImport("user32.dll")] public static extern bool EnumWindows(E cb, IntPtr l); [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr h); [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern int GetWindowText(IntPtr h, StringBuilder b, int n); public static List<string> Get(){var a=new List<string>();EnumWindows((h,l)=>{if(IsWindowVisible(h)){var b=new StringBuilder(1024);GetWindowText(h,b,b.Capacity);if(b.Length>0)a.Add(h.ToInt64().ToString("X")+"\t"+b.ToString());}return true;},IntPtr.Zero);return a;} }
'@; [W]::Get()`
	out, e := runPS(script)
	if e != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	rows := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		p := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(p) == 2 {
			rows = append(rows, map[string]any{"id": p[0], "title": p[1]})
		}
	}
	return rows, nil
}
func (native) Pointer(x, y, button int) error {
	if button < 0 || button > 3 {
		return errors.New("Windows adapter supports pointer buttons 0 through 3")
	}
	b := 0
	if button == 1 {
		b = 1
	} else if button == 2 {
		b = 2
	} else if button == 3 {
		b = 3
	}
	script := `Add-Type @'
using System.Runtime.InteropServices; public class I { [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y); [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint x,uint y,uint d,System.UIntPtr e); }
'@; [I]::SetCursorPos(` + strconv.Itoa(x) + `,` + strconv.Itoa(y) + `)|Out-Null; $b=` + strconv.Itoa(b) + `; if($b -gt 0){$down=@{1=2;2=32;3=8}[$b];$up=@{1=4;2=64;3=16}[$b];[I]::mouse_event($down,0,0,0,[UIntPtr]::Zero);[I]::mouse_event($up,0,0,0,[UIntPtr]::Zero)}`
	out, e := runPS(script)
	if e != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}
func (native) Key(keys string) error {
	ps := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait('` + strings.ReplaceAll(winSendKeys(keys), "'", "''") + `')`
	out, e := runPS(ps)
	if e != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}
func winSendKeys(keys string) string {
	parts := strings.Split(keys, "+")
	mods := ""
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(part) {
		case "ctrl", "control":
			mods += "^"
		case "alt":
			mods += "%"
		case "shift":
			mods += "+"
		case "super", "win", "cmd":
			mods += "#"
		}
	}
	key := parts[len(parts)-1]
	switch strings.ToLower(key) {
	case "return", "enter":
		key = "{ENTER}"
	case "esc", "escape":
		key = "{ESC}"
	case "tab":
		key = "{TAB}"
	case "space":
		key = " "
	case "backspace":
		key = "{BACKSPACE}"
	case "delete":
		key = "{DELETE}"
	case "up":
		key = "{UP}"
	case "down":
		key = "{DOWN}"
	case "left":
		key = "{LEFT}"
	case "right":
		key = "{RIGHT}"
	case "home":
		key = "{HOME}"
	case "end":
		key = "{END}"
	case "pageup":
		key = "{PGUP}"
	case "pagedown":
		key = "{PGDN}"
	}
	if strings.HasPrefix(strings.ToLower(key), "f") && len(key) <= 3 {
		if _, err := strconv.Atoi(key[1:]); err == nil {
			key = "{" + strings.ToUpper(key) + "}"
		}
	}
	return mods + key
}
func (native) Type(text string) error {
	ps := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetText('` + strings.ReplaceAll(text, "'", "''") + `'); [System.Windows.Forms.SendKeys]::SendWait('^v')`
	out, e := runPS(ps)
	if e != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}
func runPS(script string) ([]byte, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(utf16(script)))
	return exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded).CombinedOutput()
}
func utf16(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		if r <= 0xFFFF {
			out = append(out, byte(r), byte(r>>8))
		} else {
			r -= 0x10000
			hi := (r >> 10) + 0xD800
			lo := (r & 0x3FF) + 0xDC00
			out = append(out, byte(hi), byte(hi>>8), byte(lo), byte(lo>>8))
		}
	}
	return out
}
