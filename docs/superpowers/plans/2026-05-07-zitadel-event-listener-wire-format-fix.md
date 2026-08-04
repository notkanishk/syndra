# Zitadel Event-Listener Wire-Format Fix + Grants Index Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Note for this repo:** The user has asked that no git operations be performed. Skip every `git add` / `git commit` step verbatim and just leave the working tree dirty for review. Tasks are still organized as if commits would happen, so the boundaries are clean if commits are reintroduced later.

**Goal:** Make `/api/webhooks/zitadel` correctly process every subscribed Zitadel event end-to-end, so `/operations` reflects real lifecycle and grant changes within seconds. Today the listener accepts no real Zitadel-originated event because the translator was built against a guessed payload shape.

**Architecture:** Two-layer fix. (1) Replace the translator's `zitadelEventPayload` struct with one that matches Zitadel's actual `ContextInfoEvent` wire format from `internal/repository/execution/queue.go` — flat top-level fields, snake_case where Zitadel uses snake_case. This unblocks `user.human.added`, `user.human.selfregistered`, `user.deactivated`, `user.locked`, `user.grant.added`. (2) Add a `zitadel_grants_index` table populated from `grant.added` events; the translator uses it to enrich `grant.changed` (missing `projectId`) and `grant.removed` (missing `roleKeys`) before validation runs. On a local-index miss, fall back to Zitadel's `ListUserGrants` API; if even that fails, fill what we can and continue (best-effort) so we never bounce events back as 4xx.

**Tech Stack:** Go 1.22 / `net/http` / Postgres / `pgx` style repositories (see `backend/internal/db/repositories.go`); Zitadel Management v1 API for grant lookup (existing `managementClient.ListUserGrants` reused); migration tooling under `backend/db/migrations/`.

---

## Reference Material — read once before starting

- **Zitadel's actual event-trigger payload struct:** `internal/repository/execution/queue.go` in zitadel/zitadel — `type ContextInfoEvent`. JSON shape:

  ```json
  {
    "aggregateID": "<id>",
    "aggregateType": "user|user_grant|...",
    "resourceOwner": "<orgID>",
    "instanceID": "<instanceID>",
    "version": "v1|v2",
    "sequence": 123,
    "event_type": "user.grant.added",
    "created_at": "RFC3339Nano",
    "userID": "<editorUserID>",
    "event_payload": { /* event-specific, see below */ }
  }
  ```

  Field-name notes that look like typos but aren't: `aggregateID` (not `aggregateId`), `userID` (not `userId`), `event_type` and `event_payload` (snake_case while their siblings are camelCase). Zitadel's JSON tags are inconsistent here — match them exactly.

- **Per-event `event_payload` shapes** (from `internal/repository/user/`, `internal/repository/usergrant/`):

  | event_type | event_payload fields |
  |---|---|
  | `user.human.added` | `{userName, firstName, lastName, email, ...}` (we only need aggregate ID = userID) |
  | `user.human.selfregistered` | same as `user.human.added` |
  | `user.deactivated` | `null` (empty payload — userID is on aggregate) |
  | `user.locked` | `null` |
  | `user.grant.added` | `{userId, projectId, grantId, roleKeys}` |
  | `user.grant.changed` | `{userId, roleKeys}` — **no projectId** |
  | `user.grant.removed` | `{userId, projectId, grantId}` — **no roleKeys** |

- **Why grant.changed/removed are missing fields:** Zitadel models each user-role binding as a `user_grant` aggregate; project is set at creation and immutable, so it isn't re-sent on changes/removals. Consumers are expected to track the aggregate.

- **Why our existing translator/tests passed CI:** The test fixtures and the smoke-test script construct bodies in our (wrong) expected shape, not Zitadel's actual shape. Unit and smoke tests verify the wrong contract. Integration with a live Zitadel was never run before this fix.

- **Existing repos to mirror for new code:**
  - SQL migration pattern: `backend/db/migrations/000007_webhook_events.up.sql`
  - Repository function pattern: `backend/internal/db/repositories.go` — `InsertWebhookEvent`, `GetWebhookEvents`
  - Injectable test seam pattern: `backend/internal/handlers/deps.go` — `dbInsertWebhookEvent = db.InsertWebhookEvent`
  - Zitadel client pattern: `backend/internal/zitadel/client.go` — `managementClient.ListUserGrants`

---

## File Structure

**Create:**
- `backend/db/migrations/000011_zitadel_grants_index.up.sql` — schema for the local grants index
- `backend/db/migrations/000011_zitadel_grants_index.down.sql` — reverse migration
- `backend/internal/handlers/webhook_translate_enrich.go` — best-effort enrichment for grant.changed/removed (local index → Zitadel API → log-and-continue)
- `backend/internal/handlers/webhook_translate_enrich_test.go` — unit tests for the enrichment function
- `docs/superpowers/plans/2026-05-07-zitadel-event-listener-wire-format-fix.md` — this plan

**Modify:**
- `backend/internal/handlers/webhook_translate.go` — replace `zitadelEventPayload` struct, change shape detection key, change editor probe, update `mapGrantEvent` and `translateEventName` to use new field names
- `backend/internal/handlers/webhook_translate_test.go` — rewrite test fixtures to use real wire format
- `backend/internal/handlers/webhook.go` — call enrichment between translator and validation; pass eventID to grant_added processor so it can populate the index
- `backend/internal/handlers/webhook_test.go` — rewrite handler test bodies that use Zitadel-shape input
- `backend/internal/handlers/deps.go` — add injectable `dbUpsertGrantIndex`, `dbGetGrantIndex`, `dbDeleteGrantIndex`, `dbListUserGrantsLive` seams
- `backend/internal/db/repositories.go` — add `UpsertGrantIndex`, `GetGrantIndex`, `DeleteGrantIndex` repository functions
- `scripts/smoke-test-event-listener.sh` — update synthetic body to real Zitadel wire format (so smoke is no longer a fiction)
- `zitadel/actions/EVENTS.md` — update payload reference to match real wire format
- `openspec/changes/zitadel-event-trigger-propagation/specs/application-claims/spec.md` — update translator coverage clauses
- `openspec/changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md` — same
- `openspec/changes/zitadel-event-trigger-propagation/IMPLEMENTATION.md` — record the wire-format correction and the index addition
- `openspec/changes/zitadel-event-trigger-propagation/tasks.md` — append a follow-up section noting the post-merge fix
- `openspec/INDEX.md` — leave change row as Complete; the wire-format defect is captured in the IMPLEMENTATION.md update

---

## Plan Header (write in this order)

1. Foundational wire-format fix — must land first; without it, every event 4xx's.
2. Smoke-test repair — so future regressions are catchable.
3. Grants index migration + repo functions.
4. Enrichment function with index-then-API fallback.
5. Wire enrichment into handler; teach `processGrantAdded`/`processGrantRemoved` to maintain the index.
6. Manual end-to-end verification against a live Zitadel.
7. OpenSpec + EVENTS.md alignment.

Total: 14 tasks. Most are TDD micro-steps.

---

### Task 1: Replace `zitadelEventPayload` struct with the real Zitadel wire format

**Files:**
- Modify: `backend/internal/handlers/webhook_translate.go:19-38`
- Test: `backend/internal/handlers/webhook_translate_test.go` (full rewrite later in Task 2)

- [ ] **Step 1: Write the failing test (one focused assertion, ignore other tests for now)**

Append to `backend/internal/handlers/webhook_translate_test.go` (don't yet remove old tests — they'll be rewritten in Task 2):

```go
func TestTranslateZitadelEvent_RealWireFormat_GrantAdded(t *testing.T) {
	body := []byte(`{
		"aggregateID":"371000000000000001",
		"aggregateType":"user_grant",
		"resourceOwner":"365789968620127493",
		"instanceID":"365789968620061957",
		"version":"v1",
		"sequence":42,
		"event_type":"user.grant.added",
		"created_at":"2026-05-07T17:35:46.464Z",
		"userID":"editor-human-1",
		"event_payload":{"userId":"u-target","projectId":"p-99","grantId":"371000000000000001","roleKeys":["alpha","beta"]}
	}`)
	got, ok, err := translateZitadelEvent(body)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true (real Zitadel shape must be detected)")
	}
	if got.EventType != "grant_added" {
		t.Errorf("EventType = %q, want %q", got.EventType, "grant_added")
	}
	if got.UserID != "u-target" {
		t.Errorf("UserID = %q, want %q (must come from event_payload.userId, not top-level userID which is the editor)", got.UserID, "u-target")
	}
	if got.SourceProject != "p-99" {
		t.Errorf("SourceProject = %q, want %q", got.SourceProject, "p-99")
	}
	if !reflect.DeepEqual(got.RoleKeys, []string{"alpha", "beta"}) {
		t.Errorf("RoleKeys = %v, want [alpha beta]", got.RoleKeys)
	}
}
```

(Add `"reflect"` to the imports if missing.)

- [ ] **Step 2: Run the new test — must fail because the translator still expects the old shape**

```bash
cd backend && go test ./internal/handlers/ -run TestTranslateZitadelEvent_RealWireFormat_GrantAdded -count=1
```

Expected: FAIL — `ok = false, want true` (the shape probe looks for the key `"aggregate"`, which doesn't exist in the real body).

- [ ] **Step 3: Replace the struct + shape detection + editor probe**

Replace `backend/internal/handlers/webhook_translate.go:10-38` (the struct, the doc comment, and the `editorID` method) with:

```go
// zitadelEventPayload mirrors Zitadel's ContextInfoEvent wire format
// (see internal/repository/execution/queue.go in zitadel/zitadel). The
// JSON tags are deliberately mixed-case — Zitadel uses snake_case for
// some fields and camelCase-with-uppercase-ID for others. Match exactly.
//
// Top-level UserID is the EDITOR (the user who triggered the event),
// NOT the subject. The subject of grant events lives in event_payload;
// the subject of user.human.* events is the aggregateID itself.
type zitadelEventPayload struct {
	AggregateID   string          `json:"aggregateID"`
	AggregateType string          `json:"aggregateType"`
	ResourceOwner string          `json:"resourceOwner"`
	InstanceID    string          `json:"instanceID"`
	Version       string          `json:"version"`
	Sequence      uint64          `json:"sequence"`
	EventType     string          `json:"event_type"`
	CreatedAt     string          `json:"created_at"`
	UserID        string          `json:"userID"` // editor — see comment above
	EventPayload  json.RawMessage `json:"event_payload"`
}

// editorID returns the user ID Zitadel attributes the event to. Used by
// the self-mutation guard. Empty string means no editor was reported.
func (e zitadelEventPayload) editorID() string {
	return e.UserID
}
```

And replace the shape-detection block at `webhook_translate.go:55-67` with:

```go
func translateZitadelEvent(body []byte) (WebhookPayload, bool, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return WebhookPayload{}, false, nil
	}
	// Zitadel-shape signal: top-level "aggregateID" is present (flat —
	// not nested under an "aggregate" object). Internal-shape callers
	// use "event_type" + "user_id" with no aggregateID.
	if _, hasAgg := probe["aggregateID"]; !hasAgg {
		return WebhookPayload{}, false, nil
	}

	var ev zitadelEventPayload
	if err := json.Unmarshal(body, &ev); err != nil {
		return WebhookPayload{}, true, err
	}

	m2mID := os.Getenv("ZITADEL_M2M_USER_ID")
	if m2mID == "" {
		warnSelfMutationGuardDisabled()
	} else if editor := ev.editorID(); editor == m2mID {
		log.Printf("[WEBHOOK] dropped self-mutation event=%s aggregate=%s editor=%s", ev.EventType, ev.AggregateID, editor)
		return WebhookPayload{}, true, errSelfMutation
	}

	return translateEventName(ev), true, nil
}
```

- [ ] **Step 4: Update `translateEventName` and `mapGrantEvent` to use the new field names**

Replace `webhook_translate.go:101-140` (the `translateEventName` function and `mapGrantEvent`) with:

```go
func translateEventName(ev zitadelEventPayload) WebhookPayload {
	base := WebhookPayload{UserID: ev.AggregateID}
	switch ev.EventType {
	case "user.human.added", "user.human.selfregistered":
		base.EventType = "user_created"
	case "user.deactivated":
		base.EventType = "user_deactivated"
	case "user.locked":
		base.EventType = "user_locked"
	case "user.grant.added", "user.user.grant.added":
		return mapGrantEvent("grant_added", ev)
	case "user.grant.changed", "user.user.grant.changed":
		return mapGrantEvent("grant_changed", ev)
	case "user.grant.removed", "user.user.grant.removed":
		return mapGrantEvent("grant_removed", ev)
	default:
		log.Printf("[WEBHOOK] unknown event=%s aggregate=%s — ignoring", ev.EventType, ev.AggregateID)
		return WebhookPayload{}
	}
	return base
}

func mapGrantEvent(eventType string, ev zitadelEventPayload) WebhookPayload {
	var grant userGrantPayload
	_ = json.Unmarshal(ev.EventPayload, &grant)
	out := WebhookPayload{
		EventType:     eventType,
		UserID:        firstNonEmpty(grant.UserID, ev.AggregateID),
		SourceProject: grant.ProjectID,
		RoleKeys:      grant.RoleKeys,
	}
	// GrantID surfaces as the aggregate ID for user.grant.* events.
	// Stash it via SourceProject's sibling fields when present so the
	// enrichment step can correlate misses to a specific grant aggregate.
	out.GrantID = ev.AggregateID
	if len(out.RoleKeys) > 0 {
		out.RoleKey = out.RoleKeys[0]
	}
	if grant.ProjectID != "" {
		out.ProjectIDs = []string{grant.ProjectID}
	}
	return out
}
```

This introduces a new `GrantID` field on `WebhookPayload`. Add it next to `RoleKey`/`RoleKeys` in `WebhookPayload` (probably in `backend/internal/handlers/webhook.go` or wherever the type is defined — search and add):

```go
GrantID string `json:"grant_id,omitempty"`
```

- [ ] **Step 5: Run the new translator test**

```bash
cd backend && go test ./internal/handlers/ -run TestTranslateZitadelEvent_RealWireFormat_GrantAdded -count=1
```

Expected: PASS.

- [ ] **Step 6: Run the whole handlers package — the OLD tests will fail (expected)**

```bash
cd backend && go test ./internal/handlers/ -count=1 2>&1 | tail -40
```

Expected: many failures in `webhook_translate_test.go`, `webhook_test.go`, `contracts_test.go` because their fixtures still use the wrong shape. Note the failing test names — Task 2 fixes them. Don't fix anything else here.

- [ ] **Step 7: Skip-commit (per user instruction). For your reference, the boundary commit would be:**

```text
fix(webhook): translator wire format — match Zitadel ContextInfoEvent
```

---

### Task 2: Rewrite translator unit tests against the real wire format

**Files:**
- Modify: `backend/internal/handlers/webhook_translate_test.go`

- [ ] **Step 1: Delete the existing wrong-shape test cases**

Open `backend/internal/handlers/webhook_translate_test.go`. Locate the `cases` table and the body-construction line at the line ~58 that does `body := []byte(`{"aggregate":{"id":"` + ...)`. Delete the whole table-driven test that depends on the wrong shape. Keep any helper functions and imports.

- [ ] **Step 2: Write the new table-driven test with realistic Zitadel bodies**

Add this in place of the deleted block:

```go
func TestTranslateZitadelEvent_RealWireFormat(t *testing.T) {
	// Each case is a real-shape Zitadel ContextInfoEvent body that the
	// listener will see in production. UserID at top-level is always
	// the editor; the subject (for grant events) lives in event_payload.
	cases := []struct {
		name        string
		eventType   string
		aggID       string
		editorID    string
		payloadJSON string
		wantType    string
		wantUserID  string
		wantProject string
		wantRoles   []string
		wantGrantID string
	}{
		{"user added", "user.human.added", "u1", "editor-1", `{"userName":"u1@example"}`, "user_created", "u1", "", nil, ""},
		{"self registered", "user.human.selfregistered", "u2", "editor-2", `{"userName":"u2@example"}`, "user_created", "u2", "", nil, ""},
		{"deactivated", "user.deactivated", "u3", "editor-3", `null`, "user_deactivated", "u3", "", nil, ""},
		{"locked", "user.locked", "u4", "editor-4", `null`, "user_locked", "u4", "", nil, ""},
		{"grant added", "user.grant.added", "g-aggr-1", "editor-5", `{"userId":"u5","projectId":"p1","grantId":"g-aggr-1","roleKeys":["alpha","beta"]}`, "grant_added", "u5", "p1", []string{"alpha", "beta"}, "g-aggr-1"},
		{"grant changed (no projectId — Zitadel real shape)", "user.grant.changed", "g-aggr-2", "editor-6", `{"userId":"u6","roleKeys":["gamma"]}`, "grant_changed", "u6", "", []string{"gamma"}, "g-aggr-2"},
		{"grant removed (no roleKeys — Zitadel real shape)", "user.grant.removed", "g-aggr-3", "editor-7", `{"userId":"u7","projectId":"p3","grantId":"g-aggr-3"}`, "grant_removed", "u7", "p3", nil, "g-aggr-3"},
		{"unknown", "user.password.changed", "u8", "editor-8", `{}`, "", "", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(fmt.Sprintf(
				`{"aggregateID":%q,"aggregateType":"user","resourceOwner":"org","instanceID":"inst","version":"v1","sequence":1,"event_type":%q,"created_at":"2026-05-07T00:00:00Z","userID":%q,"event_payload":%s}`,
				tc.aggID, tc.eventType, tc.editorID, tc.payloadJSON,
			))
			got, ok, err := translateZitadelEvent(body)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !ok {
				t.Fatalf("ok = false, want true")
			}
			if got.EventType != tc.wantType {
				t.Errorf("EventType = %q, want %q", got.EventType, tc.wantType)
			}
			if got.UserID != tc.wantUserID {
				t.Errorf("UserID = %q, want %q", got.UserID, tc.wantUserID)
			}
			if got.SourceProject != tc.wantProject {
				t.Errorf("SourceProject = %q, want %q", got.SourceProject, tc.wantProject)
			}
			if !reflect.DeepEqual(got.RoleKeys, tc.wantRoles) {
				t.Errorf("RoleKeys = %v, want %v", got.RoleKeys, tc.wantRoles)
			}
			if got.GrantID != tc.wantGrantID {
				t.Errorf("GrantID = %q, want %q", got.GrantID, tc.wantGrantID)
			}
		})
	}
}
```

(Ensure `"fmt"` and `"reflect"` are in the imports.)

- [ ] **Step 3: Add a self-mutation-guard test using the new editor location**

```go
func TestTranslateZitadelEvent_SelfMutationGuard_RealWireFormat(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "m2m-bot-id")
	body := []byte(`{
		"aggregateID":"u1","aggregateType":"user","resourceOwner":"org","instanceID":"inst",
		"version":"v1","sequence":1,"event_type":"user.grant.added","created_at":"2026-05-07T00:00:00Z",
		"userID":"m2m-bot-id",
		"event_payload":{"userId":"u1","projectId":"p1","roleKeys":["x"]}
	}`)
	_, isZitadel, err := translateZitadelEvent(body)
	if !isZitadel {
		t.Fatalf("isZitadel = false, want true (Zitadel-shape body must be recognized even when guard fires)")
	}
	if err != errSelfMutation {
		t.Fatalf("err = %v, want errSelfMutation", err)
	}
}
```

- [ ] **Step 4: Delete the old `TestTranslateZitadelEvent_SelfMutation_*` if it tests the old field locations**

Search for any older self-mutation test. If it constructs a body with `"editorUserId"` or `"editor":{"userId":...}` at the top, delete it — those locations no longer exist.

- [ ] **Step 5: Run translator tests**

```bash
cd backend && go test ./internal/handlers/ -run 'TestTranslateZitadelEvent' -count=1
```

Expected: PASS.

- [ ] **Step 6: Skip-commit. Boundary commit message would be:**

```text
test(webhook): rewrite translator tests against Zitadel real wire format
```

---

### Task 3: Fix `webhook_test.go` and `contracts_test.go` Zitadel-shape fixtures

**Files:**
- Modify: `backend/internal/handlers/webhook_test.go:540-660`
- Modify: `backend/internal/handlers/contracts_test.go` (only if it has Zitadel-shape bodies)

- [ ] **Step 1: Locate the Zitadel-shape body construction sites**

```bash
grep -n '"aggregate":\|"event":' backend/internal/handlers/webhook_test.go backend/internal/handlers/contracts_test.go
```

Each match is a body that was using the wrong shape.

- [ ] **Step 2: Rewrite each body to the real Zitadel shape**

Pattern transform:

```diff
-{
-  "aggregate": {"id":"<id>","type":"<type>","resourceOwner":"<org>"},
-  "event": "<event-type>",
-  "editorUserId": "<editor>",
-  "payload": {<inner>}
-}
+{
+  "aggregateID":"<id>",
+  "aggregateType":"<type>",
+  "resourceOwner":"<org>",
+  "instanceID":"inst",
+  "version":"v1",
+  "sequence":1,
+  "event_type":"<event-type>",
+  "created_at":"2026-05-07T00:00:00Z",
+  "userID":"<editor>",
+  "event_payload":{<inner>}
+}
```

Apply this everywhere a Zitadel-shape body appears. For lifecycle event tests (`user.deactivated`, `user.locked`) the inner payload should be `null`.

For the test at `webhook_test.go:613` that uses `fmt.Sprintf` — update both the format string and any embedded field names.

- [ ] **Step 3: Run the affected tests**

```bash
cd backend && go test ./internal/handlers/ -run 'TestHandleZitadelWebhook' -count=1
```

Expected: PASS for tests where the inner payload contains everything (`grant.added`, lifecycle). Expected: FAIL for `grant.changed` and `grant.removed` tests because the handler still requires the missing fields — those are Task 11's territory. If those tests exist, mark them with `t.Skip("requires enrichment — see Task 11")` for now so the green bar isolates the unrelated tests.

- [ ] **Step 4: Run the full handlers package**

```bash
cd backend && go test ./internal/handlers/ -count=1 2>&1 | tail -20
```

Expected: PASS (any explicitly-skipped tests are fine; no failures).

- [ ] **Step 5: Skip-commit. Boundary commit message would be:**

```text
test(webhook): align handler tests with Zitadel real wire format
```

---

### Task 4: Repair the smoke-test script

**Files:**
- Modify: `scripts/smoke-test-event-listener.sh:55` (the `PAYLOAD=` block)

- [ ] **Step 1: Replace the synthetic payload**

Open the script. Find the line starting `PAYLOAD='{"aggregate":...`. Replace the assignment with:

```bash
PAYLOAD='{"aggregateID":"smoke-user-1","aggregateType":"user","resourceOwner":"org","instanceID":"inst","version":"v1","sequence":1,"event_type":"user.password.changed","created_at":"2026-05-07T00:00:00Z","userID":"smoke-editor","event_payload":{}}'
```

The event type stays `user.password.changed` so the translator's unknown-event passthrough still fires (200 OK, log line, no dispatch — safe against staging and production state).

- [ ] **Step 2: Run the smoke test against a local backend**

In one terminal, start the backend (or rely on a running container). In another:

```bash
make zitadel-actions-verify-events
```

Expected: HTTP 200 + a log line `[WEBHOOK] unknown event=user.password.changed aggregate=smoke-user-1 — ignoring`.

- [ ] **Step 3: Skip-commit. Boundary commit message would be:**

```text
test(smoke): event-listener smoke uses real Zitadel wire format
```

---

### Task 5: Manual end-to-end verification of `grant_added` and lifecycle events

This is a verification gate, not a code task. The wire-format fix should make `grant.added` flow end-to-end without the index. Confirm before continuing.

- [ ] **Step 1: Restart the backend so it picks up the new binary**

```bash
docker compose up -d --build backend
```

- [ ] **Step 2: Tail the backend log**

```bash
docker compose logs -f backend | grep -E '\[WEBHOOK\]|\[ACTION\]'
```

- [ ] **Step 3: In Zitadel Console, assign a new role grant to a test user (a user with no existing grants — emits `grant.added`)**

Expected log line:

```
[WEBHOOK] Event received: type=grant_added user=<id> project=<projectId> role=<role>
```

And in `/operations`, a row appears within 5 seconds.

- [ ] **Step 4: In Zitadel Console, deactivate a test user**

Expected log line:

```
[WEBHOOK] Event received: type=user_deactivated user=<id> project= role=
```

- [ ] **Step 5: Document the result**

If steps 3-4 pass, proceed to Task 6. If they fail, STOP — go back and read the log carefully. Common causes: backend container didn't pick up the rebuild; M2M user accidentally became the editor (re-check `ZITADEL_M2M_USER_ID`); 500 from the processor (DB issue, not translator).

---

### Task 6: Add the `zitadel_grants_index` migration

**Files:**
- Create: `backend/db/migrations/000011_zitadel_grants_index.up.sql`
- Create: `backend/db/migrations/000011_zitadel_grants_index.down.sql`

- [ ] **Step 1: Write the up migration**

`backend/db/migrations/000011_zitadel_grants_index.up.sql`:

```sql
-- Local index of Zitadel user-grant aggregates.
-- Populated from user.grant.added webhook events; consulted by the translator
-- to enrich user.grant.changed (missing projectId) and user.grant.removed
-- (missing roleKeys) before validation, since Zitadel's wire format omits
-- those fields on those event types. On a miss, the translator falls back
-- to a synchronous Zitadel ListUserGrants call.
CREATE TABLE IF NOT EXISTS zitadel_grants_index (
    grant_id     TEXT PRIMARY KEY,                      -- Zitadel user_grant aggregate ID
    user_id      TEXT NOT NULL,
    project_id   TEXT NOT NULL,
    role_keys    TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS zitadel_grants_index_user_id_idx
    ON zitadel_grants_index (user_id);

CREATE INDEX IF NOT EXISTS zitadel_grants_index_project_id_idx
    ON zitadel_grants_index (project_id);
```

- [ ] **Step 2: Write the down migration**

`backend/db/migrations/000011_zitadel_grants_index.down.sql`:

```sql
DROP TABLE IF EXISTS zitadel_grants_index;
```

- [ ] **Step 3: Run the migration locally**

```bash
docker compose exec backend /app/migrate up   # adjust to whatever the project's migration runner is
```

(If migrations are auto-applied on backend startup, restart the backend instead and tail the log to see the migration apply.)

- [ ] **Step 4: Verify the table exists**

```bash
docker compose exec db psql -U "${POSTGRES_USER:-syndra}" -d "${POSTGRES_DB:-syndra}" -c '\d zitadel_grants_index'
```

Expected: shows columns `grant_id`, `user_id`, `project_id`, `role_keys`, `created_at`, `updated_at`.

- [ ] **Step 5: Skip-commit. Boundary commit message would be:**

```text
feat(db): add zitadel_grants_index for grant.changed/removed enrichment
```

---

### Task 7: Add repository functions for the grants index

**Files:**
- Modify: `backend/internal/db/repositories.go` (append three functions)
- Modify: `backend/internal/db/models.go` (or wherever types live — add `ZitadelGrantIndex`)

- [ ] **Step 1: Add the model**

In the same file as the other `WebhookEvent`-style structs:

```go
// ZitadelGrantIndex is the local cache of Zitadel user_grant aggregates,
// keyed by grant aggregate ID. Populated from grant.added events; used
// to enrich grant.changed (missing projectId) and grant.removed (missing
// roleKeys) before handler validation runs.
type ZitadelGrantIndex struct {
	GrantID   string    `json:"grant_id"`
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	RoleKeys  []string  `json:"role_keys"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Write the failing test for `UpsertGrantIndex`**

Append to `backend/internal/db/repositories_test.go` (or wherever similar tests live — match the existing style):

```go
func TestUpsertGrantIndex_RoundTrip(t *testing.T) {
	// Uses the same test DB harness as the rest of repositories_test.go.
	ctx := t.Context()
	cleanupDB(t)

	if err := UpsertGrantIndex(ctx, "g1", "u1", "p1", []string{"alpha", "beta"}); err != nil {
		t.Fatalf("UpsertGrantIndex initial: %v", err)
	}
	got, err := GetGrantIndex(ctx, "g1")
	if err != nil {
		t.Fatalf("GetGrantIndex: %v", err)
	}
	if got.UserID != "u1" || got.ProjectID != "p1" {
		t.Errorf("got %+v", got)
	}
	if !reflect.DeepEqual(got.RoleKeys, []string{"alpha", "beta"}) {
		t.Errorf("RoleKeys = %v", got.RoleKeys)
	}

	// Upsert again with new role keys — should overwrite, not duplicate.
	if err := UpsertGrantIndex(ctx, "g1", "u1", "p1", []string{"gamma"}); err != nil {
		t.Fatalf("UpsertGrantIndex update: %v", err)
	}
	got, _ = GetGrantIndex(ctx, "g1")
	if !reflect.DeepEqual(got.RoleKeys, []string{"gamma"}) {
		t.Errorf("after update RoleKeys = %v, want [gamma]", got.RoleKeys)
	}

	// Delete.
	if err := DeleteGrantIndex(ctx, "g1"); err != nil {
		t.Fatalf("DeleteGrantIndex: %v", err)
	}
	_, err = GetGrantIndex(ctx, "g1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}
```

(Ensure `"reflect"` and `"errors"` imports exist; if there's no `ErrNotFound` sentinel yet, define it next to the function or reuse whatever the repo uses — see existing `Get*` functions in the file.)

- [ ] **Step 3: Run the failing test**

```bash
cd backend && go test ./internal/db/ -run TestUpsertGrantIndex_RoundTrip -count=1
```

Expected: FAIL — undefined `UpsertGrantIndex`.

- [ ] **Step 4: Implement the three repository functions**

Append to `backend/internal/db/repositories.go`:

```go
// UpsertGrantIndex inserts or updates the local cache row for a Zitadel
// user_grant aggregate. Called from the grant.added processor; the row
// lets later grant.changed/removed events fill the projectId/roleKeys
// fields Zitadel omits from those payloads.
func UpsertGrantIndex(ctx context.Context, grantID, userID, projectID string, roleKeys []string) error {
	if grantID == "" || userID == "" || projectID == "" {
		return fmt.Errorf("UpsertGrantIndex: grant_id, user_id, project_id are required")
	}
	const q = `
		INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (grant_id) DO UPDATE SET
			user_id    = EXCLUDED.user_id,
			project_id = EXCLUDED.project_id,
			role_keys  = EXCLUDED.role_keys,
			updated_at = now()
	`
	_, err := pool.Exec(ctx, q, grantID, userID, projectID, roleKeys)
	if err != nil {
		return fmt.Errorf("UpsertGrantIndex: %w", err)
	}
	return nil
}

// GetGrantIndex fetches the cached row by grant aggregate ID. Returns
// ErrNotFound when the grant has never been seen by an `added` event.
func GetGrantIndex(ctx context.Context, grantID string) (ZitadelGrantIndex, error) {
	var row ZitadelGrantIndex
	const q = `
		SELECT grant_id, user_id, project_id, role_keys, created_at, updated_at
		FROM zitadel_grants_index
		WHERE grant_id = $1
	`
	err := pool.QueryRow(ctx, q, grantID).Scan(
		&row.GrantID, &row.UserID, &row.ProjectID, &row.RoleKeys, &row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ZitadelGrantIndex{}, ErrNotFound
	}
	if err != nil {
		return ZitadelGrantIndex{}, fmt.Errorf("GetGrantIndex: %w", err)
	}
	return row, nil
}

// DeleteGrantIndex removes the cached row. Called from the grant.removed
// processor after the downstream effects (revoke, cache invalidation)
// have run successfully — failing to delete is non-fatal (the next
// reconciliation will clean it up).
func DeleteGrantIndex(ctx context.Context, grantID string) error {
	const q = `DELETE FROM zitadel_grants_index WHERE grant_id = $1`
	if _, err := pool.Exec(ctx, q, grantID); err != nil {
		return fmt.Errorf("DeleteGrantIndex: %w", err)
	}
	return nil
}
```

(Adjust `pool` to whatever the existing repository functions in this file use — many files use a package-level `pool`; some take a `*pgxpool.Pool`. Match the surrounding style. If `ErrNotFound` doesn't exist, mirror whatever `GetWebhookEvents` returns on no-row.)

- [ ] **Step 5: Run the test**

```bash
cd backend && go test ./internal/db/ -run TestUpsertGrantIndex_RoundTrip -count=1
```

Expected: PASS.

- [ ] **Step 6: Skip-commit. Boundary commit message would be:**

```text
feat(db): grants-index repository — UpsertGrantIndex/GetGrantIndex/DeleteGrantIndex
```

---

### Task 8: Add injectable seams to `deps.go`

**Files:**
- Modify: `backend/internal/handlers/deps.go`

- [ ] **Step 1: Add four new injectable function variables**

After the existing `dbInsertWebhookEvent` line:

```go
var (
	dbUpsertGrantIndex   = db.UpsertGrantIndex
	dbGetGrantIndex      = db.GetGrantIndex
	dbDeleteGrantIndex   = db.DeleteGrantIndex
	dbListUserGrantsLive = listUserGrantsViaZitadel // see Task 9
)
```

`listUserGrantsViaZitadel` will be defined in the next task. The seam exists so tests can stub it without touching the real Zitadel client.

- [ ] **Step 2: Compile-check**

```bash
cd backend && go build ./...
```

Expected: FAIL — `undefined: listUserGrantsViaZitadel`. That's fine; Task 9 defines it.

- [ ] **Step 3: Skip-commit. Boundary commit message would be:**

```text
chore(handlers/deps): seams for grants index + Zitadel grant lookup
```

---

### Task 9: Wire a Zitadel-side grant lookup helper

**Files:**
- Create: `backend/internal/handlers/zitadel_grant_lookup.go`
- Test: `backend/internal/handlers/zitadel_grant_lookup_test.go`

- [ ] **Step 1: Write the failing test**

`backend/internal/handlers/zitadel_grant_lookup_test.go`:

```go
package handlers

import (
	"context"
	"testing"

	"syndra/internal/zitadel"
)

type stubZitadelClient struct {
	grants []zitadel.UserGrant
	err    error
}

func (s *stubZitadelClient) ListUserGrants(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	if s.err != nil {
		return nil, s.err
	}
	return &zitadel.SearchResult[zitadel.UserGrant]{Items: s.grants, Total: int64(len(s.grants))}, nil
}

func TestListUserGrantsViaZitadel_FindsByGrantID(t *testing.T) {
	prev := zitadelClient
	t.Cleanup(func() { zitadelClient = prev })
	zitadelClient = &stubZitadelClient{
		grants: []zitadel.UserGrant{
			{ID: "g-other", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"a"}},
			{ID: "g-target", UserID: "u1", ProjectID: "p2", RoleKeys: []string{"b", "c"}},
		},
	}
	got, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-target")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.ProjectID != "p2" {
		t.Errorf("ProjectID = %q, want p2", got.ProjectID)
	}
	if len(got.RoleKeys) != 2 {
		t.Errorf("RoleKeys = %v", got.RoleKeys)
	}
}

func TestListUserGrantsViaZitadel_GrantNotFound(t *testing.T) {
	prev := zitadelClient
	t.Cleanup(func() { zitadelClient = prev })
	zitadelClient = &stubZitadelClient{grants: nil}
	_, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-missing")
	if err == nil {
		t.Fatalf("err = nil, want grant-not-found")
	}
}
```

- [ ] **Step 2: Run — must fail (undefined symbols)**

```bash
cd backend && go test ./internal/handlers/ -run TestListUserGrantsViaZitadel -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement**

`backend/internal/handlers/zitadel_grant_lookup.go`:

```go
package handlers

import (
	"context"
	"fmt"

	"syndra/internal/zitadel"
)

// zitadelGrantLookup is the subset of the Zitadel management client used
// by listUserGrantsViaZitadel. Defined as a small interface so tests can
// stub it without standing up an HTTP fake.
type zitadelGrantLookup interface {
	ListUserGrants(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error)
}

// zitadelClient is the package-level handle the lookup uses. Wired to
// the real management client at startup (see whatever bootstraps the
// handlers package today — search for `zitadelClient =`). Tests
// override it with a stub via the deps.go seam.
var zitadelClient zitadelGrantLookup

// listUserGrantsViaZitadel is the fallback path when the local
// grants_index has no row for a grant aggregate ID. Calls the Zitadel
// management API to list the user's grants and finds the one matching
// the grant ID. Returns an error if the user has no grants or the
// matching grant isn't found — caller MUST handle gracefully (best-
// effort processing; a missed enrichment shouldn't 500 the webhook).
func listUserGrantsViaZitadel(ctx context.Context, userID, grantID string) (zitadel.UserGrant, error) {
	if zitadelClient == nil {
		return zitadel.UserGrant{}, fmt.Errorf("zitadel client unset")
	}
	res, err := zitadelClient.ListUserGrants(ctx, userID, zitadel.SearchParams{Limit: 100})
	if err != nil {
		return zitadel.UserGrant{}, fmt.Errorf("list user grants for user=%s: %w", userID, err)
	}
	for _, g := range res.Items {
		if g.ID == grantID {
			return g, nil
		}
	}
	return zitadel.UserGrant{}, fmt.Errorf("grant %s not found among %d grants for user %s", grantID, len(res.Items), userID)
}
```

- [ ] **Step 4: Wire `zitadelClient` to the real client at startup**

Find the bootstrap site (probably `backend/cmd/api/main.go` or `backend/internal/handlers/setup.go`) where the existing handlers get a Zitadel client. Add:

```go
zitadelClient = realZitadelClient   // adjust to the variable name in scope
```

- [ ] **Step 5: Run the tests**

```bash
cd backend && go test ./internal/handlers/ -run 'TestListUserGrantsViaZitadel|TestUpsertGrantIndex' -count=1 && cd backend && go build ./...
```

Expected: PASS + clean build.

- [ ] **Step 6: Skip-commit. Boundary commit message would be:**

```text
feat(handlers): zitadel grant lookup helper for index-miss fallback
```

---

### Task 10: Implement the enrichment function

**Files:**
- Create: `backend/internal/handlers/webhook_translate_enrich.go`
- Test: `backend/internal/handlers/webhook_translate_enrich_test.go`

- [ ] **Step 1: Write the test cases**

`backend/internal/handlers/webhook_translate_enrich_test.go`:

```go
package handlers

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

func setupEnrichDeps(t *testing.T) {
	t.Helper()
	prevGet := dbGetGrantIndex
	prevList := dbListUserGrantsLive
	t.Cleanup(func() {
		dbGetGrantIndex = prevGet
		dbListUserGrantsLive = prevList
	})
}

func TestEnrichGrantPayload_LocalIndexHit_ChangedFillsProject(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, gid string) (db.ZitadelGrantIndex, error) {
		if gid != "g-1" {
			t.Fatalf("got grantID=%q", gid)
		}
		return db.ZitadelGrantIndex{GrantID: "g-1", UserID: "u-1", ProjectID: "p-77", RoleKeys: []string{"old"}}, nil
	}
	in := WebhookPayload{EventType: "grant_changed", UserID: "u-1", GrantID: "g-1", RoleKeys: []string{"new"}}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-77" {
		t.Errorf("SourceProject = %q, want p-77", out.SourceProject)
	}
	if len(out.RoleKeys) != 1 || out.RoleKeys[0] != "new" {
		t.Errorf("RoleKeys = %v, want [new] (the event's value, not the index's)", out.RoleKeys)
	}
}

func TestEnrichGrantPayload_LocalIndexHit_RemovedFillsBoth(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: "g-1", UserID: "u-1", ProjectID: "p-77", RoleKeys: []string{"alpha", "beta"}}, nil
	}
	in := WebhookPayload{EventType: "grant_removed", UserID: "u-1", GrantID: "g-1"} // no project, no roles
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-77" || len(out.RoleKeys) != 2 {
		t.Errorf("got %+v, want project p-77 + roles [alpha beta]", out)
	}
}

func TestEnrichGrantPayload_LocalMiss_ZitadelFallback(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrNotFound
	}
	dbListUserGrantsLive = func(_ context.Context, _, _ string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{ID: "g-1", UserID: "u-1", ProjectID: "p-99", RoleKeys: []string{"x"}}, nil
	}
	in := WebhookPayload{EventType: "grant_removed", UserID: "u-1", GrantID: "g-1"}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-99" {
		t.Errorf("SourceProject = %q, want p-99", out.SourceProject)
	}
}

func TestEnrichGrantPayload_BothFail_LeavesPayloadUnenriched(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrNotFound
	}
	dbListUserGrantsLive = func(_ context.Context, _, _ string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{}, errors.New("zitadel down")
	}
	in := WebhookPayload{EventType: "grant_changed", UserID: "u-1", GrantID: "g-1", RoleKeys: []string{"x"}}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "" {
		t.Errorf("SourceProject = %q, want empty (both lookups failed)", out.SourceProject)
	}
	// Roles from the event are preserved.
	if len(out.RoleKeys) != 1 || out.RoleKeys[0] != "x" {
		t.Errorf("RoleKeys = %v, want [x]", out.RoleKeys)
	}
}

func TestEnrichGrantPayload_NonGrantEvent_NoOp(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		t.Fatalf("must not call index for non-grant events")
		return db.ZitadelGrantIndex{}, nil
	}
	in := WebhookPayload{EventType: "user_created", UserID: "u-1"}
	out := enrichGrantPayload(context.Background(), in)
	if out != in {
		t.Errorf("payload mutated for non-grant event")
	}
}
```

- [ ] **Step 2: Run — must fail (undefined `enrichGrantPayload`)**

```bash
cd backend && go test ./internal/handlers/ -run 'TestEnrichGrantPayload' -count=1
```

Expected: FAIL.

- [ ] **Step 3: Implement**

`backend/internal/handlers/webhook_translate_enrich.go`:

```go
package handlers

import (
	"context"
	"errors"
	"log"

	"syndra/internal/db"
)

// enrichGrantPayload fills in the projectId (and roleKeys for grant_removed)
// that Zitadel omits from grant.changed and grant.removed event payloads.
//
// Strategy:
//  1. Look up the grant aggregate in the local grants index (populated by
//     prior grant.added events).
//  2. On a miss, fall back to a synchronous Zitadel ListUserGrants call.
//  3. If both fail, return the payload unmodified — handler validation will
//     reject it with 400, OR the handler MAY relax validation for these
//     events. Logging is at the WARN level so operators can spot index
//     drift in /operations failures.
//
// Roles from the event itself always win over index roles for grant_changed
// (the event represents the new state). For grant_removed, the event has no
// roles, so the index roles fill in.
func enrichGrantPayload(ctx context.Context, p WebhookPayload) WebhookPayload {
	if p.EventType != "grant_changed" && p.EventType != "grant_removed" {
		return p
	}
	if p.GrantID == "" {
		log.Printf("[WEBHOOK] enrich: missing GrantID, skipping enrichment event_type=%s", p.EventType)
		return p
	}
	needsProject := p.SourceProject == ""
	needsRoles := len(p.RoleKeys) == 0
	if !needsProject && !needsRoles {
		return p
	}

	// 1) Local index.
	idx, err := dbGetGrantIndex(ctx, p.GrantID)
	if err == nil {
		if needsProject {
			p.SourceProject = idx.ProjectID
			p.ProjectIDs = []string{idx.ProjectID}
		}
		if needsRoles {
			p.RoleKeys = idx.RoleKeys
			if len(p.RoleKeys) > 0 {
				p.RoleKey = p.RoleKeys[0]
			}
		}
		return p
	}
	if !errors.Is(err, db.ErrNotFound) {
		log.Printf("[WEBHOOK] enrich: index lookup failed grant=%s: %v — falling back to Zitadel", p.GrantID, err)
	}

	// 2) Zitadel API fallback.
	live, lerr := dbListUserGrantsLive(ctx, p.UserID, p.GrantID)
	if lerr != nil {
		log.Printf("[WEBHOOK] enrich: index miss + zitadel lookup failed grant=%s user=%s: %v — payload left unenriched", p.GrantID, p.UserID, lerr)
		return p
	}
	if needsProject {
		p.SourceProject = live.ProjectID
		p.ProjectIDs = []string{live.ProjectID}
	}
	if needsRoles {
		p.RoleKeys = live.RoleKeys
		if len(p.RoleKeys) > 0 {
			p.RoleKey = p.RoleKeys[0]
		}
	}
	return p
}
```

- [ ] **Step 4: Run the tests**

```bash
cd backend && go test ./internal/handlers/ -run 'TestEnrichGrantPayload' -count=1
```

Expected: PASS.

- [ ] **Step 5: Skip-commit. Boundary commit message would be:**

```text
feat(webhook): enrichment for grant.changed/removed (local index + zitadel fallback)
```

---

### Task 11: Wire enrichment into the handler pipeline

**Files:**
- Modify: `backend/internal/handlers/webhook.go:48-70` (between translator and validation)
- Modify: `backend/internal/handlers/webhook.go:166-183` (dispatch — hook index upsert/delete)

- [ ] **Step 1: Insert the enrichment call**

After the existing `if isZitadel { ... event = translated } else { ... }` block (around line 70), add:

```go
event = enrichGrantPayload(r.Context(), event)
```

This runs only on the Zitadel-shape path (the internal-shape path is for operator curl / contract tests and is expected to provide all fields).

- [ ] **Step 2: Update the dispatch switch to maintain the index**

Replace the dispatch block (around `webhook.go:165-183`) with:

```go
// Dispatch by event type
var processingErr error
switch event.EventType {
case "grant_added":
	processingErr = processGrantAdded(r.Context(), event, eventID)
	if processingErr == nil && event.GrantID != "" {
		if uerr := dbUpsertGrantIndex(r.Context(), event.GrantID, event.UserID, event.SourceProject, event.RoleKeys); uerr != nil {
			log.Printf("[WEBHOOK] index upsert failed grant=%s: %v (non-fatal)", event.GrantID, uerr)
		}
	}
case "grant_changed":
	processingErr = processGrantAdded(r.Context(), event, eventID)
	if processingErr == nil && event.GrantID != "" {
		// Update the index with the new role set so future events see fresh data.
		if uerr := dbUpsertGrantIndex(r.Context(), event.GrantID, event.UserID, event.SourceProject, event.RoleKeys); uerr != nil {
			log.Printf("[WEBHOOK] index update failed grant=%s: %v (non-fatal)", event.GrantID, uerr)
		}
	}
case "grant_removed":
	processingErr = processGrantRemoved(r.Context(), event, eventID)
	if processingErr == nil && event.GrantID != "" {
		if derr := dbDeleteGrantIndex(r.Context(), event.GrantID); derr != nil {
			log.Printf("[WEBHOOK] index delete failed grant=%s: %v (non-fatal)", event.GrantID, derr)
		}
	}
case "user_deactivated", "user_locked":
	processingErr = processUserDeactivated(r.Context(), event)
case "user_created":
	processingErr = processUserCreated(r.Context(), event)
}
```

The index ops are all non-fatal — failure logs but doesn't break webhook processing. The index is a cache, not the source of truth.

- [ ] **Step 3: Un-skip the grant.changed / grant.removed handler tests from Task 3**

```bash
grep -rn 't.Skip("requires enrichment' backend/internal/handlers/
```

Remove each `t.Skip` and re-run those tests.

- [ ] **Step 4: Run the full handlers package**

```bash
cd backend && go test ./internal/handlers/ -count=1
```

Expected: PASS.

- [ ] **Step 5: Skip-commit. Boundary commit message would be:**

```text
feat(webhook): wire enrichment + maintain grants index across grant lifecycle
```

---

### Task 12: Manual end-to-end verification of all subscribed events

This validates the full plan against a live Zitadel instance.

- [ ] **Step 1: Restart backend with the rebuilt binary**

```bash
docker compose up -d --build backend
```

- [ ] **Step 2: Tail backend logs**

```bash
docker compose logs -f backend | grep -E '\[WEBHOOK\]|\[ACTION\]|enrich|index'
```

- [ ] **Step 3: Verify each event type**

In Zitadel Console, perform each operation on test users and confirm the matching `[WEBHOOK] Event received` log line and a row in `/operations`:

| Console action | Expected log + /operations row |
|---|---|
| Create a new human user | `type=user_created user=<id>` |
| Self-register a new user (if enabled) | `type=user_created user=<id>` |
| Deactivate a user | `type=user_deactivated user=<id>` |
| Lock a user | `type=user_locked user=<id>` |
| Add a brand-new role grant | `type=grant_added user=<id> project=<p> role=<r>` + new row in `zitadel_grants_index` |
| Add a second role to that grant (changed) | `type=grant_changed user=<id> project=<p> role=<r>` (project filled by index) |
| Remove the entire grant | `type=grant_removed user=<id> project=<p> role=<r>` (project + roles filled by index) + row gone from `zitadel_grants_index` |

- [ ] **Step 4: Confirm `zitadel_grants_index` is being maintained**

```bash
docker compose exec db psql -U "${POSTGRES_USER:-syndra}" -d "${POSTGRES_DB:-syndra}" -c \
  "SELECT grant_id, user_id, project_id, role_keys, updated_at FROM zitadel_grants_index ORDER BY updated_at DESC LIMIT 10;"
```

- [ ] **Step 5: Force the Zitadel-API fallback path**

Manually delete a row from `zitadel_grants_index` for an active grant. Then change that grant in Zitadel Console. Watch for:

```
[WEBHOOK] enrich: index miss + zitadel lookup ... — payload left unenriched   (NO — bad)
```

…or, ideally, no warning at all (Zitadel API filled in the missing field). Either way, the event should still 200 (best-effort) and a row should appear in `/operations` (with `source_project` empty if Zitadel API also failed).

---

### Task 13: Update OpenSpec docs to record the wire-format correction and the index

**Files:**
- Modify: `openspec/changes/zitadel-event-trigger-propagation/specs/application-claims/spec.md`
- Modify: `openspec/changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md`
- Modify: `openspec/changes/zitadel-event-trigger-propagation/IMPLEMENTATION.md`
- Modify: `openspec/changes/zitadel-event-trigger-propagation/tasks.md`
- Modify: `zitadel/actions/EVENTS.md`

- [ ] **Step 1: `IMPLEMENTATION.md` — record the wire-format correction**

In the "Backend (`backend/internal/handlers/`)" section, add a sub-bullet under the `webhook_translate.go` line:

```markdown
- **Wire-format correction (post-merge fix)**: the original `zitadelEventPayload` struct was built against a guessed shape (`{aggregate:{id,...}, event, payload, editorUserId}`) that does not match Zitadel's actual `ContextInfoEvent` (`internal/repository/execution/queue.go`). All real Zitadel events 4xx'd at validation; only the (incorrectly-shaped) smoke test passed. The struct now matches Zitadel exactly: top-level `aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID` (the editor). Shape detection probes `aggregateID` (not `aggregate`). Editor location collapsed to a single field. Smoke-test script and unit/handler test fixtures updated to the real shape. See `docs/superpowers/plans/2026-05-07-zitadel-event-listener-wire-format-fix.md`.
- **Grants index (`zitadel_grants_index` table, migration 000011)**: Zitadel's `user.grant.changed` payload omits `projectId`; `user.grant.removed` omits `roleKeys`. The translator now enriches both via a local index populated by `grant.added` (and refreshed by `grant.changed`), with a synchronous Zitadel `ListUserGrants` fallback on local-index miss. Index ops are non-fatal — failures log but never bounce the webhook.
```

- [ ] **Step 2: `application-claims/spec.md` — strengthen the translator-coverage clause**

Find the section that lists the translator's per-event mapping (around line 33-39). Add this note before the bulleted mapping list:

```markdown
The translator MUST decode against Zitadel's actual `ContextInfoEvent` wire format (`zitadel/zitadel:internal/repository/execution/queue.go`): top-level flat fields `aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID` (editor). The shape detector MUST probe for `aggregateID`; bodies without that key MUST fall through to the internal-shape strict decoder. Smoke-test fixtures and unit-test bodies MUST use the real shape — the prior translator was built against a fictional `{aggregate:{id,...}, event, payload}` shape that no Zitadel-originated event actually emits, and tests passed only because they used the same fictional shape.
```

And add a new requirement after the multi-role grants clause:

```markdown
* **Grant enrichment**: `user.grant.changed` does not carry `projectId` and `user.grant.removed` does not carry `roleKeys` (verified against `internal/repository/usergrant/user_grant.go`). The translator MUST enrich these from a local `zitadel_grants_index` table (populated by `grant.added`, refreshed by `grant.changed`, deleted on `grant.removed`); on a local-index miss it MUST fall back to a synchronous Zitadel `ListUserGrants` call. Both lookups are best-effort: when both fail, the translator MUST return the unenriched payload and let the handler/processor degrade gracefully (do not 4xx Zitadel — that produces redelivery storms with no clean resolution).
```

- [ ] **Step 3: `lifecycle-event-propagation/spec.md` — same correction**

Append to the existing requirement paragraph:

```markdown
The producer-side payload follows Zitadel's `ContextInfoEvent` wire format; consumer-side translator MUST match exactly (see application-claims/spec.md "Grant enrichment").
```

- [ ] **Step 4: `tasks.md` — append a follow-up section**

At the end of the existing tasks list, append:

```markdown
## 9. Post-merge: wire-format correction + grants index
- [x] 9.1 Replace `zitadelEventPayload` struct with Zitadel's real `ContextInfoEvent` wire format
- [x] 9.2 Update shape detection probe key (`aggregate` → `aggregateID`)
- [x] 9.3 Collapse editor probe to top-level `userID`
- [x] 9.4 Rewrite translator + handler unit tests to use real wire format
- [x] 9.5 Update `scripts/smoke-test-event-listener.sh` synthetic body
- [x] 9.6 Add `zitadel_grants_index` migration (000011) + repository CRUD
- [x] 9.7 Add `enrichGrantPayload` (local-index → Zitadel ListUserGrants fallback)
- [x] 9.8 Wire enrichment between translator and handler validation
- [x] 9.9 Maintain index from grant.added/changed/removed dispatch
- [x] 9.10 Manual end-to-end verification against live Zitadel
```

- [ ] **Step 5: `EVENTS.md` — replace the example payload block with a real Zitadel-shape one**

Find any example body in EVENTS.md and replace with:

```json
{
  "aggregateID": "<grant aggregate id>",
  "aggregateType": "user_grant",
  "resourceOwner": "<orgID>",
  "instanceID": "<instanceID>",
  "version": "v1",
  "sequence": 42,
  "event_type": "user.grant.added",
  "created_at": "2026-05-07T17:35:46.464Z",
  "userID": "<editorUserID>",
  "event_payload": {
    "userId": "<subjectUserID>",
    "projectId": "<projectID>",
    "grantId": "<grant aggregate id>",
    "roleKeys": ["alpha","beta"]
  }
}
```

Add a paragraph above the example noting the field-name caveats (`aggregateID` not `aggregateId`, `userID` is the editor not the subject, snake_case for `event_type`/`event_payload`/`created_at`).

- [ ] **Step 6: Skip-commit. Boundary commit message would be:**

```text
docs(openspec): record event-listener wire-format fix + grants index
```

---

### Task 14: Refresh the codebase-memory graph

The wire-format and enrichment changes added new functions and a new table — keep the graph in sync per the project's mandatory workflow.

- [ ] **Step 1: Detect changes**

```bash
# Via the codebase-memory-mcp:
mcp__codebase-memory-mcp__detect_changes project=Users-notkanishk-Documents-Mkrspc-Projects-Syndra-backend since=HEAD
```

- [ ] **Step 2: If `impacted_symbols` is non-empty, re-index the affected scope**

Follow the project's existing re-index pattern (`mcp__codebase-memory-mcp__index_repository` with the appropriate scope).

- [ ] **Step 3: Update any ADR if architectural decisions shifted**

Specifically: the decision to use an event-derived local index with Zitadel fallback (rather than always querying Zitadel, or rather than denormalizing into the `webhook_events` table) is worth recording via `manage_adr`.

---

## Verification Gate

The plan is complete when:

1. `cd backend && go test ./... && go vet ./...` is green.
2. `cd ui && bun run test && bun run lint && bun run build` is green (no UI changes expected, but the type contract for `WebhookEventRow` is unchanged so this should be a no-op pass).
3. `cd sync && go test ./... && go vet ./...` is green (no sync changes expected; same caveat).
4. Manual verification of all 7 event types (Task 12) produces the expected log lines and `/operations` rows.
5. `zitadel_grants_index` is being maintained correctly (rows added on grant.added, updated on grant.changed, deleted on grant.removed).
6. The OpenSpec docs reflect the corrected wire format and the new enrichment path.

## Out of Scope (intentional)

- **Reconciliation of `zitadel_grants_index` against Zitadel's source-of-truth grant list.** The index is best-effort and self-healing on the next event. A periodic full-sync job is tracked in the existing reconciliation backlog under `provisioning/spec.md`.
- **Backfilling the index from Zitadel for grants that existed before this fix landed.** A one-shot bootstrap script is reasonable but not blocking — the fallback path covers grants we haven't seen yet at a small per-event API cost. Add the bootstrap if the API-call rate becomes operationally noisy.
- **Refactoring `processGrantAdded` to take a `*WebhookPayload` so post-processing index ops live inside the processor.** The current shape (post-process index in the dispatcher) keeps the processor pure; revisit if a third event source needs the same dispatch wiring.
- **Reverse-mapping `aggregateID` for non-grant aggregates** (e.g., `user_grant.cascade.changed`). Out of scope for the current subscription set; add when those events are needed.
