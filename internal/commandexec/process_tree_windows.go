//go:build windows

package commandexec

import (
	"os"
	"os/exec"
	"strconv"
)

func configureProcessTree(cmd *exec.Cmd) *exec.Cmd {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		kill := exec.Command("taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		if err := kill.Run(); err != nil {
			return os.ErrProcessDone
		}
		return nil
	}
	return cmd
}
