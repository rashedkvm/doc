# Developing

## Quick Start (SQLite -- Zero Dependencies)

```bash
# Run the application (no database setup needed)
make run
```

That's it. The doc server starts with an embedded SQLite database at `./doc.db`. The schema is auto-applied at startup. CRD data is indexed on demand when users browse repositories.

## Quick Start (PostgreSQL -- Backward Compatible)

```bash
# 1. Set up credentials (one-time, file is gitignored)
cat > .env.local << 'EOF'
IMAGE_PROXY=registry-proxy.example.com
IMAGE_PROXY_USERNAME=your-username
IMAGE_PROXY_PASSWORD=your-token
EOF

# 2. Build and push images
REGISTRY=your-registry IMAGE_TAG=v1.0.0 make build-all push-all

# 3. Generate manifests and deploy (PostgreSQL mode)
REGISTRY=your-registry IMAGE_TAG=v1.0.0 make deploy-all-pg
```

## Prerequisites

- Go 1.24+
- Docker (only for PostgreSQL mode or building container images)
- kubectl configured with a Kubernetes cluster (for deployment)
- [Carvel tools](https://carvel.dev/) (ytt, kapp, kbld) (for Kubernetes deployment)
- Helm 3.x (only for PostgreSQL mode)

## Database Backends

The application supports two database backends, selected via the `DB_DRIVER` environment variable:

| Mode | DB_DRIVER | Infrastructure Required | Data Persistence |
|---|---|---|---|
| **SQLite (default)** | `sqlite` | None | Ephemeral (lost on restart) |
| **PostgreSQL** | `postgres` | PostgreSQL server | Persistent |

### SQLite Mode (Default)

No external database needed. The SQLite database file is created automatically and the schema is applied at startup.

```bash
# Local development
make run

# Or explicitly
DB_DRIVER=sqlite DB_DSN=./doc.db go run ./cmd/doc/main.go
```

### PostgreSQL Mode

For backward compatibility with existing deployments.

```bash
# Start PostgreSQL
make run-db

# Initialize schema
make init-db

# Run with PostgreSQL
make run-pg

# Or using PG_* env vars (auto-detected)
PG_USER=postgres PG_PASS=password PG_HOST=127.0.0.1 PG_PORT=5432 PG_DB=doc \
    go run ./cmd/doc/main.go
```

The application automatically detects PostgreSQL mode when `PG_HOST` is set (even without `DB_DRIVER`).

## Configuration Reference

The application is configured through two complementary mechanisms:

1. **Values file** (`config/values.yml`) — the single source of truth for all settings. Used by ytt to generate Kubernetes manifests and by the Go binary to load providers at runtime.
2. **Environment variables** — consumed directly by the Go binary at runtime. In Kubernetes, these are injected from ConfigMaps and Secrets that ytt generates from the values file.

The table below lists every configuration key, where it originates, and where it is consumed.

### Environment Variables (consumed by Go code at runtime)

| Variable | Default | Go Source | Description |
|---|---|---|---|
| `CONFIG_FILE` | _(unset — falls back to github.com only)_ | `cmd/doc/main.go`, `cmd/gitter/main.go` | Path to YAML file containing the `providers` list. In K8s this is mounted from a ConfigMap. |
| `APP_PORT` | `5000` | `cmd/doc/main.go` | HTTP listen port for the Doc server. |
| `DB_DRIVER` | `sqlite` | `pkg/store/factory.go` | Database backend: `sqlite` or `postgres`. |
| `DB_DSN` | `./doc.db` | `pkg/store/factory.go` | SQLite file path or PostgreSQL connection string. |
| `PG_HOST` | -- | `pkg/store/factory.go` | PostgreSQL host. If set (even without `DB_DRIVER`), triggers legacy postgres auto-detection. |
| `PG_USER` | -- | `pkg/store/factory.go` | PostgreSQL username (legacy). |
| `PG_PASS` | -- | `pkg/store/factory.go` | PostgreSQL password (legacy). |
| `PG_PORT` | -- | `pkg/store/factory.go` | PostgreSQL port (legacy). |
| `PG_DB` | -- | `pkg/store/factory.go` | PostgreSQL database name (legacy). |
| `ANALYTICS` | `false` | `cmd/doc/main.go` | Enable Google Analytics (`true`/`false`). |
| `IS_DEV` | _(unset)_ | `cmd/doc/main.go` | Enable template hot-reload (`true` to enable). |
| _`<auth_secret>`_ (e.g. `GHE_TOKEN`) | -- | `pkg/provider/config.go` | Token for GitHub Enterprise providers. The name is defined by the `auth_secret` field in the providers config; the value must be an env var. |

### Values File Keys (`config/values.yml`)

The values file is consumed by **ytt** to generate Kubernetes manifests. The `providers` section is also read directly by the Go binary at runtime via `CONFIG_FILE`.

| Key | Default | Purpose |
|---|---|---|
| `db_driver` | `sqlite` | Maps to `DB_DRIVER` env var in the ConfigMap. |
| `db_dsn` | `/data/doc.db` | Maps to `DB_DSN` env var in the ConfigMap. |
| `app_port` | `5000` | Maps to `APP_PORT` env var; also sets container/service ports. |
| `namespace` | `doc-system` | Kubernetes namespace for all resources. |
| `analytics` | `false` | Maps to `ANALYTICS` env var in the ConfigMap. |
| `is_dev` | `false` | Maps to `IS_DEV` env var in the ConfigMap. |
| `providers` | `[{host: github.com, type: github}]` | List of Git host providers. Mounted as a ConfigMap file for the Go binary. |
| `dbname` | `doc` | PostgreSQL database name (maps to `PG_DB`, only when `db_driver` is `postgres`). |
| `dbuser` | `postgres` | PostgreSQL user (maps to `PG_USER`). |
| `dbpwd` | `password` | PostgreSQL password (maps to `PG_PASS` via a Secret). |
| `dbhost` | `pgsql-postgresql.postgres.svc.cluster.local` | PostgreSQL host (maps to `PG_HOST`). |
| `dbport` | `5432` | PostgreSQL port (maps to `PG_PORT`). |
| `registry` | `docker.io` | Container image registry prefix. |
| `image_tag` | `latest` | Container image tag. |
| `gateway_enabled` | `true` | Include Gateway and HTTPRoute resources in generated manifests. |
| `gateway_class_name` | `cloud-provider-kind` | GatewayClass name for the Gateway resource. |
| `gateway_port` | `80` | External listener port on the Gateway. |
| `image_proxy` | _(empty)_ | Proxy registry URL for pulling base images. |
| `image_proxy_username` | _(empty)_ | Proxy registry username. |
| `image_proxy_password` | _(empty)_ | Proxy registry password. |

### How Config Flows

```
config/values.yml
    │
    ├──▶ ytt (make dist) ──▶ ConfigMap doc-config     ──▶ env vars in Pod
    │                    ──▶ ConfigMap doc-providers   ──▶ mounted file → CONFIG_FILE
    │                    ──▶ Secret doc-db-secret      ──▶ PG_PASS in Pod
    │
    └──▶ make run        ──▶ CONFIG_FILE=$(VALUES_FILE) passed directly to Go binary
```

### Passing Sensitive Values

Sensitive values (registry credentials, passwords, tokens) are managed via a `.env.local` file that is automatically loaded by the Makefile.

1. Create `.env.local` in the project root (this file is gitignored):

```bash
cat > .env.local << 'EOF'
IMAGE_PROXY=registry-proxy.example.com
IMAGE_PROXY_USERNAME=your-username
IMAGE_PROXY_PASSWORD=your-token
DB_PASSWORD=your-db-password
GHE_TOKEN=ghp_xxxxxxxxxxxx
EOF
```

2. All make commands automatically pick up these values:

```bash
make run         # GHE_TOKEN available to the Go binary
make dist        # Generates manifests with proxy config
make deploy      # Deploys with all credentials
```

### Using Different Environments

```bash
# Local development - SQLite (default)
make run

# Local development - PostgreSQL
make run-pg

# Override registry and tag for your images
REGISTRY=your-registry IMAGE_TAG=v1.0.0 make build-all push-all deploy

# Use a different values file
VALUES_FILE=config/values-staging.yml make deploy
```

### Repository Providers

The application supports multiple Git hosting platforms through the `RepoProvider` abstraction (`pkg/provider`). At startup, the `providers` list is loaded from the YAML file pointed to by `CONFIG_FILE`. If the file is missing or contains no providers, only `github.com` (public, no auth) is registered as a fallback.

#### Adding GitHub Enterprise

1. Add an entry to `providers` in `config/values.yml`:

```yaml
providers:
  - host: "github.com"
    type: "github"
  - host: "github.mycompany.com"
    type: "github-enterprise"
    auth_secret: "GHE_TOKEN"
```

2. Set the token in `.env.local` (or as an environment variable):

```bash
echo 'GHE_TOKEN=ghp_xxxxxxxxxxxx' >> .env.local
```

3. Run the application — both hosts are now accessible:

```bash
make run
# Browse to http://localhost:5000/github.mycompany.com/org/repo
```

#### URL Format

All routes use the pattern `/{host}/{org}/{repo}`. Existing `github.com/...` URLs continue to work unchanged.

### Custom Values Files

For different environments, create custom values files:

```bash
cp config/values.yml config/values-staging.yml

# Edit with your settings, then use it
VALUES_FILE=config/values-staging.yml make deploy
```

**Note:** Custom values files matching `config/values-*.yml` are gitignored to prevent accidentally committing sensitive data.

## Local Development

### SQLite Mode (Recommended)

```bash
# Start the application (schema auto-applied, zero setup)
make run

# Clean up
make clean
```

### PostgreSQL Mode

```bash
# Start PostgreSQL container
make run-db

# Initialize the database schema
make init-db

# Run the application
make run-pg

# Optionally run standalone gitter (backward compat)
make run-gitter

# Clean up
make clean-sandbox-pg
```

### Available Make Targets

Run `make help` to see all available targets.

## Kubernetes Deployment

### Build and Push Docker Images

```bash
make build-all
make push-all
```

### Deploy (SQLite Mode -- Default)

```bash
# Deploy application only (no database needed)
make deploy
```

### Deploy (PostgreSQL Mode)

```bash
# Deploy database and application
make deploy-all-pg
```

### Accessing the Service (Gateway API)

The deployment includes [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/) resources (`Gateway` + `HTTPRoute`) to expose the doc service externally. This is enabled by default (`gateway_enabled: true` in values).

#### Local Development with KIND

For local Kubernetes development using [KIND](https://kind.sigs.k8s.io/), install [cloud-provider-kind](https://github.com/kubernetes-sigs/cloud-provider-kind) which implements the Gateway API:

```bash
# Install cloud-provider-kind
go install sigs.k8s.io/cloud-provider-kind@latest

# Start cloud-provider-kind (keep running in a separate terminal)
sudo cloud-provider-kind
```

After deploying, check the Gateway's external IP:

```bash
kubectl get gateway -n doc-system
# NAME          CLASS                 ADDRESS        PROGRAMMED   AGE
# doc-gateway   cloud-provider-kind   192.168.8.5    True         30s
```

On macOS/Windows, use `--enable-lb-port-mapping` when starting cloud-provider-kind, then access via `localhost` using the mapped port shown in `docker ps`.

#### Disabling the Gateway

If you have your own ingress solution or don't need external access:

```yaml
# In your values file
gateway_enabled: false
```

Or via the command line:

```bash
ytt ... -v gateway_enabled=false
```

#### Customizing the GatewayClass

For production deployments with a different Gateway controller (e.g., Contour, Istio, Envoy Gateway):

```yaml
gateway_class_name: "contour"  # or "istio", "eg", etc.
gateway_port: 443
```

### Full Release Workflow

```bash
# Uses default registry from config/values.yml
make release

# With custom registry and tag
REGISTRY=your-registry IMAGE_TAG=v1.0.0 make release
```

### Using an Image Registry Proxy

1. Add proxy credentials to `.env.local`:
   ```bash
   IMAGE_PROXY=registry-proxy.example.com
   IMAGE_PROXY_USERNAME=your-username
   IMAGE_PROXY_PASSWORD=your-token
   ```

2. Run any make target - proxy config is automatically applied:
   ```bash
   make dist      # App manifests include imagePullSecrets
   make db-gen    # Database manifests use proxy for PostgreSQL image
   ```
