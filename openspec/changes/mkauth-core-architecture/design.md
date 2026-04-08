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
*   **Zero-Trust Context:** The MkAuth backend inherently distrusts the UI. It validates the logged-in admin's personal permissions first before executing mutations via the Service Account.

### The Data Plane (Fast & Dumb)
*   **Components:** Local Redis Cache, Zitadel Actions v2.
*   **Behavior:** During a user login flow to a downstream application, Zitadel triggers an Action v2 script. The script pings MkAuth's fast Redis endpoint, which returns pre-compiled, flattened roles filtered specifically for that application. The Action injects these roles directly into the JWT.
*   **Cache Invalidation:** A webhook listener actively updates Redis the moment an out-of-band change happens in Zitadel, ensuring the cache is never stale.

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
*   **View Differentiation (Admin vs User):**
    *   **Admin View:** Focused on projects, raw roles, mapping rules, and global auditing. Authenticated via user session + admin role check.
    *   **User Portal:** Focused on the **Service Catalog** (Applications). Standard users see a simplified view showing only what services they have access to and a simplified "Request Access" flow.
*   **Role Assignment UX:** "Assign once, propagate correctly." Admins can assign raw roles (advanced) or Bundles (normal users). Selecting a Bundle explicitly previews exactly which underlying roles will be applied.
*   **Audit Logging & Self-Service:** Strict timeline tracking of Who granted What, and When. Temporary/Semester roles auto-expire, and users can trigger self-service permission requests for admin approval.

## 6. Technical Stack & Deployment
*   **Deployment:** Docker Compose running inside a Proxmox LXC (Linux Container). This provides a robust 1-command installation and update mechanism via an `update.sh` script that pulls GitHub changes and restarts the stack without downtime. The stack uses **Separate Containers** for Frontend and Backend to enable independent scaling and clear authentication boundaries.
*   **Authentication & Identity:**
    *   **Backend:** Authenticates via a high-scoped **Machine-to-Machine (Service Account)** token from Zitadel. It exposes a JSON API secured by an internal API key.
    *   **Frontend:** Authenticates via **User Session (OIDC)**. The frontend proxies requests to the backend using its own service credentials, but filters data based on the logged-in user's identity and permissions.
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
*   [/] **Zitadel Integration**: Currently stubbed for local dev; needs M2M credentials for live sync.
*   [ ] **Production Rollout**: Final deployment with actual keys, networking, and live Zitadel credentials.
