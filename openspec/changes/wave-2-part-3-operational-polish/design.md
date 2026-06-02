# Wave 2 · Part 3 — Operational Polish Design

**Status:** Approved for implementation
**Parent design:** [`docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md`](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 5
**Sister documents:** [Wave 1 design](../wave-1-production-trust-hardening/design.md), [Wave 2 · Part 1 design](../wave-2-part-1-frontend-palette-finalization/design.md), [Wave 2 · Part 2 design](../wave-2-part-2-backend-coherence/design.md)

---

## 1. Aim

Eleven audit findings, eleven small fixes. The cleanup is decomposed into eleven independently-shippable steps ordered by ascending blast radius; the smallest single-line fixes land first so each commit is a verification anchor for the next. Each step passes `go test ./...` and `go vet ./...` (in the touched module) before the next begins. No two steps share state.

The point of this wave is not refactoring or architecture — it is *finishing the obvious cleanups* so the deployment surface, the sync service, the scripts directory, and the spec drift around `mapping_rules.version` all stop generating recurring micro-decisions every time an operator touches them.

---

## 2. Design decisions

Five load-bearing decisions; the other six items are mechanical.

### Decision 1 — `mapping_rules.version` is dropped without a replacement (D4)

The audit-resolution design §2 (D4 row) decided the doctrinal point: versioning is removed, `audit_logs` is the historical record. This wave executes that decision.

The column has exactly one writer in production code — `UpdateMappingRule(ctx, id)` at `db/rules.go:29-39` — and that function's *only* job is the version bump. There is no UPDATE-other-fields path; rule edits go through `DELETE` + `CREATE` with both halves recorded in `audit_logs`. The version column does, however, have a *live workflow* on top of it: a `PUT /api/v1/rules/mapping/{id}` route, an operator-facing "Bump version →" button on the `/policies` page, a `useBumpMappingRule` React Query mutation, three handler tests, and an `mapping_rule.version_bumped` audit event. Deleting the column therefore implies deleting the entire bump workflow alongside it — not just `UpdateMappingRule`.

Touch surface (14 spots):

| File | Line | Change |
|---|---|---|
| `backend/db/migrations/000014_drop_mapping_rules_version.up.sql` | new | `ALTER TABLE mapping_rules DROP COLUMN version;` |
| `backend/db/migrations/000014_drop_mapping_rules_version.down.sql` | new | `ALTER TABLE mapping_rules ADD COLUMN version INT NOT NULL DEFAULT 1;` |
| `backend/internal/db/rules.go` | 29-39 | delete `UpdateMappingRule` entirely |
| `backend/internal/db/rules.go` | 44-46, 56 | drop `version` from `SELECT` and `Scan` in `GetActiveMappingRules` |
| `backend/internal/handlers/rules.go` | 128-144 | delete `handleUpdateMappingRule` entirely (with its `mapping_rule.version_bumped` audit log) |
| `backend/internal/handlers/router.go` | 60 | delete `mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", ...)` |
| `backend/internal/handlers/deps.go` | 44 | delete `dbUpdateMappingRule = db.UpdateMappingRule` injectable |
| `backend/internal/handlers/rules_test.go` | 18, 24, 151-225 | delete `TestHandleUpdateMappingRule_{NotFound,HappyPath,MissingID}` and drop `origUpdate` from `resetRulesDeps` |
| `backend/internal/models/models.go` | 37 | delete `Version int json:"version"` field |
| `backend/internal/services/views.go` | 700 | delete the `"version": fmt.Sprintf("%d", rule.Version)` Topology meta line |
| `ui/src/lib/queries/useMappingRules.ts` | 13 | delete `version: number;` from `MappingRuleRow` (the live type consumed by `/policies`, `/projects`, `GrantsClient.tsx`) |
| `ui/src/lib/queries/useMappingRules.ts` | 75-87 | delete `useBumpMappingRule` mutation hook and its JSDoc |
| `ui/src/lib/types.ts` | 59 | delete `version: number;` from the (currently unimported) `MappingRule` type — keep consistent with the live type |
| `ui/src/app/policies/page.tsx` | 16, 38, 45-49, 96, 135-145 | drop `useBumpMappingRule` import, `bumpRule` binding, `handleBump` function, version badge, and the entire "Bump version →" button row (including the now-empty wrapper `<div>`) |

The down migration restores the column with default `1` so a rollback returns to a parseable state. There is no migration of historical version values — they were never read. (If a rollback ever needs the historical bump count, `audit_logs` retains every `mapping_rule.version_bumped` event up to the cutover with timestamps and actor IDs.)

The earlier read of "zero callers" was wrong — it was based on a `grep "UpdateMappingRule\b"` against `db/...` only, which missed the handler indirection through the `dbUpdateMappingRule` injectable. The corrected sweep (`grep -rn "UpdateMappingRule\|useBumpMappingRule\|Bump version"`) found the 14 touchpoints above.

### Decision 2 — Sync env block lands in `.env.example` as a documented surface, not a sample `.env`

`.env.example` is the deployment template. Operators copy it to `.env` and edit. The block is therefore documentation in `# comment` form, not a runnable file — every variable is presented with:
- A one-line description (what it is and when it matters)
- The canonical default from `sync/internal/config/config.go` (or `# required, no default` for `MKAUTH_API_KEY`, `LLDAP_BIND_DN`, `LLDAP_BIND_PASSWORD`)
- The variable name commented out when a default exists, uncommented (with a placeholder) when required

This matches the convention already used by the backend block (compare `EXPIRY_SCHEDULER_*` at lines 21-30 — commented because defaults exist).

Two backend-block additions surface variables that today are read from env at runtime but undocumented in the template:

- `MKAUTH_EXTERNAL_URL` — read by `handlers/onboarding.go` (welcome-link generation); required when `ZITADEL_DOMAIN != ""`.
- `ZITADEL_M2M_TOKEN` — direct token-injection alternative to `ZITADEL_MACHINE_KEY_PATH`. The codebase reads whichever is set; the template only documented the keypath path.

### Decision 3 — `withConn` ctx-cancellation is fail-fast, not cooperative-mid-op

`sync/internal/ldap/client.go:73`'s `withConn` today takes only `fn func(*ldapv3.Conn) error` and serialises through `p.mu`. The audit (C8) wants context propagation for worker shutdown.

Two design options for ctx-cancellation:

| Option | Behavior | Verdict |
|---|---|---|
| **A. Fail-fast at boundary.** Check `ctx.Err()` before attempting `p.mu.Lock()`, immediately after acquiring it, and before any reconnect retry. The active LDAP call (whichever goroutine holds the mutex) is NOT interrupted — it runs to completion or the underlying `*ldapv3.Conn` timeout. A queued goroutine blocked on `Lock()` continues to wait for the mutex (Go's `sync.Mutex` is not select-able); when it eventually acquires the mutex, the post-acquisition `ctx.Err()` check returns immediately without issuing an LDAP request. | Cancellation is delivered with worst-case latency equal to one in-flight LDAP op's duration (≈100ms typical, ≈5s if the conn timeout fires). No race surface; no connection-close coordination. | **Chosen.** |
| **B. Cooperative mid-op.** Use `go-ldap`'s `Conn.SetTimeout` or close the connection on ctx-cancel to interrupt the active call. | True mid-LDAP-op cancellation, but introduces a goroutine + connection-close race that fights the `withConn` reconnect path. | Rejected — complexity not justified by the cancel-window benefit. |

Worker shutdown today blocks indefinitely on the LDAP mutex if a hung LDAP call holds it; the conn timeout is the only escape hatch. Option A doesn't fix the hung-call case (the mutex still pins the dying worker), but it does ensure that **all queued and not-yet-attempted ops fail immediately at the next mutex boundary** rather than each one issuing a doomed LDAP request after the dying op finally releases the mutex. That bound is what the C8 audit finding actually asked for — "propagate cancellation to in-flight LDAP ops" was the audit's wording, but the operational concern was "don't let queued ops flood LLDAP after the parent context is cancelled." Option A meets that concern. If a future operator measures shutdown latency as a problem from the hung-mutex case, switching to B (or adding `Conn.SetTimeout` injection on cancel) is a localised follow-up.

**Precise contract:**
- An op whose `ctx` is cancelled *before* it calls `withConn` returns `ctx.Err()` without touching the mutex.
- An op queued on `p.mu.Lock()` whose `ctx` is cancelled mid-wait still waits for the mutex (this is `sync.Mutex` semantics; we do not switch to a select-able primitive); upon acquiring the mutex it returns `ctx.Err()` without invoking `fn` and without issuing an LDAP request.
- The op currently holding the mutex is not interrupted by anything in this design — it runs `fn` to completion.
- The reconnect retry, if reached, sees a fresh `ctx.Err()` check and skips the retry on cancellation.

Signature change:

```go
// before
func (p *Pool) withConn(fn func(*ldapv3.Conn) error) error

// after
func (p *Pool) withConn(ctx context.Context, fn func(*ldapv3.Conn) error) error {
    if err := ctx.Err(); err != nil {
        return err
    }
    p.mu.Lock()
    defer p.mu.Unlock()
    if err := ctx.Err(); err != nil {
        return err
    }
    err := fn(p.conn)
    if err != nil && IsConnectionError(err) {
        if err := ctx.Err(); err != nil {
            return err
        }
        if reconnErr := p.reconnect(); reconnErr != nil {
            return fmt.Errorf("reconnect failed: %w (original: %v)", reconnErr, err)
        }
        return fn(p.conn)
    }
    return err
}
```

All callers in `client.go` already accept a `ctx context.Context` parameter (or accept `_ context.Context` and drop it — see `EnsureGroup` at line 156); the change is mechanical thread-through.

### Decision 4 — Extract `load-env.sh`, do NOT extract `zitadel-api.sh` (S1 + S2 deviation)

Audit listed both extractions in one item. Investigation found asymmetric reality:

- `_ENV_FILE` env-loader: **22 lines duplicated character-for-character** in `register.sh:74-95` and `rotate.sh:55-76`. Real duplication.
- `zitadel_api()` curl helper: lives **only** in `register.sh:139-162`. `rotate.sh` calls `register.sh`'s functions via direct invocation, not via embedded copy. Single consumer.

DRY applies when there are ≥2 consumers. Extracting a helper for one caller now would:
- Force a `source` line into `register.sh` (no benefit — it already owns the code)
- Lock the helper's interface against a hypothetical future second consumer (Postel's law cuts both ways for shell scripts — bash function signatures are particularly hard to evolve without breaking sourced scripts that capture them at parse time)
- Add a layer of indirection an operator reading `register.sh` must now navigate to understand a single workflow

Deviation: extract `load-env.sh` only. If a third script ever needs `zitadel_api()`, the second-consumer call is the right moment to extract. Documented in the proposal's "Out of scope" section so this isn't read as oversight.

The extracted `load-env.sh` interface:

```bash
# scripts/lib/load-env.sh
#
# Usage:
#   _ENV_FILE="${_ENV_FILE:-"$(cd "${SCRIPT_DIR}/../.." && pwd)/.env"}"
#   # shellcheck source=../scripts/lib/load-env.sh
#   source "${REPO_ROOT}/scripts/lib/load-env.sh"
#
# Loads KEY=VALUE pairs from "$_ENV_FILE" into the current shell
# environment. Preserves any KEY already set in the environment
# (does not overwrite). No-op when the file does not exist.

# ... body identical to the current register.sh:74-95 block ...
```

The caller pre-computes `_ENV_FILE` so the helper does not need to know about `SCRIPT_DIR` conventions. This keeps the helper a pure transformation: env file → environment.

### Decision 5 — `webhook_events.status = 'dropped_enrichment_incomplete'` budget interacts with `grantLookupMaxPages`

B8 drops `grantLookupMaxPages` from 100 to 10. `listUserGrantsViaZitadel` is the fallback path Wave 2 · Part 2's "Webhook enrichment incomplete" delta (C11/D8) measures. Question: does the tighter bound increase the rate of `dropped_enrichment_incomplete` rows?

Math: each page is 100 grants (Zitadel `DefaultSearchLimit`). 10 pages = 1000 grants per user. The makerspace audience: ≈200 users, ≈25 grants/user typical, ≈100 grants/user p99. The probability of a single user holding > 1000 grants is effectively zero. The 100-page cap (10 000 grants) was paranoia about a Zitadel `Total` reporting bug, not a real upper bound.

The tighter cap doesn't change observed drop rate. If Wave 2 · Part 2's new `?status=dropped_enrichment_incomplete` query starts surfacing drops correlated with high-grant users, the cap is the first knob to investigate — but the audit's right-sizing decision (audit-resolution design §3 Theme 5 row B8: "200-user org never paginates beyond ~2 pages") stands.

The comment above the constant is updated:

```go
// grantLookupMaxPages bounds the pagination loop in the fallback enrichment
// path. At 100 grants per page (Zitadel DefaultSearchLimit), 10 pages cover
// 1000 grants per user — already an order of magnitude beyond what the
// makerspace audience generates (p99 ≈ 100 grants/user). If a future
// deployment regularly hits the cap, the right fix is a more selective
// Zitadel query (search by grantID), not a higher page count.
const grantLookupMaxPages = 10
```

---

## 3. Execution order

The eleven steps are independent — no two share state. Order is ascending blast radius so each commit ships in a green-tests state and any rollback is localized:

| # | Audit ref | Item | Blast radius | Rough size |
|---|---|---|---|---|
| 1 | C10 | Drop shadow-password zero-buffer defer | 1 file, 5-line delete + 2-line comment | ≈7 lines |
| 2 | B8 | `grantLookupMaxPages` 100 → 10 | 1 const, 1 comment block | ≈10 lines |
| 3 | C9 | Smoke-test probes `/healthz` | 1 file, 2 lines | ≈3 lines |
| 4 | D9 | `EXPIRY_SCHEDULER_*` replicas framing | 1 file, 1 comment block | ≈8 lines |
| 5 | D7 | `.env.example` sync block + 2 backend vars | 1 file, append + 2 inserts | ≈40 lines |
| 6 | S3 | Fold `zitadel/actions/{PERMISSIONS,SIGNING_KEY}.md` into `README.md` | 2 files deleted, 1 expanded | ≈250 lines moved |
| 7 | C7 | LDAP `member: [bindDN]` placeholder | 1 line + 1 test | ≈25 lines |
| 8 | C8 | `withConn(ctx, fn)` signature change | `client.go` + 1 test | ≈50 lines |
| 9 | S4 | Read `SYNC_RETRY_*` from env | `config.go` + 1 test | ≈30 lines |
| 10 | S1 | Extract `scripts/lib/load-env.sh` | 1 new file, 2 callers updated | ≈40 lines |
| 11 | D4 | Drop `mapping_rules.version` and the bump workflow | migration + 11 source files (route, handler, deps, tests, model, view, db, two UI types, policies page) | ≈80 lines deleted across stack, 1 migration |

Each step commits independently with all tests passing. Total expected effort: ~2-3 days for a single developer following the plan strictly.

---

## 4. Cross-theme dependencies

- **Wave 1 (Theme 1)** must be merged before Wave 2 · Part 3 starts. Wave 1 added migration `000012_welcome_bundle_flag`; this wave's `000014_drop_mapping_rules_version` numbers sequentially after Wave 2 · Part 2's `000013_webhook_events_dropped_status`. Order: Wave 1 → Wave 2 · Part 2 → Wave 2 · Part 3.
- **Wave 2 · Part 1 (Theme 4 palette)** is parallel — no overlap.
- **Wave 2 · Part 2 (Theme 3 backend coherence)** is parallel **except** for one trivial line: this wave's D4 deletes `services/views.go:700` (`"version": fmt.Sprintf("%d", rule.Version)`), which Wave 2 · Part 2's B3 `accessSnapshot` refactor preserved untouched. After both waves merge, the line is gone. No merge conflict at file level — adjacent line edits at most.
- **Wave 2 · Part 4 (Theme 2 — drift control)** is independent. Theme 2 will introduce new env vars (`DRIFT_RECONCILIATION_INTERVAL_HOURS`, `OUTBOX_MAX_RETRIES`) into `.env.example`; the sync block this wave adds is in a separate section and does not collide.

---

## 5. Verification gates

Each step carries its own narrow tests. The wave-level gate at completion:

```bash
# Backend
cd backend
go test ./...
go vet ./...

# Sync
cd ../sync
go test ./...
go vet ./...

# Frontend (D4 type/badge deletion only)
cd ../ui
bun run lint
bun run test
bun run build

# gofmt — scoped to THIS wave's touch set. The full per-file list lives
# in the detailed plan at docs/superpowers/plans/2026-05-25-wave-2-part-3-operational-polish.md.
cd ..
gofmt -d <wave-2-part-3 touch set>    # zero diff after refactor
```

Plus codebase-memory refresh:

```
mcp__codebase-memory-mcp__detect_changes
mcp__codebase-memory-mcp__index_repository (affected scope)
```

And the OpenSpec validation (run from repo root):

```bash
cd /path/to/MkAuth
openspec validate wave-2-part-3-operational-polish --strict
```

Migration verification (D4):

```bash
# Forward + backward migration round-trip on a throwaway DB
cd backend
go run ./cmd/migrate up
go run ./cmd/migrate down 1   # rolls back 000014, restores column with default 1
go run ./cmd/migrate up       # re-applies 000014
```

---

## 6. Out of scope / explicit non-goals

- **No new HTTP endpoints.** Every response shape and middleware contract is preserved.
- **No `mapping_rules_history` table.** Versioning is removed; `audit_logs` is the historical record.
- **No `zitadel-api.sh` extraction.** Single-consumer; YAGNI. Documented in Decision 4.
- **No frontend rework beyond the D4 single-field deletion.** Wave 3 owns the rest of the UI consolidation.
- **No EXPIRY_SCHEDULER toggle removal.** Only the comment framing changes.
- **No reconciliation-cap change.** That is Theme 2's scope (B2 → 2000-grant cap in `backend/internal/handlers/reconciliation.go`).
- **No new Redis keys.** This wave introduces no caching.
- **No `OpenSpec mkauth-core-architecture/design.md` edits.** Cross-cutting docs (CLAUDE.md, ROADMAP.md, INDEX.md) are owned by Theme 2 per the audit-resolution design §7 table.

---

## 7. Open questions

None blocking. Two non-blocking observations:

1. **`LLDAP_INSECURE_SKIP_VERIFY=true` default in dev.** `sync/internal/config/config.go:41` defaults to `"false"`. The dev compose stack uses self-signed certs and requires `true`. The `.env.example` block lands the default as `false` (matching the code) with an inline comment: `# set to true only in dev with self-signed LLDAP certs`. If the LXC deployment uses a real certificate, the default is correct; if it doesn't, the comment is the operator's first hint.
2. **Down-migration of D4 restores the column with `DEFAULT 1`, not historical values.** Pre-drop versions are lost on rollback. This is intentional (D4 doctrinal decision: versioning was unused) but documented here so a rollback isn't a surprise. If a future operator needs version history, `audit_logs` retains every rule create/delete event with timestamps.

If implementation surfaces an unforeseen dependency, halt and re-design rather than work around.
