// Package errors provides structured error types for orc.
package errors

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Code represents a unique error code.
type Code string

// Error codes for orc.
const (
	// Initialization errors
	CodeNotInitialized     Code = "ORC_NOT_INITIALIZED"
	CodeAlreadyInitialized Code = "ORC_ALREADY_INITIALIZED"

	// Task errors
	CodeTaskNotFound     Code = "TASK_NOT_FOUND"
	CodeTaskInvalidState Code = "TASK_INVALID_STATE"
	CodeTaskRunning      Code = "TASK_RUNNING"

	// Claude errors
	CodeClaudeUnavailable Code = "CLAUDE_UNAVAILABLE"
	CodeClaudeTimeout     Code = "CLAUDE_TIMEOUT"
	CodePhaseStuck        Code = "PHASE_STUCK"
	CodeMaxRetries        Code = "MAX_RETRIES_EXCEEDED"

	// Config errors
	CodeConfigInvalid Code = "CONFIG_INVALID"
	CodeConfigMissing Code = "CONFIG_MISSING"

	// Git errors
	CodeGitDirty        Code = "GIT_DIRTY"
	CodeGitBranchExists Code = "GIT_BRANCH_EXISTS"
)

// Category groups error codes for HTTP status mapping.
type Category int

const (
	CategoryUnknown Category = iota
	CategoryNotFound
	CategoryBadRequest
	CategoryConflict
	CategoryInternal
	CategoryTimeout
	CategoryUnavailable
)

// codeCategories maps error codes to their categories.
var codeCategories = map[Code]Category{
	CodeNotInitialized:     CategoryBadRequest,
	CodeAlreadyInitialized: CategoryConflict,
	CodeTaskNotFound:       CategoryNotFound,
	CodeTaskInvalidState:   CategoryBadRequest,
	CodeTaskRunning:        CategoryConflict,
	CodeClaudeUnavailable:  CategoryUnavailable,
	CodeClaudeTimeout:      CategoryTimeout,
	CodePhaseStuck:         CategoryInternal,
	CodeMaxRetries:         CategoryInternal,
	CodeConfigInvalid:      CategoryBadRequest,
	CodeConfigMissing:      CategoryBadRequest,
	CodeGitDirty:           CategoryBadRequest,
	CodeGitBranchExists:    CategoryConflict,
}

// HTTPStatus returns the HTTP status code for a category.
func (c Category) HTTPStatus() int {
	switch c {
	case CategoryNotFound:
		return 404
	case CategoryBadRequest:
		return 400
	case CategoryConflict:
		return 409
	case CategoryTimeout:
		return 504
	case CategoryUnavailable:
		return 503
	default:
		return 500
	}
}

// OrcError is the structured error type for orc.
type OrcError struct {
	Code    Code   `json:"code"`
	What    string `json:"what"`
	Why     string `json:"why,omitempty"`
	Fix     string `json:"fix,omitempty"`
	DocsURL string `json:"docs_url,omitempty"`
	Cause   error  `json:"-"`
}

// Error implements the error interface.
func (e *OrcError) Error() string {
	var b strings.Builder
	b.WriteString(e.What)
	if e.Why != "" {
		b.WriteString(": ")
		b.WriteString(e.Why)
	}
	if e.Cause != nil {
		b.WriteString(": ")
		b.WriteString(e.Cause.Error())
	}
	return b.String()
}

// Unwrap returns the underlying cause.
func (e *OrcError) Unwrap() error {
	return e.Cause
}

// UserMessage returns a user-friendly message for CLI output.
func (e *OrcError) UserMessage() string {
	var b strings.Builder
	b.WriteString("Error: ")
	b.WriteString(e.What)
	if e.Why != "" {
		b.WriteString("\n\nWhy: ")
		b.WriteString(e.Why)
	}
	if e.Fix != "" {
		b.WriteString("\n\nFix: ")
		b.WriteString(e.Fix)
	}
	if e.DocsURL != "" {
		b.WriteString("\n\nDocs: ")
		b.WriteString(e.DocsURL)
	}
	return b.String()
}

// Category returns the error category for HTTP status mapping.
func (e *OrcError) Category() Category {
	if cat, ok := codeCategories[e.Code]; ok {
		return cat
	}
	return CategoryUnknown
}

// HTTPStatus returns the appropriate HTTP status code for this error.
func (e *OrcError) HTTPStatus() int {
	return e.Category().HTTPStatus()
}

// MarshalJSON implements json.Marshaler.
func (e *OrcError) MarshalJSON() ([]byte, error) {
	type alias OrcError
	aux := struct {
		*alias
		CauseMsg string `json:"cause,omitempty"`
	}{
		alias: (*alias)(e),
	}
	if e.Cause != nil {
		aux.CauseMsg = e.Cause.Error()
	}
	return json.Marshal(aux)
}

// Is reports whether target is an OrcError with the same code.
func (e *OrcError) Is(target error) bool {
	t, ok := target.(*OrcError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithCause returns a copy of the error with the given cause.
func (e *OrcError) WithCause(err error) *OrcError {
	return &OrcError{
		Code:    e.Code,
		What:    e.What,
		Why:     e.Why,
		Fix:     e.Fix,
		DocsURL: e.DocsURL,
		Cause:   err,
	}
}

// --- Error constructors ---

// ErrNotInitialized returns an error for uninitialized orc directory.
func ErrNotInitialized() *OrcError {
	return &OrcError{
		Code:    CodeNotInitialized,
		What:    "orc is not initialized in this directory",
		Why:     "No .orc/ directory found in the current path or its parents",
		Fix:     "Run 'orc init' to initialize orc in this directory",
		DocsURL: "https://github.com/randalmurphal/orc#quick-start",
	}
}

// ErrAlreadyInitialized returns an error when orc is already initialized.
func ErrAlreadyInitialized(path string) *OrcError {
	return &OrcError{
		Code:    CodeAlreadyInitialized,
		What:    "orc is already initialized",
		Why:     fmt.Sprintf("Found existing .orc/ directory at %s", path),
		Fix:     "Use 'orc init --force' to reinitialize, or remove .orc/ manually",
		DocsURL: "https://github.com/randalmurphal/orc#initialization",
	}
}

// ErrTaskNotFound returns an error when a task doesn't exist.
func ErrTaskNotFound(id string) *OrcError {
	return &OrcError{
		Code:    CodeTaskNotFound,
		What:    fmt.Sprintf("task %s not found", id),
		Why:     "No task with this ID exists in the current project",
		Fix:     "Run 'orc status' to list available tasks, or create one with 'orc new'",
		DocsURL: "https://github.com/randalmurphal/orc#tasks",
	}
}

// ErrTaskInvalidState returns an error when a task is in an invalid state.
func ErrTaskInvalidState(id, current, expected string) *OrcError {
	return &OrcError{
		Code:    CodeTaskInvalidState,
		What:    fmt.Sprintf("task %s is in state '%s', expected '%s'", id, current, expected),
		Why:     "The requested operation cannot be performed in the current task state",
		Fix:     fmt.Sprintf("Task must be in '%s' state. Check 'orc status %s' for current state", expected, id),
		DocsURL: "https://github.com/randalmurphal/orc#task-states",
	}
}

// ErrTaskRunning returns an error when a task is already running.
func ErrTaskRunning(id string) *OrcError {
	return &OrcError{
		Code:    CodeTaskRunning,
		What:    fmt.Sprintf("task %s is already running", id),
		Why:     "Cannot start a task that is already in progress",
		Fix:     fmt.Sprintf("Use 'orc pause %s' to pause, or wait for completion", id),
		DocsURL: "https://github.com/randalmurphal/orc#task-states",
	}
}

// ErrClaudeUnavailable returns an error when Claude CLI is not accessible.
func ErrClaudeUnavailable() *OrcError {
	return &OrcError{
		Code:    CodeClaudeUnavailable,
		What:    "Claude CLI is not available",
		Why:     "Could not find or execute the 'claude' command",
		Fix:     "Install Claude Code CLI: https://claude.ai/claude-code",
		DocsURL: "https://github.com/randalmurphal/orc#requirements",
	}
}

// ErrClaudeTimeout returns an error when Claude times out.
func ErrClaudeTimeout(phase string, duration string) *OrcError {
	return &OrcError{
		Code:    CodeClaudeTimeout,
		What:    fmt.Sprintf("Claude timed out during %s phase", phase),
		Why:     fmt.Sprintf("No response received after %s", duration),
		Fix:     "Increase timeout in config, or check Claude's status. Resume with 'orc resume'",
		DocsURL: "https://github.com/randalmurphal/orc#timeouts",
	}
}

// ErrPhaseStuck returns an error when a phase is stuck.
func ErrPhaseStuck(phase, reason string) *OrcError {
	return &OrcError{
		Code:    CodePhaseStuck,
		What:    fmt.Sprintf("phase %s is stuck", phase),
		Why:     reason,
		Fix:     "Review the phase transcript with 'orc logs', then either fix and resume or rewind",
		DocsURL: "https://github.com/randalmurphal/orc#troubleshooting",
	}
}

// ErrMaxRetries returns an error when max retries are exceeded.
func ErrMaxRetries(phase string, attempts int) *OrcError {
	return &OrcError{
		Code:    CodeMaxRetries,
		What:    fmt.Sprintf("phase %s failed after %d attempts", phase, attempts),
		Why:     "Maximum retry attempts exceeded without successful completion",
		Fix:     "Review transcripts with 'orc logs', fix issues manually, then rewind and retry",
		DocsURL: "https://github.com/randalmurphal/orc#retries",
	}
}

// ErrConfigInvalid returns an error for invalid configuration.
func ErrConfigInvalid(field, reason string) *OrcError {
	return &OrcError{
		Code:    CodeConfigInvalid,
		What:    fmt.Sprintf("invalid configuration: %s", field),
		Why:     reason,
		Fix:     "Check .orc/config.yaml and fix the invalid field",
		DocsURL: "https://github.com/randalmurphal/orc#configuration",
	}
}

// ErrConfigMissing returns an error for missing configuration.
func ErrConfigMissing(field string) *OrcError {
	return &OrcError{
		Code:    CodeConfigMissing,
		What:    fmt.Sprintf("missing required configuration: %s", field),
		Why:     "This field is required but not set in configuration",
		Fix:     fmt.Sprintf("Add '%s' to .orc/config.yaml", field),
		DocsURL: "https://github.com/randalmurphal/orc#configuration",
	}
}

// ErrGitDirty returns an error when working directory has uncommitted changes.
func ErrGitDirty() *OrcError {
	return &OrcError{
		Code:    CodeGitDirty,
		What:    "working directory has uncommitted changes",
		Why:     "Cannot start task execution with uncommitted changes",
		Fix:     "Commit or stash your changes before running a task",
		DocsURL: "https://github.com/randalmurphal/orc#git-integration",
	}
}

// ErrGitBranchExists returns an error when branch already exists.
func ErrGitBranchExists(branch string) *OrcError {
	return &OrcError{
		Code:    CodeGitBranchExists,
		What:    fmt.Sprintf("branch '%s' already exists", branch),
		Why:     "Cannot create task branch because it already exists",
		Fix:     fmt.Sprintf("Delete the existing branch with 'git branch -d %s' or use a different task name", branch),
		DocsURL: "https://github.com/randalmurphal/orc#git-integration",
	}
}

// AsOrcError attempts to convert an error to an OrcError.
// Returns nil if the error is not an OrcError.
func AsOrcError(err error) *OrcError {
	var orcErr *OrcError
	if As(err, &orcErr) {
		return orcErr
	}
	return nil
}

// As is a convenience wrapper for errors.As.
func As(err error, target any) bool {
	return asError(err, target)
}

// asError implements errors.As behavior.
func asError(err error, target any) bool {
	if err == nil {
		return false
	}
	if orcErr, ok := err.(*OrcError); ok {
		if t, ok := target.(**OrcError); ok {
			*t = orcErr
			return true
		}
	}
	// Check unwrapped error
	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		return asError(unwrapper.Unwrap(), target)
	}
	return false
}

// Wrap wraps a generic error into an OrcError with unknown code.
func Wrap(err error, what string) *OrcError {
	return &OrcError{
		Code:  Code("UNKNOWN"),
		What:  what,
		Cause: err,
	}
}
