//go:build windows

package orchestrator

import "os/exec"

// setProcAttr is a no-op on Windows.
// Windows uses job objects instead of POSIX process groups.
// Context cancellation adequately handles process termination on Windows.
func setProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}

// killProcessGroup is a no-op on Windows.
// Windows process groups work differently and context cancellation
// handles the termination of the direct child process.
func killProcessGroup(pid int) error {
	// No-op on Windows
	return nil
}
