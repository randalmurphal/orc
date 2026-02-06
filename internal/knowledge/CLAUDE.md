# Knowledge Package

Persistent memory layer for orc. Manages Docker-based infrastructure (Neo4j, Qdrant, Redis), provides graph/vector/cache stores, embedding generation, and multi-signal retrieval.

## Sub-Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `knowledge` (root) | Service orchestration, lifecycle, health, query API | `Service`, `Components`, `QueryComponents`, `TaskContext` |
| `retrieve/` | Multi-signal search pipeline with presets and scoring | `Pipeline`, `Stage`, `Scorer`, `PresetDeps`, `PipelineResult` |
| `infra/` | Docker container lifecycle (start/stop/health) | `Manager`, `DockerClient`, `Config`, `Health` |
| `store/` | Storage backends (graph, vector, cache) | `GraphStore`, `VectorStore`, `CacheStore` |
| `embed/` | Text embedding providers | `Embedder`, `VoyageEmbedder`, `SidecarEmbedder` |

## Architecture

### Components Interface

The `Components` interface (`knowledge.go:16`) decouples the `Service` from concrete infrastructure and store implementations. This enables testing without Docker, Neo4j, Qdrant, or Redis.

```go
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
```

### Startup/Shutdown Order

Start and Stop follow strict ordering with rollback on failure:

| Step | Start Order | Stop Order |
|------|-------------|------------|
| 1 | `infra.Start` (containers) | `cache.Close` |
| 2 | `graph.Connect` (Neo4j) | `vector.Close` |
| 3 | `vector.Connect` (Qdrant) | `graph.Close` |
| 4 | `cache.Connect` (Redis) | `infra.Stop` (containers) |

If any Start step fails, already-completed steps are cleaned up in reverse order (`knowledge.go:93`).

### Infrastructure Backends

The `infra.Manager` supports two backends configured via `Config.Backend`:

| Backend | Behavior |
|---------|----------|
| `docker` (default) | Manages containers via `DockerClient`. Reuses existing healthy containers, restarts unhealthy ones, rolls back newly-started containers on failure. |
| `external` | Validates connectivity to pre-existing services via `HealthCheckFunc`. No container management. |

### Stores

All stores follow the same pattern: interface-based driver/client, functional options for injection, `Connect`/`Close` lifecycle.

| Store | Backing Service | Driver Interface | Purpose |
|-------|----------------|------------------|---------|
| `GraphStore` | Neo4j | `Neo4jDriver` | Nodes, relationships, Cypher queries |
| `VectorStore` | Qdrant | `QdrantClient` | Embedding storage, similarity search |
| `CacheStore` | Redis | `RedisClient` | Key/value with TTL (embeddings: 30d, queries: 10m) |

### Embedding Providers

Selected by `knowledge.indexing.embedding_model` in config:

| Model | Provider | Type | Notes |
|-------|----------|------|-------|
| `voyage-4` | Voyage AI API | Remote | Requires `VOYAGE_API_KEY`. Batch size: 64. Retries on 429. |
| `voyage-4-large` | Voyage AI API | Remote | Same as above, larger model. |
| `voyage-4-nano` | Local sidecar | Local | FastAPI at `localhost:8100/embed`. No auth. |

Factory: `embed.NewEmbedder(cfg)` (`embed/embedder.go:22`).

### Retrieval Pipeline (`retrieve/`)

Multi-signal search pipeline that combines vector similarity, graph traversal, temporal decay, and PageRank into a scored result set.

**Pipeline stages** (implement `Stage` interface at `retrieve/types.go:57`):

| Stage | Signal | Behavior |
|-------|--------|----------|
| `SemanticStage` | `semantic` | Embed query → vector search → initial candidates |
| `HydrateStage` | - | Load full document content from graph store |
| `GraphExpansionStage` | `graph` | Add related docs with depth decay: `1/(1+depth)` |
| `TemporalDecayStage` | `temporal` | Exponential decay with configurable half-life (default 7d) |
| `PageRankStage` | `pagerank` | Graph centrality as soft signal (graceful on failure) |
| `RerankStage` | `rerank` | LLM/cross-encoder reranking of top-K candidates |

**Presets** (factory functions at `retrieve/presets.go`):

| Preset | Stages | Use Case |
|--------|--------|----------|
| `standard` | semantic → hydrate → graph → temporal → pagerank → rerank | Default full pipeline |
| `fast` | semantic → hydrate | Quick lookups, task context |
| `deep` | Same as standard, higher limits | Thorough research |
| `graph_first` | semantic → hydrate → graph → pagerank | Structure-focused queries |
| `recency` | semantic → hydrate → temporal | Recent changes focus |

**Scoring:** `WeightedScorer` computes normalized weighted sum of signals. Each preset defines its own weight map. Unknown presets return an error (no fallback).

**Service integration** (`service_query.go`):
- `Query()` — Execute pipeline with preset and options
- `QueryForTask()` — Structured task context using `fast` preset with summary-only
- `EnrichBrief()` — Add graph-derived sections (patterns, hot files, known issues) to briefs

## Configuration

Defined in `config.KnowledgeConfig` (`internal/config/config_types.go:1263`):

```yaml
knowledge:
  enabled: false              # Must be explicitly enabled
  backend: docker             # "docker" or "external"
  docker:
    neo4j_port: 7687
    qdrant_port: 6334
    redis_port: 6379
    data_dir: ~/.orc/knowledge/
  external:
    neo4j_uri: bolt://host:7687
    qdrant_uri: http://host:6334
    redis_uri: redis://host:6379
  indexing:
    embedding_model: voyage-4
```

## CLI Commands

| Command | Purpose | Status |
|---------|---------|--------|
| `orc knowledge start` | Start infrastructure containers | Stub |
| `orc knowledge stop` | Stop infrastructure containers | Stub |
| `orc knowledge status` | Show per-service health | Stub |
| `orc knowledge query <query>` | Search knowledge graph | Wired (prints availability message) |

Query flags: `--preset` (standard/fast/deep/graph_first/recency), `--limit`, `--summary`

## Testing Pattern

The `Components` interface and per-store driver interfaces enable full unit testing without external dependencies. Inject mocks via functional options:

```go
svc := NewService(cfg, WithComponents(&mockComponents{}))
store := NewGraphStore(WithNeo4jDriver(&mockDriver{}))
mgr := NewManager(cfg, WithDockerClient(&mockDocker{}))
```

Tests: `knowledge_test.go` (lifecycle), `service_query_test.go` (query/enrichment), `retrieve/*_test.go` (pipeline/stages/presets), `*_integration_test.go` (wiring).
