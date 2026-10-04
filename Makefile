COMPOSE := docker compose -f compose.yaml -f compose.dev.yaml

.DEFAULT_GOAL := help
.PHONY: help setup up down logs ps test frontend-test format fix lint typecheck check build frontend-restart migrate migrate-status generate openapi

help:
	@printf '%s\n' \
	  'make setup             Install pinned host tools and editor dependencies' \
	  'make migrate           Stop app and apply pending SQLite migrations' \
	  'make migrate-status    Show applied and pending migrations' \
	  'make generate          Generate Go queries with sqlc' \
	  'make openapi           Export OpenAPI with host Go; no running services needed' \
	  'make up                Start development servers (Ctrl-C stops them)' \
	  'make down              Remove development containers; preserve volumes' \
	  'make logs              Follow Go, frontend, and contract logs' \
	  'make ps                Show service status' \
	  'make test              Run Go package tests' \
	  'make frontend-test     Run frontend helper and rendered UI tests' \
	  'make format            Format frontend files' \
	  'make fix               Apply safe Biome fixes, including import ordering' \
	  'make lint              Lint frontend files' \
	  'make typecheck         Check frontend TypeScript' \
	  'make check             Run Go/frontend tests, Biome, and TypeScript' \
	  'make build             Build frontend assets in a temporary container' \
	  'make frontend-restart  Recreate frontend and install locked dependencies' \
	  '' \
	  'Tests and frontend checks/fixes require running services (make up).'

setup:
	asdf install golang
	asdf install bun
	env -u GOROOT asdf exec go mod download
	cd frontend && asdf exec bun install --frozen-lockfile
	cd frontend/scripts && asdf exec bun install --frozen-lockfile

up:
	$(COMPOSE) up

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f app frontend contracts

ps:
	$(COMPOSE) ps

test:
	$(COMPOSE) exec -T app go test ./...

frontend-test:
	$(COMPOSE) exec -T frontend bun run test

format:
	$(COMPOSE) exec -T frontend bun run format

fix:
	$(COMPOSE) exec -T frontend bun run check --write

lint:
	$(COMPOSE) exec -T frontend bun run lint

typecheck:
	$(COMPOSE) exec -T frontend bun run typecheck

check: test frontend-test
	$(COMPOSE) exec -T frontend bun run check
	$(COMPOSE) exec -T frontend bun run typecheck

build:
	$(COMPOSE) run --rm --no-deps -T frontend sh -c 'bun install --frozen-lockfile && bun run build'

frontend-restart:
	$(COMPOSE) up -d --force-recreate frontend

migrate:
	$(COMPOSE) stop app
	$(COMPOSE) run --rm --no-deps -T storage-init
	$(COMPOSE) run --rm --no-deps -T dbmate

migrate-status:
	$(COMPOSE) run --rm --no-deps -T dbmate status

generate:
	$(COMPOSE) run --rm --no-deps -T sqlc generate

openapi:
	@tmp=$$(mktemp openapi/api.json.XXXXXX) || exit 1; \
	trap 'rm -f "$$tmp"' EXIT; \
	env -u GOROOT asdf exec go run ./cmd/openapi > "$$tmp" && mv "$$tmp" openapi/api.json
