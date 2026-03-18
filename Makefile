# Set the shell to bash always
SHELL := /bin/bash
.SHELLFLAGS := -ec

# Load environment variables from .env.local if it exists (for secrets)
-include .env.local
export

# Suppress kapp prompts with KAPP_ARGS="--yes"
KAPP_ARGS ?= --yes=false

# Values file for environment-specific configuration
# Override with: VALUES_FILE=config/values-prod.yml make deploy
VALUES_FILE ?= config/values.yml

# Image configuration (extracted from VALUES_FILE, can be overridden)
REGISTRY ?= $(shell ytt -f $(VALUES_FILE) 2>/dev/null | grep '^registry:' | awk '{print $$2}' || echo "docker.io")
IMAGE_TAG ?= $(shell ytt -f $(VALUES_FILE) 2>/dev/null | grep '^image_tag:' | awk '{print $$2}' || echo "latest")

# Prepend IMAGE_PROXY to image URLs when set (for environments that mirror Docker Hub)
IMAGE_PREFIX = $(if $(IMAGE_PROXY),$(IMAGE_PROXY)/,)
DOC_IMAGE = $(IMAGE_PREFIX)$(REGISTRY)/doc:$(IMAGE_TAG)
GITTER_IMAGE = $(IMAGE_PREFIX)$(REGISTRY)/gitter:$(IMAGE_TAG)

# Sensitive credentials from environment variables (optional)
# Usage: IMAGE_PROXY_USERNAME=user IMAGE_PROXY_PASSWORD=token make deploy
IMAGE_PROXY_USERNAME ?=
IMAGE_PROXY_PASSWORD ?=
DB_PASSWORD ?=

# ytt arguments for injecting environment variable overrides
YTT_ENV_ARGS = $(if $(IMAGE_PROXY),-v image_proxy=$(IMAGE_PROXY)) \
               $(if $(IMAGE_PROXY_USERNAME),-v image_proxy_username=$(IMAGE_PROXY_USERNAME)) \
               $(if $(IMAGE_PROXY_PASSWORD),-v image_proxy_password=$(IMAGE_PROXY_PASSWORD)) \
               $(if $(DB_PASSWORD),-v dbpwd=$(DB_PASSWORD))

##@ Docker Builds

.PHONY: build-doc
build-doc: ## Build doc Docker image
	docker build . -f deploy/doc.Dockerfile -t $(DOC_IMAGE)

.PHONY: build-gitter
build-gitter: ## Build gitter Docker image
	docker build . -f deploy/gitter.Dockerfile -t $(GITTER_IMAGE)

.PHONY: build-all
build-all: build-doc build-gitter ## Build all Docker images

.PHONY: push-doc
push-doc: ## Push doc image to registry
	docker push $(DOC_IMAGE)

.PHONY: push-gitter
push-gitter: ## Push gitter image to registry
	docker push $(GITTER_IMAGE)

.PHONY: push-all
push-all: push-doc push-gitter ## Push all images to registry

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy -v

.PHONY: test
test: fmt vet ## Run tests
	go test ./... -short -coverprofile cover.out

##@ Database

.PHONY: db-gen
db-gen: ## Generate PostgreSQL deployment manifest from Helm chart
	@mkdir -p dist
	helm template pgsql oci://registry-1.docker.io/bitnamicharts/postgresql \
		-f <(ytt -f config/helm/values.yml -f config/app/values-schema.yml \
			--data-values-file $(VALUES_FILE) $(YTT_ENV_ARGS)) \
		--create-namespace -n postgres | kbld -f - | \
		ytt -f - -f config/database -f config/app/values-schema.yml \
			--data-values-file $(VALUES_FILE) $(YTT_ENV_ARGS) \
			--data-value-file init_sql=schema/crds_up.sql > dist/postgres.yml

.PHONY: db-deploy
db-deploy: db-gen ## Deploy PostgreSQL to the K8s cluster
	kapp deploy -a doc-db -n kube-public -f dist/postgres.yml $(KAPP_ARGS)

.PHONY: db-undeploy
db-undeploy: ## Remove PostgreSQL from the K8s cluster
	kapp delete -a doc-db -n kube-public $(KAPP_ARGS)

##@ Application

.PHONY: dist
dist: ## Generate application deployment manifests
	@mkdir -p dist
	ytt -f config/app/ --data-values-file $(VALUES_FILE) \
		-v doc_image=$(DOC_IMAGE) -v gitter_image=$(GITTER_IMAGE) \
		$(YTT_ENV_ARGS) > dist/doc-app.yml

.PHONY: deploy
deploy: dist ## Deploy doc application to the K8s cluster
	kapp deploy -a doc -n kube-public -f dist/doc-app.yml $(KAPP_ARGS)

.PHONY: undeploy
undeploy: ## Remove doc application from the K8s cluster
	kapp delete -a doc -n kube-public $(KAPP_ARGS)

.PHONY: deploy-all
deploy-all: db-deploy deploy ## Deploy database and application

.PHONY: undeploy-all
undeploy-all: undeploy db-undeploy ## Remove application and database

.PHONY: release
release: build-all push-all deploy ## Build, push, and deploy (full workflow)

##@ Local Development

POSTGRES_CONTAINER_NAME ?= doc-postgres
POSTGRES_PASSWORD ?= password
POSTGRES_PORT ?= 5432

.PHONY: run-db
run-db: ## Run PostgreSQL in a local container
	@docker stop $(POSTGRES_CONTAINER_NAME) > /dev/null 2>&1 || true
	@docker rm $(POSTGRES_CONTAINER_NAME) > /dev/null 2>&1 || true
	docker run --name $(POSTGRES_CONTAINER_NAME) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-d -p $(POSTGRES_PORT):5432 postgres:18
	@echo "PostgreSQL started. Run 'make init-db' to initialize the schema."
	@echo "Connection: psql -h localhost -U postgres -d postgres"

.PHONY: clean-db
clean-db: ## Stop and remove local PostgreSQL container
	@docker stop $(POSTGRES_CONTAINER_NAME) > /dev/null 2>&1 || true
	@docker rm $(POSTGRES_CONTAINER_NAME) > /dev/null 2>&1 || true
	@echo "PostgreSQL container removed."

.PHONY: init-db
init-db: ## Initialize the database schema
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	PGPASSWORD=$(POSTGRES_PASSWORD) psql -h 127.0.0.1 -U postgres -d postgres -a -f schema/crds_up.sql

.PHONY: run
run: ## Run the doc application locally
	PG_USER=postgres PG_PASS=$(POSTGRES_PASSWORD) PG_HOST=127.0.0.1 PG_PORT=5432 PG_DB=doc \
		go run ./cmd/doc/main.go

.PHONY: run-gitter
run-gitter: ## Run the gitter application locally
	PG_USER=postgres PG_PASS=$(POSTGRES_PASSWORD) PG_HOST=127.0.0.1 PG_PORT=5432 PG_DB=doc \
		go run ./cmd/gitter/main.go

.PHONY: sandbox
sandbox: run-db init-db run-gitter run ## Run the doc and gitter applications locally

.PHONY: clean-sandbox
clean-sandbox: clean-db clean-gitter clean ## Clean the doc and gitter applications locally

.PHONY: clean-gitter
clean-gitter: ## Clean the local gitter application
	-kill $$(lsof -t -i :1234) 2>/dev/null || true
	-pkill -f "go run cmd/gitter/main.go" 2>/dev/null || true

.PHONY: clean
clean: ## Clean the local doc application
	-kill $$(lsof -t -i :5000) 2>/dev/null || true
	-pkill -f "go run cmd/doc/main.go" 2>/dev/null || true

##@ Help

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
