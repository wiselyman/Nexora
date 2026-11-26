package commandexec

import (
	"context"
	"encoding/base64"
	"os/exec"
	"runtime"
	"strings"
)

func Build(ctx context.Context, command, cwd string, admin bool) *exec.Cmd {
	var cmd *exec.Cmd
	if !admin {
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", command)
			cmd.Dir = cwd
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-lc", command)
			cmd.Dir = cwd
		}
	} else {
		switch runtime.GOOS {
		case "darwin":
			if cwd != "" {
				command = "cd " + shellQuote(cwd) + " && " + command
			}
			cmd = exec.CommandContext(ctx, "osascript", "-e", "do shell script "+appleQuote(command)+" with administrator privileges")
		case "linux":
			if cwd != "" {
				command = "cd " + shellQuote(cwd) + " && " + command
			}
			cmd = exec.CommandContext(ctx, "pkexec", "/bin/sh", "-lc", command)
		case "windows":
			cmd = windowsElevated(ctx, command, cwd)
		default:
			cmd = exec.CommandContext(ctx, "", "")
		}
	}
	return configureProcessTree(cmd)
}

func windowsElevated(ctx context.Context, command, cwd string) *exec.Cmd {
	child := command
	if cwd != "" {
		child = "Set-Location -LiteralPath " + psQuote(cwd) + "; " + child
	}
	encodedChild := encodePowerShell(child)
	parent := `$o=[IO.Path]::GetTempFileName();$e=[IO.Path]::GetTempFileName();try{$p=Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -PassThru -ArgumentList @('-NoLogo','-NoProfile','-NonInteractive','-EncodedCommand','` + encodedChild + `') -RedirectStandardOutput $o -RedirectStandardError $e;[Console]::Out.Write([IO.File]::ReadAllText($o));[Console]::Error.Write([IO.File]::ReadAllText($e));exit $p.ExitCode}finally{Remove-Item -LiteralPath $o,$e -Force -ErrorAction SilentlyContinue}`
	return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(parent))
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

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func appleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'"+"\\"+"''") + "'" }
