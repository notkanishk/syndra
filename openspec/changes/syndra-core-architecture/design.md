# Syndra Core Architecture Design

> **Navigation:** See [INDEX.md](../../INDEX.md) for the full spec graph. See [ROADMAP.md](ROADMAP.md) for phase timeline. See [feature-coverage.md](specs/feature-coverage.md) for planned vs integrated.

## 1. Mission & Core Philosophy
* **Zitadel is the Absolute Source of Truth:** Zitadel holds the final, authoritative list of users and roles. Syndra sits on top as an orchestration and policy layer.
* **Purpose:** To simplify and manage complex hierarchical permissions across various makerspace systems (digital SSO and physical access) without bloating Zitadel with complex mapping logic.

## 2. Primary Objectives (Main Aims)
*   **Easy Application Claim Selection:** Downstream applications can effortlessly select and receive enriched claims (roles) derived from any project in the ecosystem, shaped to their specific JWT requirements.
*   **Unified User Claim Management:** Admins have a single "God-Mode" interface to view, audit, and manage a user's entire identity footprint with total "Access Lineage" visibility.
*   **Service-Centric Self-Service:** Standard users can browse a catalog of **Services** (Applications) and request access to them without needing to understand or manage underlying roles.

## 3. The Architectural Split
The system is divided into two distinct planes to balance complex logic with ultra-low latency token authorization.

### The Control Plane (Slow & Smart)
*   **Components:** Syndra UI Dashboard, Backend API, Local Policy DB (e.g., Postgres/SQLite).
*   **Behavior:** When an admin assigns a role or bundle, Syndra evaluates dependencies and structural mapping rules. It then calls the Zitadel Management API using an independent, highly-scoped Machine-to-Machine (Service Account) token to actually grant the roles.
*   **Zero-Trust Context:** The Syndra backend inherently distrusts the UI. Privileged frontend-to-backend requests MUST carry a Zitadel-issued user access token, and the backend MUST validate that token and the logged-in admin's personal permissions before executing mutations via the Service Account.
*   **Credential Rule:** A dedicated service user account is still required for backend-owned Management API operations, but it MUST be least-privileged, server-side only, rotated, and never exposed to the frontend or the sync service.

### The Data Plane (Fast & Dumb)
*   **Components:** Local Redis Cache, **Zitadel Actions v2**.
*   **Behavior:** During a user login flow to a downstream application, Zitadel triggers an **Actions v2** script. This script executes within the v2 execution environment, pings Syndra's fast Redis endpoint, and receives pre-compiled, flattened roles. The roles are then injected directly into the JWT via the v2 context's native claim manipulation APIs.
*   **Version Pinning:** Actions v1 is deprecated and MUST NOT be used. All custom JWT logic MUST reside in the modern v2 flow.
*   **Compatibility Rule:** All source-of-truth-facing claim and event assumptions MUST remain compatible with Zitadel Actions v2. Syndra MUST NOT introduce an alternate Zitadel-facing contract model outside that boundary.

### The Target Plane (Add-ons)

*Superseded the Bridge Plane on 2026-08-10. Change `addon-platform` replaced the
LLDAP sync service with one add-on container per target; the bridge, its
provisioning-intent queue and the password vault that fed it are deleted. What
follows describes what exists.*

*   **Components:** one add-on container per system Syndra provisions into (`addons/truenas` today), each reaching its target through that target's own management API. No intermediate directory.
*   **Orchestrated Flow:** Zitadel webhooks are received ONLY by the **Syndra Backend**. A role change runs the closure diff every cascade computes, and that diff fires the lifecycle trigger: the backend resolves the subject's desired state on every target the changed role is mapped to, records it as an immutable snapshot, and queues one outbox row per subject beside its Zitadel rows. An add-on never decides anything — Syndra decides who and what, the add-on decides how.
*   **Isolation:** an add-on exposes no host port and is reachable only on the internal network. Calls are mutually authenticated (signed requests carrying a timestamp and a body hash, over TLS whose server key the backend pins — both keys derived from one per-target secret), and the registered base URL is the only authority — a redirect is refused rather than followed.
*   **Credential Requirement:** certain makerspace infrastructure still requires password-based authentication. A member sets that credential in Syndra and it is **forwarded to the target and kept nowhere** — no store, no hash, no vault. The bridge's Argon2id vault was deleted with the bridge: no API on this path accepts a hash, so the only thing a stored one could do is leak.
*   **The add-on is the least trusted component.** It holds the target credential and talks to a third-party API, so its manifest is a CEILING the backend intersects with its own policy rather than a grant — an operation absent from backend policy is unavailable whatever the manifest says.
*   **Internal Contract Rule:** Frontend-to-Backend and Backend-to-add-on communication may use self-defined Syndra structures, but those contracts MUST be explicit, authenticated, validated, and kept separate from Zitadel-facing Actions v2 assumptions. The add-on contract is held to a committed artifact (`addons/contract/*.json`) asserted from both ends, because the two are separately compiled modules and each was once tested only against its own fake.

## 3. The Logic Engine & Policy Rules
*   **Explicit Mapping Rules:** Instead of fragile deep inheritance trees, Syndra uses flat conditional rules (e.g., `IF project:printing role:user THEN ADD project:door_access role:3d_lab_pin`).
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
*   **Product Priority:** Syndra is admin-console first and user-facing second. The primary product obligation is to give administrators safe, explainable, auditable control over access. Member-facing flows remain important, but they are subordinate to administrative clarity and security.
*   **View Differentiation (Admin vs User):**
    *   **Admin View:** Focused on projects, raw roles, mapping rules, and global auditing. Admins sign in through a Zitadel-backed user session, and privileged backend actions are authorized from the admin's Zitadel-issued user access token.
    *   **User Portal:** Focused on the **Service Catalog** (Applications). Standard users see a simplified view showing only what services they have access to and a simplified "Request Access" flow.
*   **Role Assignment UX:** "Assign once, propagate correctly." Admins can assign raw roles (advanced) or Bundles (normal users). Selecting a Bundle explicitly previews exactly which underlying roles will be applied.
*   **Audit Logging & Self-Service:** Strict timeline tracking of Who granted What, and When. Temporary/Semester roles auto-expire, and users can trigger self-service permission requests for admin approval.

## 6. Technical Stack & Deployment
*   **Deployment:** Docker Compose running inside a Proxmox LXC (Linux Container). This provides a robust 1-command installation and update mechanism via an `update.sh` script that pulls GitHub changes and restarts the stack without downtime. The Syndra stack uses **Separate Containers** for Frontend, Backend, and one per add-on target, to ensure isolation and security. Each add-on sits behind a Compose profile and does not start by default: it holds a credential for the system it provisions, and a container nobody asked for holding one is a container nobody is watching.
*   **Authentication & Identity:**
    *   **Backend:** Uses a high-scoped **Machine-to-Machine (Service Account)** token from Zitadel only for backend-owned Management API operations after authorization succeeds. Privileged frontend-originated requests are authorized from a Zitadel-issued user access token validated by the backend.
    *   **Frontend:** **User Session (OIDC)** backed by Zitadel-issued user access tokens. The PKCE authorization code flow is implemented: the UI performs login via Zitadel, stores the access token in an `syndra_session` cookie (discriminated union: `demo | oidc`), and forwards it as `Authorization: Bearer <token>` on all backend requests — both through the proxy route and SSR server-component fetches. Demo cookie sessions with Admin/User view differentiation remain active as a local-dev fallback when `ZITADEL_DOMAIN` is unset.
    *   **Internal API Key Rule:** A shared internal API key MAY remain as defense-in-depth for service-to-service traffic, but it MUST NOT be treated as sufficient production authorization for privileged actions.
    *   **Add-ons:** one container per target, each speaking that target's own management API. It converges a subject onto the desired state the backend approved, reports what it did, and keeps an append-only mutation log the backend anchors — a chain cannot notice its own truncation, so somebody outside remembers where the head was.
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
Before Syndra widens its live Zitadel and provisioning surface, the immediate next milestone is to harden the contract backbone of the application. That includes strict backend request decoding, bounded domain types, stable error semantics, stronger database invariants, backend-enforced authorization assumptions, and full covering backend-first tests for mission-critical flows. This hardening pass must also make Zitadel Actions v2 the explicit and only source-of-truth compatibility boundary, while allowing separately hardened internal Syndra contracts for frontend, backend, and sync-service communication. The UI may continue to evolve, but the backend contract layer is now the highest-priority stability and security concern.

## 9. Zitadel Interaction Matrix

| Function or feature | Mechanism | Notes |
| --- | --- | --- |
| token claim enrichment for downstream applications | Actions v2 | source-of-truth claim boundary; only supported claim-integration path |
| Zitadel-native event-driven trigger logic | Actions v2 or validated backend webhook intake | detection and compatibility boundary only; business mutations stay backend-owned |
| privileged frontend-to-backend admin actions | Zitadel-issued user access token validated by backend | backend identifies and authorizes the acting user directly; shared internal API key is optional defense-in-depth only |
| backend grant or revoke operations in Zitadel | service user account | Management API path; backend-owned only |
| mapping-rule propagation back into Zitadel | service user account | requires server-side control-plane mutation rights |
| welcome-bundle assignment and similar onboarding mutations | backend service account path after validated event intake | Syndra Backend remains the single mutation authority for audit, retries, and idempotency |
| webhook reception and verification | backend endpoint with validated Zitadel event contract | external intake stays on backend; add-ons are reachable only on the internal network and never receive one |
| Backend -> add-on entitlement convergence | internal Syndra contract: signed requests over a pinned TLS channel | self-defined, authenticated, isolated from Zitadel-facing contracts, and held to a committed wire artifact both ends assert against |
| Backend -> add-on one-shot operation | same transport, plus a durable record minted before the dispatch | an operation carries a secret and is never queued or retried: a retry needs the parameters, and keeping them is the vault this design removed |
| mutation traceability, every target | outbox (`propagation_outbox`, renamed from `pending_zitadel_propagations` when a second target appeared) before every Management API call; intent ledger (`direct_role_grants`) for direct/operator grants | every Syndra-mediated mutation leaves a record before it reaches Zitadel; a Zitadel-side change with no such record is not trusted after the fact — it is detected as drift and triaged (Wave 2 · Part 4, `wave-2-part-4-zitadel-state-projection-and-drift-control`) |

## 10. IdP Chain: Google Workspace -> Zitadel

Google Workspace is the sole Identity Provider. Users authenticate via Google, which federates into Zitadel as a configured external IdP. Syndra never sees Google credentials directly.

* **Account Lifecycle Gap**: Zitadel does not auto-detect when a Google Workspace account is suspended or deleted. A future dedicated service (Phase 6, separate Docker container) will poll Google Workspace monthly via the Admin SDK Directory API to verify all Zitadel users still have active Google accounts. Suspended or deleted accounts trigger user deactivation in Zitadel via the Management API, which cascades through Syndra's existing webhook pipeline (`user_deactivated` -> cache invalidation -> the closure diff -> a convergence queued for every mapped target).
* **Scope Boundary**: Syndra does not manage the Google Workspace -> Zitadel federation configuration. That is a Zitadel admin console concern. Syndra's responsibility begins at the Zitadel webhook boundary.

## 11. The observation base: one primitive, four names

Syndra keeps being asked to tell **change** from **difference**. Those are not
the same question, and only one of them can be answered by looking at the
present.

> A difference is two values that disagree right now.
> A change is a difference **with a direction**, and direction needs a third
> value: what was there last time anybody looked.

Every surface that has needed this has invented it separately. Four exist:

| Where | What it remembers | What that lets it say |
|---|---|---|
| `merge_bases` (`services/merge`) | each managed field's value after the last successful apply | which side moved: `fast_forward`, `theirs_only`, `conflict` |
| `acknowledged_expires_at` (migration `000024`) | the expiry an operator acknowledged | the grant's date moved, so the acknowledgement is stale and the row reopens |
| `addon_log_anchors` (migration `000033`) | an add-on's mutation-log head and record count | the log was truncated or rewritten — a chain cannot notice its own truncation |
| bundle and mapping-rule versions | a published snapshot of what the set contained | what to roll back to, and what changed since |

All four are the same move: **record what you last observed, so a later
disagreement can be attributed instead of merely noticed.** A fifth surface
needing it should reach for this rather than name it a fifth thing.

### The rules that come with it

**A base must be honestly obtained.** It is what the system *observed*, never
what it *intended*. `services/drift/addon.go` drops the base entirely when the
add-on reports no current state, because a base kept across an unobserved
period would make every managed field read as "the target changed it" — one
version skew manufacturing a finding per subject.

**A base must not advance past an unresolved difference.** Recording the
target's current state as "last agreed" while a finding about it is still open
is the silent revert, arriving through bookkeeping rather than through a write.

**Absence of a base is not evidence.** `no_base` is a first-class outcome and
not a finding: nothing was observed, so nothing can be attributed, and the pass
converges exactly as it did before the mechanism existed.

### Where it does NOT belong: Zitadel

A base is an **inference device**. You keep what you last saw so you can deduce
who moved. Zitadel is event-sourced and needs no deduction — the event that
created a grant carries its editor, and
`GET /governance/drift/{id}/origin` reads it.

Given a choice between inferring an actor from a snapshot delta and reading the
actor from a log, the log wins. A base here would also lie more than it does on
a target: Zitadel has many writers — the console, Actions, other integrations,
org admins — so an observation recorded at read time is stale constantly, and
every unobserved intermediate change would collapse into one `theirs_only`. On
an add-on target the add-on is effectively the only non-human writer, which is
what makes the base trustworthy there.

**The rule: a base where there is no history; the history where there is one.**

### What is borrowed from version control, and what is not

The three-way merge is borrowed. The vocabulary and the defaults are not.

- **Git merges content; this merges authority.** In git both sides are
  legitimate contributions to a shared artifact. Here one side is policy and the
  other is state, and `take_theirs` on a grant means *ratifying access somebody
  obtained outside the system*. The classifier says what happened; it never
  implies what should happen.
- **Git auto-merges by default; this must not.** `fast_forward` resolves
  unattended for one specific reason — Syndra moved and the target did not, so
  the system is applying its own decision and no authority is added. That
  reasoning does not extend to "auto-resolve anything unambiguous", which is how
  a sweep starts granting privilege nobody approved.
  `services/merge/invariants_test.go` enumerates the permitted set and fails when
  it grows.
- **A base is not an ancestor.** Git's power is the DAG: ancestry is recorded,
  so "what came before" always has an answer. Syndra keeps one base per subject
  and field, overwritten each pass. One level of before, never a chain — and no
  vocabulary should promise otherwise.
- **The console does not speak git.** "The target was changed by hand" is better
  copy than "theirs-only conflict" for the person deciding.
  `ui/src/lib/__tests__/merge-vocabulary.test.ts` keeps version-control terms and
  the wire codes out of anything that renders.
