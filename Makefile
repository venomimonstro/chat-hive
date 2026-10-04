SHELL := /bin/sh

.PHONY: up down backend web test build fmt frontend-lock release-check alpha-gate beta-gate

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

frontend-lock:
	bash ops/frontend/lock.sh

release-check:
	bash ops/release/check.sh

alpha-gate:
	bash ops/alpha/gate.sh

beta-gate:
	bash ops/beta/gate.sh
