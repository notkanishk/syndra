# Syndra Project Context — Codex Handover

This document is specifically crafted for the **Codex agent** to provide a "Cold Start" understanding of the Syndra Control and Data Plane implementation.

## 1. Core Architecture
Syndra follows a **dual-plane architecture** for identity orchestration:
- **Control Plane (Go + Postgres + Next.js)**: Management UI for defining roles, bundles, and mapping rules. Admin-facing.
- **Data Plane (Go + Redis)**: Fast, claim-shaping API used by Zitadel Actions v2 to inject roles into JWTs. Application-facing.

## 2. Recent Implementation (Bundles & Secure Data Plane)
The following features are now live and verified:

### 📦 Bundles & User Assignments
Admins can define **Bundles** (aggregates of Zitadel roles) and assign them to **User IDs** in the Syndra UI.
- **Table**: `user_bundle_assignments`
- **Logic**: When a bundle is assigned to a user, all constituent roles are considered "initial" roles during the compilation phase.

### ⚡ Smart Cache Compiler (`internal/cache/compiler.go`)
Resolves a user's total effective roles using a **Fixed-Point Iteration (Forward Pass)** algorithm:
1.  **Fetch Initial Set**: Combine Zitadel's raw roles with roles assigned via Syndra Bundles.
2.  **Iterative Resolution**: Evaluate all **Mapping Rules** (`IF Role:A THEN ADD Role:B`). Repeat until no new roles are discovered.
3.  **Redis Populate**: Store the final, flattened list of roles for each downstream application in Redis.

### 🛡️ Safety & Governance
- **Cycle Detection**: The system implements a mandatory DFS (Depth-First Search) check during mapping rule creation (`internal/db/validation.go`). Circular dependencies (e.g., `A -> B -> A`) are blocked.
- **Security Middleware**: ALL backend routes (Control & Data) now require an `Authorization: Bearer <SYNDRA_API_KEY>` header.

---

## 3. Current System State & Environment
- **Host**: Proxmox LXC (`198.51.100.14`)
- **Infrastructure**: Docker Compose (Postgres:5432, Redis:6379, Backend:8080, UI:3000).
- **Zitadel Integration**: **IMPORTANT.** The Zitadel management client is currently **STUBBED** (`internal/auth/zitadel.go`). It simulates successful role fetching and creation to allow development without production M2M credentials.
- **API Key**: `dev_auth_token_secret` (Set via `docker-compose.yml`).

## 4. Next Pointers for Action
1.  **Zitadel Live Sync**: Replace the stubbed `ZitadelClient` with the real management SDK once M2M credentials (Client ID/Secret) are provided.
2.  **User Search Proxy**: The "Users & Access" page in the UI needs a proxy to search for real users in Zitadel (Zitadel Management API).
3.  **Cleanup Hooks**: Implement the webhook listener to invalidate Redis cache when a user's roles change out-of-band in Zitadel.

---
*Created by Antigravity for Codex*
