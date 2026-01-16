//go:build windows

package orchestrator

import "os/exec"

// setProcAttr is a no-op on Windows.
//
// Windows uses job objects instead of POSIX process groups for managing
// process hierarchies. Full implementation would require:
// 1. Creating a job object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// 2. Assigning the child process to the job object
//
// TODO: Implement Windows job objects for proper child process cleanup.
// Until then, child processes spawned by Claude (MCP servers, Playwright,
// chromium, etc.) may become orphaned on worker shutdown on Windows.
func setProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}

// killProcessGroup is a no-op on Windows.
//
// On Windows, proper process group cleanup requires job objects.
// Context cancellation only terminates the direct child process, not its
// descendants. This is a known limitation on Windows platforms.
func killProcessGroup(pid int) error {
	// No-op on Windows
	return nil
}
