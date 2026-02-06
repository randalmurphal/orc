package knowledge

import (
	"context"
	"errors"
	"testing"
)

// SC-15: Service.IsAvailable returns false when knowledge.enabled is false.
func TestService_IsAvailable_Disabled(t *testing.T) {
	svc := NewService(ServiceConfig{
		Enabled: false,
	})

	if svc.IsAvailable() {
		t.Error("IsAvailable should return false when knowledge is disabled")
	}
}

// SC-15: No container or connection operations attempted when disabled.
func TestService_IsAvailable_DisabledNoOperations(t *testing.T) {
	mock := &mockComponents{}
	svc := NewService(ServiceConfig{
		Enabled: false,
	}, WithComponents(mock))

	_ = svc.IsAvailable()

	if mock.healthCheckCalled {
		t.Error("no health checks should be performed when disabled")
	}
}

// SC-16: Service.IsAvailable returns false when any component is unhealthy.
func TestService_IsAvailable_UnhealthyComponent(t *testing.T) {
	tests := []struct {
		name          string
		neo4jHealthy  bool
		qdrantHealthy bool
		redisHealthy  bool
		expectAvail   bool
	}{
		{"all healthy", true, true, true, true},
		{"neo4j unhealthy", false, true, true, false},
		{"qdrant unhealthy", true, false, true, false},
		{"redis unhealthy", true, true, false, false},
		{"all unhealthy", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockComponents{
				neo4jHealthy:  tt.neo4jHealthy,
				qdrantHealthy: tt.qdrantHealthy,
				redisHealthy:  tt.redisHealthy,
			}
			svc := NewService(ServiceConfig{
				Enabled: true,
			}, WithComponents(mock))

			got := svc.IsAvailable()
			if got != tt.expectAvail {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.expectAvail)
			}
		})
	}
}

// SC-17: Service.Start orchestrates startup in correct order.
func TestService_Start_CorrectOrder(t *testing.T) {
	mock := &mockComponents{}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify call order: infra.Start → graph.Connect → vector.Connect → cache.Connect
	expectedOrder := []string{"infra.Start", "graph.Connect", "vector.Connect", "cache.Connect"}
	if len(mock.callOrder) != len(expectedOrder) {
		t.Fatalf("call order = %v, want %v", mock.callOrder, expectedOrder)
	}
	for i, want := range expectedOrder {
		if mock.callOrder[i] != want {
			t.Errorf("call[%d] = %s, want %s", i, mock.callOrder[i], want)
		}
	}
}

// SC-17: Service.Stop reverses startup order.
func TestService_Stop_ReverseOrder(t *testing.T) {
	mock := &mockComponents{}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	err := svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Verify call order: cache.Close → vector.Close → graph.Close → infra.Stop
	expectedOrder := []string{"cache.Close", "vector.Close", "graph.Close", "infra.Stop"}
	if len(mock.callOrder) != len(expectedOrder) {
		t.Fatalf("call order = %v, want %v", mock.callOrder, expectedOrder)
	}
	for i, want := range expectedOrder {
		if mock.callOrder[i] != want {
			t.Errorf("call[%d] = %s, want %s", i, mock.callOrder[i], want)
		}
	}
}

// SC-17 error path: Start failure cleans up already-started components.
func TestService_Start_FailureCleanup(t *testing.T) {
	mock := &mockComponents{
		failOn: "vector.Connect", // Fail at step 3
	}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error when component fails")
	}

	// Error should have context about what failed
	if !containsStr(err.Error(), "vector") {
		t.Errorf("error %q should mention the failed step", err.Error())
	}

	// Already-started components should be cleaned up
	hasCleanup := false
	for _, call := range mock.callOrder {
		if call == "graph.Close" || call == "infra.Stop" {
			hasCleanup = true
		}
	}
	if !hasCleanup {
		t.Error("already-started components should be cleaned up on failure")
	}
}

// SC-17 error path: Start failure with context in error message.
func TestService_Start_ErrorContext(t *testing.T) {
	mock := &mockComponents{
		failOn:    "cache.Connect",
		failError: errors.New("connection refused"),
	}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error")
	}

	// Error should wrap the original
	if !containsStr(err.Error(), "connection refused") {
		t.Errorf("error %q should wrap original error", err.Error())
	}
}

// Integration: Service wires infrastructure manager correctly.
func TestService_Start_CallsInfraStart(t *testing.T) {
	mock := &mockComponents{}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	_ = svc.Start(context.Background())

	// Verify infra.Start was called (integration check)
	infraCalled := false
	for _, call := range mock.callOrder {
		if call == "infra.Start" {
			infraCalled = true
			break
		}
	}
	if !infraCalled {
		t.Fatal("Service.Start must call infra.Start — wiring is missing")
	}
}

// Integration: Service wires all store connects.
func TestService_Start_CallsAllStoreConnects(t *testing.T) {
	mock := &mockComponents{}
	svc := NewService(ServiceConfig{
		Enabled: true,
		Backend: "docker",
	}, WithComponents(mock))

	_ = svc.Start(context.Background())

	// Verify all store connects were called
	wantCalls := map[string]bool{
		"graph.Connect":  false,
		"vector.Connect": false,
		"cache.Connect":  false,
	}
	for _, call := range mock.callOrder {
		if _, ok := wantCalls[call]; ok {
			wantCalls[call] = true
		}
	}
	for call, found := range wantCalls {
		if !found {
			t.Errorf("Service.Start must call %s — wiring is missing", call)
		}
	}
}

// --- Types and stubs ---

// ServiceConfig configures the knowledge Service.
type ServiceConfig struct {
	Enabled bool
	Backend string
}

// Service orchestrates knowledge infrastructure.
type Service struct{}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// NewService creates a new knowledge service.
func NewService(cfg ServiceConfig, opts ...ServiceOption) *Service {
	return nil
}

// WithComponents injects mock components (for testing).
func WithComponents(comps *mockComponents) ServiceOption {
	return func(s *Service) {}
}

// Start starts infrastructure then connects stores.
func (s *Service) Start(ctx context.Context) error {
	return errors.New("not implemented")
}

// Stop disconnects stores then stops infrastructure.
func (s *Service) Stop(ctx context.Context) error {
	return errors.New("not implemented")
}

// IsAvailable returns whether the knowledge layer is available.
func (s *Service) IsAvailable() bool {
	return false
}

// Status returns infrastructure health.
func (s *Service) Status(ctx context.Context) (interface{}, error) {
	return nil, errors.New("not implemented")
}

type mockComponents struct {
	callOrder        []string
	healthCheckCalled bool
	neo4jHealthy     bool
	qdrantHealthy    bool
	redisHealthy     bool
	failOn           string
	failError        error
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
