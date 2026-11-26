//go:build darwin

package folderpicker

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

func Select(ctx context.Context) (string, error) {
	script := `try
set selectedFolder to choose folder with prompt "Choose a folder Nexora may access"
POSIX path of selectedFolder
on error number -128
""
end try`
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		return "", errors.New("could not open the macOS folder picker; check that the app has access to the desktop")
	}
	path := strings.TrimRight(string(out), "\r\n")
	if path == "" {
		return "", errors.New("folder selection cancelled")
	}
	return path, nil
}

func SelectExecutable(ctx context.Context) (string, error) {
	script := `try
set selectedFile to choose file with prompt "Choose the official OpenAI tunnel-client executable"
POSIX path of selectedFile
on error number -128
""
end try`
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).Output()
	if err != nil {
		return "", errors.New("could not open the macOS file picker; check that the app has access to the desktop")
	}
	path := strings.TrimRight(string(out), "\r\n")
	if path == "" {
		return "", errors.New("file selection cancelled")
	}
	return path, nil
}
