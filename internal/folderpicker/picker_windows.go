//go:build windows

package folderpicker

import (
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
)

func Select(ctx context.Context) (string, error) {
	script := `$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description='Choose a folder Nexora may access'; if($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){[Console]::Out.Write([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($d.SelectedPath))); exit 0}; exit 2`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-STA", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("folder selection was cancelled or the Windows folder picker failed")
	}
	encoded := strings.TrimRight(string(out), "\r\n")
	decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
	if decodeErr != nil {
		return "", errors.New("could not read the selected Windows folder path")
	}
	path := string(decoded)
	if path == "" {
		return "", errors.New("folder selection cancelled")
	}
	return path, nil
}

func SelectExecutable(ctx context.Context) (string, error) {
	script := `$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.OpenFileDialog; $d.Title='Select the official OpenAI tunnel-client executable'; $d.Filter='Executable files (*.exe)|*.exe|All files (*.*)|*.*'; $d.CheckFileExists=$true; if($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){[Console]::Out.Write([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($d.FileName))); exit 0}; exit 2`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-STA", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("file selection was cancelled or the Windows file picker failed")
	}
	encoded := strings.TrimRight(string(out), "\r\n")
	decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
	if decodeErr != nil || len(decoded) == 0 {
		return "", errors.New("could not read the selected Windows executable path")
	}
	return string(decoded), nil
}
