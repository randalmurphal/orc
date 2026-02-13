// Package embed provides text embedding providers for the knowledge layer.
package embed

import (
	"context"
	"fmt"
)

// Embedder is the interface for embedding providers.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Type() string
}

// EmbedderConfig configures embedder selection.
type EmbedderConfig struct {
	Model  string
	APIKey string
}

// NewEmbedder creates an embedder based on model configuration.
func NewEmbedder(cfg EmbedderConfig) (Embedder, error) {
	switch cfg.Model {
	case "voyage-4", "voyage-4-large":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("voyage embedder requires VOYAGE_API_KEY environment variable")
		}
		return NewVoyageEmbedder(VoyageConfig{
			APIKey: cfg.APIKey,
			Model:  cfg.Model,
		}), nil
	case "voyage-4-nano":
		return NewSidecarEmbedder(SidecarConfig{}), nil
	default:
		return nil, fmt.Errorf("unknown embedding model: %s", cfg.Model)
	}
}
