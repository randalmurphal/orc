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
	taskSvc.SetProjectCache(s.projectCache)

	initiativeSvc := NewInitiativeServerWithCache(s.backend, s.logger, s.publisher, s.projectCache)
	// Create resolver, cloner, and cache for workflow/phase source tracking
	orcDir := filepath.Join(s.workDir, ".orc")
	resolver := workflow.NewResolverFromOrcDir(orcDir)
	cloner := workflow.NewClonerFromOrcDir(orcDir)
	cache := workflow.NewCacheService(resolver, s.globalDB)
	workflowSvc := NewWorkflowServer(s.backend, s.globalDB, resolver, cloner, cache, s.logger)
	if ws, ok := workflowSvc.(*workflowServer); ok {
		ws.SetProjectCache(s.projectCache)
	}
	transcriptSvc := NewTranscriptServer(s.backend)
	if ts, ok := transcriptSvc.(*transcriptServer); ok {
		ts.SetProjectCache(s.projectCache)
	}
	eventSvc := NewEventServer(s.publisher, s.backend, s.logger)
	if es, ok := eventSvc.(*eventServer); ok {
		es.SetProjectCache(s.projectCache)
	}
	configSvc := NewConfigServer(s.orcConfig, s.backend, s.workDir, s.logger)
	if cs, ok := configSvc.(*configServer); ok {
		cs.SetProjectCache(s.projectCache)
		cs.SetGlobalDB(s.globalDB)
	}
	hostingSvc := NewHostingServerWithExecutor(s.backend, s.workDir, s.logger, s.publisher, s.orcConfig, s.startTask, nil)
	if hs, ok := hostingSvc.(*hostingServer); ok {
		hs.SetProjectCache(s.projectCache)
	}
	dashboardSvc := NewDashboardServer(s.backend, s.logger)
	if ds, ok := dashboardSvc.(*dashboardServer); ok {
		ds.SetProjectCache(s.projectCache)
	}
	projectSvc := NewProjectServer(s.backend, s.logger)
	branchSvc := NewBranchServer(s.backend, s.logger)
	if bs, ok := branchSvc.(*branchServer); ok {
		bs.SetProjectCache(s.projectCache)
	}
	decisionSvc := NewDecisionServer(s.backend, s.pendingDecisions, s.publisher, s.logger)
	if decs, ok := decisionSvc.(*decisionServer); ok {
		decs.SetProjectCache(s.projectCache)
	}
	notificationSvc := NewNotificationServer(s.backend, s.logger)
	if ns, ok := notificationSvc.(*notificationServer); ok {
		ns.SetProjectCache(s.projectCache)
	}
	mcpSvc := NewMCPServer(s.workDir, s.logger)

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

	hostingPath, hostingHandler := orcv1connect.NewHostingServiceHandler(hostingSvc, interceptors)
	s.mux.Handle(hostingPath, corsHandler(hostingHandler))

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

	s.logger.Info("registered Connect RPC handlers", "count", 13)
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
