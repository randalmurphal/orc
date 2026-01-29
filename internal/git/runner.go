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

// MockRunner is a test double for CommandRunner.
// It allows configuring responses for specific commands.
type MockRunner struct {
	// Responses maps "command args..." to predefined responses.
	// Use "*" as a wildcard for any command.
	Responses map[string]MockResponse

	// Calls records all commands that were executed.
	Calls []MockCall

	// DefaultResponse is returned when no specific response is configured.
	DefaultResponse MockResponse
}

// MockResponse represents a mock command response.
type MockResponse struct {
	Stdout string
	Err    error
}

// MockCall records a single command invocation.
type MockCall struct {
	WorkDir string
	Command string
	Args    []string
}

// NewMockRunner creates a new MockRunner with empty responses.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses: make(map[string]MockResponse),
	}
}

// Run implements CommandRunner for MockRunner.
func (m *MockRunner) Run(workDir, name string, args ...string) (string, error) {
	m.Calls = append(m.Calls, MockCall{
		WorkDir: workDir,
		Command: name,
		Args:    args,
	})

	key := name + " " + strings.Join(args, " ")

	if resp, ok := m.Responses[key]; ok {
		return resp.Stdout, resp.Err
	}

	if resp, ok := m.Responses[name]; ok {
		return resp.Stdout, resp.Err
	}

	if resp, ok := m.Responses["*"]; ok {
		return resp.Stdout, resp.Err
	}

	return m.DefaultResponse.Stdout, m.DefaultResponse.Err
}

// OnCommand configures a response for a specific command.
// Example: runner.OnCommand("git", "status", "--short").Return("M file.go", nil)
func (m *MockRunner) OnCommand(name string, args ...string) *MockResponseBuilder {
	key := name + " " + strings.Join(args, " ")
	return &MockResponseBuilder{
		runner: m,
		key:    key,
	}
}

// OnAnyCommand configures a response for any command.
func (m *MockRunner) OnAnyCommand() *MockResponseBuilder {
	return &MockResponseBuilder{
		runner: m,
		key:    "*",
	}
}

// MockResponseBuilder builds mock responses.
type MockResponseBuilder struct {
	runner *MockRunner
	key    string
}

// Return sets the response for this command.
func (b *MockResponseBuilder) Return(stdout string, err error) *MockRunner {
	b.runner.Responses[b.key] = MockResponse{Stdout: stdout, Err: err}
	return b.runner
}

// WasCalled returns true if the command was called.
func (m *MockRunner) WasCalled(name string, args ...string) bool {
	for _, call := range m.Calls {
		if call.Command == name {
			if len(args) == 0 {
				return true
			}
			if argsMatch(call.Args, args) {
				return true
			}
		}
	}
	return false
}

// CallCount returns the number of times a command was called.
func (m *MockRunner) CallCount(name string) int {
	count := 0
	for _, call := range m.Calls {
		if call.Command == name {
			count++
		}
	}
	return count
}

// argsMatch checks if two argument slices match.
func argsMatch(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

// SequentialMockRunner is a mock runner that returns responses in order.
type SequentialMockRunner struct {
	queue []MockResponse
	index int
	Calls []MockCall
}

// NewSequentialMockRunner creates a new SequentialMockRunner.
func NewSequentialMockRunner() *SequentialMockRunner {
	return &SequentialMockRunner{}
}

// AddOutput adds a response to the queue.
func (m *SequentialMockRunner) AddOutput(stdout string, err error) *SequentialMockRunner {
	m.queue = append(m.queue, MockResponse{Stdout: stdout, Err: err})
	return m
}

// AddOutputError adds a response with custom error output.
func (m *SequentialMockRunner) AddOutputError(stdout, errOutput string, err error) *SequentialMockRunner {
	if err != nil {
		m.queue = append(m.queue, MockResponse{Stdout: errOutput, Err: err})
	} else {
		m.queue = append(m.queue, MockResponse{Stdout: stdout, Err: nil})
	}
	return m
}

// Run implements CommandRunner for SequentialMockRunner.
func (m *SequentialMockRunner) Run(workDir, name string, args ...string) (string, error) {
	m.Calls = append(m.Calls, MockCall{
		WorkDir: workDir,
		Command: name,
		Args:    args,
	})

	if m.index >= len(m.queue) {
		return "", nil
	}

	resp := m.queue[m.index]
	m.index++
	return resp.Stdout, resp.Err
}
