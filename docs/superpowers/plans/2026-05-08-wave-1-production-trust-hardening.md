# Wave 1 — Production Trust Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve every Theme 1 ship-blocker from the May 2026 audit (C1, C2/D5, D1, C3, B1) so that production trust assumptions hold and architectural work in later waves cannot land on top of silent fall-throughs or destructive bootstrap scripts.

**Architecture:** Five independent fixes coordinated under a single OpenSpec change (`wave-1-production-trust-hardening`). Two surfaces (backend Go + Next.js UI) and one new SQL migration. No new abstractions, no shared subsystems — each item is a localized hardening of an existing path.

**Tech Stack:** Go 1.22 stdlib (`net/http`, `os`, `pgx/v5`), PostgreSQL with `golang-migrate` SQL migrations, Next.js 14 App Router on Bun, vitest + httptest.

---

## File Structure

| Path | Action | Responsibility |
|---|---|---|
| `openspec/changes/wave-1-production-trust-hardening/proposal.md` | create | Why this change exists (audit traceability) |
| `openspec/changes/wave-1-production-trust-hardening/design.md` | create | Per-item design notes (mostly cross-references back to the meta-spec) |
| `openspec/changes/wave-1-production-trust-hardening/tasks.md` | create | Mirror of the task list in this plan |
| `openspec/changes/wave-1-production-trust-hardening/IMPLEMENTATION.md` | create | Filled in as tasks complete |
| `openspec/changes/wave-1-production-trust-hardening/specs/automation-policies/spec.md` | create | Welcome-bundle: explicit `is_welcome` flag, error when unset |
| `openspec/changes/wave-1-production-trust-hardening/specs/production-security-boundary/spec.md` | create | Vault dev-mode `?actor=` requirement; signing-key startup gate |
| `openspec/changes/wave-1-production-trust-hardening/specs/operational-readiness/spec.md` | create | Member dashboard renders title/team/location for OIDC sessions |
| `backend/cmd/test/main.go` | **delete** | Destructive `DELETE FROM mapping_rules` bootstrap, no callers (B1) |
| `backend/cmd/api/main.go` | modify | Add startup signing-key gate when `ZITADEL_DOMAIN != ""` (C1) |
| `backend/internal/handlers/zitadel_action_auth.go` | modify | Tighten dev-mode passthrough to require `ZITADEL_DOMAIN==""` (C1) |
| `backend/internal/handlers/zitadel_action_auth_test.go` | modify | Replace passthrough test, add prod-misconfig refusal test (C1) |
| `backend/db/migrations/000012_welcome_bundle_flag.up.sql` | create | `ALTER TABLE bundles ADD is_welcome` + partial unique index (D1) |
| `backend/db/migrations/000012_welcome_bundle_flag.down.sql` | create | Drop column + index for rollback (D1) |
| `backend/internal/db/repositories.go` | modify | Rewrite `GetWelcomeBundle`; add `SetWelcomeBundle` (D1) |
| `backend/internal/db/repositories_test.go` (or new `bundles_repo_test.go`) | create/modify | Cover the new `WHERE is_welcome=TRUE` path against an in-process Postgres (D1) |
| `backend/internal/handlers/bundles.go` | modify | Add `PUT /api/v1/bundles/{id}/welcome` handler (D1) |
| `backend/internal/handlers/bundles_test.go` | modify | Test welcome-bundle handler success + audit (D1) |
| `backend/internal/services/onboarding.go` | modify | Surface the new "no welcome bundle configured" error verbatim (D1) |
| `backend/internal/services/onboarding_test.go` | modify | Update test for the new error string; verify `failed` state recorded (D1) |
| `backend/internal/handlers/profile.go` | modify | Add `handleGetMyProfile` returning the requester's `UserProfile` (C2/D5) |
| `backend/internal/handlers/router.go` | modify | Wire `GET /api/v1/me/profile` under `withUserAuth` (C2/D5) |
| `backend/internal/handlers/profile_test.go` | create | Test self-resolution against demo + live (mocked) directories (C2/D5) |
| `backend/internal/handlers/vault.go` | modify | Refuse vault mutations in dev mode unless `?actor=` is supplied (C3) |
| `backend/internal/handlers/vault_test.go` | modify | Cover dev-mode 400 path and dev-mode `?actor=alice` happy path (C3) |
| `backend/internal/handlers/router.go` | modify | Wire `PUT /api/v1/bundles/{id}/welcome` (D1) |
| `ui/src/lib/session.ts` | modify | Extend `OidcSessionCookie` with `title`/`team`/`location`; map into `SessionUser` (C2/D5) |
| `ui/src/lib/__tests__/session.test.ts` | modify | Assert OIDC profile fields round-trip through encode/decode (C2/D5) |
| `ui/src/lib/oidc.ts` | modify | Add `fetchProfileMetadata()` helper that hits `/api/v1/me/profile` (C2/D5) |
| `ui/src/app/auth/callback/route.ts` | modify | After token exchange, fetch `/me/profile`; embed in cookie (C2/D5) |
| `ui/src/lib/queries/useBundles.ts` | modify | Add `useSetWelcomeBundle` mutation; expose `is_welcome` on `BundleRow` (D1) |
| `ui/src/app/bundles/page.tsx` | modify | Render "Welcome bundle" badge + toggle in `BundleRowCard` (D1) |
| `ui/src/app/bundles/__tests__/page.test.tsx` | modify | Smoke-test the toggle interaction (D1) |
| `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` | modify | Update Welcome Bundle row from "convention-based" → "explicit; errors when not configured" (D1) |
| `openspec/INDEX.md` | modify | Add Wave 1 row in Change Log (Phase 5.5) |

Each file has one responsibility; no file does more than one fix from the audit table.

---

## Task 0: OpenSpec change scaffolding

Create the change directory before any code so every commit lands inside an existing scope.

**Files:**
- Create: `openspec/changes/wave-1-production-trust-hardening/proposal.md`
- Create: `openspec/changes/wave-1-production-trust-hardening/design.md`
- Create: `openspec/changes/wave-1-production-trust-hardening/tasks.md`
- Create: `openspec/changes/wave-1-production-trust-hardening/IMPLEMENTATION.md`

- [ ] **Step 0.1: Write `proposal.md`**

```markdown
# Wave 1 — Production Trust Hardening

**Status:** In progress
**Source:** [May 2026 audit resolution design](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 1
**Phase:** 5.5

## Why
The May 2026 codebase audit identified five ship-blocker findings that erode operator trust in production:
- **C1** — Backend silently passes through Zitadel webhook/action requests when signing keys are unset, even with `ZITADEL_DOMAIN` configured.
- **C2 / D5** — OIDC member dashboard renders blank Title/Team/Location because the cookie is never populated from Zitadel metadata.
- **D1** — `GetWelcomeBundle` falls back to "first bundle by created_at" when no welcome bundle is configured, silently assigning the wrong bundle to new users.
- **C3** — Shadow-credential vault accepts mutations in dev mode without any actor attribution; the audit log records the target user as the actor.
- **B1** — `backend/cmd/test/main.go` is a destructive bootstrap script that runs `DELETE FROM mapping_rules` and has no callers.

These five fixes share no code paths but all gate the production deployment story. Shipping them as a single coordinated change keeps the audit-resolution wave structure visible.

## What changes
- Backend fails fast at startup if `ZITADEL_DOMAIN != ""` and either signing-key env is empty.
- New `GET /api/v1/me/profile` endpoint and OIDC callback wiring populates Title/Team/Location for OIDC sessions identically to demo sessions.
- `bundles.is_welcome` column with a single-true partial unique index; `GetWelcomeBundle` errors loudly when no bundle is marked.
- Vault `enforceSelfOnly` requires `?actor=<id>` for PUT/DELETE in dev mode and refuses 400 otherwise; reads unchanged.
- `backend/cmd/test/main.go` deleted.

## Out of scope
Theme 2's drift-control architecture, Theme 3's backend coherence refactors, Theme 4's UI palette migration, and Theme 5's operational polish all ship in later waves.
```

- [ ] **Step 0.2: Write `design.md`**

```markdown
# Wave 1 — Design

This design defers to the [May 2026 meta-spec](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) for cross-cutting structure and timeline. Per-item notes:

## C1 — Production refuses missing signing keys
Two layers:
1. Startup gate in `cmd/api/main.go`. When `ZITADEL_DOMAIN != ""`, fail fast (log.Fatalf) if `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` is empty. Runs before the HTTP server is bound, so a misconfigured deploy never accepts traffic.
2. Middleware tightening in `withZitadelActionSignature`. Dev-mode passthrough is now conditional on `ZITADEL_DOMAIN == ""` *as well as* the secret being empty. If `ZITADEL_DOMAIN != ""` and (somehow) the secret is empty at request time, return 503 INTERNAL.

The two-layer design is belt-and-suspenders — startup catches misconfiguration; middleware refuses to silently degrade if startup is bypassed (e.g. signal-induced reload mid-flight).

## C2 / D5 — OIDC profile metadata
A new authenticated endpoint `GET /api/v1/me/profile` resolves the requester's user ID from their bearer token and returns the same `models.UserProfile` shape the directory layer already overlays (`title`, `team`, `location`, `name`, `email`, `status`).

The Next.js OIDC callback handler hits this endpoint immediately after token exchange, with the freshly-issued access token, and writes the full `UserProfile` into the session cookie. Cookie size impact: ~120 bytes worst-case — well below the 4 KB limit.

Demo sessions already populate these fields from the demo catalog; the change makes OIDC sessions render identically.

## D1 — Welcome bundle errors explicitly
Schema:
```sql
ALTER TABLE bundles ADD COLUMN is_welcome BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX idx_bundles_welcome_unique ON bundles (is_welcome) WHERE is_welcome = TRUE;
```

The partial unique index enforces "at most one welcome bundle" at the database layer. The default of `FALSE` keeps existing rows safe.

`GetWelcomeBundle` becomes a single-row select on `WHERE is_welcome = TRUE`. `pgx.ErrNoRows` is mapped to a domain error `ErrNoWelcomeBundleConfigured`. Onboarding propagates that error verbatim; the trigger row is marked `failed` with the same string so operators see "no welcome bundle configured" in the UI rather than a silent default-bundle assignment.

`SetWelcomeBundle(bundleID)` is a transactional clear-then-set: `UPDATE bundles SET is_welcome=FALSE WHERE is_welcome=TRUE; UPDATE bundles SET is_welcome=TRUE WHERE id=$1`. The partial unique index is a backstop, not the contract.

UI: `BundleRowCard` renders a `Welcome` badge when `is_welcome=true`; the bundle's expanded panel exposes a `Set as welcome bundle` button. No autopromote.

## C3 — Vault dev-mode self-attribution
`enforceSelfOnly` gains a `requireActor bool` parameter. Mutations (`PUT`/`DELETE`) call with `requireActor=true`; reads call with `requireActor=false`.

When `getAdminUserID(ctx) == ""` (dev-mode API-key auth) and `requireActor=true`, the handler reads `?actor=<id>` from the query. Empty → 400 `MISSING_ACTOR`. Non-empty → use it as `actorID`; log it with the prefix `[VAULT] dev-mode actor`.

The query parameter never propagates to the audit log as a JWT actor — it's stamped into `actorID` only after the JWT-actor branch is exhausted. The audit reads exactly the same.

## B1 — Delete test/main.go
Run `git log --diff-filter=A -- backend/cmd/test/main.go` for blame; verify nothing imports it (`go list ./...` from `backend/`). Remove the file and the empty `cmd/test/` directory.
```

- [ ] **Step 0.3: Write `tasks.md` (mirror of this plan's task headers)**

```markdown
# Wave 1 — Tasks

- [ ] **Task 0** — OpenSpec change scaffolding (this directory)
- [ ] **Task 1** — Delete `backend/cmd/test/main.go` (B1)
- [ ] **Task 2** — Production startup signing-key gate (C1, backend)
- [ ] **Task 3** — Tighten `withZitadelActionSignature` dev-mode passthrough (C1, backend)
- [ ] **Task 4** — `is_welcome` migration + repo rewrite (D1, backend)
- [ ] **Task 5** — Welcome bundle handler + onboarding error path (D1, backend)
- [ ] **Task 6** — Welcome bundle UI toggle (D1, ui)
- [ ] **Task 7** — Vault dev-mode `?actor=` requirement (C3, backend)
- [ ] **Task 8** — `/api/v1/me/profile` endpoint (C2/D5, backend)
- [ ] **Task 9** — OIDC callback fetches profile, cookie carries title/team/location (C2/D5, ui)
- [ ] **Task 10** — Spec deltas + INDEX update + codebase-memory refresh
```

- [ ] **Step 0.4: Stub `IMPLEMENTATION.md`**

```markdown
# Wave 1 — Implementation log

Filled in as tasks complete. Each entry should reference the audit ID and the commit SHA.
```

- [ ] **Step 0.5: Commit the scaffolding**

```bash
git add openspec/changes/wave-1-production-trust-hardening/
git commit -m "openspec: scaffold wave-1-production-trust-hardening change"
```

---

## Task 1: Delete `backend/cmd/test/main.go` (B1)

Smallest, most isolated win. Doing it first removes destructive code that could be hit accidentally during the rest of the wave.

**Files:**
- Delete: `backend/cmd/test/main.go`

- [ ] **Step 1.1: Verify nothing imports the package**

Run: `cd backend && grep -rn "syndra/cmd/test" .`
Expected: no matches.

Run: `cd backend && go list ./...`
Expected: clean output, no error from removing a referenced package.

- [ ] **Step 1.2: Delete the file and empty directory**

```bash
rm backend/cmd/test/main.go
rmdir backend/cmd/test
```

- [ ] **Step 1.3: Verify build still passes**

Run: `cd backend && go build ./... && go vet ./...`
Expected: exit 0 with no output.

- [ ] **Step 1.4: Commit**

```bash
git add -A backend/cmd/
git commit -m "backend: remove destructive cmd/test bootstrap script (audit B1)

cmd/test/main.go ran DELETE FROM mapping_rules with no callers and no
guards. Audited as ship-blocker B1 in May 2026 review."
```

---

## Task 2: Production startup signing-key gate (C1)

Fail fast at startup so misconfigured production deployments never bind a port.

**Files:**
- Modify: `backend/cmd/api/main.go` (insert gate before `db.ConnectPostgres()`)

- [ ] **Step 2.1: Add the gate function**

In `backend/cmd/api/main.go`, before `func main()`, add:

```go
// requireProductionSigningKeys aborts startup if ZITADEL_DOMAIN is set but
// either signing-key env is empty. Production deployments without these keys
// would silently accept unverified webhook/action payloads — an unacceptable
// trust posture flagged by the May 2026 audit (C1).
func requireProductionSigningKeys() {
	if os.Getenv("ZITADEL_DOMAIN") == "" {
		return // dev mode — the action-signature middleware allows passthrough
	}
	missing := []string{}
	if os.Getenv("ZITADEL_EVENT_SIGNING_KEY") == "" {
		missing = append(missing, "ZITADEL_EVENT_SIGNING_KEY")
	}
	if os.Getenv("ZITADEL_ACTION_SIGNING_KEY") == "" {
		missing = append(missing, "ZITADEL_ACTION_SIGNING_KEY")
	}
	if len(missing) > 0 {
		log.Fatalf("[STARTUP] Production refusing to start: ZITADEL_DOMAIN is set but %s is empty. Configure signing keys before deploying.", strings.Join(missing, ", "))
	}
}
```

Add `"strings"` to the import block.

- [ ] **Step 2.2: Call the gate first thing in `main`**

In the same file, the first statement of `func main()` becomes:

```go
func main() {
	requireProductionSigningKeys()
	fmt.Println("Syndra Backend Starting...")
	// ... existing body unchanged
```

- [ ] **Step 2.3: Run vet + build**

Run: `cd backend && go vet ./... && go build ./cmd/api`
Expected: exit 0. (`go build ./cmd/api` produces a binary named `api` in the current directory; the smoke test below uses `go run` so the artifact location doesn't matter.)

- [ ] **Step 2.4: Manual smoke (no unit test — `log.Fatalf` is hard to test in-process)**

```bash
cd backend
ZITADEL_DOMAIN=zitadel.test ZITADEL_EVENT_SIGNING_KEY= ZITADEL_ACTION_SIGNING_KEY= go run ./cmd/api 2>&1 | head -3
```
Expected: `[STARTUP] Production refusing to start: ZITADEL_DOMAIN is set but ZITADEL_EVENT_SIGNING_KEY, ZITADEL_ACTION_SIGNING_KEY is empty.` followed by a non-zero exit.

Then verify dev mode still starts cleanly:

```bash
unset ZITADEL_DOMAIN
go run ./cmd/api &
SMOKE_PID=$!
sleep 1
kill "$SMOKE_PID"
```
Expected: prints `Syndra Backend Starting...` and `Control Plane Backend Listening on :8080` before being killed.

- [ ] **Step 2.5: Commit**

```bash
git add backend/cmd/api/main.go
git commit -m "backend: fail fast at startup if production signing keys missing (audit C1)

When ZITADEL_DOMAIN is set, ZITADEL_EVENT_SIGNING_KEY and
ZITADEL_ACTION_SIGNING_KEY MUST both be present. Backend exits with a
named-cause log line before binding the HTTP port."
```

---

## Task 3: Tighten `withZitadelActionSignature` dev-mode passthrough (C1)

Belt-and-suspenders runtime check: if startup is somehow bypassed, the middleware still refuses to silently fall through in production.

**Files:**
- Modify: `backend/internal/handlers/zitadel_action_auth.go:50-57`
- Modify: `backend/internal/handlers/zitadel_action_auth_test.go:132-144`

- [ ] **Step 3.1: Update the middleware passthrough condition**

In `backend/internal/handlers/zitadel_action_auth.go`, replace lines 50-57 of `withZitadelActionSignature` (the `if secret == "" { ... pass through ... }` block) with:

```go
func withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv(secretEnvVar)
		if secret == "" {
			if os.Getenv("ZITADEL_DOMAIN") == "" {
				// Dev mode: no Zitadel deployment, no signing key.
				log.Printf("[ACTION] %s unset and ZITADEL_DOMAIN unset — signature verification disabled (dev mode)", secretEnvVar)
				next(w, r)
				return
			}
			// Production with empty signing key: startup gate should have caught
			// this. Refuse the request rather than fall through silently.
			log.Printf("[ACTION] %s unset while ZITADEL_DOMAIN set — refusing request (misconfiguration)", secretEnvVar)
			jsonErrorResponse(w, http.StatusServiceUnavailable, "MISCONFIGURED", "Signing key not configured for this endpoint")
			return
		}
		// ... rest of function unchanged (body read + verifyZitadelActionSignature)
```

The doc comment at lines 38-49 needs a one-line tweak:

Replace `// fall-through already established by withUserAuth (no ZITADEL_DOMAIN set).` with `// fall-through allowed only when ZITADEL_DOMAIN is also unset (dev mode).`

- [ ] **Step 3.2: Update the existing dev-mode test to assert the dev-only condition**

In `backend/internal/handlers/zitadel_action_auth_test.go:132-144`, replace `TestWithZitadelActionSignature_DevModePassthrough` with:

```go
func TestWithZitadelActionSignature_DevModePassthrough(t *testing.T) {
	// Dev mode = both ZITADEL_DOMAIN and the signing-key env are empty.
	t.Setenv(testSecretEnv, "")
	t.Setenv("ZITADEL_DOMAIN", "")

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev mode (both unset) should pass through, got %d", rr.Code)
	}
}
```

- [ ] **Step 3.3: Add a new failing test for the production-misconfig path**

Append to the same test file:

```go
func TestWithZitadelActionSignature_ProductionRefusesEmptySecret(t *testing.T) {
	// Production: ZITADEL_DOMAIN set, signing-key env empty. Even though the
	// startup gate should have refused this configuration, the middleware MUST
	// refuse the request rather than fall through silently.
	t.Setenv(testSecretEnv, "")
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.test")

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("production with empty secret must return 503, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MISCONFIGURED") {
		t.Fatalf("expected MISCONFIGURED error code in body, got %s", rr.Body.String())
	}
}
```

- [ ] **Step 3.4: Run tests, verify both pass**

Run: `cd backend && go test ./internal/handlers/ -run TestWithZitadelActionSignature -v`
Expected: all `TestWithZitadelActionSignature_*` tests PASS, including the new `_ProductionRefusesEmptySecret`.

- [ ] **Step 3.5: Commit**

```bash
git add backend/internal/handlers/zitadel_action_auth.go backend/internal/handlers/zitadel_action_auth_test.go
git commit -m "backend: refuse webhook/action requests with empty signing key in production (audit C1)

withZitadelActionSignature now distinguishes dev mode (both
ZITADEL_DOMAIN and signing-key env empty) from production
misconfiguration (ZITADEL_DOMAIN set, signing-key empty). The latter
returns 503 MISCONFIGURED instead of falling through unverified."
```

---

## Task 4: `is_welcome` migration + repository rewrite (D1)

Schema-level enforcement that at most one welcome bundle exists, plus an explicit query that errors when none does.

**Files:**
- Create: `backend/db/migrations/000012_welcome_bundle_flag.up.sql`
- Create: `backend/db/migrations/000012_welcome_bundle_flag.down.sql`
- Modify: `backend/internal/db/repositories.go:604-628` (rewrite `GetWelcomeBundle`)
- Modify: `backend/internal/db/repositories.go` (append `SetWelcomeBundle`, `ErrNoWelcomeBundleConfigured`)

- [ ] **Step 4.1: Write the up migration**

Create `backend/db/migrations/000012_welcome_bundle_flag.up.sql`:

```sql
-- Welcome-bundle flag. Replaces the convention-based name match in
-- GetWelcomeBundle (May 2026 audit D1). The partial unique index enforces
-- "at most one welcome bundle" at the database layer; application code is
-- still expected to use a transaction when toggling.

ALTER TABLE bundles
    ADD COLUMN is_welcome BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX idx_bundles_welcome_unique
    ON bundles (is_welcome)
    WHERE is_welcome = TRUE;
```

- [ ] **Step 4.2: Write the down migration**

Create `backend/db/migrations/000012_welcome_bundle_flag.down.sql`:

```sql
DROP INDEX IF EXISTS idx_bundles_welcome_unique;
ALTER TABLE bundles DROP COLUMN IF EXISTS is_welcome;
```

- [ ] **Step 4.3: Replace `GetWelcomeBundle`**

In `backend/internal/db/repositories.go:604-628`, replace the function and its docstring with:

```go
// ErrNoWelcomeBundleConfigured is returned by GetWelcomeBundle when no row in
// the bundles table has is_welcome=TRUE. Onboarding propagates this verbatim
// so operators see a named cause in the trigger UI instead of getting the
// "first bundle by created_at" silent default that the May 2026 audit (D1)
// flagged as a trust hazard.
var ErrNoWelcomeBundleConfigured = errors.New("no welcome bundle configured")

// GetWelcomeBundle returns the ID of the bundle marked is_welcome=TRUE, or
// ErrNoWelcomeBundleConfigured if no bundle has been designated. The
// at-most-one constraint is enforced at the schema layer
// (idx_bundles_welcome_unique).
func GetWelcomeBundle(ctx context.Context) (string, error) {
	var id string
	err := PG.QueryRow(ctx, `SELECT id FROM bundles WHERE is_welcome = TRUE`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoWelcomeBundleConfigured
	}
	if err != nil {
		return "", fmt.Errorf("query welcome bundle: %w", err)
	}
	return id, nil
}

// SetWelcomeBundle marks bundleID as the welcome bundle. Transactional
// clear-then-set: any previously-flagged bundle is unset before the new one
// is set, so the partial unique index never trips. Returns sql.ErrNoRows if
// bundleID does not exist.
func SetWelcomeBundle(ctx context.Context, bundleID string) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set-welcome-bundle: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE bundles SET is_welcome = FALSE WHERE is_welcome = TRUE`); err != nil {
		return fmt.Errorf("clear previous welcome bundle: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE bundles SET is_welcome = TRUE WHERE id = $1`, bundleID)
	if err != nil {
		return fmt.Errorf("mark welcome bundle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set-welcome-bundle: %w", err)
	}
	return nil
}
```

The `errors` import is already present in `repositories.go` (used by `pgx.ErrNoRows` checks elsewhere); confirm with `grep "\"errors\"" backend/internal/db/repositories.go`. If absent, add to the import block.

- [ ] **Step 4.4: Update the `Bundle` model to expose the flag**

In `backend/internal/models/models.go:13-19`, extend the `Bundle` struct:

```go
type Bundle struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsWelcome   bool      `json:"is_welcome"`
	Roles       []string  `json:"roles"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 4.5: Update `GetAllBundles` to select the new column**

In `backend/internal/db/repositories.go:43-60`, the existing function is:

```go
func GetAllBundles(ctx context.Context) ([]models.Bundle, error) {
	query := `SELECT id, name, description, created_at FROM bundles ORDER BY created_at DESC;`
	// ...
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.CreatedAt); err != nil {
```

Replace both lines with:

```go
	query := `SELECT id, name, description, is_welcome, created_at FROM bundles ORDER BY created_at DESC;`
```
and
```go
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.CreatedAt); err != nil {
```

`GetBundlesForUser` (line 102) selects the same columns; apply the same change there.

- [ ] **Step 4.6: Run the migration locally and verify**

```bash
cd backend && make db-migrate-up   # or whatever the local migration command is
psql $DATABASE_URL -c "\d bundles" | grep -E "is_welcome|idx_bundles_welcome"
```
Expected: column listed with type `boolean`, default `false`, NOT NULL; index listed.

- [ ] **Step 4.7: Run vet + build**

Run: `cd backend && go vet ./... && go build ./...`
Expected: exit 0.

- [ ] **Step 4.8: Commit**

```bash
git add backend/db/migrations/000012_welcome_bundle_flag.up.sql backend/db/migrations/000012_welcome_bundle_flag.down.sql backend/internal/db/repositories.go backend/internal/models/models.go
git commit -m "backend: replace welcome-bundle convention with explicit is_welcome flag (audit D1)

Adds bundles.is_welcome column with a partial unique index ensuring at
most one welcome bundle. GetWelcomeBundle now returns
ErrNoWelcomeBundleConfigured when no bundle is marked, eliminating the
silent first-bundle-by-created_at default. SetWelcomeBundle is
transactional clear-then-set."
```

---

## Task 5: Welcome bundle handler + onboarding error path (D1)

Surface the new repository contract through the API and propagate the named error through onboarding.

**Files:**
- Modify: `backend/internal/handlers/bundles.go` (append `handleSetWelcomeBundle`)
- Modify: `backend/internal/handlers/router.go` (wire `PUT /api/v1/bundles/{id}/welcome`)
- Modify: `backend/internal/handlers/bundles_test.go` (cover the new handler)
- Modify: `backend/internal/services/onboarding.go:36-43` (preserve sentinel via `errors.Is`)
- Modify: `backend/internal/services/onboarding_test.go:113-115` (use sentinel)
- Modify: `backend/internal/services/deps.go` (no functional change — confirm signature still matches)

- [ ] **Step 5.1: Extend the existing `resetBundleDeps` helper**

`backend/internal/handlers/bundles_test.go` already defines `resetBundleDeps(t *testing.T)` at the top. Extend it to capture `dbSetWelcomeBundle`:

```go
func resetBundleDeps(t *testing.T) {
	t.Helper()
	origCreate := dbCreateBundle
	origGetAll := dbGetAllBundles
	origGetRoles := dbGetRolesForBundle
	origSetWelcome := dbSetWelcomeBundle
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbCreateBundle = origCreate
		dbGetAllBundles = origGetAll
		dbGetRolesForBundle = origGetRoles
		dbSetWelcomeBundle = origSetWelcome
		dbInsertAuditLog = origAudit
	})
}
```

(Preserve the existing captured fields exactly — only `origSetWelcome` and `origAudit` are new. If the file's current restore block omits `dbInsertAuditLog`, add the restore line accordingly.)

- [ ] **Step 5.1.1: Write the failing handler tests**

Append to the same file:

```go
func TestHandleSetWelcomeBundle_Success(t *testing.T) {
	resetBundleDeps(t)

	called := ""
	dbSetWelcomeBundle = func(_ context.Context, id string) error {
		called = id
		return nil
	}
	dbInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b-123/welcome", nil)
	req.SetPathValue("id", "b-123")
	rr := httptest.NewRecorder()

	handleSetWelcomeBundle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if called != "b-123" {
		t.Fatalf("expected SetWelcomeBundle called with b-123, got %q", called)
	}
}

func TestHandleSetWelcomeBundle_NotFound(t *testing.T) {
	resetBundleDeps(t)
	dbSetWelcomeBundle = func(_ context.Context, _ string) error {
		return pgx.ErrNoRows
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/missing/welcome", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	handleSetWelcomeBundle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
```

Add `"github.com/jackc/pgx/v5"` to the test file imports if not already present.

- [ ] **Step 5.2: Run the test, verify it fails for "undefined"**

Run: `cd backend && go test ./internal/handlers/ -run TestHandleSetWelcomeBundle -v`
Expected: FAIL with `undefined: handleSetWelcomeBundle` and/or `undefined: dbSetWelcomeBundle`.

- [ ] **Step 5.3: Add the injectable dep**

In `backend/internal/handlers/deps.go`, append:

```go
var dbSetWelcomeBundle = func(ctx context.Context, bundleID string) error {
	return db.SetWelcomeBundle(ctx, bundleID)
}
```

(Match the file's existing pattern — confirm by reading the first 30 lines of `deps.go`.)

- [ ] **Step 5.4: Add the handler**

Append to `backend/internal/handlers/bundles.go`:

```go
// handleSetWelcomeBundle marks a bundle as the welcome bundle. Clears any
// previous welcome flag in the same transaction (see db.SetWelcomeBundle).
// PUT /api/v1/bundles/{id}/welcome
func handleSetWelcomeBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	if err := dbSetWelcomeBundle(r.Context(), bundleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.welcome_set", bundleID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Welcome bundle set"})
}
```

Add `"errors"` and `"github.com/jackc/pgx/v5"` to the imports if absent.

- [ ] **Step 5.5: Wire the route in `router.go`**

In `backend/internal/handlers/router.go`, near line 30 (the existing bundle routes block), add:

```go
mux.HandleFunc("PUT /api/v1/bundles/{id}/welcome", withCORS(withUserAuth(handleSetWelcomeBundle)))
```

- [ ] **Step 5.6: Re-run the handler tests, verify they pass**

Run: `cd backend && go test ./internal/handlers/ -run TestHandleSetWelcomeBundle -v`
Expected: PASS for both subtests.

- [ ] **Step 5.7: Update the onboarding service to preserve the sentinel**

In `backend/internal/services/onboarding.go:36-43`, replace:

```go
	bundleID, err := svcGetWelcomeBundle(ctx)
	if err != nil {
		log.Printf("[ONBOARDING] No welcome bundle available for user=%s: %v", userID, err)
		if failErr := svcFailOnboardingTrigger(ctx, triggerID, err.Error()); failErr != nil {
			log.Printf("[ONBOARDING] Failed to record failure for trigger=%s: %v", triggerID, failErr)
		}
		return fmt.Errorf("find welcome bundle: %w", err)
	}
```

with:

```go
	bundleID, err := svcGetWelcomeBundle(ctx)
	if err != nil {
		// db.ErrNoWelcomeBundleConfigured is the named cause; preserve via %w
		// so callers can errors.Is against it for tests + alerting.
		log.Printf("[ONBOARDING] No welcome bundle configured for user=%s: %v", userID, err)
		if failErr := svcFailOnboardingTrigger(ctx, triggerID, err.Error()); failErr != nil {
			log.Printf("[ONBOARDING] Failed to record failure for trigger=%s: %v", triggerID, failErr)
		}
		return fmt.Errorf("welcome bundle: %w", err)
	}
```

- [ ] **Step 5.8: Update the onboarding test to assert the sentinel**

In `backend/internal/services/onboarding_test.go:113-115`, the test currently uses `errors.New("no bundles available")`. Replace with the named sentinel and assert it:

```go
import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
)

// ... TestTriggerOnboarding_NoBundleAvailable_MarksFailedAndReturnsError:

func TestTriggerOnboarding_NoBundleAvailable_MarksFailedAndReturnsError(t *testing.T) {
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "trigger-id-2", true, nil
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		return "", db.ErrNoWelcomeBundleConfigured
	}
	failedID := ""
	svcFailOnboardingTrigger = func(_ context.Context, triggerID, _ string) error {
		failedID = triggerID
		return nil
	}
	svcAssignBundleToUser = func(_ context.Context, _, _ string) error {
		t.Error("AssignBundleToUser called when no bundle available")
		return nil
	}

	err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-nobundle")
	if err == nil {
		t.Fatal("expected error when no bundle available")
	}
	if !errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
		t.Fatalf("expected ErrNoWelcomeBundleConfigured, got %v", err)
	}
	if failedID != "trigger-id-2" {
		t.Fatalf("expected FailOnboardingTrigger called with trigger-id-2, got %q", failedID)
	}
}
```

- [ ] **Step 5.9: Run the onboarding tests**

Run: `cd backend && go test ./internal/services/ -run TestTriggerOnboarding -v`
Expected: all `TestTriggerOnboarding_*` PASS.

- [ ] **Step 5.10: Commit**

```bash
git add backend/internal/handlers/bundles.go backend/internal/handlers/bundles_test.go backend/internal/handlers/deps.go backend/internal/handlers/router.go backend/internal/services/onboarding.go backend/internal/services/onboarding_test.go
git commit -m "backend: PUT /api/v1/bundles/{id}/welcome + named onboarding error (audit D1)

Surfaces SetWelcomeBundle through the API; onboarding service propagates
db.ErrNoWelcomeBundleConfigured verbatim so operators see the named
cause in the failed-trigger UI."
```

---

## Task 6: Welcome bundle UI toggle (D1)

Operator-facing affordance. The bundle list shows a `Welcome` badge on the active bundle; the row exposes a `Set as welcome bundle` button.

**Files:**
- Modify: `ui/src/lib/queries/useBundles.ts` (add `is_welcome`, `useSetWelcomeBundle`)
- Modify: `ui/src/app/bundles/page.tsx` (badge + button + toast)
- Modify: `ui/src/app/bundles/__tests__/page.test.tsx` (smoke-test interaction)

- [ ] **Step 6.1: Extend `BundleRow` and add the mutation hook**

In `ui/src/lib/queries/useBundles.ts`:

Replace the `BundleRow` interface (around line 7-13) with:

```ts
export interface BundleRow {
  id: string;
  name: string;
  description?: string;
  is_welcome?: boolean;
  roles?: string[];
  created_at?: string;
}
```

Append before the `bundlesQueryKeys` export:

```ts
/** Set a bundle as the system's welcome bundle (transactional clear-then-set). */
export function useSetWelcomeBundle() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (bundleId: string) => {
      return await request(`/bundles/${bundleId}/welcome`, { method: "PUT" });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}
```

- [ ] **Step 6.2: Add the badge + button to `BundleRowCard`**

In `ui/src/app/bundles/page.tsx`:

Inside `BundleRowCard`, in the header `<div className="flex items-center gap-2">` block (around lines 127-137), insert before the `created_at` Badge:

```tsx
{bundle.is_welcome && (
  <Badge variant="secondary" className="bg-tertiary-container text-on-tertiary-container">
    Welcome
  </Badge>
)}
```

Then in the expanded panel (after the `Manage roles` Button, around line 165), add a new line before `<BundleImpactAccordion />`:

```tsx
<div className="flex items-center justify-between gap-3 flex-wrap pt-3 border-t border-outline-variant">
  <div>
    <Eyebrow>Welcome bundle</Eyebrow>
    <p className="mt-1 text-sm text-on-surface-variant">
      {bundle.is_welcome
        ? "Assigned to every newly-created Zitadel user via the onboarding trigger."
        : "Mark this bundle as the welcome bundle to assign it on user creation."}
    </p>
  </div>
  <SetWelcomeButton bundle={bundle} />
</div>
```

Add the new component at the bottom of the file:

```tsx
function SetWelcomeButton({ bundle }: { bundle: BundleRow }) {
  const setWelcome = useSetWelcomeBundle();
  if (bundle.is_welcome) {
    return (
      <Button variant="secondary" size="sm" disabled aria-label="Already welcome bundle">
        Welcome bundle
      </Button>
    );
  }
  return (
    <Button
      variant="primary"
      size="sm"
      onClick={() => setWelcome.mutate(bundle.id)}
      disabled={setWelcome.isPending}
      aria-label={`Set ${bundle.name} as welcome bundle`}
    >
      {setWelcome.isPending ? "Setting…" : "Set as welcome bundle"}
    </Button>
  );
}
```

Update the imports at the top of the file:

```tsx
import { useBundleRoles, useBundles, useSetWelcomeBundle, type BundleRow } from "@/lib/queries/useBundles";
```

- [ ] **Step 6.3: Add a smoke test using the existing test infra**

`ui/src/app/bundles/__tests__/page.test.tsx` already imports `describe`/`it`/`expect`/`vi`/`render`/`screen`/`fireEvent`/`waitFor` from vitest + RTL, plus `BundlesView`, `makeProxyFetch`, `UUID_REGEX`, and a top-level `beforeEach` that creates `proxy = makeProxyFetch()` and registers the Mentor Pack bundle list. There is no `TestQueryClientProvider` export — use the file's existing `renderBundles()` helper.

Two changes:

(a) **Append a new `describe` block at the bottom of the file** (no new imports):

```tsx
describe("BundlesView welcome-bundle toggle", () => {
  // Override the bundles list registered by the top-level beforeEach with one
  // that flags Mentor Pack as the welcome bundle. Use a fresh proxy so the
  // first-match-wins ordering inside makeProxyFetch is irrelevant.
  beforeEach(() => {
    proxy = makeProxyFetch();
    global.fetch = proxy.fetchImpl;
    proxy.register("GET", /\/api\/proxy\/bundles(\?|$)/, () => [
      {
        id: "b1",
        name: "Mentor Pack",
        description: "Hands-on training",
        is_welcome: true,
        created_at: new Date().toISOString(),
      },
    ]);
    proxy.register("PUT", /\/api\/proxy\/bundles\/[^/]+\/welcome/, () => ({ message: "Welcome bundle set" }));
  });

  it("renders the Welcome badge for the flagged bundle", async () => {
    renderBundles();
    expect(await screen.findByText("Welcome")).toBeInTheDocument();
  });

  it("disables the Set as welcome bundle button when already flagged", async () => {
    renderBundles();
    const header = await screen.findByRole("button", { name: /Mentor Pack/ });
    fireEvent.click(header);
    const btn = await screen.findByRole("button", { name: /Welcome bundle/i });
    expect(btn).toBeDisabled();
  });
});
```

(b) **No import edits required** — `proxy`, `makeProxyFetch`, `renderBundles`, `screen`, `fireEvent`, `describe`, `it`, `expect`, and `beforeEach` are all already in scope at the top of the file.

If for some reason the second `it` flakes because the bundle row text "Mentor Pack" is hidden (the toggle test in Step 6.2 changes header copy), assert against the `Welcome bundle` button by `aria-label` instead of role-name.

- [ ] **Step 6.4: Run UI tests + lint**

Run: `cd ui && bun run test src/app/bundles && bun run lint`
Expected: tests PASS; lint exit 0.

- [ ] **Step 6.5: Run typecheck**

Run: `cd ui && bun run build`
Expected: build succeeds.

- [ ] **Step 6.6: Commit**

```bash
git add ui/src/lib/queries/useBundles.ts ui/src/app/bundles/page.tsx ui/src/app/bundles/__tests__/page.test.tsx
git commit -m "ui: welcome-bundle toggle in bundle library (audit D1)

Bundles list shows Welcome badge for the flagged bundle; the row exposes
a Set as welcome bundle button that calls PUT /bundles/{id}/welcome
and invalidates the bundle list query."
```

---

## Task 7: Vault dev-mode `?actor=` requirement (C3)

Refuse mutations in dev mode unless an explicit actor query parameter is supplied; reads remain unchanged.

**Files:**
- Modify: `backend/internal/handlers/vault.go:13-30` (extend `enforceSelfOnly` signature)
- Modify: `backend/internal/handlers/vault.go` (update mutation handler call sites)
- Modify: `backend/internal/handlers/vault_test.go` (add dev-mode 400 + happy-path coverage)

- [ ] **Step 7.1: Write the failing tests first**

Append to `backend/internal/handlers/vault_test.go`:

```go
func TestHandleSetShadowCredential_DevModeRequiresActor(t *testing.T) {
	setupNoopVaultDeps(t)

	body := `{"password":"Str0ng!Pass99"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	// No actor in JWT context (dev mode), no ?actor= query param.
	rr := httptest.NewRecorder()

	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("dev-mode mutation without ?actor= must return 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MISSING_ACTOR") {
		t.Fatalf("expected MISSING_ACTOR in body, got %s", rr.Body.String())
	}
}

func TestHandleSetShadowCredential_DevModeWithExplicitActor(t *testing.T) {
	setupNoopVaultDeps(t)

	receivedActor := ""
	svcSetShadowPassword = func(_ context.Context, uid, actorID, _, _ string) error {
		if uid != "u1" {
			t.Fatalf("expected uid u1, got %q", uid)
		}
		receivedActor = actorID
		return nil
	}

	body := `{"password":"Str0ng!Pass99"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u1/shadow-credential?actor=alice@cli", strings.NewReader(body))
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()

	handleSetShadowCredential(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with explicit actor, got %d: %s", rr.Code, rr.Body.String())
	}
	if receivedActor != "alice@cli" {
		t.Fatalf("expected actor=alice@cli, got %q", receivedActor)
	}
}

func TestHandleGetShadowCredentialStatus_DevModeNoActorRequired(t *testing.T) {
	// Reads are NOT affected by the new requirement — operators inspecting
	// status in dev mode would otherwise need a meaningless ?actor=.
	setupNoopVaultDeps(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/shadow-credential/status", nil)
	req.SetPathValue("uid", "u1")
	rr := httptest.NewRecorder()

	handleGetShadowCredentialStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev-mode read must not require ?actor=, got %d: %s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 7.2: Run tests, verify they fail**

Run: `cd backend && go test ./internal/handlers/ -run "TestHandleSetShadowCredential_DevMode|TestHandleGetShadowCredentialStatus_DevMode" -v`
Expected: `_DevModeRequiresActor` FAILS (returns 200 instead of 400) — that's the regression. The other two may pass already.

- [ ] **Step 7.3: Extend `enforceSelfOnly` and update call sites**

Replace the entire current `enforceSelfOnly` (lines 10-30 of `backend/internal/handlers/vault.go`) with:

```go
// enforceSelfOnly verifies that the acting user matches the target {uid}.
//
// Production mode (JWT actor present): the actor MUST equal {uid}; otherwise
// 403. The actor is used for audit attribution.
//
// Dev mode (API-key auth, no JWT actor): if requireActor is true (mutations),
// the caller MUST provide ?actor=<id> to attribute the action — without it,
// the audit log would record the target user as the actor (May 2026 audit C3).
// If requireActor is false (reads), the actor falls back to {uid} silently.
//
// Returns false and writes the appropriate JSON error response if the check
// fails. The returned actorID is what should appear in audit_logs.
func enforceSelfOnly(w http.ResponseWriter, r *http.Request, requireActor bool) (uid, actorID string, ok bool) {
	uid = r.PathValue("uid")
	if uid == "" {
		jsonValidationErrorResponse(w, "Missing user ID", map[string]string{"uid": "required"})
		return "", "", false
	}
	actorID = getAdminUserID(r.Context())
	if actorID != "" && actorID != uid {
		jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "You can only manage your own shadow credential")
		return "", "", false
	}
	if actorID == "" {
		// Dev mode: no JWT actor.
		if requireActor {
			actorID = strings.TrimSpace(r.URL.Query().Get("actor"))
			if actorID == "" {
				jsonErrorResponse(w, http.StatusBadRequest, "MISSING_ACTOR", "Dev-mode mutations require ?actor=<id> for audit attribution")
				return "", "", false
			}
			log.Printf("[VAULT] dev-mode actor=%s for %s %s", actorID, r.Method, r.URL.Path)
		} else {
			actorID = uid
		}
	}
	return uid, actorID, true
}
```

Add `"log"` and `"strings"` to the import block (after the existing `errors` and `net/http`).

- [ ] **Step 7.4: Update the four call sites**

In the same file, replace the four `enforceSelfOnly(w, r)` invocations:

- `handleSetShadowCredential` (line 35): `uid, actorID, ok := enforceSelfOnly(w, r, true)` *(mutation)*
- `handleClearShadowCredential` (line 67): `uid, actorID, ok := enforceSelfOnly(w, r, true)` *(mutation)*
- `handleGetShadowCredentialStatus` (line 86): `uid, _, ok := enforceSelfOnly(w, r, false)` *(read)*
- `handleGetShadowCredentialAudit` (line 102): `uid, _, ok := enforceSelfOnly(w, r, false)` *(read)*

- [ ] **Step 7.5: Run all vault tests**

Run: `cd backend && go test ./internal/handlers/ -run TestHandle.*ShadowCredential -v`
Expected: all PASS, including the three new tests.

- [ ] **Step 7.6: Run vet**

Run: `cd backend && go vet ./...`
Expected: exit 0.

- [ ] **Step 7.7: Commit**

```bash
git add backend/internal/handlers/vault.go backend/internal/handlers/vault_test.go
git commit -m "backend: vault mutations require ?actor= in dev mode (audit C3)

enforceSelfOnly now takes a requireActor flag. Mutations (PUT/DELETE)
pass true and refuse with 400 MISSING_ACTOR if neither a JWT actor nor
a ?actor= query parameter is present. Reads pass false and continue to
fall back to {uid} for the audit row, matching prior behaviour."
```

---

## Task 8: `/api/v1/me/profile` endpoint (C2/D5)

Backend half of the OIDC-metadata fix. A user-auth-gated endpoint that returns the requester's full `UserProfile` — the same shape `directory.Default.FindUser` already produces, with the Zitadel metadata overlay applied.

**Files:**
- Modify: `backend/internal/handlers/profile.go` (add `handleGetMyProfile`)
- Modify: `backend/internal/handlers/router.go` (wire `GET /api/v1/me/profile`)
- Create: `backend/internal/handlers/profile_test.go`

- [ ] **Step 8.1: Write the failing test**

Create `backend/internal/handlers/profile_test.go`:

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/directory"
	"syndra/internal/models"
)

type stubDirectory struct {
	users map[string]models.UserProfile
}

func (s *stubDirectory) Users(_ context.Context) ([]models.UserProfile, error) { return nil, nil }
func (s *stubDirectory) FindUser(_ context.Context, userID string) (models.UserProfile, bool, error) {
	u, ok := s.users[userID]
	return u, ok, nil
}
func (s *stubDirectory) Projects(_ context.Context) ([]models.ProjectCatalog, error) { return nil, nil }
func (s *stubDirectory) FindProject(_ context.Context, _ string) (models.ProjectCatalog, bool, error) {
	return models.ProjectCatalog{}, false, nil
}
func (s *stubDirectory) Applications(_ context.Context) ([]models.ApplicationCatalog, error) {
	return nil, nil
}
func (s *stubDirectory) FindApplication(_ context.Context, _ string) (models.ApplicationCatalog, bool, error) {
	return models.ApplicationCatalog{}, false, nil
}
func (s *stubDirectory) RoleKeysForProject(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubDirectory) ProjectName(_ context.Context, _ string) (string, error)         { return "", nil }
func (s *stubDirectory) Tag() string                                                     { return "stub" }
func (s *stubDirectory) InvalidateAll()                                                  {}
func (s *stubDirectory) InvalidateProject(_ string)                                      {}
func (s *stubDirectory) InvalidateUsers()                                                {}

func TestHandleGetMyProfile_Success(t *testing.T) {
	orig := directory.Default
	directory.Default = &stubDirectory{users: map[string]models.UserProfile{
		"u1": {ID: "u1", Name: "Alice", Email: "alice@x.test", Title: "Director", Team: "Ops", Location: "HQ", Status: "active"},
	}}
	t.Cleanup(func() { directory.Default = orig })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	req = req.WithContext(withAdminUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got models.UserProfile
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "Director" || got.Team != "Ops" || got.Location != "HQ" {
		t.Fatalf("expected metadata-overlay populated, got %+v", got)
	}
}

func TestHandleGetMyProfile_NoActor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no actor in context, got %d", rr.Code)
	}
}

func TestHandleGetMyProfile_NotFound(t *testing.T) {
	orig := directory.Default
	directory.Default = &stubDirectory{users: map[string]models.UserProfile{}}
	t.Cleanup(func() { directory.Default = orig })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	req = req.WithContext(withAdminUserID(req.Context(), "ghost"))
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
```

(Drop the `errors` import line from the test file if no other usage remains; goimports will clean it on save.)

- [ ] **Step 8.2: Run, verify it fails for "undefined"**

Run: `cd backend && go test ./internal/handlers/ -run TestHandleGetMyProfile -v`
Expected: FAIL with `undefined: handleGetMyProfile`.

- [ ] **Step 8.3: Add the handler**

Append to `backend/internal/handlers/profile.go`:

```go
// handleGetMyProfile returns the requester's full UserProfile — the same
// shape directory.Default.FindUser produces, with title/team/location
// overlaid from Zitadel metadata. Used by the Next.js OIDC callback to
// populate the session cookie so OIDC and demo sessions render identically
// (May 2026 audit C2/D5).
// GET /api/v1/me/profile  (auth: withUserAuth)
func handleGetMyProfile(w http.ResponseWriter, r *http.Request) {
	uid := getAdminUserID(r.Context())
	if uid == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "No actor in request context")
		return
	}

	profile, found, err := directory.Default.FindUser(r.Context(), uid)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Failed to resolve profile: "+err.Error())
		return
	}
	if !found {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Profile not found for current actor")
		return
	}
	jsonResponse(w, http.StatusOK, profile)
}
```

Add `"syndra/internal/directory"` to the import block.

- [ ] **Step 8.4: Wire the route**

In `backend/internal/handlers/router.go`, near line 89 (the existing `handleGetUserProfile` line) add a new entry:

```go
mux.HandleFunc("GET /api/v1/me/profile", withCORS(withUserAuth(handleGetMyProfile)))
```

Place it right after the existing line `mux.HandleFunc("GET /api/v1/users/{uid}/profile", withCORS(withAPIKeyAuth(handleGetUserProfile)))` so the related routes stay grouped.

- [ ] **Step 8.5: Run tests, verify they pass**

Run: `cd backend && go test ./internal/handlers/ -run TestHandleGetMyProfile -v`
Expected: all three sub-tests PASS.

- [ ] **Step 8.6: Full backend test sweep**

Run: `cd backend && go test ./... && go vet ./...`
Expected: exit 0; no flaky regressions.

- [ ] **Step 8.7: Commit**

```bash
git add backend/internal/handlers/profile.go backend/internal/handlers/profile_test.go backend/internal/handlers/router.go
git commit -m "backend: GET /api/v1/me/profile returns requester UserProfile (audit C2/D5)

Reuses directory.Default.FindUser so the metadata overlay (Title/Team/
Location) is identical to the rendering on the /users list. Gated by
withUserAuth; resolves the user ID from the bearer-token actor."
```

---

## Task 9: OIDC callback fetches profile, cookie carries title/team/location (C2/D5)

UI half. After token exchange, call the new endpoint and embed the profile into the session cookie so the dashboard renders the operator's title/team/location for OIDC sessions.

**Files:**
- Modify: `ui/src/lib/oidc.ts` (add `fetchProfileMetadata`)
- Modify: `ui/src/lib/session.ts` (extend `OidcSessionCookie`, propagate into `SessionUser`)
- Modify: `ui/src/lib/__tests__/session.test.ts` (round-trip test)
- Modify: `ui/src/app/auth/callback/route.ts` (wire the fetch + cookie write)

- [ ] **Step 9.1: Add `fetchProfileMetadata` helper**

Append to `ui/src/lib/oidc.ts`:

```ts
// ---------------------------------------------------------------------------
// Profile metadata fetch — populates Title/Team/Location for OIDC sessions
// ---------------------------------------------------------------------------

export interface ProfileMetadata {
  title: string;
  team: string;
  location: string;
  status: string;
}

/**
 * Fetches the authenticated user's profile from /api/v1/me/profile using the
 * freshly-issued access token. Returns empty metadata on any failure — the
 * OIDC callback must continue (the cookie is still valid, dashboard fields
 * just render blank). Backend is the canonical source; we never derive these
 * from token claims.
 */
export async function fetchProfileMetadata(
  accessToken: string,
  backendUrl: string,
): Promise<ProfileMetadata> {
  const empty: ProfileMetadata = { title: "", team: "", location: "", status: "active" };
  try {
    const res = await fetch(`${backendUrl}/api/v1/me/profile`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: "no-store",
    });
    if (!res.ok) return empty;
    const body = (await res.json()) as Partial<ProfileMetadata>;
    return {
      title: typeof body.title === "string" ? body.title : "",
      team: typeof body.team === "string" ? body.team : "",
      location: typeof body.location === "string" ? body.location : "",
      status: typeof body.status === "string" ? body.status : "active",
    };
  } catch {
    return empty;
  }
}
```

- [ ] **Step 9.2: Extend `OidcSessionCookie` and the session decoder**

In `ui/src/lib/session.ts`:

Replace the `OidcSessionCookie` interface (lines 31-39) with:

```ts
export interface OidcSessionCookie {
  type: "oidc";
  accessToken: string;
  userId: string;
  role: SessionRole;
  name: string;
  email: string;
  title: string;
  team: string;
  location: string;
  status: string;
  expiresAt: number; // Unix seconds
}
```

In `decodeSessionPayload` (around lines 134-161), the OIDC validation block currently checks `accessToken`/`userId`/`role`/`name`/`email`/`expiresAt`. Add the new fields with safe defaults — they may be missing from older cookies issued before this change:

Replace the current OIDC branch:
```ts
    if (parsed.type === "oidc") {
      if (
        typeof parsed.accessToken !== "string" ||
        typeof parsed.userId !== "string" ||
        (parsed.role !== "admin" && parsed.role !== "user") ||
        typeof parsed.name !== "string" ||
        typeof parsed.email !== "string" ||
        typeof parsed.expiresAt !== "number"
      ) {
        return null;
      }
      return parsed as OidcSessionCookie;
    }
```

with:

```ts
    if (parsed.type === "oidc") {
      if (
        typeof parsed.accessToken !== "string" ||
        typeof parsed.userId !== "string" ||
        (parsed.role !== "admin" && parsed.role !== "user") ||
        typeof parsed.name !== "string" ||
        typeof parsed.email !== "string" ||
        typeof parsed.expiresAt !== "number"
      ) {
        return null;
      }
      return {
        type: "oidc",
        accessToken: parsed.accessToken,
        userId: parsed.userId,
        role: parsed.role,
        name: parsed.name,
        email: parsed.email,
        title: typeof parsed.title === "string" ? parsed.title : "",
        team: typeof parsed.team === "string" ? parsed.team : "",
        location: typeof parsed.location === "string" ? parsed.location : "",
        status: typeof parsed.status === "string" ? parsed.status : "active",
        expiresAt: parsed.expiresAt,
      };
    }
```

In `getSession()` (lines 186-222), the OIDC branch builds the `SessionUser` with empty strings for `title`/`team`/`location`. Replace those lines with the cookie fields:

```ts
  if (payload.type === "oidc") {
    if (Date.now() / 1000 > payload.expiresAt) return null;

    return {
      id: payload.userId,
      name: payload.name,
      email: payload.email,
      title: payload.title,
      team: payload.team,
      status: payload.status,
      location: payload.location,
      avatar: nameToAvatar(payload.name),
      role: payload.role,
      accessToken: payload.accessToken,
      sessionType: "oidc",
    };
  }
```

- [ ] **Step 9.3: Add a round-trip test**

`ui/src/lib/__tests__/session.test.ts` already imports `describe`/`it`/`expect`/`vi`/`beforeEach`/`afterEach` and `createOidcSessionValue`/`getSession`/`SESSION_COOKIE_NAME` at the top. **Do not re-import anything.** Append the new `describe` block at the bottom of the file:

```ts
describe("OidcSessionCookie metadata round-trip", () => {
  it("encodes title/team/location/status into the cookie payload", () => {
    const value = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Alice",
      email: "alice@x.test",
      title: "Director",
      team: "Ops",
      location: "HQ",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const decoded = JSON.parse(Buffer.from(value, "base64url").toString("utf8"));
    expect(decoded.title).toBe("Director");
    expect(decoded.team).toBe("Ops");
    expect(decoded.location).toBe("HQ");
    expect(decoded.status).toBe("active");
  });

  it("getSession surfaces title/team/location on OidcSessionUser", async () => {
    process.env.ZITADEL_DOMAIN = "https://zitadel.example";
    mockCookieValue = createOidcSessionValue({
      type: "oidc",
      accessToken: "tok",
      userId: "u1",
      role: "user",
      name: "Alice",
      email: "alice@x.test",
      title: "Director",
      team: "Ops",
      location: "HQ",
      status: "active",
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    });
    const session = await getSession();
    expect(session?.title).toBe("Director");
    expect(session?.team).toBe("Ops");
    expect(session?.location).toBe("HQ");
  });
});
```

The `mockCookieValue` and the `process.env.ZITADEL_DOMAIN` afterEach restore are defined in the file's existing `getSession` suite. Because `mockCookieValue` is a top-level `let` and the `next/headers` mock reads it dynamically, the new test can reuse it without redefining the mock — but the `afterEach` for `ZITADEL_DOMAIN` only runs inside the `getSession` describe. To avoid leaking the env var, append a one-line cleanup at the end of the second `it`:

```ts
    delete process.env.ZITADEL_DOMAIN;
```

(or, if more isolation is preferred, lift the new describe under the existing `getSession` suite as a nested describe so its afterEach already runs).

- [ ] **Step 9.4: Wire the fetch into the OIDC callback**

In `ui/src/app/auth/callback/route.ts`:

Update the imports (line 4-13) to add `fetchProfileMetadata`:

```ts
import {
  buildCallbackUri,
  decodePkce,
  exchangeCodeForToken,
  extractSessionFields,
  fetchProfileMetadata,
  nameToAvatar,
  parseJwtClaims,
  PKCE_COOKIE_NAME,
} from "@/lib/oidc";
```

Replace the `payload` construction (lines 126-134) with a metadata fetch + extended payload:

```ts
  const backendUrl = process.env.BACKEND_URL || "http://backend:8080";
  const profile = await fetchProfileMetadata(tokenResponse.access_token, backendUrl);

  const payload: OidcSessionCookie = {
    type: "oidc",
    accessToken: tokenResponse.access_token,
    userId: fields.userId,
    role: fields.role,
    name: fields.name || nameToAvatar(fields.userId),
    email: fields.email,
    title: profile.title,
    team: profile.team,
    location: profile.location,
    status: profile.status,
    expiresAt: fields.expiresAt,
  };
```

- [ ] **Step 9.5: Run UI tests + lint + typecheck**

Run: `cd ui && bun run test src/lib && bun run lint && bun run build`
Expected: all PASS, build succeeds.

- [ ] **Step 9.6: Manual smoke (if a live Zitadel is available)**

```bash
cd ui && bun run dev
# In a private window, navigate to /, sign in via Zitadel, then visit /
# Expected: dashboard "Identity" card shows your Title and Team • Location.
```

If no live Zitadel, skip — automated tests cover the wiring. Mark this step manually verified or note "deferred to staging" in the commit message.

- [ ] **Step 9.7: Commit**

```bash
git add ui/src/lib/oidc.ts ui/src/lib/session.ts ui/src/lib/__tests__/session.test.ts ui/src/app/auth/callback/route.ts
git commit -m "ui: OIDC callback populates Title/Team/Location from /me/profile (audit C2/D5)

After token exchange, the callback fetches /api/v1/me/profile with the
fresh access token and embeds title/team/location/status into the
session cookie. getSession() returns these on the SessionUser so the
member dashboard renders identically for OIDC and demo sessions. Older
cookies missing the new fields decode to safe defaults rather than
failing validation."
```

---

## Task 10: Spec deltas + INDEX update + codebase-memory refresh

Close the OpenSpec loop and refresh the graph so subsequent waves search against current state.

**Files:**
- Create: `openspec/changes/wave-1-production-trust-hardening/specs/automation-policies/spec.md` (delta)
- Create: `openspec/changes/wave-1-production-trust-hardening/specs/production-security-boundary/spec.md` (delta)
- Create: `openspec/changes/wave-1-production-trust-hardening/specs/operational-readiness/spec.md` (delta)
- Modify: `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` (welcome-bundle row)
- Modify: `openspec/INDEX.md` (Change Log row, Phase 5.5)
- Modify: `openspec/changes/wave-1-production-trust-hardening/IMPLEMENTATION.md` (final state)

- [ ] **Step 10.1: Write the automation-policies delta**

Create `openspec/changes/wave-1-production-trust-hardening/specs/automation-policies/spec.md`:

```markdown
> **Status:** Wave 1 delta — explicit `is_welcome` flag replaces convention-based name match | [< Index](../../../../INDEX.md)

# Requirement: Welcome Bundle Configuration (delta)

## ADDED Requirements

### Schema-enforced single welcome bundle
The `bundles` table MUST carry an `is_welcome BOOLEAN NOT NULL DEFAULT FALSE` column with a partial unique index `(is_welcome) WHERE is_welcome = TRUE`. At most one bundle MAY be marked as the welcome bundle at any time.

### Explicit-only welcome resolution
`GetWelcomeBundle` MUST return an error (`db.ErrNoWelcomeBundleConfigured`) when no bundle has `is_welcome = TRUE`. Convention-based fallbacks (name match, "first bundle by created_at") MUST NOT be used.

### Operator-facing toggle
`PUT /api/v1/bundles/{id}/welcome` MUST clear any previously-flagged bundle and mark the named bundle as welcome in a single transaction. The bundle list UI MUST show a `Welcome` badge on the flagged bundle and expose a `Set as welcome bundle` action on every other bundle row.

### Audit trail
Every `bundle.welcome_set` action MUST emit an `audit_logs` entry attributed to the operator (or `system` in dev mode without `?actor=`).

## REMOVED behaviour

- Name-pattern match `LOWER(name) LIKE '%welcome%'` is removed.
- "First bundle by created_at" silent fallback is removed.

(Audit ref: D1)
```

- [ ] **Step 10.2: Write the production-security-boundary delta**

Create `openspec/changes/wave-1-production-trust-hardening/specs/production-security-boundary/spec.md`:

```markdown
> **Status:** Wave 1 delta — startup signing-key gate + vault dev-mode actor requirement | [< Index](../../../../INDEX.md)

# Requirement: Production Security Boundary (delta)

## ADDED Requirements

### Production refuses missing signing keys
When `ZITADEL_DOMAIN` is set, the backend MUST exit with a non-zero status during startup if either `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` is empty. The HTTP server MUST NOT bind a port until both keys are present.

### Runtime middleware refuses misconfigured production
`withZitadelActionSignature` MUST return `503 MISCONFIGURED` when the configured signing-key env is empty AND `ZITADEL_DOMAIN` is set. Dev-mode passthrough is allowed only when both are empty.

### Vault mutations require explicit actor in dev mode
`PUT /api/v1/users/{uid}/shadow-credential` and `DELETE /api/v1/users/{uid}/shadow-credential` MUST refuse with `400 MISSING_ACTOR` when no JWT actor is in the request context AND no `?actor=<id>` query parameter is supplied. The audit row MUST record the explicit actor, not the target user. Reads (`/status`, `/audit`) MUST continue to fall back to `{uid}` for the audit field; they MUST NOT require `?actor=`.

(Audit refs: C1, C3)
```

- [ ] **Step 10.3: Write the operational-readiness delta**

Create `openspec/changes/wave-1-production-trust-hardening/specs/operational-readiness/spec.md`:

```markdown
> **Status:** Wave 1 delta — OIDC dashboard renders Title/Team/Location | [< Index](../../../../INDEX.md)

# Requirement: Operator Dashboard Identity Rendering (delta)

## ADDED Requirements

### OIDC sessions populate Title/Team/Location identically to demo
The OIDC callback MUST fetch `/api/v1/me/profile` with the freshly-issued access token immediately after token exchange and embed `title`, `team`, `location`, and `status` into the session cookie. The member dashboard's Identity card MUST render these fields for every authenticated session, regardless of session type.

### Endpoint shape
`GET /api/v1/me/profile` MUST be gated by `withUserAuth`, MUST resolve the requester's user ID from the bearer-token actor, and MUST return the same `models.UserProfile` shape returned by `directory.Default.FindUser` — including the Zitadel metadata overlay.

### Robustness
A failed metadata fetch MUST NOT block session creation; affected fields render as empty strings until the operator updates Zitadel metadata.

(Audit refs: C2, D5)
```

- [ ] **Step 10.4: Update `feature-coverage.md`**

In `openspec/changes/syndra-core-architecture/specs/feature-coverage.md`, find the `automation-policies` row (search for `Welcome` or `convention`). Replace the "Notes" cell text "convention-based" with "explicit `is_welcome` flag, errors when not configured (Wave 1)". Update the Status column entry accordingly.

- [ ] **Step 10.5: Update `openspec/INDEX.md`**

In `openspec/INDEX.md`, in the Change Log table (the table starting around line 43), insert a new row for Wave 1:

```markdown
| [Wave 1 — Production Trust Hardening](changes/wave-1-production-trust-hardening/) | 5.5 | In progress | [proposal](changes/wave-1-production-trust-hardening/proposal.md) / [design](changes/wave-1-production-trust-hardening/design.md) / [tasks](changes/wave-1-production-trust-hardening/tasks.md) |
```

Place it after the Lifecycle Event Propagation row (Phase 5).

- [ ] **Step 10.6: Refresh the codebase memory graph**

Run via the `mcp__codebase-memory-mcp__detect_changes` tool with the project name `Users-notkanishk-Documents-Mkrspc-Projects-Syndra`. Verify the diff includes the new symbols (`SetWelcomeBundle`, `ErrNoWelcomeBundleConfigured`, `handleSetWelcomeBundle`, `handleGetMyProfile`, `fetchProfileMetadata`, `useSetWelcomeBundle`) and that the deleted `cmd/test/main.go` no longer appears.

If `detect_changes` does not auto-reindex, follow up with `index_repository` for the same project.

- [ ] **Step 10.7: Update `IMPLEMENTATION.md` with one-line entries per task**

In `openspec/changes/wave-1-production-trust-hardening/IMPLEMENTATION.md`, append:

```markdown
## Implementation log

- **Task 1 (B1)** — `cmd/test/main.go` deleted. Commit: <SHA>.
- **Task 2 (C1)** — Startup gate added in `cmd/api/main.go`. Commit: <SHA>.
- **Task 3 (C1)** — `withZitadelActionSignature` tightened. Commit: <SHA>.
- **Task 4 (D1)** — `is_welcome` migration + repo rewrite. Commit: <SHA>.
- **Task 5 (D1)** — Welcome handler + onboarding error path. Commit: <SHA>.
- **Task 6 (D1)** — Welcome-bundle UI toggle. Commit: <SHA>.
- **Task 7 (C3)** — Vault `?actor=` requirement. Commit: <SHA>.
- **Task 8 (C2/D5)** — `/me/profile` endpoint. Commit: <SHA>.
- **Task 9 (C2/D5)** — OIDC callback profile fetch. Commit: <SHA>.
- **Task 10** — Spec deltas + INDEX update + memory refresh.
```

Replace `<SHA>` with the actual commit SHAs as you complete each task.

- [ ] **Step 10.8: Final full-suite test sweep**

Run in parallel:
- `cd backend && go test ./... && go vet ./...`
- `cd ui && bun run test && bun run lint && bun run build`
- `cd sync && go test ./... && go vet ./...` (defensive — sync has no Wave 1 changes but ensures the cross-module surface still builds)

Expected: all green.

- [ ] **Step 10.9: Commit the spec deltas**

```bash
git add openspec/changes/wave-1-production-trust-hardening/specs/ openspec/changes/wave-1-production-trust-hardening/IMPLEMENTATION.md openspec/changes/syndra-core-architecture/specs/feature-coverage.md openspec/INDEX.md
git commit -m "openspec: wave-1 spec deltas + INDEX update

Adds three deltas (automation-policies, production-security-boundary,
operational-readiness) capturing the doctrinal shifts in Wave 1.
Welcome-bundle row in feature-coverage.md updated from convention-based
to explicit. INDEX gains the Wave 1 row under Phase 5.5."
```

- [ ] **Step 10.10: Verify the final tree state**

Run: `git status && git log --oneline -12`
Expected: clean working tree; the last 9-11 commits cover Tasks 1-10 in order.

---

## Self-Review Checklist (run before handing off)

Run through this checklist after the plan is committed and tasks are kicked off:

1. **Spec coverage:** Every audit ID in §3 Theme 1 of the meta-spec maps to at least one task above. Check the table in §10 of the meta-spec — B1, B2, ..., D1, ..., C1, C2, C3 are all covered (B2 belongs to Theme 2 and is intentionally NOT in this plan).

2. **Placeholder scan:** Search this file for `TBD`, `TODO`, `implement later`, `Similar to Task`, `<insert>`. None should remain in the steps.

3. **Type consistency:**
   - `db.ErrNoWelcomeBundleConfigured` referenced in Task 5 matches the definition in Task 4.
   - `SetWelcomeBundle(ctx, bundleID)` signature is the same in Tasks 4, 5, and 10.
   - `OidcSessionCookie` in Task 9 has all four new fields (`title`, `team`, `location`, `status`); the round-trip test references those same names.
   - `enforceSelfOnly(w, r, requireActor bool)` signature is consistent across Task 7 steps.
   - `directory.Default.FindUser` interface is the one used in Task 8 (returns `(UserProfile, bool, error)`).

If you hit a mismatch, fix it inline before starting execution.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-08-wave-1-production-trust-hardening.md`.** Two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration. The Wave-1 tasks are mostly independent; ten task-shaped agents should run cleanly in sequence (with optional parallelism between Tasks 1, 4, and 8 since they touch disjoint files).

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints. Pick this if you want to keep all decisions in one conversation.

**Which approach?**
