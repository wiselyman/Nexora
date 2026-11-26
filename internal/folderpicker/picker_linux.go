//go:build linux

package folderpicker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

func Select(ctx context.Context) (string, error) {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		out, runErr := exec.CommandContext(ctx, zenity, "--file-selection", "--directory", "--title=Choose a folder Nexora may access").Output()
		if runErr != nil {
			return "", errors.New("folder selection was cancelled or the Linux folder picker failed")
		}
		return selectedPath(out)
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		out, runErr := exec.CommandContext(ctx, kdialog, "--getexistingdirectory", os.Getenv("HOME"), "Choose a folder Nexora may access").Output()
		if runErr != nil {
			return "", errors.New("folder selection was cancelled or the Linux folder picker failed")
		}
		return selectedPath(out)
	}
	return "", errors.New("no native folder picker found; install zenity or kdialog, or enter the folder path manually")
}

func SelectExecutable(ctx context.Context) (string, error) {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		out, runErr := exec.CommandContext(ctx, zenity, "--file-selection", "--title=Select the official OpenAI tunnel-client executable").Output()
		if runErr != nil {
			return "", errors.New("file selection was cancelled or the Linux file picker failed")
		}
		return selectedPath(out)
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		out, runErr := exec.CommandContext(ctx, kdialog, "--getopenfilename", os.Getenv("HOME"), "*|Executable files", "Select the official OpenAI tunnel-client executable").Output()
		if runErr != nil {
			return "", errors.New("file selection was cancelled or the Linux file picker failed")
		}
		return selectedPath(out)
	}
	return "", errors.New("no native file picker found; install zenity or kdialog, or enter the executable path manually")
}

func selectedPath(out []byte) (string, error) {
	path := strings.TrimRight(string(out), "\r\n")
	if path == "" {
		return "", errors.New("folder selection cancelled")
	}
	return path, nil
}
