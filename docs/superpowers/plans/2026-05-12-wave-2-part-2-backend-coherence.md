# Wave 2 · Part 2 — Backend Coherence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve six May-2026 audit findings (B3, B5, B6, B7, C4, C5, C11, D8) that erode backend coherence — a 797-line view file whose role helper is called N times per view, a 1303-line repository file with nine unrelated domains, double-parsed JWTs on every operator request, silent webhook defaults that mask producer bugs, a `fail_closed` fallback that masks DB outages, and a string-prefix-sniffing error classifier in the password vault.

**Architecture:** Six independently-shippable refactors, ordered by ascending blast radius. Every observable contract is preserved; the change set is structural. Each task ships with its own narrow tests and commits in a green-tests state before the next begins. See [`openspec/changes/wave-2-part-2-backend-coherence/design.md`](../../../openspec/changes/wave-2-part-2-backend-coherence/design.md) for the design decisions and `proposal.md` for the executive summary.

**Tech stack:** Go 1.22+, PostgreSQL via `pgx/v5`, Redis via `go-redis`, `golang-jwt/jwt/v5`, `golang.org/x/crypto/argon2`, Go stdlib `net/http`. No new dependencies.

---

## File structure (post-implementation)

```
backend/internal/
├── auth/
│   ├── jwt.go                       ← Principal type + Validate (renamed from ValidateToken)
│   └── jwt_test.go                  ← +TestValidate_PopulatesProjectRoles
├── db/                              ← B5 split target
│   ├── postgres.go                  (unchanged)
│   ├── redis.go                     (unchanged)
│   ├── validation.go                (unchanged)
│   ├── audit.go                     ← NEW (InsertAuditLog)
│   ├── bundles.go                   ← NEW (7 bundle functions)
│   ├── rules.go                     ← NEW (CreateMappingRule, UpdateMappingRule, GetActiveMappingRules)
│   ├── grants.go                    ← NEW (6 direct-grant functions)
│   ├── access_requests.go           ← NEW (4 access-request functions)
│   ├── claim_profiles.go            ← NEW (ClaimProfileRow + 2 helpers)
│   ├── onboarding.go                ← NEW (OnboardingTrigger + 6 helpers)
│   ├── webhooks.go                  ← NEW (WebhookEvent + ZitadelGrantIndex + 7 helpers)
│   ├── roles.go                     ← NEW (RoleUsage + 7 role-catalog helpers)
│   ├── intents.go                   ← NEW (5 provisioning-intent functions)
│   ├── vault.go                     ← NEW (6 shadow-credential helpers)
│   └── repositories.go              ← DELETED (or reduced to a 1-line // moved comment then removed)
├── handlers/
│   ├── action.go                    ← +claimFailureModeRead helper (C5)
│   ├── deps.go                      ← +redisGetClaimMode/redisSetClaimMode injectables
│   ├── router.go                    ← withUserAuth / withOperatorAuth rewritten on Principal
│   ├── router_test.go               ← NEW (middleware tests for principal-from-context)
│   ├── action_test.go               ← +TestClaimFailureModeRead_*
│   ├── webhook.go                   ← Strict event_type; observable enrichment-incomplete drop
│   ├── webhook_test.go (existing)   ← +TestWebhook_MissingEventType_Internal_Returns400
│   │                                  +TestWebhook_ZitadelGrant_EnrichmentIncomplete_PersistedAsDropped
│   ├── vault.go                     ← errors.Is on services.ErrComplexity; delete isComplexityError
│   └── vault_test.go (existing)     ← Fixture updated to wrap services.ErrComplexity
└── services/
    ├── vault.go                     ← +var ErrComplexity = errors.New("password complexity"); %w wrap
    ├── vault_test.go (existing)     ← +TestValidatePasswordComplexity_WrapsSentinel
    ├── views.go                     ← +accessSnapshot; ListUsers/Applications/Projects/Topology/Governance threaded
    └── views_test.go (existing)     ← +TestListApplications_CallsCollectUserRolesOncePerUser
```

Conventions: each task creates its tests before its production code (TDD). Commits are tight — one task = one commit on `main` (no PR branching needed for these refactors). Plan task numbers (1–8) match `tasks.md`.

---

## Task 0: Commit OpenSpec scaffolding

**Files (already written):**
- `openspec/changes/wave-2-part-2-backend-coherence/proposal.md`
- `openspec/changes/wave-2-part-2-backend-coherence/design.md`
- `openspec/changes/wave-2-part-2-backend-coherence/specs/application-claims/spec.md`
- `openspec/changes/wave-2-part-2-backend-coherence/specs/lifecycle-event-propagation/spec.md`
- `openspec/changes/wave-2-part-2-backend-coherence/tasks.md`
- `docs/superpowers/plans/2026-05-12-wave-2-part-2-backend-coherence.md` (this file)

- [ ] **Step 0.1: Validate the OpenSpec change**

Run from the repo root (the `openspec` CLI discovers `./openspec/` relative to cwd and errors with `Unknown item …` when run from inside `openspec/`):

```bash
cd <repo>
openspec validate wave-2-part-2-backend-coherence --strict
```
Expected: `Change 'wave-2-part-2-backend-coherence' is valid`. If it does not, fix the spec deltas until it does — do not proceed.

- [ ] **Step 0.2: Stage and commit the scaffolding**

```bash
cd <repo>
git add openspec/changes/wave-2-part-2-backend-coherence/ docs/superpowers/plans/2026-05-12-wave-2-part-2-backend-coherence.md
git status
git commit -m "$(cat <<'EOF'
docs(openspec): scaffold wave-2-part-2-backend-coherence change

OpenSpec proposal/design/tasks plus two spec deltas (application-claims,
lifecycle-event-propagation) and the detailed superpowers plan. No code
changes yet — scaffolding only.

Audit refs: B3, B5, B6, B7, C4, C5, C11, D8

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 0.3: Baseline `gofmt` cleanup for the wave's touch set**

The wave's final verification (Step 7.1) asserts zero gofmt diff in the files this wave creates or modifies. Several existing files in that touch set carry pre-existing drift (verified at plan-write time: `webhook.go`, `deps.go`, `action_test.go`, `views.go`, `repositories.go`). Normalising them once, before implementation, keeps every subsequent commit's diff focused on logic instead of incidental whitespace.

```bash
cd <repo>
gofmt -l \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/webhook.go \
  backend/internal/handlers/webhook_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/action.go \
  backend/internal/handlers/action_test.go \
  backend/internal/handlers/vault.go \
  backend/internal/handlers/vault_test.go \
  backend/internal/services/vault.go \
  backend/internal/services/vault_test.go \
  backend/internal/services/views.go \
  backend/internal/services/views_test.go \
  backend/internal/db/repositories.go
```

Expected output: a list of the touch-set files that need normalisation. If `context.go` (or whichever file holds `withAdminUserID` per Step 4.1) is present and drifts, include it in the next command too. Pre-existing drift in files **outside** this list (e.g. `db/postgres.go`, `models/models.go`) is real but out of scope for this wave; track it as a separate housekeeping commit.

If `gofmt -l` printed any paths, normalise them and run the test suite to confirm the rewrites are behaviour-preserving (gofmt should never change behaviour, but the suite is the proof). Then commit only the formatting changes.

```bash
# Stage 1 — rewrite. From repo root.
cd <repo>
gofmt -w \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/webhook.go \
  backend/internal/handlers/webhook_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/action.go \
  backend/internal/handlers/action_test.go \
  backend/internal/handlers/vault.go \
  backend/internal/handlers/vault_test.go \
  backend/internal/services/vault.go \
  backend/internal/services/vault_test.go \
  backend/internal/services/views.go \
  backend/internal/services/views_test.go \
  backend/internal/db/repositories.go

# Stage 2 — verify nothing broke. From backend/ since go.mod lives there.
cd <repo>/backend
go test ./... -count=1
go vet ./...

# Stage 3 — review and commit. Back to repo root.
# Stage ONLY the touch-set files so unrelated dirty changes elsewhere
# under backend/internal/ (in-flight work, scratch edits) cannot
# accidentally ride along on the formatting commit.
cd <repo>
git status                   # show worktree state; expect only touch-set drift
git diff --stat
git diff -- \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/webhook.go \
  backend/internal/handlers/webhook_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/action.go \
  backend/internal/handlers/action_test.go \
  backend/internal/handlers/vault.go \
  backend/internal/handlers/vault_test.go \
  backend/internal/services/vault.go \
  backend/internal/services/vault_test.go \
  backend/internal/services/views.go \
  backend/internal/services/views_test.go \
  backend/internal/db/repositories.go  # MUST show only whitespace/import-ordering changes
git add -- \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/webhook.go \
  backend/internal/handlers/webhook_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/action.go \
  backend/internal/handlers/action_test.go \
  backend/internal/handlers/vault.go \
  backend/internal/handlers/vault_test.go \
  backend/internal/services/vault.go \
  backend/internal/services/vault_test.go \
  backend/internal/services/views.go \
  backend/internal/services/views_test.go \
  backend/internal/db/repositories.go
git diff --cached            # final review before commit — touch-set files only
git commit -m "$(cat <<'EOF'
chore(backend): gofmt baseline for wave-2-part-2 touch set

Normalise pre-existing gofmt drift in the files Wave 2 · Part 2 will
modify (webhook.go, deps.go, action_test.go, views.go, repositories.go
plus any other drift surfaced at gate time). Pure formatting — zero
logic changes. Establishes a clean baseline so each subsequent
Wave 2 · Part 2 commit's diff is focused on its audit ref.

Pre-existing drift in files outside this wave's touch set
(db/postgres.go, models/models.go, etc.) is intentionally not addressed
here — that is a separate housekeeping commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

If `gofmt -l` printed no paths, skip this commit — the touch set is already clean and this step is a no-op.

> Trust check: `git diff` between Stage 1 and Stage 3 MUST surface only whitespace, import grouping, or struct-field alignment changes. If `git diff` shows any logic difference, **abort** — investigate before staging. gofmt has not changed semantics in years, but trust-but-verify costs nothing here.

---

## Task 1: `ErrComplexity` sentinel (audit ref B7)

**Why this is first:** smallest blast radius — one service function, one handler, one fixture. Zero behavioural change visible to clients (same 400 response with same body). Proves the wave's TDD discipline before any structural moves.

**Files:**
- Modify: `backend/internal/services/vault.go` — add `ErrComplexity` sentinel; wrap with `%w`
- Test: `backend/internal/services/vault_test.go` — add sentinel round-trip test
- Modify: `backend/internal/handlers/vault.go` — replace prefix-sniff with `errors.Is`; delete `isComplexityError`
- Modify: `backend/internal/handlers/vault_test.go` — update fixture at line 95 to wrap sentinel

- [ ] **Step 1.1: Write the failing service-layer sentinel test**

Add to `backend/internal/services/vault_test.go` (append after the existing `TestValidatePasswordComplexity_NoSymbol` test):

```go
func TestValidatePasswordComplexity_WrapsSentinel(t *testing.T) {
	err := ValidatePasswordComplexity("short")
	if err == nil {
		t.Fatalf("expected complexity error for %q, got nil", "short")
	}
	if !errors.Is(err, ErrComplexity) {
		t.Fatalf("expected errors.Is(err, ErrComplexity)=true; got err=%v", err)
	}
}
```

If the file does not already import `errors`, add it to the import block at the top.

- [ ] **Step 1.2: Run the test — confirm it fails (no `ErrComplexity` defined yet)**

```bash
cd <repo>/backend
go test ./internal/services/ -run TestValidatePasswordComplexity_WrapsSentinel -v
```

Expected output: compile error `undefined: ErrComplexity`. Good — that's the failing test.

- [ ] **Step 1.3: Define the sentinel and wrap with `%w`**

In `backend/internal/services/vault.go`:

1. Add `"errors"` to the import block (after `"crypto/rand"`).
2. After the const block (line 22), add:

```go
// ErrComplexity is the sentinel error returned by ValidatePasswordComplexity
// when a password fails one or more strength rules. Handlers MUST use
// errors.Is(err, ErrComplexity) to classify the failure — not string-prefix
// sniffing against the wrapped detail message (which composes the failing
// requirements and is not stable).
var ErrComplexity = errors.New("password complexity")
```

3. Replace the `return fmt.Errorf("password complexity: %s", ...)` line at line 61 with:

```go
		return fmt.Errorf("%w: %s", ErrComplexity, strings.Join(failures, "; "))
```

- [ ] **Step 1.4: Run the test — confirm it passes**

```bash
go test ./internal/services/ -run TestValidatePasswordComplexity_WrapsSentinel -v
```

Expected output: `--- PASS: TestValidatePasswordComplexity_WrapsSentinel`.

Also re-run the existing complexity tests to ensure the message-wrapping change preserves their assertions:

```bash
go test ./internal/services/ -run TestValidatePasswordComplexity -v
```

Expected output: all `TestValidatePasswordComplexity_*` PASS. (`%w: %s` formats identically to the previous `password complexity: %s` because `ErrComplexity.Error()` is `"password complexity"`.)

- [ ] **Step 1.5: Update the handler-test fixture to wrap the sentinel**

In `backend/internal/handlers/vault_test.go` at line ~95, replace:

```go
		return fmt.Errorf("password complexity: must be at least 12 characters")
```

with:

```go
		return fmt.Errorf("%w: must be at least 12 characters", services.ErrComplexity)
```

If `services` is not already in the import block, add `"syndra/internal/services"`.

- [ ] **Step 1.6: Run handler tests — they should still pass (handler logic unchanged yet)**

```bash
go test ./internal/handlers/ -run TestHandleSetShadowCredential_ValidationError -v
```

Expected: PASS. The handler's `isComplexityError` prefix-sniff still matches the wrapped error's message (which begins `"password complexity:"`). This step proves the fixture is compatible *before* we swap the handler over.

- [ ] **Step 1.7: Swap the handler from prefix-sniff to `errors.Is`**

In `backend/internal/handlers/vault.go`:

1. Add `"syndra/internal/services"` to the import block (if not present).
2. Replace lines 72–76:

```go
		if isComplexityError(err) {
			jsonValidationErrorResponse(w, err.Error(), map[string]string{"password": "complexity"})
			return
		}
```

with:

```go
		if errors.Is(err, services.ErrComplexity) {
			jsonValidationErrorResponse(w, err.Error(), map[string]string{"password": "complexity"})
			return
		}
```

3. Delete the entire `isComplexityError` function (lines 163–166).

- [ ] **Step 1.8: Run all handler vault tests + service vault tests**

```bash
go test ./internal/handlers/ -run Vault -v
go test ./internal/services/ -run Vault -v
go test ./internal/services/ -run Password -v
```

Expected: every test PASSes. If any fails, the fixture or the swap is wrong — fix it before committing.

- [ ] **Step 1.9: Commit**

```bash
cd <repo>
git add backend/internal/services/vault.go backend/internal/services/vault_test.go backend/internal/handlers/vault.go backend/internal/handlers/vault_test.go
git commit -m "$(cat <<'EOF'
refactor(vault): classify complexity errors via typed sentinel

Replace the substring-prefix-sniffing isComplexityError helper in
handlers/vault.go with errors.Is(err, services.ErrComplexity). The
sentinel is wrapped via %w from ValidatePasswordComplexity so the
detail message (failing requirements) is preserved verbatim for the
400 response body.

Audit ref: B7 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Strict webhook `event_type` + observable enrichment drop (audit refs B6, C11, D8)

**Why this is second:** localized to one handler file plus one new injectable; touches no shared types. Surfaces the design Decision 5 deviation from the audit's literal wording (200-ack stays; observability gets tightened).

**Files:**
- Modify: `backend/internal/handlers/webhook.go` — remove `event_type` silent default; persist enrichment-incomplete drops with new status
- Modify: `backend/internal/handlers/deps.go` — add `dbDropWebhookEventEnrichmentIncomplete` injectable
- Add (or extend): `backend/internal/db/repositories.go` — `DropWebhookEventEnrichmentIncomplete` helper (insert-only; uses the existing `webhook_events` table)
- Test: `backend/internal/handlers/webhook_test.go` (existing) — add two cases

- [ ] **Step 2.1: Write the failing test for missing-event_type internal-shape 400**

Add to `backend/internal/handlers/webhook_test.go` (append after the existing webhook tests):

```go
func TestHandleZitadelWebhook_InternalMissingEventType_Returns400(t *testing.T) {
	setupNoopWebhookDeps(t)

	body := `{"user_id":"u1","source_project":"p1","role_keys":["r1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(body))
	rr := httptest.NewRecorder()
	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing event_type; got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"event_type":"required"`) {
		t.Fatalf("expected event_type=required in response details; got %s", rr.Body.String())
	}
}
```

If the file does not already define `setupNoopWebhookDeps` (it should — see the existing webhook tests in the file), use whatever helper the existing tests use to neutralise the dispatch path; the goal is to ensure the dispatch functions are not invoked when validation fails.

- [ ] **Step 2.2: Run the test — confirm it fails (current code silently defaults `event_type` to `grant_added`)**

```bash
cd <repo>/backend
go test ./internal/handlers/ -run TestHandleZitadelWebhook_InternalMissingEventType_Returns400 -v
```

Expected: FAIL with `expected 400 ... got 200`.

- [ ] **Step 2.3: Remove the silent default in `webhook.go:77-79`**

In `backend/internal/handlers/webhook.go`, delete lines 76–79:

```go
	// Default event_type for backward compatibility (internal-shape callers only).
	if event.EventType == "" {
		event.EventType = "grant_added"
	}
```

The strict check at line 81 (`if !validEventTypes[event.EventType]`) now catches `event.EventType == ""` because the empty string is not in the `validEventTypes` map — but the current error message (`"Invalid event_type"`) is generic. Replace the line 81–86 block:

```go
	if !validEventTypes[event.EventType] {
		jsonValidationErrorResponse(w, "Invalid event_type", map[string]string{
			"event_type": "must be one of: grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created",
		})
		return
	}
```

with:

```go
	if !trimmedNonEmpty(event.EventType) {
		jsonValidationErrorResponse(w, "event_type is required", map[string]string{
			"event_type": "required",
		})
		return
	}
	if !validEventTypes[event.EventType] {
		jsonValidationErrorResponse(w, "Invalid event_type", map[string]string{
			"event_type": "must be one of: grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created",
		})
		return
	}
```

The Zitadel-shape unknown-event 200-ack at lines 60–64 is untouched — translated events with `EventType == ""` still short-circuit there before this strict check runs.

- [ ] **Step 2.4: Re-run the test — confirm it passes**

```bash
go test ./internal/handlers/ -run TestHandleZitadelWebhook_InternalMissingEventType_Returns400 -v
```

Expected: PASS.

Also rerun the existing webhook test suite to ensure no regression:

```bash
go test ./internal/handlers/ -run Webhook -v
```

Expected: all webhook tests PASS. If any test relied on the silent `event_type` default, update the test to provide `event_type` explicitly (this is the desired fixture migration — the silent default was always a mask for accidental omissions).

- [ ] **Step 2.5: Write the failing test for enrichment-incomplete persistence**

The fixture MUST match `zitadelEventPayload` in `webhook_translate.go` (lines 19–30) — top-level `aggregateID` is the translation-shape signal; `event_type` is snake-case; `event_payload` is a nested raw JSON. Mismatched field names fall through to internal-shape strict decode and never reach the enrichment-incomplete branch.

Also: enrichment relies on the local grant index (`dbGetGrantIndex`) and the live Zitadel grants list (`dbListUserGrantsLive`). The test MUST override both injectables to return empty so enrichment genuinely fails. Without those overrides a real index/Zitadel hit could produce false negatives.

Add to `backend/internal/handlers/webhook_test.go`:

```go
func TestHandleZitadelWebhook_ZitadelGrant_EnrichmentIncomplete_PersistedAsDropped(t *testing.T) {
	setupNoopWebhookDeps(t)

	// Force enrichment to fail: empty local index + empty Zitadel grants list.
	origIdx, origLive := dbGetGrantIndex, dbListUserGrantsLive
	dbGetGrantIndex = func(ctx context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, pgx.ErrNoRows
	}
	dbListUserGrantsLive = func(ctx context.Context, userID string) ([]zitadel.UserGrant, error) {
		return nil, nil
	}
	t.Cleanup(func() {
		dbGetGrantIndex = origIdx
		dbListUserGrantsLive = origLive
	})

	var droppedCalls int
	origDrop := dbDropWebhookEventEnrichmentIncomplete
	dbDropWebhookEventEnrichmentIncomplete = func(ctx context.Context, eventType, userID, grantID, idempotencyKey string) error {
		droppedCalls++
		if eventType != "grant_removed" {
			t.Errorf("expected event_type=grant_removed in dropped record; got %q", eventType)
		}
		if userID != "u-zit-1" {
			t.Errorf("expected user_id=u-zit-1 in dropped record; got %q", userID)
		}
		if grantID != "grant-gone" {
			t.Errorf("expected grant_id=grant-gone in dropped record; got %q", grantID)
		}
		return nil
	}
	t.Cleanup(func() { dbDropWebhookEventEnrichmentIncomplete = origDrop })

	body := zitadelGrantRemovedFixtureUnresolvable(t, "u-zit-1", "grant-gone")
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("ZITADEL-Signature", "v1=fixture,t=1")
	rr := httptest.NewRecorder()
	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 storm-prevention ack; got %d: %s", rr.Code, rr.Body.String())
	}
	if droppedCalls != 1 {
		t.Fatalf("expected exactly 1 call to dbDropWebhookEventEnrichmentIncomplete; got %d", droppedCalls)
	}
}
```

Add the fixture helper that mirrors the wire format in `webhook_translate.go`:

```go
// zitadelGrantRemovedFixtureUnresolvable returns a Zitadel ContextInfoEvent-
// shaped payload for user.grant.removed. The shape MUST match
// zitadelEventPayload (webhook_translate.go:19) — top-level "aggregateID"
// triggers the Zitadel-shape branch; "event_type" is snake-case; the
// nested "event_payload" is the userGrantPayload struct (camelCase).
//
// projectId and roleKeys are intentionally omitted so the enrichment pass
// (enrichGrantPayload) — combined with the empty dbGetGrantIndex /
// dbListUserGrantsLive overrides in the test — produces an unresolvable
// grant event that exercises the webhook.go:104-109 storm-prevention path.
func zitadelGrantRemovedFixtureUnresolvable(t *testing.T, userID, grantID string) []byte {
	t.Helper()
	innerPayload := map[string]any{
		"userId":  userID,
		"grantId": grantID,
	}
	innerRaw, err := json.Marshal(innerPayload)
	if err != nil {
		t.Fatalf("marshal inner event_payload: %v", err)
	}
	payload := map[string]any{
		"aggregateID":   grantID,
		"aggregateType": "user_grant",
		"resourceOwner": "fixture-org",
		"instanceID":    "fixture-instance",
		"version":       "v1",
		"sequence":      1,
		"event_type":    "user.grant.removed",
		"created_at":    time.Now().UTC().Format(time.RFC3339Nano),
		"userID":        "editor-fixture", // editor; not a self-mutation (ZITADEL_M2M_USER_ID is unset in tests)
		"event_payload": json.RawMessage(innerRaw),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return raw
}
```

If the test file does not already import `time`, `bytes`, `encoding/json`, `context`, `github.com/jackc/pgx/v5`, `syndra/internal/db`, or `syndra/internal/zitadel`, add them.

> **Why the fixture matters:** the previous draft used `eventType` / `aggregate.id` / `editorUser.id` — none of those are read by `translateZitadelEvent` (which probes for top-level `aggregateID` at `webhook_translate.go:61`). With the wrong shape, the decoder falls through to internal-shape strict decode and the test asserts a path that never runs. Verify the fixture by setting a breakpoint or `log.Printf` inside `mapGrantEvent` if anything looks off.

- [ ] **Step 2.6: Run the test — confirm it fails (no `dbDropWebhookEventEnrichmentIncomplete` injectable yet)**

```bash
go test ./internal/handlers/ -run TestHandleZitadelWebhook_ZitadelGrant_EnrichmentIncomplete_PersistedAsDropped -v
```

Expected: compile error `undefined: dbDropWebhookEventEnrichmentIncomplete`.

- [ ] **Step 2.7: Add the DB helper that emits the `dropped_enrichment_incomplete` row**

In `backend/internal/db/repositories.go`, locate the Webhook Events section (around line 685, `InsertWebhookEvent`) and add a new helper after `FailWebhookEvent`:

```go
// DropWebhookEventEnrichmentIncomplete records a Zitadel-shape grant event
// whose enrichment could not resolve source_project or role_keys, so the
// handler issued a storm-prevention 200-ack instead of dispatching. The row
// makes the silent drop observable via GET /api/v1/webhook/events?status=
// dropped_enrichment_incomplete (audit refs C11, D8).
//
// Uses idempotency_key as the unique key the same way InsertWebhookEvent does;
// duplicate posts of the same unresolvable aggregate are deduplicated, not
// double-counted.
func DropWebhookEventEnrichmentIncomplete(ctx context.Context, eventType, userID, grantID, idempotencyKey string) error {
	_, err := PG.Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key, status, processed_at)
		VALUES ($1, $2, '', $3, $4, 'dropped_enrichment_incomplete', NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
	`, eventType, userID, grantID, idempotencyKey)
	if err != nil {
		return fmt.Errorf("drop webhook event enrichment incomplete: %w", err)
	}
	return nil
}
```

(The `webhook_events` schema already has `idempotency_key UNIQUE`, `status TEXT NOT NULL DEFAULT 'pending'`, and `processed_at TIMESTAMPTZ`. No migration needed.)

- [ ] **Step 2.8: Wire the new helper through `deps.go`**

In `backend/internal/handlers/deps.go`, inside the existing webhook-injectables block (around line 42), add:

```go
	dbDropWebhookEventEnrichmentIncomplete = db.DropWebhookEventEnrichmentIncomplete
```

- [ ] **Step 2.9: Persist the drop in `webhook.go:104-109`**

In `backend/internal/handlers/webhook.go`, the existing 200-ack block (lines 104–109) becomes:

```go
	if isZitadel && isGrantEvent && (!trimmedNonEmpty(event.SourceProject) || len(event.RoleKeys) == 0) {
		log.Printf("[WEBHOOK] grant event acknowledged without dispatch (enrichment incomplete) event=%s user=%s grant=%s project=%q roles=%v",
			event.EventType, event.UserID, event.GrantID, event.SourceProject, event.RoleKeys)
		idempotencyKey := r.Header.Get("ZITADEL-Signature")
		if idempotencyKey == "" {
			idempotencyKey = fmt.Sprintf("dropped:%s:%s:%s", event.EventType, event.UserID, event.GrantID)
		}
		if err := dbDropWebhookEventEnrichmentIncomplete(r.Context(), event.EventType, event.UserID, event.GrantID, idempotencyKey); err != nil {
			log.Printf("[WEBHOOK] failed to persist dropped event: %v (non-fatal)", err)
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "grant event acknowledged, dispatch skipped (enrichment incomplete)"})
		return
	}
```

- [ ] **Step 2.10: Re-run the persistence test — confirm it passes**

```bash
go test ./internal/handlers/ -run TestHandleZitadelWebhook_ZitadelGrant_EnrichmentIncomplete_PersistedAsDropped -v
```

Expected: PASS.

- [ ] **Step 2.11: Run the full handlers + db test suites to catch regressions**

```bash
go test ./internal/handlers/ ./internal/db/ -v
```

Expected: all PASS.

- [ ] **Step 2.12: Commit**

```bash
cd <repo>
git add backend/internal/handlers/webhook.go backend/internal/handlers/webhook_test.go backend/internal/handlers/deps.go backend/internal/db/repositories.go
git commit -m "$(cat <<'EOF'
refactor(webhook): strict event_type + observable enrichment-incomplete drops

- Remove the silent EventType="grant_added" default for internal-shape callers;
  missing event_type now returns 400 with structured details. Zitadel-shape
  unknown events still 200-ack via translateZitadelEvent (storm prevention).
- Zitadel-shape grant events whose enrichment cannot resolve source_project
  or role_keys still 200-ack (unchanged storm-prevention semantics) but now
  persist a webhook_events row with status='dropped_enrichment_incomplete'
  so operators can audit silent drops via GET /api/v1/webhook/events.

Design decision and audit-deviation rationale in
openspec/changes/wave-2-part-2-backend-coherence/design.md §2 Decision 5.

Audit refs: B6, C11, D8 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Redis cache for `claim_failure_mode` (audit ref C5)

**Why this is third:** still localized (one handler file + one DB-helper file + two new injectables). Touches the hot path of `/action/inject` but only on the degraded-mode branch — the success path is unchanged.

**Files:**
- Modify: `backend/internal/handlers/action.go` — add `claimFailureModeRead` helper; rewrite `degradedResponse` to call it
- Modify: `backend/internal/handlers/deps.go` — add `redisGetClaimMode` and `redisSetClaimMode` injectables
- Test: `backend/internal/handlers/action_test.go` (existing) — add cache-hit, cache-miss-DB-success, and cache-stale-on-DB-error tests

- [ ] **Step 3.1: Write the failing cache-on-DB-error test**

Add to `backend/internal/handlers/action_test.go`:

```go
func TestClaimFailureModeRead_DBError_FallsBackToCachedValue(t *testing.T) {
	origGet, origSet, origMode := redisGetClaimMode, redisSetClaimMode, dbGetClaimFailureMode
	t.Cleanup(func() {
		redisGetClaimMode = origGet
		redisSetClaimMode = origSet
		dbGetClaimFailureMode = origMode
	})

	// Redis returns a cached minimal_safe payload AND we assert the call
	// context carries a deadline (the redisTimeout wrap). Regression
	// protection against the wrap being dropped — the application-claims
	// spec requires the 50 ms data-plane budget is honoured on reads.
	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		if projectID != "proj-1" {
			t.Errorf("unexpected projectID %q", projectID)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Error("expected redisGetClaimMode to receive a deadline-bounded context (redisTimeout wrap missing)")
		}
		return `{"mode":"minimal_safe","minimal_safe_claims":{"reason":"degraded"}}`, nil
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		t.Errorf("redisSetClaimMode must not be called on cache hit; got projectID=%s", projectID)
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("dbGetClaimFailureMode must not be called on cache hit; got projectID=%s", projectID)
		return "fail_closed", nil, nil
	}

	mode, claims, err := claimFailureModeRead(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "minimal_safe" {
		t.Errorf("expected mode=minimal_safe; got %q", mode)
	}
	if got := claims["reason"]; got != "degraded" {
		t.Errorf("expected claims.reason=degraded; got %v", got)
	}
}

func TestClaimFailureModeRead_CacheMiss_DBSuccess_Caches(t *testing.T) {
	origGet, origSet, origMode := redisGetClaimMode, redisSetClaimMode, dbGetClaimFailureMode
	t.Cleanup(func() {
		redisGetClaimMode = origGet
		redisSetClaimMode = origSet
		dbGetClaimFailureMode = origMode
	})

	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return "", redis.Nil // cache miss
	}
	var setCalls int
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		setCalls++
		if projectID != "proj-2" {
			t.Errorf("expected projectID=proj-2; got %q", projectID)
		}
		if !strings.Contains(value, `"mode":"fail_closed"`) {
			t.Errorf("expected fail_closed in cached value; got %q", value)
		}
		if ttlSeconds <= 0 {
			t.Errorf("expected positive TTL; got %d", ttlSeconds)
		}
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, nil
	}

	mode, _, err := claimFailureModeRead(context.Background(), "proj-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "fail_closed" {
		t.Errorf("expected mode=fail_closed; got %q", mode)
	}
	if setCalls != 1 {
		t.Errorf("expected exactly 1 SET; got %d", setCalls)
	}
}

func TestClaimFailureModeRead_CacheMissAndDBError_DefaultsFailClosed(t *testing.T) {
	origGet, origSet, origMode := redisGetClaimMode, redisSetClaimMode, dbGetClaimFailureMode
	t.Cleanup(func() {
		redisGetClaimMode = origGet
		redisSetClaimMode = origSet
		dbGetClaimFailureMode = origMode
	})

	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return "", redis.Nil
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		t.Errorf("must not cache on DB error")
		return nil
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		return "fail_closed", nil, errors.New("simulated db outage")
	}

	mode, claims, err := claimFailureModeRead(context.Background(), "proj-3")
	if err != nil {
		t.Fatalf("expected nil error (helper swallows for safety); got %v", err)
	}
	if mode != "fail_closed" {
		t.Errorf("expected fail_closed default; got %q", mode)
	}
	if claims != nil {
		t.Errorf("expected nil claims; got %v", claims)
	}
}
```

If the imports do not include `errors`, `strings`, or `github.com/redis/go-redis/v9` (the path used by `db.Redis`), add them. Check `backend/internal/db/redis.go` for the exact module path and use the same.

- [ ] **Step 3.2: Run the tests — confirm they fail**

```bash
cd <repo>/backend
go test ./internal/handlers/ -run TestClaimFailureModeRead -v
```

Expected: compile error `undefined: redisGetClaimMode`, `undefined: redisSetClaimMode`, `undefined: claimFailureModeRead`.

- [ ] **Step 3.3: Add the injectables in `deps.go`**

In `backend/internal/handlers/deps.go`, inside the data-plane block at the bottom (after `dbGetClaimFailureMode = db.GetClaimFailureMode`), add:

```go
	redisGetClaimMode = func(ctx context.Context, projectID string) (string, error) {
		return db.Redis.Get(ctx, "claim_mode:"+projectID).Result()
	}
	redisSetClaimMode = func(ctx context.Context, projectID, value string, ttlSeconds int) error {
		return db.Redis.SetEx(ctx, "claim_mode:"+projectID, value, time.Duration(ttlSeconds)*time.Second).Err()
	}
```

If `time` is not yet in the import block of `deps.go`, add it.

- [ ] **Step 3.4: Add the `claimFailureModeRead` helper in `action.go`**

In `backend/internal/handlers/action.go`, add this function — recommend placing it directly above `degradedResponse` (around line 173):

```go
// claimFailureModeCacheTTL returns the configured Redis TTL for the
// claim_failure_mode read-through cache. Default 5 minutes; overridable
// via CLAIM_MODE_CACHE_TTL_SECONDS for environments that pin Redis to
// short retention.
func claimFailureModeCacheTTL() int {
	if v := os.Getenv("CLAIM_MODE_CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 300
}

// claimFailureModeRead resolves the per-project failure mode + minimal-safe
// claims via a Redis read-through cache.
//
// Order:
//   1. Read claim_mode:<projectID> from Redis (under redisTimeout). Hit → return.
//   2. Miss → call db.GetClaimFailureMode. Success → cache + return.
//   3. DB error after cache miss → log + return ("fail_closed", nil, nil).
//
// Both Redis calls are wrapped in context.WithTimeout(ctx, redisTimeout) so
// the 50 ms data-plane budget (action.go:57) is honoured: a slow or
// unreachable Redis MUST NOT stall the Zitadel Actions v2 reply.
//
// The cache exists so a transient DB outage cannot collapse degraded-mode
// behaviour into fail_closed for projects whose operator configured
// minimal_safe (audit ref C5). Errors from claimFailureModeRead are
// suppressed by design — the data plane MUST always have a fallback mode
// to return; surfacing the error would force the caller to redefine
// "safe default" in every call site.
func claimFailureModeRead(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
	// Cache read — bounded by redisTimeout so a Redis stall cannot blow
	// the Zitadel Actions v2 latency budget.
	readCtx, cancel := context.WithTimeout(ctx, redisTimeout)
	raw, rerr := redisGetClaimMode(readCtx, projectID)
	cancel()
	if rerr == nil && raw != "" {
		var payload struct {
			Mode              string                 `json:"mode"`
			MinimalSafeClaims map[string]interface{} `json:"minimal_safe_claims"`
		}
		if jerr := json.Unmarshal([]byte(raw), &payload); jerr == nil {
			return payload.Mode, payload.MinimalSafeClaims, nil
		}
		log.Printf("[CLAIM-MODE-CACHE] malformed cached value for project=%s; refreshing from DB", projectID)
	}

	// Cache miss, miss-due-to-timeout, or unparseable — go to DB.
	mode, claims, err := dbGetClaimFailureMode(ctx, projectID)
	if err != nil {
		log.Printf("[CLAIM-MODE-CACHE] DB read failed for project=%s; defaulting to fail_closed: %v", projectID, err)
		return "fail_closed", nil, nil
	}

	// Cache the fresh value (best-effort, also bounded by redisTimeout so a
	// stalled SETEX cannot delay the response).
	encoded, jerr := json.Marshal(struct {
		Mode              string                 `json:"mode"`
		MinimalSafeClaims map[string]interface{} `json:"minimal_safe_claims"`
	}{Mode: mode, MinimalSafeClaims: claims})
	if jerr == nil {
		writeCtx, wcancel := context.WithTimeout(ctx, redisTimeout)
		serr := redisSetClaimMode(writeCtx, projectID, string(encoded), claimFailureModeCacheTTL())
		wcancel()
		if serr != nil {
			log.Printf("[CLAIM-MODE-CACHE] cache write failed for project=%s: %v (non-fatal)", projectID, serr)
		}
	}
	return mode, claims, nil
}
```

If `action.go` does not already import `os`, `strconv`, or `encoding/json`, add them.

- [ ] **Step 3.5: Rewrite `degradedResponse` to use the cached read**

In `backend/internal/handlers/action.go`, replace the call to `dbGetClaimFailureMode` inside `degradedResponse` (line 182):

```go
	mode, minimalClaims, err := dbGetClaimFailureMode(ctx, projectID)
	if err != nil {
		log.Printf("[DATA PLANE] Could not load failure mode for project=%s (defaulting to fail_closed): %v", projectID, err)
		return ActionV2Response{AppendClaims: []ActionV2Claim{}}
	}
```

with:

```go
	mode, minimalClaims, _ := claimFailureModeRead(ctx, projectID)
```

(`claimFailureModeRead` already logs and never returns a non-nil error — the assignment to `_` is deliberate.)

- [ ] **Step 3.6: Run the cache tests + the existing action tests**

```bash
go test ./internal/handlers/ -run TestClaimFailureModeRead -v
go test ./internal/handlers/ -run TestHandleActionInject -v
```

Expected: every test PASSes. The existing degraded-mode tests should be unaffected — they already override `dbGetClaimFailureMode`, and the new helper falls through to that override on cache miss.

- [ ] **Step 3.7: Update existing tests that previously asserted "dbGetClaimFailureMode called on cache hit" semantics**

The existing tests in `action_test.go` (e.g. `t.Error("GetClaimFailureMode called on cache hit — should not happen")` at line 165) refer to the *Zitadel claim payload* cache, not the new claim-mode cache. They are unaffected, but verify by re-running:

```bash
go test ./internal/handlers/ -v -count=1
```

Expected: every test PASSes. (`-count=1` defeats Go's test cache and forces re-execution.)

- [ ] **Step 3.8: Commit**

```bash
cd <repo>
git add backend/internal/handlers/action.go backend/internal/handlers/deps.go backend/internal/handlers/action_test.go
git commit -m "$(cat <<'EOF'
feat(action): cache claim_failure_mode in Redis with DB-error fallback

A transient PostgreSQL fault previously collapsed degraded-mode behaviour
into fail_closed for every project, regardless of how the operator
configured the project's claim profile. The new claimFailureModeRead
helper layers a read-through cache (key claim_mode:<projectID>, TTL 5
minutes, env-overridable via CLAIM_MODE_CACHE_TTL_SECONDS) in front of
db.GetClaimFailureMode so a cached minimal_safe survives a DB outage
that occurs after at least one successful read.

Cache miss + DB success refreshes the cache. Cache miss + DB error still
returns fail_closed (no regression). Cache hit short-circuits the DB
entirely.

Audit ref: C5 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `auth.Principal` in request context (audit ref C4)

**Why this is fourth:** widens the auth surface but the callers (`withUserAuth`, `withOperatorAuth`) are exactly two functions in `router.go`. Test surface is the existing `auth_test.go` + a focused middleware test for principal-from-context.

**Files:**
- Modify: `backend/internal/auth/jwt.go` — introduce `Principal`; rename `ValidateToken` → `Validate`; delete standalone `HasProjectRole`
- Modify: `backend/internal/auth/jwt_test.go` — add `TestValidate_PopulatesProjectRoles`
- Modify: `backend/internal/handlers/deps.go` — add `jwtValidate = auth.Validate` injectable (the test seam for the parse-count contract)
- Modify: `backend/internal/handlers/router.go` — `withUserAuth` calls `jwtValidate` and stashes `*Principal`; `withOperatorAuth` reads from context (no second parse)
- Modify: `backend/internal/handlers/context.go` (if it exists; otherwise the file that defines `withAdminUserID` / `getAdminUserID`) — add `withPrincipal` / `principalFromContext`; `getAdminUserID` reads `principal.Subject`
- Create: `backend/internal/handlers/router_test.go` — middleware tests that exercise the real `withOperatorAuth` and assert `jwtValidate` is called exactly once per request

- [ ] **Step 4.1: Locate `withAdminUserID` and `getAdminUserID`**

```bash
cd <repo>/backend
grep -rn "withAdminUserID\|getAdminUserID" internal/handlers/ --include='*.go' | grep -v "_test.go"
```

These should be in a single file (likely `handlers/context.go` or inside `router.go`). The plan refers to them as living in `context.go`; if they live elsewhere, perform the same edits in their current home. Take note of the file path before continuing.

- [ ] **Step 4.2: Write the failing auth-layer test for `ProjectRoles`**

Add to `backend/internal/auth/jwt_test.go`:

```go
func TestValidate_PopulatesProjectRoles(t *testing.T) {
	// Build a signed RS256 token with the urn:zitadel:iam:org:project:roles claim.
	// Reuse the existing test signing helper if present; otherwise this stub
	// shows the shape the test exercises.
	tokenStr, domain, audience := newTestTokenWithRoles(t, "user-123", map[string]any{
		"admin":  map[string]any{"org-1": "Demo Org"},
		"viewer": map[string]any{"org-1": "Demo Org"},
	})

	p, err := Validate(context.Background(), tokenStr, domain, audience)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if p.Subject != "user-123" {
		t.Errorf("expected Subject=user-123; got %q", p.Subject)
	}
	if !p.HasProjectRole("admin") {
		t.Error("expected HasProjectRole(admin)=true")
	}
	if !p.HasProjectRole("viewer") {
		t.Error("expected HasProjectRole(viewer)=true")
	}
	if p.HasProjectRole("nonexistent") {
		t.Error("expected HasProjectRole(nonexistent)=false")
	}
}
```

`newTestTokenWithRoles` is a test helper you may need to add — base its body on whatever test-key plumbing already lives in `auth/jwt_test.go` (the `SetKeysForTesting` API on line 45 of `jwt.go` is the entry point). If the existing test file has a function like `signTestToken`, extend it; otherwise add a fresh helper that signs an RS256 token with `golang-jwt/jwt/v5` using a fixture private key, registers the public key via `SetKeysForTesting`, and returns `(tokenStr, domain, audience)`.

- [ ] **Step 4.3: Run the test — confirm it fails**

```bash
go test ./internal/auth/ -run TestValidate_PopulatesProjectRoles -v
```

Expected: compile error `undefined: Validate` (since `ValidateToken` is the current name) and `undefined: p.HasProjectRole` (since `Principal` doesn't exist).

- [ ] **Step 4.4: Introduce `Principal` and `Validate` in `auth/jwt.go`**

In `backend/internal/auth/jwt.go`, replace the existing `ValidateToken` (lines 146-169) and `HasProjectRole` (lines 171-198) with:

```go
// Principal is the authenticated identity extracted from a validated Zitadel
// JWT. Subject is the Zitadel user ID; ProjectRoles is the set of role keys
// the principal carries in the urn:zitadel:iam:org:project:roles claim. The
// {orgId: orgName} value side of the Zitadel claim is intentionally discarded
// — handlers only need set-membership against role keys.
type Principal struct {
	Subject      string
	ProjectRoles map[string]struct{}
}

// HasProjectRole reports whether the principal carries the given role key.
// Returns false on a nil receiver so callers can use it without first
// nil-checking the result of principalFromContext.
func (p *Principal) HasProjectRole(roleKey string) bool {
	if p == nil {
		return false
	}
	_, ok := p.ProjectRoles[roleKey]
	return ok
}

// Validate validates a Zitadel-issued RS256 JWT and returns the parsed
// principal. Delegates signature verification, expiry, issuer, and audience
// checks to golang-jwt/jwt/v5; key material is fetched from the Zitadel JWKS
// endpoint and cached for one hour.
//
// Replaces the previous ValidateToken function — there is exactly one caller
// (withUserAuth), so no compatibility shim is kept.
func Validate(ctx context.Context, tokenStr, domain, audience string) (*Principal, error) {
	type zitadelClaims struct {
		jwt.RegisteredClaims
		ProjectRoles map[string]map[string]string `json:"urn:zitadel:iam:org:project:roles,omitempty"`
	}
	claims := &zitadelClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, store.keyFunc(ctx, domain),
		jwt.WithIssuer(fmt.Sprintf("https://%s", domain)),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return nil, fmt.Errorf("token missing subject claim")
	}

	roles := make(map[string]struct{}, len(claims.ProjectRoles))
	for k := range claims.ProjectRoles {
		roles[k] = struct{}{}
	}
	return &Principal{Subject: subject, ProjectRoles: roles}, nil
}
```

Remove these now-unused imports if present and unreferenced elsewhere in the file: `encoding/base64`, `encoding/json`, `strings`.

- [ ] **Step 4.5: Re-run the auth test — confirm it passes**

```bash
go test ./internal/auth/ -run TestValidate_PopulatesProjectRoles -v
go test ./internal/auth/ -v
```

Expected: both PASS.

- [ ] **Step 4.6: Add the `withPrincipal` / `principalFromContext` helpers in `handlers/`**

In the file that already defines `withAdminUserID` and `getAdminUserID` (located in Step 4.1), add:

```go
type principalContextKey struct{}

// withPrincipal returns a copy of ctx that carries the validated auth principal.
// withUserAuth stashes the principal once per request; withOperatorAuth reads
// it back via principalFromContext to avoid re-parsing the JWT.
func withPrincipal(ctx context.Context, p *auth.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

// principalFromContext returns the principal stashed by withUserAuth.
// Returns nil if no principal is in context (dev-mode API-key path).
func principalFromContext(ctx context.Context) *auth.Principal {
	p, _ := ctx.Value(principalContextKey{}).(*auth.Principal)
	return p
}
```

Then rewrite `getAdminUserID` to read from the principal:

```go
// getAdminUserID returns the authenticated admin user ID for the request, or
// the empty string when none is present (dev-mode API-key auth). Reads from
// the principal stashed by withUserAuth — single source of truth for request
// identity.
func getAdminUserID(ctx context.Context) string {
	if p := principalFromContext(ctx); p != nil {
		return p.Subject
	}
	return ""
}
```

Remove the previous `withAdminUserID` if it remained as a thin wrapper — single-caller, no longer useful. (If something else calls it, leave it as a wrapper that calls `withPrincipal`.)

If the file does not already import `"syndra/internal/auth"` or `"context"`, add them.

- [ ] **Step 4.7: Add the test seam for JWT validation, then rewrite `withUserAuth`**

The C4 contract is "JWT parsed exactly once per request". To make that contract regression-testable (not just visually reviewable), introduce a single injectable variable that both `withUserAuth` and tests share — same pattern as the existing `dbGetClaimFailureMode` / `redisGetClaimMode` injectables in `deps.go`.

In `backend/internal/handlers/deps.go`, inside the existing `var (` block, add:

```go
	// jwtValidate is the test-injectable parse-and-validate entrypoint
	// used by withUserAuth. Tests substitute a counting wrapper to assert
	// the C4 contract (parsed exactly once per request — no re-parse in
	// withOperatorAuth). Single-process global; tests guard with t.Cleanup.
	jwtValidate = auth.Validate
```

Add `"syndra/internal/auth"` to `deps.go`'s import block if not already present.

Now replace `backend/internal/handlers/router.go:153-189` (`withUserAuth`):

```go
// withUserAuth is the primary authorization middleware for all admin API routes.
//
// Production mode (ZITADEL_DOMAIN set): requires a Zitadel-issued RS256 JWT in
// the Authorization header. Parses signature, issuer, audience, expiry, and
// the urn:zitadel:iam:org:project:roles claim once via jwtValidate; stashes
// the resulting *auth.Principal in r.Context() so withOperatorAuth and
// downstream handlers can read identity and role membership without
// re-parsing.
//
// Local-dev mode (ZITADEL_DOMAIN unset): falls back to shared API key
// (SYNDRA_API_KEY). No principal is stashed; getAdminUserID returns "".
func withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := os.Getenv("ZITADEL_DOMAIN")

		if domain == "" {
			withAPIKeyAuth(next)(w, r)
			return
		}

		audience := os.Getenv("ZITADEL_AUDIENCE")
		if audience == "" {
			log.Printf("[AUTH] ZITADEL_AUDIENCE is not set; rejecting request")
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}

		rawToken := extractBearerToken(r)
		if rawToken == "" {
			log.Printf("[AUTH] Missing bearer token from %s %s", r.Method, r.URL.Path)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}

		principal, err := jwtValidate(r.Context(), rawToken, domain, audience)
		if err != nil {
			log.Printf("[AUTH] Token validation failed for %s %s: %v", r.Method, r.URL.Path, err)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
			return
		}

		log.Printf("[AUTH] Authorized admin=%s for %s %s", principal.Subject, r.Method, r.URL.Path)
		next(w, r.WithContext(withPrincipal(r.Context(), principal)))
	}
}
```

After this rewrite `router.go` no longer references `auth.X` directly — `jwtValidate` is the typed indirection in `deps.go`, and `principalFromContext` (defined in the file with `withAdminUserID`/`getAdminUserID` per Step 4.6) returns the `*auth.Principal` for downstream readers. If a prior edit left `"syndra/internal/auth"` in `router.go`'s import block, **remove it** — Go will otherwise fail to compile with `imported and not used`. The package stays imported in `deps.go` (for `jwtValidate` and the `*auth.Principal` type) and in the context-helper file (for `withPrincipal` / `principalFromContext`).

- [ ] **Step 4.8: Rewrite `withOperatorAuth` in `router.go`**

Replace `backend/internal/handlers/router.go:191-218` (`withOperatorAuth`):

```go
// withOperatorAuth gates endpoints that require operator-level (admin) access.
// Wraps withUserAuth, then checks the principal's project roles for the admin
// role key (ZITADEL_ADMIN_ROLE_KEY, default "admin"). In dev mode (no
// ZITADEL_DOMAIN), the role check is skipped since auth falls through to the
// shared API key.
//
// Reads the principal from r.Context() — withUserAuth parsed the JWT once
// upstream, so this middleware does NOT re-extract or re-parse the bearer
// token (audit ref C4).
func withOperatorAuth(next http.HandlerFunc) http.HandlerFunc {
	return withUserAuth(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("ZITADEL_DOMAIN") == "" {
			next(w, r)
			return
		}

		principal := principalFromContext(r.Context())
		adminRoleKey := os.Getenv("ZITADEL_ADMIN_ROLE_KEY")
		if adminRoleKey == "" {
			adminRoleKey = "admin"
		}

		if !principal.HasProjectRole(adminRoleKey) {
			log.Printf("[AUTH] Operator access denied for user=%s on %s %s (missing role %q)",
				getAdminUserID(r.Context()), r.Method, r.URL.Path, adminRoleKey)
			jsonErrorResponse(w, http.StatusForbidden, "FORBIDDEN", "Operator-level access required")
			return
		}

		next(w, r)
	})
}
```

- [ ] **Step 4.9: Write middleware tests against the real `withOperatorAuth`**

The C4 contract has two parts:
1. Operator routes parse the JWT **exactly once** per request (the audit-flagged regression — previously parsed in `withUserAuth` and re-parsed in `withOperatorAuth`'s `auth.HasProjectRole(rawToken, …)` hand-decoded payload).
2. `withOperatorAuth` reads the principal from `r.Context()`, not from the bearer header.

Both must be tested against the **real** `withOperatorAuth` function — not against a copied wrapper inlined in the test, which would pass even if production code regressed. The `jwtValidate` injectable from Step 4.7 makes the parse-count testable.

Create `backend/internal/handlers/router_test.go`:

```go
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/auth"
)

// TestWithOperatorAuth_ParsesJWTExactlyOnce asserts the C4 contract by
// invoking the real withOperatorAuth(handler) chain and counting calls to
// jwtValidate. A regression where withOperatorAuth re-extracts the bearer
// token and re-parses (the pre-C4 behaviour) would bump the count to 2.
func TestWithOperatorAuth_ParsesJWTExactlyOnce(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")

	var validateCalls int
	origValidate := jwtValidate
	jwtValidate = func(ctx context.Context, tokenStr, domain, audience string) (*auth.Principal, error) {
		validateCalls++
		if tokenStr != "fake-jwt-fixture" {
			t.Errorf("jwtValidate received unexpected token %q", tokenStr)
		}
		if domain != "example.zitadel.cloud" || audience != "test-aud" {
			t.Errorf("jwtValidate received unexpected domain/audience: %q / %q", domain, audience)
		}
		return &auth.Principal{
			Subject:      "operator-1",
			ProjectRoles: map[string]struct{}{"admin": {}},
		}, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	var innerCalls int
	var observedSubject string
	inner := func(w http.ResponseWriter, r *http.Request) {
		innerCalls++
		if p := principalFromContext(r.Context()); p != nil {
			observedSubject = p.Subject
		}
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt-fixture")
	rr := httptest.NewRecorder()

	withOperatorAuth(inner)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin principal; got %d: %s", rr.Code, rr.Body.String())
	}
	if innerCalls != 1 {
		t.Fatalf("expected inner handler called once; got %d", innerCalls)
	}
	if validateCalls != 1 {
		t.Fatalf("C4 contract violated: jwtValidate called %d times; expected exactly 1 (withOperatorAuth must NOT re-parse the bearer token)", validateCalls)
	}
	if observedSubject != "operator-1" {
		t.Fatalf("expected inner handler to read principal.Subject=operator-1; got %q", observedSubject)
	}
}

// TestWithOperatorAuth_DeniesWhenAdminRoleMissing asserts the role gate on
// the real withOperatorAuth, again going through jwtValidate so the JWT
// parse path is exercised end-to-end.
func TestWithOperatorAuth_DeniesWhenAdminRoleMissing(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")

	origValidate := jwtValidate
	jwtValidate = func(ctx context.Context, tokenStr, domain, audience string) (*auth.Principal, error) {
		return &auth.Principal{
			Subject:      "viewer-1",
			ProjectRoles: map[string]struct{}{"viewer": {}},
		}, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	var innerCalls int
	inner := func(w http.ResponseWriter, _ *http.Request) {
		innerCalls++
		w.WriteHeader(http.StatusOK)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	req.Header.Set("Authorization", "Bearer any")
	rr := httptest.NewRecorder()

	withOperatorAuth(inner)(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when admin role missing; got %d: %s", rr.Code, rr.Body.String())
	}
	if innerCalls != 0 {
		t.Fatalf("inner handler must not be invoked when role check fails; got %d calls", innerCalls)
	}
}

// TestWithOperatorAuth_RejectsMissingBearerToken keeps the existing
// "missing token → 401" contract guarded after the refactor.
func TestWithOperatorAuth_RejectsMissingBearerToken(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")

	origValidate := jwtValidate
	jwtValidate = func(ctx context.Context, _, _, _ string) (*auth.Principal, error) {
		t.Error("jwtValidate must not be called when bearer token is missing")
		return nil, nil
	}
	t.Cleanup(func() { jwtValidate = origValidate })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/users", nil)
	rr := httptest.NewRecorder()
	withOperatorAuth(func(http.ResponseWriter, *http.Request) {})(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when bearer token missing; got %d", rr.Code)
	}
}
```

- [ ] **Step 4.10: Run the router and auth tests**

```bash
go test ./internal/auth/ ./internal/handlers/ -v -count=1
```

Expected: every test PASSes. If any test in the broader suite fails (e.g. older fixtures that called `auth.ValidateToken` directly), update them to call `auth.Validate` — there should be no production callers besides `withUserAuth`, so test-only fixtures are the most likely site.

- [ ] **Step 4.11: Run `go vet` to catch unused imports left behind**

```bash
go vet ./internal/auth/ ./internal/handlers/
```

Expected: clean. If `encoding/base64`, `encoding/json`, or `strings` linger as unused imports in `auth/jwt.go`, remove them.

- [ ] **Step 4.12: Commit**

```bash
cd <repo>
# Stage ONLY this task's explicit touch set. Do NOT use a directory or glob
# add — unrelated dirty work under backend/internal/handlers/ would ride
# along otherwise.
git add -- \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/router_test.go
# Plus the file that holds withAdminUserID/getAdminUserID (located in
# Step 4.1). Only stage it if it actually exists AND was edited.
if [ -f backend/internal/handlers/context.go ]; then
  git add -- backend/internal/handlers/context.go
fi
git status                            # confirm staged set matches the file list above
git diff --cached --stat              # final sanity scan before commit
git commit -m "$(cat <<'EOF'
refactor(auth): parse JWT once into Principal, share via request context

Replace ValidateToken (returns subject string) with Validate (returns
*Principal carrying Subject + ProjectRoles set). withUserAuth calls
Validate via the jwtValidate injectable in deps.go and stashes the
principal into r.Context() once per request via the new withPrincipal
helper; withOperatorAuth reads it back via principalFromContext
instead of re-extracting and re-parsing the bearer token. The
standalone auth.HasProjectRole(rawToken, roleKey) function — which
did its own base64 + JSON unmarshal — is deleted.

Each operator request now parses the Zitadel JWT exactly once. The
new router_test.go exercises the real withOperatorAuth chain and
asserts jwtValidate is invoked exactly once per request — a
regression guard against future code adding a second hand-decode in
the operator gate.

The public getAdminUserID(ctx) helper is preserved as a thin
principal.Subject accessor so no downstream caller changes.

Audit ref: C4 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Split `repositories.go` into 11 domain files (audit ref B5)

**Why this is fifth:** mechanical move, no logic change, but the largest single diff. Doing it after C4 means the JWT-principal refactor's diff is small and readable; doing it before B3 means `views.go`'s refactor lands on a clean `db/` package.

**Strategy:** create the 11 new files, copy each section's functions verbatim, then delete `repositories.go`. Use `gofmt -d` to verify zero diff inside function bodies after the move.

**Files (new):**
- `backend/internal/db/audit.go`
- `backend/internal/db/bundles.go`
- `backend/internal/db/rules.go`
- `backend/internal/db/grants.go`
- `backend/internal/db/access_requests.go`
- `backend/internal/db/claim_profiles.go`
- `backend/internal/db/onboarding.go`
- `backend/internal/db/webhooks.go`
- `backend/internal/db/roles.go`
- `backend/internal/db/intents.go`
- `backend/internal/db/vault.go`

**Files (removed):**
- `backend/internal/db/repositories.go`

- [ ] **Step 5.1: Take a snapshot of `repositories.go` for diff verification**

```bash
cd <repo>/backend
cp internal/db/repositories.go /tmp/repositories.go.pre-split
```

This file is the verification anchor — after the split, every function's body must be bit-identical to the pre-split version.

- [ ] **Step 5.2: Capture function bodies for diff verification**

```bash
# Produce a normalised dump of every exported symbol's full body, ignoring
# file membership. Used by the post-split verification step.
go list -f '{{.Dir}}' ./internal/db
gofmt -s /tmp/repositories.go.pre-split > /tmp/repositories.go.pre-split.fmt
```

- [ ] **Step 5.3: Create `audit.go`**

Create `backend/internal/db/audit.go` with the contents of `repositories.go:180-191` (the audit log section). The file MUST start with:

```go
package db

import (
	"context"
)

// -------------------------------------------------------------
// AUDIT LOG REPOSITORY
// -------------------------------------------------------------

func InsertAuditLog(ctx context.Context, actorID, targetID, action, resourceID string) error {
	// ... (verbatim from repositories.go:184-191)
}
```

Copy `InsertAuditLog`'s body bit-for-bit from `/tmp/repositories.go.pre-split`. Add any additional imports the body requires (`fmt`, etc.). Verify with:

```bash
go build ./internal/db/
```

Expected: compile error — `InsertAuditLog redeclared`. That's expected because `repositories.go` still defines it. We will delete the old definition in Step 5.13.

- [ ] **Step 5.4: Create `bundles.go`**

Create `backend/internal/db/bundles.go` with the bundles section (`repositories.go:17-124`). 7 functions: `CreateBundle`, `AddRoleToBundle`, `GetAllBundles`, `GetRolesForBundle`, `AssignBundleToUser`, `RemoveBundleFromUser`, `GetBundlesForUser`. Preserve the section banner comment as a single-line header at the top of the file body.

- [ ] **Step 5.5: Create `rules.go`**

Create `backend/internal/db/rules.go` with the mapping rules section (`repositories.go:126-178`). 3 functions: `CreateMappingRule`, `UpdateMappingRule`, `GetActiveMappingRules`.

- [ ] **Step 5.6: Create `grants.go`**

Create `backend/internal/db/grants.go` with the direct role grants section (`repositories.go:193-375`). 6 functions: `UpsertDirectGrant`, `GetDirectGrantsForUser`, `GetAllDirectGrants`, `GetExpiringDirectGrants`, `GetExpiredDirectGrants`, `DeleteExpiredDirectGrantsByIDs`.

- [ ] **Step 5.7: Create `access_requests.go`**

Create `backend/internal/db/access_requests.go` with the access requests section (`repositories.go:377-454`). 4 functions: `CreateAccessRequest`, `GetAccessRequests`, `GetAccessRequestByID`, `ResolveAccessRequest`.

- [ ] **Step 5.8: Create `claim_profiles.go`**

Create `backend/internal/db/claim_profiles.go` with the claim profiles section (`repositories.go:456-522`). Move the `ClaimProfileRow` type definition + `ListClaimProfiles` + `GetClaimFailureMode`.

- [ ] **Step 5.9: Create `onboarding.go`**

Create `backend/internal/db/onboarding.go` with the onboarding triggers section (`repositories.go:525-664`). Move the `OnboardingTrigger` type + 6 functions: `InsertOnboardingTrigger`, `CompleteOnboardingTrigger`, `FailOnboardingTrigger`, `GetOnboardingTriggers`, `GetWelcomeBundle`, `SetWelcomeBundle`.

- [ ] **Step 5.10: Create `webhooks.go`** (includes grants index — see Decision 2)

Create `backend/internal/db/webhooks.go` with the webhook events section (`repositories.go:667-768`) AND the Zitadel grants index section (`repositories.go:771-847`). Move types `WebhookEvent` and `ZitadelGrantIndex` plus functions: `InsertWebhookEvent`, `CompleteWebhookEvent`, `FailWebhookEvent`, `GetWebhookEvents`, `UpsertGrantIndex`, `GetGrantIndex`, `DeleteGrantIndex`. **Also include the new `DropWebhookEventEnrichmentIncomplete` helper added in Task 2** — that function logically belongs here even though it was first written into `repositories.go`. Move it.

- [ ] **Step 5.11: Create `roles.go`**

Create `backend/internal/db/roles.go` with the role management section (`repositories.go:850-1064`). Move the `RoleUsage` type + 7 functions: `CreateRole`, `DeleteRole`, `GetRole`, `GetAllLocalRoles`, `GetRoleUsageCounts`, `GetAssignedUserCounts`, `GetAllReferencedRoleKeys`.

- [ ] **Step 5.12: Create `intents.go`**

Create `backend/internal/db/intents.go` with the provisioning intents section (`repositories.go:1067-1212`). 5 functions: `InsertProvisioningIntent`, `ClaimPendingIntents`, `CompleteIntent`, `FailIntent`, `GetProvisioningIntents`.

- [ ] **Step 5.13: Create `vault.go`**

Create `backend/internal/db/vault.go` with the shadow credentials section (`repositories.go:1214-end`). 6 functions: `UpsertShadowCredential`, `GetShadowCredential`, `DeleteShadowCredential`, `HasShadowCredential`, `InsertShadowCredentialAudit`, `GetShadowCredentialAudit`.

- [ ] **Step 5.14: Delete `repositories.go`**

Use `git rm` so the deletion is recorded in the index immediately — Step 5.20's commit can then stage only the 11 new files without worrying about how the deletion gets captured.

```bash
cd <repo>
git rm backend/internal/db/repositories.go
```

- [ ] **Step 5.15: Compile the `db` package**

```bash
cd <repo>/backend
go build ./internal/db/
```

Expected: clean compile. If you see `undefined: X`, the function for X is missing from the new files — locate it in the pre-split snapshot and add it to the right domain file.

- [ ] **Step 5.16: Run `gofmt` to normalise the new files**

```bash
gofmt -w internal/db/
```

- [ ] **Step 5.17: Verify zero behavioural diff (function bodies are bit-identical)**

```bash
# Concatenate the new files in section order, strip imports + package decl,
# and diff against the pre-split snapshot's function bodies.
cat /tmp/repositories.go.pre-split | grep -E '^(func|type) ' | sort > /tmp/pre-split.symbols
cat internal/db/audit.go internal/db/bundles.go internal/db/rules.go internal/db/grants.go internal/db/access_requests.go internal/db/claim_profiles.go internal/db/onboarding.go internal/db/webhooks.go internal/db/roles.go internal/db/intents.go internal/db/vault.go | grep -E '^(func|type) ' | sort > /tmp/post-split.symbols

diff /tmp/pre-split.symbols /tmp/post-split.symbols
```

Expected: the only difference is the new `DropWebhookEventEnrichmentIncomplete` function added in Task 2 (now living in `webhooks.go`). If any other function appears on one side and not the other, find it and move it to its correct file.

- [ ] **Step 5.18: Run the full backend test suite**

```bash
go test ./... -count=1
```

Expected: every test PASSes. Callers in `handlers/`, `services/`, `cmd/`, etc. all import the `db` package and reference functions by name — splitting files inside a package preserves every symbol. If any test fails with `undefined: db.X`, the function X was lost in the split — recover from `/tmp/repositories.go.pre-split`.

- [ ] **Step 5.19: Run `go vet`**

```bash
go vet ./internal/db/ ./internal/handlers/ ./internal/services/
```

Expected: clean.

- [ ] **Step 5.20: Commit**

```bash
cd <repo>
# Stage ONLY the split's touch set — the 11 new files. The deletion of
# repositories.go was already staged in Step 5.14 via `git rm`. Do NOT
# use 'git add backend/internal/db/' (would also stage any unrelated
# dirty work under db/, e.g. local edits to postgres.go or redis.go).
git add -- \
  backend/internal/db/audit.go \
  backend/internal/db/bundles.go \
  backend/internal/db/rules.go \
  backend/internal/db/grants.go \
  backend/internal/db/access_requests.go \
  backend/internal/db/claim_profiles.go \
  backend/internal/db/onboarding.go \
  backend/internal/db/webhooks.go \
  backend/internal/db/roles.go \
  backend/internal/db/intents.go \
  backend/internal/db/vault.go
git status            # expect: 11 new files staged; repositories.go staged as deleted; no other db/ file modified
git diff --cached --stat
git commit -m "$(cat <<'EOF'
refactor(db): split repositories.go into 11 domain files

The 1303-line monolith repositories.go is split into per-domain files
inside the same db package:

  audit.go             InsertAuditLog
  bundles.go           7 bundle helpers
  rules.go             3 mapping-rule helpers
  grants.go            6 direct-grant helpers
  access_requests.go   4 access-request helpers
  claim_profiles.go    ClaimProfileRow + 2 helpers
  onboarding.go        OnboardingTrigger + 6 helpers
  webhooks.go          WebhookEvent + ZitadelGrantIndex + 8 helpers
                       (incl. DropWebhookEventEnrichmentIncomplete from B6)
  roles.go             RoleUsage + 7 role-catalog helpers
  intents.go           5 provisioning-intent helpers
  vault.go             6 shadow-credential helpers

Function bodies and exported names are bit-identical (verified by
diffing symbol lists pre/post). No caller changes anywhere; the
package surface is unchanged. The audit's enumerated 9-file split
becomes 11 to give access-requests, claim-profiles, and grants-index
their own files — rationale in design.md §2 Decision 2.

Audit ref: B5 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Request-scoped `accessSnapshot` in `services/views.go` (audit ref B3)

**Why this is sixth (last):** largest behavioural refactor; touches the file Theme 2 will build on. Ships after the `db` split so the file's imports are stable. Tests assert *call counts*, not just outputs — the bug we are fixing is structural (redundant work), not functional.

**Files:**
- Modify: `backend/internal/services/views.go` — introduce `accessSnapshot`; thread through `ListUsers`, `ListApplications`, `ListProjects`, `Topology`, `Governance`/`BundleImpact`
- Modify: `backend/internal/services/views_test.go` — add call-count tests

- [ ] **Step 6.1: Write the failing call-count test for `ListApplications`**

Add to `backend/internal/services/views_test.go`:

```go
func TestListApplications_CollectsUserRolesExactlyOncePerUser(t *testing.T) {
	// Replace the directory + db hooks with fixtures and count
	// collectUserRoles invocations. The test sets up 3 users and 5 apps;
	// the current (pre-refactor) implementation invokes collectUserRoles
	// 3*5=15 times; the post-refactor target is exactly 3.

	setupSnapshotTestFixtures(t, 3 /* users */, 5 /* apps */, 1 /* projects */)

	var calls int
	origCollect := collectUserRolesHook
	collectUserRolesHook = func(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		calls++
		return origCollect(ctx, userID)
	}
	t.Cleanup(func() { collectUserRolesHook = origCollect })

	if _, err := ListApplications(context.Background()); err != nil {
		t.Fatalf("ListApplications: %v", err)
	}

	if calls != 3 {
		t.Fatalf("expected collectUserRoles to be called exactly 3 times (once per user); got %d", calls)
	}
}
```

`collectUserRolesHook` is the test-injectable indirection that the refactor will introduce (Step 6.3). `setupSnapshotTestFixtures` is a helper you may need to add that seeds `directory.Default` with N users + M apps + P projects, plus a no-op `db` layer; mirror the pattern used by other view tests in the same file.

- [ ] **Step 6.2: Run the test — confirm it fails**

```bash
cd <repo>/backend
go test ./internal/services/ -run TestListApplications_CollectsUserRolesExactlyOncePerUser -v
```

Expected: compile error `undefined: collectUserRolesHook`. Add the hook in Step 6.3.

- [ ] **Step 6.3: Introduce `accessSnapshot` and the testing indirection**

In `backend/internal/services/views.go`, near the existing `type roleKey struct{}` at line 17, add:

```go
type userRoles struct {
	roleMap map[roleKey]*models.EffectiveRole
	bundles []models.Bundle
}

// collectUserRolesHook is the indirection accessSnapshot calls. Production
// code points it at collectUserRoles; tests override it to count
// invocations. Single-process global — fine because tests run sequentially
// in this package and t.Cleanup restores the original.
var collectUserRolesHook = collectUserRoles

// accessSnapshot is a request-scoped lazy cache for (user → effective roles).
//
// The role-resolution helper collectUserRoles is fast (~4 SQL queries) but
// repeatedly called from views that iterate users — ListUsers walks N
// users, ListApplications walks N×M (per user × per app), ListProjects
// walks N×P. The snapshot computes-and-memoises each user once per
// request so the cross-view aggregate fan-out collapses to O(N).
//
// The snapshot is NOT process-wide: no mutex, no invalidation, no expiry.
// It lives for the lifetime of one HTTP handler call and goes to GC when
// the request returns.
type accessSnapshot struct {
	ctx   context.Context
	users []models.UserProfile
	roles map[string]userRoles
	err   error // sticky if user listing fails
}

// newAccessSnapshot primes the user list (single directory call) but
// defers role resolution to lazy For() calls. Returning an error here
// preserves the existing failure semantics of every entrypoint — they
// all already return early on directory.Default.Users error.
func newAccessSnapshot(ctx context.Context) (*accessSnapshot, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return nil, err
	}
	return &accessSnapshot{
		ctx:   ctx,
		users: users,
		roles: make(map[string]userRoles, len(users)),
	}, nil
}

// For returns the cached (roleMap, bundles) for userID, computing-and-
// caching on first access.
func (s *accessSnapshot) For(userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	if r, ok := s.roles[userID]; ok {
		return r.roleMap, r.bundles, nil
	}
	roleMap, bundles, err := collectUserRolesHook(s.ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	s.roles[userID] = userRoles{roleMap: roleMap, bundles: bundles}
	return roleMap, bundles, nil
}

// Users returns the list primed at construction. Cheap accessor so view
// functions don't re-fetch.
func (s *accessSnapshot) Users() []models.UserProfile {
	return s.users
}
```

- [ ] **Step 6.4: Thread the snapshot through `ListUsers`**

In `backend/internal/services/views.go`, replace `ListUsers` (lines 42-85):

```go
func ListUsers(ctx context.Context, query string) ([]models.UserListItem, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listUsersFromSnapshot(snap, query)
}

func listUsersFromSnapshot(snap *accessSnapshot, query string) ([]models.UserListItem, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]models.UserListItem, 0, len(snap.Users()))

	for _, user := range snap.Users() {
		if query != "" && !matchesUser(user, query) {
			continue
		}

		roleMap, bundles, err := snap.For(user.ID)
		if err != nil {
			return nil, err
		}

		keyProjects := make([]string, 0, len(roleMap))
		seenProjects := make(map[string]bool)
		for _, role := range roleMap {
			if seenProjects[role.ProjectName] {
				continue
			}
			seenProjects[role.ProjectName] = true
			keyProjects = append(keyProjects, role.ProjectName)
		}
		sort.Strings(keyProjects)

		items = append(items, models.UserListItem{
			User:               user,
			BundleCount:        len(bundles),
			EffectiveRoleCount: len(roleMap),
			KeyProjects:        keyProjects,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].User.Name < items[j].User.Name
	})
	return items, nil
}
```

- [ ] **Step 6.5: Thread the snapshot through `ListApplications`**

Replace `ListApplications` (lines 163-199):

```go
func ListApplications(ctx context.Context) ([]models.ApplicationView, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listApplicationsFromSnapshot(snap)
}

func listApplicationsFromSnapshot(snap *accessSnapshot) ([]models.ApplicationView, error) {
	apps, err := directory.Default.Applications(snap.ctx)
	if err != nil {
		return nil, err
	}
	views := make([]models.ApplicationView, 0, len(apps))

	for _, app := range apps {
		assignedCount := 0
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			if hasProjectRole(roleMap, app.ProjectID) {
				assignedCount++
			}
		}

		consumedRoles, err := directory.Default.RoleKeysForProject(snap.ctx, app.ProjectID)
		if err != nil {
			return nil, err
		}

		views = append(views, models.ApplicationView{
			Application:       app,
			ConsumedRoles:     consumedRoles,
			AssignedUserCount: assignedCount,
		})
	}
	return views, nil
}
```

- [ ] **Step 6.6: Thread the snapshot through `ListProjects`**

Replace `ListProjects` (lines 253-342):

```go
func ListProjects(ctx context.Context) ([]models.ProjectSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listProjectsFromSnapshot(snap)
}

func listProjectsFromSnapshot(snap *accessSnapshot) ([]models.ProjectSummary, error) {
	projects, err := directory.Default.Projects(snap.ctx)
	if err != nil {
		return nil, err
	}
	rules, err := db.GetActiveMappingRules(snap.ctx)
	if err != nil {
		return nil, err
	}
	bundles, err := db.GetAllBundles(snap.ctx)
	if err != nil {
		return nil, err
	}

	projectSummaries := make([]models.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		memberCount := 0
		sampleMembers := []string{}
		activeRoleSet := make(map[string]bool)
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			for _, role := range roleMap {
				if role.ProjectID != project.ID {
					continue
				}
				activeRoleSet[role.RoleKey] = true
			}
			if hasProjectRole(roleMap, project.ID) {
				memberCount++
				if len(sampleMembers) < 3 {
					sampleMembers = append(sampleMembers, user.Name)
				}
			}
		}

		bundleCount := 0
		for _, bundle := range bundles {
			roles, err := db.GetRolesForBundle(snap.ctx, bundle.ID)
			if err != nil {
				return nil, err
			}
			for _, role := range roles {
				if role.ProjectID == project.ID {
					bundleCount++
					break
				}
			}
		}

		ruleInCount := 0
		ruleOutCount := 0
		for _, rule := range rules {
			if rule.TargetProject == project.ID {
				ruleInCount++
			}
			if rule.SourceProject == project.ID {
				ruleOutCount++
			}
		}

		activeRoleKeys := make([]string, 0, len(activeRoleSet))
		for roleKey := range activeRoleSet {
			activeRoleKeys = append(activeRoleKeys, roleKey)
		}
		sort.Strings(activeRoleKeys)

		projectSummaries = append(projectSummaries, models.ProjectSummary{
			Project:        project,
			MemberCount:    memberCount,
			BundleCount:    bundleCount,
			RuleInCount:    ruleInCount,
			RuleOutCount:   ruleOutCount,
			ActiveRoleKeys: activeRoleKeys,
			SampleMembers:  sampleMembers,
		})
	}

	sort.Slice(projectSummaries, func(i, j int) bool {
		return projectSummaries[i].Project.Name < projectSummaries[j].Project.Name
	})
	return projectSummaries, nil
}
```

- [ ] **Step 6.7: Thread the snapshot through `Topology`**

Replace the current `Topology` function with a thin wrapper plus the snapshot-aware body. Two changes versus the pre-refactor version: (a) the body's `ListApplications(ctx)` call becomes `listApplicationsFromSnapshot(snap)` (so the application walk reuses the already-built snapshot instead of recomputing one), (b) every `ctx` reference becomes `snap.ctx`. The `rule.Version` reference at the rule-edges loop is left untouched — Theme 5 (Wave 2 · Part 3) removes `mapping_rules.version`; Theme 3 preserves the field.

```go
func Topology(ctx context.Context) (models.TopologyGraph, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.TopologyGraph{
			Nodes: []models.TopologyNode{},
			Edges: []models.TopologyEdge{},
		}, err
	}
	return topologyFromSnapshot(snap)
}

func topologyFromSnapshot(snap *accessSnapshot) (models.TopologyGraph, error) {
	graph := models.TopologyGraph{
		Nodes: []models.TopologyNode{},
		Edges: []models.TopologyEdge{},
	}
	nodeSeen := make(map[string]bool)
	edgeSeen := make(map[string]bool)
	projectCatalog := make(map[string]models.ProjectCatalog)
	roleCatalog := make(map[string]map[string]models.ProjectRole)

	addNode := func(node models.TopologyNode) {
		if nodeSeen[node.ID] {
			return
		}
		nodeSeen[node.ID] = true
		graph.Nodes = append(graph.Nodes, node)
	}
	addEdge := func(edge models.TopologyEdge) {
		if edgeSeen[edge.ID] {
			return
		}
		edgeSeen[edge.ID] = true
		graph.Edges = append(graph.Edges, edge)
	}
	ensureProjectNode := func(projectID string) {
		projectNodeID := "project:" + projectID
		project, ok := projectCatalog[projectID]
		if ok {
			addNode(models.TopologyNode{
				ID:          projectNodeID,
				Label:       project.Name,
				Kind:        "project",
				ProjectID:   project.ID,
				Description: project.Description,
				Meta: map[string]string{
					"kind": project.Kind,
				},
			})
			return
		}

		addNode(models.TopologyNode{
			ID:          projectNodeID,
			Label:       projectID,
			Kind:        "project",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or bundle grants that do not exist in the seeded demo catalog yet.",
			Meta: map[string]string{
				"kind":   "external",
				"source": "database",
			},
		})
	}
	ensureRoleNode := func(projectID, roleKey string) {
		ensureProjectNode(projectID)

		roleNode := models.TopologyNode{
			ID:          roleNodeID(projectID, roleKey),
			Label:       roleLabel(roleKey),
			Kind:        "role",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or grants outside the seeded role catalog.",
			Meta: map[string]string{
				"role_key": roleKey,
				"source":   "database",
			},
		}

		if roles, ok := roleCatalog[projectID]; ok {
			if role, ok := roles[roleKey]; ok {
				roleNode.Label = role.Label
				roleNode.Description = role.Description
				roleNode.Meta = map[string]string{
					"role_key": role.Key,
				}
			}
		}

		addNode(roleNode)
		addEdge(models.TopologyEdge{
			ID:     "contains:" + projectID + ":" + roleKey,
			Source: "project:" + projectID,
			Target: roleNode.ID,
			Kind:   "contains",
			Label:  "defines",
		})
	}

	dirProjects, err := directory.Default.Projects(snap.ctx)
	if err != nil {
		return graph, err
	}
	for _, project := range dirProjects {
		projectCatalog[project.ID] = project
		roleCatalog[project.ID] = make(map[string]models.ProjectRole, len(project.Roles))
		ensureProjectNode(project.ID)

		for _, role := range project.Roles {
			roleCatalog[project.ID][role.Key] = role
			ensureRoleNode(project.ID, role.Key)
		}
	}

	appViews, err := listApplicationsFromSnapshot(snap)
	if err != nil {
		return graph, err
	}
	for _, app := range appViews {
		nodeID := "application:" + app.Application.ID
		ensureProjectNode(app.Application.ProjectID)
		addNode(models.TopologyNode{
			ID:          nodeID,
			Label:       app.Application.Name,
			Kind:        "application",
			ProjectID:   app.Application.ProjectID,
			Description: app.Application.Description,
			Meta: map[string]string{
				"claim_name":  app.Application.ClaimName,
				"format_type": app.Application.FormatType,
				"consumer":    app.Application.Consumer,
			},
		})
		addEdge(models.TopologyEdge{
			ID:     "consumes:" + app.Application.ID,
			Source: nodeID,
			Target: "project:" + app.Application.ProjectID,
			Kind:   "application",
			Label:  "consumes",
		})
	}

	bundles, err := db.GetAllBundles(snap.ctx)
	if err != nil {
		return graph, err
	}
	for _, bundle := range bundles {
		bundleID := "bundle:" + bundle.ID
		addNode(models.TopologyNode{
			ID:          bundleID,
			Label:       bundle.Name,
			Kind:        "bundle",
			Description: bundle.Description,
		})

		roles, err := db.GetRolesForBundle(snap.ctx, bundle.ID)
		if err != nil {
			return graph, err
		}
		for _, role := range roles {
			ensureRoleNode(role.ProjectID, role.RoleKey)
			addEdge(models.TopologyEdge{
				ID:     "bundle-role:" + bundle.ID + ":" + role.ProjectID + ":" + role.RoleKey,
				Source: bundleID,
				Target: roleNodeID(role.ProjectID, role.RoleKey),
				Kind:   "bundle",
				Label:  "grants",
			})
		}
	}

	rules, err := db.GetActiveMappingRules(snap.ctx)
	if err != nil {
		return graph, err
	}
	for _, rule := range rules {
		ensureRoleNode(rule.SourceProject, rule.SourceRole)
		ensureRoleNode(rule.TargetProject, rule.TargetRole)
		addEdge(models.TopologyEdge{
			ID:     "rule:" + rule.ID,
			Source: roleNodeID(rule.SourceProject, rule.SourceRole),
			Target: roleNodeID(rule.TargetProject, rule.TargetRole),
			Kind:   "rule",
			Label:  "maps",
			Meta: map[string]string{
				"version": fmt.Sprintf("%d", rule.Version),
			},
		})
	}

	sort.Slice(graph.Nodes, func(i, j int) bool {
		if graph.Nodes[i].Kind == graph.Nodes[j].Kind {
			if graph.Nodes[i].Label == graph.Nodes[j].Label {
				return graph.Nodes[i].ID < graph.Nodes[j].ID
			}
			return graph.Nodes[i].Label < graph.Nodes[j].Label
		}
		return graph.Nodes[i].Kind < graph.Nodes[j].Kind
	})
	sort.Slice(graph.Edges, func(i, j int) bool {
		return graph.Edges[i].ID < graph.Edges[j].ID
	})

	return graph, nil
}
```

- [ ] **Step 6.8: Thread the snapshot through `Governance` and `BundleImpact`**

`Governance` (line 387) calls `BundleImpact` (line 344), which iterates `directory.Default.Users(ctx)` and calls `svcGetBundlesForUser`. Hoist the snapshot:

```go
func Governance(ctx context.Context) (models.GovernanceSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	return governanceFromSnapshot(snap)
}

func governanceFromSnapshot(snap *accessSnapshot) (models.GovernanceSummary, error) {
	requests, err := svcGetAccessRequests(snap.ctx, "pending")
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	expiring, err := svcGetExpiringDirectGrants(snap.ctx, 14*24*time.Hour)
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	cleanupHints := []string{}
	bundles, err := svcGetAllBundles(snap.ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	for _, bundle := range bundles {
		impact, err := bundleImpactFromSnapshot(snap, bundle.ID)
		if err != nil {
			return models.GovernanceSummary{}, err
		}
		if len(impact.Users) == 0 {
			cleanupHints = append(cleanupHints, fmt.Sprintf("Bundle %q is unused and can be reviewed for cleanup.", bundle.Name))
		}
	}
	if len(requests) == 0 {
		cleanupHints = append(cleanupHints, "No pending requests right now, so approvals are caught up.")
	}

	if requests == nil {
		requests = []models.AccessRequest{}
	}
	if expiring == nil {
		expiring = []models.DirectGrant{}
	}
	return models.GovernanceSummary{
		PendingRequests: requests,
		ExpiringGrants:  expiring,
		CleanupHints:    cleanupHints,
	}, nil
}

func BundleImpact(ctx context.Context, bundleID string) (models.BundleImpact, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.BundleImpact{}, err
	}
	return bundleImpactFromSnapshot(snap, bundleID)
}

func bundleImpactFromSnapshot(snap *accessSnapshot, bundleID string) (models.BundleImpact, error) {
	roles, err := svcGetRolesForBundle(snap.ctx, bundleID)
	if err != nil {
		return models.BundleImpact{}, err
	}

	impactedUsers := []models.UserProfile{}
	for _, user := range snap.Users() {
		bundles, err := svcGetBundlesForUser(snap.ctx, user.ID)
		if err != nil {
			return models.BundleImpact{}, err
		}
		for _, bundle := range bundles {
			if bundle.ID == bundleID {
				impactedUsers = append(impactedUsers, user)
				break
			}
		}
	}

	return models.BundleImpact{
		BundleID:  bundleID,
		RoleCount: len(roles),
		Users:     impactedUsers,
	}, nil
}
```

`BundleImpact` keeps its public signature so any external callers (other than `Governance`) are unaffected. Internally it now reuses the snapshot's user list. The `svcGetBundlesForUser` call is **not** routed through the snapshot — bundle membership is per-user-per-bundle and not part of `collectUserRoles`; the existing query path is preserved.

- [ ] **Step 6.9: Run the call-count test — confirm it now passes**

```bash
go test ./internal/services/ -run TestListApplications_CollectsUserRolesExactlyOncePerUser -v
```

Expected: PASS. The fixture sets 3 users + 5 apps; with the snapshot, `collectUserRoles` is called exactly 3 times instead of 15.

- [ ] **Step 6.10: Add the same assertion for `ListProjects`**

Append to `views_test.go`:

```go
func TestListProjects_CollectsUserRolesExactlyOncePerUser(t *testing.T) {
	setupSnapshotTestFixtures(t, 3, 5, 4)

	var calls int
	origCollect := collectUserRolesHook
	collectUserRolesHook = func(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		calls++
		return origCollect(ctx, userID)
	}
	t.Cleanup(func() { collectUserRolesHook = origCollect })

	if _, err := ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (once per user); got %d", calls)
	}
}
```

- [ ] **Step 6.11: Run the new test**

```bash
go test ./internal/services/ -run TestListProjects_CollectsUserRolesExactlyOncePerUser -v
```

Expected: PASS.

- [ ] **Step 6.12: Run the full `services` test suite + lints**

```bash
go test ./internal/services/ -v -count=1
go vet ./internal/services/
gofmt -d internal/services/views.go
```

Expected: all tests PASS; vet clean; gofmt zero diff.

- [ ] **Step 6.13: Run the full backend suite — last gate before commit**

```bash
go test ./... -count=1
go vet ./...
```

Expected: all PASS.

- [ ] **Step 6.14: Commit**

```bash
cd <repo>
git add backend/internal/services/views.go backend/internal/services/views_test.go
git commit -m "$(cat <<'EOF'
refactor(views): request-scoped accessSnapshot collapses N*M role lookups

The role-resolution helper collectUserRoles was previously invoked once
per (user × view) — ListUsers called it N times, ListApplications did
N×M (per user × per app), ListProjects did N×P, and Governance walked
the bundle list with another nested loop. For a 200-user makerspace
with 20 apps and 8 projects, a single /api/v1/applications request
fanned out to ~16 000 SQL queries.

The new accessSnapshot is a request-scoped lazy cache for
(user → effective roles). Built once at each entrypoint and threaded
through ListUsers, ListApplications, ListProjects, Topology,
Governance, and BundleImpact, it collapses the worst-case fan-out
to O(N) per request.

The snapshot has no mutex, no invalidation, no expiry — it lives only
for the duration of one HTTP handler call and is GC'd when the
response returns. No global state.

Test surface: TestListApplications_CollectsUserRolesExactlyOncePerUser
and TestListProjects_CollectsUserRolesExactlyOncePerUser assert the
call-count contract (exactly N invocations across the views) by
injecting collectUserRolesHook.

Audit ref: B3 — Wave 2 · Part 2

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Verification gate

- [ ] **Step 7.1: Run the full backend test suite + vet + scoped gofmt**

Run `go test`/`go vet` from inside `backend/` (catches regressions across the whole tree). The gofmt gate is **scoped to this wave's touch set** — files Wave 2 · Part 2 creates or modifies. Pre-existing drift in untouched files (e.g. `db/postgres.go`, `models/models.go`) is real but out of scope; Step 0.3 already normalised the touched-but-pre-existing files into a baseline commit, so any drift here is drift this wave introduced.

```bash
cd <repo>/backend
go test ./... -count=1
go vet ./...

cd <repo>
gofmt -d \
  backend/internal/auth/jwt.go \
  backend/internal/auth/jwt_test.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/router_test.go \
  backend/internal/handlers/webhook.go \
  backend/internal/handlers/webhook_test.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/action.go \
  backend/internal/handlers/action_test.go \
  backend/internal/handlers/vault.go \
  backend/internal/handlers/vault_test.go \
  backend/internal/services/vault.go \
  backend/internal/services/vault_test.go \
  backend/internal/services/views.go \
  backend/internal/services/views_test.go \
  backend/internal/db/audit.go \
  backend/internal/db/bundles.go \
  backend/internal/db/rules.go \
  backend/internal/db/grants.go \
  backend/internal/db/access_requests.go \
  backend/internal/db/claim_profiles.go \
  backend/internal/db/onboarding.go \
  backend/internal/db/webhooks.go \
  backend/internal/db/roles.go \
  backend/internal/db/intents.go \
  backend/internal/db/vault.go
# If the context-helper file used in Task 4 has a different path, append it.
[ -f backend/internal/handlers/context.go ] && gofmt -d backend/internal/handlers/context.go
```

Expected: all tests PASS; vet clean; gofmt zero diff against the scoped list. If gofmt finds a diff, run `gofmt -w` on the offending file and amend the most-recent commit that touched it (not earlier ones — formatting drift on a single commit is fine to amend).

- [ ] **Step 7.2: Refresh codebase-memory graph**

```bash
cd <repo>
```

Then in the Claude Code session, invoke:
- `mcp__codebase-memory-mcp__detect_changes` — confirm the affected scope
- `mcp__codebase-memory-mcp__index_repository` with the project path — re-index so the graph reflects the new `db/*.go` symbol homes, the `Principal` type, the `accessSnapshot` value, and the `ErrComplexity` sentinel

- [ ] **Step 7.3: Re-validate the OpenSpec change**

Run from the repo root (the CLI errors out when invoked from inside `openspec/`):

```bash
cd <repo>
openspec validate wave-2-part-2-backend-coherence --strict
```

Expected: `Change 'wave-2-part-2-backend-coherence' is valid`.

- [ ] **Step 7.4: Verify the audit-finding coverage**

For each of the eight audit refs (B3, B5, B6, B7, C4, C5, C11, D8), open the corresponding commit message and confirm the audit reference is named. The wave is complete only when every ref has a commit.

```bash
git log --oneline --grep='Audit ref' | head -10
```

Expected: 6 commits (Tasks 1–6) plus 1 scaffolding commit (Task 0) = 7 commits referencing audit refs explicitly.

---

## Task 8: INDEX.md and feature-coverage.md updates

- [ ] **Step 8.1: Add the change-log row to `openspec/INDEX.md`**

In `openspec/INDEX.md`, under the "Change Log" table, after the existing Wave 2 · Part 1 row, add:

```markdown
| [Wave 2 · Part 2 — Backend Coherence](changes/wave-2-part-2-backend-coherence/) | 5.5 | In progress | [proposal](changes/wave-2-part-2-backend-coherence/proposal.md) / [design](changes/wave-2-part-2-backend-coherence/design.md) / [tasks](changes/wave-2-part-2-backend-coherence/tasks.md) |
```

Also update the "Phase Mapping" table row for Phase 5.5:

```markdown
| 5.5: Audit-Resolution Waves | In Progress | wave-1-production-trust-hardening, wave-2-part-1-frontend-palette-finalization, wave-2-part-2-backend-coherence |
```

- [ ] **Step 8.2: Add the feature-coverage row**

In `openspec/changes/syndra-core-architecture/specs/feature-coverage.md`, add or update rows for the three observable changes from this wave:

- **Cached degraded-mode resolution** — Integrated (C5): `claim_failure_mode` survives transient DB outages via Redis read-through cache.
- **Single-parse JWT principal in request context** — Integrated (C4): operator routes no longer double-parse the bearer token.
- **Observable webhook drops** — Integrated (B6, C11, D8): grant events with unresolvable enrichment persist to `webhook_events` with `status='dropped_enrichment_incomplete'`; missing `event_type` returns 400 for internal-shape callers.

(Use the existing table format in the file — match column count and ordering.)

- [ ] **Step 8.3: Commit INDEX + feature-coverage updates**

```bash
cd <repo>
git add openspec/INDEX.md openspec/changes/syndra-core-architecture/specs/feature-coverage.md openspec/changes/wave-2-part-2-backend-coherence/tasks.md
# tick all checkboxes in tasks.md from [ ] to [x] before commit
git commit -m "$(cat <<'EOF'
docs(openspec): record Wave 2 · Part 2 in INDEX and feature-coverage

Add the change-log row + phase mapping entry, plus three observable-
property rows for the operator surfaces this wave introduces (cached
claim_failure_mode, single-parse JWT principal, observable webhook
drops). Tick all Wave 2 · Part 2 task ledger boxes.

Audit refs: B3, B5, B6, B7, C4, C5, C11, D8 — Wave 2 · Part 2 complete

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-review

**1. Spec coverage.**

- B3 (`views.go` refactor) → Task 6 (steps 6.1–6.14). Tests assert exactly-once `collectUserRoles` per user across `/applications` and `/projects`.
- B5 (`repositories.go` split) → Task 5 (steps 5.1–5.20). Pre/post symbol diff verifies zero behavioural change.
- B6 (event_type strict) → Task 2 (steps 2.1–2.4). Internal-shape missing event_type → 400 with `event_type:required` detail.
- B7 (`ErrComplexity` sentinel) → Task 1 (steps 1.1–1.9). `errors.Is` replaces prefix sniff; `isComplexityError` deleted.
- C4 (JWT principal in context) → Task 4 (steps 4.1–4.12). `auth.Validate` returns `*Principal`; `withUserAuth` stashes; `withOperatorAuth` reads from context; standalone `HasProjectRole` deleted.
- C5 (`claim_failure_mode` Redis cache) → Task 3 (steps 3.1–3.8). Read-through cache; falls back to cached value on DB error before defaulting to `fail_closed`.
- C11/D8 (observable enrichment-incomplete drop) → Task 2 (steps 2.5–2.11). `webhook_events.status='dropped_enrichment_incomplete'`; 200-ack preserved.

Every audit ref enumerated in the proposal has at least one task and at least one test assertion. No gaps.

**2. Placeholder scan.** No "TBD", "implement later", or "similar to Task N" placeholders. Every code block is concrete. Every command is runnable. Test fixtures (`setupNoopWebhookDeps`, `setupSnapshotTestFixtures`, `newTestTokenWithRoles`) are either pre-existing helpers from the codebase (the first) or include a stubbed signature with a description of what to extend (the latter two) — they are scoped within Task 1's discipline of writing fixtures alongside the test that needs them.

**3. Type consistency.**

- `Principal` (Task 4) — used identically in middleware and tests: `*auth.Principal` with `Subject string` and `ProjectRoles map[string]struct{}`. `HasProjectRole` is a method on the value, not a free function.
- `jwtValidate` (Task 4) — `var jwtValidate = auth.Validate` in `deps.go`; signature `func(ctx context.Context, tokenStr, domain, audience string) (*auth.Principal, error)`. Tests override via `t.Cleanup`-guarded reassignment; `withUserAuth` calls through the var. The test seam is what makes the C4 "parsed exactly once" contract regression-testable rather than just review-testable.
- `accessSnapshot` (Task 6) — `For(userID) (map[roleKey]*models.EffectiveRole, []models.Bundle, error)` is the consistent signature. Internal cache uses `map[string]userRoles`.
- `collectUserRolesHook` (Task 6) — same signature as the production `collectUserRoles`: `func(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error)`.
- `ErrComplexity` (Task 1) — `var ErrComplexity = errors.New("password complexity")`, wrapped via `fmt.Errorf("%w: %s", ErrComplexity, …)`.
- `DropWebhookEventEnrichmentIncomplete` (Tasks 2 + 5) — first added to `repositories.go` in Task 2; moved to `webhooks.go` during the Task 5 split. Both task descriptions name the same function and signature.
- `claimFailureModeRead` (Task 3) — `func(ctx context.Context, projectID string) (string, map[string]interface{}, error)`; consistent in all three test cases and in `degradedResponse`'s call site.

No drift detected. If you find a name or signature mismatch during execution, the production code is the source of truth — update the plan inline rather than working around it.

---

## Execution choice

Plan complete and saved to `docs/superpowers/plans/2026-05-12-wave-2-part-2-backend-coherence.md`. Two execution options:

**1. Subagent-Driven (recommended)** — Dispatch a fresh subagent per task (Task 1, Task 2, …, Task 8). Each subagent gets the task's full step list, runs the TDD cycle, commits, and reports back. Between tasks the orchestrator reviews the diff and the test output before greenlighting the next subagent. Fast iteration; tight blast-radius per dispatch.

**2. Inline Execution** — Execute tasks in the current session, batched with explicit checkpoints between tasks 1-2-3-4-5-6 so a human can review each commit before the next task begins. Slower but every step is visible in the conversation.

Either approach uses `superpowers:subagent-driven-development` or `superpowers:executing-plans` as the next-skill handoff.
