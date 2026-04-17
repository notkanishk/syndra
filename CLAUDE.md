# MkAuth

Identity & Access Management orchestration layer for an academic makerspace. Companion to Zitadel with Google Workspace as sole IdP.

## Quick Navigation

- **Spec index:** `openspec/INDEX.md` — master hub for all specs, capabilities, and changes
- **Architecture:** `openspec/changes/mkauth-core-architecture/design.md` — three-plane design, Zitadel interaction matrix, IdP chain
- **Roadmap:** `openspec/changes/mkauth-core-architecture/ROADMAP.md` — phase timeline (1-4 complete, 5-6 ahead)
- **Reality check:** `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — planned vs integrated

## Tech Stack

- **Backend:** Go, PostgreSQL, Redis (`backend/`)
- **Frontend:** Next.js with Bun runtime (`ui/`)
- **Sync Service:** Go, go-ldap/v3, separate container (`sync/`)
- **Deployment:** Docker Compose in Proxmox LXC

## Build & Test

```bash
cd backend && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
cd sync && go test ./... && go vet ./...
```

## Key Conventions

- Strict JSON decoding (`decodeJSONStrict`) on all mutation endpoints
- Injectable dependency pattern for testability (`deps.go` files)
- Zitadel Actions v2 is the only source-of-truth claim integration path
- Backend is the single mutation authority — frontend and triggers signal, backend decides
- Internal contracts (FE->BE, BE->Sync) are self-defined but isolated from Zitadel-facing boundary
