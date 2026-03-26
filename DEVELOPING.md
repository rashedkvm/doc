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

## Configuration

All environment-specific settings are managed through a single values file. The default file is `config/values.yml` for local development.

### Key Configuration Values

```yaml
# Database driver: "sqlite" (default) or "postgres"
db_driver: "sqlite"
db_dsn: "/data/doc.db"

# Repository host providers
# By default, only github.com (public) is registered.
# Add entries to support GitHub Enterprise or other hosts.
providers:
  - host: "github.com"
    type: "github"
  # - host: "github.mycompany.com"
  #   type: "github-enterprise"
  #   auth_secret: "GHE_TOKEN"   # env var name holding the token

# PostgreSQL settings (only used when db_driver is "postgres")
dbname: doc
dbuser: postgres
dbpwd: password
dbhost: pgsql-postgresql.postgres.svc.cluster.local
dbport: "5432"

# Application settings
app_port: 5000
namespace: doc-system
analytics: "false"
is_dev: "true"
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `CONFIG_FILE` | `config/values.yml` | Path to the YAML values file. The `providers` list is loaded from this file at startup. If unset or the file is missing, only `github.com` (public) is registered. |
| `APP_PORT` | `5000` | HTTP listen port for the Doc server |
| `DB_DRIVER` | `sqlite` | Database backend: `sqlite` or `postgres` |
| `DB_DSN` | `./doc.db` | SQLite file path or PostgreSQL connection string |
| `PG_USER` | -- | PostgreSQL user (legacy, triggers auto-detection) |
| `PG_PASS` | -- | PostgreSQL password (legacy) |
| `PG_HOST` | -- | PostgreSQL host (legacy, if set enables postgres mode) |
| `PG_PORT` | -- | PostgreSQL port (legacy) |
| `PG_DB` | -- | PostgreSQL database name (legacy) |
| `ANALYTICS` | `false` | Enable Google Analytics |
| `IS_DEV` | `true` | Enable development mode (template reloading) |
| `GHE_TOKEN` | -- | Token for GitHub Enterprise auth (referenced by `auth_secret` in provider config) |

### Passing Sensitive Values

Sensitive values (registry credentials, passwords) are managed via a `.env.local` file that is automatically loaded by the Makefile.

1. Create `.env.local` in the project root (this file is gitignored):

```bash
cat > .env.local << 'EOF'
IMAGE_PROXY=registry-proxy.example.com
IMAGE_PROXY_USERNAME=your-username
IMAGE_PROXY_PASSWORD=your-token
DB_PASSWORD=your-db-password
EOF
```

2. Now all make commands automatically pick up these values:

```bash
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
```

### Repository Providers

The application supports multiple Git hosting platforms through the `RepoProvider` abstraction (`pkg/provider`). At startup, the `providers` list is loaded from the YAML file pointed to by `CONFIG_FILE` (default `config/values.yml`). If the file is missing or contains no providers, only `github.com` (public, no auth) is registered as a fallback.

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

2. Set the token as an environment variable (or add it to `.env.local`):

   ```bash
   export GHE_TOKEN=ghp_xxxxxxxxxxxx
   ```

3. Run the application -- both `github.com` and `github.mycompany.com` repos are now accessible:

   ```bash
   make run
   # Browse to http://localhost:5000/github.mycompany.com/org/repo
   ```

4. To use a different config file:

   ```bash
   CONFIG_FILE=config/values-staging.yml make run
   ```

#### URL Format

All routes use the pattern `/{host}/{org}/{repo}`. Existing `github.com/...` URLs continue to work unchanged.

### Custom Values Files

For different environments, create custom values files:

```bash
# Copy the default values file
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
