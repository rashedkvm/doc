# Developing

## Quick Start

```bash
# 1. Set up credentials (one-time, file is gitignored)
cat > .env.local << 'EOF'
IMAGE_PROXY=tapi-docker-virtual.usw1.packages.broadcom.com
IMAGE_PROXY_USERNAME=your-username
IMAGE_PROXY_PASSWORD=your-token
EOF

# 2. Build and push images
REGISTRY=krashed843 IMAGE_TAG=v1.0.0 make build-all push-all

# 3. Generate manifests and deploy
REGISTRY=krashed843 IMAGE_TAG=v1.0.0 make deploy-all
```

## Prerequisites

- Go 1.23+
- Docker
- kubectl configured with a Kubernetes cluster
- [Carvel tools](https://carvel.dev/) (ytt, kapp, kbld)
- Helm 3.x

## Configuration

All environment-specific settings are managed through a single values file. The default file is `config/values.yml` for local development.

### Values File Structure

The `config/values.yml` file contains non-sensitive configuration:

```yaml
# Registry and image settings (can be overridden via env vars)
registry: "docker.io"
image_tag: "latest"

# Application settings
app_port: 5000
namespace: doc-system

# Database connection settings
dbname: doc
dbuser: postgres
dbpwd: password
dbhost: pgsql-postgresql.postgres.svc.cluster.local

# Feature flags
analytics: "false"
is_dev: "true"
```

**Note:** The `registry` and `image_tag` values are used as defaults. Override them via environment variables: `REGISTRY=krashed843 IMAGE_TAG=v1.0.0 make build-all`

### Passing Sensitive Values

Sensitive values (registry credentials, passwords) are managed via a `.env.local` file that is automatically loaded by the Makefile.

1. Create `.env.local` in the project root (this file is gitignored):

```bash
cat > .env.local << 'EOF'
IMAGE_PROXY=tapi-docker-virtual.usw1.packages.broadcom.com
IMAGE_PROXY_USERNAME=your-username
IMAGE_PROXY_PASSWORD=your-token
DB_PASSWORD=your-db-password
EOF
```

2. Now all make commands automatically pick up these values:

```bash
make dist        # Generates manifests with proxy config
make db-gen      # Generates database manifests with proxy config
make deploy-all  # Deploys with all credentials
```

You can also pass values directly on the command line (overrides `.env.local`):

```bash
IMAGE_PROXY_USERNAME=myuser IMAGE_PROXY_PASSWORD=mytoken make deploy
```

### Using Different Environments

```bash
# Local development (default - uses config/values.yml)
make deploy

# Override registry and tag for your images
REGISTRY=krashed843 IMAGE_TAG=v1.0.0 make build-all push-all deploy
```

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

### Using Postgres Docker Image

The easiest way to get started developing locally is with the official Postgres Docker image.

1. Start PostgreSQL container:

```bash
make run-db
```

2. Initialize the database schema:

```bash
make init-db
```

3. Run the application:

```bash
make run
```

### Available Make Targets

Run `make help` to see all available targets.

## Kubernetes Deployment

### Build and Push Docker Images

```bash
# Build images
make build-all

# Push to registry
make push-all

# Or do both
make build-all push-all
```

### Deploy PostgreSQL Database

```bash
make db-deploy
```

### Deploy Application

```bash
make deploy
```

### Deploy Everything

```bash
make deploy-all
```

### Full Release Workflow

Build, push, and deploy in one command:

```bash
# Uses default registry from config/values.yml
make release

# With custom registry and tag
REGISTRY=krashed843 IMAGE_TAG=v1.0.0 make release
```

### Using an Image Registry Proxy

The image proxy is used for pulling base images (e.g., PostgreSQL) through a mirror to avoid Docker Hub rate limits.

1. Add proxy credentials to `.env.local`:
   ```bash
   IMAGE_PROXY=tapi-docker-virtual.usw1.packages.broadcom.com
   IMAGE_PROXY_USERNAME=your-username
   IMAGE_PROXY_PASSWORD=your-token
   ```

2. Run any make target - proxy config is automatically applied:
   ```bash
   make dist      # App manifests include imagePullSecrets
   make db-gen    # Database manifests use proxy for PostgreSQL image
   ```

When proxy variables are set, the generated manifests will include:
- A `registry-credentials` Secret with dockerconfigjson for the proxy
- `imagePullSecrets` on all Deployments, CronJobs, and StatefulSets
