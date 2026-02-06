package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const sidecarDefaultURL = "http://localhost:8100/embed"

// SidecarConfig configures the sidecar embedder.
type SidecarConfig struct {
	URL string
}

type sidecarRequest struct {
	Texts []string `json:"texts"`
}

type sidecarResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// SidecarEmbedder implements Embedder using local FastAPI sidecar.
type SidecarEmbedder struct {
	url    string
	client *http.Client
}

// NewSidecarEmbedder creates a new sidecar embedder.
func NewSidecarEmbedder(cfg SidecarConfig) *SidecarEmbedder {
	url := cfg.URL
	if url == "" {
		url = sidecarDefaultURL
	}
	return &SidecarEmbedder{
		url:    url,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Embed embeds texts using the local sidecar service.
func (e *SidecarEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := sidecarRequest{Texts: texts}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal sidecar request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sidecar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header for sidecar

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sidecar embed request failed (is the sidecar running?): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar API error: status %d", resp.StatusCode)
	}

	var sidecarResp sidecarResponse
	if err := json.NewDecoder(resp.Body).Decode(&sidecarResp); err != nil {
		return nil, fmt.Errorf("decode sidecar response: %w", err)
	}

	return sidecarResp.Embeddings, nil
}

// Type returns the embedder type.
func (e *SidecarEmbedder) Type() string { return "sidecar" }
