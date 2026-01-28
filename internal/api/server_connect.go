// Package api provides the REST API and Connect RPC server for orc.
// This file registers Connect RPC service handlers.
package api

import (
	"net/http"
	"path/filepath"

	"connectrpc.com/connect"

	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/workflow"
)

// registerConnectHandlers sets up Connect RPC service handlers.
// These run alongside REST handlers on the same mux.
func (s *Server) registerConnectHandlers() {
	// Create interceptor chain with logging and error mapping
	interceptors := connect.WithInterceptors(
		ErrorInterceptor(),
		LoggingInterceptor(s.logger),
	)

	// Create service implementations
	// Use NewTaskServerWithExecutor to enable RunTask to spawn actual executor
	taskSvc := NewTaskServerWithExecutor(s.backend, s.orcConfig, s.logger, s.publisher, s.workDir, s.diffCache, s.projectDB, s.startTask)
	initiativeSvc := NewInitiativeServer(s.backend, s.logger, s.publisher)
	// Create resolver for workflow/phase source tracking
	orcDir := filepath.Join(s.workDir, ".orc")
	resolver := workflow.NewResolverFromOrcDir(orcDir)
	workflowSvc := NewWorkflowServer(s.backend, resolver, s.logger)
	transcriptSvc := NewTranscriptServer(s.backend)
	eventSvc := NewEventServer(s.publisher, s.backend, s.logger)
	configSvc := NewConfigServer(s.orcConfig, s.backend, s.workDir, s.logger)
	githubSvc := NewGitHubServerWithExecutor(s.backend, s.workDir, s.logger, s.publisher, s.orcConfig, s.startTask, nil)
	dashboardSvc := NewDashboardServer(s.backend, s.logger)
	projectSvc := NewProjectServer(s.backend, s.logger)
	branchSvc := NewBranchServer(s.backend, s.logger)
	decisionSvc := NewDecisionServer(s.backend, s.pendingDecisions, s.publisher, s.logger)
	notificationSvc := NewNotificationServer(s.backend, s.logger)
	mcpSvc := NewMCPServer(s.workDir, s.logger)
	knowledgeSvc := NewKnowledgeServer(s.backend, s.logger)

	// Create and register Connect handlers with CORS support
	// Each NewXxxServiceHandler returns (path string, handler http.Handler)

	taskPath, taskHandler := orcv1connect.NewTaskServiceHandler(taskSvc, interceptors)
	s.mux.Handle(taskPath, corsHandler(taskHandler))

	initiativePath, initiativeHandler := orcv1connect.NewInitiativeServiceHandler(initiativeSvc, interceptors)
	s.mux.Handle(initiativePath, corsHandler(initiativeHandler))

	workflowPath, workflowHandler := orcv1connect.NewWorkflowServiceHandler(workflowSvc, interceptors)
	s.mux.Handle(workflowPath, corsHandler(workflowHandler))

	transcriptPath, transcriptHandler := orcv1connect.NewTranscriptServiceHandler(transcriptSvc, interceptors)
	s.mux.Handle(transcriptPath, corsHandler(transcriptHandler))

	eventPath, eventHandler := orcv1connect.NewEventServiceHandler(eventSvc, interceptors)
	s.mux.Handle(eventPath, corsHandler(eventHandler))

	configPath, configHandler := orcv1connect.NewConfigServiceHandler(configSvc, interceptors)
	s.mux.Handle(configPath, corsHandler(configHandler))

	githubPath, githubHandler := orcv1connect.NewGitHubServiceHandler(githubSvc, interceptors)
	s.mux.Handle(githubPath, corsHandler(githubHandler))

	dashboardPath, dashboardHandler := orcv1connect.NewDashboardServiceHandler(dashboardSvc, interceptors)
	s.mux.Handle(dashboardPath, corsHandler(dashboardHandler))

	projectPath, projectHandler := orcv1connect.NewProjectServiceHandler(projectSvc, interceptors)
	s.mux.Handle(projectPath, corsHandler(projectHandler))

	branchPath, branchHandler := orcv1connect.NewBranchServiceHandler(branchSvc, interceptors)
	s.mux.Handle(branchPath, corsHandler(branchHandler))

	decisionPath, decisionHandler := orcv1connect.NewDecisionServiceHandler(decisionSvc, interceptors)
	s.mux.Handle(decisionPath, corsHandler(decisionHandler))

	notificationPath, notificationHandler := orcv1connect.NewNotificationServiceHandler(notificationSvc, interceptors)
	s.mux.Handle(notificationPath, corsHandler(notificationHandler))

	mcpPath, mcpHandler := orcv1connect.NewMCPServiceHandler(mcpSvc, interceptors)
	s.mux.Handle(mcpPath, corsHandler(mcpHandler))

	knowledgePath, knowledgeHandler := orcv1connect.NewKnowledgeServiceHandler(knowledgeSvc, interceptors)
	s.mux.Handle(knowledgePath, corsHandler(knowledgeHandler))

	s.logger.Info("registered Connect RPC handlers", "count", 14)
}

// corsHandler wraps a handler with CORS support for Connect/gRPC-web clients.
func corsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, Connect-Timeout-Ms, Grpc-Timeout, X-Grpc-Web, X-User-Agent")
		w.Header().Set("Access-Control-Expose-Headers", "Grpc-Status, Grpc-Message, Grpc-Status-Details-Bin")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		h.ServeHTTP(w, r)
	})
}
