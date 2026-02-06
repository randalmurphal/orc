package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/variable"
)

// maxAPIResponseBody is the maximum response body size to read (1MB).
const maxAPIResponseBody = 1024 * 1024

// APIPhaseConfig holds configuration for an API phase.
type APIPhaseConfig struct {
	Method        string            `json:"method,omitempty"`
	URL           string            `json:"url"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body,omitempty"`
	SuccessStatus []int             `json:"success_status,omitempty"`
	OutputVar     string            `json:"output_var,omitempty"`
}

// APIPhaseExecutor makes HTTP requests as workflow phases.
type APIPhaseExecutor struct{}

// NewAPIPhaseExecutor creates a new APIPhaseExecutor.
func NewAPIPhaseExecutor() *APIPhaseExecutor {
	return &APIPhaseExecutor{}
}

// Name returns the executor type name.
func (e *APIPhaseExecutor) Name() string {
	return "api"
}

// ExecutePhase implements PhaseTypeExecutor. It extracts API config from
// the template and variables, then delegates to ExecuteAPI.
func (e *APIPhaseExecutor) ExecutePhase(ctx context.Context, params PhaseTypeParams) (PhaseResult, error) {
	cfg := APIPhaseConfig{
		OutputVar: params.PhaseTemplate.OutputVarName,
	}

	// Try to find URL from PromptContent (after variable interpolation)
	if params.PhaseTemplate.PromptContent != "" {
		cfg.URL = variable.RenderTemplate(params.PhaseTemplate.PromptContent, params.Vars)
	}

	// Fall back to scanning vars for URL-like values
	if cfg.URL == "" && params.Vars != nil {
		for _, v := range params.Vars {
			if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
				cfg.URL = v
				break
			}
		}
	}

	// If no URL configured, complete with empty content
	if cfg.URL == "" {
		storeOutputVar(params, cfg.OutputVar, "")
		return PhaseResult{
			PhaseID: params.PhaseTemplate.ID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
		}, nil
	}

	return e.ExecuteAPI(ctx, params, cfg)
}

// ExecuteAPI makes an HTTP request with the given config.
func (e *APIPhaseExecutor) ExecuteAPI(ctx context.Context, params PhaseTypeParams, cfg APIPhaseConfig) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: params.PhaseTemplate.ID,
	}

	// Apply defaults
	method := cfg.Method
	if method == "" {
		method = "GET"
	}
	successStatus := cfg.SuccessStatus
	if len(successStatus) == 0 {
		successStatus = []int{200}
	}

	// Validate
	if cfg.URL == "" {
		return result, fmt.Errorf("api phase: URL is required")
	}

	// Interpolate variables in config fields
	url := variable.RenderTemplate(cfg.URL, params.Vars)
	body := variable.RenderTemplate(cfg.Body, params.Vars)

	resolvedHeaders := make(map[string]string, len(cfg.Headers))
	for k, v := range cfg.Headers {
		resolvedHeaders[k] = variable.RenderTemplate(v, params.Vars)
	}

	// Build HTTP request
	start := time.Now()

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return result, fmt.Errorf("api phase: build request: %w", err)
	}

	for k, v := range resolvedHeaders {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		result.DurationMS = durationMS(start)
		return result, fmt.Errorf("api phase: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body (with size limit)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBody+1))
	if err != nil {
		result.DurationMS = durationMS(start)
		return result, fmt.Errorf("api phase: read response: %w", err)
	}

	result.DurationMS = durationMS(start)

	// Truncate if over limit
	content := string(respBody)
	if len(respBody) > maxAPIResponseBody {
		content = content[:maxAPIResponseBody]
	}

	// Check status code against success list
	statusMatch := false
	for _, s := range successStatus {
		if resp.StatusCode == s {
			statusMatch = true
			break
		}
	}
	if !statusMatch {
		return result, fmt.Errorf("api phase: unexpected HTTP status %d (expected one of %v)", resp.StatusCode, successStatus)
	}

	// Store output variable
	storeOutputVar(params, cfg.OutputVar, content)

	result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
	result.Content = content

	return result, nil
}
