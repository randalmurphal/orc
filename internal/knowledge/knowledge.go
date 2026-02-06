// Package knowledge provides the knowledge layer service for orc.
package knowledge

import (
	"context"
	"fmt"
)

// ServiceConfig configures the knowledge Service.
type ServiceConfig struct {
	Enabled bool
	Backend string
}

// Components abstracts the infrastructure and store dependencies for testing.
type Components interface {
	InfraStart(ctx context.Context) error
	InfraStop(ctx context.Context) error
	GraphConnect(ctx context.Context) error
	GraphClose() error
	VectorConnect(ctx context.Context) error
	VectorClose() error
	CacheConnect(ctx context.Context) error
	CacheClose() error
	IsHealthy() (neo4j, qdrant, redis bool)
}

// Service orchestrates knowledge infrastructure.
type Service struct {
	cfg   ServiceConfig
	comps Components
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// NewService creates a new knowledge service.
func NewService(cfg ServiceConfig, opts ...ServiceOption) *Service {
	s := &Service{cfg: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithComponents injects mock components (for testing).
func WithComponents(comps Components) ServiceOption {
	return func(s *Service) {
		s.comps = comps
	}
}

// IsAvailable returns whether the knowledge layer is available.
func (s *Service) IsAvailable() bool {
	if !s.cfg.Enabled {
		return false
	}
	if s.comps == nil {
		return false
	}
	neo4j, qdrant, redis := s.comps.IsHealthy()
	return neo4j && qdrant && redis
}

type startStep struct {
	name string
	fn   func(ctx context.Context) error
}

// Start starts infrastructure then connects stores.
// Order: infra.Start → graph.Connect → vector.Connect → cache.Connect
func (s *Service) Start(ctx context.Context) error {
	if s.comps == nil {
		return fmt.Errorf("knowledge service: components not configured")
	}
	steps := []startStep{
		{"infra.Start", s.comps.InfraStart},
		{"graph.Connect", s.comps.GraphConnect},
		{"vector.Connect", s.comps.VectorConnect},
		{"cache.Connect", s.comps.CacheConnect},
	}

	for i, st := range steps {
		if err := st.fn(ctx); err != nil {
			s.cleanupFrom(ctx, steps, i-1)
			return fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return nil
}

// cleanupFrom cleans up completed steps in reverse order.
func (s *Service) cleanupFrom(ctx context.Context, steps []startStep, lastCompleted int) {
	for i := lastCompleted; i >= 0; i-- {
		switch steps[i].name {
		case "infra.Start":
			_ = s.comps.InfraStop(ctx)
		case "graph.Connect":
			_ = s.comps.GraphClose()
		case "vector.Connect":
			_ = s.comps.VectorClose()
		case "cache.Connect":
			_ = s.comps.CacheClose()
		}
	}
}

// Stop disconnects stores then stops infrastructure.
// Order: cache.Close → vector.Close → graph.Close → infra.Stop
func (s *Service) Stop(ctx context.Context) error {
	if s.comps == nil {
		return fmt.Errorf("knowledge service: components not configured")
	}
	var firstErr error
	if err := s.comps.CacheClose(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("cache.Close: %w", err)
	}
	if err := s.comps.VectorClose(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("vector.Close: %w", err)
	}
	if err := s.comps.GraphClose(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("graph.Close: %w", err)
	}
	if err := s.comps.InfraStop(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("infra.Stop: %w", err)
	}
	return firstErr
}

// ServiceStatus holds the health status of the knowledge service.
type ServiceStatus struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
	Neo4j   bool `json:"neo4j"`
	Qdrant  bool `json:"qdrant"`
	Redis   bool `json:"redis"`
}

// Status returns infrastructure health.
func (s *Service) Status(_ context.Context) (*ServiceStatus, error) {
	st := &ServiceStatus{Enabled: s.cfg.Enabled}
	if !s.cfg.Enabled || s.comps == nil {
		return st, nil
	}
	st.Neo4j, st.Qdrant, st.Redis = s.comps.IsHealthy()
	st.Running = st.Neo4j && st.Qdrant && st.Redis
	return st, nil
}
