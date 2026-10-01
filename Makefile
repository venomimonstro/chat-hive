SHELL := /bin/sh

.PHONY: up down backend web test build fmt release-check

up:
	docker compose up -d postgres redis nats

down:
	docker compose down

backend:
	cd backend && go run ./cmd/api

web:
	cd web && npm run dev

test:
	cd backend && go test ./...
	cd web && npm run typecheck

build:
	cd backend && go build ./cmd/api
	cd web && npm run build

fmt:
	cd backend && gofmt -w .

release-check:
	sh ops/release/check.sh
