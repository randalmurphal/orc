package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SC-6: Config loads knowledge: section with all documented fields.
func TestKnowledgeConfig_FullYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `version: 1
knowledge:
  enabled: true
  backend: external
  docker:
    neo4j_port: 17687
    qdrant_port: 16334
    redis_port: 16379
    data_dir: /custom/knowledge/
  external:
    neo4j_uri: bolt://myhost:7687
    qdrant_uri: http://myhost:6334
    redis_uri: redis://myhost:6379
  indexing:
    embedding_model: voyage-4-large
worktree:
  enabled: true
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Verify all fields parsed
	if !cfg.Knowledge.Enabled {
		t.Error("Knowledge.Enabled should be true")
	}
	if cfg.Knowledge.Backend != "external" {
		t.Errorf("Knowledge.Backend = %s, want external", cfg.Knowledge.Backend)
	}

	// Docker config
	if cfg.Knowledge.Docker.Neo4jPort != 17687 {
		t.Errorf("Knowledge.Docker.Neo4jPort = %d, want 17687", cfg.Knowledge.Docker.Neo4jPort)
	}
	if cfg.Knowledge.Docker.QdrantPort != 16334 {
		t.Errorf("Knowledge.Docker.QdrantPort = %d, want 16334", cfg.Knowledge.Docker.QdrantPort)
	}
	if cfg.Knowledge.Docker.RedisPort != 16379 {
		t.Errorf("Knowledge.Docker.RedisPort = %d, want 16379", cfg.Knowledge.Docker.RedisPort)
	}
	if cfg.Knowledge.Docker.DataDir != "/custom/knowledge/" {
		t.Errorf("Knowledge.Docker.DataDir = %s, want /custom/knowledge/", cfg.Knowledge.Docker.DataDir)
	}

	// External config
	if cfg.Knowledge.External.Neo4jURI != "bolt://myhost:7687" {
		t.Errorf("Knowledge.External.Neo4jURI = %s, want bolt://myhost:7687", cfg.Knowledge.External.Neo4jURI)
	}
	if cfg.Knowledge.External.QdrantURI != "http://myhost:6334" {
		t.Errorf("Knowledge.External.QdrantURI = %s, want http://myhost:6334", cfg.Knowledge.External.QdrantURI)
	}
	if cfg.Knowledge.External.RedisURI != "redis://myhost:6379" {
		t.Errorf("Knowledge.External.RedisURI = %s, want redis://myhost:6379", cfg.Knowledge.External.RedisURI)
	}

	// Indexing config
	if cfg.Knowledge.Indexing.EmbeddingModel != "voyage-4-large" {
		t.Errorf("Knowledge.Indexing.EmbeddingModel = %s, want voyage-4-large", cfg.Knowledge.Indexing.EmbeddingModel)
	}
}

// SC-7: Config defaults are sensible when knowledge: section is absent.
func TestKnowledgeConfig_Defaults(t *testing.T) {
	cfg := Default()

	// Disabled by default
	if cfg.Knowledge.Enabled {
		t.Error("Knowledge.Enabled should default to false")
	}

	// Backend defaults to docker
	if cfg.Knowledge.Backend != "docker" {
		t.Errorf("Knowledge.Backend = %s, want docker", cfg.Knowledge.Backend)
	}

	// Port defaults
	if cfg.Knowledge.Docker.Neo4jPort != 7687 {
		t.Errorf("Knowledge.Docker.Neo4jPort = %d, want 7687", cfg.Knowledge.Docker.Neo4jPort)
	}
	if cfg.Knowledge.Docker.QdrantPort != 6334 {
		t.Errorf("Knowledge.Docker.QdrantPort = %d, want 6334", cfg.Knowledge.Docker.QdrantPort)
	}
	if cfg.Knowledge.Docker.RedisPort != 6379 {
		t.Errorf("Knowledge.Docker.RedisPort = %d, want 6379", cfg.Knowledge.Docker.RedisPort)
	}

	// Data dir default
	if cfg.Knowledge.Docker.DataDir != "~/.orc/knowledge/" {
		t.Errorf("Knowledge.Docker.DataDir = %s, want ~/.orc/knowledge/", cfg.Knowledge.Docker.DataDir)
	}

	// Embedding model default
	if cfg.Knowledge.Indexing.EmbeddingModel != "voyage-4" {
		t.Errorf("Knowledge.Indexing.EmbeddingModel = %s, want voyage-4", cfg.Knowledge.Indexing.EmbeddingModel)
	}
}

// SC-7: Verify defaults survive save/load round-trip when section absent.
func TestKnowledgeConfig_DefaultsSurviveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config without knowledge section
	yamlContent := `version: 1
worktree:
  enabled: true
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Should still have defaults
	if cfg.Knowledge.Enabled {
		t.Error("Knowledge.Enabled should default to false when section absent")
	}
	if cfg.Knowledge.Backend != "docker" {
		t.Errorf("Knowledge.Backend = %s, want docker (default)", cfg.Knowledge.Backend)
	}
}

// SC-8: Config validates embedding_model values.
func TestKnowledgeConfig_ValidateEmbeddingModel(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		expectError bool
		errContains string
	}{
		{
			name:        "valid voyage-4",
			model:       "voyage-4",
			expectError: false,
		},
		{
			name:        "valid voyage-4-large",
			model:       "voyage-4-large",
			expectError: false,
		},
		{
			name:        "valid voyage-4-nano",
			model:       "voyage-4-nano",
			expectError: false,
		},
		{
			name:        "empty model is valid (uses default)",
			model:       "",
			expectError: false,
		},
		{
			name:        "invalid model name",
			model:       "gpt-4",
			expectError: true,
			errContains: "voyage-4",
		},
		{
			name:        "invalid model with typo",
			model:       "voyage4",
			expectError: true,
			errContains: "voyage-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Knowledge.Indexing.EmbeddingModel = tt.model

			err := cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("expected validation error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

// SC-6: Malformed YAML returns parse error.
func TestKnowledgeConfig_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `version: 1
knowledge:
  enabled: [invalid yaml structure
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadFile(configPath)
	if err == nil {
		t.Error("LoadFile should fail with malformed YAML")
	}
}

// Edge case: Config with data_dir using ~ should expand correctly.
func TestKnowledgeConfig_DataDirTildeExpansion(t *testing.T) {
	cfg := Default()

	// Default data_dir uses ~
	if !strings.HasPrefix(cfg.Knowledge.Docker.DataDir, "~") {
		t.Errorf("Knowledge.Docker.DataDir = %s, should start with ~", cfg.Knowledge.Docker.DataDir)
	}

	// ExpandPath should expand it
	expanded := ExpandPath(cfg.Knowledge.Docker.DataDir)
	if strings.HasPrefix(expanded, "~") {
		t.Errorf("ExpandPath(%s) = %s, still starts with ~", cfg.Knowledge.Docker.DataDir, expanded)
	}
}
