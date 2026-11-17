//go:build darwin || linux

package login

import (
	"os/exec"
	"syscall"
)

func runtimeSupported() bool { return true }

func configureRuntimeProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func killRuntimeProcess(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	// The authorization worker and all of its descendants share this process group.
	_ = syscall.Kill(-cmd.Process.Pid, signal)
}
