package api

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/storage"
)

type briefServer struct {
	orcv1connect.UnimplementedBriefServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
}

// NewBriefServer creates a new brief API server.
func NewBriefServer(backend storage.Backend, projectCache *ProjectCache) orcv1connect.BriefServiceHandler {
	return &briefServer{
		backend:      backend,
		projectCache: projectCache,
	}
}

func (s *briefServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

func (s *briefServer) GetProjectBrief(
	ctx context.Context,
	req *connect.Request[orcv1.GetProjectBriefRequest],
) (*connect.Response[orcv1.GetProjectBriefResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	b, err := s.generateBrief(ctx, backend)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generate brief: %w", err))
	}

	return connect.NewResponse(briefToProto(b)), nil
}

func (s *briefServer) RegenerateProjectBrief(
	ctx context.Context,
	req *connect.Request[orcv1.RegenerateProjectBriefRequest],
) (*connect.Response[orcv1.RegenerateProjectBriefResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	b, err := s.generateBriefWithOptions(ctx, backend, true)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("regenerate brief: %w", err))
	}

	resp := briefToProto(b)
	return connect.NewResponse(&orcv1.RegenerateProjectBriefResponse{
		Sections:    resp.Sections,
		GeneratedAt: resp.GeneratedAt,
		TokenCount:  resp.TokenCount,
		TaskCount:   resp.TaskCount,
	}), nil
}

func (s *briefServer) generateBrief(ctx context.Context, backend storage.Backend) (*brief.Brief, error) {
	return s.generateBriefWithOptions(ctx, backend, false)
}

func (s *briefServer) generateBriefWithOptions(ctx context.Context, backend storage.Backend, invalidateCache bool) (*brief.Brief, error) {
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return &brief.Brief{}, nil
	}

	cfg := brief.DefaultConfig()
	gen := brief.NewGenerator(dbBackend, cfg)
	if invalidateCache {
		if err := gen.Invalidate(); err != nil {
			return nil, fmt.Errorf("invalidate brief cache: %w", err)
		}
	}
	return gen.Generate(ctx)
}

func briefToProto(b *brief.Brief) *orcv1.GetProjectBriefResponse {
	if b == nil {
		return &orcv1.GetProjectBriefResponse{}
	}

	var sections []*orcv1.BriefSection
	for _, s := range b.Sections {
		var entries []*orcv1.BriefEntry
		for _, e := range s.Entries {
			entries = append(entries, &orcv1.BriefEntry{
				Content: e.Content,
				Source:  e.Source,
				Impact:  e.Impact,
			})
		}
		sections = append(sections, &orcv1.BriefSection{
			Category: s.Category,
			Entries:  entries,
		})
	}

	var generatedAt *timestamppb.Timestamp
	if !b.GeneratedAt.IsZero() {
		generatedAt = timestamppb.New(b.GeneratedAt)
	}

	return &orcv1.GetProjectBriefResponse{
		Sections:    sections,
		GeneratedAt: generatedAt,
		TokenCount:  int32(b.TokenCount),
		TaskCount:   int32(b.TaskCount),
	}
}
