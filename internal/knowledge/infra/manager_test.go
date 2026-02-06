package infra

import (
	"context"
	"errors"
	"testing"
)

// SC-1: Start starts Neo4j, Qdrant, and Redis containers when backend is "docker".
func TestManager_Start_DockerBackend(t *testing.T) {
	mock := &mockDockerClient{}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify 3 containers started
	if mock.startCount != 3 {
		t.Errorf("start calls = %d, want 3", mock.startCount)
	}

	// Verify correct images
	wantImages := map[string]bool{
		"neo4j":  false,
		"qdrant": false,
		"redis":  false,
	}
	for _, name := range mock.startedContainers {
		wantImages[name] = true
	}
	for name, started := range wantImages {
		if !started {
			t.Errorf("container %s was not started", name)
		}
	}
}

// SC-1 error path: Returns error if container fails to start; stops already-started on failure.
func TestManager_Start_RollbackOnFailure(t *testing.T) {
	mock := &mockDockerClient{
		failOnContainer: "qdrant", // Second container fails
	}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error when container fails")
	}

	// Error should mention the failed container
	if !containsString(err.Error(), "qdrant") {
		t.Errorf("error %q should mention 'qdrant'", err.Error())
	}

	// Already-started containers should be stopped (rollback)
	if mock.stopCount == 0 {
		t.Error("already-started containers should be stopped on failure (rollback)")
	}
}

// SC-2: Stop stops all running knowledge containers.
func TestManager_Stop(t *testing.T) {
	mock := &mockDockerClient{
		runningContainers: []string{"neo4j", "qdrant", "redis"},
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	err := mgr.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if mock.stopCount != 3 {
		t.Errorf("stop calls = %d, want 3", mock.stopCount)
	}
}

// SC-2: Stop is idempotent (no error if already stopped).
func TestManager_Stop_AlreadyStopped(t *testing.T) {
	mock := &mockDockerClient{
		runningContainers: []string{}, // Nothing running
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	err := mgr.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop should not error when already stopped: %v", err)
	}
}

// SC-2 error path: Returns error with context if Docker API fails.
func TestManager_Stop_DockerAPIError(t *testing.T) {
	mock := &mockDockerClient{
		runningContainers: []string{"neo4j", "qdrant", "redis"},
		failOnStop:        true,
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	err := mgr.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop should return error when Docker API fails")
	}
}

// SC-3: Status reports per-service health.
func TestManager_Status(t *testing.T) {
	mock := &mockDockerClient{
		healthStatus: map[string]string{
			"neo4j":  "healthy",
			"qdrant": "healthy",
			"redis":  "unhealthy",
		},
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	health, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if health.Neo4j != "healthy" {
		t.Errorf("Neo4j status = %s, want healthy", health.Neo4j)
	}
	if health.Qdrant != "healthy" {
		t.Errorf("Qdrant status = %s, want healthy", health.Qdrant)
	}
	if health.Redis != "unhealthy" {
		t.Errorf("Redis status = %s, want unhealthy", health.Redis)
	}
}

// SC-3: Missing containers reported as "not-found", not error.
func TestManager_Status_MissingContainers(t *testing.T) {
	mock := &mockDockerClient{
		healthStatus: map[string]string{}, // No containers exist
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	health, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status should not error for missing containers: %v", err)
	}

	if health.Neo4j != "not-found" {
		t.Errorf("Neo4j status = %s, want not-found", health.Neo4j)
	}
	if health.Qdrant != "not-found" {
		t.Errorf("Qdrant status = %s, want not-found", health.Qdrant)
	}
	if health.Redis != "not-found" {
		t.Errorf("Redis status = %s, want not-found", health.Redis)
	}
}

// SC-4: External backend mode — no Docker calls; connects to external URIs.
func TestManager_Start_ExternalBackend(t *testing.T) {
	mock := &mockDockerClient{}
	mgr := NewManager(Config{
		Backend:   "external",
		Neo4jURI:  "bolt://external:7687",
		QdrantURI: "http://external:6334",
		RedisURI:  "redis://external:6379",
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// No Docker calls should be made
	if mock.startCount != 0 {
		t.Errorf("Docker start calls = %d, want 0 for external backend", mock.startCount)
	}
}

// SC-4: Stop is no-op for external backend containers.
func TestManager_Stop_ExternalBackend(t *testing.T) {
	mock := &mockDockerClient{}
	mgr := NewManager(Config{
		Backend: "external",
	}, WithDockerClient(mock))

	err := mgr.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// No Docker stop calls
	if mock.stopCount != 0 {
		t.Errorf("Docker stop calls = %d, want 0 for external backend", mock.stopCount)
	}
}

// SC-4 error path: Returns connection error with URI if external service unreachable.
func TestManager_Start_ExternalUnreachable(t *testing.T) {
	mock := &mockDockerClient{}
	mgr := NewManager(Config{
		Backend:   "external",
		Neo4jURI:  "bolt://unreachable:7687",
		QdrantURI: "http://unreachable:6334",
		RedisURI:  "redis://unreachable:6379",
	}, WithDockerClient(mock), WithHealthCheck(func(uri string) error {
		return errors.New("connection refused")
	}))

	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error when external service is unreachable")
	}

	// Error should include the URI for debugging
	if !containsString(err.Error(), "unreachable") {
		t.Errorf("error %q should mention unreachable URI", err.Error())
	}
}

// SC-5: Start is idempotent — reuses existing containers.
func TestManager_Start_IdempotentReusesExisting(t *testing.T) {
	mock := &mockDockerClient{
		existingContainers: []string{"neo4j", "qdrant", "redis"},
		healthStatus: map[string]string{
			"neo4j":  "healthy",
			"qdrant": "healthy",
			"redis":  "healthy",
		},
	}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Should not create new containers (reuse existing)
	if mock.createCount > 0 {
		t.Errorf("create calls = %d, want 0 (should reuse existing containers)", mock.createCount)
	}
}

// SC-5 error path: If existing container is unhealthy, attempts restart.
func TestManager_Start_RestartsUnhealthyExisting(t *testing.T) {
	mock := &mockDockerClient{
		existingContainers: []string{"neo4j", "qdrant", "redis"},
		healthStatus: map[string]string{
			"neo4j":  "healthy",
			"qdrant": "unhealthy",
			"redis":  "healthy",
		},
	}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Should have restarted the unhealthy container
	if mock.restartCount == 0 {
		t.Error("expected restart of unhealthy container")
	}
}

// Failure mode: Docker daemon not running.
func TestManager_Start_DockerDaemonNotRunning(t *testing.T) {
	mock := &mockDockerClient{
		daemonDown: true,
	}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error when Docker daemon is not running")
	}

	if !containsString(err.Error(), "docker") {
		t.Errorf("error %q should mention docker", err.Error())
	}
}

// Failure mode: Port conflict (container fails to start).
func TestManager_Start_PortConflict(t *testing.T) {
	mock := &mockDockerClient{
		failOnContainer: "qdrant",
		failError:       errors.New("port 6334 already in use"),
	}
	mgr := NewManager(Config{
		Backend:    "docker",
		Neo4jPort:  7687,
		QdrantPort: 6334,
		RedisPort:  6379,
		DataDir:    t.TempDir(),
	}, WithDockerClient(mock))

	err := mgr.Start(context.Background())
	if err == nil {
		t.Fatal("Start should return error on port conflict")
	}

	if !containsString(err.Error(), "port") || !containsString(err.Error(), "6334") {
		t.Errorf("error %q should mention port conflict", err.Error())
	}
}

// Edge case: Stop when partially started.
func TestManager_Stop_PartiallyStarted(t *testing.T) {
	mock := &mockDockerClient{
		runningContainers: []string{"neo4j"}, // Only 1 of 3 running
	}
	mgr := NewManager(Config{
		Backend: "docker",
	}, WithDockerClient(mock))

	err := mgr.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop should handle partial state: %v", err)
	}

	// Should stop only the running container
	if mock.stopCount != 1 {
		t.Errorf("stop calls = %d, want 1 (only running containers)", mock.stopCount)
	}
}

// Edge case: Status when knowledge disabled.
func TestManager_Status_Disabled(t *testing.T) {
	mgr := NewManager(Config{
		Backend:  "docker",
		Disabled: true,
	})

	health, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status should not error when disabled: %v", err)
	}

	if health.Neo4j != "disabled" || health.Qdrant != "disabled" || health.Redis != "disabled" {
		t.Errorf("all services should report 'disabled' when knowledge is disabled, got neo4j=%s qdrant=%s redis=%s",
			health.Neo4j, health.Qdrant, health.Redis)
	}
}

// --- Test doubles ---

type mockDockerClient struct {
	startCount         int
	stopCount          int
	createCount        int
	restartCount       int
	startedContainers  []string
	runningContainers  []string
	existingContainers []string
	healthStatus       map[string]string
	failOnContainer    string
	failError          error
	failOnStop         bool
	daemonDown         bool
}

func (m *mockDockerClient) IsDaemonRunning(_ context.Context) error {
	if m.daemonDown {
		return errors.New("docker daemon not running")
	}
	return nil
}

func (m *mockDockerClient) StartContainer(_ context.Context, name string, port int, dataDir string) error {
	if m.failOnContainer == name {
		if m.failError != nil {
			return m.failError
		}
		return errors.New("failed to start " + name)
	}
	m.startCount++
	m.startedContainers = append(m.startedContainers, name)
	return nil
}

func (m *mockDockerClient) StopContainer(_ context.Context, name string) error {
	if m.failOnStop {
		return errors.New("docker API error")
	}
	m.stopCount++
	return nil
}

func (m *mockDockerClient) ContainerExists(_ context.Context, name string) (bool, error) {
	for _, c := range m.existingContainers {
		if c == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockDockerClient) ContainerHealth(_ context.Context, name string) (string, error) {
	if m.healthStatus == nil {
		return "", nil
	}
	return m.healthStatus[name], nil
}

func (m *mockDockerClient) RestartContainer(_ context.Context, name string) error {
	m.restartCount++
	return nil
}

func (m *mockDockerClient) CreateContainer(_ context.Context, name string, port int, dataDir string) error {
	m.createCount++
	return nil
}

func (m *mockDockerClient) ListRunning(_ context.Context) ([]string, error) {
	return m.runningContainers, nil
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
