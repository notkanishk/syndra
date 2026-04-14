# MkAuth Core Architecture Design

## 1. Mission & Core Philosophy
* **Zitadel is the Absolute Source of Truth:** Zitadel holds the final, authoritative list of users and roles. MkAuth sits on top as an orchestration and policy layer.
* **Purpose:** To simplify and manage complex hierarchical permissions across various makerspace systems (digital SSO and physical access) without bloating Zitadel with complex mapping logic.

## 2. Primary Objectives (Main Aims)
*   **Easy Application Claim Selection:** Downstream applications can effortlessly select and receive enriched claims (roles) derived from any project in the ecosystem, shaped to their specific JWT requirements.
*   **Unified User Claim Management:** Admins have a single "God-Mode" interface to view, audit, and manage a user's entire identity footprint with total "Access Lineage" visibility.
*   **Service-Centric Self-Service:** Standard users can browse a catalog of **Services** (Applications) and request access to them without needing to understand or manage underlying roles.

## 3. The Architectural Split
The system is divided into two distinct planes to balance complex logic with ultra-low latency token authorization.

### The Control Plane (Slow & Smart)
*   **Components:** MkAuth UI Dashboard, Backend API, Local Policy DB (e.g., Postgres/SQLite).
*   **Behavior:** When an admin assigns a role or bundle, MkAuth evaluates dependencies and structural mapping rules. It then calls the Zitadel Management API using an independent, highly-scoped Machine-to-Machine (Service Account) token to actually grant the roles.
*   **Zero-Trust Context:** The MkAuth backend inherently distrusts the UI. Privileged frontend-to-backend requests MUST carry a Zitadel-issued user access token, and the backend MUST validate that token and the logged-in admin's personal permissions before executing mutations via the Service Account.
*   **Credential Rule:** A dedicated service user account is still required for backend-owned Management API operations, but it MUST be least-privileged, server-side only, rotated, and never exposed to the frontend or the sync service.

### The Data Plane (Fast & Dumb)
*   **Components:** Local Redis Cache, **Zitadel Actions v2**.
*   **Behavior:** During a user login flow to a downstream application, Zitadel triggers an **Actions v2** script. This script executes within the v2 execution environment, pings MkAuth's fast Redis endpoint, and receives pre-compiled, flattened roles. The roles are then injected directly into the JWT via the v2 context's native claim manipulation APIs.
*   **Version Pinning:** Actions v1 is deprecated and MUST NOT be used. All custom JWT logic MUST reside in the modern v2 flow.
*   **Compatibility Rule:** All source-of-truth-facing claim and event assumptions MUST remain compatible with Zitadel Actions v2. MkAuth MUST NOT introduce an alternate Zitadel-facing contract model outside that boundary.

### The Bridge Plane (Provisioning)
*   **Components:** LLDAP Sync Service (Go), LLDAP Server.
*   **Orchestrated Flow:** Zitadel webhooks are received ONLY by the **MkAuth Backend**. The Backend validates the change against its policy engine and, if a sync is required, emits a **Provisioning Intent** to the Sync Service via an internal encrypted channel.
*   **Isolation:** The Sync Service is a private worker that does not expose external ports; it only reacts to verified Backend commands to manage LLDAP groups and user passwords.
*   **Credential Bridge Requirement:** MkAuth MUST support a Samba/LLDAP-specific secondary credential because certain makerspace infrastructure still requires password-based authentication now. This is a necessary bridge capability, not a general identity model.
*   **Identity Reflection:** MkAuth manages the "Shadow Password" vault as an infrastructure credential bridge; these secrets are pushed to the Sync Service only during the physical propagation event.
*   **Isolation Rule:** Samba/LLDAP password handling MUST remain logically isolated from normal Zitadel/OIDC identity flows, independently auditable, and narrowly scoped to infrastructure access only.
*   **Internal Contract Rule:** Frontend-to-Backend and Backend-to-Sync communication may use self-defined MkAuth structures, but those internal contracts MUST be explicit, authenticated, validated, and kept separate from Zitadel-facing Actions v2 compatibility assumptions.

## 3. The Logic Engine & Policy Rules
*   **Explicit Mapping Rules:** Instead of fragile deep inheritance trees, MkAuth uses flat conditional rules (e.g., `IF project:printing role:user THEN ADD project:door_access role:3d_lab_pin`).
*   **Versioned Policies:** All rules are explicitly versioned in the database, tracking changes over time and enabling seamless rollbacks if a policy breaks access.
*   **Validation Constraints:** The policy engine must actively detect and block **Circular Dependencies** within rules and gracefully handle **Partial Assignment Failures** (rolling back if the Zitadel API becomes unreachable mid-update).

## 4. Core Dashboard Views
*   **User-Centric View (Most Important):** Unifies a user's roles grouped by project and bundle. Visually separates "Source" (raw roles) vs "Derived" roles, explicitly answering the question: *"Why does this user have access to X?"* (Access Lineage).
*   **Application-Centric View:** Shows the roles an app consumes, its claim-shaping rules, and includes a **Token Simulator** to preview the exact JWT payloads the app will receive.
*   **Project View:** Displays roles natively defined in Zitadel, who currently has them, and which bundles/policies actively utilize them.
*   **Bundle / Policy View:** Displays definitions of bundles, affected projects, and the ultimate impacted user pool.
*   **Access Requests / Governance View:** Surfaces pending access requests, expiring direct grants, and cleanup hints so admins can review the system from a least-privilege perspective.
*   **Topology Graph / God Mode View:** A macroscopic graph of projects, roles, bundles, applications, and mapping rules for visual exploration and debugging.

## 5. UI, UX, & Governance
*   **Aesthetic & Style:** Inspired by Linear/Stripe. Clean, enterprise-grade layout using distinct cards, soft shadows, and moderate whitespace to prevent intimidating non-technical staff. Dark/light modes prominently feature **vibrant accent colors** to highlight primary actions.
*   **Product Priority:** MkAuth is admin-console first and user-facing second. The primary product obligation is to give administrators safe, explainable, auditable control over access. Member-facing flows remain important, but they are subordinate to administrative clarity and security.
*   **View Differentiation (Admin vs User):**
    *   **Admin View:** Focused on projects, raw roles, mapping rules, and global auditing. Admins sign in through a Zitadel-backed user session, and privileged backend actions are authorized from the admin's Zitadel-issued user access token.
    *   **User Portal:** Focused on the **Service Catalog** (Applications). Standard users see a simplified view showing only what services they have access to and a simplified "Request Access" flow.
*   **Role Assignment UX:** "Assign once, propagate correctly." Admins can assign raw roles (advanced) or Bundles (normal users). Selecting a Bundle explicitly previews exactly which underlying roles will be applied.
*   **Audit Logging & Self-Service:** Strict timeline tracking of Who granted What, and When. Temporary/Semester roles auto-expire, and users can trigger self-service permission requests for admin approval.

## 6. Technical Stack & Deployment
*   **Deployment:** Docker Compose running inside a Proxmox LXC (Linux Container). This provides a robust 1-command installation and update mechanism via an `update.sh` script that pulls GitHub changes and restarts the stack without downtime. The stack uses **Separate Containers** for Frontend, Backend, and the LLDAP Sync Service to ensure isolation and security.
*   **Authentication & Identity:**
    *   **Backend:** Uses a high-scoped **Machine-to-Machine (Service Account)** token from Zitadel only for backend-owned Management API operations after authorization succeeds. Privileged frontend-originated requests are authorized from a Zitadel-issued user access token validated by the backend.
    *   **Frontend:** **User Session (OIDC)** backed by Zitadel-issued user access tokens. The PKCE authorization code flow is implemented: the UI performs login via Zitadel, stores the access token in an `mkauth_session` cookie (discriminated union: `demo | oidc`), and forwards it as `Authorization: Bearer <token>` on all backend requests — both through the proxy route and SSR server-component fetches. Demo cookie sessions with Admin/User view differentiation remain active as a local-dev fallback when `ZITADEL_DOMAIN` is unset.
    *   **Internal API Key Rule:** A shared internal API key MAY remain as defense-in-depth for service-to-service traffic, but it MUST NOT be treated as sufficient production authorization for privileged actions.
    *   **Sync Service:** A dedicated worker that synchronizes identity state from MkAuth/Zitadel into the LLDAP server.
*   **Backend / Orchestrator:** Go (Golang).
*   **Frontend Dashboard:** Next.js (React).

## 7. Current Implementation Status (v1.0-Bundles)
*   [x] **Secure Data Plane**: Bearer token authentication enforced on all routes.
*   [x] **Smart Cache Compiler**: Iterative, transitive role resolution in Go.
*   [x] **Mapping Rules**: Rule management API and UI.
*   [x] **Bundles**: Role aggregation and User-Bundle assignments.
*   [x] **Access Workflows**: Direct grants with optional expiry plus request/approval flows.
*   [x] **Cycle Detection**: DFS validation on rule creation.
*   [x] **Audit Logging**: Mandatory tracking for all admin actions.
*   [x] **Governance Summary**: Pending requests, expiring grants, and cleanup hints.
*   [x] **Topology Graph**: Visual "God Mode" graph and supporting API.
*   [x] **Seeded Demo Catalog**: Users, projects, applications, and dummy relationships for local testing.
*   [x] **Frontend Session Split**: PKCE authorization code flow implemented (`ui/src/lib/oidc.ts`, `ui/src/app/auth/zitadel/route.ts`, `ui/src/app/auth/callback/route.ts`). Zitadel access tokens stored in session cookie and forwarded as `Authorization: Bearer <token>` on all backend requests. Demo mode remains active when `ZITADEL_DOMAIN` is unset.
*   [x] **Zitadel Integration**: M2M Management Client implemented (direct HTTP, JWT profile auth, retry with backoff). Requires `ZITADEL_MACHINE_KEY_PATH` for live sync; degrades gracefully without it.
*   [ ] **Production Rollout**: Final deployment with actual keys, networking, and live Zitadel credentials.


## 8. Immediate Priority: Contract Hardening & Test Coverage
Before MkAuth widens its live Zitadel and provisioning surface, the immediate next milestone is to harden the contract backbone of the application. That includes strict backend request decoding, bounded domain types, stable error semantics, stronger database invariants, backend-enforced authorization assumptions, and full covering backend-first tests for mission-critical flows. This hardening pass must also make Zitadel Actions v2 the explicit and only source-of-truth compatibility boundary, while allowing separately hardened internal MkAuth contracts for frontend, backend, and sync-service communication. The UI may continue to evolve, but the backend contract layer is now the highest-priority stability and security concern.

## 9. Zitadel Interaction Matrix

| Function or feature | Mechanism | Notes |
| --- | --- | --- |
| token claim enrichment for downstream applications | Actions v2 | source-of-truth claim boundary; only supported claim-integration path |
| Zitadel-native event-driven trigger logic | Actions v2 or validated backend webhook intake | detection and compatibility boundary only; business mutations stay backend-owned |
| privileged frontend-to-backend admin actions | Zitadel-issued user access token validated by backend | backend identifies and authorizes the acting user directly; shared internal API key is optional defense-in-depth only |
| backend grant or revoke operations in Zitadel | service user account | Management API path; backend-owned only |
| mapping-rule propagation back into Zitadel | service user account | requires server-side control-plane mutation rights |
| welcome-bundle assignment and similar onboarding mutations | backend service account path after validated event intake | MkAuth Backend remains the single mutation authority for audit, retries, and idempotency |
| webhook reception and verification | backend endpoint with validated Zitadel event contract | external intake stays on backend; sync service remains private |
| Backend -> Sync provisioning intents | internal MkAuth contract | self-defined, authenticated, and isolated from Zitadel-facing contracts |
