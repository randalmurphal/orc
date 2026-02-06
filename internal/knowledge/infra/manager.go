// Package infra manages knowledge infrastructure containers (Neo4j, Qdrant, Redis).
package infra

import (
	"context"
	"fmt"
)

// Health represents per-service health status.
type Health struct {
	Neo4j  string
	Qdrant string
	Redis  string
}

// Config holds infrastructure manager configuration.
type Config struct {
	Backend    string
	Neo4jPort  int
	QdrantPort int
	RedisPort  int
	DataDir    string
	Neo4jURI   string
	QdrantURI  string
	RedisURI   string
	Disabled   bool
}

// DockerClient abstracts Docker container operations for testing.
type DockerClient interface {
	StartContainer(ctx context.Context, name string, port int, dataDir string) error
	StopContainer(ctx context.Context, name string) error
	ContainerExists(ctx context.Context, name string) (bool, error)
	ContainerHealth(ctx context.Context, name string) (string, error)
	RestartContainer(ctx context.Context, name string) error
	CreateContainer(ctx context.Context, name string, port int, dataDir string) error
	ListRunning(ctx context.Context) ([]string, error)
	IsDaemonRunning(ctx context.Context) error
}

// HealthCheckFunc checks connectivity to a service URI.
type HealthCheckFunc func(uri string) error

// Manager manages knowledge infrastructure containers.
type Manager struct {
	cfg         Config
	docker      DockerClient
	healthCheck HealthCheckFunc
}

// ManagerOption configures the Manager.
type ManagerOption func(*Manager)

// NewManager creates a new infrastructure manager.
func NewManager(cfg Config, opts ...ManagerOption) *Manager {
	m := &Manager{cfg: cfg}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// WithDockerClient sets a custom Docker client (for testing).
func WithDockerClient(client DockerClient) ManagerOption {
	return func(m *Manager) {
		m.docker = client
	}
}

// WithHealthCheck sets a custom health check function (for testing).
func WithHealthCheck(fn HealthCheckFunc) ManagerOption {
	return func(m *Manager) {
		m.healthCheck = fn
	}
}

// containerNames are the managed container names in start order.
var containerNames = []string{"neo4j", "qdrant", "redis"}

// Start starts the knowledge infrastructure.
func (m *Manager) Start(ctx context.Context) error {
	if m.cfg.Backend == "external" {
		return m.startExternal(ctx)
	}
	return m.startDocker(ctx)
}

func (m *Manager) startDocker(ctx context.Context) error {
	if err := m.docker.IsDaemonRunning(ctx); err != nil {
		return fmt.Errorf("start knowledge infrastructure: docker daemon not running: %w", err)
	}

	var started []string
	for _, name := range containerNames {
		port := m.portFor(name)

		// Check if container already exists
		exists, err := m.docker.ContainerExists(ctx, name)
		if err != nil {
			m.rollback(ctx, started)
			return fmt.Errorf("check %s container: %w", name, err)
		}

		if exists {
			// Check health of existing container
			health, err := m.docker.ContainerHealth(ctx, name)
			if err != nil {
				m.rollback(ctx, started)
				return fmt.Errorf("check %s health: %w", name, err)
			}
			if health != "healthy" {
				if err := m.docker.RestartContainer(ctx, name); err != nil {
					m.rollback(ctx, started)
					return fmt.Errorf("restart %s container: %w", name, err)
				}
			}
			// Existing container reused — don't count as "started" for rollback
			continue
		}

		if err := m.docker.StartContainer(ctx, name, port, m.cfg.DataDir); err != nil {
			m.rollback(ctx, started)
			return fmt.Errorf("start %s container: %w", name, err)
		}
		started = append(started, name)
	}
	return nil
}

func (m *Manager) startExternal(ctx context.Context) error {
	if m.healthCheck == nil {
		return nil
	}
	uris := map[string]string{
		"neo4j":  m.cfg.Neo4jURI,
		"qdrant": m.cfg.QdrantURI,
		"redis":  m.cfg.RedisURI,
	}
	for _, name := range containerNames {
		uri := uris[name]
		if err := m.healthCheck(uri); err != nil {
			return fmt.Errorf("connect to external %s at %s: %w", name, uri, err)
		}
	}
	return nil
}

func (m *Manager) rollback(ctx context.Context, started []string) {
	for i := len(started) - 1; i >= 0; i-- {
		_ = m.docker.StopContainer(ctx, started[i])
	}
}

func (m *Manager) portFor(name string) int {
	switch name {
	case "neo4j":
		return m.cfg.Neo4jPort
	case "qdrant":
		return m.cfg.QdrantPort
	case "redis":
		return m.cfg.RedisPort
	default:
		return 0
	}
}

// Stop stops the knowledge infrastructure.
func (m *Manager) Stop(ctx context.Context) error {
	if m.cfg.Backend == "external" {
		return nil
	}

	running, err := m.docker.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("list running containers: %w", err)
	}

	for _, name := range running {
		if err := m.docker.StopContainer(ctx, name); err != nil {
			return fmt.Errorf("stop %s container: %w", name, err)
		}
	}
	return nil
}

// Status returns health status for each service.
func (m *Manager) Status(ctx context.Context) (*Health, error) {
	if m.cfg.Disabled {
		return &Health{
			Neo4j:  "disabled",
			Qdrant: "disabled",
			Redis:  "disabled",
		}, nil
	}

	h := &Health{}
	for _, name := range containerNames {
		status, err := m.docker.ContainerHealth(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("check %s status: %w", name, err)
		}
		if status == "" {
			status = "not-found"
		}
		switch name {
		case "neo4j":
			h.Neo4j = status
		case "qdrant":
			h.Qdrant = status
		case "redis":
			h.Redis = status
		}
	}
	return h, nil
}
