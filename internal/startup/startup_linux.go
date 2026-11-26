//go:build linux

package startup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func entryPaths() (string, string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(config, "autostart", "Nexora.desktop"), filepath.Join(config, "Nexora"), nil
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
	content := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Nexora\nComment=Start Nexora\nExec=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", desktopQuote(exe))
	return atomicWrite(path, []byte(content), 0600)
}

func desktopQuote(s string) string {
	// First encode the Exec argument syntax, then encode the resulting string
	// value. Desktop Entry parsing applies string escapes before Exec quoting.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + s + `"`
}
