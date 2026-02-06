package embed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// SC-12: Voyage embedder batches requests at batch size 64.
func TestVoyageEmbedder_BatchSplitting(t *testing.T) {
	var requestCount atomic.Int32
	var requestBodies []voyageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		var req voyageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		requestBodies = append(requestBodies, req)

		// Return vectors for each input
		vectors := make([][]float32, len(req.Input))
		for i := range vectors {
			vectors[i] = make([]float32, 1024)
		}

		resp := voyageResponse{
			Data: make([]voyageEmbedding, len(req.Input)),
		}
		for i := range resp.Data {
			resp.Data[i] = voyageEmbedding{
				Embedding: vectors[i],
				Index:     i,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "test-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	// Embed 150 texts — should make 3 requests (64 + 64 + 22)
	texts := make([]string, 150)
	for i := range texts {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	vectors, err := embedder.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	// Verify 3 HTTP requests
	if int(requestCount.Load()) != 3 {
		t.Errorf("HTTP requests = %d, want 3 (batches of 64+64+22)", requestCount.Load())
	}

	// Verify batch sizes
	if len(requestBodies) >= 1 && len(requestBodies[0].Input) != 64 {
		t.Errorf("batch 1 size = %d, want 64", len(requestBodies[0].Input))
	}
	if len(requestBodies) >= 2 && len(requestBodies[1].Input) != 64 {
		t.Errorf("batch 2 size = %d, want 64", len(requestBodies[1].Input))
	}
	if len(requestBodies) >= 3 && len(requestBodies[2].Input) != 22 {
		t.Errorf("batch 3 size = %d, want 22", len(requestBodies[2].Input))
	}

	// Verify 150 vectors returned, all 1024-dimensional
	if len(vectors) != 150 {
		t.Fatalf("vectors count = %d, want 150", len(vectors))
	}
	for i, v := range vectors {
		if len(v) != 1024 {
			t.Errorf("vector[%d] dimension = %d, want 1024", i, len(v))
		}
	}
}

// SC-12: Verify request body contains correct model.
func TestVoyageEmbedder_RequestModel(t *testing.T) {
	var capturedModel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req voyageRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedModel = req.Model

		resp := voyageResponse{
			Data: []voyageEmbedding{{Embedding: make([]float32, 1024), Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "test-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	_, err := embedder.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if capturedModel != "voyage-4" {
		t.Errorf("request model = %s, want voyage-4", capturedModel)
	}
}

// SC-12: Single text makes single API call (no batching overhead).
func TestVoyageEmbedder_SingleText(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		resp := voyageResponse{
			Data: []voyageEmbedding{{Embedding: make([]float32, 1024), Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "test-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	vectors, err := embedder.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if int(requestCount.Load()) != 1 {
		t.Errorf("HTTP requests = %d, want 1", requestCount.Load())
	}
	if len(vectors) != 1 {
		t.Errorf("vectors count = %d, want 1", len(vectors))
	}
}

// SC-12 edge case: Empty text slice returns empty slice, no API call.
func TestVoyageEmbedder_EmptyInput(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "test-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	vectors, err := embedder.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if int(requestCount.Load()) != 0 {
		t.Errorf("HTTP requests = %d, want 0 for empty input", requestCount.Load())
	}
	if len(vectors) != 0 {
		t.Errorf("vectors count = %d, want 0", len(vectors))
	}
}

// SC-12 error path: Missing VOYAGE_API_KEY returns error before any API call.
func TestVoyageEmbedder_MissingAPIKey(t *testing.T) {
	_, err := NewVoyageEmbedderFromEnv(VoyageConfig{
		Model: "voyage-4",
		// No APIKey set, env var not set
	})
	if err == nil {
		t.Fatal("NewVoyageEmbedderFromEnv should return error when API key is missing")
	}

	if !containsStr(err.Error(), "VOYAGE_API_KEY") {
		t.Errorf("error %q should mention VOYAGE_API_KEY", err.Error())
	}
}

// SC-12 error path: HTTP 429 returns retryable error.
func TestVoyageEmbedder_RateLimit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Succeed on retry
		resp := voyageResponse{
			Data: []voyageEmbedding{{Embedding: make([]float32, 1024), Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "test-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	vectors, err := embedder.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("Embed should succeed after retry: %v", err)
	}

	if callCount < 2 {
		t.Errorf("expected retry after 429, got %d calls", callCount)
	}
	if len(vectors) != 1 {
		t.Errorf("vectors count = %d, want 1", len(vectors))
	}
}

// SC-12 error path: HTTP 401 returns auth error.
func TestVoyageEmbedder_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	embedder := NewVoyageEmbedder(VoyageConfig{
		APIKey:  "bad-key",
		Model:   "voyage-4",
		BaseURL: server.URL,
	})

	_, err := embedder.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("Embed should return error on 401")
	}
}

// SC-13: Sidecar embedder calls local FastAPI service.
func TestSidecarEmbedder_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sidecarRequest
		json.NewDecoder(r.Body).Decode(&req)

		vectors := make([][]float32, len(req.Texts))
		for i := range vectors {
			vectors[i] = make([]float32, 1024)
		}

		resp := sidecarResponse{Embeddings: vectors}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewSidecarEmbedder(SidecarConfig{
		URL: server.URL,
	})

	vectors, err := embedder.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if len(vectors) != 2 {
		t.Fatalf("vectors count = %d, want 2", len(vectors))
	}
	for i, v := range vectors {
		if len(v) != 1024 {
			t.Errorf("vector[%d] dimension = %d, want 1024", i, len(v))
		}
	}
}

// SC-13: Sidecar embedder does not require API key.
func TestSidecarEmbedder_NoAPIKeyRequired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("sidecar request should not have Authorization header, got %s", auth)
		}

		resp := sidecarResponse{
			Embeddings: [][]float32{make([]float32, 1024)},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := NewSidecarEmbedder(SidecarConfig{URL: server.URL})

	_, err := embedder.Embed(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
}

// SC-13 error path: Sidecar not running returns helpful message.
func TestSidecarEmbedder_NotRunning(t *testing.T) {
	embedder := NewSidecarEmbedder(SidecarConfig{
		URL: "http://localhost:1", // Unreachable port
	})

	_, err := embedder.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("Embed should return error when sidecar is not running")
	}

	if !containsStr(err.Error(), "sidecar") {
		t.Errorf("error %q should mention sidecar", err.Error())
	}
}

// SC-14: Embedder selection driven by config model value.
func TestEmbedderSelection(t *testing.T) {
	tests := []struct {
		model      string
		expectType string
	}{
		{"voyage-4", "voyage"},
		{"voyage-4-large", "voyage"},
		{"voyage-4-nano", "sidecar"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			embedder, err := NewEmbedder(EmbedderConfig{
				Model:  tt.model,
				APIKey: "test-key", // Needed for voyage models
			})
			if err != nil {
				t.Fatalf("NewEmbedder(%s): %v", tt.model, err)
			}

			typ := embedder.Type()
			if typ != tt.expectType {
				t.Errorf("embedder type = %s, want %s", typ, tt.expectType)
			}
		})
	}
}

// SC-14 error path: Unknown model returns error at construction time.
func TestEmbedderSelection_UnknownModel(t *testing.T) {
	_, err := NewEmbedder(EmbedderConfig{
		Model: "unknown-model",
	})
	if err == nil {
		t.Fatal("NewEmbedder should return error for unknown model")
	}
}

// SC-14: Voyage models require API key at construction.
func TestEmbedderSelection_VoyageRequiresAPIKey(t *testing.T) {
	_, err := NewEmbedder(EmbedderConfig{
		Model:  "voyage-4",
		APIKey: "", // No API key
	})
	if err == nil {
		t.Fatal("NewEmbedder(voyage-4) should require API key")
	}

	if !containsStr(err.Error(), "VOYAGE_API_KEY") {
		t.Errorf("error %q should mention VOYAGE_API_KEY", err.Error())
	}
}

// SC-14: Nano model does not require API key.
func TestEmbedderSelection_NanoNoAPIKey(t *testing.T) {
	embedder, err := NewEmbedder(EmbedderConfig{
		Model: "voyage-4-nano",
		// No API key needed for nano
	})
	if err != nil {
		t.Fatalf("NewEmbedder(voyage-4-nano) should not require API key: %v", err)
	}

	if embedder.Type() != "sidecar" {
		t.Errorf("nano embedder type = %s, want sidecar", embedder.Type())
	}
}

// --- Types and stubs ---

type voyageRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	InputType string   `json:"input_type,omitempty"`
}

type voyageEmbedding struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type voyageResponse struct {
	Data []voyageEmbedding `json:"data"`
}

type sidecarRequest struct {
	Texts []string `json:"texts"`
}

type sidecarResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embedder is the interface for embedding providers.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Type() string
}

// VoyageConfig configures the Voyage AI embedder.
type VoyageConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// SidecarConfig configures the sidecar embedder.
type SidecarConfig struct {
	URL string
}

// EmbedderConfig configures embedder selection.
type EmbedderConfig struct {
	Model  string
	APIKey string
}

// VoyageEmbedder implements Embedder using Voyage AI API.
type VoyageEmbedder struct{}

// NewVoyageEmbedder creates a new Voyage AI embedder.
func NewVoyageEmbedder(cfg VoyageConfig) *VoyageEmbedder {
	return nil
}

// NewVoyageEmbedderFromEnv creates a Voyage embedder using env var for API key.
func NewVoyageEmbedderFromEnv(cfg VoyageConfig) (*VoyageEmbedder, error) {
	return nil, errors.New("not implemented")
}

func (e *VoyageEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errors.New("not implemented")
}

func (e *VoyageEmbedder) Type() string { return "voyage" }

// SidecarEmbedder implements Embedder using local FastAPI sidecar.
type SidecarEmbedder struct{}

// NewSidecarEmbedder creates a new sidecar embedder.
func NewSidecarEmbedder(cfg SidecarConfig) *SidecarEmbedder {
	return nil
}

func (e *SidecarEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errors.New("not implemented")
}

func (e *SidecarEmbedder) Type() string { return "sidecar" }

// NewEmbedder creates an embedder based on model configuration.
func NewEmbedder(cfg EmbedderConfig) (Embedder, error) {
	return nil, errors.New("not implemented")
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
