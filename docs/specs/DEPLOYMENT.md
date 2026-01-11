# Deployment Specification

> Simple deployment for DevOps: single container, auto-TLS, no complex infrastructure.

## Design Principles

1. **Single container** - No microservices, no sidecars
2. **Zero-config TLS** - Caddy handles certificates automatically
3. **Works behind proxies** - Corporate proxy compatible
4. **Air-gap ready** - Offline deployment supported
5. **No magic** - Clear, predictable behavior

---

## Deployment Targets

| Target | Database | Infrastructure | Complexity |
|--------|----------|----------------|------------|
| Solo dev | SQLite | `orc serve` | Minimal |
| Solo remote | SQLite | Container | Low |
| Small team | SQLite + Litestream | Container + Caddy | Low |
| Team server | PostgreSQL | Container + Caddy + Postgres | Medium |
| Enterprise | PostgreSQL | K8s-ready (optional) | Medium |

---

## Solo Developer

### Local Development

```bash
# Just run orc
cd my-project
orc init
orc serve
# → http://localhost:8080
```

### Remote Access (SSH Tunnel)

```bash
# On server
orc serve

# On laptop (SSH tunnel)
ssh -L 8080:localhost:8080 server
# → http://localhost:8080
```

No auth needed - localhost binding.

---

## Single Container Deployment

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o orc ./cmd/orc

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    git \
    bash

# Create non-root user
RUN adduser -D -u 1000 orc
USER orc
WORKDIR /home/orc

# Copy binary
COPY --from=builder /build/orc /usr/local/bin/orc

# Copy default templates
COPY --from=builder /build/templates /home/orc/.orc-templates/

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/health || exit 1

# Default command
ENTRYPOINT ["orc"]
CMD ["serve", "--host", "0.0.0.0"]
```

### Minimal docker-compose

```yaml
# docker-compose.yml
version: "3.9"

services:
  orc:
    image: ghcr.io/randalmurphal/orc:latest
    container_name: orc
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - orc_data:/home/orc/.orc
      - ./projects:/projects:ro  # Mount project directories
    environment:
      - ORC_AUTH_TOKEN=${ORC_AUTH_TOKEN}  # Required for remote access
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  orc_data:
```

---

## Production Deployment with Caddy

### Why Caddy?

| Feature | Caddy | Traefik | Nginx |
|---------|-------|---------|-------|
| Auto HTTPS | Zero-config | Requires setup | Manual |
| Config format | Caddyfile (simple) | YAML (complex) | nginx.conf |
| Certificate renewal | Automatic | Automatic | Manual |
| Learning curve | Low | Medium | Low |

### docker-compose with Caddy

```yaml
# docker-compose.prod.yml
version: "3.9"

services:
  caddy:
    image: caddy:2-alpine
    container_name: orc-proxy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      orc:
        condition: service_healthy

  orc:
    image: ghcr.io/randalmurphal/orc:latest
    container_name: orc
    restart: unless-stopped
    expose:
      - "8080"
    volumes:
      - orc_data:/home/orc/.orc
    environment:
      - ORC_BASE_URL=https://${DOMAIN}
      - ORC_AUTH_ENABLED=true
      - ORC_AUTH_TYPE=token
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  caddy_data:
  caddy_config:
  orc_data:
```

### Caddyfile

```
{$DOMAIN} {
    # Automatic HTTPS

    # API and WebSocket
    reverse_proxy orc:8080 {
        # WebSocket support
        header_up Connection {>Connection}
        header_up Upgrade {>Upgrade}
    }

    # Compression
    encode gzip zstd

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        Referrer-Policy strict-origin-when-cross-origin
        -Server
    }

    # Logging
    log {
        output stdout
        format json
    }
}
```

### .env file

```bash
DOMAIN=orc.company.com
ORC_AUTH_TOKEN=your-secure-token-here
```

---

## Team Server with PostgreSQL

### docker-compose with Postgres

```yaml
# docker-compose.team.yml
version: "3.9"

services:
  caddy:
    image: caddy:2-alpine
    container_name: orc-proxy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
    depends_on:
      orc:
        condition: service_healthy

  orc:
    image: ghcr.io/randalmurphal/orc:latest
    container_name: orc
    restart: unless-stopped
    expose:
      - "8080"
    volumes:
      - orc_data:/home/orc/.orc
    environment:
      - DATABASE_URL=postgres://orc:${POSTGRES_PASSWORD}@db:5432/orc?sslmode=disable
      - ORC_BASE_URL=https://${DOMAIN}
      - ORC_AUTH_ENABLED=true
      - ORC_AUTH_TYPE=oidc
      - ORC_OIDC_ISSUER=${OIDC_ISSUER}
      - ORC_OIDC_CLIENT_ID=${OIDC_CLIENT_ID}
      - ORC_OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET}
    depends_on:
      db:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  db:
    image: postgres:16-alpine
    container_name: orc-db
    restart: unless-stopped
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=orc
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=orc
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U orc"]
      interval: 10s
      timeout: 5s
      retries: 5

  backup:
    image: prodrigestivill/postgres-backup-local:16
    container_name: orc-backup
    restart: unless-stopped
    volumes:
      - ./backups:/backups
    environment:
      - POSTGRES_HOST=db
      - POSTGRES_DB=orc
      - POSTGRES_USER=orc
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - SCHEDULE=@daily
      - BACKUP_KEEP_DAYS=7
      - BACKUP_KEEP_WEEKS=4
      - BACKUP_KEEP_MONTHS=6
    depends_on:
      db:
        condition: service_healthy

volumes:
  caddy_data:
  orc_data:
  postgres_data:
```

### .env for team

```bash
DOMAIN=orc.company.com
POSTGRES_PASSWORD=secure-password-here

# OIDC (Google example)
OIDC_ISSUER=https://accounts.google.com
OIDC_CLIENT_ID=your-client-id.apps.googleusercontent.com
OIDC_CLIENT_SECRET=your-client-secret
```

---

## SQLite with Litestream Backup

For teams that want SQLite simplicity with cloud backup:

```yaml
# docker-compose.litestream.yml
version: "3.9"

services:
  orc:
    image: ghcr.io/randalmurphal/orc:litestream
    container_name: orc
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - orc_data:/home/orc/.orc
      - ./litestream.yml:/etc/litestream.yml:ro
    environment:
      - LITESTREAM_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
      - LITESTREAM_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}

volumes:
  orc_data:
```

### litestream.yml

```yaml
dbs:
  - path: /home/orc/.orc/orc.db
    replicas:
      - type: s3
        bucket: orc-backups
        path: orc.db
        region: us-east-1
        sync-interval: 1m
```

### Litestream Dockerfile

```dockerfile
FROM ghcr.io/randalmurphal/orc:latest

# Add litestream
ADD https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz /tmp/
RUN tar -xzf /tmp/litestream-v0.3.13-linux-amd64.tar.gz -C /usr/local/bin/

# Wrapper script
COPY <<EOF /usr/local/bin/start.sh
#!/bin/sh
exec litestream replicate -exec "orc serve --host 0.0.0.0"
EOF
RUN chmod +x /usr/local/bin/start.sh

ENTRYPOINT ["/usr/local/bin/start.sh"]
```

---

## Kubernetes (Optional)

For teams that already use Kubernetes:

### deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orc
  labels:
    app: orc
spec:
  replicas: 1  # Single replica for SQLite, scale with Postgres
  selector:
    matchLabels:
      app: orc
  template:
    metadata:
      labels:
        app: orc
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
      containers:
      - name: orc
        image: ghcr.io/randalmurphal/orc:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: orc-secrets
              key: database-url
        - name: ORC_AUTH_ENABLED
          value: "true"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        - name: data
          mountPath: /home/orc/.orc
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: orc-data
---
apiVersion: v1
kind: Service
metadata:
  name: orc
spec:
  selector:
    app: orc
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: orc
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - orc.company.com
    secretName: orc-tls
  rules:
  - host: orc.company.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: orc
            port:
              number: 80
```

---

## Air-Gapped Deployment

### Export Images

```bash
# On connected machine
docker pull ghcr.io/randalmurphal/orc:latest
docker pull postgres:16-alpine
docker pull caddy:2-alpine

# Save to tarballs
docker save ghcr.io/randalmurphal/orc:latest | gzip > orc.tar.gz
docker save postgres:16-alpine | gzip > postgres.tar.gz
docker save caddy:2-alpine | gzip > caddy.tar.gz
```

### Import Images

```bash
# On air-gapped machine
gunzip -c orc.tar.gz | docker load
gunzip -c postgres.tar.gz | docker load
gunzip -c caddy.tar.gz | docker load
```

### Self-Signed Certificates

```
# Caddyfile for air-gapped
{
    auto_https off
}

orc.internal {
    tls /certs/orc.crt /certs/orc.key
    reverse_proxy orc:8080
}
```

---

## Health Checks & Monitoring

### Health Endpoints

| Endpoint | Purpose | K8s Probe |
|----------|---------|-----------|
| `/api/health` | Basic liveness | livenessProbe |
| `/api/health/ready` | Ready to serve | readinessProbe |
| `/api/metrics` | Prometheus metrics | - |

### Health Check Implementation

```go
// internal/api/handlers/health.go
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "status":  "ok",
        "version": version.Version,
    })
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{}

    // Database check
    if err := h.db.Ping(); err != nil {
        checks["database"] = "error: " + err.Error()
    } else {
        checks["database"] = "ok"
    }

    // Determine overall status
    status := "ok"
    for _, v := range checks {
        if v != "ok" {
            status = "degraded"
            break
        }
    }

    w.Header().Set("Content-Type", "application/json")
    if status != "ok" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    json.NewEncoder(w).Encode(map[string]any{
        "status":  status,
        "checks":  checks,
        "version": version.Version,
    })
}
```

### Prometheus Metrics

```go
// internal/api/metrics.go
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orc_http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "orc_http_request_duration_seconds",
            Help:    "HTTP request duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    tasksActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "orc_tasks_active",
        Help: "Number of currently running tasks",
    })

    tokensUsedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orc_tokens_used_total",
            Help: "Total tokens used",
        },
        []string{"model", "type"},  // type: input, output
    )
)
```

---

## Backup & Restore

### SQLite Backup

```bash
# Manual backup
docker exec orc sqlite3 /home/orc/.orc/orc.db ".backup /backup/orc-$(date +%Y%m%d).db"

# Or using VACUUM INTO (safer)
docker exec orc sqlite3 /home/orc/.orc/orc.db "VACUUM INTO '/backup/orc-$(date +%Y%m%d).db'"
```

### PostgreSQL Backup

```bash
# Manual backup
docker exec orc-db pg_dump -U orc orc | gzip > backup-$(date +%Y%m%d).sql.gz

# Restore
gunzip -c backup-20260110.sql.gz | docker exec -i orc-db psql -U orc orc
```

### Automated Backup Script

```bash
#!/bin/bash
# /etc/cron.daily/orc-backup

BACKUP_DIR=/var/backups/orc
DATE=$(date +%Y%m%d)

# Create backup directory
mkdir -p $BACKUP_DIR

# Backup database
if [ -n "$DATABASE_URL" ]; then
    # PostgreSQL
    docker exec orc-db pg_dump -U orc orc | gzip > $BACKUP_DIR/orc-$DATE.sql.gz
else
    # SQLite
    docker exec orc sqlite3 /home/orc/.orc/orc.db "VACUUM INTO '/tmp/backup.db'"
    docker cp orc:/tmp/backup.db $BACKUP_DIR/orc-$DATE.db
fi

# Cleanup old backups (keep 30 days)
find $BACKUP_DIR -name "orc-*.sql.gz" -mtime +30 -delete
find $BACKUP_DIR -name "orc-*.db" -mtime +30 -delete
```

---

## Upgrade Process

### Standard Upgrade

```bash
# Pull new version
docker pull ghcr.io/randalmurphal/orc:v1.2.0

# Backup first
docker exec orc-db pg_dump -U orc orc > backup-before-upgrade.sql

# Stop and update
docker compose down
docker compose up -d

# Verify
curl https://orc.company.com/api/health
```

### Rollback

```bash
# Stop current
docker compose down

# Revert image
docker pull ghcr.io/randalmurphal/orc:v1.1.0

# Restore database if needed
cat backup-before-upgrade.sql | docker exec -i orc-db psql -U orc orc

# Restart
docker compose up -d
```

---

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | Database connection string | SQLite in ~/.orc |
| `ORC_HOST` | Listen address | 127.0.0.1 |
| `ORC_PORT` | Listen port | 8080 |
| `ORC_BASE_URL` | Public URL for callbacks | http://localhost:8080 |
| `ORC_AUTH_ENABLED` | Enable authentication | false |
| `ORC_AUTH_TYPE` | Auth type (token, oidc) | token |
| `ORC_AUTH_TOKEN` | Bearer token for auth | - |
| `ORC_OIDC_ISSUER` | OIDC issuer URL | - |
| `ORC_OIDC_CLIENT_ID` | OIDC client ID | - |
| `ORC_OIDC_CLIENT_SECRET` | OIDC client secret | - |
| `ORC_LOG_LEVEL` | Log level | info |
| `ORC_LOG_FORMAT` | Log format (json, text) | json |

---

## Troubleshooting

### Container won't start

```bash
# Check logs
docker logs orc

# Common issues:
# - Database connection failed: check DATABASE_URL
# - Permission denied: ensure volume permissions
# - Port in use: check for conflicting services
```

### Health check failing

```bash
# Check detailed health
curl http://localhost:8080/api/health/ready

# Common issues:
# - Database not ready: wait or check connection
# - Disk full: check volume space
```

### WebSocket not connecting

```bash
# Check Caddy config includes WebSocket headers
# Check browser console for errors
# Verify wss:// URL matches TLS config
```

### Performance issues

```bash
# Check resource usage
docker stats orc

# SQLite: ensure WAL mode
sqlite3 orc.db "PRAGMA journal_mode;"

# Postgres: check connection pool
docker exec orc-db psql -U orc -c "SELECT count(*) FROM pg_stat_activity;"
```
