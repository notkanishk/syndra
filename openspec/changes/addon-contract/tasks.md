# The add-on contract: task ledger

> **Read `design.md` §12 first.** Slice 0 lands alone and before anything
> else: a schema written after the wire changes pins whatever the code
> happened to do, which is the defect `addons/contract/README.md` was written
> to stop. Every slice is one PR, shippable and reversible on its own.
>
> **Owner decisions 1–23 were accepted on 10 Nov 2026**; the role names are
> final (`account, channel, membership, flag, limit`), and 14 was decided by
> the owner verbatim: admins see usage and statistics directly, no view
> logging. Decisions 24–27 (raw query signing, RFC 9457 envelope,
> `account_id`, paging deferred) accepted as recommended, 10 Nov 2026.
> Revised after the two Opus reviews of 10 Nov 2026
> (`.claude/jobs/146ae399/tmp/addon-contract-review{,-2}.md`).
>
> **Build order:** 0 → 1a (schemas only) → 4 (kit and reference, core, against
> 0 and 1a) → 1 (the kit passes against the v2 TrueNAS add-on on staging
> *before* 1.10) → 2 ∥ 3 → 5 → 6.
>
> Labels per `docs/agents/triage-labels.md`. No Docker on the Mac; the
> add-on's own fake target and the reference add-on cover every local test.

## 0. Pin today (contract 1, no wire change)

- [x] 0.1 `addons/contract/schema/` with one JSON Schema 2020-12 per message
  for all twelve routes exactly as the wire is today
  (`research/contract-1-inventory.md` is the inventory, derived from
  `addons/truenas/server.go:68-93`; `health.log_records` pinned as integer
  ≥ 0; `taken_at` as RFC 3339), the v1 error body as `error.json`
  (`{"error": "<CODE>", ...extras}`, `server.go:396-410`),
  `common/{field,effect,outcome}.json`, `profiles/storage/{reach,homes}.json` #ready-for-agent
- [x] 0.2 `addons/contract/openapi.yaml` naming every route, method, status
  code and body `$ref`; validated by `go test` in the kit module once it
  exists, by a `bunx @redocly/cli lint` step until then #ready-for-agent
- [x] 0.3 A fixture for every message that has none today: manifest,
  health, subjects, values, reach, settings list + one setting, the
  `PUT /settings/{name}` request body, homes, lifecycle request/response,
  plan response, the operation **response** (`OperationResult` with its
  `Activity`/`StorageStatus`/`Health` pockets, one fixture per pocket), and
  one error body; every field populated, WWE ring names and `example.org`
  only; the four existing fixtures move to `fixtures/` #ready-for-agent
- [x] 0.4 Two-ended assertions for all twelve, as `contract_test.go` does for
  four: producer marshals byte-equal after normalisation, consumer decodes and
  every field it reads lands. Backend reads the fixtures by path as today #ready-for-agent
- [x] 0.5 `registry/errors.json` (every `error` code the add-on writes, all
  24 distinct ones including `NO_CURRENT_READ`, `REACH_NEVER_READ`,
  `STORE_UNREADABLE`, with status and retry class) and
  `registry/operations.json` (every id in `operationPolicy`, with scope
  ceiling, params, secret params); the add-on's codes are inline string
  literals today, so either hoist them into constants (no wire change) or
  have the test AST-scan `"error":` literals; tests on both sides assert
  the code equals the registry (a code the add-on writes and the registry
  lacks fails the add-on's suite; a policy entry the registry lacks fails
  the backend's) #ready-for-agent
- [x] 0.6 Schema validation in both suites: every fixture validates against
  its schema (`santhosh-tekuri/jsonschema/v6`, test scope, `AssertFormat`
  on). Owner decision 4, accepted #ready-for-agent
- [x] 0.7 `addons/contract/README.md` rewritten around the directory above;
  `addons/README.md` route table corrected to twelve #ready-for-agent

## 1a. The contract-2 schemas, alone (a schemas-only PR, before slice 4)

Decisions 24–27 accepted 10 Nov 2026 (27: no subjects cursor in 1a);
nothing here is code.

Decided while building (10 Nov 2026; where design and tasks were silent or
disagreed, design won): four raw-query vectors (design §4), not three;
`PLAN_TOO_LARGE` 422, `UNKNOWN_METRIC` 404 (statuses unspecified); a blocked (422 `APPLY_BLOCKED`) or conflicting (409 `BINDING_CONFLICT`) apply is problem+json with the outcome as extension members; a preview carries no `call_id`; the
`acknowledge_scope` request field and `SCOPE_ACKNOWLEDGEMENT_REQUIRED` are
gone from major 2 (§11); `homes.usage_read` becomes `quota_read` (the §4
table, over decision 21's wording); `status.json` lists its rows as `meters`;
`activity` events drop `share` (vendor data goes under `x-`); `retry` in
`error.json` is optional and one of `never|backoff|retry_after|replan`;
identity objects always carry `params` and `subject`; files `v1/`, `v2/` per
design §1; `CHANGELOG.md` at `addons/contract/`.

- [x] 1a.1 Manifest v2 schema and fixture: `contract_version: 2`, `kind` from
  `registry/kinds.json` (`storage|access|inventory|machine`, each with its
  scope denylist; required), `features[]`, `entitlement_schema[].role` with
  `unit`/`combine`/`unlimited` required on `limit` through `if`/`then`,
  `type`/`lifecycle`/`single` removed, `operations[].params` optional and
  UI-hint only, `limits{subjects_per_plan, fresh_read_cooldown, stats_buffer}`
  and nothing else; field, operation and metric objects
  `unevaluatedProperties: false` with `patternProperties ^x-`; `x-*`
  allowed at the top level; `registry/{roles,features,kinds,units,constants}.json`
  (`constants.json`: `max_reply_bytes` 1 MiB, `call_memory` P30D,
  `in_doubt_bound` PT1H, `series_per_metric` 500, `cooldown_default` PT30S;
  every declared duration ISO 8601); `errors.json` gains
  `UNKNOWN_REQUEST_FIELD` (400, the strict decoder's own code, N3),
  `CALL_IN_PROGRESS`, `CALL_UNSETTLED`, `UNKNOWN_METRIC`,
  `FRESH_READ_COOLDOWN`, `TARGET_UNREACHABLE`, `PLAN_TOO_LARGE`, and
  `CALL_ID_REUSED` moves to 422; feature-introduced keys listed under
  their feature in `features.json`; OpenAPI feature gates as `x-feature`;
  the validator with `AssertFormat` on in every suite. Decision 23 #ready-for-agent
- [x] 1a.2 Every other major-2 schema and fixture: the per-message reply
  fields exactly as design §4's table lists them (`reach.read_at` →
  `taken_at`; `homes` gains `taken_at` + `current` and keeps its
  completeness flag as `quota_read`; plan keeps `truncated`; values exempt;
  `common/stale.json` with `cached|cooldown|unreachable|draining|never_read`
  and `retry_after` in seconds), the RFC 9457 `error.json` (`type`, `title`,
  `status`, `detail`, extensions `code`, `retry`), the request-identity
  objects of §5 as `identity/*.json` with a JCS vector per kind,
  `standing` present iff an account exists, `account_id` replacing `uid`,
  `call.json`, `stats.json` (per-metric reply: `metric`, `taken_at`,
  `watermark`, points, gaps with `dimensions`/`start`/`end`, `unbound`,
  keyset `next_cursor`), `common/{records,activity,status}.json`,
  `transport_derivation.json` with the three raw-query vectors;
  `CHANGELOG.md` entry for major 2 naming each file; WWE ring names and
  `example.org` only #ready-for-agent

## 1. Contract 2 on the wire (one deploy, both containers)

Decisions 1, 3 (names final), 6, 10, 22-27 accepted 10 Nov 2026. Depends on 1a and on slice 4 having passed against the v2 add-on.

Built 11 Nov 2026 (branch feat/addon-contract-1). The kit passes against the
v2 TrueNAS add-on in process (`addons/truenas/conformance_test.go`: the kit's
binary over the add-on's real routes, transport and derived TLS key, on a fake
NAS; reads only, `-mutate`, `-mutate -mutate-settings`; no MUST fails), and
CI runs it inside the add-on's `go test`. `addons/contract/v1/` is deleted.
No schema, fixture or registry changed. Decided while building, where the
design and tasks were silent (design won where they disagreed):

- *The effective limit is observed.* TrueNAS reports `home_quota` in
  `/subjects` `state` as the limit it holds (one quota table per home
  dataset, the way `/homes` reads usage; `"0"` when there is no limit) and
  leaves it out when the table could not be read (unknown, never "none").
  It is not in an apply's `observed` (the read-back does not read quotas).
  The kit's `limit-combine` needs it (design §3). **The backend does not
  compare a limit-role field in the drift sweep in slice 1**
  (`drift.stateByBinding` drops them by `targets.field_roles`): the core
  compares limits only once it combines them (slice 2, 2.x below), and an
  unreadable quota table must never surface as a finding.
- *A channel is off whenever the account is*, enforced by TrueNAS too
  (`decodeDesiredFields`): the backend always sent `smb_enabled: false` with
  `enabled: false`; the kit's `channel-follows-account` asks the add-on.
- *Plan equals apply, word for word*: an apply's `no_change` (and its
  quota-only `applied`) carries the plan's consequence (`describeHolding`).
  A plan outcome about an existing account carries `account_id` and
  `standing` (the kit writes only after a plan names its own account).
- *Standing (TrueNAS)*: the add-on's own account usable; a switched-off
  account `none` ("The account is switched off."); no password `member` with
  `smbWaiting`'s sentence and "Set a storage password on your storage page.";
  otherwise usable. An unverified apply reports the standing last read.
- *Call identity*: the lease is keyed by call_id; another request under a
  running call_id is 422 `CALL_ID_REUSED`, the same one 409
  `CALL_IN_PROGRESS` with `Retry-After: 30`. Read operations
  (`storage.status`, `activity.get`, `health.get`) are neither leased nor
  recorded, as in contract 1, and still need a call_id. A contract-1 apply
  entry replays with `account_id` and `standing` read from the mirror, or
  without both when the mirror lacks the account; an `account.adopt` entry's
  `account_uid` replays as `account_id`. A replayed lifecycle change is the
  recorded reply even if the state moved since. A blocked apply from a
  truncated read is 422 `APPLY_BLOCKED` (contract 1 answered 409). A planned
  subject with a blank `subject` is 400 `BAD_REQUEST`.
- *Refusals*: every route not served (and a known path with another method)
  is 404 `UNKNOWN_ROUTE`, authenticated. A `by_status` code's `retry` is
  `backoff` for 429/5xx, else `never`. The backend's feature routes for the
  404 rule are `/operations/`, `/settings`, `/reach`, `/homes`, `/calls/`,
  `/stats`, and the 404 must name no code or `UNKNOWN_ROUTE`: a 404 with
  another code (`NO_SUCH_SETTING`, an adoption's `OPERATION_FAILED`) is the
  add-on answering about the request and stays a refusal; a shape refusal re-reads the manifest in the background,
  single-flighted on the on-demand read's 30 s cooldown. A role outside the
  vocabulary is refused at registration like an unknown kind. The persisted
  role map is a union: a field the manifest drops keeps its role, so it cannot
  return meaning something else.
- *Plans in chunks*: the backend plans below `limits.subjects_per_plan`, and
  the chunks answer as one plan (current only if every chunk was, truncated
  if any was, dated by the oldest read; one failing chunk fails the plan).
  `acknowledge_scope` left `addons.Plan` and `EntitlementRehearsal`; every
  plan surface's own acknowledgement at issue time is unchanged.
- *Views keep the UI's names* (task 1.0): `TargetHealth` keeps
  `key_expiry`/`key_expires_at` (wire: `credential_*`), `Homes` keeps
  `read`/`usage_read` (wire: `quota_read`), `Reach` keeps `read`/`read_at`
  (wire: `taken_at`). `TestEveryReplyDecodesIntoAWireTypeOfContractFields`
  holds every decode target to its schema's properties. The UI moved only
  its types (`account_id`, `standing`, `also_says`).
- *account_id in Postgres*: written to `account_id` and, while it is a
  canonical decimal, to `account_uid` (a UUID leaves `account_uid` NULL); read
  as `COALESCE(account_id, account_uid::text)`; `db.BackfillAccountIDs` runs at
  every backend start after the migrations. The add-on's bbolt store keeps the
  same rule (`uid` beside `account_id`; a binding with `account_id` alone is
  read too).
- *Not built*: TrueNAS never adopts an unbound account by derived name, so
  the crash window between `user.create` and the binding is a
  `BINDING_CONFLICT` an operator adopts (the MUST "only under a logged create
  intent" holds trivially); the intent-based adoption waits for an add-on
  that needs it. `credential_scopes` is not reported (TrueNAS cannot read its
  own key's roles; advisory). The target page does not yet render
  `also_says` or the refusal reason (3.6).

After the two Opus reviews (11 Nov 2026), decided by the owner and built:

- *A cache is not current* (design §4): `/reach` serves its five-minute cache
  as `current: false, stale.reason: cached` (with `retry_after`), and the
  failed-read fallback as `unreachable`; `/homes` never caches. The home
  location write reads the NAS now, never the cache (a place deleted a minute
  ago must not pass). The kit's `current` check covers `/reach` and `/homes`
  when the storage features are declared.
- *Every backend-filled field is out of reach of the wire* (1.0): a place's
  `smb_address`, `unc_address` and `roles` are decoded through `placeWire`,
  in `/reach` and the `storage.status` pocket alike, and no wire type holds a
  view type anywhere inside it (`TestNoWireTypeHoldsAView`).
- *An unreadable `taken_at` is no reading* (§2(b)): absent, unparseable or
  zero, it makes `/subjects` and `/plan` not current and truncated, so a plan
  is refused and a chunk merge never meets a zero date.
- *A feature's route is called only when declared* (§2 rule 2): the pipe
  refuses `/operations/`, `/settings`, `/reach`, `/homes`, `/calls/`,
  `/stats` on an add-on whose accepted manifest does not declare the feature,
  before the breaker and before any operation row; no manifest yet is no
  feature yet.
- *account_uid never outvotes account_id*: a write that carries an id sets
  `account_uid` from it (NULL for a non-numeric one), so the backfill
  (`account_id IS DISTINCT FROM account_uid::text`) corrects exactly what a
  rolled-back contract-1 backend wrote or updated.

- [x] 1.0 (found by slice 0) The backend's `Homes.Read` and `Reach.Read`
  (`addons/read.go`) are its own "the read succeeded" flags but share the
  wire's JSON key `read`, so a tolerant decode would let an add-on claim a
  read. Split the wire type (decoded from the add-on) from the view type
  (sent to the UI) for every read reply, so a backend-only field can never
  be set by an add-on; the v2 schemas then stay the only description of
  what the add-on sends #ready-for-agent

- [x] 1.0a (found by slice 0 review) The add-on emits `[]`, not `null`, for an
  empty list field (wire change): `entitlementState` (`apply.go`) builds the
  group list with `append([]string(nil), ...)`, so an account with no groups
  is `"group": null` in `/subjects` `state` and in apply `observed`. Contract 1
  pins that (the reply schemas allow null for list fields; `desired` stays
  arrays only). `services/merge` canonical must also treat a null list as `[]`
  (today `sameValue(null, [])` is false; harmless now because the add-on's plan
  compares groups as sets) #ready-for-agent

- [x] 1.2 TrueNAS add-on declares it (`capabilities.go`): `kind: storage`,
  features `operations, settings, storage.reach, storage.homes`, the five
  fields by role as in design §3; `ContractVersion = 2` #ready-for-agent
- [x] 1.3 Backend manifest decode becomes tolerant **everywhere**, inside
  objects too; every unknown key and `x-` key shown raw (quoted, escaped,
  length-capped) on the target page and logged once; unknown features
  skipped; `FieldsByRole()`, exactly-one-`account` and an unknown `kind`
  refused at registration with a reason; **the role map persisted** to
  `targets.field_roles` (migration, additive) at every registration and
  refresh, and **a changed `role` on an existing field name or a different
  `account` field refused**, last good map kept, reason on the target page
  (test: a manifest that moves `account` to `smb_enabled` is refused and
  revocation still plans on `enabled`); the reply fields of design §4's
  table decoded as pointers per message and an absent one makes the read
  unusable (`current: false`, `truncated: true`), test per field per
  message #ready-for-agent
- [x] 1.4 Every write carries `contract_version` and `call_id` in the body:
  `/apply`, `/operations/{name}`, `/lifecycle`, `PUT /settings/{name}`.
  Backend `pipe.go` drops `Idempotency-Key`; the add-on's `kindSetting` and
  lifecycle dedupe read `call_id` through the one store; **request identity
  per kind** = SHA-256 over RFC 8785 JCS of `{subject, desired}` (apply;
  `email`, `plan_id`, `fingerprint` excluded), `{name, subject, params −
  secret_params}` (operation; `preview` excluded, it is never recorded),
  `{state, reason}` (lifecycle), `{name, value}` (setting), asserted
  against the `identity/*.json` vectors; **only a 2xx is recorded**; same
  identity → the first reply **before the fingerprint check** (the
  planner's re-plan of an indeterminate row,
  `propagation/addon.go:188,296-306,358-398`, is the test case; a
  `password.set` replay with a different password is the second);
  different identity → **422 `CALL_ID_REUSED`** (widening `store.go:131`,
  correcting the status per draft-ietf-httpapi-idempotency-key-header);
  **in flight = executing in this process now**, an in-memory lease taken
  before the log intent and released on reply or return, never inferred
  from an intent without a result → **409 `CALL_IN_PROGRESS`** +
  `Retry-After`, classed `unreached` (test: a re-send after a simulated
  crash between intent and result is executed through lookup-before-create,
  not answered 409); **an entry remembered by v1 (no digest) matches any
  identity**, as a no-`Kind` entry does today (`store.go:126-132`) (test: a
  v1-shaped entry replays, is not 422); a refused call re-evaluated on
  replay; the store holds no secret and no hash of one (test: grep the
  bbolt file for the canary after `password.set`); call memory ≥
  `constants.call_memory`. Mutation-checked on all four #ready-for-agent
- [x] 1.4a The raw query joins the MAC: `ts.method.target.body` with
  `target` = escaped path as sent + `"?" + RawQuery` when non-empty, on
  both ends (`backend/internal/addons/transport.go:441-450`,
  `addons/truenas/transport.go:173`), the path signed **as sent, escaped**
  (both ends sign the decoded `URL.Path` today, `transport.go:317`, add-on
  `:104`); `transport_derivation.json` gains four vectors (no query, whose
  MAC equals major 1's; a base64url `cursor`; a value with a reserved
  character; an escaped path); tests: each vector reproduces, a reordered
  or altered query → 401. Decision 24 #ready-for-agent
- [x] 1.5 Error envelope: every add-on refusal is `application/problem+json`
  per `schema/error.json` through one `writeError` (decision 25); the
  strict decoder answers its own code `UNKNOWN_REQUEST_FIELD` (400), and
  `BAD_REQUEST` keeps meaning a missing `call_id`/`subject`
  (`apply.go:331-332`, `operations.go:93-94`) and stays `rejected`; backend
  reads `code` and `retry` to refine `classifyStatus` (status stays
  primary; `PLAN_STALE` → re-plan as today), **falling back to the v1
  `error` key for classification only** until the next major (a v1 add-on
  writes `{"error": CODE}`, no `code`, no problem+json); **a lifecycle
  refusal is detected by the code `LIFECYCLE_REFUSAL` alone**, no longer by
  503 + `Retry-After` (`transport.go:342,401`); **`CONTRACT_VERSION_MISMATCH`,
  `UNKNOWN_REQUEST_FIELD`, and a 404 on a feature route classify as
  `unreached` and trigger a manifest refresh, never `target_refused`**
  (tests, the rollback shape: the **new** backend against a **v1** add-on
  answering `{"error":"CONTRACT_VERSION_MISMATCH"}` 422 leaves no
  `target_refused` row and the next withdrawal does not wait for an
  operator; a v1 `{"error":"LIFECYCLE_REFUSAL"}` 503 classes as a lifecycle
  refusal, not `indeterminate`; a `BAD_REQUEST` for a missing subject is
  still `rejected` and does not requeue; the deploy-window shape, old
  backend against v2 add-on, cannot be tested in the new suite and is
  closed by the 1.10 order instead) #ready-for-agent
- [x] 1.6 `standing` on every subject (`common/standing.json`) and on the
  apply outcome; TrueNAS: no password → `member` with the existing
  `smbWaiting` sentence as `note` and the storage page's next step as `next`;
  `password_set` removed from the wire #ready-for-agent
- [x] 1.7 `account_id` (string) replaces `uid` on `subjects[]`, the apply
  reply and the binding conflict (decision 26), and on `homes[]` and the
  `account.adopt` reply (`account_uid`), which carry it too in major 2; migration adds
  `account_id TEXT` beside `account_uid BIGINT` in **both** tables
  (`000032:38`, `000039:22`), the partial unique index **on
  `target_account_bindings` only** (`target_binding_conflicts` keeps
  resolved history and stays unique only on its open-conflict index,
  `000039:52-54`), backfill written as a re-runnable statement and run
  again on every forward deploy, both columns written for one release
  (owner decision 10); **the add-on's bbolt store written additively**:
  the `bindings` bucket carries both `uid` and `account_id` for one
  release and v2 reads either, call entries with and without a digest are
  both readable (1.4), so a v1 binary opens a v2 `state.db` and rollback
  restores nothing (a restored store would unbind every account v2 created
  in the window and `fingerprintFor` would mark each `target_refused`,
  `propagation/addon.go:392-401`; `ids` alone would be safe, `store.go:392-410`)
  (test: the v1 store reader opens a v2-written file and finds the
  binding); adoption of an unbound account by external reference or derived
  name **only under a logged create intent**, else `conflict` (test: an
  `employee_number` typed in a console does not bind; a second conflict on
  the same account inserts after the first is resolved) #ready-for-agent
- [ ] 1.7b A release later: drop `account_uid` from both tables and the
  dual write; the one task the earlier draft lacked #ready-for-agent
- [x] 1.9 Code for every message that moved in 1a: `PLAN_TOO_LARGE` (422)
  replaces `SCOPE_ACKNOWLEDGEMENT_REQUIRED` in the add-on (the backend
  chunks under `limits.subjects_per_plan` and owns the acknowledgement);
  `reach.read_at` → `taken_at`, `homes` gains `taken_at` + `current` and
  keeps `quota_read` as its completeness flag, on both ends; when the
  TrueNAS add-on answers a read from its mirror because the live read
  failed (`server.go:321-349`, its only mirror path today) the reason is
  `stale.reason: unreachable`, not `cached`, which is reserved for a
  deliberate mirror answer under `freshness` (6.2); the drift sweep already
  treats `current: false` as a skipped pass #ready-for-agent
- [ ] 1.10 (the workflow order is written, 11 Nov 2026, held by
  `repoguard/deploy_order_test.go`: the images are built first, while the old
  containers serve, then the stop order below, then `up -d --build
  --remove-orphans` from the cached images, and an `if: failure()` step
  starts everything again so a failure never leaves the target down.
  **Rollback = dispatch `deploy.yml` (from main) with `ref=<previous
  production commit SHA>`** (from the deploy run history; the repo has no
  tags): a `resolve` job deploys only a full SHA on `main` whose `ci-ok`
  passed, and staging and production check out exactly that SHA; the current
  workflow file, with its stop order, deploys the old code; never a revert
  commit (it reverts the file too) and never the old commit's own workflow.
  A backend from this commit on starts on a clean database that is ahead of
  it (`db.migrateUp`, never migrating down), and staging's migration check
  is "at least, and clean"; **the first rollback across 000088 to a pre-088
  backend needs `schema_migrations` reset to 87 by hand first, then a fresh
  dump pushed to staging (`/usr/local/bin/syndra-push-staging-dump`, as
  root on production), then the dispatch** (DEPLOY.md, Rollback, has the
  SQL; 000088 is additive, the old backend ignores it; without the push,
  staging restores last night's copy at 88 and the old backend refuses it).
  The staging rehearsal and the after-deploy check below are the owner's)
  Deploy. **The order is written into `deploy.yml`'s production
  job and into `_staging.yml` in this PR**, because merging to main runs
  `deploy.yml` on push (`deploy.yml:31-34`) at the pushed commit, and the
  standing job is `pg_dump` → `compose up -d --build --remove-orphans`
  (`:78`) with no stop step; a one-off dispatch would run only after that
  unsafe deploy had already happened. The steps, which cost seconds and
  stay permanently: `docker compose stop backend` (this also stops the
  propagation runner, which is in-process in `backend`, `cmd/api`,
  `periodic`; **never** the GitHub self-hosted runner `syndra-prod`/
  `syndra-staging`, which is running the job), `docker compose stop
  truenas-addon`, copy `state.db` out of `truenas_addon_state` to a dated
  file (only after the add-on is stopped, never from a live bbolt file;
  never `mutations.log`, which stays forward and whose restore would raise
  `AnchorRecordsDecreased`, `db/log_anchor.go:71-75`), then `up -d --build`.
  The add-on's lifecycle state is env-at-startup (`main.go:121,233`), so no
  `read_only` is set and none is needed. Before the merge: the kit (4.2)
  has passed against the v2 add-on on staging, and a staging rehearsal
  starts backend v2 against the **v1** add-on first and sees
  `ErrUnsupportedContract` once in the log (proof the gate works), then
  swaps the add-on. After each deploy: no `target_refused` row from the
  window. Rollback = the previous production commit dispatched through the same workflow (both containers); **no
  store restore** (the store is additive, 1.7; the dated copy is a fallback
  only if a v1 binary cannot open the file); the re-runnable backfill
  (1.7) covers rows v1 writes meanwhile #ready-for-human

## 2. The core forgets the names (backend only)

Built 11 Nov 2026 (branch feat/addon-contract-2), all but 2.6 (the staging
run, the owner's). Behaviour for TrueNAS is unchanged except 2.7. Decided
while building, where the design and tasks were silent:

- *The set carries its role map.* `EntitlementSet.Roles` (`addons.Roles`,
  read by `ResolveEntitlements` from `targets.field_roles` through
  `addons.FieldRoles`) is what `Desired` and `WithdrawsOnly` name the account
  and channels from; a target with no persisted role map, or none with one
  `account`, does not resolve (an error, never a guess). `Desired` writes every
  channel the role map declares, off unless the set says on.
  `LifecycleState` is `{Account, Channels, NotAMember, Reason}` (JSON
  `account`, `channels`; no UI read the old keys).
- *Validation reads the accepted manifest's roles*, as it already read the
  schema: the add-on must answer to validate a mapping at all, and the role
  map cannot disagree with the accepted manifest for a declared field
  (`acceptRoles` refuses a changed role). `ValidateMappingField` and
  `ValidateAllowanceTerm` take the schema. The resolver's defence in depth
  reads the persisted map.
- *A revocation with no role map* answers 409 `ACCOUNT_FIELD_UNKNOWN` and
  writes nothing ("connect its add-on once"): there is no field to hold.
- *The storage kind's view* is `handlers/kinds` (`storage.go`): its
  membership and home flag are the target's one field of each role (none when
  zero or two: the page says less), its home limit the one `limit` whose
  `unit` is `bytes`, read from the accepted manifest (the role map carries no
  unit) with `unlimited` from the field, compared numerically. The address is
  built only for `connection.protocol == "smb"` with a host.
- *Mapping sentences* (`mappedTo`): a `membership` with a label is "the
  <value> <label, lowercased>" ("the Staff group"); anything else is
  "<label or name> <value>"; an unknown schema falls back to the name.
- *2.7, the comparison*: `drift.comparable` puts the intent in the target's
  terms: a `limit` field becomes the one combined limit
  (`EntitlementField.CombineLimit`), and is left out (not compared, never
  `theirs_only`) when no value sets it, when the target did not report it, or
  when no accepted schema says how it combines. The wire map is still what is
  planned and queued; the merge base and findings carry the combined value.
  An apply's `observed` now carries the managed limit as the add-on read it
  back (`homeLimits`, the rule `/subjects` state uses; left out when the quota
  table could not be read), so the base has the limit straight after an apply
  (review gap, closed 11 Nov; contract v2 amended, CHANGELOG).
- *Take theirs on a limit* (owner decision, 11 Nov): a finding on a `limit`
  the add-on lets an operator set per person is adoptable; adopting saves the
  target's observed value (one whole number) as the person's own limit, a
  `set` allowance with the operator, the required reason and an end or review
  date (the store refuses one without, and writes its audit row), decided now
  and settled when a pass agrees; nothing is written to the target. The UI's
  reason/date input on that action is 3.8.
- *Retired fields* (Opus review, 11 Nov): a field the manifest drops keeps
  its role (never re-purposed) and is recorded in `targets.retired_fields`
  (migration 000090, additive); `addons.FieldRoles` returns only declared
  fields, so `Desired`, `resolveLifecycle`, `WithdrawsOnly` and the resolver
  never send a retired field (the add-on refuses an undeclared one, which made
  every apply, departure locks included, `target_refused`); a mapping or own
  value on a retired field confers nothing. Declared again, it returns. The
  account can never be retired (exactly one per manifest, and only the stored
  one). Mapping validation and the UI's schema read the accepted manifest,
  which never holds a retired field. Rows queued before the retirement still
  name the field (a desired state is frozen when queued, and sent as planned),
  so `db.SaveTargetFieldRoles`, in the transaction that records a newly
  retired field, supersedes the target's `pending` rows whose snapshot names
  it, with the reason and an audit row; a claimed (`in_flight`) row is never
  touched, and a superseded row holds no one (the `target_refused` hold reads
  `failed` rows only). The next drift sweep queues the person again from the
  current roles (Opus re-review, 11 Nov).
- *No role map yet* (Opus review): a target with no persisted role map is
  "not read" on the member's and the operator's pages (not entitled, no
  resources), never a 500 for the page; a role change skips such a target with
  a logged reason (`convergeOn`) instead of rolling back, and the drift sweep
  converges it once its manifest is accepted. Any other read failure still
  fails.
- *2.5 guards*: (a) stricter than lookup sites: every string literal outside
  a struct tag under `services/`, `handlers/`, `addons/` (drift included) is
  checked, with a per-file allowlist (`handlers/kinds/storage.go` "smb",
  Zitadel's "group" in `services/roles.go`, the insights lens's "group" in
  `services/insights_lens.go` and `handlers/insights.go`, the claim probe's
  "enabled"), and an allowlist entry no longer used fails too; `truenas` in
  any literal outside `addons/registry.go`. (b) resolves each package
  identifier through the file's imports (alias, else the path's last element)
  and watches `net/http` (`Client`, `Transport`, `NewRequest*`,
  `DefaultClient`, the package-level helpers, naming `*http.Client` as a
  type), `net` and `crypto/tls` (`Dial*`), so a renamed import is caught; a
  stale entry fails.
- *Migration 000089* swaps the CHECK (validated on the way up); its down
  restores the 000087 form `NOT VALID`, so a withdraw-only row written for a
  non-TrueNAS account cannot strand a rollback.

Added to slice 2 by the coordinator (11 Nov, found by the slice-3 agent),
built: `GET /targets` carries `kind` and the accepted `entitlement_schema`
(name, role, label, unit, combine, unlimited, per_person) and nothing before a
manifest is accepted; the member's own `/me/targets` (and the operator's
person view, which shares it) carries the same two and the member's own
account `standing`, from the `storage.status` reply about that account
(owner decision, 11 Nov: `standing` added to the storage pocket in contract
v2, sent by the TrueNAS add-on; no `/subjects` read per page); the
inventory's bound rows carry `standing` beside the binding (unmanaged rows
already did). See 3.7.

- [x] 2.1 `services/entitlement.go`: `LifecycleState{Account, Channels map[string]bool, NotAMember, Reason}`;
  `resolveLifecycle` and `applyDirectoryStanding` over the `account`/`channel`
  fields read from **`targets.field_roles`** (1.3), never the live manifest;
  `Desired()` writes every lifecycle field, a withdraw-only set is those
  alone; `WithdrawsOnly()` reads the `account` field by role; `FieldEnabled`,
  `FieldSMBEnabled`, `IsLifecycleField` deleted and **every** caller moved:
  `handlers/revocation.go:99`, `handlers/merge_findings.go:105,574`,
  `handlers/mappings.go:325`, `services/role_members.go:358`, plus those in
  2.4. `TestWithdrawsOnly`, `TestWithdrawOnlyDesiredIsTheSwitchAlone` and
  the §37 suite pass against a manifest with a different lifecycle name
  (`active`), and a revocation plans with the add-on unreachable and the
  registry empty (the role map is the column), mutation-checked #ready-for-agent
- [x] 2.2 Migration: the `000087` CHECK on `desired_state_snapshots.withdraws_only`
  becomes the target-agnostic form, verified on Postgres 16:
  `CHECK (NOT withdraws_only OR (state_json <> '{}'::jsonb AND NOT jsonb_path_exists(state_json, 'strict $.* ? (@.type() != "boolean" || @ == true)')))`
  (only `account`/`channel` are JSON booleans on the wire; no column, no
  registry read, a CHECK cannot reference another table; the lax
  `'$.* ? (@ != false)'` form accepted arrays, because a non-boolean
  compared with a boolean is unknown). Live-DB test with exactly these five
  rows under `withdraws_only = true`: `{"enabled":false,"smb_enabled":false}`
  accepted; `{"enabled":false,"smb_enabled":false,"group":["admins"]}`
  refused; `{"enabled":true}` refused; `{"enabled":"false"}` refused; `{}`
  refused; plus every existing row re-validates #ready-for-agent
- [x] 2.3 Mapping validation by role (`mappings.go`): `account`/`channel`
  not bindable; `limit` parses as a non-negative integer; `flag` accepts only
  `"true"`; `mapping_plan.go:174` wording from `label` #ready-for-agent
- [x] 2.4 `handlers/target_homes.go`, `my_storage.go`, `person_storage.go`,
  `services/role_members.go:306`, `drift/addon.go:723`: read the field by
  role (`limit` with `unit: bytes`, the `flag`, the `membership`); the
  storage kind's view (`handlers/kinds/storage.go`) owns `smb://`/UNC from
  `connection.protocol`; `read.go:755-781` deleted #ready-for-agent
- [x] 2.5 Two guard tests. (a) `backend/internal/addons/no_target_words_test.go`:
  the literals `"group"`, `"home"`, `"home_quota"`, `"enabled"`,
  `"smb_enabled"`, `"smb"` appear at no manifest-lookup site
  (`Fields[...]`, `FieldsByRole`, `state[...]`, `desired[...]`) in a non-test
  file under `services/`, `handlers/`, `addons/` and `drift/` except
  `handlers/kinds/storage.go`, with an allowlist for the legitimate uses in
  `zitadel/client.go:190`, `models/models.go:1049`, `services/roles.go:29`
  and `handlers/claim_probe.go:450` (a repo-wide ban is unbuildable); the
  same for the word `truenas` outside `addons/registry.go` config parsing.
  (b) **Boundary guard** `no_second_client_test.go`: `http.Client`,
  `http.NewRequest` and `net.Dial` are constructed under `backend/internal`
  only in an explicit allowlist of files (no line numbers), each entry
  with a one-line reason: `addons/transport.go` (the pipe),
  `addons/credentials.go` (the pinned-TLS client, `newAddonClient`),
  `zitadel/client.go`, `zitadel/token.go`, `zitadel/deps.go` (the Zitadel
  client), `auth/jwt.go` (JWKS), `handlers/deps.go` (the probe client),
  `handlers/claim_probe.go` (the claim probe), `handlers/claim_watch.go`
  (the dead-man ping); there is no Google client. The test *guards* "Syndra talks only to add-ons"; its comment says
  it cannot prove it, since a scan cannot see where an operator-set URL
  points #ready-for-agent
- [ ] 2.6 Staging run: a mapping edit, a departure (withdraw-only), a
  suspension and a home limit each plan and apply as before; Pending changes,
  the person's page and the holds review read the same words #ready-for-human
- [x] 2.7 (from slice 1) The drift sweep compares a `limit` field: the
  desired limit combined the way the add-on combines (`combine: max`,
  `unlimited` dominant) against the observed effective limit an add-on now
  reports in `/subjects` `state`, and a limit absent from `state` (a quota
  table the add-on could not read) is "unknown, not compared", never
  `theirs_only`; then `drift.stateByBinding` stops dropping limit-role fields
  (slice 1 drops them, so no limit is compared, as before contract 2).
  Merge bases recorded before this hold no limit #ready-for-agent

## 3. The UI reads roles (UI only)

- [x] 3.1 `ui/src/lib/fields.ts`: `fieldsByRole(schema)`, `accountField`,
  `limitFields`, `formatLimit(value, unit)`; the manifest's schema reaches
  the UI through the existing targets endpoint unchanged #ready-for-agent
- [x] 3.2 `lib/role-mapping-plan.ts`, `targets/HomeLimit.tsx`,
  `AddMappingDialog.tsx`, `MappingManagement.tsx`, `PeopleOnTarget.tsx`,
  `ui/Withheld.tsx`, `review/HoldsDueForReview.tsx`, `people/PersonStorage.tsx`,
  `queries/useMyStorage.ts`: by role, copy from `label`/`unit`; the
  `enabled`/`smb_enabled` sentences in `AddMappingDialog.tsx:403-416` become
  "the account" and "its {channel label}" #ready-for-agent
- [x] 3.3 `storage/MyStorage.tsx` becomes the storage kind's module
  (`components/kinds/storage/`), chosen by `kind`; a kind with no module
  (`access`, `inventory`, `machine` today) renders the core member card
  (account standing + `next`) #ready-for-agent
- [x] 3.4 Lint guard: the five field literals and `smb` are allowed only
  under `components/kinds/storage/`, `lib/fields.ts` and tests #ready-for-agent
- [x] 3.5 Copy guard run (`plain-language-copy`): every new sentence passes;
  `standing.next` always ends in a next step or "nothing for you to do" #ready-for-agent
- [x] 3.7 The UI reads field roles and kind from the backend, never a
  fallback. Backend half built in slice 2 (11 Nov 2026): `GET /targets`
  carries `kind` and the accepted `entitlement_schema` per target (absent
  before a manifest is accepted: "not read"), `/me/targets` carries the same
  two plus the member's own `standing`, and the inventory's bound and unmanaged
  rows carry `standing`. **Once that lands, the UI's `STORAGE_SCHEMA` and its
  "no kind = storage" fallback must be removed** (UI half, done 11 Nov 2026):
  `STORAGE_SCHEMA` and the "no kind = storage" rule are deleted; `lib/fields.ts`
  holds no schema. An operator page takes its schema from the `GET /targets` it
  loaded (`useFieldSchema`, `useFieldSchemaState`), a member's from
  `/me/targets` (`useSchemaSource(false)`, which never asks the roster their
  proxy refuses), a person's page from the view it loaded. A system with no
  accepted manifest has no schema or kind and renders "not read yet" with its
  next step (the member card, the mappings page, a disabled Add and Put on
  hold), never a guess; an account hold with no schema is shown as the code it
  is. Standing shows on bound and unmanaged account rows #ready-for-agent
- [x] 3.8 (from slice 2) "Take theirs" on a limit finding: the action now
  answers `adoptable: true` and needs a reason and an end or review date in
  the request (as adopting a switched-off account does); the dialog asks for
  both and says it saves the target's value as this person's own limit
  (done: a limit finding's "Use {system}'s value" opens a panel with the
  sentence, an Ends on / Review on choice and a date, sends `expires_at` or
  `review_date`, and the result shows as "Saved" in the outcome pattern where
  it was decided; a switched-off account keeps its one press) #ready-for-agent
- [x] 3.6 (from slice 1) The target page renders what the backend now
  carries: `also_says` (the manifest's keys Syndra does not use, shown raw
  under "What the add-on also says") and `last_error` when a manifest was
  refused (the reason, and that the last good one is still in use), and
  `standing` where an unmanaged account's `password_set` used to be read
  #ready-for-agent

- [x] 3.9 (found by the slice-2 review, older than this change) The audit actions
  `entitlement.<target>.superseded` and `entitlement.<target>.abandoned` have no
  wording in `ui/src/lib/audit-vocabulary.ts` (only `.enqueued` does), so Audit and
  a person's Activity show the raw key; the coverage guard cannot see actions built
  by joining strings. Add a pattern with a plain-language sentence for each, and
  extend the guard to cover joined `entitlement.<target>.*` actions (done:
  both worded in `audit-vocabulary.ts`; the guard reads the three ways the
  backend joins one, Go `"entitlement."+x+".verb"`, SQL `'allowance.' || x ||
  '.lapsed'` and `Sprintf("entitlement.%s.verb")`, with a floor per family
  (entitlement 3, allowance 1) and fails on one with no sentence,
  mutation-checked by removing a wording and by fixture lines for each form) #ready-for-agent

- [x] 3.10 (found by the slice-3 review) A door or laser system's card appears
  under "Network storage" on `/storage` and in the person page's storage
  section, because both list every kind. **Owner decided 11 Nov 2026: option 1,
  plus a member sidebar item per live add-on**: the storage surfaces show only
  kind `storage`; every other live add-on gets its own member sidebar item when
  it goes live, and the person page's access section shows the other kinds.
  Design: `310-design` note (11 Nov), with the owner's answers overriding it:
  name = `targets.name`, set on the system's page, default the manifest's
  `product`, then the id; **a registration with no accepted manifest is
  operators-only: no member row, no member card**; no placeholder row; the
  touch bar keeps its ≤4-tabs-then-sheet rule. Built as 3.10a–h:
  - [x] 3.10a Backend: migration **000091** `targets.name TEXT` (nullable,
    `targets_name_check`: never blank, ≤ 60 chars; additive, down drops it).
    `handlers.systemName` is the one coalescing function (operator's name, else
    the accepted manifest's `product`, else the id title-cased) and fills `name`
    (+ `name_set`) on `GET /targets` and `name` on `GET /me/targets` and a
    person's targets. `PUT /api/v1/targets/{target}/name {"name": ...}`
    (operator, `decodeJSONStrict`, unregistered → 404, blank clears, too long →
    400 `VALIDATION_FAILED` `{name: too_long}`); `db.SetTargetName` writes the
    name and a `target.named` audit row `{from, to}` in one transaction.
    Tests: coalescing (set / product / title-case), both rosters carry it,
    strict JSON / 404 / clear / too long, live: write + audit + clear + CHECK
    refusal with no audit row, migration guard.
  - [x] 3.10b `lib/nav.ts`: `NavLeaf.family`, the family seat
    `Connected systems /systems` last in `MEMBER_NAV` (pattern
    `^/systems/[^/]+$`), `memberNav(systems)` (one row per system whose kind is
    known and not `storage`, `sortByName` by `name`), `memberSystems`,
    `STORAGE_KIND`; `targetNav`/`operatorNav` take `{target, name}`;
    `targetLabel` is the id title-cased only (the map is gone; a source guard
    forbids an object literal in it); `crumbsFor(path, audience, nameOf)` names
    `/system/targets/{t}` and `/systems/{t}` by the roster's name and never the
    seat; `MEMBER_ROUTES` and `leafMatches` skip the seat's own href.
  - [x] 3.10c Shell: `Sidebar`, `TouchNav`, `GoToHost` draw
    `memberNav(useMyStorage(member).data)`; `TopBar` passes
    `useSchemaSource(isOperator).nameOf` to `crumbsFor`. Middleware unchanged:
    reachability is the seat's pattern.
  - [x] 3.10d `app/systems/[target]` (`member/MemberSystemPage`): the kind's
    card under a header named by `name`; a storage-kind system
    `router.replace("/storage")`; not connected or not read → "This system
    isn't connected here." + "Nothing for you to do." + My access. `/storage`
    (`kinds/storage/StoragePage`, moved from `member/MySystems`) lists kind
    `storage` only.
  - [x] 3.10e `MemberTargetCard` draws nothing for a system with no kind (the
    member `NotReadCard` is gone); `CoreMemberCard` reads `view.name`, has the
    offline Notice ("{name} is offline", the page updates on its own), the
    unentitled step "Ask for access in Requests" (`/requests?ask`), and
    "Nothing for you to do. Syndra makes it." while the account is made. Every
    surface reads `name`: the storage card, the person cards, the system page,
    the roster page, and the operator pages through `useSystemNames()`
    (`lib/queries/useFieldSchema.ts`, the roster's `name`, else the id); pure
    helpers (`describeAction`, `describeSends`, `describeDrain`,
    `systemsOf`) take a `nameOf` from their component.
  - [x] 3.10f Person page: `PersonStorage` moved to
    `components/kinds/storage/` and filtered to kind `storage`;
    `people/PersonSystems.tsx` holds the kind-agnostic parts (rows, status,
    hold row, `offlineNext`, `OfflineNotice`) and the new **Connected
    systems** section after it: one `PersonSystemCard` per other system,
    including not read (operators only): account, the add-on's standing,
    roles reaching, hold. Chips are named by `name`, storage first, then the
    rest, each alphabetical. The operator system page has a **Name** region
    (`targets/TargetName.tsx`), the one write place.
  - [x] 3.10g Tests and guards: `nav.test` (rows alphabetical from a shuffled
    fixture with "Étage" and "big laser", storage and unkinded give no row, no
    placeholder, no count, every drawn row reachable, `/systems` not, crumbs,
    the `targetLabel` source guard), `middleware.test` (every href
    `memberNav(fixture)` renders passes; `/system/targets/x`, `/systems` and
    `/systems/x/y` redirect; the `MEMBER_ROUTES` canary kept), `Sidebar` /
    `TouchNav` (member rows, double render with zero and non-zero counts,
    fourth tab then sheet), `AddOnSays` (every core-card state with its step;
    no card unread). The storage-word lint guard (`eslint.config.mjs`):
    `folder(s)`, `share(s)`, `SMB`, `mount…`, `Network storage` as a literal,
    any template element or JSX text, allowed only in
    `components/kinds/storage/`, `app/storage/`, `lib/nav.ts` and tests; the
    `no-restricted-syntax` blocks re-cut into five that partition the files,
    each listing every selector set it owes (invalidation, field names,
    storage words). `kind-word-guard.test.ts` is the mutation check (ten
    mutations, each failing it, including each allow-list entry and each
    re-cut block). Moved to make it pass: the home-folder setup card and
    dialog (`kinds/storage/HomeLocation`), the mapping dialog's home fields
    (`HomeMapping`), "what each group opens" (`MappingReach`), TargetOverview's
    TrueNAS service words (`readings`), the home-place unconfirmed-change copy
    (`unconfirmed`), the activity coverage lines (`ActivityCoverage`),
    `HomeLimit`; the glossary and ShapeCard reworded product-free;
    `headerLine` reads the storage row's label from `MEMBER_NAV`, AccountSheet
    its caption from `kinds/storage/captions`; two verbs ("shares", "share a
    day") carry a reasoned `eslint-disable-next-line`.
  - [x] 3.10h Specs: this list, `design.md` §6 (the name), `basic-advanced-ia`
    §1 (the family seat), `NEXT.md`.
  - [x] 3.10i Review fixes (11 Nov 2026), `design.md` §6.1 updated:
    - The header's "set a password" step is `needsPassword(view)`
      (`kinds/storage/state.ts`): the storage card's own password step
      (`stateOf`/`PASSWORD_STEP`, moved there from `MyStorage`), only for kind
      `storage`; a door, a laser or a system not read never asks. No other
      kind-blind credential read exists.
    - `versionLabel` also strips the whole product name off a version that
      starts with it ("TrueNAS-SCALE-24.04.0" beside "TrueNAS"), as text.
    - The Name hint for a named storage system says "Network storage page"
      (it has no menu row of its own); the account hold is found by the
      schema's account ROLE (`isHold`, `kinds/storage/state.ts`), never the
      field name `enabled`.
    - `product` is a display name: the TrueNAS add-on reports `"TrueNAS"` from
      one constant (`productName`) that its fixture test pins; fixtures,
      `contract_routes_test`, the schema's `product` description and the
      CHANGELOG follow. Nothing keyed on it. No name seeded by migration.
    - `GET /api/v1/me/systems` (no add-on call; registry, accepted kind,
      `targets.name`; sorted by `textsort`) is the one source for the member
      rail, `TouchNav`, `GoToHost`, the `/systems/{target}` gate and crumb, and
      the account sheet (`accountNav`). On the proxy's member allowlist. The
      touch bar holds an empty seat while it loads.
    - `handlers.memberMeets` is the one server-side rule: `/me/systems` and
      `/me/targets` skip a registration with no kind. UI: `isStorage` and
      `memberSystems` (`lib/nav.ts`) are the only kind predicates; the card's
      kindless guard and `PersonSystems`' copy of `isStorage` are gone;
      `crumbsFor` no longer crumbs `/systems/{t}` (the page's `useCrumb`).
    - `nameOf` is required on `describeAction`, `describeSends`, `systemsOf`,
      `describeDrain(result, nameOf, auto)` and `outcomeFromDrain(result,
      nameOf, auto, total)`; the audit CSV uses `useSystemNames()`.
    - `GET /targets` carries `name_from` (`operator`/`product`/`id`) in place
      of `name_set`; the Name hint is worded per case, not read included.
      `versionLabel` strips the product's name (compared as text), not the
      system's, so an operator's name cannot break the pattern or double it.
    - Name limit: one parity test (`name-limit.test.ts`) ties the CHECK, the
      Go refusal and `NAME_MAX`.
    - Copy: `/storage` empty state "No shared storage to show yet." +
      "Nothing for you to do."; the `/systems/{t}` card drops the title its
      page heading already says.
    - `migrate_rollback_test` restores the recorded version, not `newest`.

Decided while building (slice 3, 11 Nov 2026), for the owner:

- The field hooks read the response the page loaded (3.7): `useFieldSchema`
  and `useMappingFields` the roster, `useSchemaSource(isOperator)` the roster or
  `/me/targets` (only the one it needs is fetched); `Withheld` takes a
  `schemaOf` from its caller.
- `role-mapping-plan.ts` keeps `RoleRows {group, home, quota}` and takes the
  field names as a required trailing `fields` argument; the names come from
  `mappingFields(schema)` (first field of each role).
- Words now come from the add-on's `label`: "home folder size" in the version
  history and findings reads "home folder limit" (the add-on's label), and a
  flag reads by its label minus a leading "Gives a" ("home folder"). The
  refused-fields note reads "Some settings cannot be mapped, on purpose: the
  account and its SMB." (the channel's label), one sentence for any kind.
- The lint guard is one `no-restricted-syntax` block set (a later block would
  replace an earlier one, so the invalidation selectors are repeated in each).
  `home`, `home_quota`, `enabled`, `smb_enabled` and `smb` are refused as any
  string literal; `group` only where it names a field (compared with `.field`, a
  `field=` prop, a `field:` property, a `FIELD_*` constant), since it is also a
  layout word. `field-literal-guard.test.ts` is the mutation check. Left on
  purpose: `smb_enabled` as a TypeScript property name in `useMyStorage.ts` (a
  wire key, not a literal) and `value="true"` in the hold dialog.
- `HomeLimit.tsx`, `PersonStorage.tsx` and `HoldsDueForReview.tsx` stay where
  they are (they read by role now); only `MyStorage`, `ConnectionGuide` and
  `device` moved to `components/kinds/storage/`. The kind registry is
  `components/kinds/MemberTargetCard.tsx`; `CoreMemberCard` is the card for a
  kind with no module.
- Review fixes (11 Nov 2026), in one line each:
  A. A limit field's values in a finding read by role through `limitWordsFor`
  ("100 GB", "no limit" for the field's own unlimited); other roles unchanged.
  B. `HoldsDueForReview` words an own value from the hold's field
  (`limitWordsFor`, `fieldNoun`), not "home folder limit"/bytes/0; not read says so.
  C. One schema-state shape `{schema, loading, error, refetch}` (`useFieldSchemaState`;
  `useSchemaSource` is the same facts as props of the lookup function).
  `components/states/SchemaState.tsx` is the one place that words them:
  `SchemaNotReady` (skeleton / failed read with Try again / "not read yet", only
  when the read succeeded with no schema) and `schemaReason` for a disabled
  control. Used by MappingManagement, Add a mapping, Put on hold, MergeFindings,
  HoldsDueForReview, Withheld.
  D. "Use {system}'s value" is off with its reason until the fields are known.
  E. `targets/UntilChoice.tsx` is the one Ends on / Review on chooser (hint and
  outcome words) for `HomeLimit` and the finding panel; a review date is said to
  stay until somebody keeps or ends it. Save says why when the reason is empty.
  F. Audit coverage guard sees Go, SQL and Sprintf joins, floor per family.
  G. Lint guard also refuses template literals, `switch (x.field)` and a bare
  `field` identifier (mutants in `field-literal-guard.test.ts`).
  H. Production code may not import `__fixtures__` (`no-restricted-imports`);
  `truenas-schema.ts` reads `addons/contract/v2/fixtures/manifest_truenas.json`,
  the file the add-on's Go test keeps equal to `capabilities.go`.
  I. Core copy by role: glossary PROVISION generic; "map roles to {mappedTo}",
  `membershipNoun` in mapping dialogs, finding policy line. The home-folder
  setup card, the mapping dialog's home block, "what each group opens" and
  `TargetOverview`'s TrueNAS service words moved under
  `components/kinds/storage/` in 3.10g. Still drawn by the storage module, not
  by role: the mapping dialog's home fields (`HomeChoice`).
  J. The `/storage` page shell moved to `components/member/MySystems.tsx`
  (since 3.10d: `components/kinds/storage/StoragePage.tsx`);
  `kinds/storage/MyStorage.tsx` exports only the storage card; no import cycle.
- Review of the fix round (11 Nov 2026), folded into 3.10, one line each:
  2. `__fixtures__/truenas-schema.ts` reads the contract fixture at test time
  (`readFileSync`), not by import: the UI image builds from `ui/` alone, where
  `addons/` is absent, and `next build` type-checks `__fixtures__` (TS2307).
  One source kept; checked by `tsc` and `next build` on a copy of `ui/` alone.
  3. `sourceOf` (useFieldSchema) reports a failed read only when there are no
  rows (`error: data ? null : error`): a failed 30s roster refresh no longer
  blanks MergeFindings (and its typed reason), HoldsDue or Withheld.
  MergeFindings says which read failed. Unit-tested, mutation-checked.
  4. `fields.oneMappedTo`: "one group on TrueNAS" / "something on TrueNAS"
  (no cut plural, no article, no built plural); MappingScreen uses it.
  5. HomeLimit's Save says why it is off (`schemaReason`, then "Say why…",
  then "Pick the day…", then "Say how large the limit is, in GB."), the chain
  MergeFindings uses plus the size it also needs; a test per reason.
  6. `useSchemaSource(isOperator, enabled)`: My access asks for the fields
  only when a hold is in force. (Since 3.10 the member rail reads
  `/me/targets` for its rows on every page, so this saves the hold card's
  wait, not the read.)
  7. `WithheldInline` says "still reading" / "could not read… Reload the page"
  (`schemaReason`) instead of the raw value while the fields are not known.
- `standing.next` falls back to a line from who is waited on, or "Nothing for
  you to do." (`lib/account-standing.ts`), so no standing ends without a step.

## 4. The conformance kit and the reference add-on

Decisions 5 and 8 accepted 10 Nov 2026. **Core only, built before slice 1
deploys** (design §12): the kit is written against the slice-0 and slice-1a
schemas and run against the v2 TrueNAS add-on on staging as the gate for
1.10. The feature halves (reference and checks) are slice 6 (6.0, 6.11).

Decided while building (10 Nov 2026; where design §7 was silent, and the
review of the same day): each module is a library plus `cmd/`
(`cmd/reference-addon`, `cmd/syndra-addon-conform`) so the kit's self-tests
serve the reference in-process; the kit requires the reference by `replace
../reference` for its tests only, and a test proves the binary does not link
it. The kit embeds the contract as §7 says: `addons/contract` gained an
embed-only module (`embed.go`, files only, no Go type, so §1's rejection of a
shared types module stands), the kit imports it by `replace ../contract`, and
there is no override flag; nothing is fetched. Results are `pass|fail|skip` in
the §7 table's order, plus `feature.<name>` skip rows (slice-6 features; an
unknown feature, whose top-level manifest keys are then not judged);
`dangerous-scopes` is advisory (`WARN`, exit unaffected). Exit 0 / 1 (a MUST
failed) / 2 (could not start, or `-mutate` refused) / 130 (interrupted, after
putting things back). Reads only means GET routes and `/plan`. `-mutate`
writes only to a subject named `conformance-...`, refuses to write at all once
a read has failed a MUST, and refuses a subject whose plan carries an
`account_id`, `standing` or `conflict` (by presence, any type) or any effect
but `apply`; the withdraw-only and refusal probes run before the account
exists; `read_only` is remembered before the request is sent; SIGINT/SIGTERM
stop it between requests (an interrupt is exit 130 wherever it lands; nothing
asserted after it is judged, and the checks judged across steps or over every
reply say "not judged" rather than pass); a create answered with an account
that was there before the run fails `create-is-new` and stops the run without
touching that account. Whose account an id is has one rule in one function
(`owner.go`): until the kit's creating apply answers 200 with a string
`account_id` that was not there before the run, the kit owns nothing and any
id named for its subject (plan, apply, operation, the end-of-run plan) is
foreign; after it, the kit owns exactly that id; only an absent `account_id`
is "no account", and null, "" or a non-string is foreign and never claimable;
a claimed account must also be labelled as the kit's (its username, from the
create's reply or else /subjects, begins `conformance-`, so `-email` must too),
and a 2xx plan or apply after the create that names no account fails
`bound-to-created`. After the create, every write to the subject (converge,
replay, operations, the lifecycle applies, the end-of-run disable) goes through
one gate, `toOwned`, which sends only when the plan read just before it names
exactly the owned account, and otherwise sends nothing and stops the kit
writing; `send` refuses any after-create write to the subject that did not come
through it.
Every reply about the subject goes through it, and no code outside `owner.go`
reads `account_id`; a foreign id fails `bound-to-created` and freezes every
apply and operation, for any subject, with no exception, the end of the run included,
so the end never adopts an id it first sees there. The last line lists, with
step and body (never a secret value), every write that may have reached a
foreign account (its reply named it, was not a 2xx, never came, or was the
create or came after it without naming the kit's own account, as an
operation's never does) and says
"did not touch" only when none could have; a step cut short by a freeze, or by
the run stopping before it, marks the checks it judges "not judged"; "one
disabled account" is said only when the end-of-run plan was read and confirmed
it, otherwise "NOT confirmed disabled" with the reason and `bound-to-created`
not judged; it refuses to write if the accounts before the first write cannot
be read, are read truncated, or are not a current read; and whatever happened
after the first write, it ends by putting lifecycle back to `active`, waiting
(about a minute, with backoff) for `health.in_flight` 0, and re-planning the
subject through the same gate: it disables the account only when that plan
names exactly the account it owns. An account first named after a create that
never answered (or answered no account) is never adopted: it is foreign, left
as it is, and reported with the writes that may have made it, for the owner to
check; a rebound subject is never written to. "No account was created" is
said only after a drained plan with every write answered; otherwise the last
line says `UNKNOWN` and why, and it also names any account the kit does not
own that its subject was answered with, and any other account that appeared
during the run. Writing a target-wide
setting back is its own opt-in, `-mutate-settings` (not in §7's table).
`log_records` must advance across the creating apply and the first
`password.set`, and a count that cannot be read never passes "nothing
written"; without `-mutate`, `health` and `required-fields`, which each have a
half only writes can judge, report "not judged" unless they failed. Beyond the table's wording, all from §4/§5: `current` (two reads
1.1 s apart that both say `current: true` must move `taken_at` forward), an
unknown route answering 404 `UNKNOWN_ROUTE` (a code added to
`v2/registry/errors.json` and named in `openapi.yaml`: every non-2xx needs a
registered code and an undeclared route had none), a PLAN_STALE-then-replan
probe under `replay.refusal`, and a managed value held before the withdraw of
an existing account. `plan = apply` is asserted on every converge the kit does,
the `password.set` replay with another password sits under `operations`,
`replay.29d` is skip (no `-clock`), observed limits are `[effective]`, the
kit's own requests are validated against the request schemas, and replies are
capped at `constants.max_reply_bytes`. The reference is a print queue (`kind:
machine`, one field per role, a builtin `lobby` printer as the unlisted value);
its handlers read as a correct add-on, every defect lives in `defects.go`; its
request identity reproduces the four JCS vectors; it is ~860 code lines
(plus ~175 in `defects.go`) rather than ~400: every core route, operations and
settings with the full §5 rules.

- [x] 4.1 `addons/reference`: an in-memory target with accounts, one field of
  each role, one member operation (`password.set`), lifecycle, settings;
  ~400 lines; the derived-key transport (with the raw query in the MAC)
  from `addons/truenas/transport.go` copied, not imported #ready-for-agent
- [x] 4.2 `addons/conformance`: the binary in design §7, every check in the
  table, `pass|fail|skip` with the MUST it enforces, JSON and text report,
  non-zero on a MUST failure, `--mutate` gated, reads only without it #ready-for-agent
- [x] 4.3 CI: the kit runs against the reference add-on on every push
  (`ci.yml` job `addon-conformance`); the job is required for `ci-ok` #ready-for-agent
- [x] 4.4 The kit's own tests: each check is watched to fail against a
  deliberately broken reference (a flag per defect: ignores fingerprint,
  creates on withdraw-only, echoes the secret, answers a replay after the
  fingerprint check, remembers a refusal, omits `truncated`, says
  `current: true` from its mirror, accepts an altered query, treats
  max(`"0"`, 5 GiB) as 5 GiB, lets a withdraw-only apply touch a membership) #ready-for-agent
- [ ] 4.4a (found by the last slice-4 review, below its bar) A `-mutate` run refused
  before any write (subject exists, `before` unreadable or not current, a reads
  MUST failed) still reports `health` and `required-fields` as pass on their read
  halves; give it the reads-only "not judged: needs a write" marking too. Report
  wording only: such a run already exits 2 #ready-for-agent
- [ ] 4.4b (found by the guide review, finding 4) The kit owns only an account
  whose `username` begins `conformance-` (`owner` in `addons/conformance/owner.go`),
  so an app that cannot hold that name (generated ids, display names only, a
  length or charset the prefix breaks) can never pass `-mutate`. Not done now:
  every fallback found (claim by `account_id` alone, by a label in another
  field the kit cannot read generically, by the subject's email) weakens
  `owner()`'s rule that a member's account answered as the create's is never
  the kit's, which is the one thing the kit must not get wrong on a real
  instance. Needs a design that names how an add-on proves an account is the
  kit's without the prefix (a manifest-declared label field, say) before any
  code. The guide states the rule meanwhile (The one create) #needs-triage
- [ ] 4.5 Reads-only run against production TrueNAS from the app host;
  report attached to this change #ready-for-human
- [ ] 4.6 `--mutate` run against staging TrueNAS with a throwaway subject,
  against the **v2** add-on before 1.10; report attached; the kit's last
  line names the one disabled, labelled account it leaves (removal disables,
  never deletes) #ready-for-human

## 5. The guide

- [x] 5.1 `addons/AUTHORING.md` to the outline in design §8, every rule
  BCP 14 (RFC 2119 + RFC 8174), every core route with its fixture inline, the reference add-on
  walked file by file; §1 opens with the boundary rule (the add-on alone
  holds the app's address and credential and alone talks to it; Syndra
  talks only to add-ons) and §4 carries the freshness contract and the
  `stale`-not-error rule #ready-for-agent
  *Decided while writing (11 Nov 2026):* statistics is its own chapter 8,
  between Operations and Security, so §8's fourteen chapters are fifteen
  (§8's 8–14 are 9–15). `GET /capabilities`'s fixture is chapter 3's
  (`manifest.json`, which shows every key); the settings routes, which §8
  gives no home, are a feature section at the end of chapter 4. The
  reference is ~860 lines, not ~400, and is walked as a file:line table.
  The guard is `addons/contract/guide_test.go` (the module that already
  embeds the files; it reads `../AUTHORING.md` from disk), run by the
  existing `addon-conformance` CI step; mutation-checked (a changed byte in
  an inline fixture, a route heading removed, `/settings` alone removed
  while `/settings/{name}` stays, a marker at a missing file, a marker
  parted from its block: each fails). `by_status` resolves as
  `addons/truenas/errors.go` does (`backoff` on 429 and 5xx, else `never`):
  the registry says only "the status says".
- [x] 5.1a (found writing 5.1) `v2/fixtures/subjects.json` and
  `subjects_stale.json` carry `home_quota: ["0","5368709120"]` in `state`,
  where the rule is the one effective value (design §3,
  `apply_response.json`'s `observed`, the kit's `limit-combine`,
  `addons/truenas/server.go:388`). The guide states the rule and flags the
  fixture; correcting it is a contract-file change for both ends' suites.
  Also not yet expressible: `common/outcome.json` has only `succeeded`, so
  the `physical` rule 4 words (`no_change`, `conflict`) have no wire form
  until 6.3 #needs-triage
  *Resolved (Opus review ruling, 11 Nov 2026):* (a) the fixture was wrong.
  `subjects.json` holds `home_quota: ["5368709120"]` (as `apply_response.json`'s
  `observed`); `subjects_stale.json` drops it (the TrueNAS mirror holds no
  limits: absent is unknown); `common/field.json` says desired is one value
  per role and state/observed exactly one; the kit's shape check fails a
  limit in state or observed that is not one value. (b) "conflict" is never
  an outcome, it is a refusal (problem+json 409, like `BINDING_CONFLICT`);
  "changed nothing" is a real slice-6 gap, now task 6.3a.
- [x] 5.2 Mapping sketches for UniFi Access, Ruby and the inventory system as
  the guide's §12, from the two research notes and design §10; each names
  its unverified items as such (the inventory one names its API as unknown
  and lists the four questions in design §10) #ready-for-agent
  *Decided while writing:* chapter 13, with TrueNAS (shipped) first. Ruby's
  `materials` flag is `MdbAdmin` + `CanModifyMdb`, and its `role`
  membership lists only `User` and `Admin`, so two fields never contend for
  one Ruby role. UniFi has no version endpoint, so its gate is per probed
  endpoint.
- [x] 5.2a The guide's statistics chapter: `metrics[]`, the collector is the
  add-on's (schedule, backoff, the app's rate limits), the buffer and its
  bounds, a point is the whole UTC day from the app's log (not a delta,
  never an explicit zero), the per-metric watermark and when not to move it,
  a gap is never a zero, no local name for an unbound account, the total
  beside every per-subject point, `against`; and the
  operations chapter's `result`, `effect`, and the six `physical` rules
  with the inventory checkout as the worked case #ready-for-agent
  *Decided while writing:* chapters 7 and 8. Every slice-6 part is marked
  "Designed, not yet served by Syndra", including `effect` (the backend's
  policy does not read it yet) and the ids, codes and `format: target_id`
  the registry does not hold yet, which chapter 7 lists by name.
- [x] 5.1b The Opus review of the draft guide (findings 1-17 and its writing
  notes) fixed in the guide and, where it found the contract or the code
  wrong, there first #ready-for-agent
  *Decided while writing (11 Nov 2026):*
  - A write that sent the app nothing answers 503 `TARGET_UNREACHABLE` with
    `Retry-After` (the registry meaning now covers writes); one that may
    have reached it stays `APPLY_FAILED`/`OPERATION_FAILED` 5xx. "Sent
    nothing" is decided by where the failure happened, not by sniffing
    `net.OpError`/`syscall.ECONNREFUSED`: in TrueNAS, `ErrNotSent` is any
    failure to get a session (dial, TLS, login, the breaker, the reconnect
    cooldown) and nothing else, because every such error is certainly before
    the call left, and any error on an established socket may be after. Only
    before the first write of a request: `acted` drops it after
    `group.create`. A read that timed out before any write still answers
    `APPLY_FAILED` (conservative; a widening for later). Applied to apply,
    `password.set`, `password.rotate` and `PUT /settings`; reads unchanged.
  - The TrueNAS read/write split is now the registry's `effect`
    (`readOperation`, tested equal to it): `account.release` was treated as
    a read, so it was neither deduplicated nor refused while `read_only`.
  - `log_head` is never empty (`health.json` `minLength: 1`, the digest of
    the empty log until the first record); both add-ons reported `""` on a
    fresh log, which Syndra refuses at first sight. The kit checks movement
    across every `/health` of a run (`logAnchored`), Syndra's three rules.
  - A `limit` given `[]` sets nothing. The reference cleared it; a new
    reference account starts at the unlimited value so `state` always holds
    one. Kit check `limit-empty` (sets 5 GiB, then `[]`, wants 5 GiB).
  - Read operations ignore `call_id`. The reference gained `health.get` so
    the kit's new `operations.reads` check (reads under the creating apply's
    `call_id`) runs against it; three new reference defects watch
    `limit-empty`, `health` and `operations.reads` fail.
  - Fixtures: the plan/apply pair names one account (`rey_mysterio`,
    `3051`) and the apply answers a new fingerprint; `group` values sorted;
    `operation_request.json` is the `account.adopt` call
    `operation_response.json` answers (no `plan_id`, `fingerprint`,
    `preview` or `fresh`, which Syndra never sends), and that reply loses a
    stray `rotated`.
  - The guide: the designed features and the three unbuilt sketches moved
    to an appendix; code cited by symbol; one place per rule, linked by
    anchor; registration refusals listed once. Guard additions: every JSON
    block marked, routes named as `METHOD /path`, every
    `` `symbol` in `path` `` declared there, every section link landing.
- [ ] 5.3 A fresh agent with only `AUTHORING.md`, `addons/contract/` and the
  kit builds an add-on for a toy target and passes the kit; what it had to
  ask is written back into the guide #ready-for-agent
- [x] 5.4 `openspec/NEXT.md` and `architecture-deepening/tasks.md` item 6
  point here; `addon-platform/design.md` §5 and §16 gain a one-line pointer
  to this contract as the current statement #ready-for-agent
  *Decided while writing:* `addons/README.md` and `addons/contract/README.md`
  link the guide too, and `addons/README.md`'s route count now says twelve
  served and fourteen in `openapi.yaml` (design §12 slice 5: route tables
  corrected).

## 6. Four additive features: `results`, `freshness`, `calls`, `stats`

Decisions 11–21 accepted 10 Nov 2026 (each item names its own). Every item
is behind a feature the TrueNAS add-on declares only when its half is built;
nothing here is a wire break and nothing in slices 0–5 depends on it. Depends on
1a (the feature schemas are there) and slice 1; the UI items on 3.1.

- [ ] 6.0 The reference add-on's feature halves, each behind its feature
  flag so the kit's feature checks (6.11) can be written against it:
  `stats` (one counter and one gauge with one dimension collected on a
  timer into a bounded buffer with a per-metric watermark the reference
  holds back while its target is marked off; subject-less totals; keyset
  pages), `freshness` (`fresh` with a cooldown and single-flight,
  `cached` from the mirror, observable through `target_reads`), `results`
  (`holdings.list` answering `common/records.json` through the read path),
  `calls` (`GET /calls/{call_id}`, `preview`, `CALL_UNSETTLED`) and one
  `physical` operation (`item.checkout` with `item_id`, per-subject
  serialisation, natural idempotency, refused when the target is marked
  unreachable). ~200 lines #ready-for-agent

**`results` (decision 20)**

- [ ] 6.1 `registry/operations.json[id].result` (a schema ref); an add-on
  declaring `results` answers one `result` validated against the id's
  schema, and the backend reads `result` only from such an add-on (the
  typed pockets `Activity`/`Health` stay readable from one without, and are
  deleted at the next major); **every `effect: read` operation is served
  through the one generalised read path** (`addons.MyStorage` today:
  never dispatched, never logged, never confirmed; member via
  `/me/targets`, admin for any subject via `/users/{id}/targets`), with a
  test that an admin's read of `holdings.list` writes no operations-log
  row; TrueNAS declares it and `activity.get` answers
  `common/activity.json` (`uncovered[]`, no share word), `storage.status`
  answers `common/status.json`; the UI's generic renderers for the three
  shapes, the `records` one driven by the reference add-on's
  `holdings.list` so the inventory target lands as a manifest and a binary #ready-for-agent

**`freshness` (decision 21)**

- [ ] 6.2 `fresh: true` (query on `GET /subjects`, `GET /values/{field}`,
  the storage profile reads; body on `effect: read` operations; **never**
  on `/plan`, `/apply`, `/stats`); `current` means "read after this request
  arrived" and nothing else; the invariant `current == false` ⇔ `stale`
  present (test); `FRESH_READ_COOLDOWN` (429) and `TARGET_UNREACHABLE`
  (503, classified `unreached`) registered with `Retry-After`; **the drift
  sweep's `/subjects` read sends `fresh: true`** (the only scheduled
  sender; the planner's re-plan is a `/plan` call and is untouched) and
  treats a `cooldown` answer as a skipped pass; display reads send it only
  on "Refresh" and otherwise accept `cached`; the TrueNAS add-on serves
  from its mirror as `cached`, re-reads the NAS on `fresh` within a
  per-target and per-subject cooldown (`limits.fresh_read_cooldown`),
  single-flights concurrent fresh reads, counts `health.target_reads` on
  every NAS call, and answers the last known value with `stale` when the
  NAS is unreachable (mutation-checked: a cooldown that does not bite, a
  coalescing that makes two reads, a `stale` that is an error, a mirror
  answer marked `current`, a drift sweep that omits `fresh`, a `/plan` that
  is cooled down). The UI shows `taken_at` on every card and a "Refresh"
  that sends `fresh: true` #ready-for-agent

**`calls` and `effect` (decision 18)**

- [ ] 6.3 `registry/operations.json[id].effect` ∈ `read|account|credential|physical`
  and `secret_result` for every id (today's are `read`, `account`,
  `credential`; none is `physical`; none has a secret reply); `GET
  /calls/{call_id}` → `{outcome: recorded|in_doubt|unknown, result?}` in
  the reference add-on and TrueNAS, the remembered reply **redacted** when
  `secret_result`; `preview: true` sent only to an add-on declaring
  `calls`, never recorded, never with a `call_id`, the confirm with a fresh
  one; the backend refuses to dispatch a `physical` operation without
  `confirm` or without `calls` declared, shows the preview's `result` in
  the confirm, never resends an `indeterminate` physical call, polls
  `/calls/{call_id}` on the operations page, raises a `ready-for-human`
  item after `constants.in_doubt_bound`; the errors `CALL_UNSETTLED` (409),
  `UNKNOWN_RESOURCE` (404), `NOT_ENTITLED` (403), `LIMIT_REACHED` (409)
  registered with their `detail` shapes; `params[].format: target_id`
  renders an input with a scanner affordance. Tests on the reference's
  `item.checkout`: a second identical call is `no_change`, a concurrent
  pair passes the limit once, an `in_doubt` call is never re-executed and a
  retry meanwhile is `CALL_UNSETTLED`, a preview leaves nothing behind
  (`/calls` on any never-used id → `unknown`, `log_records` unchanged) and
  the confirm with its fresh id executes #ready-for-agent

- [ ] 6.3a (the guide review's ruling (b)) Add `no_change` to
  `common/outcome.json`, listed under feature `calls`, mirroring
  `common/effect.json`, so an operation can say it changed nothing (a
  checkout of an item already held; `log_records` stays put); and have
  Syndra read the wire `outcome` instead of classifying by status alone, so
  a no-op is not recorded and audited as done. "Conflict" is not an outcome:
  it is a refusal, problem+json 409 #ready-for-agent

**`stats` (decisions 11–17)**

- [ ] 6.4 Fixtures for `stats.json` and `manifest.metrics[]` (the schemas
  are 1a.2): one per-metric reply with points, a dimensioned gap, `unbound`
  and a keyset `next_cursor` #ready-for-agent
- [ ] 6.5 TrueNAS add-on collector: on its own timer, `audit.query` for SMB
  sessions per bound account per share into `storage.sessions` (a UTC day,
  the whole day's total, a `gap` per `share` value auditing does not
  cover, from `sharesNotWatching`) and the per-share and home usage it
  already reads for `storage.status` and `/homes` into `storage.used_bytes`
  (gauge, last sample of the day, `against: home_quota`); a subject-less
  total for every bucket that has a per-subject point; unbound local
  accounts fold into the total and `unbound.accounts`, never a per-subject
  point; a bounded durable buffer (`limits.stats_buffer`, default `P90D`,
  oldest dropped first, the dropped window reported as a gap) with a
  **watermark per metric held back while the NAS is unreachable**;
  `GET /stats?metric=&since=` answering one metric per call with
  `taken_at`, `watermark`, points, gaps, `unbound`, keyset pages under
  `constants.max_reply_bytes`, 404 `UNKNOWN_METRIC`; the manifest declares
  `stats` and the two descriptors. Mutation-checked: a `since` not
  honoured, a second collection that changes a final point, a watermark
  moved past a day the NAS was off, a local name on the wire, a zero where
  the audit was off, two shares' gaps colliding #ready-for-agent
- [ ] 6.6 Backend collection: hourly per target declaring `stats`, **one
  call per declared metric** with that metric's stored watermark as
  `since` (first run `now − limits.stats_buffer`), keyset-paged, never
  with `fresh`; descriptors validated at registration (unit in the
  registry, ≤ 4 dimensions, ≤ 50 metrics, `against` names a `limit` of the
  same unit, a changed `instrument`/`unit` under an existing name refused;
  a bad descriptor drops that metric and logs once); **one transaction per
  metric per collection**: the provisional window (points and gaps with
  `end > stored watermark`) deleted and replaced by the reply's, final
  points upserted by `(target_id, metric, subject, dims, bucket_start)`
  with `subject = ''` for target-wide, the watermark written last after
  the final page; **content validation**: a `subject` with no binding on
  this target, a `subject` on a metric not `per_subject`, a dimension value
  equal to a subject's `account_id` or display name, a point past
  retention: each dropped and counted; series beyond
  `constants.series_per_metric` dropped and counted. Tests: the same
  `(metric, since)` collected twice changes nothing, for points **and**
  gaps; a provisional 3 that comes back absent is gone; a day reported late
  under a held-back watermark lands and retracts its gap; one dark metric
  does not re-send another #ready-for-agent
- [ ] 6.7 Migration: `addon_metric_points` (PK `target_id, metric, subject
  NOT NULL DEFAULT '', dims, bucket_start`; no `bucket_end`),
  `addon_metric_gaps` (`dims JSONB NOT NULL DEFAULT '{}'`; PK `target_id,
  metric, dims, gap_start`),
  `addon_stats_watermarks` (per metric), and
  `targets.stats_retention_days INT NULL` for the per-target shortening
  (same migration, not a fifth) (design §9.6); nightly retention:
  per-subject points deleted after 400 days, target-wide after 5 years,
  gaps with the points they cover; no rollup; the per-target setting
  shortens either retention (one write place) and cannot lengthen it.
  Day-maths tests use `clock.Now`. Decision 13 #ready-for-agent
- [ ] 6.8 Access: `/me/targets` carries the actor's `per_subject` points
  for the last 30 days and 12 months; `GET /users/{id}/targets` (admin
  role) carries that person's with the page, beside their storage status;
  target and insights endpoints (admin role) serve the totals and the
  per-dimension series. No view audit, no separate endpoint, no
  suppression (decision 14). Tests: a member cannot read another's; a
  non-admin operator cannot read the target or insights series #ready-for-agent
- [ ] 6.9 UI, from descriptors only: the member card "What I have used"
  (counters summed with a sparkline, gauges latest with their UTC day,
  `against` as a meter over the **observed** limit, gaps footnoted, every
  card ending in a next step or "nothing for you to do"), the same card on
  the person page, the target overview's target-wide series by dimension
  with gaps hatched and the unbound-accounts line, "This
  add-on does not report statistics" when the feature is absent; the
  Insights page (Advanced view) grouped by unit with distributions and no
  names; "UTC day" wherever a date is shown; add-on text quoted and capped;
  the `dataviz` skill's rules for gap-vs-zero and both themes; copy guard
  for the new sentences and for the absence of ranking words #ready-for-agent
- [ ] 6.10 Staging: 48 hours of collection against staging TrueNAS, then an
  operator checks that the member storage card's current number matches
  `storage.status`, that the trend exists, that a share with auditing off
  shows a gap and not zero, that a day with the NAS stopped shows a gap and
  then fills after it returns, and that "Refresh" advances `taken_at` once
  and reports the cooldown the second time #ready-for-human
- [ ] 6.11 Kit checks per feature (design §7 `freshness` row, the
  `operations` row's `physical`/`preview`/`calls`/`secret_result` part, and
  the §9.9 table: descriptors, points, totals, since, idempotent, gaps, no
  people in dimensions, unbound, buffer, page size), each `skip` with the
  reason when the feature is absent; each watched to fail against the
  broken reference (a flag per defect: a mirror answer marked current, a
  point on an undeclared dimension, a local name as a subject, a zero where
  a gap belongs, a watermark moved past an unfilled window, per-subject
  points summing above the total, two app reads for two concurrent fresh
  reads, a `physical` call re-executed after `in_doubt`, a preview that is
  recorded, a secret in a `/calls` answer) #ready-for-agent

## Out of scope, stated so nobody builds it here

- The UniFi Access, Ruby and inventory add-ons themselves (each its own
  change, after 5.3; the inventory one after its API is researched, and
  with it the question of a product with no borrower profile).
- `subjects.paged` (decision 27): the snapshot cursor of design §11, built
  as its own change when a target has more members than fit in
  `max_reply_bytes`.
- Hourly buckets (`stats.hourly`, a later feature with its own rollup);
  histograms, exemplars, a Prometheus endpoint or any export of the points
  (design §9.2); a term calendar for the statistics windows.
- Response signatures (the pinned TLS already authenticates the answerer);
  a canonical query form (the raw query is signed, decision 24).
- Code generation from the schema (design §1: the day a third Go add-on or
  one UI type drift makes it cheaper than the hand-written pair).
- A second supported major at the backend (owner decision 2).
- Any change to the key derivation or its vector (the MAC input gains the
  raw query in 1.4a; the derivation does not move).
- A plugin SDK, dynamic loading, or add-ons in-process.
