# Local development shortcuts. Every target reads configuration from .env.
LOAD_ENV = set -a; . ./.env; set +a;
# The whole Docker stack lives in docker/; configuration comes from the root .env.
COMPOSE = docker compose --env-file .env -f docker/docker-compose.yml

.PHONY: up down app-up app-up-traefik app-logs migrate api web web-test web-build test test-integration

up: ## Start only PostgreSQL and the local object storage (run the API and the web from the host)
	$(COMPOSE) up -d postgres object-storage

down: ## Stop all the containers (data volumes are kept)
	$(COMPOSE) down

app-up: ## Build and start everything in containers (PostgreSQL, storage, API, nginx)
	$(COMPOSE) up -d --build --remove-orphans

app-up-traefik: ## Same as app-up, plus the labels for a server where Traefik already runs
	$(COMPOSE) -f docker/docker-compose.traefik.yml up -d --build --remove-orphans

app-logs: ## Follow the logs of the API and nginx
	$(COMPOSE) logs -f api nginx

migrate: ## Apply pending database migrations
	$(LOAD_ENV) cd backend && go run ./cmd/migrate

api: ## Run the Go API
	$(LOAD_ENV) cd backend && go run ./cmd/api

web: ## Run the web app (Vite dev server, proxies /api to the Go API)
	cd web && pnpm dev

web-test: ## Web unit and component tests
	cd web && pnpm test

web-build: ## Type-check and build the web app
	cd web && pnpm build

test: ## Backend unit tests only (no containers needed)
	cd backend && go test ./...

test-integration: ## All backend tests, including PostgreSQL and object storage
	$(LOAD_ENV) \
	$(COMPOSE) exec -T postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB -tc "SELECT 1 FROM pg_database WHERE datname='gumunova_test'" | grep -q 1 \
	  || $(COMPOSE) exec -T postgres psql -U $$POSTGRES_USER -d $$POSTGRES_DB -c "CREATE DATABASE gumunova_test"; \
	cd backend && \
	TEST_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:$$POSTGRES_PORT/gumunova_test?sslmode=disable" \
	TEST_OBJECT_STORAGE_ENDPOINT="http://127.0.0.1:$$OBJECT_STORAGE_PORT" \
	go test -count=1 ./...
