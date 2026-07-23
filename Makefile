.PHONY: help deps up down api test lint lint-fix marketing org-admin employee migrate-indexes coverage

help:
	@echo "LaunchPad targets:"
	@echo "  make deps             Install Go and JS dependencies"
	@echo "  make up               Start MongoDB and Redis"
	@echo "  make down             Stop local infrastructure"
	@echo "  make migrate-indexes  Ensure MongoDB indexes"
	@echo "  make api              Run the Go API locally"
	@echo "  make marketing        Run marketing web on :3000"
	@echo "  make org-admin        Run organization admin on :3002"
	@echo "  make employee         Run employee portal on :3003"
	@echo "  make test             Run Go tests"
	@echo "  make coverage         Run Go tests with coverage.out"
	@echo "  make lint             Run golangci-lint (all enabled linters)"
	@echo "  make lint-fix        Auto-fix golangci-lint findings where supported"

deps:
	go mod tidy
	pnpm install

up:
	docker compose up -d mongo redis

down:
	docker compose down

migrate-indexes:
	go run ./scripts/migrate_indexes

api:
	go run ./apps/api/cmd/api

marketing:
	pnpm --filter @launchpad/marketing-web dev

org-admin:
	pnpm --filter @launchpad/organization-admin-web dev

employee:
	pnpm --filter @launchpad/employee-web dev

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...
