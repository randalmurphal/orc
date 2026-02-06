package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/variable"
)

// ScriptPhaseConfig holds configuration for a script phase.
type ScriptPhaseConfig struct {
	Command        string        `json:"command"`
	Workdir        string        `json:"workdir,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
	SuccessPattern string        `json:"success_pattern,omitempty"`
	OutputVar      string        `json:"output_var,omitempty"`
}

// ScriptPhaseExecutor runs shell commands as workflow phases.
type ScriptPhaseExecutor struct{}

// NewScriptPhaseExecutor creates a new ScriptPhaseExecutor.
func NewScriptPhaseExecutor() *ScriptPhaseExecutor {
	return &ScriptPhaseExecutor{}
}

// Name returns the executor type name.
func (e *ScriptPhaseExecutor) Name() string {
	return "script"
}

// ExecutePhase implements PhaseTypeExecutor. It extracts script config from
// the template and variables, then delegates to ExecuteScript.
func (e *ScriptPhaseExecutor) ExecutePhase(ctx context.Context, params PhaseTypeParams) (PhaseResult, error) {
	cfg := ScriptPhaseConfig{
		OutputVar: params.PhaseTemplate.OutputVarName,
	}

	// Get command: SCRIPT_COMMAND variable takes precedence, then PromptContent
	if params.Vars != nil {
		if cmd, ok := params.Vars["SCRIPT_COMMAND"]; ok && cmd != "" {
			cfg.Command = cmd
		}
	}
	if cfg.Command == "" && params.PhaseTemplate.PromptContent != "" {
		cfg.Command = params.PhaseTemplate.PromptContent
	}

	// If no command configured, complete with empty content
	if cfg.Command == "" {
		storeOutputVar(params, cfg.OutputVar, "")
		return PhaseResult{
			PhaseID: params.PhaseTemplate.ID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
		}, nil
	}

	return e.ExecuteScript(ctx, params, cfg)
}

// ExecuteScript runs a shell command with the given config.
func (e *ScriptPhaseExecutor) ExecuteScript(ctx context.Context, params PhaseTypeParams, cfg ScriptPhaseConfig) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: params.PhaseTemplate.ID,
	}

	// Validate
	if cfg.Command == "" {
		return result, fmt.Errorf("script phase: command is required")
	}

	// Validate regex before execution
	if cfg.SuccessPattern != "" {
		if _, err := regexp.Compile(cfg.SuccessPattern); err != nil {
			return result, fmt.Errorf("script phase: invalid success_pattern %q: %w", cfg.SuccessPattern, err)
		}
	}

	// Interpolate variables in config fields
	command := variable.RenderTemplate(cfg.Command, params.Vars)
	workdir := variable.RenderTemplate(cfg.Workdir, params.Vars)

	// Build command with optional timeout
	start := time.Now()
	var execCtx context.Context
	var cancel context.CancelFunc
	if cfg.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", command)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	result.DurationMS = durationMS(start)

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("script phase: command timeout after %v", cfg.Timeout)
		}
		return result, fmt.Errorf("script phase: command failed: %w", err)
	}

	content := strings.TrimSpace(stdout.String())

	// Check success pattern if configured
	if cfg.SuccessPattern != "" {
		re := regexp.MustCompile(cfg.SuccessPattern)
		if !re.MatchString(content) {
			return result, fmt.Errorf("script phase: output did not match success pattern %q", cfg.SuccessPattern)
		}
	}

	// Store output variable
	storeOutputVar(params, cfg.OutputVar, content)

	result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
	result.Content = content

	return result, nil
}

// durationMS returns the elapsed time since start in milliseconds, with a
// minimum of 1 to indicate that execution occurred (sub-millisecond operations
// would otherwise report 0).
func durationMS(start time.Time) int64 {
	ms := time.Since(start).Milliseconds()
	if ms <= 0 {
		return 1
	}
	return ms
}

// storeOutputVar stores a value to both params.Vars and params.RCtx.PhaseOutputVars
// when outputVar is non-empty.
func storeOutputVar(params PhaseTypeParams, outputVar, value string) {
	if outputVar == "" {
		return
	}
	if params.Vars != nil {
		params.Vars[outputVar] = value
	}
	if params.RCtx != nil && params.RCtx.PhaseOutputVars != nil {
		params.RCtx.PhaseOutputVars[outputVar] = value
	}
}
