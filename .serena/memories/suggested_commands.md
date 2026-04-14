# Suggested Commands

## Development
- `make dev` — Run backend and frontend in parallel
- `make dev-backend` — Run backend only (`cd backend && go run ./cmd/api`)
- `make dev-ui` — Run frontend only (`cd ui && bun run dev`)

## Testing
- `make test` — Run all tests (backend + frontend)
- `make test-backend` — `cd backend && go test ./...`
- `make test-ui` — `cd ui && bun test`
- `cd backend && go test ./internal/zitadel/... -v` — Run specific package tests

## Linting
- `make lint` — Run all linters
- `make lint-backend` — `cd backend && go vet ./...`
- `make lint-ui` — `cd ui && bun run lint`

## Build
- `cd backend && go build ./...` — Build backend
- `cd ui && bun run build` — Build frontend

## Git (Darwin)
- `git status`, `git diff`, `git log --oneline -10`

## System (Darwin)
- `ls`, `find`, `grep -r`, `cat`, `head`, `tail`
- Homebrew: `brew install <pkg>`
