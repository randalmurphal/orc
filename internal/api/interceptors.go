// Package api provides the Connect RPC and REST API server for orc.
package api

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
)

// LoggingInterceptor returns a Connect interceptor that logs RPC calls with
// method name, duration, and any errors.
func LoggingInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			procedure := req.Spec().Procedure

			resp, err := next(ctx, req)

			duration := time.Since(start)

			// Extract just the method name from the procedure
			// Procedure format: /orc.v1.TaskService/ListTasks
			parts := strings.Split(procedure, "/")
			method := procedure
			if len(parts) >= 3 {
				method = parts[2]
			}

			if err != nil {
				logger.Error("rpc failed",
					"method", method,
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Debug("rpc completed",
					"method", method,
					"duration", duration,
				)
			}

			return resp, err
		}
	}
}

// ErrorInterceptor returns a Connect interceptor that maps internal errors
// to appropriate Connect error codes.
func ErrorInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err != nil {
				return resp, mapError(err)
			}
			return resp, nil
		}
	}
}

// mapError converts internal errors to Connect errors with appropriate codes.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	// Already a Connect error - return as is
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return err
	}

	// Check for OrcError
	var orcErr *orcerrors.OrcError
	if errors.As(err, &orcErr) {
		code := mapOrcErrorCode(orcErr.Code)
		return connect.NewError(code, errors.New(orcErr.What))
	}

	// Check for specific error messages
	errMsg := err.Error()

	// Task not found
	if strings.Contains(errMsg, "task not found") ||
		strings.Contains(errMsg, "not found") {
		return connect.NewError(connect.CodeNotFound, err)
	}

	// Validation errors
	if strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "required") ||
		strings.Contains(errMsg, "validation") {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Circular dependency
	if strings.Contains(errMsg, "circular dependency") {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}

	// Conflict (e.g., task already running)
	if strings.Contains(errMsg, "already running") ||
		strings.Contains(errMsg, "cannot") ||
		strings.Contains(errMsg, "conflict") {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}

	// Default to internal error
	return connect.NewError(connect.CodeInternal, err)
}

// mapOrcErrorCode maps OrcError codes to Connect error codes.
// Uses the error's Category for mapping since it already groups codes by HTTP semantics.
func mapOrcErrorCode(code orcerrors.Code) connect.Code {
	switch code {
	// Not found errors
	case orcerrors.CodeTaskNotFound:
		return connect.CodeNotFound

	// Bad request / validation errors
	case orcerrors.CodeNotInitialized,
		orcerrors.CodeTaskInvalidState,
		orcerrors.CodeConfigInvalid,
		orcerrors.CodeConfigMissing,
		orcerrors.CodeGitDirty:
		return connect.CodeInvalidArgument

	// Conflict / precondition errors
	case orcerrors.CodeAlreadyInitialized,
		orcerrors.CodeTaskRunning,
		orcerrors.CodeGitBranchExists:
		return connect.CodeFailedPrecondition

	// Timeout errors
	case orcerrors.CodeClaudeTimeout:
		return connect.CodeDeadlineExceeded

	// Unavailable errors
	case orcerrors.CodeClaudeUnavailable:
		return connect.CodeUnavailable

	// Internal errors (phase stuck, max retries)
	case orcerrors.CodePhaseStuck,
		orcerrors.CodeMaxRetries:
		return connect.CodeInternal

	default:
		return connect.CodeInternal
	}
}

