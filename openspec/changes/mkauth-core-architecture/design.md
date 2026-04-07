# MkAuth Core Architecture Design

## 1. Mission & Core Philosophy
* **Zitadel is the Absolute Source of Truth:** Zitadel holds the final, authoritative list of users and roles. MkAuth sits on top as an orchestration and policy layer.
* **Purpose:** To simplify and manage complex hierarchical permissions across various makerspace systems (digital SSO and physical access) without bloating Zitadel with complex mapping logic.

## 2. The Architectural Split
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

## 5. UI, UX, & Governance
*   **Aesthetic & Style:** Inspired by Linear/Stripe. Clean, enterprise-grade layout using distinct cards, soft shadows, and moderate whitespace to prevent intimidating non-technical staff. Dark/light modes prominently feature **vibrant accent colors** to highlight primary actions.
*   **Role Assignment UX:** "Assign once, propagate correctly." Admins can assign raw roles (advanced) or Bundles (normal users). Selecting a Bundle explicitly previews exactly which underlying roles will be applied across which projects before confirmation, ensuring total transparency. Power users can trigger this instantly via a `⌘K` Command Palette.
*   **Secondary Visual Graph View:** A dedicated "God-Mode" page using node-based visual logic (e.g., React Flow) lets admins see a macroscopic, overhead view of how all Rules, Projects, and Roles interconnect across the entire makerspace.
*   **Governance & Cleanup:** The system routinely flags **Cleanup Suggestions** (unused roles, redundant mappings) and **Least-Privilege Hints** (detecting users with excessive or unused permissions).
*   **Audit Logging & Self-Service:** Strict timeline tracking of Who granted What, and When. Temporary/Semester roles auto-expire, and users can trigger self-service permission requests for admin approval.

## 6. Technical Stack & Deployment
*   **Deployment:** Docker Compose running inside a Proxmox LXC (Linux Container). This provides a robust 1-command installation and update mechanism via an `update.sh` script that pulls GitHub changes and restarts the stack without downtime.
*   **Backend / Orchestrator:** Go (Golang) running in a dedicated container. Chosen for its strict security boundary and low-latency response times for the Data Plane.
*   **Frontend Dashboard:** Next.js (React) running in a dedicated container.

## 7. Current Implementation Status (v1.0-Bundles)
*   [x] **Secure Data Plane**: Bearer token authentication enforced on all routes.
*   [x] **Smart Cache Compiler**: Iterative, transitive role resolution in Go.
*   [x] **Mapping Rules**: Rule management API and UI.
*   [x] **Bundles**: Role aggregation and User-Bundle assignments.
*   [x] **Cycle Detection**: DFS validation on rule creation.
*   [x] **Audit Logging**: Mandatory tracking for all admin actions.
*   [/] **Zitadel Integration**: Currently stubbed for local dev; needs M2M credentials for live sync.
*   [ ] **Visual Graph View**: Roadmap feature for v1.1.
