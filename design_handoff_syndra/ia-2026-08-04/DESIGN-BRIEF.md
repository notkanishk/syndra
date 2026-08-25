# Syndra — Design Brief

**For:** Claude Design
**Product:** Syndra — access management for an academic makerspace
**Date:** 2026-07-30
**Ask:** Fix the information architecture and specify every view, split across two views of the product — an everyday surface and a system surface.

---

## 0. Read this first

The current UI is functionally complete but organizationally chaotic. Thirteen top-level admin links across six sidebar sections, with names that overlap and don't map to how anyone thinks about the job.

The redesign's job is **not** to add features. It is to make existing capability legible, and to keep most of it out of the way so the daily case is calm.

The core object here is not a page, it's a question. Operators arrive to answer one of five:

1. Who can get into this thing?
2. What can this person get into?
3. Why does this person have this access?
4. Give this person access.
5. Why did the system do that on its own?

**Everyday** answers 1–4. **System** answers 5 and everything underneath it.

---

## 1. Naming contract

Every term below is fixed. Left column is what users see; right column is what it's called in code and API responses. Designers and engineers must be able to translate in both directions — that mapping is why the current UI drifted into jargon.

| UI label (use this) | Code / API term | What it actually is |
|---|---|---|
| **People** | `users`, `UserProfile` | Humans with accounts |
| **Project** | `project`, `ProjectCatalog` | A boundary owning a set of roles — one system you can have access to |
| **Role** | `role`, `(project_id, role_key)` | Per-project permission. Same key in two projects means different things |
| **App** (nav/headings) · "application" (prose) | `application`, `ApplicationCatalog` | OIDC client / API / SAML SP that receives the token and reads roles from it |
| **Bundle** | `bundle` | Named set of roles handed out as one unit |
| **Default for new members** | `is_welcome` | The one bundle auto-assigned on onboarding |
| **Direct access** | `DirectGrant` | One role given straight to one person, may carry expiry |
| **Automatic rule** | `MappingRule` | "Role A in project X ⇒ role B in project Y" |
| **Granted** | `source_roles` | Role someone was deliberately given |
| **Automatic** | `derived_roles` | Role produced by a rule firing. Nobody clicked it |
| **Effective access** | `effective_role_keys` | What the person can actually do right now |
| **Access source** | `RoleReason` | Why a person holds a role — see §6 |
| **Token format** | `ClaimProfile` | Claim name + format (`csv` / `array`) per project |
| **Preview token** | `ApplicationSimulation` | Dry run of exactly what claims an app would receive |
| **Unexplained access** | `DriftItem`, "drift" | Access in Zitadel that Syndra never caused |
| **Adopt in Syndra** | `attribute` → status `attributed` | Claim unexplained access as legitimate and take ownership |
| **Revoke** | `revoke` → status `revoked` | Remove it |
| **Owned elsewhere** | `mark-external` → `marked_external` | A known integration owns this — stop flagging it |
| **Pending changes** | `PendingPropagation` | Queued Zitadel writes awaiting confirmation |
| **Change history** | `CascadeSummary`, "cascades" | What a bundle/rule change actually did downstream |
| **Access map** | `TopologyGraph` | Visual graph of projects, roles, bundles, rules |
| **Identity provider** | Zitadel | Upstream system that issues tokens and owns authorization state |
| **Hardware sync** | `ProvisioningIntent`, LLDAP | Legacy hardware that can't speak OIDC |
| **Event activity** | webhook events, onboarding triggers | Inbound system activity |
| **Request** | `AccessRequest` | Member asks for access; operator decides |

Retired names, do not reuse: *Users & Access*, *Policy Engine*, *God Mode*, *Mapping Rules*, *Drift*, *Zitadel Diagnostics*, *Operations*, *Grants*, *Attribute*.

---

## 2. The two views

**Naming decision needed from the product owner.** The original request specified "Basic" and "Advanced." Those labels sort *people* by competence rather than sorting *work* by kind — nobody wants to self-identify as basic, and the operator doing a quick bundle assignment isn't being basic, they're being fast. Recommendation is **Everyday** and **System**. This brief uses Everyday/System throughout; it's a find-and-replace to revert if you disagree.

Internally the preference is `ui_view` with values `everyday` | `system`. **It must not be called `mode`** — `GET /api/v1/system/mode` already exists and reports whether the backend is on demo or live Zitadel data. Two different "modes" in one product is how this mess started.

| | **Everyday** | **System** |
|---|---|---|
| Who | Student staff, lab managers, anyone doing daily access work | The one or two people who own the system |
| Mental model | "People and what they can use" | "The machine that decides who gets what" |
| Cadence | Daily, in a hurry, laptop in a noisy room | Weekly, deliberate, at a desk |
| Fear | Giving the wrong person laser cutter access | Silent drift, cascades firing wrong, malformed tokens |
| Jargon tolerance | Zero | High — they wrote the jargon |

### Switching rules

1. **The URL does not change.** A person's detail page is the same route in both views. Switching reveals additional panels and actions in place; it never navigates you somewhere else. Losing your place is the fastest way to make a mode switch feel punitive.
2. **No dead ends.** If something can only be resolved in System, Everyday says so plainly and offers a scoped jump: "This came from an automatic rule — **Open automation details**." That jump returns to the same person/project context, not to a bare list.
3. **Everyday is not System-minus-features.** It's a smaller job done completely.

---

## 3. Navigation — the stable contract

**Structure never moves. Only badge values change.** The current sidebar conditionally injects a Drift section at the top when the count is non-zero, so every other item shifts down underneath the user. That is prohibited. If a section has nothing in it, it renders with a zero/absent badge — it does not disappear, and nothing else appears above it.

Same rule in both views. Switching Everyday → System *appends* sections; it does not reorder or rename what was already there.

### Sidebar

```
EVERYDAY                    SYSTEM (everything in Everyday, plus)
─────────────               ────────────────────────────────────
Today                       Today
People                      People
Access                      Access
  · Projects                  · Projects
  · Roles                     · Roles
  · Apps                      · Apps
Requests            [n]     Requests                        [n]
                            Bundles
                            Automation
                              · Automatic rules
                              · Pending changes              [n]
                              · Change history
                              · Access map
                              · Settings
                            Review
                              · Unexplained access           [n]
                              · Expiring access              [n]
                              · Audit
                            System
                              · Identity provider
                              · Hardware sync
                              · Event activity
```

### Member sidebar

**System is operator-only. It does not exist for members** — not greyed out, not present-but-403. The Everyday/System switch is not rendered for them at all.

Members get two destinations, and that is deliberate:

```
MEMBER
──────────────
My access
Requests            [n]
```

- **My access** — the E3 person view scoped to themselves: their bundles, their roles grouped by project, each with its Access source in plain language. This is the one screen that explains to a member why they can badge into the laser lab. It replaces today's vaguer "My Services."
- **Requests** — submit a request, track its state. Backend self-filters the list for non-operators, so there is nothing to hide client-side.

No Today for members: with two destinations there is no navigation problem to solve, and a work queue for someone with no queue is an empty room. **My access** is the landing route.

**Today** is the landing page for operators — not "Dashboard," not "Overview," both of which promise a summary and deliver a link farm. Today shows **actionable work only**: open requests, pending changes awaiting confirmation, direct access expiring soon, unexplained access needing triage. If there's nothing to do, it says so and gets out of the way. In Everyday it shows only requests and expiring access.

Today has a single backing endpoint that returns exactly these four things plus advisory hints — `GET /api/v1/governance/summary` → `GovernanceSummary` (`pending_requests`, `expiring_grants`, `cleanup_hints`, `pending_propagation`, `drift`). Note `pending_propagation.zitadel_reachable` — when false, the "Resume now" action must be disabled with the reason shown, not silently fail.

**The sidebar must not consume that endpoint.** It needs the same four signals as bare integers, and today it gets them by fetching the full summary and taking `.length` of the returned arrays — downloading every pending request and expiring grant object on each mount to render two numbers. Engineering should add a compact **`GET /api/v1/governance/indicators`** returning scalars only:

```json
{ "pending_requests": 3, "expiring_grants": 1, "pending_propagation": 0, "drift": 12 }
```

Sidebar polls indicators; Today consumes the full summary. This matters for design because the sidebar's badges should be cheap enough to refresh often, while Today's content is heavier and can refresh on view.

**People and Access need real sub-navigation.** Everyday covers projects, role membership, people, direct access, bundle assignment, and app token work — that is a lot of ground for three nav entries. Don't hide it behind generic landing pages: Access carries explicit Projects / Roles / Apps sub-nav, and People uses in-page tabs on the person detail. The sidebar stays shallow; local navigation does the work.

### Route → home matrix

Every existing route gets exactly one home. No route appears twice.

| Route | Member | Everyday | System | Notes |
|---|---|---|---|---|
| `/` | **My access** | Today | Today | Members land on their own access view; operators get the work queue |
| `/users` | — | People | People | Person detail is the highest-traffic screen |
| `/users/{id}` | *self only* | People › detail | People › detail | Member's own record is the same view, scoped |
| `/projects` | — | Access › Projects | Access › Projects | |
| *(new)* `/roles` | — | Access › Roles | Access › Roles | Cross-project role index — the landing target for the sidebar item. **Partially backed** — §5 |
| *(new)* `/projects/{id}/roles/{key}` | — | Access › Roles | Access › Roles | Role detail + members. **Needs new endpoint** — §5 |
| `/applications` | — | Access › Apps | Access › Apps | Includes token format + preview |
| `/requests` | **Requests** | Requests | Requests | Members see own only — backend self-filters |
| `/bundles` | — | — *(assign from People)* | Bundles | See split below |
| `/policies` | — | — | Automation › Automatic rules | Legacy path — see URL note |
| *(new)* `/automation/settings` | — | — | Automation › Settings | Global confirmation default (S2b) |
| `/governance/pending` | — | — | Automation › Pending changes | |
| `/operations/cascades` | — | — | Automation › Change history | |
| `/graph` | — | — | Automation › Access map | |
| `/governance/drift` | — | — | Review › Unexplained access | |
| *(new)* `/review/expiring-access` | — | — | Review › Expiring access | Dedicated route, not an audit tab — see below |
| `/audit` | — | — | Review › Audit | |
| `/grants` | — | *(gone — redirects)* | *(gone — redirects)* | 301 → `/governance/drift?tab=reconciliation`. See below |
| `/zitadel` | — | — | System › Identity provider | |
| `/operations` | — | — | System › Event activity | |
| *(hardware)* | — | — | System › Hardware sync | Parked — §5 |
| `/login` | unauthenticated | unauthenticated | unauthenticated | |

Any route with `—` in the Member column is **not rendered and not reachable** for members; the backend returns 403 on the underlying reads regardless.

**Expiring access gets its own route, not an `/audit` tab.** Audit is a historical record you consult; expiring access is time-boxed work you act on before a deadline. Nesting the second inside the first buries a deadline in a log, and makes the sidebar badge point at a tab rather than a destination. They share no filters and no mental mode.

**URL note (optional, not blocking design).** Several legacy paths no longer match their nav labels — `/policies` under Automation, `/graph` for Access map, `/operations` for Event activity. New routes in this brief follow the group they live in (`/automation/…`, `/review/…`). Aligning the legacy ones is a phase-2 cleanup behind redirects; it does not change any design here.

**`/grants` ceases to exist as a destination.** It is the one route in the matrix that doesn't map cleanly, because it currently hosts two tabs doing unrelated jobs. Resolution:

- *All grants* (every user/project/role across sources) — **removed**. It is redundant with People and role membership, which answer the same question with the Access source attached. Nothing to redirect; the capability is absorbed.
- *Reconciliation* (the Syndra ↔ Zitadel diff, for spotting drift before it widens) — **relocated** to Review › Unexplained access as a second tab.
- Legacy `/grants` issues a **301 to `/governance/drift?tab=reconciliation`**. Reconciliation is the only part with no other home, so it is the correct landing target; anyone arriving from an old bookmark for the all-grants view lands somewhere adjacent and legible rather than on a 404.

After this, "every route has exactly one home" holds with no exceptions.

**Bundles split by verb, not by view:**
- **Assign / unassign a bundle to a person** — Everyday, from the person's detail page. This is core daily work.
- **Create, edit, delete a bundle; set the default for new members** — System › Bundles. These change what every holder gets.

The rule generalizes: *acting on one person* is Everyday; *changing the machine that acts on everyone* is System.

---

## 4. Views

### Everyday

#### E1 · Projects → roles
"What roles exist for the laser lab?"

Projects list: name, member count, active role count, apps served. Drill into a project for its roles — display name, role key, description, group, and a **member count linking to E2**. Show provenance when a role was cloned (`cloned from <project>/<role>`).

`GET /api/v1/projects` → `ProjectSummary` · `GET /api/v1/roles`

#### E2 · Roles index and role → members
"What roles exist?" then "Who can currently use the laser cutter?"

**`/roles`** is a cross-project index — every role with its project, key, display name, group, and member count. Filter by project and group. This is where the sidebar's Roles item lands. Clone provenance shows when present.

**`/projects/{id}/roles/{key}`** is the role detail: everyone who effectively holds that `(project, role)`, each row carrying its **Access source** (§6).

**Removal actions here are source-specific. There is never a generic "Revoke role."** A person can hold one role through several sources at once, so a generic action is ambiguous at best and destructive at worst. Name the action after the thing being removed, and state the residual outcome:

| Source | Action | Confirmation must say | Backed? |
|---|---|---|---|
| `direct` | **Remove direct access** | "They will still retain this role via *Lab Tech*." — or, if this was the only source, "They will lose this role." | **No — §5** |
| `bundle` | **Remove bundle assignment** | Which bundle, and every *other* role they'd lose with it | Yes |
| `mapping` | *(no removal here)* | "This is automatic, from *3D Lab / operator*." → link to the rule | n/a |

The residual-outcome sentence is not optional garnish — it is the difference between a safe click and an outage on the laser cutter.

**⚠ Endpoints:** role→members does not exist; the `/roles` index is only partially backed. §5.

#### E3 · Person → access
The highest-traffic screen in the product.

Header: name, email, title, team. Then **grouped by project**, granted and automatic roles shown separately, every role carrying its Access source. `cleanup_hints` from the API render as gentle advisory notes, never errors.

**Bundle chips are not individually removable.** Chips communicate membership and nothing else — no inline ✕. Removal lives behind an explicit **Manage bundles** control (or an overflow menu) that opens the assign/unassign surface with impact preview. Inline removal invites a misclick that silently strips a dozen roles, and it stops scaling the moment someone holds four bundles.

Per-role removal follows the source-specific rules in E2 — never a generic revoke.

In System this same page gains: raw grant ids, cascade lineage, hardware sync state — appended in place, same URL.

`GET /api/v1/users/{id}/access` → `UserAccessView`

#### E4 · Assign a bundle
Reached from **Manage bundles** on the person's detail page (and from the bundle itself, in System). Before confirming, show **what this actually grants** — bundles expand to roles, and rules may cascade further.

Unassigning shows the same preview in reverse, and must distinguish roles that will actually be lost from roles the person retains through another source.

`POST /api/v1/users/{id}/bundles` · `DELETE /api/v1/users/{id}/bundles/{bundleId}` · `GET /api/v1/bundles/{id}/impact` · `GET /api/v1/bundles/{id}/roles`

#### E5 · Grant direct access
Project → role → optional expiry. Expiry is real (a sweep auto-revokes), so the date field deserves proper design and "expires in 3 days" states need to be visible on the person's page. Operator-only; members get 403 by design, so don't render the affordance for them.

`POST /api/v1/users/{id}/grants` · `GET /api/v1/users/{id}/grants` → `DirectGrant`

#### E6 · App token — format and preview
"This app isn't seeing the roles it expects. What is it actually getting?"

One screen, two halves:
1. **Token format** — per project, set claim name and format (`csv` / `array`).
2. **Preview token** — pick an app and a person, see exactly what would be delivered: raw roles and final claim payload.

The most technical thing in Everyday, and it earns its place by being the fastest way to debug "my app is broken." Preview output should read like a receipt — monospace, copyable, unambiguous.

`GET /api/v1/applications` → `ApplicationView` · `GET /api/v1/applications/{id}/simulate?user=` → `ApplicationSimulation`

#### E7 · Requests
Submit (members) and approve/deny (operators). Members see only their own — enforced at the backend, so the UI must match.

`GET/POST /api/v1/requests` · `POST /api/v1/requests/{id}/decision`

### System

#### S1 · Bundles
Create, edit roles within, set the default for new members, view impact before changing. Bundle edits cascade to every holder — the impact preview is the safety rail and belongs above the fold, not in a footnote.

`GET/POST /api/v1/bundles` · `POST /api/v1/bundles/{id}/roles` · `DELETE /api/v1/bundles/{id}/roles/{projectId}/{roleKey}` · `PUT /api/v1/bundles/{id}/welcome` · `GET /api/v1/bundles/{id}/impact`

#### S2 · Automation › Automatic rules
"Role A in project X ⇒ role B in project Y." Includes validate-before-save and per-rule confirmation mode (`auto` applies immediately, `manual` queues for review).

`GET/POST /api/v1/rules/mapping` · `PUT /api/v1/rules/mapping/{id}` · `POST /api/v1/rules/mapping/validate` · `POST /api/v1/policies/confirmation-mode`

#### S2b · Automation › Settings
Route: `/automation/settings`.

The **global default confirmation mode** — whether new rules apply automatically or queue for review by default.

This is system-wide policy affecting every future cascade, so it gets a stable home inside Automation. It must **not** live in the sidebar footer, an account menu, or a preferences drawer: those locations read as personal settings, and someone will flip an org-wide policy thinking they changed something about themselves.

`GET/PUT /api/v1/config/confirmation-mode-default`

#### S3 · Automation › Pending changes
Queued Zitadel writes awaiting confirmation, with manual drain.

`GET /api/v1/propagations` · `POST /api/v1/propagations/drain`

#### S4 · Automation › Change history
What a bundle or rule change actually did downstream.

`GET /api/v1/propagations/cascades` → `CascadeSummary`

#### S5 · Automation › Access map
Visual graph of projects, roles, bundles, rules. Currently the least legible screen despite being the most conceptually valuable — this is the one view that could genuinely explain the system to a newcomer. Worth real attention.

`GET /api/v1/topology` → `TopologyGraph`

#### S6 · Review › Unexplained access
The highest-stakes screen in the product. Two tabs:
1. **Triage queue** — each item is access found in Zitadel that Syndra cannot explain. Three resolutions: **Adopt in Syndra**, **Revoke**, **Owned elsewhere**. Bulk adopt and manual reconcile supported.
2. **Reconciliation** — the Syndra ↔ Zitadel diff (relocated from `/grants`), for spotting drift before it widens.

Each row must make "what is this, and what happens if I revoke it" answerable in about two seconds. If one screen gets extra scrutiny, it's this one.

`GET /api/v1/governance/drift` · `POST /api/v1/governance/drift/{id}/{attribute|revoke|mark-external}` · `POST /api/v1/governance/drift/bulk-attribute` · `POST /api/v1/governance/drift/reconcile` · `GET /api/v1/reconciliation/grants`

#### S7 · Review › Expiring access
Route: `/review/expiring-access`.

Direct grants approaching expiry, soonest first, with the person, project, role, and remaining time. Currently only a badge on the audit log — it earns its own destination because it is the one review item with a deadline, and because a sidebar badge should point at a page rather than a tab.

**There is exactly one action here: extend.** Doing nothing is not an action and must not be drawn as one — no "Let it lapse" button, no dismiss, no secondary control that looks like it submits something. The row's default state simply reads **"No action — expires 14 Aug"**, and expiry then happens on its own via the sweep.

Extending is backed today: `POST /api/v1/users/{id}/grants` upserts on `(user, project, role)` and overwrites `expires_at`, so re-submitting with a later date renews in place rather than creating a duplicate.

If operators later need to record *"I've seen this and I'm deliberately letting it go"* so it stops drawing attention, that is a genuine new capability — an acknowledged/deferred state with its own column and endpoint. Do not fake it client-side by hiding rows, which would make a shared queue diverge per operator. Flag it if design finds the need; don't assume it.

`GET /api/v1/governance/summary` → `expiring_grants` (until a dedicated paginated endpoint exists) · `POST /api/v1/users/{id}/grants` to extend

#### S8 · Review › Audit
Who did what, when.

`GET /api/v1/audit`

#### S9 · System › Identity provider
Live Zitadel health, direct project/role/grant inspection, action signing-key rotation status. Read-mostly with direct-write escape hatches.

`GET /api/v1/zitadel/{health,projects,users,grants,action-rotation-status}` and nested variants

#### S10 · System › Hardware sync
Provisioning intents for the sync service, plus per-user shadow credentials for hardware that can't speak OIDC. **Parked** — design an honest "not connected yet" state.

`GET /api/v1/intents` · `GET/PUT/DELETE /api/v1/users/{uid}/shadow-credential*`

#### S11 · System › Event activity
**A raw timeline, not a dashboard.** Inbound webhook events and onboarding triggers in time order, filterable, drillable to payload. Its job is forensic — "what did the identity provider tell us at 14:12, and what did we do about it."

No summary counts, no tiles, no roll-ups. Today already owns actionable summary (`GET /api/v1/governance/summary` belongs there, see §3); a second dashboard here would split attention and make it ambiguous which one is authoritative.

`GET /api/v1/webhook/events` · `GET /api/v1/onboarding/triggers`

---

## 5. Gaps to design around

1. **Role → members has no endpoint** (E2). `POST /api/v1/lookup` is only an id→display-name resolver; `GET /api/v1/bundles/{id}/impact` covers bundles only. There is no reverse role→members query anywhere in `services/views.go`. Design the view; engineering adds the endpoint. Assume a response shaped like `BundleImpact` — role identifier plus users with their access sources.
2. **The `/roles` cross-project index is only partially backed.** `GET /api/v1/roles` calls `GetAllLocalRoles`, which returns *only roles created through Syndra*. Roles that exist in Zitadel but were not created here are invisible to it. Complete coverage needs either a new aggregate endpoint or per-project fan-out over `GET /api/v1/zitadel/projects/{id}/roles`. Until then the index is honestly incomplete — **design an explicit "Syndra-managed roles" scope indicator** rather than implying the list is exhaustive. Silently partial lists are how people conclude a role doesn't exist and create a duplicate.
3. **Removing a direct grant has no endpoint.** `/api/v1/users/{id}/grants` supports only `GET` and `POST` (upsert). Syndra's `direct_grants` rows are otherwise deleted only by the expiry sweep. The `DELETE /api/v1/zitadel/users/{id}/grants/{grantId}` that does exist removes the **Zitadel-side** grant, which is a different object — using it would leave the Syndra row behind and the next cache compile would put the access back.

   This blocks **Remove direct access** in E2 and E3. Design the flow; engineering adds `DELETE /api/v1/users/{id}/grants/{grantId}`. Until it exists, the action must not be rendered — a revoke button that silently doesn't revoke is worse than no button on a screen about who can operate a laser cutter.
4. **No compact indicators endpoint.** The sidebar needs four integers; the only source today is the full `governance/summary` payload. Engineering should add `GET /api/v1/governance/indicators` (§3). Design can assume it exists.
5. **Hardware sync is parked** pending LLDAP integration. Needs a real disconnected state, not a spinner.
6. **Apps and projects are not 1:1.** A project can serve several apps. Don't design as though each project has exactly one.

---

## 6. Access source — the signature component

The "why does this person have this" explanation appears in E2, E3, unexplained-access triage, and change history. It is the single element that will do most to make this product feel calm. Design it once, properly, with a compact form (inline chip) and an expanded form (popover or detail row).

The data model emits exactly **three** kinds from `ExplainUserAccess`, so the vocabulary is fixed and small:

| `RoleReason.Kind` | Label | Expanded text | Revocable here? |
|---|---|---|---|
| `direct` | **Direct** | "Granted directly" + who/when, expiry if set | Yes |
| `bundle` | **Via bundle** | "From the *Lab Tech* bundle" → links to bundle | No — remove the bundle instead |
| `mapping` | **Automatic** | "Automatic — from *3D Lab / operator*" → links to the rule | No — change the rule instead |

Consistency is the whole point: the same three labels, same colors, same order, everywhere they appear. A role can carry more than one source (granted directly *and* via a bundle) — the component must handle multiples without turning into a wall of chips, and E4's removal warning depends on exactly this case.

*(Note: `views.go` also emits kinds `project`, `role`, `contains`, `application`, `bundle`, `rule` — those belong to the topology graph and are a separate namespace. They are not access sources and must not use this component.)*

---

## 7. Cross-cutting requirements

**Two permission classes.** **Operator** (full) and **member** (self-service only). The backend enforces at the trust boundary — cross-user reads return 403 — so never render an affordance that will fail. Member navigation is specified in §3; System is operator-only and is not rendered for members at all.

**Badges.** Live counts on unexplained access, expiring grants, pending requests, pending changes. In Everyday, only requests and expiring access. Badge values change; sidebar structure does not (§3).

**Four states minimum per list view:** empty, loading, error, and **degraded**. Degraded is real and specific: `GET /api/v1/system/mode` returns `degraded: true` when Zitadel is configured but the backend fell back to demo data — at which point every number on screen is fiction. That needs a persistent, unmissable banner.

**Stack.** Next.js App Router + Tailwind, Bun, dark-first with a light theme. Existing component layer: Modal, Drawer, shared focus-trap hook, toasts, and an async name-resolver context that turns ids into display names. Reuse it. Because resolution is async, **every id-bearing surface needs a resolving state** — never flash raw UUIDs.

**Accessibility, non-negotiable.** Keyboard navigation, dialog focus traps, contrast in both themes. There's a standing per-route manual checklist (empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast) — design to pass it.

---

## 8. Deliverables

1. Sitemap and sidebar for all three audiences — Everyday, System, member — honoring the stable-structure rule (§3).
2. The view switch: placement, affordance, and how Everyday handles System-only dead ends without losing context.
3. High fidelity: E1–E7. Everyday is the priority — it's the daily surface.
4. Mid fidelity: S1–S11, with S6 (unexplained access) treated as high-stakes.
5. **Access source** component, compact and expanded, all three kinds, multiples handled.
6. **Source-specific removal flows** (§E2) — the action naming and residual-outcome confirmation, applied consistently in E2 and E3.
7. Empty / loading / error / degraded for every list view.
8. Member surface (§3) as a first-class audience, not a leftover.

**In-repo reference:** `openspec/INDEX.md` (capability specs) · `openspec/NEXT.md` (open work) · `openspec/changes/syndra-core-architecture/design.md` (architecture) · `backend/internal/models/models.go` (every response shape named here).
