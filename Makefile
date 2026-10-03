COMPOSE := docker compose -f compose.dev.yaml

.DEFAULT_GOAL := help
.PHONY: help setup up down logs ps test format fix lint typecheck check build frontend-restart

help:
	@printf '%s\n' \
	  'make setup             Install pinned host tools and editor dependencies' \
	  'make up                Start development servers (Ctrl-C stops them)' \
	  'make down              Remove development containers; preserve volumes' \
	  'make logs              Follow Go and frontend logs' \
	  'make ps                Show service status' \
	  'make test              Run Go package tests' \
	  'make format            Format frontend files' \
	  'make fix               Apply safe Biome fixes, including import ordering' \
	  'make lint              Lint frontend files' \
	  'make typecheck         Check frontend TypeScript' \
	  'make check             Run Go tests, Biome checks, and TypeScript checks' \
	  'make build             Build frontend assets in a temporary container' \
	  'make frontend-restart  Recreate frontend and install locked dependencies' \
	  '' \
	  'Tests and frontend checks/fixes require running services (make up).'

setup:
	asdf install golang
	asdf install bun
	env -u GOROOT asdf exec go mod download
	cd frontend && asdf exec bun install --frozen-lockfile

up:
	$(COMPOSE) up

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f app frontend

ps:
	$(COMPOSE) ps

test:
	$(COMPOSE) exec -T app go test ./...

format:
	$(COMPOSE) exec -T frontend bun run format

fix:
	$(COMPOSE) exec -T frontend bun run check --write

lint:
	$(COMPOSE) exec -T frontend bun run lint

typecheck:
	$(COMPOSE) exec -T frontend bun run typecheck

check: test
	$(COMPOSE) exec -T frontend bun run check
	$(COMPOSE) exec -T frontend bun run typecheck

build:
	$(COMPOSE) run --rm --no-deps -T frontend sh -c 'bun install --frozen-lockfile && bun run build'

frontend-restart:
	$(COMPOSE) up -d --force-recreate frontend
