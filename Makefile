SHELL := /bin/sh

.PHONY: up down backend web test fmt

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
	cd web && npm run typecheck && npm run lint

fmt:
	cd backend && gofmt -w .
