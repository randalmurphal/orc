//go:build !windows

package orchestrator

import (
	"os/exec"
	"syscall"
)

// setProcAttr sets process attributes for Unix systems.
// Enables process group creation so child processes can be killed together.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends a signal to the entire process group.
// On Unix, the process group ID equals the PID of the group leader.
// Negative PID signals the entire process group.
func killProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Kill the entire process group (negative PID)
	return syscall.Kill(-pid, syscall.SIGKILL)
}
