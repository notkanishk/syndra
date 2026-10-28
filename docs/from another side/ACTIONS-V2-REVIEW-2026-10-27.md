# Actions v2: deterministic verification and UX review — 2026-10-27

Read-only review of Syndra’s Zitadel Actions v2 claim-injection path, the
operator-facing claim editor and preview, and the deployment artifacts.

No code was changed.

---

## Short version

The **shaper** is in good shape: one `claims.Shape` implementation is used by
both the data plane and the simulator, output is key-sorted, and the Actions v2
envelope is well formed.

The **verification story** is not deterministic yet, and the **preview is not
actually the live path**. That is the main thing to fix before trusting v2 in
place of v1.

---

## 1. What the Actions v2 implementation actually does

- Zitadel target: `syndra-claim-injector`, type `restCall`, timeout 3s,
  `interruptOnError: false` (`zitadel/actions/targets.json`).
- Bound to `function.preaccesstoken` and `function.preuserinfo`.
- Endpoint: `POST /api/action/inject`, HMAC-verified with
  `ZITADEL_ACTION_SIGNING_KEY` (`backend/internal/handlers/router.go:398`,
  `backend/internal/handlers/zitadel_action_auth.go`).
- Handler reads only:
  - `user.id`
  - `user_grants[].projectId`
- Everything else in the payload — `userinfo`, `user.human`, `org`,
  `user_metadata`, and the `roles` inside grants — is ignored
  (`backend/internal/handlers/action.go:30-40`).
- For each unique project in `user_grants`, it reads
  `mapping:<user>:<project>` from Redis, shapes it with the project’s claim
  profiles, and returns `{append_claims: [{key, value}, ...]}`
  (`backend/internal/handlers/action.go:83-145,170-200`).
- If Redis misses or the cache is malformed, it falls back to `fail_closed`
  (empty claims) or `minimal_safe` per project
  (`backend/internal/handlers/action.go:345-357`).
- The simulator uses `cache.CompileFacts` + `ResolveClaimProfiles`
  (`backend/internal/services/views.go:810-855`); the live path uses
  `claimsForProject` + `claimProfilesRead`. Same shaper, **different inputs and
  different failure behavior**.

---

## 2. Where determinism breaks today

### 2.1 The preview is not the live path

`TokenPreview` says “Exactly what X would receive right now”
(`ui/src/components/apps/TokenPreview.tsx`). That is not guaranteed:

| Preview | Live Actions v2 |
|---|---|
| `cache.CompileFacts(user, project)` — computed from Syndra’s ledger, bundles, rules | `mapping:<user>:<project>` — whatever is in Redis |
| `ResolveClaimProfiles` — reads Postgres | `claimProfilesRead` — Redis cache, then Postgres |
| Always targets the app’s project | Only emits projects present in Zitadel’s `user_grants` |
| Returns claims even when the cache is empty | Returns `fail_closed` empty claims on cache miss |
| One project only | Merges every project in `user_grants` and may namespace keys |

So the preview can show claims while the real token gets `{}`, especially:

- after a grant/revoke/expiry emptied the cache (the audit’s cache-lifecycle
  bug),
- when a bundle/rule gives access but the Zitadel grant has not landed yet,
- when Redis is unavailable.

**This is the first thing to make deterministic: preview and live must share
the same read path, or the UI must clearly show both “computed” and “live”
answers.**

### 2.2 Multi-project claim keys change based on the user’s other grants

`mergeProjectClaims` only namespaces a key when two projects emit the same key
(`backend/internal/handlers/action.go:125-145`).

Example:

- Project A default: `roles`
- Project B default: `roles`
- User has only A → response contains `roles`
- User later gets B → response contains `syndra.A.roles` and `syndra.B.roles`

App A, which reads `roles`, works for the first user and breaks for the second.
The claim key is conditional on the user’s grant set.

The spec says multi-project responses must always be namespaced
(`openspec/changes/syndra-core-architecture/specs/application-claims/spec.md`),
while the code only namespaces collisions, and the implementation record says
“2+ → namespaced”. Three different answers. Pick one and pin it with a test.

**This is the biggest migration trap from v1**, because v1 could shape per app
and per claim without this collision behavior.

### 2.3 `user_grants` is the gate, not Syndra’s ledger

A role can exist in Syndra’s direct-grant ledger, a bundle, or a mapping rule
and still produce no claims if Zitadel does not list that project in
`user_grants`.

That is correct for “only claim projects the user actually has a Zitadel grant
in”, but it means:

- the preview can show a role that the live token will not carry,
- a queued direct grant has no claims until the drain lands,
- a bundle/rule-only access has no claims until the grant is projected.

The UI never shows this gate. The preview should say “Zitadel lists this
project / does not list this project”.

### 2.4 The smoke test only checks shape

`scripts/smoke-test-action-v2.sh` uses:

```json
{"user":{"id":"smoke-test-user"},"user_grants":[{"projectId":"smoke-test-project","roles":["viewer"]}]}
```

That user/project almost certainly has no Redis facts, so the endpoint
correctly returns empty claims, and the script passes because it only checks
that `append_claims` is an array of `{key,value}`.

It does **not** prove:

- any claim value is correct,
- the configured claim name/format is applied,
- cache hits work,
- multi-project merging is correct,
- the preview matches the live token.

### 2.5 Missing `user_grants` is silently “no claims”

`ActionV2Request.UserGrants` is a slice. Absent and empty are
indistinguishable. A payload with `user.id` but no `user_grants` returns `200`
with `append_claims: []` (`backend/internal/handlers/action.go:95-101`). If
Zitadel ever changes the field name or a caller sends the wrong shape, the
token silently loses all claims.

For a deterministic contract, distinguish “field absent” from “empty list” and
return 400/log on absent.

### 2.6 The degraded DB fallback is not bounded by the 3s budget

`redisTimeout` is 50ms (`backend/internal/handlers/action.go:61`), but
`claimFailureModeRead` and `dbResolveClaimProfiles` can call Postgres without an
explicit timeout. Zitadel’s target timeout is 3s. A slow DB can blow the
budget, and because `interruptOnError: false`, token issuance continues without
claims.

Add a total request deadline and a metric/log for “degraded because budget
exceeded”.

---

## 3. How to check Actions v2 deterministically

Do it in five layers. Each layer should fail on an exact value, not on “looks
plausible”.

### Layer 1 — Pure shaper golden tests

No HTTP, no Redis, no Postgres.

Test `claims.Shape` with fixed `claims.Facts` and fixed profiles:

- `array`, `csv`, `space_delimited`
- attribute claims (`email`, `team`, `role_count`, etc.)
- static claims
- missing attribute omitted, not emitted as `null`
- default + app override
- deterministic profile ordering
- conflict detection

The existing `backend/internal/claims/claims_test.go` covers some of this;
extend it with golden maps.

### Layer 2 — Handler golden tests with seeded facts/profiles

Keep using the fake `redisGetClaims` / `dbResolveClaimProfiles` seam, but assert
the **entire** `append_claims` array, not just one key.

Use a table:

| Case | Input | Expected |
|---|---|---|
| Single project, default profile | user `u1`, project `p1`, facts `["operator"]` | `[{"key":"roles","value":["operator"]}]` |
| Single project, CSV | profile `x-groups` / `csv` | `[{"key":"x-groups","value":"operator,viewer"}]` |
| App override | default `roles` + app `badge.roles` | both keys, correct formats |
| Attributes/statics | `team`, `tenant` | exact values |
| No grants | `user_grants: []` | `[]` |
| Duplicate project rows | two `p1` grants | one Redis read, one claim |
| Two projects, distinct keys | `p1:roles`, `p2:permissions` | both flat |
| Two projects, same key | both default `roles` | `syndra.p1.roles`, `syndra.p2.roles` |
| One project degraded | `p1` hit, `p2` miss | only `p1` claim |
| Cache miss, fail_closed | no facts | `[]` |
| Cache miss, minimal_safe | configured map | exact map |
| Malformed cache | bad JSON | `[]` or minimal_safe |
| Absent `user_grants` | missing field | 400, not empty 200 |
| Unknown fields | full canonical payload | 200, ignored |
| Missing `user.id` | no id | 400 |

Use a fixed `CompiledAt` in facts, and ignore it in comparisons if it is part of
a claim.

### Layer 3 — Signed endpoint contract test

Test the real middleware with a fixed clock:

- valid `ZITADEL-Signature: t=...,v1=...`
- invalid signature → 401
- stale timestamp > 300s → 401
- multiple `v1=` entries → accepts one valid
- body rewind still lets the handler decode

The existing `backend/internal/handlers/zitadel_action_auth_test.go` covers
much of this; keep it and add the exact-envelope case.

### Layer 4 — Live token black-box

Use a disposable test user and app on a staging Zitadel:

1. Give the user a known grant in one project.
2. Configure a known claim profile.
3. Ensure the cache is warm:
   ```bash
   redis-cli GET "mapping:<user>:<project>"
   redis-cli GET "claim_shape:<project>"
   ```
4. Sign in and decode the access token:
   ```bash
   echo "<token>" | cut -d. -f2 | base64 -d | jq
   ```
5. Compare only the custom claims, ignoring `iat`, `exp`, `nbf`, `jti`.
6. Repeat twice; the custom claim map must be identical.

Then repeat with a user who has **two** projects, and a user who has a
bundle/rule role but **no** Zitadel grant. Those are the two cases the preview
currently hides.

### Layer 5 — v1 → v2 parity

Before removing v1, capture a baseline matrix:

- 3–5 users: new member, member with a direct grant, bundle-only,
  rule-derived, no roles, deactivated
- 2–3 apps across projects
- decode the v1 token and keep only the custom claim keys your apps read

After switching to v2, run the same matrix and diff the normalized maps.

Important: do **not** compare the whole token. v1 and v2 may differ in standard
Zitadel claims, and v2 does not emit `set_user_metadata`. Compare only the keys
the application contract says it reads.

Also: **v1 and v2 must not coexist on the same trigger**. Retire the v1 action
before binding v2, or use a separate Zitadel instance for the comparison. The
README already warns about this (`zitadel/actions/README.md`, “Operator
warnings”).

---

## 4. What v1 can do that v2 cannot

These are the migration decisions you need to make explicit:

| v1 capability | v2 reality |
|---|---|
| JavaScript logic, conditionals, string manipulation | No runtime. Only `append_claims` from Syndra’s resolver. |
| Read `userinfo`, `user`, `org`, `user_metadata` | Ignored. Only `user.id` and `user_grants` are read. |
| Per-application shaping using the client id | The v2 function payload has no application id. Every app on the project receives the same claim set; each reads its own key. |
| Write user metadata | `set_user_metadata` is intentionally not emitted. |
| Different logic for `preaccesstoken` vs `preuserinfo` | The handler does not branch on `function`; both get the same claims. |
| Arbitrary claim values | `claims.Shape` supports arrays/strings/statics, but the current UI coerces static values to strings. |

If any of those were load-bearing in your v1 scripts, v2 is not a drop-in
replacement; it is a redesign of where that logic lives.

---

## 5. How to make the UX more empowering

The current screen already has the right ingredients: editor, preview, key
inventory, conflict notice. The missing piece is **truthfulness and
testability**.

### 5.1 Make the preview honest

Either:

- make the preview use the exact live path (`claimsForProject` +
  `claimProfilesRead` + `user_grants` gate), or
- show two previews side by side:
  - **Computed:** what Syndra would shape from current state.
  - **Live now:** what Zitadel would actually receive from the cache and
    `user_grants`.

The second is more empowering because it shows drift instead of hiding it.

Add a badge for:

- `cache hit` / `cache miss`
- `fail_closed` / `minimal_safe`
- `project listed in Zitadel grants` / `not listed`
- `no roles in this project`

Then “empty claims” stops looking like a broken preview and starts explaining
itself.

### 5.2 Preview the draft before saving

Today the preview shows the saved shape; the editor only says “save to preview
your changes” (`TokenPreview.tsx`, `behindEdits`).

Add a dry-run endpoint:

```
POST /api/v1/projects/{id}/claim-shape/preview
body: { draft_profile, user_id }
```

It must call the same `claims.Shape` and return claims **without persisting**.
That is the single biggest UX upgrade: an operator can see the effect before
changing every app on the project.

### 5.3 Show the whole token, not just one app

The real token is per project and, for multi-project users, merges projects.
The current preview is per app/project and cannot show:

- multi-project key namespacing,
- collisions across projects,
- the difference between `roles` and `syndra.p1.roles`.

Add a **“Whole token for this person”** view that lists every project in their
Zitadel grants, the claims each contributes, and the final merged
`append_claims`. This is where the current collision behavior becomes visible.

### 5.4 Show the actual Actions v2 envelope

The preview shows the claim map, not the v2 response. Add a toggle:

```json
{
  "append_claims": [
    {"key": "roles", "value": ["operator"]}
  ]
}
```

with a “Copy reproduction” button. This is what a developer can paste into a
test fixture or compare against a decoded token.

Do not expose the HMAC key. For a signed reproduction, use a backend dry-run
endpoint or a staging-only script.

### 5.5 Add a representative-user test matrix

One user at a time is not enough. Add a table:

| Person | Why | Claims |
|---|---|---|
| New member | no grants | `{}` |
| Direct grant | happy path | `roles` |
| Bundle-only | derived access | `roles` |
| Rule-derived | mapping chain | `roles` |
| Multi-project | collision behavior | namespaced keys |
| Deactivated | lifecycle | `{}` / disabled |
| No Zitadel grant but Syndra role | gate | `{}` + explanation |

This turns the screen from a single dry run into a contract check.

### 5.6 Let each app declare its contract

The app list currently shows `claim_name` and `format_type` from the **project
default** (`backend/internal/directory/zitadel.go:504-518`,
`ui/src/app/applications/page.tsx:142`). An app override is invisible on the
index, and the mixed-format warning uses the project default, so it can miss
real per-app differences.

Fix:

- show the **effective** claim for each app (override if present, otherwise
  default),
- let an app declare “I read key X in format Y”,
- validate the profile against that contract before save,
- show “matches / mismatch” on the app page.

This is more useful than a generic claim-name editor because it answers “will
my app break?”

### 5.7 Make editing safe

- **Version history and revert.** A claim edit changes the next token for
  every app on the project. The audit already records claim template
  versioning as open (C5). Add an immutable snapshot + “revert to previous
  shape”.
- **Confirm project-default saves.** Show how many apps and people are
  affected.
- **Inline validation.** Check invalid keys and conflicts before save, not
  only after the backend rejects them.
- **Expose `configured`.** The editor’s `looksUnset` heuristic guesses whether
  `roles`/`array` was deliberate (`TokenFormatEditor.tsx`). Add a `configured`
  boolean to `ClaimShape.Default`.
- **Typed static claims.** The UI coerces `static_claims` values to strings
  (`toExtras`/`fromExtras`). A boolean or number saved by v1 becomes
  `"true"`/`"42"` after an edit. Add a type selector or a JSON value editor.
- **Better error attribution.** The claim-shape handlers map every save
  failure to `claim_name: invalid`; a conflict on `team` or `tenant` is
  reported as a bad claim name. Return the actual key/conflict.

### 5.8 Add a v1 migration assistant

This is the most directly empowering thing for your situation:

- paste a decoded v1 token (custom claims only),
- map each v1 key to a v2 profile key/format/source,
- show which v1 claims cannot be reproduced (metadata, client-id logic,
  arbitrary JS),
- generate the claim profile and a fixture for the deterministic test,
- show a before/after diff for each app.

That turns “I hope v2 is equivalent” into “here is the exact difference.”

### 5.9 Make the empty state explain itself

The preview currently says “none of the roles they hold belong to this
project” when the claim map is empty. It should distinguish:

- no Zitadel grant in this project,
- grant exists but no roles,
- cache miss,
- `fail_closed`,
- profile key is empty,
- app is reading a different key.

Each has a different fix. Collapsing them into one sentence is the same failure
mode the audit found elsewhere.

---

## 6. What to do first

1. **Fix the cache lifecycle** — a token must still carry claims after a
   drain. Until this is true, every live test is testing a cache the app may
   never see.
2. **Add the preview/live parity test** — same facts, same profiles, same
   user, compare `SimulateApplication.CustomClaims` to
   `HandleActionInject.AppendClaims`.
3. **Decide the multi-project rule** — always namespace, or collision-only.
   Then pin it with tests and make the UI show the whole token.
4. **Replace the shape-only smoke test** with a fixture that asserts exact
   claims for a known user/project.
5. **Add draft preview + before/after diff + version history** — this is the
   UX that makes claim editing safe.
6. **Fix the app index’s effective claim** — project default vs app override
   is currently a lie on the index.
7. **Add the v1 comparator** — capture v1 custom claims for a small matrix,
   diff after cutover, ignore standard Zitadel claims and metadata.
