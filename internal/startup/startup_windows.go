//go:build windows

package startup

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func entryPaths() (string, string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", "", os.ErrNotExist
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Nexora.lnk"), filepath.Join(config, "Nexora"), nil
}

func writeEntry(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
		exe = resolved
	}
	script := shortcutScript(path, exe)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(script))
	if out, err := cmd.CombinedOutput(); err != nil {
		return &execError{err: err, output: string(out)}
	}
	return nil
}

func shortcutScript(path, exe string) string {
	workingDir := filepath.Dir(exe)
	return "$w=New-Object -ComObject WScript.Shell;$s=$w.CreateShortcut(" + psQuote(path) + ");$s.TargetPath=" + psQuote(exe) + ";$s.WorkingDirectory=" + psQuote(workingDir) + ";$s.IconLocation=" + psQuote(exe+",0") + ";$s.WindowStyle=7;$s.Save()"
}

type execError struct {
	err    error
	output string
}

func (e *execError) Error() string {
	return "cannot create Windows startup shortcut: " + e.output + ": " + e.err.Error()
}
func (e *execError) Unwrap() error { return e.err }

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func encodePowerShell(s string) string {
	words := make([]uint16, 0, len(s))
	for _, r := range s {
		if r <= 0xFFFF {
			words = append(words, uint16(r))
		} else {
			r -= 0x10000
			words = append(words, uint16((r>>10)+0xD800), uint16((r&0x3FF)+0xDC00))
		}
	}
	b := make([]byte, len(words)*2)
	for i, v := range words {
		b[i*2], b[i*2+1] = byte(v), byte(v>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
