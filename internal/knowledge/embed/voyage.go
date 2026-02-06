package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	voyageBatchSize  = 64
	voyageDefaultURL = "https://api.voyageai.com/v1/embeddings"
	maxRetries       = 3
)

// VoyageConfig configures the Voyage AI embedder.
type VoyageConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

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

// VoyageEmbedder implements Embedder using Voyage AI API.
type VoyageEmbedder struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewVoyageEmbedder creates a new Voyage AI embedder.
func NewVoyageEmbedder(cfg VoyageConfig) *VoyageEmbedder {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = voyageDefaultURL
	}
	return &VoyageEmbedder{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// NewVoyageEmbedderFromEnv creates a Voyage embedder using env var for API key.
func NewVoyageEmbedderFromEnv(cfg VoyageConfig) (*VoyageEmbedder, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("VOYAGE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("voyage embedder requires VOYAGE_API_KEY environment variable")
	}
	cfg.APIKey = apiKey
	return NewVoyageEmbedder(cfg), nil
}

// Embed embeds texts using Voyage AI API with batch splitting.
func (e *VoyageEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	var allVectors [][]float32
	for i := 0; i < len(texts); i += voyageBatchSize {
		end := i + voyageBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		vectors, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		allVectors = append(allVectors, vectors...)
	}
	return allVectors, nil
}

func (e *VoyageEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := voyageRequest{
		Model: e.model,
		Input: texts,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var resp *http.Response
	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.apiKey)

		resp, err = e.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("voyage API request: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}
		break
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voyage API error: status %d", resp.StatusCode)
	}

	var voyageResp voyageResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("decode voyage response: %w", err)
	}

	vectors := make([][]float32, len(voyageResp.Data))
	for _, d := range voyageResp.Data {
		vectors[d.Index] = d.Embedding
	}
	return vectors, nil
}

// Type returns the embedder type.
func (e *VoyageEmbedder) Type() string { return "voyage" }
