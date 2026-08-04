# Wave 2 · Part 2 — Backend Coherence Design

**Status:** Approved for implementation
**Parent design:** [`docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md`](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 3
**Sister documents:** [Wave 1 design](../wave-1-production-trust-hardening/design.md), [Wave 2 · Part 1 design](../wave-2-part-1-frontend-palette-finalization/design.md)

---

## 1. Aim

Six audit findings (B3, B5, B6, B7, C4, C5, C11, D8) all live in the Go backend, all rate as "internal seams that hurt readability or repeat work", and all are prerequisites for Wave 2 · Part 4 (Theme 2 — drift control). This wave resolves them in one coordinated pass, preserving every observable contract while leaving the substrate that Theme 2 needs.

The wave is decomposed into six independently-shippable refactors. Order is chosen to minimise blast radius: the smallest, lowest-coupling fix lands first; the largest behavioural refactor lands last. Each refactor passes the existing test suite as a hard gate before the next begins.

---

## 2. Design decisions

Six load-bearing decisions; everything else follows.

### Decision 1 — `accessSnapshot` is a request-scoped value, not a global cache

`collectUserRoles(ctx, userID)` is the single helper that walks `direct_role_grants ∪ bundle expansions ∪ mapping-rule outputs` for one user. It is fast (≈4 SQL queries) but called repeatedly by views that iterate users: `ListUsers` calls it N times, `ListApplications` does N per app (N×M), `ListProjects` does N per project (N×P), `Governance` calls `BundleImpact` which calls it again. For a 200-user makerspace with 20 apps and 8 projects, a single `/api/v1/applications` request can issue ≈16 000 SQL queries.

The fix is a request-scoped helper, not a process-wide cache:

```go
type accessSnapshot struct {
    ctx   context.Context
    users []models.UserProfile  // primed once
    roles map[string]userRoles  // userID → (roleMap, bundles); lazy
}

func newAccessSnapshot(ctx context.Context) (*accessSnapshot, error) { /* prime users */ }
func (s *accessSnapshot) For(userID string) (userRoles, error)        { /* lazy collectUserRoles */ }
```

The snapshot lives for the lifetime of one HTTP request. No mutex, no global state, no invalidation hazard — when the request returns, the map is GC'd. Each view function takes `*accessSnapshot` as its first parameter; the entrypoints (`ListUsers`, `ListApplications`, `ListProjects`, `Topology`, `Governance`) construct one at the top and thread it down.

The snapshot deliberately does **not** memoise `directory.Default.Users(ctx)`, `Projects(ctx)`, or `Applications(ctx)` outputs — those are already cached inside the directory layer (`directory/zitadel.go` overlay cache). Adding another caching layer here would just shadow the cache that already exists.

### Decision 2 — `repositories.go` splits into 11 files, not 9

The audit's enumerated list (`bundles, grants, rules, webhooks, vault, intents, roles, onboarding, audit`) names 9 files. The current `repositories.go` contains 9 banner-commented sections that map cleanly to those, **plus three sections the audit didn't enumerate**: `ACCESS REQUESTS` (78 lines), `CLAIM PROFILES` (66 lines), and `ZITADEL GRANTS INDEX` (77 lines).

These three need a decision:

| Section | Audit treatment | Decision |
|---|---|---|
| `ACCESS REQUESTS` (lines 377–454) | Not enumerated. | **Own file: `access_requests.go`.** Distinct domain — approval flow, not grant storage. Folding into `grants.go` would muddy a clean boundary. |
| `CLAIM PROFILES` (lines 456–522) | Not enumerated. | **Own file: `claim_profiles.go`.** The C5 Redis-cache wrapper lands here too. Separate from `roles.go` (it shapes claim *output*, not role catalog). |
| `ZITADEL GRANTS INDEX` (lines 771–847) | Not enumerated. | **Fold into `webhooks.go`.** The grant index is webhook-maintained: it's written from `maintainGrantIndex` (a webhook-only function) and read by webhook enrichment. They change together. |

Result: 11 domain files instead of 9. Function bodies and exported names are bit-identical; only the file each function lives in changes. Callers (handlers, services) are untouched. `gofmt -d` after the split shows zero diff inside function bodies.

### Decision 3 — `auth.Principal` is the authentication primitive, not a bag of claims

`auth.ValidateToken` today returns just the subject string. `auth.HasProjectRole(rawToken, roleKey)` re-parses the JWT payload by hand (base64 + JSON unmarshal) to read project roles. The audit (C4) flags this double-parse.

The fix is not "stash the raw token in context" — that would just defer the second parse. The fix is to expose the parsed shape:

```go
type Principal struct {
    Subject      string
    ProjectRoles map[string]struct{}  // role keys; set membership only
}

func (p *Principal) HasProjectRole(key string) bool {
    _, ok := p.ProjectRoles[key]
    return ok
}

func Validate(ctx context.Context, tokenStr, domain, audience string) (*Principal, error)
```

`auth.HasProjectRole(rawToken, ...)` — the standalone function — is **deleted**, not deprecated. There's exactly one caller (`withOperatorAuth`); deprecation buys nothing.

Context plumbing:
- `withUserAuth` calls `auth.Validate`, then `r = r.WithContext(withPrincipal(r.Context(), p))`.
- `withOperatorAuth` reads via `principalFromContext(ctx)`; falls through to the existing dev-mode skip if `ZITADEL_DOMAIN` is unset.
- The existing `getAdminUserID(ctx)` helper is preserved as a thin wrapper: `return principalFromContext(ctx).Subject` (returns `""` when no principal). All downstream callers keep their current API.

The `ProjectRoles` field deliberately ignores the `{orgId: orgName}` value side of the Zitadel claim. `HasProjectRole` only checks key presence — value preservation is YAGNI today.

### Decision 4 — Claim-failure-mode cache is read-through with 5-minute TTL; cache-on-success only

The C5 audit finding wants: "Cache last-known `claim_failure_mode` per project in Redis next to the claim payload. `GetClaimFailureMode` returns the cached value on transient DB error rather than silently defaulting to `fail_closed`."

Design:

```
key:   claim_mode:<projectID>            (sibling of mapping:<userID>:<projectID>)
value: {"mode":"fail_closed","minimal_safe_claims":{...}}   (JSON-encoded)
TTL:   5 minutes                         (env-overridable: CLAIM_MODE_CACHE_TTL_SECONDS)

read path (claimFailureModeRead):
    redis GET → parse → return            (cache hit; no DB call)
    redis miss / decode error →
        db.GetClaimFailureMode →
            ok       → redis SETEX, return
            db error → redis GET stale → return cached if any, else fail_closed
```

Cache invalidation: **none**, because no `claim_profiles` UPDATE path exists in the codebase today (seeded at install time; manually edited via SQL when needed). A 5-minute TTL bounds staleness for the rare manual edit. When a write path is added (future Theme), it must `DEL claim_mode:<projectID>`.

Cache placement: the helper lives in `handlers/action.go` (or a new sibling `claim_mode_cache.go`), not in `db/claim_profiles.go`. Rationale: the cache is a **data-plane concern** for the Zitadel Actions v2 hot path; the `db` package stays Redis-free. Two new injectables in `deps.go` (`redisGetClaimMode`, `redisSetClaimMode`) make the cache testable without a live Redis instance.

Failure modes:
- Redis unreachable + DB succeeds → DB result returned, cache write fails silently (logged). Subsequent requests pay the DB cost.
- Redis unreachable + DB fails → `fail_closed` (no degradation from today).
- Redis cached + DB fails → cached value returned (the new behavior; the entire point of C5).
- Redis stale + DB succeeds → DB wins; cache refreshed. No stale-pinning.

The 50ms Zitadel Actions latency budget (`redisTimeout` at `action.go:57`) covers the cache fetch comfortably — Redis GET round-trip is sub-millisecond on the LXC deployment.

### Decision 5 — `event_type` strict; `dropped_enrichment_incomplete` becomes observable, not 400

The audit (B6/C11/D8) says: "Missing `event_type` returns 400. Zitadel-shape with missing `source_project` returns 400 unless the event is one of the documented 'no source project' types (e.g. `user.added`)."

`event_type`: clear. Remove the silent `EventType = "grant_added"` default; missing → 400. Internal-shape only — `translateZitadelEvent` always resolves the type before the strict check.

`source_project` missing for Zitadel-shape grant events: **the audit and the current code disagree**.

- Audit reading: "returns 400".
- Current code (`webhook.go:104-109`): returns **200** with a log line, citing redelivery-storm prevention — Zitadel re-fires 4xx-rejected webhooks on a back-off schedule, and a grant event whose enrichment is unresolvable (the most common case is a `grant.removed` for an already-gone aggregate where neither the local index nor `ListUserGrants` can find the role keys) will redeliver forever.

The current code's comment is operator wisdom from the live-webhook-listener change (Phase 3) — not a defensible silent default, but not arbitrary either. The audit's "returns 400" wording does not address the storm concern.

**Decision:** keep the 200-ack — the storm risk is real — but make the outcome observable. The webhook event is persisted with a new status enum value `dropped_enrichment_incomplete`, distinct from `success` / `failed` / `duplicate`. Operators querying `/api/v1/webhook/events?status=dropped_enrichment_incomplete` see exactly the events the audit's "silent default" critique was pointing at.

Concretely:
- `webhook_events.status` already accepts arbitrary strings (`DEFAULT 'pending'`). No migration; just a new value emitted from `dbDropWebhookEventEnrichmentIncomplete(ctx, ...)`.
- The log line at line 105 stays (operators still see it in stdout).
- A new test `TestWebhook_ZitadelGrant_EnrichmentIncomplete_PersistedAsDropped` asserts the row appears with the right status.

This is "tighten silent defaults" applied to observability rather than to redelivery semantics. The deviation is explicit, documented here, and visible in the spec delta. If a future operator measurement shows the 200-ack is masking real defects rather than preventing storms, switching to 400 is a one-line follow-up.

### Decision 6 — `ErrComplexity` is a sentinel, not a wrapper struct

`isComplexityError(err) bool` today does `err.Error()[:20] == "password complexity:"`. Two failure modes: a future error message that happens to start with "password complexity:" misclassifies; a message rename breaks every caller silently.

The minimal fix: a sentinel error.

```go
// services/vault.go
var ErrComplexity = errors.New("password complexity")

func ValidatePasswordComplexity(password string) error {
    var failures []string
    // ... existing checks ...
    if len(failures) > 0 {
        return fmt.Errorf("%w: %s", ErrComplexity, strings.Join(failures, "; "))
    }
    return nil
}
```

```go
// handlers/vault.go
if errors.Is(err, services.ErrComplexity) {
    jsonValidationErrorResponse(w, err.Error(), map[string]string{"password": "complexity"})
    return
}
```

`isComplexityError` is deleted (single caller, no public exports). Existing test fixture in `vault_test.go` that returns `fmt.Errorf("password complexity: must be at least 12 characters")` is updated to wrap the sentinel; behavior is preserved.

A wrapper struct (e.g. `ComplexityError{Failures []string}`) would be over-engineering: the handler doesn't need the structured failures, it just needs the "is this a complexity error?" boolean. YAGNI.

---

## 3. Execution order

The six refactors are independent — no two share state. Order is chosen by ascending blast radius so each commit ships in a green-tests state and any rollback is localized:

| # | Audit ref | Item | Blast radius | Rough size |
|---|---|---|---|---|
| 1 | B7 | `ErrComplexity` sentinel | 1 service func, 1 handler, 1 test fixture | ≈25 lines |
| 2 | B6 + C11 + D8 | Webhook `event_type` strict + observable drop | `handlers/webhook.go` only | ≈40 lines |
| 3 | C5 | `claim_failure_mode` Redis cache | `handlers/action.go` + 2 deps.go injectables | ≈70 lines |
| 4 | C4 | JWT `Principal` in request context | `auth/jwt.go` + `handlers/router.go` middleware | ≈90 lines |
| 5 | B5 | `repositories.go` 11-file split | Mechanical `git mv`-equivalent; no caller change | ≈1300 lines moved, 0 changed |
| 6 | B3 | `accessSnapshot` in `services/views.go` | `services/views.go` + test surface | ≈250 lines |

Each step commits independently with all tests passing. Total expected effort: ~3-5 days for a single developer following the plan strictly.

---

## 4. Cross-theme dependencies

- **Wave 1 (Theme 1)** must be merged before Wave 2 · Part 2 starts. The vault dev-mode actor work (`enforceSelfOnly`) in Wave 1 added `getAdminUserID` callers that this wave's C4 refactor flows through unchanged.
- **Wave 2 · Part 1 (Theme 4 palette)** is parallel — no overlap. Frontend.
- **Wave 2 · Part 3 (Theme 5 operational polish)** is parallel. Theme 5 will drop `mapping_rules.version` and the `rule.Version` reference in `services/views.go:Topology` (line 604 in the pre-refactor file). Theme 3 preserves that line during the `accessSnapshot` refactor; Theme 5 deletes it afterward. No file-level merge conflict expected — adjacent line edits at most.
- **Wave 2 · Part 4 (Theme 2 — drift control)** depends on Theme 3 being merged. Theme 2's projection lookups will add functions to `services/views.go` and new helpers under `db/`; both files must be clean before Theme 2 begins. This is the load-bearing dependency that makes Theme 3 a prerequisite rather than a nice-to-have.

---

## 5. Verification gates

Each task carries its own narrow tests (call-count assertions, sentinel round-trips, fixture-based middleware tests). The wave-level gate at completion:

```bash
# Test + vet — from backend/ (where go.mod lives).
cd backend
go test ./...          # full unit + integration suite, no skip
go vet ./...

# gofmt — scoped to THIS wave's touch set (the files Wave 2 · Part 2
# creates or modifies). Pre-existing drift in untouched files
# (db/postgres.go, models/models.go, etc.) is real but out of scope;
# Step 0.3 of the plan normalises the touch set into a baseline commit
# before Task 1 runs. The full per-file list lives in the detailed
# plan at docs/superpowers/plans/2026-05-12-wave-2-part-2-backend-coherence.md
# (Step 7.1) — design.md keeps the contract, not the path enumeration.
cd ..                  # repo root, for gofmt to resolve backend/... paths
gofmt -d <wave-2-part-2 touch set>    # zero diff after refactor
```

Plus codebase-memory refresh:

```
mcp__codebase-memory-mcp__detect_changes
mcp__codebase-memory-mcp__index_repository (affected scope)
```

And the OpenSpec validation (run from repo root — the CLI errors with `Unknown item …` when invoked from inside `openspec/`):

```bash
cd /path/to/Syndra     # repo root
openspec validate wave-2-part-2-backend-coherence --strict
```

---

## 6. Out of scope / explicit non-goals

- **No public API changes.** Every HTTP endpoint, response shape, and middleware contract is preserved.
- **No database migrations.** `webhook_events.status` already accepts arbitrary strings — `dropped_enrichment_incomplete` is a new enum value, not a schema change.
- **No new Redis keys other than `claim_mode:<projectID>`.** No Redis-side migration; the key just appears on first hit.
- **No directory-layer changes.** `directory/zitadel.go` already caches projects/users/applications; not in scope.
- **No `mapping_rules.version` removal.** That is Theme 5's scope (D4). The version field stays referenced in `Topology`'s `Meta` map until then.
- **No reconciliation-cap change.** That is Theme 2's scope (B2 → 2000-grant cap).
- **No webhook redelivery semantics change.** 200-ack for unresolvable enrichment stays (Decision 5).

---

## 7. Open questions

None blocking. Two non-blocking parameters that take a default and need no further discussion:

1. `CLAIM_MODE_CACHE_TTL_SECONDS` env default: **300** (5 minutes). Documented in `.env.example` by Theme 5 (D7's env-block sweep) alongside other backend tunables.
2. `webhook_events.status = 'dropped_enrichment_incomplete'` — added as a string literal; no enum-extension migration required because the column is already `TEXT NOT NULL DEFAULT 'pending'`.

If implementation surfaces an unforeseen dependency, halt and re-design rather than work around.
