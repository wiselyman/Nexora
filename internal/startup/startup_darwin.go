//go:build darwin

package startup

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func entryPaths() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com.nexora.agent.plist"), filepath.Join(config, "Nexora"), nil
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
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logs := filepath.Join(home, "Library", "Logs")
	if err := os.MkdirAll(logs, 0700); err != nil {
		return err
	}
	escape := func(s string) string {
		var b strings.Builder
		if err := xml.EscapeText(&b, []byte(s)); err != nil {
			return ""
		}
		return b.String()
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.Nexora.agent</string>
<key>ProgramArguments</key><array><string>%s</string></array>
<key>RunAtLoad</key><true/>
<key>StandardOutPath</key><string>%s</string>
<key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, escape(exe), escape(filepath.Join(logs, "Nexora.log")), escape(filepath.Join(logs, "Nexora-error.log")))
	return atomicWrite(path, []byte(content), 0600)
}
