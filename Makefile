.PHONY: dev dev-backend dev-ui test test-backend test-ui lint lint-backend lint-ui

dev-backend:
	cd backend && go run ./cmd/api

dev-ui:
	cd ui && bun run dev

dev:
	$(MAKE) -j2 dev-backend dev-ui

test-backend:
	cd backend && go test ./...

test-ui:
	cd ui && bun test

test: test-backend test-ui

lint-backend:
	cd backend && go vet ./...

lint-ui:
	cd ui && bun run lint

lint: lint-backend lint-ui
