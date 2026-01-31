package git

import (
	"bytes"
	"os/exec"
	"strings"
)

// CommandRunner executes shell commands.
// This interface allows mocking command execution in tests.
type CommandRunner interface {
	// Run executes a command and returns the trimmed stdout.
	// workDir is the working directory for the command.
	// If the command fails, it returns the stderr/stdout as the error message.
	Run(workDir string, name string, args ...string) (stdout string, err error)
}

// ExecRunner is the default CommandRunner using exec.Command.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

// Run executes the command using exec.Command.
func (r *ExecRunner) Run(workDir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		if errMsg == "" {
			errMsg = err.Error()
		}
		return errMsg, &CommandError{
			Command: name,
			Args:    args,
			WorkDir: workDir,
			Output:  errMsg,
			Err:     err,
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CommandError represents a command execution error.
type CommandError struct {
	Command string
	Args    []string
	WorkDir string
	Output  string
	Err     error
}

func (e *CommandError) Error() string {
	if e.Output != "" {
		return e.Output
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "command failed"
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

