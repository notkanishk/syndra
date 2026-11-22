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
  - *Rollback note (slice 7b, 13 Nov 2026):* the TrueNAS mutation log's
    records gain `account_id` (5.3d). A binary from before 7b decodes such a
    record and drops the field, so its own `VerifyChain`/`VerifyLog` would
    recompute those records' digests without it and call them altered. Both
    are test-only: no running path calls them. `Append` chains on the stored
    `digest`, and recovery (`recoverHead`, `InDoubt`, `Recover`) reads the
    stored head and the outcome fields, so a rolled-back add-on appends,
    recovers and reports its head as before, and Syndra's anchor still moves
    forward. Rollback works; only an offline verify by the old binary
    misreads the newer records.
- [ ] 1.7b A release later: drop `account_uid` from both tables and the
  dual write; the one task the earlier draft lacked; and drop
  `target_binding_conflicts` (retired in 7t-6, kept one release for the
  rollback) #ready-for-agent
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
    *Fixed (13 Nov 2026):* the operator's crumb drew the id title-cased while
    the roster was being read, then swapped it for the name. `TopBar` now
    passes `useSystemNamesIfRead` (undefined while the roster is first read,
    otherwise the one `sourceOf`'s `nameOf`) and
    `crumbsFor` stops at "System" until a name is known, as a member's page
    waits for its `useCrumb`. `MappingScreen`'s own `useCrumb` waits for the
    same read, so the trail only grows at its end (review of PR 173). Left: the
    page heading and lede (`TargetOverview`, `MappingScreen`'s subline) still
    show the id title-cased for that first read, then the name
    (`useSystemNames`). Fixing that needs a heading placeholder, so it is a
    follow-up, not this fix. #ready-for-agent
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
    (`targets/TargetName.tsx`), the one write place. *(18 Nov 2026, D-1 Q4:
    the region is now the settings card's Name row, whose Change name opens
    the same write place as a dialog; 7u-7.)*
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
  non-zero on a MUST failure, `-mutate` gated, reads only without it #ready-for-agent
- [x] 4.3 CI: the kit runs against the reference add-on on every push
  (`ci.yml` job `addon-conformance`); the job is required for `ci-ok` #ready-for-agent
  - *Done (13 Nov 2026, slice 7g):* the end-to-end step also starts a second
    reference with every feature (`-features results,freshness,calls,stats`;
    `freshness,calls,stats` since 7h, `result` being core)
    and runs reads, `-mutate -item item-0001`, reads, `-mutate
    -mutate-settings -item item-0002` against that one process, as an
    operator's runs follow each other (step bound: 8 minutes; about 30 s
    locally). The featureless run stays. Run against the pre-fix kit, the
    step exits 1 on `stats.unbound` (#51 D1, below).
- [x] 4.4 The kit's own tests: each check is watched to fail against a
  deliberately broken reference (a flag per defect: ignores fingerprint,
  creates on withdraw-only, echoes the secret, answers a replay after the
  fingerprint check, remembers a refusal, omits `truncated`, says
  `current: true` from its mirror, accepts an altered query, treats
  max(`"0"`, 5 GiB) as 5 GiB, lets a withdraw-only apply touch a membership) #ready-for-agent
- [x] 4.4a (found by the last slice-4 review, below its bar) A `-mutate` run refused
  before any write (subject exists, `before` unreadable or not current, a reads
  MUST failed) still reports `health` and `required-fields` as pass on their read
  halves; give it the reads-only "not judged: needs a write" marking too. Report
  wording only: such a run already exits 2 #ready-for-agent
  *Done (13 Nov 2026, slice 7a):* `Run` marks `health` and `required-fields` not
  judged whenever no write was sent (`!k.wrote`, decided after the run), so a
  refused `-mutate` run is marked like a reads-only one; its wording says the
  run sent none. `TestMutateRefusesBeforeAnyWriteWhenItCannotProveTheSubjectIsNew`
  holds it.
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
- [ ] 4.6 `-mutate` run against staging TrueNAS with a throwaway subject,
  against the **v2** add-on before 1.10; report attached; the kit's last
  line names the one disabled, labelled account it leaves (removal disables,
  never deletes) #ready-for-human
  *Prepared (13 Nov 2026), owner's rule "only temporary, back to the
  production setting afterwards":* run the kit inside
  `scripts/addon-quiet-run.sh`. It refuses off staging (the dump directory
  and a stand-in-Zitadel `.env`, both required). It recreates the add-on
  with `TRUENAS_STATS_COLLECT_EVERY=0` and stops the backend (its 30 s HEALTH
  probe) for the one command, then restores both on any exit. That includes a
  dropped ssh session or a closed pipe: PIPE is ignored, the restore ignores
  HUP, INT and TERM, and output stops after a failed write. The restore
  verifies the add-on's setting equals its value before and the backend
  answers `/healthz`, and records the outcome in syslog. Nothing is written to
  `.env`. Staging shares production's NAS, so its add-on runs `read_only`
  (`TRUENAS_LIFECYCLE_STATE`, which the script requires): nothing staging's
  backend decides can write there, unless an operator switches it to active
  on staging's target page (until the next recreate). The restore never
  starts the backend beside an add-on it could not put back to read_only. `--writes` makes it `active` for the one command, the kit's `-mutate`
  run, while the backend is stopped, and the restore puts `read_only` back and
  verifies it. The whole staging procedure, including switching staging's
  add-on back off afterwards, is in `scripts/README.md`. Held by
  `repoguard/addon_quiet_run_test.go` against a fake `docker`, which checks:
  - restore on success, on a failing command, and on a failing quiet
    (`env -u` proved, for both overrides);
  - read_only during a run without `--writes`, active only inside a
    `--writes` command;
  - a SIGHUP with nobody reading;
  - a restore that did not take, and a backend that does not answer, both
    reported;
  - a stopped backend left stopped;
  - refusal off staging and with a production `.env`.
  *Run (14 Nov 2026, TrueNAS 25.10.7, add-on at `85ebaba9`): 48 pass, 1
  fail, `limit-combine` (`home_quota` held nil). Slice 7k:* the kit's
  account got no home folder (deduced: staging's add-on has no `home_root`
  chosen, as its create succeeded, which TrueNAS refuses for any create carrying a
  home, below), so no home folder limit was read and `state` left
  `home_quota` out, "not known". The add-on was right; the kit judged an
  unreported limit as a fail (`limit-combine`) and as a pass on its way in
  (`limit-empty`). Now both skip a limit `state` leaves out, naming it, and
  with none read the check is a skip (`check.unjudged`, `kit.limits`,
  `kit.limitEmpty`; `TestTheKitSkipsALimitThisAddOnDoesNotReport`). Found
  beside it, a real defect the fake hid: TrueNAS's `home_create` takes
  `home` as the parent and makes `<home>/<username>` itself, refusing a
  parent that is not a folder (middlewared `plugins/account.py`, TS-25.04
  to TS-25.10.7). The add-on sent `<place>/<name>`, so every home it made on
  the real NAS was refused. `homeFields` now sends the place, and the fake
  (`mutatingRPC.homeFor`, `folders`) behaves as TrueNAS does
  (`TestTheConformanceKitPassesThisAddOn`, `TestARoleMakesAPrivateHomeFolder`).
  Still open: a `-mutate` re-run with a `home_root` chosen on staging is
  the only proof `limit-combine` passes on the real NAS #ready-for-human
  *Done (14 Nov 2026), slice 7n:* `syndra-addon-call`
  (`addons/conformance/cmd/syndra-addon-call`, `conformance.Call`), one
  signed request over the kit's own `deriveKeys`/`pinnedClient`/`mac`, no
  second signer, for the staging `--writes` window: set and clear `home_root`,
  release `conformance-` subjects. Allow-list `checkCall` before anything is
  sent: PUT/GET `/settings/<name>`, POST `account.release` for a
  `conformance-` subject, four GET reads; `account.purge` and every other
  write refused (`TestCallRefusalsSendNothing`, mutation-checked), signed
  round trip (`TestCallSettingRoundTrips`, `TestCallReleasesAConformanceSubject`).
  *Review fix (14 Nov 2026):* Go's decoder folds key case and Unicode, so a
  re-encoded operator body let `{"subject":"conformance-x","\u017fubject":"wwe-the-rock"}`
  release a member on TrueNAS's real handler. Now nothing is passed through:
  `onlyString` parses one key, byte-equal, once, a string, nothing after the
  object, and the payload is built (`TestCallRefusalsSendNothing`: folded,
  capitalised, duplicate, trailing); `actor` is the tool's own
  (`TestCallActorIsTheToolsOwn`); `-url` is an origin only
  (`TestCallRefusesABaseURLThatIsMoreThanAnOrigin`); `GET /settings/<name>`
  dropped (no such route); a refusal exits 2, not 1 (not sent, versus
  answered non-2xx); accepted by TrueNAS's real `decodeStrict`
  (`TestTheAddonCallToolIsAcceptedByThisAddOn`).
- [ ] 7n.1 Contract follow-up, not for now: the TrueNAS add-on's
  `decodeStrict` accepts case-folded keys (Go `encoding/json`), so any signed
  caller can send `Subject` or `\u017fubject`. Options: exact-case key
  validation, or `encoding/json/v2` case-sensitive decoding. Also
  `PUT /settings/<name>` is not lifecycle-gated #needs-triage
  *Decoder half done (14 Nov 2026, slice 7o; owner: "yes add exact-case
  decoding to contract 2"):* `Decode` in `addons/strictjson`, a module of
  its own with no dependency (the contract module stays files only,
  design §1), walks the body token by token against the target type and
  refuses a name not byte-equal to a field's JSON name (`UnknownMemberError`,
  400 `UNKNOWN_REQUEST_FIELD`) and a repeated name (`DuplicateMemberError`,
  400 with the route's malformed-body code: `BAD_REQUEST`, `INVALID_BODY` on
  a setting), RFC 8259 §4 and RFC 7493 §2.3. Field names resolve as
  `encoding/json`'s `typeFields` (tags, `-`, embedded structs, dominance,
  the diamond); maps, `any` and `UnmarshalJSON` types take any name, never
  twice. Errors quote in ASCII (`QuoteToASCII`) and echo at most 64 bytes;
  the JSON pointer is built only for an error. A 1 MB `/plan`: 38.8 ms,
  283 MB, 540k allocs before; 11.5 ms, 10.8 MB, 218k allocs after
  (`BenchmarkDecodePlan1MB`). Not `encoding/json/v2`: still a GOEXPERIMENT
  in Go 1.25. TrueNAS `decodeStrict` and the reference's `parse` call it
  (reference defect `folds-names` restores the old decoding); the kit's
  `requests.exact-names` probes `/plan`, `/apply` under a wrong fingerprint
  (`-mutate`) and `PUT /settings/{name}` with its current value
  (`-mutate-settings`; both add-ons decode before anything is stored).
  Tests: `strictjson_test.go` (mutation-checked: folded lookup, duplicate
  check, the diamond's cancelling), `TestEveryRouteMatchesNamesExactlyAndOnce`
  (TrueNAS, every body route; red with the old decoder),
  `TestExactNamesProbesTheApplyUnderAWrongFingerprint`,
  `TestExactNamesProbesTheSettingWithItsCurrentValue`, the defect row. The
  settings-lifecycle half stays open.

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
- [x] 5.3 A fresh agent with only `AUTHORING.md`, `addons/contract/` and the
  kit builds an add-on for a toy target and passes the kit; what it had to
  ask is written back into the guide #ready-for-agent
  *Outcome (11 Nov 2026):* a fresh agent passed the kit first run (Doorman,
  `kind: access`, reads-only and `-mutate`); 31 gaps written back.
  *Decided while writing:*
  - `state` and `observed` show a membership's managed set only (A5). The
    drift sweep compares the whole list (`merge.Classify`), so a value no
    mapping can name is a finding for ever; TrueNAS already hid built-in
    groups, the reference showed `lobby`. The reference now hides it
    (`shown`), the kit's `unlisted-preserved` (which needed it visible) is
    replaced by `unlisted-hidden` (read-only, every `state` it reads;
    reference defect `state-shows-unlisted`), and preservation itself is
    the add-on's own test (`TestAnUnlistedValueIsKeptAndNotShown`).
  - An add-on that creates accounts MUST serve `account.adopt` and
    `account.release` (B16): the backend reads a `BINDING_CONFLICT` apply as
    an ordinary 4xx (`target_refused`), and its only resolutions inside
    Syndra are `handleAdoptAccount` and
    `handleReleaseBinding`/`releaseOnTarget`, both through the operations
    route (renaming the account in the app is the other).
  - A log sync that fails before a write: the write is not sent, answered
    `APPLY_FAILED`/`OPERATION_FAILED` 500 (a person looks; the registry has
    no "add-on store" code for writes). `product_version` is `""` until read
    (cold start, schema allows it). A pre-write read failure MAY be
    `TARGET_UNREACHABLE`. Which 5xx, which of blocked/`PLAN_STALE` first,
    `drained` in `active`, the log head's format, the username rules and a
    limit's leading zeros are the add-on's, said once each.
  - Review of fc3cfd14: TrueNAS `unavailable` is the one source for the
    manifest's `available` and the operation gate (release answers on an
    unread or untested version); an undeclared operation id is 404 before
    any gate; `acted` is the one after-write rule (a 429 there becomes 503
    `APPLY_FAILED`); the kit's `limit-empty` judges only from a 5 GiB it saw
    set and skips an unreported limit; kit texts cite `AUTHORING.md#…`
    anchors, guarded by `TestEveryCheckCitesAGuideSection`.
  - Review of 64584cd0, one line each:
    - A JSON `null` for any `desired` field is `unholdable` (422 `APPLY_BLOCKED`, plan `blocked`), refused in `decodeDesiredFields` before any field is read.
    - `/plan` is `blocked` only for `unholdable`, a binding conflict, or an absence a truncated read cannot prove; an unreadable binding or home setting is 500 `STORE_UNREADABLE`, a home folder the NAS would not stat 5xx `TARGET_UNREADABLE` (as the apply).
    - The bound-name-reused next step: "Release this subject's binding first, then adopt that account only if it is theirs, or rename it in the app."
    - A repeat `account.adopt` answers the uid `boundAccount` resolves now, not the stored one (a contract-1 binding held a record key).
    - Writes for one subject run one at a time (`once`, rule 5: lifecycle gate, then a per-subject mutex, after the lease and the replay); `undoGroup` deletes only its own group while still empty.
    - `adoptable` is false for the add-on's own service account (`unboundConflict`, the one constructor, by `refuseSelfAccount`'s rule), with its own words.
  - Review of 246c4d26, one line each:
    - A plan from the stored copy (`current: false`) skips the home folder's `filesystem.stat` as it skips the group check (`planOne` `live`, `decideHome` `live`): 200 with the home planned, never a 5xx that aborts the target's drift; only a live plan's stat error is a 5xx.
    - A repeat `account.adopt` answers the found account's uid (`boundAccount`'s `found`, as apply), so an adopted out-of-range account with a contract-1 key answers its uid; bound with the account gone is 404, the ordinary miss.
    - `auth.me` with a blank name is a failed read (last known name kept, asked again); while the own account is unknown `account.adopt` and `account.purge` refuse through one guard, `requireSelfKnown`, with 422 `OPERATION_FAILED` (nothing done; the sentence reaches the operator; an operation is not re-queued, so 503 would have shown "unconfirmed") and a conflict is `adoptable: false`, "can't check yet … Try again shortly."; other operations unaffected.
    - `once` takes the subject lock, then `life.Begin()`: a write whose turn comes in a drain or `read_only` is 503 `LIFECYCLE_REFUSAL` with nothing done, so a drain waits only for writes running.
    - A write operation with a blank `subject` is 400 `BAD_REQUEST` in `handleOperation` before `once` (registry: "a required field is missing (call_id, subject)"; the reference agrees); adopt's own check went.
- [ ] 5.3a (found at 5.3) Where TrueNAS and the contract differ #needs-triage
  - Moved to 5.3b: the mutation log written after the write.
  - [x] An unknown key in `desired` was 400 `APPLY_FAILED`, the guide's
    rule 422 `APPLY_BLOCKED`. *Done (11 Nov 2026):* one error type,
    `unholdable` (`addons/truenas/apply.go`), for an unknown key, a value
    that does not fit its role (a malformed or negative limit, a non-boolean
    flag) and a group the NAS does not have or a built-in one
    (`groupIDs` in `subjects.go`). `/plan` answers it `blocked` (groups
    checked against the snapshot's own group table, so no extra read; a
    plan from the mirror skips it and is `current: false`), `/apply` 422
    `APPLY_BLOCKED` with the outcome (`blockedOr`); a group table that
    cannot be read stays the target's failure (5xx), and a store failure
    reading where homes go is 500, never a 4xx that drops the change. The
    backend reads 422 `APPLY_BLOCKED` as rejected with `effect: blocked`
    (`classifyStatus`, `Apply` in `internal/addons/entitlement.go`, tested
    by `contract_test.go` on the fixture); no backend change.
    `TestAnUnholdableDesiredStateIsBlockedByPlanAndApplyAlike` (six inputs,
    new and existing account), `TestAGroupTableThatCannotBeReadIsNotABlockedApply`.
  - [x] `v2/fixtures/error_binding_conflict.json` said "not bound to
    anyone" beside a `bound_to`. *Done (11 Nov 2026):* `bound_to` is
    possible by the schema, so it stays and the text says "bound to another
    subject" with the next step; `plan_response.json` the same. Every
    conflict's words come from `BindingConflict.words` (plan and apply),
    and the contract test builds the fixtures' outcome from it, so the text
    cannot drift. The words also corrected the case where the subject's
    bound name is now held by another account: `adoptable: false`, release
    first; `adoptAccount` now refuses that case instead of answering
    "already bound" (it checked the name, not the account), so `adoptable`
    is exactly "account.adopt would succeed".
  - The reference serves neither `account.adopt` nor `account.release` (the
    guide lists it as a simplification); once it does, the kit's `manifest`
    check can require both of an add-on that creates.
  - Moved to 5.3c: a membership that is not enumerable.
- [x] 5.3b The TrueNAS mutation log becomes an intent log: an intent
  recorded and fsynced before each NAS write, a 500 refusal (nothing sent)
  when the sync fails, and the result recorded after. Today `record` in
  `addons/truenas/apply.go` appends after the write, best-effort; design §8
  and the guide (`addons/AUTHORING.md#every-write-once-per-call_id`) say intent
  first. **MUST before contract 2 reaches prod.** #ready-for-agent
  *Done (11 Nov 2026):* one seam, `server.mutate` (`apply.go`): intent
  appended and fsynced, then the write, then a result record naming the
  intent's seq; `send` is mutate around `NAS.sendWrite`, `record` is gone.
  Every write goes through it: `user.create/update`, `group.create/delete`,
  `pool.dataset.set_quota`, the purge's `user.delete` on its own session, and
  the three store-only writes (`account.adopt`, `account.release`,
  `PUT /settings`). Tests in `addons/truenas/intent_test.go`, each
  mutation-checked.
  *Decided while building:*
  - The refusal is 500 `APPLY_FAILED` / `OPERATION_FAILED` /
    `SETTING_NOT_SAVED`, never 503 `TARGET_UNREACHABLE`: the NAS was
    reachable (that code's registry meaning is false here), and within one
    apply an earlier write (a personal group) may have landed, so the
    backend's "may have acted" (`classifyStatus`) is the true reading; it
    also surfaces as unconfirmed, which a failing disk needs. The guide
    already said so.
  - The guard is structural at runtime plus a syntax check: `NAS.call`
    serves only the listed `readMethods` and refuses any other method before
    sending; `NAS.sendWrite` is called only from `send`, the raw client's
    `Call` only from `invoke`, the ping and `callOnce`, and `callOnce` only
    inside a function literal handed to `mutate`
    (`TestEveryNASWriteGoesThroughTheSeam`, go/ast).
  - Record gains `write` and `intent`, both `omitempty` and in the digest
    only when set, so a record written before them digests as it did; the
    chain Syndra anchors verifies across the upgrade and the head only moves
    forward (`TestALogWrittenBeforeIntentsCarriesOnUnderThem`, on
    `testdata/mutations-before-intents.log` written by the code before this
    change). `operation` keeps the call's name (`apply`, `password.set`);
    `apply.create` and `apply.unverified` are no longer written (the write
    says `user.create`; an unverified read-back is not a log fact).
  - Four results, not three: `succeeded`, `failed` (the NAS answered no, or
    a store transaction did not commit), `not_sent` (`ErrNotSent`) and
    `unknown` (any other error: it may have landed), so no record claims
    nothing happened when that is not known.
  - Two records per write: the backend anchor, the kit's monotonic checks
    and the guide only ever required at least one; the guide now says the
    TrueNAS count grows by two, and that a write the NAS refuses still
    counts.
  - In doubt: settled `unknown` at start, before the server is built, said once, a second start appends nothing (`MutationLog.Recover`, `recoverInDoubt`).
  - Directory fsynced on every open; after a rotation by the next `Append`, which fails while `dirDirty` cannot be cleared.
  - Only an answer is a refusal: `classifyNASError` is the one classifier; a transport error (the client's "call timed out" included) is `unknown`, 503 "may or may not have happened".
  - `failedWrite` says "refused" only for an answer; purge, set and rotation no longer claim a refusal after a timeout.
  - Every binding change is its own `mutate` beside its NAS write (create bind, rebind, wanted SMB, purge unbind); a no-op store write is skipped; the AST guard holds store writes to `mutate` closures.
  - The fake NAS answers a missing method with an error member, as TrueNAS does, not a transport error.
  - Review of 3f774667: whether the NAS answered is decided before any wording, so no text turns a lost answer into a refusal (a login's rate limit is read by the breaker itself); an adopted group keeps a failure's own status only when this attempt's `group.create` was answered or never sent, otherwise the failure is "possibly acted"; a binding the log refuses after a create says the account was created. The fake NAS rate-limits with an error frame, as TrueNAS does.
- [x] 5.3d Adopting the account a crashed TrueNAS create left behind.
  Settling a write in doubt is done (5.3b: each is settled `unknown` at
  start, so `/health` needs no count). Left: whether the create path adopts
  an unbound account of the derived name under a logged `user.create`
  intent for that subject whose result is `unknown` (design §5 and the
  guide's "Look up before you create" allow exactly that adoption; today it
  is `BINDING_CONFLICT` for an operator).
  *Done (13 Nov 2026):* `leftByCrashedCreate` (apply.go), asked by `locate`
  and `planOne` alike, adopts only when every check holds: the account's name
  is exactly the derived one; `unboundConflict(...).Adoptable` (no binding
  names it, the add-on knows its own account and this is not it); its uid is
  in the add-on's range; no binding holds that uid under an older name
  (`Store.UIDBound`); and `MutationLog.CreateInDoubt(subject, account_id)`,
  reading every subject's records of that id, finds a `user.create` intent
  for this subject asking for that uid whose latest result is `unknown`,
  with no later binding record of the uid by anyone (`store.bind` or
  `store.unbind`, succeeded or unknown), no later `store.bind` of this
  subject, and no other subject's create of the uid that did not plainly
  fail (failed, not_sent). Anything else stays `BINDING_CONFLICT`.
  - Log format: `Record.AccountID` (`account_id`, omitempty, in the digest
    only when set, so older records digest as before and the anchored chain
    verifies across the upgrade), set on a create's writes from the uid on
    and on every binding write: apply's binds (create, rename, wanted SMB,
    adoption), and `account.adopt`, `account.release` and `account.purge`
    (`caller.on`). The intent is tied to the account by the uid it asked
    for, not the name (a name is display, never identity). A uid is NOT
    unique across the log on trust: an orphan group's gid is reused by the
    next create (`emptyGroupNamed`), and a restored `state.db` rolls
    `ClaimID`'s high-water mark back. Hence the cross-subject reading: any
    later binding of the uid cancels the doubt whoever wrote it (an operator
    adopting it to someone else, who then released it, review finding (a)),
    and two subjects' creates of one uid are a conflict for a person, never
    an adoption (design §5's two candidates, finding (b)). An intent written
    before this field is never adopted.
  - The trace that settles the doubt is the adoption's own bind, logged
    through `mutate` (`store.bind`, `BindIfFree`, carrying the uid) like
    every binding change, so a second create (or one after an operator's
    `account.release`) does not adopt again. No new record kind or outcome.
  - What can block the converge (the groups) is asked before the bind, so
    a blocked adoption binds nothing and "Nothing was changed." is true; a
    block that arises later (the group table changed in between) says
    "Adopted X …" and "Only the binding was recorded", and a converge
    error after the bind says the account was adopted.
  - The bind happens in `applyOne` after the fingerprint check, so a stale
    plan writes nothing; then `converge` and the outcome is `applied`
    ("Adopted X, which an earlier create made and could not record."), the
    plan saying `apply` ("Adopts X, …") with the account's fingerprint, so a
    re-planned system row dispatches instead of going `target_refused`.
    Under the subject lock (`once`); `BindIfFree` holds the name against
    every other subject.
  - Tests: `adopt_in_doubt_test.go` (crash after the intent then `Recover`
    then adopt; another subject's failed create of the uid still adopts; no
    intent, failed, not_sent, still open, another subject's create, another
    uid, two subjects' creates both unknown, bound since by another subject
    with an unknown result, released since, this subject bound since before
    ids, bound under an older name, the add-on's own account, below and
    above the range: conflict, plan alike; adopted elsewhere then released,
    end to end: conflict; a blocked adoption binds nothing; rotated
    segments; second create bound path; release then create: conflict; the
    bind logged; a create names its account) and `intent_test.go`
    (`namesItsAccount`: every bind, unbind and purge delete carries the id).
    Mutation-checked, each red: subject equality, the `unknown` check, the
    uid tie, `!conflict.Adoptable`, the floor, the ceiling, an unknown bind
    result cancelling, another subject's bind cancelling, the two-candidate
    rule, `UIDBound`, the group check before the bind, the id on release and
    on purge, the account still as created (password, SMB), and the scoped
    own-bind cancel.
  - *Review round 3 (13 Nov 2026):* a whole-volume restore (log and
    `state.db` together, as a vzdump or LXC restore does) brings back an old
    create in doubt with a log that agrees with it, after the account was
    adopted, released and adopted again. So one check does not read the log:
    the account must still be exactly as a create leaves it, no usable
    password and SMB off (`!PasswordSet && !SMBEnabled`; the create sends
    `password_disabled: true`, `smb: false`, and only a member's
    `password.set` changes either). The fake NAS now keeps
    `password_disabled` on a create, as TrueNAS does. And a later bind of
    this subject cancels only this subject's candidates, never another
    subject's claim on the uid. Staging's nightly restore (`_staging.yml`)
    resets only Postgres and the stand-in Zitadel's world file, and staging
    runs with the add-on off; deploys copy `state.db` out (never
    `mutations.log`) and restore nothing, so no repo path restores the
    add-on volume. A Proxmox-level restore of the app LXC would.
  - A name bound to another managed subject never reaches this rule: the
    derivation suffixes past it (unchanged).
- [x] 5.3c A membership field that is not enumerable: the add-on cannot
  tell its managed set, and the drift sweep compares the whole list. No
  add-on has one yet.
  *Decided (11 Nov 2026):* a membership MUST be enumerable (SCIM: a
  provisioner lists what it manages). Guide, design §3 and glossary drop the
  mapping-union managed set; the kit's `values` check requires it; a mapping
  on a non-enumerable membership is refused at `ResolvesValue`, not at
  registration (the manifest does not say it; `/values` does, once). Not a
  rollout gate. #decided
  *Confirmed by the owner (13 Nov 2026):* a membership must be enumerable.
  *Review fixes (11 Nov 2026):* the refusal is `ErrFieldNotListable`
  (inside the `ErrValueNotResolvable` family), answered 400 with
  `details.field = "unlistable"`, the system's name and the field's label, and
  its own next step in `details.next`; the Edit dialog shows both, never the
  default "change what it says". The Add dialog says the same plain sentence
  when `MappableValues.enumerable` is false (it used to show the built-in-groups
  sentence and never read the flag).
- [x] 5.3e Pre-existing: allowances (`validateAllowanceAgainstTarget`,
  `backend/internal/handlers/allowances.go`) check structure only and never
  call `ResolvesValue`, so an allowance can name a value no `/values` lists,
  for any field. #ready-for-agent
  *Done (13 Nov 2026, slice 7a):* `validateAllowanceAgainstTarget` ends in
  `resolveValue` (`handlers/mappings.go`), the one rule `validateMappingAgainstTarget`
  now calls too, and `writeAllowanceError` answers through `writeValueRefusal`,
  the one writer of the `ErrFieldNotListable` (`details.field = "unlistable"`,
  with `details.next`) and `ErrValueNotResolvable` (`details.value =
  "unresolvable"`) answers; tests post the same value to both surfaces and
  compare the two answers (`allowances_value_test.go`). The UI needs nothing:
  the hold dialog shows a refusal through `outcomeFromError`, which already
  words the `unlistable` step.
  Note for a future group-hold dialog: it must handle `details.value ===
  "unresolvable"` the way `AddMappingDialog.tsx` does (read the list again,
  pick another). Today the only hold the UI creates is lifecycle, so that
  refusal is unreachable from it.
  *Display names (7a review):* a name needs a letter as well as two characters,
  and names and checked values are compared in NFC (`golang.org/x/text`, already
  a dependency).
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

- [x] 6.0 The reference add-on's feature halves, each behind its feature
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
  - [x] The `results` half (slice 6a, 11 Nov 2026): `Config.Features` /
    `-features results` adds `results` and `holdings.list` (member, the
    printers the account may use, as `common/records.json`, current, call_id
    ignored, every lifecycle, 422 for an unbound subject), and `health.get`
    answers under `result`; defects `result-off-schema` and
    `result-in-pocket`. The other four halves stay open.
  - [x] The `freshness` half (slice 6b, 11 Nov 2026): `-features freshness`
    declares it with `limits.fresh_read_cooldown: PT1S` and `target_reads`;
    `/subjects` from a mirror (`cached`), `fresh` within the cooldown
    `cooldown` + `retry_after` (429 with nothing kept), one app read for
    concurrent asks (a joiner is `cooldown`), the last value `unreachable`
    (`Config.Down`, test-only), every write forgets the mirror, `/plan` live;
    defects `cached-as-current`, `reads-per-request`, `cooldown-ignored`.
  - [x] The `calls` half and the `physical` operation (slice 6c, 12 Nov
    2026): `-features results,calls` declares `calls` and `item.checkout`
    (`checkout.go`: item-0001..0004 on three printers, `loanLimit` 1), the
    preview that writes nothing, every check and the loan in one section
    under a per-subject lock, the intent before the loan, `CALL_UNSETTLED`
    while one is open (the same call_id included), `no_change` for an item
    already held, `RESOURCE_TAKEN` for one someone else holds, and
    `GET /calls` for every write (`callState`); `Config.LoseReply`
    (test-only) loses an answer before or after the loan. Defects
    `preview-recorded`, `reexecutes-in-doubt`, `checkout-unserialised`,
    `no-change-as-succeeded`, `calls-echo-params`.
    - Decided while building: an open intent is settled only on `/calls`
      (`settle`), never by the next checkout, which would clear it and run,
      so a retry would never be `CALL_UNSETTLED`; and never while `Down` or
      while the call still runs. A preview answers `CALL_UNSETTLED` too: it
      answers what the confirm would.
    - Decided while building: `problem()` resolves the registry's
      `by_status` class (`backoff` for 429 and every 5xx), as the guide says.
  - [x] The `stats` half (slice 6d-1, 12 Nov 2026): `-features stats`
    declares `machine.jobs` (counter, `count`, per person, dimension
    `printer`) and `machine.longest_job_seconds` (gauge, `seconds`, per
    person, no dimension, `against: job_seconds`), `stats_zone:
    Asia/Kolkata`, `limits.stats_buffer: P30D`; a deterministic job log
    (`jobsOn`) for every account from the day the log first has it, plus a
    `guest` login no binding names (totals and `unbound` only); `xl-1`'s log
    lost the week before it started (a final gap); a buffer of P30D under
    2,000 points, oldest day first, the dropped window a gap; the watermark
    the start of today, held while `Config.Down` (its own test); keyset
    pages of 4 KiB. Defects `stats-undeclared-dimension`,
    `stats-local-name-subject`, `stats-zero-for-gap`, `stats-watermark-ahead`,
    `stats-subjects-above-total` (a counter's total the sum less one, a
    gauge's the largest less one), `stats-since-ignored`, `stats-final-moves`,
    `stats-unknown-metric-ok`, `stats-page-overlap`, `stats-no-buffer-gap`,
    `stats-total-missing`.
    - Decided while building: the reference reads its in-memory app on
      every `/stats`, before answering, not on an hourly tick: that costs an
      in-memory app nothing, and it is what lets the kit's `-mutate` step see
      the kit's own account's usage. The guide says a real add-on reads on
      its own timer.
    - Decided while building: the gauge is `machine.longest_job_seconds`
      with no dimension, not the plan's `machine.queued_seconds` per
      printer: ruling O3 allows `against` only on an undimensioned
      per-person gauge, and a longest job is the maximum O2's gauge rule is
      written for.

**`results` (decision 20)**

- [x] 6.1 `registry/operations.json[id].result` (a schema ref); an add-on
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
  *Done, Go side (slice 6a, 11 Nov 2026):* contract (`holdings.list`, the
  storage profile `profiles/storage/status.json`, activity `resource`,
  registry `pocket`, five fixtures), backend (`addons.Read` in
  `backend/internal/addons/results.go` the one read path; `MemberReads`;
  `ReadReply.Storage/Activity/Health/View`; `Dispatch` refuses a read with
  `ErrReadNotDispatched`; `myTargetView.Reads`; the activity route answers
  `uncovered`, `truncated`, `taken_at`), TrueNAS declares `results`, the
  reference and the kit. The admin-read test is
  `TestAnAdminsReadOfHoldingsWritesNoOperationsRow` (handlers) over
  `TestAnOperatorsReadOfHoldingsWritesNoRow` (addons, a begun row or a claim
  fails it); the deploy-order pair is
  `TestReadOperationsSendTheContractShapeAndReadTheirAnswer` (both shapes)
  and `TestTheTrueNASAddOnWithResultsLandsInTheReader` against the fixtures
  `TestWhatTheAddOnSendsMatchesTheContract` holds the add-on's encoding to.
  The UI's generic renderers are 6.1u below.
  *Decided while building:*
  - Owner rulings O1–O3 (10 Nov): `storage.status` answers the storage
    profile (`quota_read`); `health.get` is read into today's
    `SystemReport` with no registry `result` (a `ponytail:` comment names
    major 3); events carry a generic optional `resource`, not an `x-` key.
  - Which reads have a pocket is the registry's (`pocket`), held equal to
    the policy's `Pocket` by `TestEveryPolicyOperationIsInTheRegistry`, so
    the backend and the kit read one list instead of each keeping one.
  - Syndra's `StorageStatus` loses `username`, `smb_enabled`, `shares` and
    `usage_readable`: no page read them, two could not be filled honestly
    from the profile, and use now travels once, as the status meters in
    `reads`.
  - `reads` lists only replies under `result`: a legacy pocket feeds its own
    typed reader (the storage card) and nothing generic. The backend reads
    both shapes, so it may deploy first or together with the add-on; never
    the add-on alone (it answers only under `result`, which a pre-6a backend
    does not read). `deploy.yml` ships both from one commit, so the workflow
    never produces that order. Pockets are not re-added for it.
  - A result missing a key its schema requires (top level, per item, inside
    any `standing` and each `places[]` item, and the profile's five) is not
    a reading (indeterminate, `readable: false`);
    the `current` ⇔ `stale` invariant is the kit's, not refused at runtime.
  - `no_account` on the activity route stays the add-on's 422 (the plan's
    B3, reading the binding first, was not needed: `ReadReply.Status`
    carries it). The read call_id is `read-<target>-<subject>` for every
    read, `health.get`'s subject empty (it was `-system-health`; ignored by
    contract either way).
  - Only member-scope reads with no required parameter are read on
    `/me/targets` and `/users/{id}/targets`; `activity.get` and `health.get`
    keep their admin routes, now through `Read` (B2).
  - Fix round (11 Nov 2026), one line each:
  - `quota_read` is carried, not dropped: `StorageStatus.QuotaRead` (the old `usage_readable`, read from either shape) and, in `reads`, `StorageResult` (status plus `quota_read`); meters are kept beside the flag, never dropped (`TestAnUnreadQuotaReachesThePageAsUnread`).
  - TrueNAS `shareUsage` reports one unread share's quota table as `quota_read: false` with the meters it did read; the profile schema says a partial read is false (`TestAnUnreadQuotaTableMakesTheUsageUnread`).
  - `activity.get` and `health.get` on the operator routes send the operator (`resolveActor`) as actor, the person as subject (`TestTargetOperatorReads_CarryTheOperatorAsActor`).
  - `also_says` caps each whole entry, key path included, at 200 bytes (cut on a rune) and the list at 50 plus "…and N more"; the manifest's `also_says` shares the cap (`TestWhatTheAddOnAlsoSaysIsCappedWhole`).
  - Required keys are one recursive `needSpec` per registry schema, held in `shapes` beside the keys and the decode type; a nested `null` is not an object.
  - The TrueNAS kit test requires `results` to pass under `-mutate`.
  - Every registry `pocket` is a property of `operation_response.json` (`TestEveryRegistryPocketIsAReplyProperty`).
  - The records fixture gains an overdue record (`item-0931`, `standing.next`) for the UI's next-step test; the guide's inline copy follows.
  - `addons.Call` refuses an `effect: read` operation (`ErrReadNotCalled`), as `Dispatch` does.
  - Deploy order: the backend reads both shapes, so it may deploy first or together with the add-on, never the add-on alone; `deploy.yml` ships both from one commit. Pockets stay off the TrueNAS add-on.
- [x] 6.1u The UI half of 6.1: the generic renderers for the three shapes
  from `myTargetView.reads`, and the activity list's `resource` column
  (a separate builder, from the backend shape 6.1 left)
  - Done: `ui/src/components/results/` (`ReadResult` picks by shape, raw fallback; `copy.ts` is the sentences, pinned to the guide's section 12), wired into the core member card, the person page's system card and the storage cards (which skip `storage.status`); `MemberTargetCard.inventory.test.tsx` renders `manifest_inventory.json` + `operation_response_records.json` with no inventory code; storage type lost `username`, `smb_enabled`, `shares`; both lint guards cover `components/results/`.

  - Decided while building (6.1u fix round, 11 Nov 2026), one line each:
  - Copy that differs by reader takes a `Reader` (member, or operator with the person's name), passed from the page down through `Reads` to every renderer; an operator's next step is the add-on's health on its system page, a member's is makerspace staff.
  - The "not recording" note names the person ("you" to a member), says one place or many, and tells an operator to switch recording on; storage keeps its own "shares" words and `PersonActivity` uses them only for kind storage (`isStorage`).
  - A failed read's title is per read ("One read from {name} did not come back") so it never contradicts the system's own "read just now" line.
  - `quota_read: false` on a storage card shows the shared `UsageUnread` notice instead of silently omitting usage lines; "some" is `usageShown(drawn)` (`kinds/storage/state.ts`) over the usage the card actually draws (the operator card's home folder from `view.home`; the member card's places they can change, none offline), never over every place.
  - Long text: `[overflow-wrap:anywhere]` on the read root, the activity row and `StandingLine`; the address columns are `min-w`, not a fixed width; the activity detail is `inline-block` so a short reason moves whole rather than breaking mid-token.
  - Records, meters and uncovered places are alphabetical through `sortByName`/`sortNames`.
    *Records superseded (13 Nov 2026, slice 7a):* see "Owner's answers of 13 Nov 2026" below; meters and uncovered places stay alphabetical.
  - Every results sentence, including the "Since", "Due", "of" fragments, lives in `copy.ts` and the copy guide's section 12; the guard is an ESLint `no-restricted-syntax` block on `src/components/results/*.tsx` (JSX text, string literals, template elements; `className`/`key`/`data-*`, the directive and imports exempt, `aria-label`/`title`/`alt` not), proved by mutation in `results-sentence-guard.test.ts`; `resultsCopyGuide.test.ts` keeps only copy.ts against the guide table.
  - `OWN_READS` is held to the registry by a test (`ownReads.test.ts`); `needSpec` is held to the result schemas by `TestNeedSpecMatchesTheSchemas`, and now requires a `stale`'s `reason`.
  - `Observation.stale` is in the UI type since 6b; its reasons are named once (`STALE` in `lib/queries/fresh.ts`), never written as `reason: "..."`, which `TestEveryFixedReasonIsNeverQuoted` reads.

**`freshness` (decision 21)**

- [x] 6.2 `fresh: true` (query on `GET /subjects`, `GET /values/{field}`,
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
  is cooled down). The UI shows `taken_at` on every card and a "Read again"
  (the product's words, not "Refresh") that sends `?fresh=true` #ready-for-agent
  - [x] Go side (slice 6b, 11 Nov 2026): contract fixtures and tests, the
    backend, the TrueNAS add-on (declares `freshness`), the reference and
    the kit.
  - [x] UI half (slice 6b, 11 Nov 2026): "Read again" is the card's own GET with `?fresh=true` through one hook (`lib/queries/fresh.ts` `useReadAgain`, the queryFn and the polls never send it; held by test), one strip (`ReadFreshness`: copy, cooldown countdown, unreachable) on every card that shows `taken_at` (member and person cards, activity, inventory, what each group opens), a 429 `FRESH_READ_COOLDOWN` through `outcomeFromError` (dormant list and removal, inventory) with a live countdown, copy guide section 13 with a guard. Homes show no `taken_at` and carry no strip.
  - Decided while building, one line each:
  - One sender of `fresh`: `sendSpec.Fresh` in the pipe adds the query on a GET and the body key on an operation, only for an add-on declaring the feature, and refuses it outside `GET /subjects|/values/|/reach|/homes` and read operations (`ErrFreshOutOfScope`), so never `/plan`, `/apply`, `/stats` is code, not a comment.
  - A `FRESH_READ_COOLDOWN` neither opens nor clears the backend's breaker (five "Read again" presses used to open it for the whole target).
  - A cooldown has one shape everywhere: `stale.reason: cooldown` when the add-on served its copy, `retry_after` on the view (reach, homes, read views, activity, inventory) when it refused with nothing to serve; `CallResponse.RetryAfter` is RFC 9110 delay-seconds only.
  - The invariant is held on read in one helper (`staleInvariant`): a reply whose `current` and `stale` disagree is not a reading (subjects unusable, reach/homes/results unreadable).
  - "Read again" is the card's own GET with `?fresh=true` (`wantsFresh`, literal `true` only), no new endpoint; a page load and the waiting poll never send it.
  - The drift sweep always asks fresh; a cooldown halts as `read_stale` with "skipped" in the log (no new reason, no migration, B6), after one wait and a second read (fix round (3)).
  - The dormant list and its removal always ask fresh; inside the cooldown both answer 429 with `Retry-After` and `retry_after` (owner Q1). `DormantAccounts` takes no flag: both callers would pass true.
  - The inventory takes "Read again"; a page load shows the copy dated, and Adopt stays blocked until read again (owner Q2).
  - `SystemHealth` takes no `fresh`: TrueNAS `health.get` carries no `current`, so it is read live every time, like `/values` (not in its fresher).
  - TrueNAS: one `fresher` (`fresh.go`) keyed per route and per subject (+ public params), 5-minute copy, 30 s cooldown begun at the read's start, request collapsing with a channel (no `x/sync`), a generation so a read begun before a write is never kept; `forget()` in `mutate`; a copy covers a NAS that cannot be read, never an operation's 4xx refusal; `target_reads` counted in `NAS.call` (probes included, writes not), in memory (B2).
  - TrueNAS `/reach` keeps groups and ACLs as one copy (a cached `/reach` reads nothing), and the home-place check before a setting write reads the NAS directly, unkept.
  - With nothing to serve, `/subjects` answers 503 `TARGET_UNREACHABLE` when nothing was sent and `NO_CURRENT_READ` otherwise (B3).
  - A joiner of a read in flight is `cooldown`, never `current` (B1); the kit accepts either for the joiner and wants one of two current.
  - Deploy and rollback: the backend and the TrueNAS add-on ship and roll back together (NEXT.md).
  - Fix round (review, 11 Nov), one line each:
  - (1) A write forgets reads under way too: TrueNAS `forget()` clears `flights` and a finishing read deletes its entry only while it is still the current one; the reference's `forget` clears its flight and cooldown the same way, so a read after a write never joins (nor waits out) a read begun before it.
  - (2) TrueNAS remembers each key's last failure (cleared on success and by `forget`); a fresh read inside the cooldown of a failed read answers through `fallback` (the copy `unreachable`, `/subjects`' disk mirror, or the error), still one NAS attempt per 30 s; 429 is left only for nothing at all, a state a finished read never leaves. The reference does the same (`failed` in `reference/fresh.go`, held by `TestAFreshReadInsideTheCooldownOfAFailedReadSaysUnreachable`).
  - (3) The drift sweep waits out one cooldown answer (`retry_after`, capped at the target's declared `limits.fresh_read_cooldown`, else `constants.cooldown_default`, bounded by ctx; `addons.FreshCooldown`, seams `freshCooldown`/`waitOut`) and reads fresh once more; only a second cooldown answer halts as `read_stale`.
  - (4) `addons.StorageStatus` carries the add-on's `Observation` (results only; the legacy pocket has none), and "last known" storage is dated by its `taken_at`, the backend's clock only when the reply did not say.
  - (5) TrueNAS `storage.status` passes `fresh` through `memberPlaces` to `reach(fresh)`, and its reply is `Observed.least` of the account read and the folder read (current only when both are, else the not-current one, the older of two), so places from a copy never ride a `current: true`.
  - (6) A refusal (an operation's 4xx other than 429) drops the key's copy, so a plain read inside the five minutes reads the NAS rather than serve the account as it was.
  - Decided while building 6b (Go review round 2): a reply made of several reads (storage.status and its folder read) is no fresher than its least fresh part only when read now; answered from the copy it keeps the copy's own reason (`cached`/`cooldown`/`unreachable`), dated by its oldest part (`Observed.with`, addons/truenas/fresh.go). A copy never replays a part's `cooldown`: plain polls at +90 s/+210 s say `cached`, a fresh read in an outage says `unreachable` (TestStorageStatusIsNoFresherThanItsPlaces).
  - Decided while building 6b: the drift sweep waits a cooldown answer once, `min(retry_after, declared, sweepWaitCeiling)` with `sweepWaitCeiling = 60 s` (2 x constants.cooldown_default); `retry_after` 0 or absent waits the declared cooldown (`sweepWait`, drift/addon.go). "Check now" calls `drift.ReconcileAddonNow`, the same pass that never waits: two entry points over one `reconcileAddon(ctx, target, patient)`, so neither call site carries a bare bool and no option rides a context value.
  - Decided while building 6b: `limits.fresh_read_cooldown` is parsed strictly as an ISO 8601 duration of days and clock time (RFC 3339 App. A dur-day/dur-time, plus a fraction on seconds); `P1D` and `PT24H` both read as 24 h and are capped alike by the sweep ceiling; `PT30MS`, `PT+30S`, `PT30S5M`, `P`, `PT`, weeks/months/years and overflow read as the 30 s default (`isoCooldown`, backend/internal/addons/manifest.go).
  - Decided while building 6b: the reference add-on answers a read that joins a failed read as that failure (mirror `unreachable`, or 503), never `cooldown`/`cached` and never a second read of the app, agreeing with TrueNAS's `fresher.fallback`; `forget` now clears the failure too, so a join a write forgot decides again (TestAReadJoiningAFailedReadIsAnsweredAsItWas, TestAWriteForgetsTheFailureToo).
  - Decided while building 6b: the inventory's 429 is the one cooldown envelope (`writeCooledDown`): `"error":"FRESH_READ_COOLDOWN"`, `retry_after`, `Retry-After`; the dormant list and sweep already were. UI may now require the code.
  - Decided while building 6b: an add-on `taken_at` more than `clockSkew` (2 min) ahead of the backend's clock is read as now and logged once per target (`takenAt(target, raw)`, addons/read.go); /reach now reads its taken_at through `takenAt` too (was a bare `*time.Time`). [needs-triage] Results decoded generically into `Observation` (records, activity, status views) still take `taken_at` as sent.
  Decided while building (6b UI fix round 2):
  - A refused READ (dormant list, inventory with nothing to show) is said once, by the freshness line and its waiting button; no ActionOutcome block, no "nothing was changed". The refusal block stays for removals (writes). Copy guide section 13 updated.
  - A 429 is a cooldown only with the code FRESH_READ_COOLDOWN (body or envelope); a code-less 429 is an ordinary failure.
  - Read again slot: a thumb tall on touch (h-[var(--tap)]), zero tall only at desktop (desktop:h-0), so a wrapped sentence is never cut by the button.
  - Stale gained `since` (ms, when the answer arrived); cooldownOf(error, since), useCountdown(seconds, since), useReadAgain exposes errorAt. Countdown deadline = since + seconds; remount does not restart it, a second refusal does.
  - useReadAgain.press cancels in-flight queries on its key before the fresh fetch.
  - toneOf(reading, why) is the one place that draws the line and dot: cached = neutral (bg-faint, text-faint) at any age; cooldown and unreachable stay amber; the rest by age.
  - Decided while building 6b (Go review round 3): every `taken_at` is held to this clock in one place. `takenAt` parses (present, RFC 3339, not the zero time) and `ahead(target, ts)` reads one more than `clockSkew` (2 min) ahead as now, logged once per target. `decodeShape` is now `ReadReply.decodeShape`, so it knows `r.Target`; after the unmarshal it clamps through `observed` (`observation() *Observation`, defined on `*Observation` and promoted to `*Records`, `*Status`, `*StorageResult` and `*ActivityReport`; a nil embedded pointer answers nil) and refuses the zero time. Records, activity and status were kept exactly as sent before this. `Storage()` decodes an embedded `Observation` and drops its own clamp, and `decodeActivity` is gone. A storage reply with a zero or unparseable `taken_at` is now unreadable (indeterminate), like reach and homes, where before it was read with no Observation (TestEveryResultShapeIsClampedAlike).
  - Decided while building 6b (round 3): the reference add-on's joiner answers the failure whenever `failed` is set (`if f.failed`, addons/reference/fresh.go). `forget` clears `failed`, so the old `&& f.flight == nil` only misreported one race: another reader begins the next read before the joiner wakes, and the joiner took that read's answer as its own. The test is deterministic: `testing/synctest` parks the joiner, and both reads are played by hand under `a.mu` (TestAJoinerOfAFailedReadNeverJoinsTheNextRead).
  - [x] 6b.f1 (follow-up from 6b round 3) Date a kept copy by its age, not by the add-on's clock, as RFC 9111 §4.2.3 does: age = the add-on's `Date` header − `taken_at`, both read off the add-on's own clock, so a skew between the clocks cancels out; the copy is then dated this backend's receive time − age. Today a copy is re-clamped to now on every serve, so with the NAS clock S ahead it reads "just now" for about S − 2 min. This needs response headers (`Date`) on `CallResponse`, carried through `ReadReply` to the clamp. #needs-triage
    *Done (13 Nov 2026):* RFC 9111 §4.2.3, Calculating Age. `CallResponse.Clock` (`Clock{Date, Sent}`, set in `doAuthenticated`, `Date` by `parseDate`/`http.ParseTime`) rides `ReadReply.Clock`, `Settlement.Clock` (the `/calls` answer) and `ViewOf(..., Clock)` into `ahead(target, ts, Clock)`, the one place a taken_at is dated on this clock: the typed readers (`takenAt`), every result shape (`ReadReply.decodeShape`), a physical confirm and its recorded reply, `/stats` (`ReadStats`), and the health `snapshot_taken_at` and `last_read_at` (`dated`; `checked_at` is this backend's own `timeNow` and needs none). age_value = `Date` - `taken_at` (both the add-on's clock, so a skew cancels), corrected_age = age_value + response_delay (the conservative older bound), the copy dates request_time - age_value; the RFC's apparent_age (response_time - `Date`) is omitted on purpose, it compares two clocks. A `Date` more than 1 s before `taken_at` is another clock's: ignored, said once; so is none (taken_at as sent, as before); a clock more than `clockSkew` from this one is said once; the clamp to now stays last. Both add-ons are Go `net/http` servers, which add `Date`; nothing strips it. Guide: an add-on MUST send `Date`. Kit: check `date` on `/subjects` (no feature gate), a fail for no, unparseable, or earlier-than-taken_at `Date`. Tests: TestACopyIsDatedByItsAgeNotByTheAddonsClock, TestACopyDatesFromWhenTheRequestWasSent, TestALiveReadIsDatedWhenItWasAsked, TestAResultShapeIsDatedByItsAgeToo, TestARecordedReplyIsDatedByTheCallsAnswer, TestStatsAndHealthAreDatedByAge, TestAClockOffAndABogusDateAreSaidOnce, TestTheDateCheckFailsAReadWithoutAUsableDate; mutation-checked (the age arithmetic, `Sent`, the bogus-`Date` rule, the kit's 1 s rule). Not corrected: the stats and activity `since` values stay on the backend's clock.
  - Decided while building (6b UI round 3): `useReadAgain` cancels in-flight queries only when a copy is cached (`getQueryData !== undefined`); cancelling a first load reverts it to idle and the list drew "Nothing outside Syndra.". A press that fails for any reason but the cooldown is shown as `ReadAgainError` ("Not read" + failure sentence) under the line, on every page that owns a press (inventory, reach, person activity, my/person storage via the provider).
  - Decided while building (flipped in the 6b UI fix round 4): a cooldown carried in the data only ever comes from a press (page loads and polls never send `fresh`, and both add-ons label a cooldown only on fresh reads), so the reader caused it unless it is marked otherwise. `Stale.pressed` is `false` or absent, and `pageLoad()` (`lib/queries/fresh.ts`) is its one marker, set at the one load site that can meet a cooldown, the dormant list (it reads fresh on every load), when `errorUpdatedAt < pressedAt` or there was no press. `toneOf` draws a cooldown amber unless `pressed === false`. Tone follows the line: cooldown while counting, then cached (neutral) if `current === false`, else by reading.
  - Decided while building: every cooldown counts from its answer: `since` = query `dataUpdatedAt` (reach, person activity, and my/person storage through `ReadAgainProvider value={{ press, since }}`); round 4 stamps it on a cooldown that comes with a copy too (`observed(o, since)`, `stamped()`), at every `stale` site, so a remount does not restart the count; `useCountdown` reads a `since` of 0 as absent. The dormant list keeps its rows when a refetch is refused with data on hand (the refusal is the line's to say, not ListStates').
  - 6b.f2 (found in round 4, fixed there): the dormant list's own Read again, failing for a reason other than the cooldown, went through `ListStates` and hid the list; it now shows `ReadAgainError` ("Not read") and keeps the data, as the other sites do.
  - Fixed: stale comment and test name for the inventory 429 body in api-client.ts / apiClientCooldown.test.ts (it is the standard error envelope). Copy guide §13 gained the "Not read" row and the pressed/neutral/since rules.

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
  - [x] The Go half (slice 6c, 12 Nov 2026): the registry has `effect` and
    `secret_result` for every id and `item.checkout`; `UNKNOWN_RESOURCE`,
    `NOT_ENTITLED`, `LIMIT_REACHED` and `RESOURCE_TAKEN` are registered with
    their extension members (`error.json`: `param`; `limit`, `unit`,
    `count`). The backend offers a physical op only with `calls` (and
    `results` for its record: `physicalNeeds`), previews it unrecorded with no
    call_id (`addons.Preview`, `addonop.Preview`), refuses it without
    confirm, refuses a resend (`ErrPhysicalNotResent`) and the next while one
    is unsettled (`ErrCallUnsettled`, before any record), sends once
    (`TestAnIndeterminatePhysicalCallIsSentOnce`), and settles any lost
    answer through `/calls` (`CallAnswer`, `settleByCall`): the member's poll
    `GET /me/targets/{t}/operations/{id}` and the operator's list take the
    same path. A physical row is held off Waiting on you until
    `in_doubt_bound` (`PhysicalHold`), then is the existing unconfirmed item.
    Routes `POST /me/targets/{t}/operations/{name}` (`preview` or `confirm`)
    and the poll; `/me/targets` gains `actions[]` and `unsettled`. TrueNAS
    answers `/calls` from its call store, lease and mutation log and declares
    `calls`; the reference and the kit as 6.0 and 6.11 say.
    - Decided while building (B1): `GET /calls` keeps the shipped
      `{call_id, state, reply}`, not `{outcome, result}`: `outcome` is the
      reply's own field. (B2) A preview answers the outcome the confirm
      would. (B3) The backend refuses `CALL_UNSETTLED` itself too, so "never
      twice" does not rest on the add-on alone. (B4) No server-side "preview
      before confirm" state. (B5) No redaction code in either add-on until an
      id has `secret_result`; the backend drops such a reply unread. (B6) A
      physical op needs `results` when it names a result.
    - Decided while building (O1): someone else's item is `RESOURCE_TAKEN`
      (409, `param`, RFC 9110 §15.5.10). (O2) `item.return` comes with the
      inventory add-on. (O3) A non-physical op on a `calls` target keeps the
      two-minute grace; only a physical one waits `in_doubt_bound`.
    - Decided while building: `/calls` is asked about any lost write, not
      only a physical one (the operator's list asks first, then the settings
      read); what it may settle is narrowed by the fix round below.
    - Decided while building (6c fix round, 1): `recorded` settles any row;
      `unknown` settles only a physical row sent within `constants.call_memory`
      (`addons.CallMemory`, held to the registry), less a minute for the
      clocks. Any other `unknown` stays for a person: the TrueNAS add-on in
      production keeps no write-ahead intent, and past call_memory an add-on
      may have forgotten (`settleByCall`, AUTHORING "What settles a record").
    - Decided while building (6c fix round, 2): the backend's
      `CALL_UNSETTLED` is one check inside the record's insert: a
      `pg_advisory_xact_lock` on (target, subject, operation), then
      `INSERT … WHERE NOT EXISTS` an open row (unclaimed `dispatching`
      included, any row not only the newest), `db.OpenCallError` mapped to
      `UnsettledError`; the separate pre-read is gone. A failed insert of a
      physical record stays `CALL_UNSETTLED_UNKNOWN` (fail closed).
    - Decided while building (6c fix round, 3): `account.adopt` and
      `account.release` are never asked `/calls`; a person repeats the action,
      which writes or forgets the backend's binding as well as the add-on's.
    - Decided while building (6c fix round, 4): the member poll is limited per
      person (`withPollLimit`, `ratelimit.DefaultPoll` 60/min, one a second
      sustained): the UI's 5-second poll spends a fifth of it.
    - Decided while building (6c fix round, 5): the kit's
      `operations.physical` counts the item as lent unless the confirm was a
      4xx refusal, so a lost or unreadable confirm's report says the item
      may be checked out.
  - [x] 6.3r (follow-up from the 6c fix round) A recorded `/calls` answer for
    an adoption or release could settle itself by running the same binding
    half as the adopt and release handlers (`handlers/inventory.go`), so the
    backend's copy is written with it; until then those two stay with a
    person.
    *Done (13 Nov 2026):* the halves are extracted once,
    `recordAdoptedBinding` and `forgetReleasedBinding` (handlers/inventory.go),
    and called by the live handlers and by `settleByCall` through
    `bindingHalf` (handlers/unconfirmed.go). A `recorded` adopt or release is
    settled by `db.ResolveBindingCall`: one transaction that locks the row
    (`FOR UPDATE`), skips the half when the subject's binding was decided
    since (see the review fixes below), runs the
    half (its writes join via `beginOrJoin`/`querier`), then
    `ResolveAddonOperation` (now joining an ambient transaction) as happened
    by `system:syndra`. One transaction rather than an order, so a crash
    leaves both or neither; a second settle finds the row answered and runs
    nothing.
    *Review fixes (13 Nov 2026, rounds 2 and 3):* two locks, always in this
    order, so settles cannot deadlock: the row `FOR UPDATE`, then the
    subject's binding row `FOR UPDATE`. No advisory lock: the insert takes
    one only for a physical call, so it would never meet an adopt or
    release. The copy counts as decided since, and the half is NOT run, when:
    a later adopt/release/purge of the subject may have reached the add-on
    (succeeded or no_change, still open, or answered happened or superseded;
    one unreached, refused or answered not happened does not count); for an
    adoption, the binding row was written or seen at or after the call
    (`last_seen_at`; the drain's `recordBinding` writes the copy with no
    operation row) AND names another account than the recorded reply (an
    older copy naming another account is stale, since the add-on accepted the
    adoption, and is rewritten); for a release, the copy names another
    account than the one released (the reply's `account_id`, else the row's
    `about`), or, with neither, the copy is at or after the call. Decided
    since, the row is still settled as happened (the add-on recorded it; the
    later write owns the copy), never left with a person who could only
    repeat what is true; `ErrBindingDecidedSince` is gone. `superseded` was
    not used: its CHECK needs `superseded_by`, and a drain write has no row
    to name. An adoption's half needs `account_id` from the recorded reply
    (`addonop.AccountIDOf`, the one reading of it, now also Dispatch's) and
    the row's `about` (the username); without either it is not settled.
    `in_doubt` and `unknown` are unchanged (neither is physical). The
    insert's `CALL_UNSETTLED` and advisory lock are untouched (byte for byte
    as before 7b).
    - Tests: `binding_settle_test.go` (the settle leaves exactly the live
      handler's binding / forgets exactly what the live release forgets;
      no account id, in_doubt, unknown, decided since: open) and
      `binding_call_live_test.go` (live: adopt and release settle with the
      binding, twice runs the half once, a failed half or an unresolvable row
      rolls the binding back, a later call of the same subject blocks and
      another subject's does not). Mutation-checked: a settle that writes its
      own binding, forgets without the merge base, settles by status alone,
      or drops the account-id check, each fails.
    - Copy traced: AUTHORING "What settles a record" said these were never
      asked (fixed); the unconfirmed list's header comment (fixed);
      `unconfirmed-and-self-settling` 2.5 annotated. The Say what happened
      dialog (`UnconfirmedChanges.tsx`) told a person to look under Accounts
      Syndra manages and promised "Syndra manages it", but a person's answer
      writes no binding (`handleSettleUnconfirmed`) and those lists read
      Syndra's copy: now it says the lists can't tell and the next step is to
      assign it (or stop tracking it) again, which the system answers either
      way and the lists then follow (test in `UnconfirmedChanges.test.tsx`).
    - `merge_findings.go`'s `ResolutionUnbound` now calls
      `forgetReleasedBinding`, so an unbind also forgets what Syndra landed
      (`ForgetPropagations`), which the inline copy skipped.
    - More tests: concurrent settles of one row run the half once (live, 5
      rounds); a later release blocks the copy when dispatching, succeeded or
      indeterminate, and does not when unreached or refused; a release keeps
      a drain-written copy of another account, forgets one of the account
      released, and, naming nothing, goes by time; an adoption skips only a
      newer copy of another account (an older one, or one of the same
      account, is rewritten); an adopt row with no `about` stays for a
      person. Mutation-checked: each of those rules.
    - The Say what happened copy (round 3): "it didn't happen" now says the
      next step too ("Assign it again if {account} should be {who}'s.",
      "Stop tracking it again if Syndra shouldn't manage it."), and How to
      tell no longer says the system "answers either way": an assign is
      refused when someone else now holds the account.
    - Review round 4: the TrueNAS `account.release` reply carries the
      `account_id` it let go of (none when nothing was bound, or for a
      legacy binding whose uid is outside the add-on's range and may be the
      NAS record key: `inIDRange`, the one check `boundAccount` and
      `leftByCrashedCreate` use too), so the settle compares ids, not the name:
      a release of charlotte (200001) recorded before the drain made
      charlotte again (200002) keeps the new copy (live test). The reply
      schema already allowed `account_id`; its description and the guide's
      account operations now say so (SHOULD; the reference serves no
      release, so the kit is unchanged). The copy guide's §14 carries the
      assign and let-go rows as a table, guarded by `actionsCopyGuide.test.ts`
      against `wording` in `UnconfirmedChanges.tsx`.
  - [x] The UI half (6.3u, a separate builder from the 6c UI handoff): the
    member action with `format: target_id` as a typed-or-scanned input,
    preview then confirm, the poll, and the desk's sentence on Waiting on you
    #ready-for-agent
    *Done in slice 6c's UI, verified (13 Nov 2026, slice 7a):* a typed-or-scanned
    input (`TargetIdInput.tsx`, the scan step in `MemberAction.tsx`), preview then
    confirm (`MemberAction.tsx`, `mode: "preview"`), the poll
    (`useMemberOperation` in `lib/queries/useMemberAction.ts`), and the desk's
    sentence (`ASK_DESK*` in `actions/copy.ts`, drawn for a physical row by
    `targets/UnconfirmedChanges.tsx`); `MemberAction.test.tsx` covers the flow.

- [x] 6.3a (the guide review's ruling (b)) Add `no_change` to
  `common/outcome.json`, listed under feature `calls`, mirroring
  `common/effect.json`, so an operation can say it changed nothing (a
  checkout of an item already held; `log_records` stays put); and have
  Syndra read the wire `outcome` instead of classifying by status alone, so
  a no-op is not recorded and audited as done. "Conflict" is not an outcome:
  it is a refusal, problem+json 409 #ready-for-agent
  - Done in slice 6c: `common/outcome.json` is a `oneOf` with `no_change`
    under `x-feature: calls` (`features.json`: `outcome.no_change`; the
    feature-marking test reads a `oneOf` member's `x-feature`). The backend
    reads the 2xx reply's `outcome` (`readOutcome`): `no_change` only from a
    `calls` add-on, missing or unknown as indeterminate; the record says
    `no_change` (migration 000092 widens the status CHECK, expand-only), the
    dispatch audit row stays, and no count or "last succeeded" read takes it
    for done.
  - Decided while building (6c, open call): "an open call" has one spelling, `openCall` in db/addon_operations.go (`resolution IS NULL AND status IN ('dispatching','indeterminate')`), and one read, `newestOpenCall`: the insert's CALL_UNSETTLED check and the member page's `unsettled` (`db.OpenCall`, replacing `UnconfirmedChangeAt(…, 0)`, now the password page's read only) run the same statement, so the refusal and the page name the same row; `SupersedeAddonOperations` uses the fragment too. An unclaimed open row (a crash between record and claim) shows on the member's page as `checking` like any other; the poll (GET /me/targets/{t}/operations/{id}) settles it exactly as the operator's list does, via `SettleUnclaimedAddonOperations` (unreached once past the 2-minute grace, target-wide, certain because nothing was sent) and then reads `not_done`; /calls is never asked about it. Guard `TestAnOpenCallIsSpelledOnce`: the open-status list appears only in `openCall` and in `CountAddonOperations`' status bucket.
  # 6c UI: decided while building
  - Copy guide: the handoff and plan say "§13", but §13 is already "How fresh a read is" (6b). The 6c sentences are **§14 "Acting on things in the world"**; guard is `ui/src/lib/__tests__/actionsCopyGuide.test.ts`. The ESLint sentence guard now covers `components/actions/*.tsx` too (its message names both copy.ts files).
  - One dialog, steps inside it (form, camera, preview, result, checking), never a second sheet: Modal's push-never-stack rule. The camera is a step of the action dialog, not its own Modal.
  - Files: `components/actions/{MemberAction.tsx,TargetIdInput.tsx,copy.ts,outcomes.ts}`, `lib/queries/useMemberAction.ts` (preview/confirm mutation + the poll), new write kind `memberAction` in `invalidation.ts` (reads `["me","targets"]`, not the rail).
  - The poll query key is `["me","operations",id]` with `staleTime: Infinity`: the settling poll alone carries `view`, a later poll answers `state` alone, so a settled record is never asked again (a remount would otherwise lose the record). `refetchInterval` 5000 until settled; stops on a 4xx except 429.
  - The row keeps its watched id (`kept`) after the server stops listing it as `unsettled`, so a settled result stays on the row until the next press; the action is off while the server lists one OR this page's own poll says `checking` (covers the gap before the list refetch).
  - `CALL_UNSETTLED` on a confirm with `details.operation`: the dialog shows that earlier record and polls it (nothing was sent); without `details` it is an ordinary refusal.
  - Refusal words come from the error code (`copy.refusal`), never the server `message`; an unknown code falls to `outcomeFromError` (shared words). 5xx coded refusals are `failed` (request id quoted), 4xx are `refused`. All wear the word "Not done".
  - `ask_desk` is drawn amber (`kind: "queued"` with its own `next`, so it never reads "Press Send"); `not_done` is `refused` with "You can try again."
  - `ActionOutcome` no longer appends "Nothing was changed." after a message that says "Nothing went out" (regex widened); needed for the unreachable and unanswered-preview sentences.
  - Operator row: `UnconfirmedChanges` words `item.checkout` through `checkoutWording` (copy.ts): "Ask at the desk whether {item} went out to {person}. If it did, it happened." Other physical operation ids fall to the generic row; add a case when a second physical operation ships.
  - `components/actions/copy.ts` labels `item.checkout` as "Check out an item"; any other action id is shown as its id in words ("Item return"). A real label belongs in the manifest (a future `label` on the operation), noted, not built.
  - Camera: `BarcodeDetector` + `getUserMedia({facingMode:"environment"})`, feature-detected after mount (no hydration mismatch); asked for only on the press; the stream stops on unmount however the step ends (read, Close, dialog closed, a stream that arrives after close). First label read fills the field; nothing is auto-submitted.
  - Dev-only learning: the plain-language scan flags the identifier `target` as a parameter name in `lib/queries` code ("say the system's name"); the hook's parameter is `system`.
  - 6c UI round 2: a lost-reply confirm (`mayHaveGoneThrough`) never says "Not done". It shows Checking at once (step `asking`), refetches `["me","targets"]`, and goes to checking on that target's `unsettled.id` for the same action. With no id it says "Not sure yet" (`UNSURE*` in copy.ts) with the step "Reload this page in a minute... Do not try again before then." The row has no outcome in that case because there is no id to watch. This is the one place dialog and row cannot show the same thing.
  - RATE_LIMITED worded in `refusal`/`REFUSAL_CODES` (now 16) and the section 14 table: "Syndra is getting too many requests from you right now" / "Wait a minute, then try again." Its wording is deliberately different from TOO_MANY_ATTEMPTS, which is the per-action limit.
  - One shared `inFlight` ref in ActionDialog guards both preview and confirm (`go`), set before `mutate` and cleared in `onSettled`. Confirm and "Show what will happen" have distinct `key`s so the footer button is a new node and focus is not carried onto it.
  - Scan button: `aria-label` and `SCAN_ARIA` dropped; the visible text is the name. `useCanScan` is now async: it needs a camera and `BarcodeDetector.getSupportedFormats()` containing one of code_128, code_39, qr_code, ean_13, data_matrix. A detector with no `getSupportedFormats` is not offered.
  - ask_desk: `UNSETTLED_DESK_REASON` "We still can't tell whether your last one went through. Ask at the desk." The row uses the polled state first, then `unsettled.state`.
  - Guide section 14 gained 5 rows: the desk reason, 3 for the unsure outcome, and RATE_LIMITED.

**`stats` (decisions 11–17)**

- [x] 6.4 Fixtures for `stats.json` and `manifest.metrics[]` (the schemas
  are 1a.2): one per-metric reply with points, a dimensioned gap, `unbound`
  and a keyset `next_cursor` #ready-for-agent
  - Done (slice 6d-1, 12 Nov 2026): `stats.json` in makerspace days
    (`+05:30`), points in keyset order (total first) and `next_cursor` the
    base64url key of the last one; new `stats_storage.json` (TrueNAS
    `storage.sessions`: two shares, a person and the total each, a gap for
    `scratch`, `unbound.accounts: 1`, `next_cursor: null`), registered in
    both fixture tables and inlined in the guide; `manifest.json` splits
    the storage gauge (`storage.home_bytes` against `home_quota`,
    `storage.used_bytes` per share) and carries `stats_zone`;
    `manifest_truenas.json` declares `stats`. Schema: `stats_zone` (IANA
    pattern, `x-feature: stats`, required with the feature by `if`/`then`),
    `against` only on a `per_subject` gauge with no dimensions (`if`/`then`
    on `Metric`); `stats.json` descriptions say midnights in `stats_zone`.
    Refusal rows: `per_subject: "yes"`, a dimension without a label, a point
    value as text, `unbound.accounts: -1`, no `next_cursor`, `stats` without
    `stats_zone`, `stats_zone` an offset or an abbreviation, `against` on a
    counter, on a dimensioned metric, on a metric not per person or without
    `per_subject`. `TestV2StatsFixturesAreMakerspaceDays` holds both replies
    to midnights in the manifest's zone, keyset order and the cursor.
    - Decided while building: `stats_zone`'s requiredness and O3's
      `against` rule live in the schema (`if`/`then`), one source the kit
      and both test suites already validate with; the backend's tolerant
      decode will check them again in 6d-2.
- [x] 6.5 TrueNAS add-on collector: on its own timer, `audit.query` for SMB
  sessions per bound account per share into `storage.sessions` (a calendar
  day in `stats_zone`, the whole day's total, a `gap` per `share` value
  auditing does not record for everyone, from each share's own audit
  settings) and the per-share and home usage it already reads for
  `storage.status` and `/homes` into `storage.used_bytes` (gauge per share,
  last sample of the day) and `storage.home_bytes` (gauge, `against:
  home_quota`); a subject-less
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
  - Done (slice 6d-1, 12 Nov 2026), with ruling O3's split: three
    metrics, `storage.sessions` (counter, per share), `storage.home_bytes`
    (gauge, against `home_quota`, no dimension) and `storage.used_bytes`
    (gauge, per share, no `against`), in `stats_zone` the container's `TZ`
    (`loadZone`, tzdata embedded). `stats.go` (`statsMetrics`, the one list
    for manifest and handler; `handleStats`), `stats_collect.go`
    (`collectStats`, `STATS_COLLECT_EVERY`, default `PT1H`, `0` off, joined
    before the NAS session is dropped), `stats_store.go` (bucket `stats`, one
    record per metric and day, one transaction per collection,
    `BoundUIDs`). One bounded `audit.query` per run (successful `CONNECT`
    since the watermark, ascending, 50 pages of 1000); a session is a
    distinct `session` per uid, share and day; only a share audited for
    everyone (B1: on, both lists empty) is counted, any other a gap per day.
    Usage owned by uid through the binding; unbound accounts in totals and
    `unbound` only. The collector calls reads only: no `mutate`, no
    mutation log, no read cache (`TestTheCollectorOnlyReads`). Tests in
    `stats_test.go` watch each of the six mutations above fail, and
    `TestTheConformanceKitPassesThisAddOn` requires every `stats.*` check to
    pass after one seeded collection.
    - Decided while building: the buffer is one record per metric and day
      (replaced whole, so a provisional 3 that is now 0 is gone) and the
      whole run is one bbolt transaction, rather than composite point keys;
      the size ceiling is bytes (64 MiB per metric), which needs no decode.
    - Decided while building: the first collection begins its own day: the
      NAS keeps its audit log for its own retention, often under P90D, and a
      day before it would read as zeros. Before it, the reply's buffer gap.
    - Decided while building: the failure gap starts the day after the last
      successful read; that day keeps its provisional points, and no gap
      lies over a point. Past the audit page bound, the days from the last
      row's day are cleared into one provisional gap ("Too much activity to
      count yet") and the watermark stops there; the next run goes on.
    - Display names (slice 7a, 13 Nov 2026): the privacy matcher (`peopleOn`)
      also holds each **bound** person's directory display name
      (`directory.Default.Users`, the cached list, read once per collection),
      through the same pattern and no second path; a name is trimmed, added
      only at two characters or more (a blank or one-letter name would name
      every text), and `QuoteMeta`d so a name with spaces matches as a phrase.
      An unreadable directory fails the collection (the watermark stays):
      a check that could not look must not read as one that found nothing.
      `TestADisplayNameIsCheckedLikeAnyOtherName`.
    - Decided while building: a CONNECT row's share is
      `service_data.service` (the observed key, Samba's word for a share),
      for the collector and `activity.get` alike (`auditRow.share`).
      `activity.get` read `event_data.share`, which no observed row has, so
      it never named the share (a live bug, also on main); fixed in the
      review round below. *Owner (13 Nov 2026):* the live fix ships with
      contract 2, with no hotfix on main.
    - Decided while building: `home_bytes` is a level for every bound member
      with a home, 0 included (the meter needs one); `used_bytes` per person
      only above 0; for both levels a uid under 1000 counts in totals,
      never in `unbound`. Sessions are keyed by account, so a system
      account or a name the NAS no longer resolves (neither is a subject)
      is in `unbound` as a digest of its name.
      Before the first collection `/stats` is 503 `NO_CURRENT_READ`; an
      unreadable store 500 `STORE_UNREADABLE` (the registry's status).
    - Review round (12 Nov 2026), each with a test watched to fail:
      - A day's start is its first instant: where a DST change skips 00:00
        (America/Santiago, 6 Sep 2026) Go resolves it to the previous day's
        23:00, which made the next-day step return the same day and the
        collector's loops spin. The day after is now `nextDayKey`
        (calendar only) and a skipped 00:00 takes the zone period's end
        (`time.Time.ZoneBounds`), in the add-on, the reference and the kit
        alike (`TestADayWhoseMidnightIsSkippedStartsWhenTheClocksResume`
        in each, with a 25-hour day).
      - A run catching up first reads the oldest SMB row the NAS keeps
        (`audit.query`, ascending, `limit: 1`, no event filter): every day
        through that row's day is a final gap "The NAS no longer keeps its
        log of these days", and counting goes on from the day after, so an
        outage longer than the NAS's retention is never final zeros.
      - A day with more rows than one run may read is a final gap "Too
        much activity to count" and the watermark moves past it, rather
        than holding there for ever.
      - A gauge's `unbound` for today is the run's that replaced the
        record (`putLevels`), not a union over the day's runs.
      - `TZ` is refused at startup unless it matches the manifest schema's
        `stats_zone` pattern (`statsZonePattern`, held equal to the schema
        by `TestTheZonePatternIsTheSchemas`): `Singapore`, `EST5EDT`.
    - Decided while building (6d-1 fix round): an `activity.get` event's `resource` is its service only when that is a known enabled share (`auditRow.share`, the one rule the collector also counts by), else empty, never a name: a `[homes]`-style share's service is the username (`TestAnActivityEventNamesOnlyAKnownShare`); backend and UI already took an absent resource.
Items 6.6, 6.7 and 6.8 are slice 6d-2 (backend collection, migration and
retention, access); 6d-1 changed no backend behaviour (a backend before 6d-2
ignores `metrics` and `stats_zone` and never calls `/stats`). With rulings
O1 (`acceptMetrics` drops the feature when `stats_zone != clock.Name`;
`bucket_start = clock.Day(start)`), O3 (drop a metric whose `against`
breaks the three conditions) and O4 (`stats_subject_retention_days`,
per-person only) applied there.

- [x] 6.6 Backend collection: hourly per target declaring `stats`, **one
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
  - Done (slice 6d-2, 12 Nov 2026): `acceptMetrics` and `ReadStats` in
    `backend/internal/addons/stats.go`, the collector in
    `backend/internal/services/stats/collect.go` (`Collect`, `CollectAll`),
    its transaction in `backend/internal/db/stats.go` (`StatsTx`), the
    `ADDON-STATS` runner in `cmd/api/main.go`. Live tests
    (`internal/db/stats_collect_live_test.go`, the collector against the
    real schema with a scripted add-on): the same `(metric, since)` twice
    changes nothing for points and gaps; a provisional value absent next
    time is gone (the watermark's own day included); a late day under a
    held-back watermark lands and retracts its gap; one dark metric does not
    re-send another; a failed second page keeps the watermark and the
    provisional rows; the watermark never goes back, is floored to a
    midnight and never passes today's; each drop reason counted; 500 series
    kept and the 501st dropped; a repeating cursor stops, and so does a
    cycle (A, B, A). Registration
    (`internal/addons/stats_test.go`): a changed unit or instrument refuses
    the manifest; each bad descriptor dropped alone and said once; another
    zone drops the feature; `/stats` never carries `fresh`.
    - Decided while building (ruling O1): `acceptMetrics` drops the whole
      feature, said once, when `stats_zone` is not `clock.Name`; a point is
      kept only when `start` is the first instant of its makerspace day and
      `end` the next day's (`clock.Midnight`, which now resolves a skipped
      00:00 to the instant the clocks resume, as the add-ons do; `clock.AddDate`
      for day arithmetic), and `bucket_start` is that day as `$n::date`.
    - Decided while building: `ADDON-STATS` is its own runner (hourly,
      started after `awaitFirstManifests`), not a step in the reconcile;
      `STATS_COLLECTION_ENABLED=false` stops collection only, never the
      retention runner (fix round). Each metric has its own five-minute
      context. `/stats` goes through the pipe and the breaker like every
      read, but a coded answer there neither opens nor clears it (fix round).
    - Decided while building: the stored watermark is the reply's floored to
      its makerspace midnight and capped at the start of today (a watermark
      of tomorrow would freeze today's provisional points); `GREATEST` in SQL
      keeps it from going back. The series the cap counts are read before
      the provisional window is deleted, so a held series is not forgotten
      because today is being replaced. A person in a dimension value or a
      gap reason is any text containing a binding's subject id, account
      name or account id, in any case (B7; fix round); directory display
      names wait. Drops are counted by reason (`not_a_day`, `bad_value`,
      `not_per_subject`, `unbound_subject`, `undeclared_dimension`,
      `person_in_dimension`, `past_retention`, `series`) in
      `addon_metrics.dropped`, the last collection's, and logged as counts,
      never values. A descriptor is stored only when it changed (a label), so
      the 15-minute refresh writes nothing. A failure to store descriptors
      leaves the manifest accepted with no metrics collected until the next
      refresh, rather than refusing the add-on.
- [x] 6.7 Migration: `addon_metric_points` (PK `target_id, metric, subject
  NOT NULL DEFAULT '', dims, bucket_start`; no `bucket_end`),
  `addon_metric_gaps` (`dims JSONB NOT NULL DEFAULT '{}'`; PK `target_id,
  metric, dims, gap_start`),
  `addon_metrics` (per metric: descriptor and watermark, ruling B3), and
  `targets.stats_subject_retention_days INT NULL` for the per-target shortening
  (same migration, not a fifth) (design §9.6); daily retention:
  per-subject points deleted after 400 days, target-wide after 5 years,
  gaps with the points they cover; no rollup; the per-target setting
  shortens the per-person retention only (ruling O4; one write place) and
  cannot lengthen it.
  Day-maths tests use `clock.Now`. Decision 13 #ready-for-agent
  - Done (slice 6d-2, 12 Nov 2026): migration **000093_addon_stats**,
    expand-only (`TestMigration93IsExpandOnly`; `TestLiveMigration93RunsDownAndUp`
    on the real engine; `TestLiveMigration92RunsDownAndUp` now works from any
    later version). `db.PruneStats` on the `STATS-RETENTION` runner (daily);
    `PUT /api/v1/targets/{target}/stats-retention {"days": 1..400|null}`
    (admin, audited `target.stats_retention_set`, 422 on the CHECK, 400 on a
    malformed body, 404 for an unknown target). Live tests: per-person
    points kept 400 days and the totals beyond; a 30-day setting shortens
    the people and not the totals; gaps go at five years; 0, 401 and 1000
    refused by the CHECK, 1, 400 and none accepted, each change audited.
    - Decided while building (ruling O4): one column, per-person only;
      `COALESCE(t.stats_subject_retention_days, 400)` and `INTERVAL '5
      years'` are SQL constants in `db/stats.go` that the prune and a late
      point at ingestion both read (a point or gap already past retention is
      not stored, counted `past_retention`); the ceiling is the CHECK, no Go
      `min`. A gap is kept until its end is five years old, with the totals
      it explains; the per-person setting does not touch gaps.
    - Decided while building: day-maths tests pin the four instants around
      India's and UTC's midnight (`TestStatsWindowsAreMakerspaceDays`); the
      live tests take their days from `clock.Now`.
- [x] 6.8 Access: `/me/targets` carries the actor's `per_subject` points
  for the last 30 days and 12 months; `GET /users/{id}/targets` (admin
  role) carries that person's with the page, beside their storage status;
  target and insights endpoints (admin role) serve the totals and the
  per-dimension series. No view audit, no separate endpoint, no
  suppression (decision 14). Tests: a member cannot read another's; a
  non-admin operator cannot read the target or insights series #ready-for-agent
  - Done (slice 6d-2, 12 Nov 2026), `backend/internal/handlers/stats.go`:
    `myTargetView.usage` (filled by the two views, `/me/targets` and the
    person page, from Syndra's store, so it shows while the target is down) carries
    the person's own per-person metrics; `GET /api/v1/targets/{target}/stats`
    (admin) every metric's totals with the collection line and the
    retention; `GET /api/v1/insights/stats` (admin) each per-person series
    over the last 30 days as members, p50, p90. Tests:
    `TestMyUsageIsTheActorsOwn` (`/me/targets?subject=u2&id=u2` as u1 reads
    u1 only; the person page reads the path's person),
    `TestStatsRoutesAreAdminOnly` (a member gets 403 on the three routes and
    the person page through the real router; every `stats` route in
    `router.go` is `withOperatorAuth`), `TestInsightsStatsNameNobody` and
    `TestLiveDistributionsNameNobody` (no subject in the spread),
    `TestUsageCarriesSumsLatestAndGaps` (the shape).
    - Decided while building: the API shape 6e reads. `usage` is
      `{"reported": false}` for a target with no stored metric, else
      `{"reported": true, "metrics": [...]}`, each metric its descriptor
      (`name, instrument, unit, label, per_subject, dimensions, against`)
      plus `watermark` (the makerspace midnight before which it is final,
      null before the first collection), `series` and `gaps` (`start`,
      `end` exclusive, `dimensions`, `reason`, days `YYYY-MM-DD`). A
      series is `{dimensions, days: [{day, value}]}` over the last 30 days,
      sparse; a counter adds `sum_30d`, `sum_12m` (0, never absent) and
      `months: [{month: "YYYY-MM", value}]` over 12 calendar months; a
      gauge adds `latest: {day, value}`. The totals page adds `collected:
      {at, unbound_accounts, dropped}` per metric and `retention:
      {subject_days, set}`; insights is `{window_days: 30, from, metrics:
      [{target, system, ...descriptor, series: [{dimensions, members, p50,
      p90}]}]}`. Metrics sort by label and series by dimension value
      (textsort); sums are over days, never across series (ruling B8).
    - Decided while building: no `?window=` parameter: every page carries
      both windows. A person's page shows only `per_subject` metrics and
      never the collection line; the person page is the member's own path
      (`describeTargetFor`, then `personalUsage`), so the two cannot differ.
    - Decided while building (6d-2 fix round, 12 Nov 2026), one line each:
      - Retention always runs: `statsRunners` builds `STATS-RETENTION`
        unconditionally and `ADDON-STATS` only with an add-on and
        `STATS_COLLECTION_ENABLED`; `main` starts retention at top level
        (`TestStatsRetentionRunsWithoutCollection`, with a source guard).
      - A coded `/stats` answer neither opens nor clears the breaker;
        transport failures, timeouts and uncoded 5xx still count, the
        breaker is still consulted first, and `NO_CURRENT_READ` on
        `/subjects` still counts
        (`TestACodedStatsAnswerIsNeverABreakerFailure`; where the rule
        lives: round 2 below).
      - A person's page keeps a gap only when its dimensions are `{}` or
        one of that person's series; at ingestion one rule, `people.names`,
        guards dimension values and gap reasons, counted
        `person_in_dimension` (`TestAPersonSeesOnlyGapsOnTheirOwnSeries`,
        `TestNoValueOrReasonNamesABoundPerson`; both rules sharpened in
        round 2 below).
      - Every cursor followed in a collection is remembered; any repeat
        stops it (`TestLiveACursorCycleStopsTheCollection`).
      - `PUT stats-retention` requires the `days` key: `{}` is 400
        `VALIDATION_FAILED` (`days: required`) and writes nothing; only
        `{"days": null}` clears.
      - Usage is filled by the two views only, never in `describeTargetFor`,
        so setting a password and a member action make no statistics query
        (`TestOnlyTheViewsReadUsage`).
    - Decided while building (6d-2 fix round 2, 12 Nov 2026), one line each:
      - A name is matched as a whole token, not contained: one RE2 pattern per collection, `(?i)(?:^|[^\pL\pN])(?:` + the `QuoteMeta`d non-empty names joined by `|` + `)(?:[^\pL\pN]|$)`, an empty class when there is none; not `\b`, ASCII-only and `_`-blind (`TestANameIsAWholeToken`).
      - A gap whose reason names a person is kept with the fixed reason "The add-on's reason named a person." and counted `person_in_dimension`; one whose dimensions name a person is dropped, as its series is (`TestNoValueOrReasonNamesABoundPerson`). *Superseded by 7i (14 Nov 2026): kept as sent with whom it names (`named_subjects`); see "Owner's ruling of 14 Nov 2026".*
      - A person's page keeps a gap whose dimensions are `{}` or a subset of one of their series (every key=value of the gap in it), so `{"share":"makers"}` reaches their makers/smb series (`TestAGapOnPartOfASeriesIsOnThatSeries`).
      - Every breaker counting rule lives in `breaker.record`: `send` only marks a coded `/stats` answer `CallResponse.StatsRefusal`, and `record` neither counts nor clears it (`TestACodedStatsAnswerDoesNotClearTheBreaker`).
- [x] 6.9 UI, from descriptors only: the member card "What I have used"
  (counters summed with a sparkline, gauges latest with their day,
  `against` as a meter over the **observed** limit, gaps footnoted, every
  card ending in a next step or "nothing for you to do"), the same card on
  the person page, the target overview's target-wide series by dimension
  with gaps hatched and the unbound-accounts line, "This
  add-on does not report statistics" when the feature is absent; the
  Insights page (Advanced view) grouped by unit with distributions and no
  names; dates are makerspace days, as everywhere (ruling O1, no "UTC day"); add-on text quoted and capped;
  the `dataviz` skill's rules for gap-vs-zero and both themes; copy guard
  for the new sentences and for the absence of ranking words #ready-for-agent
    - Built (6e, 12 Nov 2026): `components/usage/` (sentences in `copy.ts`, copy guide section 15, lint-guarded like 12 and 14), `lib/usage.ts`, `lib/queries/useStats.ts`; the card sits beside every kind's card (`MemberTargetCard`) and every person-page system card (`SystemsSection`), a fixed "What has been used here" region on the target overview, and a Usage tab on Insights. Meters read the limit the kind observed (`kinds/limits.ts`, storage from its home place), never a computed one; a totals page draws none. Not built: the System-page retention control (read-only line only), a per-day table view, provisional-day styling after the watermark.
  - [x] 6e (Go) **Decided while building:** a person's `usage` carries `kept_from` (`YYYY-MM-DD`, first day per-person points are kept), read by `db.StatsKeptFrom` from the same `statsSubjectKeptFrom` SQL the prune and ingestion use (today − (N − 1): N days counting today, since fix round 3), and the per-person points reads start at `max(window, kept_from)` so a sum never counts a day the daily prune has yet to delete; every metric carries `counted_through` (the watermark as its last final day, null with it), computed once in Go from the stored makerspace-midnight watermark; insights metrics carry the start of their spread (`window_30d_from` since fix round 3, was `kept_from`), since they read per-person points whose retention can be shorter than the 30-day window; totals unchanged (five years outlast 12 months). Live test proves the read line is the prune's and ingestion's line.
  Decided while building (6e UI fix round 2):
  - One meter: `components/ui/Meter.tsx`; >=90% warn + "nearly full", >=100% "full"; used by MetricBlock, MyStorage, PersonStorage, StatusMeters; `Meter.test.tsx` also fails if any of those writes `role="meter"` itself.
  - Retention: `kept_from` (usage, per-person card only) and `counted_through` (per metric, null-safe) are read as the backend names them; days before kept_from are hatched with Syndra's own reason ("Kept for only {n} days on this system"); headline "{value} in the last {n} days" when n < window; Insights shows "Over the last {n} days only" per metric when its kept_from is inside the window. (Superseded by fix round 3: N counts today, and windows are named from the backend's starts.)
  - A day after counted_through with no point is blank ("not counted yet"), never 0; a day with a point is drawn.
  - Retention control `StatsRetention.tsx` under TargetStats in region #stats; form is noValidate so the backend's 422 words are shown; shortening asks first, lengthening and "Keep the full 400 days" do not.
  - Spark: any day without a value breaks the line; single-day runs are dots (gauges are never interpolated).
  - Dropped sentence uses the shared `formatList` (format.ts); `joinNames` in MyStorage removed.
  - storageLimits: exactly one bytes limit field, gated on drawnUsage(home, offline).
  - Insights: fewer than 5 people shows the count and no spread or bar (MIN_SPREAD_MEMBERS; the backend has no floor for this read, insights.go's 5 is the single-route holder count). *Superseded by 7i (14 Nov 2026): no floor; the spread is always shown, and the people are named for operators.*
  - capText strips Cf and turns Cc into a space.
  - Open: handoff said kept days are "not hatched like a gap"; built as the same hatch with its own reason per the brief. Say if a distinct treatment is wanted.
  Decided while building (6e fix round 3):
  - Retention of N days counts today: `statsSubjectKeptFrom` is today − (N − 1), the one SQL line the prune, ingestion, `StatsKeptFrom` and every per-person read share; 400 keep 400 days, 30 the same days as "the last 30 days"; the CHECK stays 1..400; live tests assert today − (N − 1).
  - Windows are named from the backend's starts: a person's (and the totals') `usage` carries `window_30d_from` / `window_12m_from`, the starts `statsUsageFor` already read from; the card says "in the last 30 days" / "in the last 12 months", or "in the last {n} days" (n = start to today, today included) when the start is `kept_from`; the year line is not said when both start on one day; only drawn days before the 30-day start are hatched, and "Kept for only {n} days" is said only when one is; `YEAR_DAYS` and `keptDays` are gone.
  - Insights' spread is clamped in its own query to `GREATEST(from, kept_from)` (targets joined, today a parameter, the shared line), before the prune as after; each metric returns that start as `window_30d_from` (replacing `kept_from`), and the page says "Over the last {n} days only" from it and the response's `from`, no clock of its own.
  - "Deleted tonight" was false: the prune runs at start-up, then every 24 hours; the copy says "within a day", and the PruneStats, Prune, main and design text says daily.
  - Shortening is confirmed in a dialog (Modal / ModalFooter): title "Keep per-person numbers for only {n} days?", the act `dangerConfirm` naming the number, "Keep them" the cancel; the destructive-fill guard now reads a label said by a copy module (`{copy.name(…)}`) from that export's text.
  - The dialog asks only for a value the backend accepts (1 ≤ wanted < current); anything else is sent as typed, so the backend's 422 is the only answer; typing clears an old error (`save.reset()`).
  Decided while building (6e fix round 4):
  - The one-control-surface guard reads a confirm's label from the LAST `{copy.X` in its body (an attribute such as `reason={copy.NOTHING_TO_DELETE}` is not the label), and an export it cannot read on one line returns "" and fails closed; two self-tests hold both.
  - The "Kept for only {n} days" footnote shows whenever a drawn row's window starts at `kept_from` (30-day or 12-month line), not only when a cell is hatched, so the 12-month line vanishing at N = 30 is explained; n counts `kept_from` to `today`.
  - `usage.today` (makerspace day, YYYY-MM-DD) is sent with every reported `usage` and the UI counts `daysSince(start, usage.today)`, so a response cached across midnight keeps its n; Go test on the makerspace day (not UTC's), UI test on a response built yesterday. "nightly" comments reworded to "within a day".
- [ ] 6.10 (the owner's, not an agent's) Staging: 48 hours of collection against staging TrueNAS, then an
  operator checks that the member storage card's current number matches
  `storage.status`, that the trend exists, that a share with auditing off
  shows a gap and not zero, that a day with the NAS stopped shows a gap and
  then fills after it returns, and that "Refresh" advances `taken_at` once
  and reports the cooldown the second time #ready-for-human
  - Found on staging (21 Nov 2026, during this check): a team folder that is its own dataset inside a share read 67,036,288 bytes live while the share's collected total was 33,344 (the top dataset only), so members had no per-person series; and a member's card said "Nothing counted" for a share they can open whose auditing is off. Both fixed in 6.10a and 6.10b; the check itself stays the owner's.
  - [x] Staging prep (21 Nov 2026, the owner's "yes to all"):
    `syndra-addon-call` also sends `account.adopt` of a `conformance-`
    account with its `account_id`, and `account.release`, for any subject
    that is not all digits (every person's is), so a test home can be linked
    to `rey_student` and released again (`scripts/README.md` step 4c). The
    body is read strictly, every key byte-equal and once, nothing else, and
    the payload built from it; `account.purge` and every other write stay
    refused (`TestCallSendsAdoptAndReleaseForATestSubject`,
    `TestCallRefusalsSendNothing`, each guard mutation-checked)
    *Review fix (21 Nov 2026):* Zitadel accepts custom user ids, so "not all
    digits" was no proof a subject is not a person's. Now an allow-list:
    adopt names exactly `rey_student`, release `rey_student` or a
    `conformance-` subject, byte for byte, after refusing invalid UTF-8,
    surrounding whitespace and control characters
    (`TestCallSubjectAllowList`); `username` must match
    `^conformance-[a-z0-9-]+$` and `account_id` `^[0-9]+$`; a body over
    64 KiB is refused (`TestCallBodyCap`); each guard mutation-checked
- [x] 6.10a The collector counts usage by the live read's resolution (`fix/addon-contract-stats-nested-gaps`, 21 Nov 2026): `usageUnder` (`addons/truenas/reach.go`, since 6.10c) is every dataset a share reaches, the one mounted at its path plus each mounted inside it, summed once each (a dataset's userused leaves out its children); a home is `datasetAt` of the home, as `storage.status` reads it; one `pool.dataset.query` per run, still reads only. Test: `TestCollectedLevelsAreWhatTheLiveReadSees` (collected equals `memberPlaces`, no double count, a per-person series where the top dataset has no row of theirs).
- [x] 6.10b A gap on a share a person can open is on their page (same branch): `statsUsageFor` keeps a gap within one of their series or within a share their storage read reaches (`reachedShares`: the page's `storage.status`, else the last one remembered while the target is down), never one on a share they cannot open; the card says "Not everything could be counted in the last 30 days" in place of "Nothing counted" or "No value" when a gap falls in the window (copy guide 15). Tests: `TestAGapOnAShareTheyCanOpenIsOnTheirPage`, two in `UsageCard.test.tsx`.
- [x] 6.10c One number per thing (fix round 1 of PR 246, 21 Nov 2026; design "The storage numbers, reconciled"): `storage.status` counts a share with the same `usageUnder` the collector samples (a member with no row on the share's own dataset has a figure live, as collected; the limit stays the share's own dataset's); `/homes` takes used bytes from the home's own dataset (`datasetAt`) and the limit from the parent, as `convergeQuota` writes it, so the admin homes page and `storage.home_bytes` agree. A nested dataset that cannot be read (locked, or its table unreadable) leaves its share counted without it, with an `unread` gap naming the folder ("Not counted, could not be read: …"); a share that is a plain folder inside a dataset is never counted as that dataset, it is an `unobserved` gap ("The NAS does not count this share's space on its own"), live it is a partial read. A closed day that carries a gap keeps it rather than adding "could not be read that day". The card's empty sentence reads gaps through `gapLines`, the fold's one window check. Tests: `TestLiveAndCollectedStorageAreOneNumber`, `TestAnUnreadableNestedDatasetIsAGapNotTheShare`, `TestAPlainFolderShareNeverCountsItsParent`, `TestASharesDatasetsStopAtItsFolder`.
- [ ] 6.10d (needs the owner: a write path) A home that is a dataset of its own has its limit written on the parent (`convergeQuota`, `homeLimits`, `home.go`), where a ZFS user quota does not reach the bytes in the child. Found in 6.10c; predates it. The reads stay paired with the write until the owner decides where the limit goes (the home's own dataset, or a refquota).
- [x] 6.10e A locked dataset is never 0 (fix round 2 of PR 246, 21 Nov 2026): `datasetMounts` (`addons/truenas/reach.go`) also selects `locked`, and places a dataset that answers no mountpoint where its name puts it under its nearest mounted parent (ZFS's inherited mountpoint), so a locked folder under a share or a home is found by name. A locked dataset is never read: under a share it leaves an `unread` gap naming "<folder> (locked)" (since 6.10p, "a folder in <share> (locked)", over the total alone, 6.10o), and the place that is it says `not_counted: "locked"` (since 6.10j, a locked dataset affects only itself: the card and `/homes` are not partial for it, and `storage.home_bytes` samples everyone else). **UNVERIFIED:** neither `locked` nor a locked dataset's mountpoint is in `truenas_observed.json` yet; the field is `shape-optional` until `scripts/lib/record-truenas.py` (which now asks for it) records it on the next staging read. Tests: `TestALockedNestedDatasetIsNeverZero`, `TestALockedHomeAffectsOnlyThatHome` (6.10j).
- [x] 6.10f One number on the member's card (same round): `memberPlaces` counts a place through `memberCount.place`, the `usageUnder` the meters and the collector use (a home through `homeUsage`, the one rule `/homes` reads too), so a share's root is every dataset it reaches and a plain folder is never its parent's whole: `not_counted: "folder"` (contract: `profiles/storage/reach.json` `Place.not_counted`; the card says "Not counted on its own"). Test: `TestTheCardsPlacesAreItsMetersAndHomes`.
- [x] 6.10g A plain folder never makes a read partial (same round): it has no table that could have been read, so it has no meter and leaves `quota_read` true; it is on the card only where the member can open it. Partial means something could have been read and was not. Test: `TestAPlainFolderNeverMakesTheReadPartial`.
- [x] 6.10h A card load's NAS reads do not grow with the members (same round). Superseded by 6.10i: skipping other members' homes dropped bytes a member owns in them.
- [x] 6.10i Every dataset is counted on the card, other members' homes included (fix round 3 of PR 246, 21 Nov 2026): the skip of "another account's home" is gone, so Rey's files in jobber_0's home, or in a team account's home, are on his meter and places as collected. The NAS calls stay bounded by one shared copy per dataset (`quotaTable`, `addons/truenas/storage_status.go`): the tables go through the fresher (key `quota:<dataset>`), so every member's card shares them, a copy answers while it is fresh (`freshMaxAge`) labelled as a copy, a fresh ask reads again outside the cooldown, concurrent loads make one read, a write forgets it, and the card is dated by its oldest table (`StorageStatus.quotaSeen`, `Observed.with`), never by now. One load still asks each table once. The collector keeps its own reads. Tests: `TestBytesInAnotherMembersHomeAreCounted` (live equals collected), `TestCardLoadsShareOneReadPerDataset` (19 tables read once across 5 members' loads, dated by the copy, read again past `freshMaxAge`).
- [x] 6.10j A locked dataset affects only itself (owner, 21 Nov 2026, "Only that home affected"; same round): a locked home is "not counted" on its own card and `/homes` row (`not_counted: "locked"`, `profiles/storage/homes.json`; the backend shows it with no use, never 0 B, `target_homes.go`), `/homes` stays whole, no other card is partial, everyone else's home is sampled, and only the home total carries an `unread` gap (since 6.10o, `scope: "total"`: before it, a gap had no scope and hatched every person's home chart), "Not counted, could not be read: a home folder in main › home_dir (locked)". The gap names where the folder is, not the folder: a home is named for its account, and no account's name crosses the wire (design: "No local name crosses the wire for an unbound account"). A locked dataset inside a share leaves the share's meter and place counted, read whole; only the place that is it says locked; the collected share keeps its gap naming it. **Open (needs the owner):** a gap has no subject in the contract, so the home total's gap is over the whole metric and still hatches that day on each person's home chart (`lib/usage.ts` `plot`); a total-only gap needs a contract change. Tests: `TestALockedHomeAffectsOnlyThatHome` (locked by its own dataset or by its parent's), `TestALockedNestedDatasetIsNeverZero`, `TestALockedHomeHasNoUseAndTheOthersKeepTheirs` (backend).
- [x] 6.10k A locked share is a gap, not today's earlier sample (same round): `collectUsage` records `reasonLocked` ("Not counted: this share is encrypted and locked on TrueNAS") and drops the day's earlier points for it. Test: `TestALockedShareDropsTodaysSample`.
- [x] 6.10l One rule for where a home is read (same round): `homeDatasets` (own dataset for the bytes, the parent's for the limit, locked when either is) is used by `homeUsage` (card, `/homes`) and by the collector, which before looked at the home's own dataset only. Test: the parent-locked case of `TestALockedHomeAffectsOnlyThatHome`.
- [x] 6.10m The card's wording for a locked folder (same round): "Not counted: this folder is encrypted and locked. Ask makerspace staff to unlock it." (`PLACE_NOT_COUNTED`, `kinds/storage/captions.ts`; copy guide 15 rows for it and "Not counted on its own"). Never read as their account, a person to ask, no "the NAS" (copy guide 1, 6). **UNVERIFIED:** a locked dataset whose mountpoint was set by hand is still placed by its name (`datasetMounts`), so it is looked for where its name puts it; if the real NAS answers its mountpoint while locked, that answer wins.
- [x] 6.10n A share whose audit lists name a group everyone is in records everyone (owner change on the NAS, 21 Nov 2026: SMB auditing on for `main`, watch list `builtin_users`, ignore list empty; TrueNAS 25.10 refuses auditing with both lists empty, so `everyone()` gave `main` a "does not record everyone" gap every day). `shareAudit.recordsEveryone` (`addons/truenas/stats_collect.go`): enabled, and every account that can connect over SMB (`user.query` smb on and not locked, `smbMemberships`; groups by `memberOfNames`, the one helper `groupsOf` now uses) is recorded by `watches`, the rule a member's own coverage uses. Read only when a list narrows the share; when that read fails the share is a gap saying the NAS could not be read, never counted. Reads only (`user.query`, `group.query`, both allowlisted). Test: `TestAShareRecordsEveryoneWhenItWatchesEverySMBAccount` (every SMB account watched, one outside, an ignored group holding one, empty lists, the read failing; since 6.10q, a locked account and one with SMB off outside the watched group, which leave nobody out). **Its ceiling:** `user.query` lists local accounts only, never a directory service's (AD or LDAP): if the NAS ever joins one, its users' group must be on the watch list, or the share is not claimed as recording everyone. Whether one is configured is not read (`directoryservices.status` is not on `readMethods`; its 25.10 reply is UNVERIFIED).
- [x] 6.10o A gap over the totals alone, or over one person (fix round 5 of PR 246, 21 Nov 2026; owner, "Only that home affected"): contract `stats.json` gives a gap `scope: "total"` (the metric's totals at its dimensions, every person's own points standing) or `subject` (that person's own points only), never both (CHANGELOG). The add-on (`StatGap.forTotal`, `forSubject`, `addons/truenas/stats.go`) sends a locked home's gap over the home total alone, and one over that home's person, `reasonHomeLocked`; a locked dataset inside a share is a gap over the share's total, and, where it is a home, one over its person on that share. An unreadable nested dataset stays a gap over every series (whose bytes are missing is not known). The backend keeps both (`keepGaps`: a person not bound here, or on a metric not per person, is dropped; both, or an unknown scope, a bad value, the scope kept as total), stores them (migration 000103: `scope`, `subject` in the gaps' key; a person's gap pruned with their points), and shows a totals gap on the totals page alone and a person's on that person's page alone (`statsUsageFor`). The UI never hatches a person's card with a totals gap (`gapsShown`, `ui/src/lib/usage.ts`). Tests: `TestALockedHomeAffectsOnlyThatHome`, `TestALockedNestedDatasetIsNeverZero`, `TestAGapKeepsItsScopeAndItsPerson`, `TestATotalsGapAndAPersonsGapStayOnTheirOwnPages`, `TestMigration103OnlyWidensTheGapsKey`, `TestLiveScopedGapsAreKeptBesideEachOther` (live DB; not run in this round), and in `UsageCard.test.tsx` Booker's home chart drawing all 14 days and his change line beside Charlotte's locked home, only the total hatching.
- [x] 6.10p A gap names no member's folder (same round): a nested dataset locked or unreadable inside a share is said as "a folder in main" ("2 folders in main"; "(locked)" when locked), never its name: a folder is often named for its account, and the share's gap is read by everyone it reaches (`foldersIn`). The person whose folder it is sees "not counted, locked" on their own card. Test: no folder name on the wire in `TestALockedNestedDatasetIsNeverZero` and `TestALockedHomeAffectsOnlyThatHome`.
- [x] 6.10q Two guards (same round): a write forgets the shared quota copy, so the next read is the NAS's (`TestAWriteForgetsTheQuotaCopy`); a locked account, or one with SMB off, outside the watched group does not stop a share recording everyone (`TestAShareRecordsEveryoneWhenItWatchesEverySMBAccount`; the fake applies `user.query`'s `smb` and `locked` filters through `filterRows`).
- [x] 6.10r A target that cannot be read shows no count, list or date (found on staging, 21 Nov 2026, the add-on stopped on purpose; `fix/addon-contract-inventory-unread`): the inventory's halted 503 carried `bound: 0` beside seven recorded bindings and `read_at: 0001-01-01`, so the system's page said "Accounts 0", "Nobody yet.", "Not created by Syndra 0", "No longer needed 0" and "the last state seen, 1 Jan 1". `AddonReconcileResult` (`services/drift/addon.go`) now leaves out what it did not read: `bound` is a pointer and `unmanaged`, `accounts` and `read_at` are `omitzero`, so a halted body has none of them and a read that found nothing still says 0 and `[]`. The UI: `ReadFreshness` treats a zero or unparseable `readAt` as no copy, for every caller; the account cards show no count and no empty state when the list was not read, and the halted line says it once. Tests: `TestAHaltedInventoryCarriesNoCountListOrDateItDidNotRead`, "a read with no moment" (`ReadFreshness.test.tsx`), `PeopleOnTargetHalted.test.tsx`, and one each in `InventoryFreshness.test.tsx` and `DormantAccounts.test.tsx` (which said "at zero too" before). Fix round 1 (Opus review): the Accounts region says "could not be read" once, above its cards, with Read again on the line (`AccountsNotRead`, the strip's own `ReadAgainButton`); the systems list says "Not read" for a halted target, never "Not yet"; a reconcile pass sends `stale: []` on a whole read and none on a truncated one, so the card never says "0 people with an account missing" off a cut list; `Reach`, the last storage read, a mapping's values and the dormant report send no zero time (`omitzero`); one guard, `moment()` in `lib/format.ts`, makes every shared date formatter treat Go's zero time as absent. Tests: `TestAHaltedPassSendsNoCountListOrDate` (the 503 and the check's 200, through the handlers), `TestAHaltedReconcileCarriesNoCountItDidNotTake`, `TestStaleIsEmptyOnAWholeReadAndAbsentOnATruncatedOne`, `TestAZeroReadTimeIsNeverSent`, `TestReachNotReadSendsNoReadTime`, `TestADormantReportWithNoReadSendsNoReadTime`, and in the UI the region line, "Not read", the zero time and the count beside its positive control. Fix round 2 (Opus review): Read again on the region's line counts down inside the add-on's cooldown with the button waiting on it, as the strip's does; a read that lands and takes the line away hands focus to the region's heading; a truncated check says "missing accounts not checked: only the first part of the list was read" instead of dropping the clause; `moment()` blanks year 1 only, so the epoch dates as before; it is the one guard, so the per-caller branches in `AddMappingDialog` and `OpensNote` are gone, `read_at` is required again where the backend always sends it (a remembered storage read is dated on arrival when the add-on's `taken_at` is zero), and the systems list's flag is `unread`. Tests: the cooldown and the focus in `InventoryPressDuringLoad.test.tsx`, a whole empty list counting 0, `SystemWords`, `TestARememberedStorageReadIsAlwaysDated`, `TestAWholeEmptyReadSendsItsZeros`. Fix round 3 (Opus review): inside the cooldown the region's line keeps its reason (for an outdated add-on or a refused read the reason is the next step) and says "You can try again in N s." after it (`cooldownNext`, no second helper); Read again is off by `aria-disabled` while it reads or waits, so focus stays on it (`Button`'s `keepFocus`), and the wait is said once in its own `role="status"`; a failed press clears the focus intent, and the heading is found by its id and focusable only while focused (GOV.UK Frontend's `setFocus`); `TargetReach` is a union, so a read carries `read_at` and a read not taken none; the systems list's flag is `readFailed`; a cut list says "only the first part was read" in one place (`FIRST_PART_READ`). Round 2's dating of a zero `taken_at` on arrival is reverted (unreachable: `decodeShape` refuses a zero time) and its test deleted. Tests: three in `InventoryPressDuringLoad.test.tsx` (the reason during the countdown with focus and status, a failed press then a background read leaving focus alone, the heading's `tabindex` gone on blur), `targetReachShape.test.ts`, `SystemWords`.
- [x] 6.10s A share is counted only from when it was seen recording everyone (`fix/addon-contract-sessions-coverage-instant`, 21 Nov 2026): observed live, main's watch list was widened at 17:25Z and the 17:41Z collection counted the whole day, though nothing before 17:25Z was audited. `statsMeta.Recording` (`addons/truenas/stats_store.go`) keeps, per share, the first instant a collection saw `recordsEveryone`, dropped when a collection sees it stop (or cannot read the accounts); `collectSessions` counts a share's day only when that instant is at or before the day's midnight, so the day it began on, and every day before, stay an `unobserved` gap with the same reason (a point is a whole day, so the change day stays whole). A store with no instant starts at the run that reads it, never earlier. Fix round 1: coverage lasts only across an unbroken chain of runs that each saw the share record everyone; a run that sees it narrowed, cannot read the SMB accounts, or fails whole (`meta.Recording = nil` before `failedRun`) ends the chain, so a recount from the watermark never counts a broken day in full; a share that records everyone now but whose chain began after the day's midnight says "Not counted: this share wasn't yet seen recording everyone" (`reasonNotYet`, `unobserved`). Tests: `TestASharesDayIsCountedOnlyFromWhenItWasSeenRecordingEveryone`, `TestARestartKeepsWhenRecordingBegan`, `TestNarrowingAuditingEndsCoverage`, `TestNoStoredInstantIsNeverBackdated`, `TestABrokenChainOfRunsEndsCoverage` (now `TestUnreadSMBAccountsEndCoverage`). Fix round 2, one rule in place of the special cases: `statsMeta.LastSeen` keeps, per share, the last run that saw it record everyone, and the chain breaks whenever a sighting comes more than 2 × `STATS_COLLECT_EVERY` (`statsClock.every`) after it, whatever the cause; the failed-run reset is gone, and a run that read the share settings but failed on `auditConnects`/`auditOldest` still counts as a sighting (`coverSessions`). Narrowing or unread SMB accounts still end it at once. `statsMeta.Before` says why the days before a new start are gaps: "The NAS could not be read" (`reasonNAS`) after an outage or unread accounts, "Not counted: Syndra wasn't watching this share then" (`reasonUnwatched`, `unread`) after a gap with no run, `reasonNotYet` only after narrowing or a first sighting. Tests: `TestTheChainBreaksOnlyOnAGapInSightings`, `TestUnreadSMBAccountsEndCoverage`, `TestARestartKeepsWhenRecordingBegan`, `TestTheWatermarkIsHeldWhileTheNASIsDown`, `TestAnOutageLongerThanTheLogIsKeptIsAGapNotZeros`. Fix round 3, the final rule: a share's chain breaks only when it is seen narrowed or no longer enabled, or when a sighting comes more than 3 × `STATS_COLLECT_EVERY` after the last (the larger of the interval stored with that sighting, `statsMeta.Every`, and the current one), so one skipped run, which the scheduler lands a little past 2 ×, or a change to the setting, never breaks it; unread SMB accounts or bindings fail the run like any unread NAS (`failedRun`), so a long outage of either says `reasonNAS`; a store with `Recording` but no `LastSeen` restarts coverage with `reasonNotYet`. Tests: `TestTheChainBreaksOnlyOnAGapInSightings`, `TestUnreadSMBAccountsAreNoSighting`, `TestAStoreWithoutLastSeenRestartsCoverage`.
- [ ] 6.10t Extend coverage back from the middleware audit log (`audit.query` service MIDDLEWARE: the last successful `sharing.smb.update`/`create` touching the share's audit, the last `user.*`/`group.*` change, and the oldest kept row), with the chain as the floor; first confirm on the NAS (read-only) that `params` carries the watch and ignore lists.
- [x] 6.11 Kit checks per feature (design §7 `freshness` row, the
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
  - [x] The `results` part (slice 6a, 11 Nov 2026): check `results`
    (`-mutate`, in the reads step): every available read answers a `result`
    valid against its registry schema, or an object for its own; `skip`
    without the feature, `fail` for a read with no `pocket` declared without
    it. Watched to fail against `result-off-schema` and `result-in-pocket`,
    to skip against the plain reference and pass with `-features results`
    (`TestTheResultsCheckJudgesOnlyWhatIsDeclared`). The appendix lost A.2
    to chapter 7, so A.3–A.6 are A.2–A.5 and `operations.physical` cites
    `a2-…`.
  - [x] The `freshness` part (slice 6b, 11 Nov 2026): checks
    `freshness.cached`, `.cooldown`, `.coalesce`, `.unreachable` (always
    skips; held by the reference's own test, B4), counted by `target_reads`
    against a baseline of two `/health` reads, in units of one fresh read's
    cost (a TrueNAS read is several NAS calls). Watched to fail against
    `cached-as-current`, `cooldown-ignored` and `reads-per-request` and
    against rewritten replies; skip without the feature, pass with
    `-features freshness`, `-mutate` included; the TrueNAS kit test requires
    all three to pass. A mirroring add-on's `-mutate` "before" read asks
    fresh and waits a cooldown out once. The guide's A.1 moved into chapter
    4, so A.2–A.5 are A.1–A.4 and `operations.physical` cites `a1-…`.
  - [x] The `calls` part (slice 6c, 12 Nov 2026): checks `calls.unknown`
    (reads), `calls.recorded` and `operations.physical` (`-mutate`; the
    physical check needs `-item`, and the report names the item left checked
    out), `calls.unsettled` and `calls.serialised` (always skip: a black-box
    kit cannot lose a reply or know the limit; the reference's own tests hold
    them). Watched to fail against `preview-recorded` and
    `no-change-as-succeeded` (`operations.physical`) and `calls-echo-params`
    (`operations.secret`), to skip on the plain reference and pass with
    `-features results,calls -item item-0001`; the TrueNAS kit test requires
    `calls.unknown` and `calls.recorded` to pass. Every calls check cites the
    guide's chapter 7 section (A.1 moved there; A.2–A.4 are A.1–A.3).
    - Decided while building: a `/calls` reply is compared to the first
      reply by JSON value (it is embedded in another body); the replay of a
      confirm is compared byte for byte. A physical op declared without
      `calls` fails `operations.physical` in the reads phase, a MUST, so
      `-mutate` refuses to write.
  - [x] The `stats` part (slice 6d-1, 12 Nov 2026): checks
    `stats.descriptors`, `.points`, `.totals`, `.since`, `.idempotent`,
    `.gaps`, `.unbound`, `.buffer`, `.page-size`, `.no-people` (a MUST: no
    dimension value is the username or `account_id` of the account the kit
    created) and `.no-people.ids` (advisory: those of any other account
    `/subjects` lists, which may be a service account a resource is named
    after), each citing the guide's chapter 4 section "Statistics". In
    the reads step every metric is read from now minus `stats_buffer`, all
    pages, twice; from a later since; from ten days before the buffer; and
    an undeclared metric; with `-mutate`, a `stats` step after the writes
    reads again, when the kit's own account has usage. Midnights are judged
    in `stats_zone` (tzdata embedded). The totals rule is the instrument's
    (O2: `TestTheTotalsRuleIsTheInstruments`). Watched to fail against all
    twelve stats defects (`stats-dimension-names-person` fails
    `.no-people`, and only it, so the run fails) and against rewrites the reference cannot serve
    (a meter against a non-limit, an unknown zone, P401D, UTC days, a 23-hour
    day, a negative value, an off-midnight watermark, a gap with no reason,
    usage by nobody it names, a dimension naming the kit's own account id); skip without the
    feature and pass with `-features stats`, `-mutate` included.
    - Decided while building: the explicit-counter-zero warning is not
      built (no defect asks for it; an open question), and the `-mutate`
      half of `stats.unbound` is not either: a black-box kit cannot make an
      account outside a binding. The guide's "What the kit cannot check"
      names both halves the add-on's own tests carry. A 503
      `NO_CURRENT_READ` (nothing collected yet) skips the stats checks with
      the reason.
    - *Fixed (13 Nov 2026, slice 7g; #51 D1):* reads after a `-mutate`
      run against the same add-on failed `stats.unbound`, and the next
      `-mutate` was refused. Root cause, in the kit: the reference was
      right. Removal disables and keeps the binding, so the earlier run's
      subject still has usage, reported by its subject (`sendersOn`, from
      `a.bindings`). The reference names an account from the email's local
      part, and the kit's email is `<subject>@example.org`, so that
      account's username *is* the earlier subject. `statsSetup` put every
      listed username in `locals` and excused only this run's subject.
      `/subjects` carries no binding (its schema has `id`, `username`,
      `account_id`; never a Syndra subject), so the kit asks the add-on:
      a point subject that is also a listed id or name is excused only when
      one batched withdraw-only `/plan` (`learnBound`; desired
      `{account: false}`, email `""`) answers it with an `account_id`. A
      plan changes nothing (guide "POST /plan"), an outcome carries
      `account_id` only when an account exists (`apply_response.json`),
      and withdraw-only on an unbound subject is `no_change` whatever the
      email (guide; reference `decide`), so the empty email cannot
      mislead. Only collisions are asked, in plans of at most
      `subjects_per_plan`: usually no extra call. A plan that is refused
      or fails proves nothing, so the point stays a local name, and the
      failure says what `/plan` answered (`TestBindingsAreAskedInPlansOfTheLimit`,
      `TestARefusedPlanSaysTheBindingIsUnproven`). A
      username equal to a bound subject is excused: ambiguous by
      construction. The first cut excused the `conformance-` namespace;
      review showed an add-on reporting under usernames (`-email
      conformance-nN@…`) then passed a reads-only run, so it was replaced.
      `locals` no longer drops `k.cfg.Subject`, which is stricter for
      `.no-people.ids` when an account is named after the kit's subject.
      Tests: `TestPointsForAnEarlierKitSubjectAreNotAccountNames` (one
      reference, every feature: `-mutate`, reads, `-mutate
      -mutate-settings`; fails before the fix),
      `TestPointsUnderAnEarlierRunsUsernameAreLocalNames` (that bad add-on,
      three runs; the reads-only run fails, and passes under the
      namespace rule) and
      `TestAPointNamingTheKitsOwnAccountIsStillALocalName`;
      `stats-local-name-subject` still fails `stats.unbound`.
    - Decided while building (6d-1 fix round): the `.no-people` MUST is the kit's own account only (username and `account_id`, which it makes unique); any other listed account is advisory (`.no-people.ids`), since TrueNAS lists service accounts and a `scanner` share by a `scanner` user is no leak (`TestAResourceNamedLikeAServiceAccountIsAdvisory`; `stats-dimension-names-person` still fails the MUST).

- [x] 6.1r (slice 7h) `result` is core; the pockets and feature `results`
  go. Owner's ruling (14 Nov 2026): "We currently only have TrueNAS as the
  sole add-on on production, and the backend and the add-on are updated
  together, so the backwards compatibility is not needed." Contract 2 has not
  shipped, so this is a change to major 2 before release (CHANGELOG), not a
  major 3. #ready-for-agent
  *Done (14 Nov 2026):* every `effect: read` answers one `result`, read by
  `ReadReply.answer` alone (`results.go`); a reply under an old key
  (`storage`, `activity`, `health`) and no `result` is indeterminate, as a
  result missing a required key already was
  (`TestAReplyUnderAnOldPerReadKeyIsNotAReading`). Gone: `Pocket` from
  `OperationPolicy`, `EffectiveOperation` and the registry; `ReadReply.Pocket`
  and `ReadReply.Results`; the pocket branches of `Storage` and `Activity`
  (`storageWire` now decodes the profile, its `Observation` embedded);
  `physicalNeeds` (a physical op needs `calls` alone, in
  `resolveOperations`); `pocket` from `operation_response.json` with its
  three `$defs`; feature `results` from `features.json`, the fixtures, the
  TrueNAS add-on and the reference (`OptionalFeatures`; `holdings.list` and
  `health.get` under `result` always); the three pocket fixtures. The kit's
  `results` check is the core `operations.result`
  (`TestEveryReadIsHeldToItsResult`, a `health.get` under `health` fails),
  judged in a reads-only run too (`results(c, owned)`: the reads go without
  the owned-account plan, and a read about a subject answering 422
  `OPERATION_FAILED` for a subject with no account is skipped and named
  beside the pass ("judged …; skipped …"); `health.get` refused, or any other
  code, fails. The registry has no "no account" code and no "about the
  target" marker; the marker is now the registry's `subject` (required on
  every id, `false` for `health.get` alone; the kit's loader and the
  backend's registry test refuse an entry without it,
  `TestTheRegistryMustSayWhetherAnOperationIsAboutASubject`), and the
  dedicated code stays the owner's to add. The TrueNAS add-on answers a
  binding its store cannot read 500 `STORE_UNREADABLE` on every operation
  that resolves one (`boundAccount`, then `operateNow`/`readNow`; reads and writes, before
  any NAS call), never the 422 of "no account"
  (`TestAnUnreadableBindingIsTheAddOnsFaultNotNoAccount`); an effect: read operation is no write, one rule for
  the kit and its tests, `contract.writes`, so a reads-only run still reports
  "nothing written"),
  and ci.yml's every-feature reference is `-features freshness,calls,stats`.
  `TestAReadNeedsNoFeature` holds `holdings.list` from an add-on declaring
  operations alone. The person and member pages now list `storage.status`
  among `reads` from any add-on, as they already did from TrueNAS.
  Deploy and rollback: unchanged. The backend and the add-ons roll back
  together, to production's contract-1 pair (1.10), which never read a
  contract-2 read reply; removing the pockets changes nothing there, and the
  rollback guards (1.7b's `account_uid` dual write, the mutation log's format
  compatibility) are untouched.

## Owner's answers of 13 Nov 2026

- **Statistics days are makerspace days (IST).** Ruling O1 is confirmed and
  stands as the amendment to design decision 12; no "UTC day" anywhere.
- **List order.** Dropdowns and pickers of names stay alphabetical (case
  aside, locale-aware). Every other list defaults to its most logical order
  (most recent first, soonest due or overdue first, and so on) with sortable
  column headers (`SortHead`). Built for records (slice 7a):
  `results/RecordsList.tsx` opens on Due, soonest first (an earlier date is
  what is overdue; lateness itself stays the add-on's word), then records with
  no due date, then by name, with Record, Since and Due headings
  (`RECORD_HEADS`, copy guide section 12); a record with no date for the
  column in force sits last either way. Meters and uncovered places stay
  alphabetical: a place's name is what a person looks up.
- **The live activity-share fix** (`service_data.service`) ships with contract
  2; no hotfix on main.
- **5.3c confirmed:** a membership must be enumerable.

## Owner's ruling of 14 Nov 2026

Verbatim: "those statistics would need to be able to identify individuals.
visible only to the admins. under the required situations, admin should be
able to trace down the individual." Approved: Insights names who, what
names a person is kept with whom it names, members unchanged. Declined: an audit
record of each view.

- [x] 7j One unambiguous timestamp in every log line, and the zone set once. *Done (14 Nov 2026), reworked to the owner's ruling "do not hardcode anything":*
  - **Logs are UTC:** every Go service stamps `2006-01-02T15:04:05.000Z` (RFC 3339 UTC, literal `Z`) whatever `TZ` is. One copy per module (the backend and add-ons share no Go module for this): `backend/internal/logstamp.Install`, `addons/truenas` and `addons/reference/cmd/reference-addon` `stampLogs`, each with the same prefix test, a same-unix-instant test, and an `Install` test (a dropped `log.SetFlags(0)` would double-stamp). The add-on's `http.Server.ErrorLog`, go-redis's own logger (`db.libLogger`, `[CACHE]`) and the bare `fmt.Print*` lines (`[STARTUP]`, `[SHUTDOWN]`, `[DATA PLANE]`) go through it. Runtime panics carry no stamp (documented).
  - **No zone literal in code; `TZ` only:** `clock.Default` is gone. Unset or empty is UTC (POSIX, Go); an invalid `TZ` or `Local` refuses to start (`clock.mustLoad`, the reference `zoneFromTZ`, the TrueNAS `loadZone`), never a silent fallback. Compose passes `TZ=${TZ:-}`; `.env.example` keeps `Asia/Kolkata` as an example; `scripts/gen-prod-env.sh` requires `TZ`. Migration 000079 keeps its literal (a one-time re-bucket of this deployment's history), now said so in its header.
  - **UI:** `/healthz` carries `time_zone` (`clock.Name`); the root layout reads it (`ui/src/lib/zone.ts`, in parallel with the session, 1 s bound, validated with Intl once) and `<Providers timeZone>` hands it to `lib/time.ts`. If unreadable or invalid, times are shown in UTC and labelled " UTC" (`zoneTag`), never a guessed zone. `Time.tsx`, `DebugMode.tsx` and `claims.ts clockOf` no longer follow the browser; the zone-less `toLocale*String`/`Intl.DateTimeFormat` guard (`time.test.ts`) also refuses `timeZone: undefined`.
  - **UI zone is write-once (reviewed 14 Nov 2026):** the zone is one deployment value (a `TZ` change is a redeploy, which restarts the UI), so `setMakerspaceZone` keeps the first VALID backend answer, ignores the same value, logs a different valid one once and ignores it, and never acts on a missing or invalid one: a failed `/healthz` read can never overwrite a learned zone (it renders with what is held, UTC labelled only at a cold start). The layout offers the read zone and passes down what the server really holds (`effectiveZone`), and the client sets that same value, so server and client agree. A per-request context through ~300 formatter call sites was judged out of proportion for a value that cannot differ between requests.
  - **The wire carries the date the person means (re-review, 14 Nov 2026):** while the UI's zone is unknown it used to compute end instants in UTC, so "until 30 Nov" could land on 1 Dec. A calendar end is now sent as `YYYY-MM-DD` and resolved by the backend, in one place (`clock.EndDay`, decode to `clock.EndOfDay` in `clock.Loc`), replacing the instant field on `POST /requests` (`until`), `POST /allowances`, merge-finding resolve and `revoke-access` (`expires_at`, `review_date`); the column stays `timestamptz`. An instant (or any non-date) is refused. The only non-UI callers are this repo's handler tests, updated to dates; no script or add-on sends these. Direct grants and bulk keep `duration_days` (the plan binds the count); the UI holds the date and term presets of the direct-grant sheet until the zone is known. While the zone is unknown, days are labelled " UTC" like clocks. `/healthz` is read with a 3 s bound (over its 2 s database ping). `redis.SetLogger` moved into `logstamp.Install` so the claims binary is covered. Final review: the merge-finding "Use <system>'s value" no longer sends a review instant; the resolve handler defaults a review date six months on (`adoptionBound`, `clock.EndAfterMonths`); `clock.EndAfterDays` now ends the day when DST skips midnight; AskForAccess's keep and end-of-term choices wait for the zone like the direct-grant sheet. `gen-prod-env.sh` takes `DEPLOYMENT_TZ` (a login shell's own `TZ` is never captured).
  - Storage and wire audit: no `timestamp without time zone` in any migration; wire instants are RFC 3339; remaining `Format` calls are dates or display text in `clock.Loc`. `DEPLOY.md` Operations states the rules.
- [x] 7i Admins can trace usage to a person (design §9.6, §9.7, §9.8
  annotated; copy guide §15).
  *Done (14 Nov 2026, slice 7i; PR 180 with its review round):*
  - Ingestion keeps what names a person, with whom it names: migration
    000094 (`named_subjects text[] NOT NULL DEFAULT '{}'` on
    `addon_metric_points` and `addon_metric_gaps`, expand-only; its down
    deletes points that name anyone and keeps every gap that does, window
    and all, with 000093's withheld reason, so a 000093 backend never shows
    what names someone and no uncounted day turns into a counted one).
    `stats.keep` records a point's from its values and `keepGaps` a gap's
    from its values or reason (`people.names`, `people.namesIn`; the rule
    of a match unchanged). The name set is everyone in the directory (id,
    display name, username; a binding's account name and id too; never a
    machine user), not only who is bound on the target; whom is asked per person, so overlapping
    names are both found. `DropPersonInDimension`, the collector's
    `reasonWithheld` and the UI's `person_in_dimension` line are gone
    (`TestWhatNamesAPersonIsKeptWithWhomItNames`,
    `TestADisplayNameIsCheckedLikeAnyOtherName`,
    `TestEveryoneInTheDirectoryIsAName`, `BenchmarkNames`,
    `TestLiveWhatNamesAPersonIsKeptAndRecorded`,
    `TestLiveEachDropIsCountedByReason`).
  - Member view, in the backend: `statsUsageFor(…, memberView)`; `/me/targets`
    passes true, `/users/{id}/targets` and the target overview false. What
    names someone other than the member (`namesOther`) is hidden; their
    own name hides nothing of theirs. Such a series (any point read, days
    or months) is left out and counted in `left_out`; such a gap keeps its
    window and dimensions (inside a displayed series, so nothing new) with
    "The add-on's reason named a person."; a gap on a series left out is on
    none of theirs
    (`TestAMemberNeverSeesWhatNamesAPerson`,
    `TestAMembersOwnNameHidesNothingOfTheirs`,
    `TestOnlyTheMembersOwnPageIsMasked`). The card says "{n} more lines are
    not shown here, as the add-on named a person in them." and never
    "Nothing counted" when lines were left out (`UsageCard.test.tsx`).
  - Insights: `db.StatsDistributions(today, from30, from12m)` is one query:
    per person a counter's sum or gauge's latest over each window's
    `GREATEST(window, kept_from)`, the spread over the 30-day values
    (`count(v30)`, `percentile_cont` ignoring NULL), joined back so each
    row carries its people. `GET /insights/stats` adds `people` per series
    (user id, directory name or "", `last_30d`, `last_12m`),
    `window_12m_from` per metric and `from_12m`
    (`TestInsightsStatsNameThePeopleForAdmins`,
    `TestLiveDistributionsNameThePeople`,
    `TestLiveDistributionsKeepToRetention`). The UI table (`People` in
    `InsightsUsage.tsx`, `SortHead`) opens on Last 30 days, largest first,
    re-sorts by Person, Last 30 days, Last 12 months, links each name to
    `/users/{id}`, says an unreadable name with `Unresolved`, and keeps a
    gauge's missing value last either way. `MIN_SPREAD_MEMBERS` and
    `FEW_PEOPLE` are gone (`InsightsUsage.test.tsx`).
  - Sorting says what it does: `useSort(initial, remember, descending)`
    starts a listed heading descending (a time newest first, an amount
    largest first), so `aria-sort` and the arrow match the rows, and
    `orderBy` puts a null last whichever way. Moved off the negated-key
    trick: Insights' people, Requests' When, Operations' Time, Withdrawn's
    When, Drift's Found (`sortDrift`), People's Joined, Records' dates
    (`SortHead.test.tsx`, `drift-filters.test.ts`). A list with a heading
    that starts descending keeps its sort in `sort.v2`, so a sort stored
    before (newest first stored as ascending) is never read back the wrong
    way round; those lists open on their default once.
  - Migration round trip on the live engine
    (`TestLiveMigration94RunsUpDownUp`, `TestMigration94IsExpandOnly`).
  - Decided while building: people are per series, not per metric (a
    counter is never summed across series); a person with a value in the
    12 months but none in the 30 days is listed and not in the spread
    (a breakdown with only such people says "Nobody used it in the last 30
    days."); a member's series naming someone else is left out rather than
    relabelled (design §9.7), and such a gap keeps its window and
    dimensions and loses only its reason (review round 2 reversed blanking
    the dimensions, which hatched every line); whom a row names is recorded at collection, so a name
    added later is recorded from its next collection on, and a series
    counts as naming someone when any point read for it does.
  - Not built: paging of a long people table (every person is listed).

## Production parity (owner, 14 Nov 2026)

Verbatim: "ensure there is no function in the current prod truenas addon that
our new one cant or fails to do." Two audits (parity A, the add-on; parity B,
end to end) found no missing function; three regressions with one root cause
(R1-R3), two frictions, one self-contradiction and one test gap, all fixed in
7l.

- [x] 7l Production-parity fixes (design §3, §4 and §6.1 annotated).
  *Done (14 Nov 2026, slice 7l):*
  - R1-R3, saved facts instead of the manifest in memory: migration 000095
    adds `targets.accepted_shape JSONB` (nullable, expand-only, clean down);
    `addons.acceptRoles` saves the manifest's `Shape` (kind, product,
    entitlement schema with each field's role and unit) beside the role map
    in the same write, and skips the write when nothing changed (compared
    decoded, as jsonb reorders keys); `addons.AcceptedShape` reads it, and
    `ErrNoManifest` before any acceptance. `addons.AcceptedManifest` is gone:
    `handlers.acceptedShape(ctx, target)` reads the saved shape for
    `memberMeets` (`/me/targets`, `/me/systems`), `describeTarget`,
    `describeTargetFor`, `systemName`'s product, and the home limit on the
    person page (from the view's schema) and `/targets/{t}/homes`. A
    restart with the add-on down keeps the member card and rail row, the
    person page (account, entitlement, roles, hold, roles' limit, own
    limit) and the limits; a system never accepted stays operators-only
    (`TestTheAcceptedShapeOutlivesTheManifestInMemory`,
    `TestARestartWithTheAddonDownKeepsWhatSyndraHolds`,
    `TestMigration95_AddsAcceptedShape`, `TestLiveMigration95RunsDownAndUp`,
    `TestTargetFieldRolesArePersisted`). The handlers' `TestMain` defaults
    the seam to "never accepted".
  - A press inside the cooldown honours `Retry-After` (RFC 9110 §10.2.3;
    review round, replacing a first cut that read a young cooldown copy as
    current): the backend keeps a cooldown a cooldown
    (`TestACooldownIsNeverReportedCurrent`); a skipped "Check now" pass
    carries `retry_after` (`AddonReconcileResult.RetryAfter`,
    `TestCheckNowNeverWaitsOutACooldown`); the UI's `useRetryOnce`
    (`lib/queries/fresh.ts`) waits it out with the countdown, at most
    `RETRY_CEILING_S` (60 s, the sweep's ceiling), and sends the same press
    once more, once only, cancellable with **Stop waiting**, on "Check now"
    (`TargetOverview` `ReconcileControl`) and the dormant removal (a fresh
    one-shot box for the key). Owner Q1 met literally: the removal runs on
    a live read (`ReconcileRetry.test.tsx`, `DormantAccounts.test.tsx`;
    copy guide §13 row "trying again by itself in {n} s"). Both controls are
    off while it waits, the line their reason; leaving the page cancels it.
    When nothing goes again (a second refusal, or Stop waiting), Check now
    says `cooldownLine` counting from the skipped pass with
    `checkAgainNext` ("Press Check now again when the count ends."), never
    the pass's raw reason, and Remove shows the refusal it held ("Not
    removed", its countdown, "Press the same button once more."); each
    control stays off until its count ends. The retrying line is amber in
    the card (`RetryWait`, `ReadFreshness.tsx`), its start announced once
    (`role="status"`), focus moved to **Stop waiting**, and every
    `Button` reason is linked with `aria-describedby`.
  - One standing everywhere (parity A W5): `Subject.needsNewPassword`
    (password set, SMB off, SMB wanted) is the one fact and `standingOf`
    answers it "waiting on the member, set a new password". The wish
    (`Subject.wantsSMB`) comes from the bindings on `/subjects` and in the
    legacy replay (`Store.WantedSMB`, so the operator's target page agrees),
    from the binding in `storage.status`, and from the request in plan and
    apply (`desiredState.standingOf`, so a no-change outcome's standing
    matches its `smbWaiting` detail); `storage.status` takes `usable` from
    its standing (`TestAccessGivenBackNeedsANewPassword` extended,
    `TestEveryStandingSeesTheWaitForANewPassword`,
    `TestALegacyReplaysStandingSeesTheWaitForANewPassword`).
  - Production's own `state.db`: `addons/truenas/testdata/state-contract-1.db`
    written by 981efba3's store code (`testdata/state-contract-1.gen.go`,
    `//go:build ignore`, with how to write it again): three bindings, the
    mirror, five contract-1 call entries (apply, adopt, password.set,
    password.rotate, the home folder setting), the setting and the id
    high-water mark. `TestAStoreWrittenByContractOneOpensWhole` opens it with
    this binary, reads every binding, entry and setting, claims the next id
    above production's, and replays the calls in contract 2's words with
    nothing written to the NAS.
  - Decided while building, one line each:
    - The shape is one JSONB column beside `field_roles`, not more keys in
      it: `field_roles` stays the map every lifecycle reader decodes, and a
      shape is opaque to the db package.
    - The product is saved too, so a system's name does not fall back to
      its id title-cased in the same window.
    - Write paths (mapping and allowance validation, `mappedTo`, merge
      findings) still read the held manifest: a value is checked against
      what the add-on says now.
    - The retry is the UI's, not the server's: the request never holds a
      person's press open for up to a minute, and the scheduled sweep keeps
      its own one wait.
    - The removal's retry keeps the key in the wait's closure for at most
      60 s, no longer than it already sits in the field.
    - A target whose contract-2 manifest was accepted before 000095 has no
      saved shape until its next acceptance (the 15-minute refresh), and is
      operators-only until then, as before.

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

- [x] 7m The display locale is configuration (owner's ruling, 14 Nov 2026: "make this a config").
  *Done (14 Nov 2026, slice 7m):* `LOCALE`, a BCP 47 tag (RFC 5646), read by
  `internal/locale` (`locale.Tag`, validated with `golang.org/x/text/language`;
  invalid refuses to start, unset is empty and nothing is sent), sent as `locale`
  beside `time_zone` on `/healthz` (`healthBody`). UI: `lib/locale.ts`
  (`validLocale`, write-once `setDisplayLocale`, `displayLocale`,
  `effectiveLocale`), read in the zone's one fetch (`fetchMakerspaceFormat`),
  passed by the layout to `Providers`; unset or unsupported falls back to the
  runtime's default, passed down so the client matches. `format.ts`,
  `greeting.ts` and `usage.ts` no longer name `en-GB`. The zone-less Intl guard in
  `time.test.ts` now also fails a locale literal or a missing `displayLocale()`
  in `toLocale*String` / `Intl.DateTimeFormat` / `Intl.NumberFormat`.
  Review round (14 Nov 2026): the layout passes the CONFIGURED tag (undefined
  when unset or unread) and `runtimeLocale()` as a separate fallback, so a failed
  first `/healthz` never pins the server's default (`setDisplayLocale` stores
  only configured values, write-once, warns once on a different one; the
  fallback is `setFallbackLocale`; no valid fallback means `undefined`, Intl's
  own default on both sides). The browser trusts the server-validated tag.
  Ordering is display: `sort.ts` collates in `displayLocale()` and
  `textsort` in `locale.Language()` (sv: å after z). Numbers go through
  `formatDecimal` (`Intl.NumberFormat`), not `toFixed`; ISO timestamps compare
  with `compareInstants`, not `localeCompare`. The backend refuses the root and
  private-use-only tags and any tag not already in canonical form (Go and ICU
  disagree on tl, mo, grandfathered tags; checked against Node). The guard covers
  every `Intl.<Name>(`, `toLocale*String` and `localeCompare` and any locale
  literal. Compose, `.env.example`, `gen-prod-env.sh` (`DEPLOYMENT_LOCALE`), DEPLOY.md and
  NEXT.md (owner's cutover step: `LOCALE=en-GB` on staging and production).

- [x] 7p The backend decodes with the same exact-name decoder (owner's ruling, 14 Nov 2026: "yes, use the same decoder in the backend too"). One rule for every JSON input to Syndra: member names exactly as the schema spells them (RFC 8259 §4), never twice in one object (RFC 7493 §2.3).
  *Done (14 Nov 2026, slice 7p):*
  - **The module:** `backend/go.mod` requires `syndra/addons/strictjson` (`replace ../addons/strictjson`). `strictjson.DecodeOptions{AllowUnknown: true}` (protojson's `DiscardUnknown`, as an option on the one decoder rather than a second function) skips a member the type does not know, a case or fold variant included: its bytes and joining comma are blanked before `encoding/json` reads, so it can never be folded in. Duplicates are still refused, inside skipped members too. Fixed on the way: `json.Decoder.Token` answers `io.EOF` at any token boundary, so `{"x":` came back as `io.EOF`, which the optional-body routes read as "no body" (`TestDeletingAMappingRefusesABodyItCannotUnderstand` caught it); past the first token it is now `io.ErrUnexpectedEOF` (`walker.token`). Review round: the walk recursed per level with no limit, and Go 1.25's `json.Decoder.Token` keeps none (1.27's does), so a 1 MiB body of `[` cost hundreds of MB of stack before `encoding/json` refused it; the walk now stops at `encoding/json`'s own 10000 (`maxDepth`, `TestNestingIsBoundedAsInEncodingJSON`, mutation-checked under Go 1.25).
  - **FE→BE:** `decodeJSONStrict` (`internal/handlers/contracts.go`) reads the body and calls `strictjson.Decode`; its 69 call sites are unchanged, and every one already maps any decode error to its one 400, so a variant or a duplicate gets exactly the unknown-field refusal (`VALIDATION_FAILED`, or `BAD_REQUEST` where the route says so). An empty body is still `io.EOF`.
  - **Add-on replies:** one helper, `addons.DecodeReply` (`internal/addons/transport.go`), at every decode of reply bytes: `Apply` (entitlement.go), `Plan`, `Subjects` (and its `standing` read), `Health`, `SetLifecycle`, `ResolveValue`, `ListValues`, `ReadSettings`, `ReadHomes`, `ReadReach`, `ReadReply.Health`, `CallAnswer`, `ReadStats`, `readOutcome`, `refusalOf`, `decodeManifest` (raw map and struct; its unknown-key list was already exact), `ReadReply.answer` (map, already exact, but a duplicate `result` was last-wins: now refused), `decodeShape`, `staleInvariant`; outside the package, `handlers` `PUT /settings` reply and `replyNoChange`, and `addonop.AccountIDOf`. Unknown members stay allowed (AUTHORING.md chapter 14, rule 1). A duplicate reads as a reply that did not decode, as each site already treats one: indeterminate for a 2xx write or plan, no reading for a read, no code for a refusal.
  - **Images:** both backend images build from the repository root (`context: .`, compose and the CI images matrix); the root `.dockerignore` keeps the context to `backend/` and `addons/strictjson`. `Dockerfile.claims` copies only strictjson's `go.mod` (module graph; the binary does not import it), and `TestTheImageCopiesExactlyWhatTheBinaryImports` now also requires the `go.mod` of every module `go.mod` replaces. Both build stages emulated locally (no Docker on the Mac); CI's images job builds them.
  - **The UI proxy** parsed every body with `JSON.parse` and re-sent `JSON.stringify`, which keeps the last of two names, so a duplicate never reached the backend. It now forwards the bytes sent (review round 2 below).
  - **Review round 2 (14 Nov 2026, the coordinator's seven):**
    1. The proxy forwards the bytes sent whenever it adds no member, so a body that is not a JSON object (malformed, an array, a scalar) reaches the backend for its 400 instead of arriving as "no body" on the optional-body routes (drift mark-external and revoke, the mapping deletes, discovery).
    2. Where it adds one (member `POST /requests` `requester_id`, the demo `granted_by` and `reviewer_id`), `withMember` splices it in after the opening brace of the bytes sent, never a re-serialised copy: a duplicate, the caller's own `requester_id` among them, reaches the backend as a duplicate and is refused (RFC 7493 §2.3), where `JSON.parse` used to settle it (and silently overrode a spoofed `requester_id`; now refused). Chosen over a JS duplicate checker (a second decoder) and a header (a backend change).
    3. `decodeJSONLenient` deleted (its one caller was a test reading a response).
    4. Zitadel inputs NOT converted: no recorded Actions v2 or event payload exists (`claimcheck/testdata` is Management API listings), so no tag is proven to be Zitadel's exact spelling; design §1 says what is and is not covered.
    5. `readOutcome` says an unreadable reply is one (`ErrUnreadableReply`: not JSON, or a name twice), not a missing outcome.
    6. Root `.dockerignore` re-excludes `.env*`, `*.pem`, `*.key`, `zitadel-machine-key.json`, `DEPLOY.local.md` inside the allowed directories.
    7. `backend/Dockerfile` pinned to `golang:1.25-alpine`; `TestEveryGoBuildUsesTheModulesGoMinor` (repoguard) holds every Go Dockerfile to its go.mod's minor, finds any Go Dockerfile not listed, and holds ci.yml to `go-version-file` only.
  - **Review round 3 (14 Nov 2026), blocking:** CI's images job was red at `2a2d7ff0`: `backend/Dockerfile` ran `go get <pkg>` (no version) and `go mod tidy`, so every image resolved its dependencies at build time, newer than CI tested; go-redis v9.23 needs Go 1.26 and the image's Go is 1.25 (`GOTOOLCHAIN=local`). It now copies `go.mod` and `go.sum`, runs `go mod download`, then copies the source and builds, as `Dockerfile.claims` and the TrueNAS image already did (both clean, as is the UI image). `TestNoImageResolvesItsOwnDependencies` (repoguard) refuses `go get`, `go mod tidy` and `go install pkg@version` in any Dockerfile (red on the old one: 6 lines). Reproduced locally with `GOTOOLCHAIN=go1.25.0`, an empty module cache and `-mod=readonly`: download, `cmd/api`, `cmd/syndra-world-export` and `cmd/claims` build, go-redis v9.18.0 as pinned. Upgrades are slice 7q.
  - Tests: `TestAllowUnknownSkipsVariantsAndStillRefusesDuplicates`, `TestMalformedAndTrailingInputIsRefused` (cut short is not `io.EOF`), `TestMutationBodiesMatchNamesExactlyAndOnce` (terms, sign-in lookup, bulk decision, mark-external: variant and duplicate give the unknown-field refusal and do nothing), `TestDecodeJSONStrictTellsAnEmptyBodyFromATruncatedOne`, `TestAReplysCaseVariantIsSkippedNeverRead` (`Effect`, `Kind`, `Code`), `TestAReplyNamingAMemberTwiceIsNotRead`, `TestAReplyCarryingMoreStillReads`, the claims image guard, the proxy's "forwards the body's bytes", "adds requester_id … beside the bytes sent", "adds the demo admin …" and "forwards a body that is not a JSON object as sent", `TestTheWireOutcomeIsReadNotTheStatus` (unreadable versus missing), `TestEveryGoBuildUsesTheModulesGoMinor`. Each mutation-checked (the old decoder, `AllowUnknown` off, no blanking, no comma, no `io.EOF` mapping, no depth limit, no go.mod copy, no raw forward): red.

- [x] 7q Every component up to date (owner's ruling, 14 Nov 2026: "if updating/upgrading versions for any component (go for example) can help or is available, please update"). Patch and minor taken; majors listed below for the owner; Go 1.x counts as minor (the Go 1 compatibility promise).
  *Done (14 Nov 2026, slice 7q):*
  - **One Go, exact:** every module's `go` directive is `go 1.27.4` (no `toolchain` line: a full version in the directive is the minimum release, go.dev/doc/toolchain), every Go image builds `FROM golang:1.27.4-alpine3.24`, CI reads it with `go-version-file` (setup-go reads `toolchain`, else `go`), `.tool-versions` names it. CI had been testing on Go 1.25.0, the images on 1.25.x; Go 1.25 is out of support since 1.27 (two releases are), so its fixes stopped.
  - **Go 1.27 behaviour read (go.dev/doc/go1.26, go.dev/doc/go1.27):** `encoding/json` is backed by the v2 implementation (same semantics, error texts may differ; no test compares one), and `json.Decoder.Token` now refuses nesting past 10000 levels. strictjson's own depth counter (7p, written because 1.25's Token kept no limit) is deleted: the walk stops on Token's error. `TestNestingIsBoundedAsInEncodingJSON` holds it and goes red with `GOEXPERIMENT=nojsonv2` (the old decoder, no limit). `go test` now runs vet's `stdversion` check: clean.
  - **Exact pins everywhere**, held by repoguard `TestEveryVersionIsPinnedExactly` (new) and `TestEveryGoBuildUsesTheModulesGo` (was `…GoMinor`; now the exact release across every `go.mod`, Dockerfile, CI and `.tool-versions`): image tags name a release (PostgreSQL's is major.minor), GitHub Actions are pinned to the release's full commit SHA with the release in a comment (GitHub, "Security hardening for GitHub Actions"), `ui/package.json` names exact versions (no `^`, no `latest`), and `.tool-versions`, CI's `bun-version` and `ui/Dockerfile` name one Bun. Mutation-checked: `alpine:latest`, `postgres:15-alpine`, `redis:7-alpine`, `actions/checkout@v7`, `"next": "^15.5.29"`, `oven/bun:1.3-alpine`, `bun-version: "1.4"`, a go.mod at `1.27.0`, `golang:1.27-alpine`, `.tool-versions` at 1.26.2: each red.
  - **Before → after:**

    | Component | Before | After | Note |
    |---|---|---|---|
    | Go (6 modules, CI) | `go 1.25.0` (CI ran 1.25.0) | `go 1.27.4` | |
    | `golang` build images (3) | `1.25-alpine` | `1.27.4-alpine3.24` | |
    | `alpine` runtime: backend | `latest` | `3.24.4` | |
    | `alpine` runtime: claims, TrueNAS, `truenas-addon-secret` | `3.22` | `3.24.4` | |
    | `postgres` (compose, CI service) | `15-alpine` (production ran 15.18 on alpine 3.24.1, staging 15.17) | `15.20-alpine3.24` | same major; Alpine required (below) |
    | `redis` | `7-alpine` | `7.4.12-alpine3.22` | newest 7.x |
    | `oven/bun` image, CI `bun-version`, `.tool-versions` | `1.3-alpine`, `"1.3"`, none (`golang 1.26.2`) | `1.4.4`; the image `1.4.4-alpine@sha256:4cf82f56…` | Docker Hub has no versioned-Alpine bun tag, so the digest pins its Alpine |
    | `actions/checkout` | `@v4` | `@3d3c42e5… # v7.0.1` | Node 24, ESM; deploy.yml's is on a GitHub-hosted runner |
    | `actions/setup-go` | `@v5` | `@b7ad1dad… # v7.0.0` | reads `toolchain` first; we have none |
    | `oven-sh/setup-bun` | `@v2` | `@0c5077e5… # v2.2.0` | |
    | `docker/setup-buildx-action` | `@v3` | `@f87e5991… # v4.4.1` | removed inputs: none used |
    | `docker/build-push-action` | `@v6` | `@c3c9e263… # v7.4.0` | removed envs: none used |
    | `jackc/pgx/v5` | v5.9.1 | v5.13.0 | security, below; libpq-exact DSN parsing (DEPLOY.md) |
    | `redis/go-redis/v9` | v9.18.0 | v9.23.0 | needs Go 1.26; default timeouts changed, ours are explicit (`claimserve.NewRedisClient`); a pipeline pool of 10 per client |
    | `golang-migrate/migrate/v4` | v4.19.1 | v4.20.1 | |
    | `golang.org/x/text` | v0.36.0 (backend), v0.14.0 (add-ons) | v0.44.0 | security, below |
    | `golang.org/x/sync`, `x/sys` | v0.20.0, v0.43–0.45 | v0.25.0, v0.50.0 | |
    | `truenas/api_client_golang` | `fc6edc0b6ebe` | `508624041bb3` | README-only commit |
    | `golang-jwt/jwt/v5`, `santhosh-tekuri/jsonschema/v6`, `gorilla/websocket`, `bbolt` | v5.3.1, v6.0.3, v1.5.3, v1.5.0 | unchanged | already newest |
    | `next`, `eslint-config-next` | 15.5.15 | 15.5.29 | |
    | `react`, `react-dom` | 19.2.5 | 19.2.9 | held to the React Next vendors (below) |
    | `@tanstack/react-query(-devtools)` | 5.100.5 | 5.107.1 | |
    | `tailwindcss`, `@tailwindcss/postcss` | 4.2.2 | 4.3.5 | |
    | `vitest`, `@vitejs/plugin-react` | 4.1.4, 6.0.1 | 4.1.12, 6.3.0 | |
    | `eslint`, `jsdom`, `postcss`, `autoprefixer` | 9.39.4, 29.1.0, 8.5.9, `latest` | 9.39.5, 29.1.1, 8.5.33, 10.6.2 | |
    | `@types/react(-dom)`, `@types/bun`, testing-library | 19.2.14 / 19.2.3, `latest`, … | 19.2.18 / 19.2.7, 1.4.4, dom 10.4.2, react 16.3.3 | |
    | `sharp` (Next's optional) | 0.34.5 | 0.35.5 | inside Next 15.5.29's own range `^0.34.3 \|\| ^0.35.5` |

  - **Review round (14 Nov 2026, the coordinator's six):**
    1. PostgreSQL stays on Alpine: `imageRefProblem` refuses any `postgres` reference without `-alpine<X.Y>`, digest or not. Text indexes sort by the C library's collation; musl's compares byte by byte, every index here was built under it, and glibc's `en_US.utf8` orders differently, so a Debian image would read them in the wrong order, silently. `TestImageRefRule` (`postgres:15.20`, `postgres:15.20-bookworm@sha256:…` refused); mutation: `postgres:15.20` in compose, `-bookworm` in CI, each red.
    2. Both deploys pull the released images (`docker compose pull --ignore-buildable`) before anything stops: production in "Build the images", staging in its own step before the database reset (which itself stops the backend). A Docker Hub failure now fails the deploy with everything serving, instead of after the backend stopped, with the failure restart retrying the same pull. `TestBothDeploysBuildThenStopTheBackendThenTheAddOnBeforeStarting` requires the pull before the first `docker compose stop`; mutation (pull removed in either file, production's pull moved after the stop, staging's moved into its build step after the reset's stop): red. Staging's Compose is v5.5.1 (has `--ignore-buildable`).
    3. React parity: Next 15.5 renders App Router pages with the React it vendors (`next/dist/compiled/react`: 19.2.0-canary), while tests and tsc ran the installed 19.3.1. `react`, `react-dom` 19.2.9 and `@types/react` 19.2.18, `@types/react-dom` 19.2.7, the newest stable on the vendored minor. React's majors and minors move only with Next's: `ui/src/lib/__tests__/react-parity.guard.test.ts` holds all four to the vendored major.minor (mutation: `react-dom` 19.3.1, red).
    4. Go 1.27's `encoding/json` (v2-backed) words its errors differently, and those texts reach clients: `decodeJSONStrict` failures are answered with `details.body = err.Error()` (`handlers/bundles.go:178` and 61 more). Wording only, no status or code changes; no UI or test reads the text.
    5. The guards: `fromImages` reads `FROM [--platform=…] image [AS stage]` and skips `scratch` and earlier stage names; an `@sha256:` digest pins; an `-alpine` tag must name its Alpine release (or carry a digest); `.yaml` workflows are scanned; both guards walk `git ls-files`, not the disk (`repoFiles` over `trackedFiles`). `TestImageRefRule` covers each; mutations (a `--platform` stage on `golang:1.27-alpine`, `alpine:3.24` as a named stage, a tracked `.yaml` workflow with `node:22` and `checkout@v7`, the bun digest removed): red; an untracked stray Dockerfile: ignored.
    6. DEPLOY.md's DSN rules corrected to pgx 5.11's changelog: the first `@` ends the user part, a malformed `%` and a space are errors, `+` in the query is literal, `#` is ordinary data.
  - **Cutover (owner, just before 7q merges to main):** re-run the read-only check on production that no GIN or GiST index and no `bpchar` column and no `btree_gist`, `ltree` or `pg_trgm` extension (`select extname from pg_extension`, for 15.19's ltree btree item) exists (`SELECT … FROM pg_index JOIN pg_am …` / `information_schema.columns WHERE data_type = 'character'`), which PostgreSQL 15.19's and 15.20's release notes ask to REINDEX after the update. None found: no action. Any found: REINDEX them after the deploy.
  - **Vulnerabilities.** `govulncheck@v1.9.0` (DB of 11 Nov 2026), every module: before (Go 1.25.14, the images' Go) the backend had 14 reachable (11 standard library: `net/http` HTTP/1 and HTTP/2 server crashes and desyncs, `crypto/x509` compromised intermediates, `crypto/tls` ECH, `net/textproto`; `x/text` GO-2026-6629 and GO-2026-5970; pgx GO-2026-5004, SQL injection via dollar-quoted placeholders), the TrueNAS add-on 9, reference 9, conformance 11 (all standard library); after: **none in any module**. pgx 5.12 and 5.13 also fix GHSA-j38q-h3ph-jjpq and GHSA-95fj-3fxw-g62g (high) and GHSA-w9rp-vfrh-q8vv. `bun audit`: 99 before (3 critical, 46 high: Next, undici, vite, sharp, …), 5 after, none fixable inside the rule: `postcss@8.4.31` (2 high, 2 moderate; Next 15 pins it exactly, used at build time on our own CSS only; Next 16 takes 8.5.23) and `minimatch@10.2.6` (1 moderate, a glob DoS in `@typescript-eslint` under lint; its fix is minimatch 11). Both leave with the Next 16 major below.
  - **Majors, for the owner (not taken).** Order agreed by the owner (14 Nov 2026), also in NEXT.md 0h2: (1) contract 2 ships (staging checks, 6.10's 48 hours, cutover); (2) one Next 16 slice with TypeScript 6.0, ESLint 10, Vitest 5, jsdom and jest-dom; (3) Redis → Valkey 9 (the latest stable major; 9.2 now) (BSD, drop-in) on a fresh volume, since everything in Redis is rebuildable (checked on production 14 Nov; plan in NEXT.md 0h2); (4) PostgreSQL 15 → 18, rehearsed on staging, then production in a short window after the automatic `pg_dump`, within about six months; (5) TypeScript 7 once Next supports it.

    | Component | Current | Latest | What changes | Risk | Migration |
    |---|---|---|---|---|---|
    | PostgreSQL | 15.20 | 18.7 | three majors (16, 17, 18): SQL/JSON `JSON_TABLE` (17), asynchronous I/O and `md5` password authentication deprecated (18) | data | `pg_dump`/`pg_restore` or `pg_upgrade` of both databases, a rehearsal on staging; PG 15 is supported to Nov 2027 |
    | Redis | 7.4.12 | 8.6.7 | Redis 8: modules in core, AGPLv3 joins the licence options; 7.4 is already RSALv2/SSPL | low (a cache and the claims snapshot) | planned as Valkey 9 instead (step 3 above); staging first |
    | Next.js (+ `eslint-config-next`) | 15.5.29 | 16.4.6 | Turbopack the default bundler, `middleware` deprecated for `proxy`, synchronous Dynamic APIs removed, linting out of the build and `next lint` (our `lint` script, deprecated since 15.5) gone, flat ESLint config by default (v16.0.0 release notes) | medium | `@next/codemod` upgrade and the `middleware` → `proxy` codemod, the lint script to the ESLint CLI; clears the postcss audit findings |
    | TypeScript | 5.9.3 | 7.0.2 (6.0.3 between) | 6.0 turns 5.x deprecations into errors; 7 is the Go-native compiler | medium | through 6.0 first, fixing any deprecated `tsconfig.json` option |
    | ESLint | 9.39.5 | 10.15.0 | eslintrc support removed, Node `^20.19 \|\| ^22.13 \|\| >=24` (v10.0.0 notes) | low | with Next 16 (its flat config) |
    | Vitest | 4.1.12 | 5.1.0 | Node 22 and Vite 6.4 required, `sequential` option and deprecated entry points removed (v5.0.0 notes) | low | `vitest.config.ts`, rerun the 399 files |
    | jsdom | 29.1.1 | 31.0.1 | two majors of the test DOM | low | with Vitest 5, its Changelog read first |
    | `@testing-library/jest-dom` | 6.9.1 | 7.0.1 | `@testing-library/dom` a required peer (present), Node 22 (v7.0.0 notes; 6.10.0 is deprecated for shipping the same) | low | with Vitest 5 |
    | Go modules | | | none: no direct dependency has a newer `/vN` | | |

  - **Verified:** every Go module `go vet` and `go test` (backend also against the live `syndra_test` database: it applied 000095, rolled back to `94|f` as found); the conformance kit end to end as CI runs it (both references, every `-mutate` run: 0 fail); `-mod=readonly` linux builds of `cmd/api`, `cmd/syndra-world-export`, `cmd/claims` and the TrueNAS add-on; UI `test` (4261), `lint`, `typecheck`, `build` on Next 15.5.29; `bun install --frozen-lockfile` clean. The images themselves build in CI's images job (no Docker on the build Mac). DEPLOY.md says what an operator sees when a pinned image changes, and pgx's DSN parsing.
- [x] 7r TrueNAS page bugs and refinements; Basic hides the technical parts (visual check on staging, 15 Nov 2026).
  *Done (15 Nov 2026, slice 7r):*
  - **1** `account.purge` no longer says "and its files". TrueNAS middleware TS-25.10.7, `plugins/account.py` `do_delete`, removes the account, its primary group when asked, the SMB passdb entry, and `shutil.rmtree(os.path.join(user['home'], '.ssh'))` only; there is no `rmtree` of `user['home']`, and the add-on calls `user.delete` with `[id]` and no options. Label: "Deletes an account" (`OPERATION_LABEL`; see the review round below for why it names no system); the false claim removed from the dormant-sweep acknowledgement (`filesSentence`), the unconfirmed-change wording (`wording`), the audit vocabulary (`audit-vocabulary.ts`) and the backend's `CONFIRMATION_REQUIRED` text (`handlers/inventory.go`).
  - **2** `HomeLocationDialog`: no default folder; a disabled "Choose a folder…" option, the primary "Use this folder" off with the reason "Choose a folder first."; the lede says the same folder serves every role.
  - **3** `MappingRow`: the count is "N people hold this role" (`[data-holders]`); nobody holds it is said once, as the convergence button's reason.
  - **4** `lib/lifecycle-reason.ts` `lifecycleWhy` is the one place a lifecycle reason becomes words (the reach card and Maintenance both call it): the add-on's own codes (`startup`, `shutting down`, an unrecognised configured state) are sentences, an operator's text is quoted.
  - **5** `password.set`, `holdings.list` and `item.checkout` have labels; `OperationLabels.test.ts` fails for any registry operation id without one.
  - **6** `SectionPill` draws a `bg-canvas` band (`[data-section-band]`) behind the pill, 64px like the `glideTo` clearance, only while the pill shows.
  - **7** `DormantAccounts` uses `formatSignIn` (A-10 §6) for the whole column: "Last signed in 15 Oct", never "23 days ago" beside it.
  - **8** The Mappings card header says the freshness of what each group opens only when the read line below (which owns the Read again control) is not shown.
  - **9** `AppShell`: `overflow-clip` on the shell (never a scroll container, so nothing can scroll the window and push the top bar off) and `relative` on `#app-scroll` (the containing block of what is positioned inside it, such as the `sr-only` live regions). Not reproduced in a browser (the staging page is not reachable from here); the cause is argued from the layout, and the guard is the two classes.
  - **10** "Not created by Syndra" rows: the username has a 220px seat and the standing a seat of its own (`[data-standing-seat]`), so the status starts at one x.
  - **R1** TrueNAS add-on: `statsMeta.Began` records the first collection; while the buffer still starts there, the gap reads "Not collected yet: counting began on 9 Nov" (`reasonBegan`), and "The add-on no longer holds these days" only for days it dropped. UI: `gapLines` groups one reason over the same days across series and consecutive days, sorted by date, and drops days outside the window shown; `gapWhere` names the series.
  - **R2** `standingWords`: not usable, nobody waited on, is "Switched off" with why it may be (hold, deactivated in Zitadel, switched off on the system by hand) and what to do; "yet" is gone where it would promise an end. (Reworded in the review round below.)
  - **R3** `CopyableValue` copies through `navigator.clipboard`, or `document.execCommand("copy")` on the plain-http LAN where there is no Clipboard API, so the button is "Copy" (with its "copied" live announcement) wherever the browser can copy; "Select" remains only where it cannot.
  - **R4** `AdvancedOnly` (an `Unfold` on `useIsAdvanced`) hides in Basic: the operations' ids, Maintenance, "What the add-on also says", a mapping's ids and a finding's exact sides; the "Merge findings" glossary term is Advanced, Basic says the sentence. Rule written in `basic-advanced-ia/design.md`.

  *Review round (15 Nov 2026, PR 189, Opus review):*
  - **R1 at the source** `PutStats` (`stats_store.go`) is the one place `Began` is set: a metric whose `HeldFrom` is later than the oldest day the buffer could hold has never trimmed, so `HeldFrom` is the day counting began. This covers all three metrics (sessions, used-bytes, home-bytes) and adopts a `state.db` written before `Began` existed (staging, prod); at or before the oldest day it may have trimmed, so `Began` stays empty and the gap stays "no longer holds". `TestADayBeforeCountingBeganIsNotCollectedYet` (all metrics), `TestStateWithoutBeganIsAdoptedOnlyWhenNeverTrimmed`.
  - **Section band** `--section-band` (`globals.css`) is the one value: the band's height, `#app-scroll`'s `scroll-padding-top` (anchors, focus: WCAG 2.4.11) and `glideTo`'s default clearance; `revealInAdvanced` no longer passes its own 24.
  - **Paused add-on, Basic** The reach card's next step is "Open Maintenance in Advanced" (`revealInAdvanced("maintenance")`; "Open Maintenance" in Advanced). Maintenance stays hidden in Basic: structure does not move with data.
  - **R2** `standingWords`: off with nobody waited on is Syndra's own doing by contract, and off outside Syndra is a drift, not a standing, so there is one statement and a next step that stays in Syndra ("Nothing for you to do. Syndra keeps it off while..."); the add-on's duplicate "The account is switched off." is not shown beside the state.
  - **Mappings row** the count is in the "Gives" column; the Technical fold is the row's last part and `AdvancedOnly` takes a `className`, so Basic renders no empty box (no line break, no gap).
  - **R3** `CopyableValue` plain-http path: a hidden read-only textarea, the reader's selection ranges and focused element restored, "Copied" only when `execCommand("copy")` returned true; otherwise it selects the value and announces "Couldn't copy {what}: select it and copy".
  - **Generic surfaces** `OPERATION_LABEL` names no system and no file claim (the manifest gives an operation no description to show per system): "Deletes an account"; dormant sweep, unconfirmed delete and the backend's confirmation text say what Syndra does ("Syndra does not delete their files"), not what a system does.
  - **Shell** `#app-root` is `relative` as well, so `overflow-clip` clips absolutely positioned descendants too.
  - **R1 `gapLines`** clips the end to the window; the window is the widest the block shows (a counter's 12 months); three or more series read as `Intl.ListFormat` in the display locale.
  - **Nits** `lifecycleWhy` caps and strips an operator's text like every add-on text (`capText`); "Not created by Syndra" status has `basis-[200px]`; `DormantAccounts` keeps `<time dateTime>`.
  *Review round 2 (15 Nov 2026, PR 189, Opus re-review):*
  - **R2 at the add-on** `standingOf` tells the three kinds of "off" apart: Syndra asked (`none`), TrueNAS has it off while Syndra wants it on (`waiting_on: operator`, "Switched off on TrueNAS, outside Syndra", next: Check for differences on Mappings), and bound to nobody ("not by Syndra"; nothing to do unless assigned). `Binding.WantsEnabled` is what each apply asked (written in the binding's own write on create and adopt, so no extra log entry; `rememberWishes`), `Store.Wishes` replaces `WantedSMB`, `Subject.unbound` is absent from the wire; an unread map claims nothing. A binding from before the wish was kept keeps the old rule. AUTHORING's standing rule says `none` is only for the first case; the reference add-on's comment says why its `none` is true. `standing_off_test.go`; the UI shows the add-on's note and next as written (`account-standing.test.ts`).
  - **Neutral dormant copy** "The accounts cannot be brought back. They held {size} on {name} when it was last read." (unknown: "Syndra cannot see how much they held on {name}."); the backend's confirmation text is the first sentence alone; no sentence promises anything of the files.
  - **`Began` exactly** `beganFor` in `PutStats`, one transaction: Began is set at the first successful collection (stored watermark empty, this run's not), never again. A version-0 meta is migrated once by the held-from rule, then `Version` is 1 and it is never judged again. Tests: first success, failed first run then success, clock moves, both migration branches for all metrics.
  - **Band offsets** `glideTo` uses the scroller's computed `scroll-padding-top` plus the target's `scroll-margin-top`, the browser's own formula; the padding is `#app-scroll:has([data-section-band])` (globals.css), so a page without a band is not shifted.
  - **`revealInAdvanced`** focuses the panel after the glide (tabindex -1 when needed), in the shared effect.
  - **Nits** the fallback textarea is no longer `aria-hidden`; `ConvergeEntitlements.test.tsx` waited for the apply call and then read the result synchronously (a race with the mutation resolving): it now waits for the sentence.
  *Review round 4 (15 Nov 2026, PR 189, Opus review of 62bf81ca):*
  - **Legacy wishes** `deriveWishes` (`wishes_derive.go`): at the first complete live `/subjects` read, after the answer (its writes make every copy a read of the past), each binding's unset `WantsEnabled` and `WantsSMB` is set to what the NAS holds, through the logged `store.bind` path, then a marker in `bucketSettings` (`wishes_derived`, `store.setting`) stops it for good. Without it a contract-1 binding's hand switch-off (a `theirs_only` finding nothing converges) would read "Syndra keeps it off" for ever. A truncated read decides nothing; a failed write leaves the marker unset. `account.adopt` (the operator's) writes no wish, so the old rule holds for an account adopted after the derivation ran, until the next apply; adoptions made before it ran are covered by it.
  - **Wishes are never false** `rememberWishes`: an off wish is written before the lock (and in the adoption's own binding write), an on wish only after an enable landed (applied or no change; not blocked, not `Unverified`); the SMB wish is kept unless the apply was blocked. The adoption path now logs one more binding write (the on wish after a landed enable).
  - **One matcher** `Binding.uid` (UID, else account_id) is the one reading of which account a binding names: `UIDBound`, `Wishes`, `BoundUIDs`, `GetBinding`.
  - **Release** an account bound to nobody reads "Switched off on TrueNAS. No one in Syndra is assigned to it." (a release leaves the NAS as it was, so who switched it off is unknown).
  - **`beganFor`** a version-0 meta that never collected falls to the first-collection rule.
  - **Kit** `standing.off` (guide: get-subjects, `-mutate` only): after the withdraw-only apply, the account the app reports off answers `usable: false, waiting_on: none`; the reference's `off-waits-on-operator` defect must fail it.
  - **Cutover** NEXT.md: the operator's read-only check of production's `state.db` for legacy-shaped bindings.
  *Review round 5 (15 Nov 2026, PR 189, Opus review of 69b76fbd):*
  - **Derivation is one safe path** `deriveWishes` handles each binding alone, as its other writers do: under `s.subjects.lock(subject)`, through the logged `mutate(store.bind)`, with the write a compare-and-set (`Store.DeriveWish`: one `Update` that checks the binding still names the same account and name and that each wish is still unset). A binding released, purged, renamed or given a wish since the read stays as it is (`wishGone`, `wishSet`, `wishChanged`); a changed binding leaves the marker unset so the next read finishes it. No lock is held across NAS I/O. It derives only from a read the fresher kept (`fresher.keeps`), only when the add-on is active, and counts in flight (`life.Begin`), so read_only and draining skip it. Tests hold a subject's lock as an apply does and change the binding under the derivation (deleted, an off wish written, renamed); run with `-race`.
  - **One account-identity rule** (for apply, plan, wishes, stats and `accountBound`; 7t covers the rest) `boundAccount`'s: an uid outside the id range names none by number, only the account its name finds. `resolveBinding` / `boundAccounts` (`binding_accounts.go`) apply it to a read of the accounts for the wishes, the derivation, the statistics' `boundUIDs` and `accountBound` (replacing `Wishes`, `BoundUIDs`, `UIDBound`, which matched by number alone); the plan, `locate` and the adopt operation already used it. A record-key binding is derived from the account its name finds, is bound for the wishes, and is never matched to a stranger holding that number. The NEXT.md cutover check stays, as a must.
  - **Kit** `standing.off` judges `state.<account>` and `standing` from the same `/subjects` document.
  - **Docs** AUTHORING's standing rule says `none` also covers an off account no subject is bound to (nothing in Syndra waits on anyone), said as what it is; `standingOf`'s comment matches. This heading and its numbering fixed.
  - **Nit** `rememberOffWish` replaces the fake `ApplyOutcome` before the lock write.
  *Review round 6 (15 Nov 2026, PR 189, Opus review of 2dc62575):*
  - **The legacy wish derivation is gone** (`deriveWishes`, `deriveOne`, `Store.DeriveWish`, the `wishes_derived` marker, `fresher.keeps`, their tests). It had needed a fix in three rounds running (a race with the applies, the lifecycle, a stale read, then request latency and a stale-read gap): a background rewrite of every binding at the moment of cutover is the wrong shape. A bound account that is off and whose wish is unknown (a legacy binding never applied since wishes were kept) now makes no claim: `waiting_on: none`, "Switched off on TrueNAS.", next "If no one in Syndra asked for this, use Check for differences on the system's Mappings page." The wish fills in by itself at the next apply (`rememberWishes`, `rememberOffWish`, unchanged). A known off wish says "Syndra keeps it off" (now the add-on's own `next`, since the UI's fallback is the neutral sentence); off while wanted on says "outside Syndra". AUTHORING, the copy guide and the UI fallback say the same; the reference add-on gives its off account a next of its own. NEXT.md's cutover line no longer has a derivation step.
  - **An adopted account outside the id range is no longer lost after a rename (since 7b)** `boundAccount` took any uid outside the range this add-on hands out for a contract-1 record key and matched it by name only; `account.adopt` binds the account's real uid (hand-made accounts start at 3000, the range at 200000), so after a rename on the NAS stats, standing and `accountBound` missed it and the next apply created a second account, leaving the member's data on the first. Now the binding says what its uid is: `Binding.UIDKind: "account"`, written by create, both adoptions and boundAccount's rewrite. A marked uid is matched by number whatever its range (`ownsUID`); an unmarked one (contract 1) is matched by name as before, and the apply that finds the account by name (`locate`) rewrites it once with the real uid and the marker, in the logged `store.bind` under the subject's lock. `encodeBinding` stamps `account_id` only for a marked uid, so a record key never reads as a stable account id. The stats fixture is back on the production-default range with its 3051 account (marked, as an adoption), so it now passes for the right reason. Tests: `binding_uid_test.go`.
  - **Kit** `standing.off` skips (not judged) when the second read shows the account back on.
  - **`plan.go`** builds its lookups with the shared `indexAccounts`.
  - **Test restored** a binding holding `account_id` alone reads as bound in `wishesFor` and the standing. (Removed again in round 7: nothing writes that shape.)
  *Review round 7 (15 Nov 2026, PR 189, Opus review of a2cafb3f):*
  - **A marked uid outside the range no longer takes over a reused number** (`boundAccount`; for apply, plan and `resolveBinding` only: the account operations, storage status and the backend's drift are 7t): a number is followed to a new name only inside the range (never reused: `ClaimID`'s high-water mark); outside it, name gone and number held by another name is a `BindingConflict` (new words: "its number is now held by a different account"), the account is neither renamed, written nor created, the binding is untouched, and `resolveBinding` attributes nothing to the holder (stats, standing, `accountBound`). Name and number both gone is still the re-create. The marker now drives refusal only. It reaches the operator as the apply's and the plan's `blocked` outcome with the next step in its words (the backend settles it as the target refusing the row; the same path as a name another account holds). Tests replace the one that asserted the unsafe rename: out-of-range rename and reused number (conflict, no write, no create, holder unattributed), in-range rename followed, name-still-matches keeps working, both gone is re-created. The stats fixture's range is back to 3000-3999 because it tests the in-range rename; the production-default range is the one the new tests use. design.md 9.4b.
  - **`next` reads right to both audiences** TrueNAS's known-off `next` is "Nothing for you to do. It stays off while the person it belongs to is on hold or deactivated, or holds no role that reaches TrueNAS."; the unknown-wish standing leaves `next` empty so the UI's per-audience fallback applies (operator: the Check for differences line; member: ask makerspace staff). AUTHORING says `next` is shown to the member too. Check for differences compares what Syndra wants with the NAS; whether it re-sends a subject with no difference was not confirmed, so no sentence promises that it records a wish (the wish fills in at any apply).
  - **Kit** `standing.off` is `unjudged` as well as skipped when the account is back on (a skip alone reported a pass); a self-test shows it.
  - **Comments** `binding_accounts.go`, `boundAccount` and `inIDRange` say what the rule is now. **YAGNI** the `account_id`-alone read branch and its test are gone: nothing writes it.
  *Review round 8 (15 Nov 2026, PR 189, the last round of 7r):*
  - **Claims narrowed** design.md 9.4b, the `binding_accounts.go` header and the bullets above now say what 7r covers (apply, plan, `resolveBinding`: wishes, stats, `accountBound`) and that the account operations (`recordID`), `storageStatus` / `smbActivity` and the backend's `account_id` matching do not follow the rule yet; slice **7t** (below, at the end) covers them with the operator-surfacing gaps. Not started here.
  - **Conflict words** "The account {was}, bound to this subject, is gone, and its number is now held by a different account, {name}." Next: "Release this subject's binding; the next apply creates a new account for them. Adopt {name} instead only if it is theirs." (copy guide row and test updated.)
  - **`store.go` comments** (`Binding`, `UIDKind`, `AccountID`, `encodeBinding`) describe the current rule; `account_id` is the uid, which outside the range TrueNAS can reuse (7t).
  - **The id range is validated at start** (`idRangeFromEnv`, `main.go`): `TRUENAS_ID_FLOOR` and `TRUENAS_ID_CEILING` must be integers, the floor above 65535 (the numbers TrueNAS hands out itself, `identity.go`), the ceiling above the floor, or the add-on does not start and names the variable (before, a bad value silently fell back to the default). `docker-compose.yml`'s defaults (200000, 999999) pass (`TestComposeDefaultsForTheIDRangeAreTheAddOnsDefaults`); staging's and production's own `.env` are on the hosts and were not read: the operator confirms no override below 65536 before deploy.

- [ ] 7t One account identity (added after the 7r reviews, 15 Nov 2026; own design first). 7r made `boundAccount` the rule for apply (`locate`), plan and `resolveBinding` (wishes, statistics, `accountBound`). These places each still use their own rule, and were so before 7r; they are fixed together here, not patched one by one:
  - **Account operations**: `recordID` (`addons/truenas/subjects.go:423-445`), used by set-password, rotate and purge, found the account its own way (fixed in 7t-1: deleted).
  - **By name**: `storageStatus` and `smbActivity` looked the account up by the binding's name (fixed in 7t-1).
  - **Backend drift**: matching on `account_id` (`backend/internal/services/drift/addon.go:701-707` and `:380-397`). An out-of-range uid can be reused by TrueNAS, so `account_id` is not an identity there (what Syndra holds instead is 7t's design question).
  - **Operator surfacing of a conflict**: a sweep outcome `blocked` with `conflict` is dropped by `convergeBound`; `applyWire` does not decode `consequence`; the pending page's "Syndra stopped sending" line shows no reason. So the next step the add-on words (release the binding; the next apply creates a new account; adopt the holder only if it is theirs) reaches the operator only through some paths.
  - [x] **7t-1 One account identity in the TrueNAS add-on.** *Done (15 Nov 2026):* one module, `addons/truenas/identity_resolve.go` (absorbs `binding_accounts.go`), owns which account a binding names, with `boundAccount` its only rule (moved from `apply.go`, with `inIDRange` and `indexAccounts`). `resolveLive(subject, who)` asks it of the target now and rewrites the binding once (logged `store.bind` under `who`; `who` nil for a read operation, which writes nothing, since it runs outside the subject lock and the lifecycle gate); `resolveIn(binding, ix)` asks it of one read; `operatedAccount(subject, who)` maps it to an operation's refusal: 422 no binding or account gone, 409 on a conflict (the conflict's own detail and next step, `conflictRefusal` over `BindingConflict.words()`) or on two accounts holding one key (`accountWhere` refuses that as `errAmbiguousAccount`), 500 `STORE_UNREADABLE`. `recordID` and `lookupOne` are deleted. Callers: `setPassword`, `rotatePassword` and `purgeAccount` write to `r.Account.ID` (purge's self check is on the account's current name); `storageStatus` drops its own `user.query` and reads `r.Account`; `smbActivity` filters the audit log on `r.Account.Username`; `adoptAccount`'s repeat check, `locate`'s bound half and `planOne` go through the module; `boundAccounts` (wishes, statistics, `accountBound`) is a view over `resolveIn`. A conflict is a 409 everywhere, `OPERATION_FAILED` for an operation (no wire change; whether operations answer `BINDING_CONFLICT` is for 7t-2), and nothing is sent to the holder or logged. An apply whose lookup is ambiguous answers 409 `APPLY_FAILED` (was 502). **Legacy policy B** (design 7t D6): an unmarked binding outside the id range whose name is gone while its number is held by another name halts as a conflict and creates nothing (`legacyHaltsOnHeldNumber`, one constant; **confirmed by the owner 17 Nov 2026**: `false` is policy A). Tests: `identity_resolve_test.go` (`TestEveryCallerResolvesOneIdentity`: 14 cases, marked/unmarked × in/out of range × name found/gone × number free/held, asserted through set, rotate, purge, storage.status, activity.get, adopt's repeat, apply, plan, stats, standing and `accountBound`, with the record id each write went to, no write and no log record on a conflict, the holder's number absent from the answer; `TestOnlyAWriteRewritesTheBinding`); operations tests moved to the in-range rename and to an ambiguous number with the name gone. Mutation-checked (each red): the name-held conflict branch, the in-range rename follow, policy B flipped, and each of the nine callers reverted to `recordID`'s by-uid-then-name lookup, plus `activity.get` filtering on the stored name. *Review round (15 Nov 2026):* naming an account reads the user table alone (`accountWhere` over `readAccountsWhere`; `Subject.groupIDs` carries its raw group ids), so set, rotate, purge, storage.status, activity.get and adopt answer with `group.query` refused; only the apply names groups, in `locate` (`subjectWhere` = `accountWhere` + `nameGroupsOf`), and `memberPlaces` is back on raw ids (`TestOperationsNeedNoGroupTable`). Two accounts on one key read the same in the plan as live: `accountIndex` marks the key and `resolveIn` returns `errAmbiguousAccount`, the plan blocks and the apply answers 409 in `ambiguityWords` (one source with `lookupRefusal`; `TestPlanAndApplyAgreeOnTwoAccountsOnOneUID`). Adoption's unreadable binding goes through `liveRefusal` (500 `STORE_UNREADABLE`). `legacyReply` takes a contract-1 apply's standing from the account `resolveIn` names, never whoever holds that uid now. The name-first rule is stated in the module header and pinned again (`TestANameThatFindsTheRecordedUidWinsOverASharedUid`). The table's policy-B case takes its expected conflict from `legacyHaltsOnHeldNumber`, and the whole package is green under both values. Mutation-checked again (each red): the live lookup reading the group table, the index keeping one of two accounts on a uid, the apply's raw ambiguity error, adopt's raw store error, `legacyReply` by uid alone, the name-held conflict branch, the rename follow, the policy-B branch removed, and `activity.get` reading the audit log before resolving.. *Copy round (15 Nov 2026):* the words the resolver and the conflict say are in the copy guide's voice (TrueNAS named, never target, subject or binding; "Stop tracking" is the system page's own control for `account.release`): `ambiguityWords`, `errAccountNotFound`, the no-account and gone refusals, both `BindingConflict.words()` cases the resolver raises; the apply's lookup failure is `statusFor` again (a rate-limited TrueNAS is 429, `TestARateLimitedLookupIsA429OnTheApply`, mutation-checked). design.md 9.4b.
  - [x] **7t-2 Attribution on the wire.** *Done (15 Nov 2026):* `GET /subjects` says whose account is whose. Each row carries `subject` (the subject the add-on's binding names it for; absent when none does, never on `self`), and the reply carries `conflicts[]`: each binding that names no account because another holds its name or number, with the subject, the holder's `username` and `account_id`, `adoptable`, `bound_to`, and the add-on's `detail` and `next`. One conflict shape, `v2/schema/common/binding_conflict.json` (`detail` and `next` required), is shared by an apply or plan outcome's `conflict`, the `BINDING_CONFLICT` problem's extension member and `subjects.json#/$defs/Conflict`; `apply_response.json#/$defs/BindingConflict` is removed. It has no `additionalProperties: false`: it is a reply shape, and replies MAY carry more (guide, "Replies MAY carry more"). Registry `BINDING_CONFLICT` now also covers a recorded name or number held by another account. `account_id` keeps its value, the uid (**confirmed by the owner 17 Nov 2026**), and its meaning is narrowed (AUTHORING ch. 2 step 3 in BCP 14 words, citing SCIM RFC 7643 §3.1, where `subject` plays `externalId`): it is unique within one read and survives a rename, but it is not an identity across reads. An app that can hand an id out again obliges the add-on to keep the name beside the number and to report a conflict. `subject` and `conflicts` are core to contract 2, not a feature (**confirmed by the owner 17 Nov 2026**).
    - **TrueNAS:** `attribute(subjects)` in `identity_resolve.go` replaces `boundAccounts` and runs `resolveIn` once over every binding. It yields `subjectOf` (the wire, keyed by uid and name, so a uid two rows share names neither), `bindingOf` (wishes, statistics and `accountBound`, unchanged) and `conflicts`. (A binding whose lookup is ambiguous was first left in neither list and logged; the review round below makes it a conflict.) `BindingConflict.Detail` and `Next` are set once, where a conflict is made (`worded()`). `/subjects` attributes inside the read and keeps the result with the copy, which every write forgets. Bindings that cannot be read answer 500 `STORE_UNREADABLE`, never a reply with no row bound, and `Store.Bindings` now errors on an undecodable binding instead of skipping it (a skipped binding would have read as gone). Operations still answer a conflict 409 `OPERATION_FAILED`: `error.json` has `BINDING_CONFLICT` carry an apply outcome (`effect: blocked`), and an operation has no vocabulary for that. The "stable" comments on `account_id` are corrected.
    - **Reference:** `subject` per row and `conflicts: []` (it never has a conflict). It gains `account.purge` (elevated key, off accounts only) and takes ids from a high-water mark (`newID`; `1001 + len` would have collided after a purge). New defects: `omits-subject`, `conflict-also-attributed`, `reused-id`.
    - **Kit:** `subjects.subject` checks one subject per row and never one on `self`; with `-mutate`, no row carries the kit's subject before the create and exactly its own row does after. `subjects.conflicts` checks that a conflict's subject is on no row and is listed once, and that the kit's bound subject is never a conflict. `identity.not-reused` is **advisory**, because the guide lets an app recycle ids. It runs under `-mutate` with `-elevated-key-file` when `account.purge` is offered: it purges the kit's own account once that account is off, creates it again, and expects a new `account_id`.
    - **Backend:** `TargetAccount.Subject`, `SubjectsResult.Conflicts` (`SubjectConflict`) and `BindingConflict.Detail`/`Next` are decoded and kept. `unmanaged`, `of()`, the sweep and the findings are unchanged (7t-3).
    - **Tests:** backend schema refusals (an empty row subject, a conflict without its subject or next, a problem's conflict without words, a plan conflict without its holder) and `TestSubjectsReplyLands`. TrueNAS: `TestEveryCallerResolvesOneIdentity` asserts `/subjects` for all 14 cases (marked, unmarked, renamed, conflict), plus `TestAReleasedAccountCarriesNoSubject`, `TestSubjectsWithAnUnreadableBindingAreRefused` and the ambiguity case. Kit: the three defects, `TestAReusedIDWarnsAndNeverFailsTheRun`, `TestWithoutAnElevatedKeyNothingIsPurged` and `TestAttributionIsJudgedOnAReadsOnlyRun`. design.md 9.4b.
    - **Mutation-checked (each red):** the backend's `Conflicts` decode removed; the TrueNAS conflicts dropped from `attribute`; the row `subject` dropped; rows keyed by uid alone (`TestASharedUIDPutsTheSubjectOnTheNamedRowOnly`); `Store.Bindings` skipping an undecodable binding; a conflict built without `worded()`; the reference minting `1001 + len`; the kit's `subjects.subject` own-row assertion, its two conflict rules, and `identity.not-reused`'s id comparison.
    - *Review round (15 Nov 2026, Opus on `97012365`):* every binding is now attributed, a conflict, or gone, never silent. Every conflict carries a required `reason` (`name_held`, `number_held`, `ambiguous`, `claimed_twice`), and `binding_conflict.json` switches on it with `if`/`then`/`else`: `ambiguous` lists `holders` (two or more, never `username`/`account_id`, never adoptable), and the rest name the holder. `attribute` emits an ambiguous binding as a conflict listing every account on the key `boundAccount` asked (`holdersOf`), in `ambiguityWords`; the log line is gone. Two bindings naming one account give it to neither: both become `claimed_twice` conflicts naming each other in `bound_to`, the uid leaves `bindingOf` (no stats or wish) and stays bound for `accountBound` (`claimedTwice`), so no third subject can adopt it. Every binding reader decodes through `decodeBinding` and fails closed (`BindIfFree`, `BoundUsernames`, `GetBinding`, `Bindings`): an undecodable binding's name is never free. A store failure on a live read answers 500 instead of serving the kept copy as `stale: unreachable` (`fresher.fallback`). `openapi.yaml` gives `/subjects` its 500. The kit validates each conflict per reason (`subjects.json#/$defs/Conflict`) under `subjects.conflicts`, refuses `-elevated-key-file` without `-mutate`, and its last line says the purge as the one deletion. AUTHORING documents the three states, the four reasons and `-elevated-key-file`; `scripts/README.md` keeps staging's `-mutate` run keyless. The reference cannot model `ambiguous` or `claimed_twice` (a binding is the account's id, handed out once, and only a create binds), so no defect silences one; the per-reason shape is watched failing by `TestAttributionIsJudgedOnAReadsOnlyRun`. Tests: `TestPlanAndApplyAgreeOnTwoAccountsOnOneUID` (the ambiguous conflict), `TestTwoBindingsOnOneAccountGiveItToNeither`, `TestAnUndecodableBindingIsNeverReadAsFree`, `TestAStoreFailureIsNeverServedAsAnUnreachableNAS`, `TestAnElevatedKeyWithoutMutateIsRefused`, and the backend schema refusals per reason. Mutation-checked (each red): ambiguous emitted as silence, the claimed-twice branch picking the last binding, `BindIfFree` and `BoundUsernames` skipping a bad binding, the fallback serving the copy on a store failure, and the kit's per-reason validation.
    - *Round 3 (15 Nov 2026, Opus on `8a64325e`):* `claimed_twice` now holds wherever a binding is asked, not only on `/subjects`. `attributeOver(bindings, read)` is the one judgement. `attribute` runs it over a whole read for `/subjects`, the plan (`handlePlan`, once per plan; `planOne` picks `at.of(subject)`) and a contract-1 reply's standing. `resolveLive` runs it over exactly the accounts its verdict can depend on: `keyRows` reads the user table alone, by exact key, for the subject's binding's name and number and for those of every other binding that holds the name or number of an account found. A binding's verdict depends only on the accounts on its own keys, so this equals the whole-read verdict, and an account past the list's cap is still found. So the apply (`locate`), the plan and every operation answer the same conflict for either claimant: `blocked` or 409 in its words, and nothing sent or logged. An ambiguous key now reaches the apply as the same `BINDING_CONFLICT` (reason `ambiguous`) instead of 409 `APPLY_FAILED`. `resolveIn` is now used only inside `attributeOver`. One wrapper, `unreadable`, makes every store failure `errStoreUnreadable`; `decodeBinding` goes through it. Every route answers it 500 `STORE_UNREADABLE` (`applyNow` gained the case; the apply's derive path had answered 502) in one sentence per scope (`storeUnreadableDetail`, `planStoreUnreadable`, `subjectsStoreUnreadable`; copy guide row), never the store's own error. A create whose binding could not then be recorded because another binding is unreadable answers the same. Adoption asks `attribution.bound` (accountBound's rule) over the same read instead of the names on file, so an account a binding names by number after a rename is not adoptable; `BindIfFree` still holds a name whose account is gone. Tests: `TestTwoBindingsOnOneAccountGiveItToNeither` (apply, plan and each of the five operations, for both claimants), `TestAnUnreadableBindingIsStoreUnreadableOnEveryRoute` (create, apply, plan, adopt, an operation), `TestAdoptRefusesAnAccountABindingNamesByNumber`, and the ambiguous apply in `TestPlanAndApplyAgreeOnTwoAccountsOnOneUID`. Mutation-checked (each red): `resolveLive` judging its binding alone, `applyNow` without the store case, adoption without `attribution.bound`, `decodeBinding` without the wrapper, and an operation answering the store's own error.
    - *Round 4 (15 Nov 2026, Opus on `4bae1d94`):* `keyRows` holds rows by TrueNAS record key (`recordID`), so one account read twice is one account. If it comes back under another name or number across the reads (a rename between them), nothing is judged: `errTornRead`, 503 ("TrueNAS changed while Syndra was reading it. Try again.", copy guide), on the apply and every operation. Before this, the account counted as two rows, the other claimant read as ambiguous, and the first was given an account a whole read calls `claimed_twice`. A read operation's fresher no longer serves a kept copy on a torn read. Where home folders go (`homeRoot`) failing to read goes through `unreadable`, so the apply answers 500 `STORE_UNREADABLE` as the plan does, not `APPLY_FAILED` with the store's text. `holdersOf` sorts holders by username, then account_id, so `/subjects` and the apply list them alike. Tests: `TestARenameBetweenKeyReadsIsATornReadNeverJudged` (apply and `password.set`, with a rename in the fake between sub-1's and sub-2's key reads), `TestAnUnreadableHomeSettingIsStoreUnreadableForPlanAndApply`, `TestAmbiguousHoldersAreInOneOrder`, and `TestTheAmbiguousApplyRefusalSatisfiesTheSchema` (the 409 body against `error.json`, its conflict against `binding_conflict.json`). Mutation-checked (each red): the torn-read check removed, `statusFor` without the torn case, `decodeDesired` returning the raw store error, and the holders left unsorted.
    - *Round 5 (15 Nov 2026):* the torn read gets its own code, `TARGET_CHANGED`, registered as 503 with `retry_after` (CHANGELOG, the guide's error table and writes rules, `openapi.yaml`'s `Retry-After`). Before this it answered `APPLY_FAILED`/`OPERATION_FAILED`, which Syndra reads as possibly acted, and parked a change that never went out. `TARGET_UNREACHABLE` would say the target could not be reached, which is untrue here. The add-on answers it through one writer (`writeTornRead`, `Retry-After: 2`) from the apply, write operations and read operations. The backend follows the one existing path: `classifyStatus` reads it as unreached, as it does `TARGET_UNREACHABLE`, so the outbox requeues it (`unreachedWhy` words the reason truthfully). A member's action answered so is told to try again (503 `TARGET_CHANGED`), never "checking". Tests: the classifier table and the end-to-end call table, `TestAnApplyAnsweredTargetChangedIsUnreached`, `TestATargetThatChangedMidReadIsRequeuedNotInDoubt`, `TestAConfirmAnsweredTargetChangedIsTryAgain`, and the add-on's torn-read test asserting the code, `Retry-After` and `error.json`. Mutation-checked (each red): `classifyStatus` without the code, so it reads as possibly acted (three tests red), and the add-on answering `OPERATION_FAILED`.
  - [x] **7t-3 Backend attribution** (design 7t §6). From the 7t-2 review: the live `/subjects` read reads the NAS before the bindings, so an apply that lands between them can leave a subject in neither list for one read; the sweep must never conclude "deleted" for a subject with an apply in flight. From round 3: the backend treats every `/subjects` failure as `target_unreachable` ("It did not answer when Syndra last tried"; `drift.Inventory` in `backend/internal/services/drift/addon.go`, and the sweep). For a 500 `STORE_UNREADABLE` the NAS answered and the add-on's own records failed, so 7t-3 maps it to a truthful state (`read_refused` or a new one) with a next step (check the add-on's storage).
    - *Done (15 Nov 2026):* **The backend stops working out attribution.** A row is a recorded binding's account exactly when its `subject` is that binding's subject; the `account_id` and name matching is removed, not kept beside it: `unmanaged(accounts)` is the rows with no `subject`, `statesBySubject` replaces `stateByBinding`/`of()`, `partitionByPresence` and the inventory's standing go by subject. `target_account_bindings` stays the mirror for the outbox, Home and the UI.
    - **The finding.** `merge.BindingConflict` (`binding_conflict`), account-level beside `deleted_upstream` (`Subject.Conflict`, `merge.Conflicted`, `AccountLevel`; never convergeable, `AllOutcomes` and the invariants' policy table answer it). Migration **000096** widens `target_merge_findings_outcome_check` (down deletes `binding_conflict` rows first, then restores 000042's check). Keyed `(target, subject, "")`, `theirs` the add-on's whole report (`reason`, holder, `adoptable`, `bound_to`, `detail`, `next`). **Raise:** `raiseConflicts` from `/subjects.conflicts` before anything is classified, for every subject the add-on names; such a subject is neither planned, nor queued by `convergeUnbound`, nor read as absent, so the old wrong `deleted_upstream` + "recreate" is gone. **Close:** `persistFindings`' account-slot clear (`agreed`) the first pass the subject's own row is back; ended with the account gone, the slot becomes `deleted_upstream`. **Resolve:** `unbound` only (`resolutionFits`; `account.release`), **confirmed by the owner 17 Nov 2026**. `decorate` treats it like `deleted_upstream` (not adoptable, no policy).
    - **In-flight applies.** `db.AppliesInFlightSince(target, since)`: apply rows `in_flight` (also where a lost answer stays) or settled since the read began; the sweep asks it from `readBegan` (taken before the first read) and concludes nothing about those subjects that pass.
    - **Propagation.** `applyWire` decodes `consequence` and `conflict` (`ApplyResponse.Consequence`, `.Conflict`). A rejected apply, or a system row's re-plan that blocks, with a conflict raises the same finding (`raiseConflict`, through `merge.Conflicted`), **confirmed by the owner 17 Nov 2026**. A refused row's `last_error` is the add-on's `detail` then its `consequence`, as written (`refusalReason`, `addonWords`), for "{system} refused it:"; with none, "{system} gave no reason (status n).".
    - **`STORE_UNREADABLE` is not an outage.** `SubjectsResult.Code`/`.Detail` (the problem's `detail`, RFC 9457 §3.1.4, `problemDetail`); `readFailure` maps a read the add-on refused in its own code (and not an unreached one) to `read_refused` on the target's record, the sweep's result and the inventory's, with `AddonReconcileResult.Detail` the add-on's sentence and next step. The System page's Check now line shows it ("The check stopped before it concluded anything. {sentence}", `TargetOverview`) instead of "try again once it is reachable" and the raw reason.
    - **Requeue reasons** reach `last_error`, which the pending page and the stopped-sending list show, so they are plain now: "{system} could not be reached", "{system} changed while Syndra was reading it", "{system} is not taking changes right now" (the lifecycle release). The name is `addons.SystemNameOf`, the one path (moved from `handlers.systemName`, which now calls `SystemNameFrom` over its seams). Copy guide rows added.
    - **Tests:** drift `TestAConflictIsRaisedAndNeverPlannedOrCalledGone`, `TestAConflictForAnUnrecordedSubjectIsRaisedNotCreated`, `TestAConflictTheAddOnStopsReportingIsClosed`, `TestASubjectWithAnApplyInFlightIsNeverConcludedDeleted`, `TestAnUnreadableStoreIsARefusedReadNeverAnOutage`, `TestOnlyTheAddOnsAttributionMakesAnAccountManaged`, `TestPresenceIsBySubjectNeverByNameOrNumber`, `TestAConflictOrAnApplyInFlightIsNeverLiveOrStale` (the identity-matching tests replaced; existing fixtures carry `subject`); propagation `TestABlockedApplyWithAConflictRaisesTheSweepsFinding`, `TestARefusalWithoutAConflictRaisesNoFinding`, `TestAReplanBlockedOnAConflictRaisesTheSameFinding`, `TestUnreachedReasonsNameTheSystem`; addons `TestAConflictingApplyCarriesItsConflict`, the consequence in `TestABlockedApplyLandsAsARejectionWithItsOutcome`, `TestARefusedSubjectsReadLandsWithTheAddOnsWords`; handlers `TestABindingConflictTakesOnlyUnbound`; merge invariants over both account-level outcomes; db `TestMigration96_AllowsBindingConflictAndDownClearsIt` and live `TestLiveABindingConflictTakesTheAccountSlotAndClosesAgreed`, `TestLiveAppliesInFlightSince`, `TestLiveMigration96RunsDownAndUp`; UI `HaltedReconcile` (the add-on's words).
    - **Mutation-checked (each red):** the sweep raise dropped; the conflicted subject not skipped (it went stale and `deleted_upstream`); `convergeUnbound` ignoring conflicts; the conflict pass clearing its own slot; the account-slot clear removed; the in-flight guard dropped; a name fallback in `partitionByPresence`; `unmanaged` ignoring `subject`; state indexed by name; `readFailure` always unreachable; the drain raise dropped (apply and re-plan, each); the `consequence`/`conflict` decode dropped; the `/subjects` detail decode dropped; the resolution gate dropped; live: the in-flight query without settled-since, and 000096's check without `binding_conflict` (which also showed the static guard read the comment, now fixed to the constraint).
    - Left for 7t-4: rendering the finding (`MergeFindings`), the pending page's cause, and the inventory card's halted line (the inventory result carries `detail` too).
    - *Review round (15 Nov 2026, Opus on `79bf95a4`):*
      - **The drain raises only a binding's conflict.** A blocked create (the name it would be made under is taken, `unboundConflict`) is in no `/subjects` conflicts, so the sweep never closed its finding and `unbound` released nothing. `raiseConflict` now asks the backend's mirror (`getBinding` = `db.GetTargetBinding`) and raises only for a subject with a binding on the target; a create keeps its refused row and the add-on's words and next step. One the mirror lacks the next sweep raises from `/subjects`. Test fixtures split into `numberHeld` (a bound subject) and `nameTaken` (a create): `TestABlockedCreateRaisesNoFindingAndKeepsItsWords`.
      - **A new outcome in a slot is a new question.** `RecordMergeFinding`'s upsert resets `decision`, `decided_by`, `decided_at`, `decision_reason` and `detected_at` when the outcome changes (`CASE WHEN target_merge_findings.outcome = EXCLUDED.outcome ...`): a decided `deleted_upstream` that becomes a `binding_conflict` is undecided and `unbound` resolves it (live `TestLiveANewOutcomeInTheSlotDropsTheOldDecision`).
      - **A conflict closes when the add-on stops listing it**, binding or not: on a whole read `raiseConflicts` closes every standing `binding_conflict` on the target outside this read's conflicts (`db.ClearMergeFindingsOutside`, `COALESCE(decision,'agreed')`, attributed as `ClearMergeFinding`), before the stale loop; never on a truncated read (`TestAWholeReadClosesTheConflictsItNoLongerLists`, live `TestLiveClearMergeFindingsOutside`).
      - **Name-based attribution outside drift, found and fixed.** `services/dormant.go` matched accounts by the mirror's username: now by `subject`, listing the row's own name (`TestADormantAccountIsTheOneTheAddOnAttributes`). `handlers/target_homes.go` matched `/homes` by username: `/homes` rows now carry `subject` from the add-on's one `attribute()` (contract: `profiles/storage/homes.json`, fixture, `openapi.yaml` 500, AUTHORING, CHANGELOG; TrueNAS `readHomes`, 500 `STORE_UNREADABLE` when the bindings cannot be read; backend `HomeUsage.Subject`), matched on it, the row showing the account's current name (`TestHomesCarryTheSubjectTheBindingNames`, `TestHomesWithAnUnreadableBindingAreStoreUnreadable`, `TestTargetHomesSaysEachLimitAndWhereItComesFrom`). The reference add-on offers no `storage.homes`, so it has nothing to change; the kit validates `/homes` against the schema, and no cross-read kit check was added (a home and a subjects row are two reads, and `account_id` is no identity across reads). Checked and not matched by name: `/stats` points carry `subject` already; `activity.get` and the other account operations are asked by subject and resolved by the add-on. Left as designed: the statistics' record text is attributed by name tokens (`services/stats/collect.go` `peopleOn`, design §9.6), since a record is free text with no account key; a stale mirror name there can miss or misname a record (worth a 7t-6 look). `handlers/my_storage.go` and `inventory.go` show the mirror's username as a label only.
      - **The in-flight guard is bounded.** `AppliesInFlightSince(target, since, window)` counts an `in_flight` row only when `started_at >= since - window`, `window` = `addons.ApplyWindow()` (2 × `callTimeout`, one re-plan and one apply, each sent `addonAttempts` times), so a row a crashed drain left never hides a deletion; `readBegan` is `db.DatabaseNow` (`SELECT now()`), the clock `started_at`/`completed_at` are on (live `TestLiveAppliesInFlightSince` with an orphaned row).
      - **`read_refused` is `STORE_UNREADABLE` only** (`addons.CodeStoreUnreadable`); every other failure, `NO_CURRENT_READ` and `TARGET_UNREADABLE` included, stays `target_unreachable`, worded by the page with the system's name and its next step. The Check now line says the reason in words (`whyNotChecked`), never its code, with "Check {system}'s add-on, then check again." for a refused read without the add-on's words, and the second "Nothing was concluded" row no longer repeats a halted pass.
      - **Copy.** "{system} gave no reason." without the HTTP status (§8; guide row fixed). One name path: `addons.SystemNameOf` (and `SystemNameFrom` over a caller's seams), used by `handlers.systemName` and the drain.
      - **`partitionByPresence`** checks `conflicted` before `bound` (`TestAConflictWinsOverARowForTheSameSubject`).
      - **Deploy order.** `/subjects` `conflicts` is now required (`[]` when none, a served copy included; schema, fixtures `subjects_stale.json`/`subjects_cooldown.json`, AUTHORING, CHANGELOG): no minor version or feature tells a pre-7t-2 add-on apart, so this is the marker. A reply without it is not usable (`addons.Subjects`: not current, truncated), so a backend deployed before its add-on halts drift as a stale read instead of reading every binding as deleted. NEXT.md and DEPLOY.md say the add-on goes out with or before the backend, from one commit.
      - **Mutation-checked (each red):** the drain raising for a create; the decision kept across an outcome change (live); the whole-read close dropped; the close run on a truncated read; an empty keep (live, `NULL`); dormant by name; homes by name (handler); `/homes` without `subject` and its store error as `TARGET_UNREADABLE` (add-on); the in-flight age bound dropped (live); the process clock for `readBegan`; `read_refused` for any code; the status back in the no-reason sentence; `bound` before `conflicted`; a reply without `conflicts` left usable; the raw reason on the Check now line (UI).
    - *Round 3 (15 Nov 2026, Opus on `2593bda7`):*
      - **A row is stamped when it is sent.** A claim stamps its whole batch with one `started_at` and the drain sends the rows one by one, so row k went out about k calls later and could fall outside the in-flight window while still able to land (a false `deleted_upstream`). `db.MarkPropagationSending` sets `started_at = now()` (the database's clock) for a row still in flight, at the head of `dispatchEntitlement`, which the operator drain and the runner's revocation and retry claims all send through; a row it cannot stamp (settled meanwhile) is not sent. No migration (`started_at` exists). Tests: `TestEachRowIsStampedAsItIsSent`; live `TestLiveARowSentLateInItsBatchIsStillInFlight` (a row claimed ten minutes before the read and sent during it is covered, one of the same batch still waiting is not, a settled one is not stamped).
      - **The close keeps a subject with an apply in flight.** `raiseConflicts` adds the pass's `inFlight` subjects to what it keeps, so a conflict the drain raised after the add-on read is not closed `agreed` by that read (`TestTheCloseKeepsASubjectWithAnApplyInFlight`).
      - **An outdated add-on has its own state.** `addon_outdated` (`db.UnreconciledAddonOutdated`; Go vocabulary, no column check, so no migration). `addons.SubjectsResult.Outdated` is set by the one test (no `conflicts` on `/subjects`); `readHalt` in drift is the one mapping (read_refused for `STORE_UNREADABLE`, target_unreachable for any other failure, addon_outdated for an outdated reply), used by the sweep and the inventory. The Check now line: "{system}'s add-on is older than this Syndra, so Syndra stopped checking it. Update the add-on from the same release as Syndra, then check again."; `whyNotChecked` (Home): "Its add-on is older than this Syndra, so Syndra stopped checking it. Update the add-on from the same release as Syndra." Copy guide row. Tests: `TestAnOutdatedAddOnStopsThePassAsOutdated`, UI `HaltedReconcile` (the TrueNAS page's Check now line) and `UnvouchedReason`.
      - **`/homes` asks the same test.** The add-on's verdict on `/subjects` is remembered per add-on (`Addon.attribution`), and `addons.Outdated(target)` answers it, reading `/subjects` once (its copy is enough) when nothing has said yet. `ReadHomes` returns `Outdated` (not read) for an outdated add-on, never rows it cannot attribute, and the homes handler answers `read: false` with `reason: addon_outdated`. Test add-ons default to having said (`signedAddon`). Tests: `TestAnAddOnOlderThanAttributionIsOutdatedForSubjectsAndHomes` (both ways, `/subjects` read once and then remembered), `TestHomesFromAnOutdatedAddOnAreNotShownAsAnybodys`.
      - **Mutation-checked (each red):** the per-row stamp dropped (unit) and the stamp not moving `started_at` (live); the in-flight subjects not kept by the close; `addon_outdated` not mapped; `/homes` not asking `Outdated`; the verdict not remembered; the homes reason dropped; the Check now line's outdated sentence (UI).
      - `syndra_test`: fingerprinted (all 59 tables) before and after, identical, at `94|false`; only this slice's live tests were run.
    - *Round 4 (15 Nov 2026, Opus on `86f95bb8`):* the one mapping is now `addons.SubjectsResult.Halt()` (addon_outdated, then read_refused for `STORE_UNREADABLE`, then read_stale for a refusal inside the cooldown, then target_unreachable), used by the sweep, the inventory and `/homes`, and ahead of any cooldown or stale label: the sweep asks it before its cooldown copy branch and sets `retry_after` only for read_stale, the inventory no longer relabels after it (`TestAnOutdatedAddOnStaysOutdatedInsideItsCooldown`, `TestHaltSaysWhatTheReplySaysAheadOfTheCooldown`). `/homes` learns whose through `addons.Attribution`: the verdict remembered per add-on, else one `/subjects` read at a time (`TryLock`) and at most once per the add-on's fresh-read cooldown; while unknown it answers `read: false` with the reason that read gave (`Halt`), never homes read with nothing matched (`TestHomesNeverReadAsNobodysWhenWhoseIsUnknown`). The homes `reason` is typed (`TargetHomes`) and the Accounts card says it through `whyNotChecked` ("Home folders aren't shown. …"; `PeopleOnTargetHomesReason.test.tsx`; copy guide row). Mutation-checked (each red): `Halt` putting the cooldown first, the sweep's cooldown branch before `Halt`, the inventory's read_stale relabel, the fallback read without its cooldown, homes read true while unknown, and the card's reason line (UI).
  - [x] **7t-4 UI for the binding conflict** (design 7t §3, UI). *Done (15 Nov 2026):* `MergeFindings` has a `binding_conflict` branch (outcome in `MergeFinding`): the row is the add-on's `detail` then `next` as written (`conflictReport`, `describe`; a fallback sentence and next step when a record carries none), `ConflictWho` adds the other person of `claimed_twice` by name (`Person`, never the id) and every holder of `ambiguous`, and the holder's number stays in Advanced's Technical detail. The one action is **Stop tracking** (`unbound`, the system page's own `account.release` control) through the existing decision form, whose line says "Nothing on {system} is changed." (the add-on's `next` carries the rest; see the review round below); `keep_ours`/`take_theirs`/`reprovisioned` buttons never render for it. The list now uses the leaving motion (`useLeavingRows`/`LeavingRow`), so a resolved row fades and the gap closes. **Pending page:** `StoppedRow` shows a `target_refused` row's stored `last_error` after "{system} refused it:" (`targetRefusedLine`, `wait-reason.ts`: once, "gave no reason" folded in, no second prefix); for a bound person's conflict (a standing `binding_conflict` finding for that subject on that system) the next step is the link "Decide it under Waiting on you" to the system's page, otherwise the row's own next step (the add-on's next step is already in its words, so it is not repeated). `WithdrawnAccess` already shows `last_error` as stored with no prefix and links to the system's page, so it is unchanged. **Inventory card:** `TargetInventory.detail`; a stopped pass shows `inventoryNotRead` (the add-on's sentence and next step, else `whyNotChecked` and "Read again once that is fixed.") in place of "Nothing outside Syndra.". **Copy:** `plain-language-copy/design.md` §16 rows, `bindingConflict.ts` holds the sentences, `bindingConflictCopyGuide.test.ts` keeps guide and code in step. **Tests:** `MergeFindingsBindingConflict.test.tsx` (each `reason`, the only action, the consequence line, the resolution flow with the row leaving, the next step with no words, Advanced-only ids), pending `page.test.tsx` (stored reason once, no double prefix, link only for the bound person's own conflict), `InventoryFreshness.test.tsx` (stopped pass with and without the add-on's words, a concluded pass says nothing). **Review round (15 Nov 2026, Opus on #193):** (1) the inventory's halted answer is 503 with the inventory as its body and the reader threw it away, so the card never had `detail`: `readInventory` (`useTargets.ts`) takes a 503 body that says `halted: true` (kept on `ApiError.body`) as data, for the load and for Read again (`useReadAgain`'s `read`), and any other 503, a null body, a 429's cooldown and every other failure still throw; tests go through the real reader and the real `ApiError` shapes (`useTargetInventory.test.tsx`, `InventoryPressDuringLoad.test.tsx`). (2) A change the add-on applied that Syndra cannot attribute stored raw ids, "subject", a Go error and "Do not retry" as `last_error`: it is now a plain sentence naming no system (`bindingConflictReason`, ids and error in the log) under its own `wait_reason` `binding_conflict` (migration 000097, reversible: down maps it to `target_refused`; `runnerSends` holds withdrawals for both), and the pages switch on that, never on wording (`stoppedLine`); a refusal with no words stores none; the pending page also asks `health.binding_conflicts` (matched by the row's own `outbox_id`) beside the merge findings. (3) `adoptInstead` and the repeated next step are gone; the form's line says only what stopping does to the system, and adoptability is never inferred from `reason`. (4) The pending conflict match is by person for a finding (no join exists from an outbox row to it) and by row for a recorded disagreement; while either is read the seat says "Looking up what to do next.". (5) The link is the system page's `#waiting` region (already there). (6) "Finish: stop tracking it". (7) `inventoryNotRead` names the system; an unknown reason is a general sentence, never its code; `CONFLICT_NEXT_FALLBACK` says what happens; the copy guard compares whole sentences, and the backend's sentence against the Go source. (8) Spec and ADR corrections. (9) Withdrawn access and Pending changes share `stoppedLine` (the revocation list now carries `wait_reason`). **Round 3 (15 Nov 2026, Opus on `053ec9e2`):** the two text matches (`applied the change`, `gave no reason.`) are gone for the wait reason above; the inventory discriminator is `halted === true`; the guard covers Zitadel's refusal line, Finish: stop tracking it, Waiting on you is on {system}'s page., Decide it under Waiting on you, and the backend's whole sentence against `addon.go`. Still read from text: the out-of-retries line (`last_error` starting `out of retries`), since a failed row with no wait reason has no other marker. **Mutation-checked in the round (each red):** `readInventory` dropping a 503 body; any 503 taken as an inventory; the health source dropped from the conflict test; the health match by person instead of row; the loading line removed; the applied sentence prefixed with "refused it"; Withdrawn access printing the bare reason.
    **Mutation-checked (each red):** the `binding_conflict` branch in `describe`; the DecisionForm conflict button (keep/take back); the `bound_to` person line; the holders line; the pending refusal using the stored reason; the conflict link (off, and for anyone); the inventory halted line and its empty-list suppression.
  - [x] **7t-5 Spec and ADR.** *Done (15 Nov 2026):* design.md 9.4b is rewritten as "One account identity" (module, rule, wire, backend half, finding lifecycle, legacy policy B confirmed by the owner 17 Nov 2026, deploy order and `addon_outdated`), replacing the per-slice patches; `docs/adr/007-account-id-is-a-key-within-a-read.md` records D1-D6 with RFC 7643 §3.1 and the `identity.go:7-12` incident; NEXT.md promotes the cutover pre-check to a must and records that 7t-4 landed.
  - [x] **7t-6 The backend mirror's unique indexes on a reused number** (added from the 7t-3 review, 15 Nov 2026). `target_account_bindings` holds unique indexes on `(target, username)` (`backend/db/migrations/000032_target_account_bindings.up.sql:55`), `(target, account_uid)` (`:61`) and `(target, account_id)` (`000088_addon_contract_2.up.sql:37`). When TrueNAS hands a freed uid to a new account and person B's apply rightly lands on it, `recordBinding` (`backend/internal/services/propagation/addon.go:211`, `db.RecordTargetBinding` `backend/internal/db/target_bindings.go:56-95`) fails on A's stale mirror row holding that number, and the drain settles B's correct apply as a conflict (`recordConflict` → `db.BindingHolder` `binding_conflicts.go:208`, `target_binding_conflicts`, "Do not retry"). **Recommended:** the mirror is not an identity (D3), so drop the `account_id`/`account_uid` unique indexes (keep the non-unique lookup) and keep only `(target, subject_id)`; on a write, the add-on's answer is authority, so a mirror row of another subject on the same name or number is the stale one and is cleared in the same transaction (logged, audited), not a conflict. `target_binding_conflicts` then only arises from two live subjects the add-on itself reports, which `/subjects` already says as `claimed_twice`; retire it into the `binding_conflict` finding. Also look at `services/stats/collect.go` `peopleOn` (record text attributed by the mirror's username). Two minor items from the 7t-3 round-4 review, left on purpose: a rollback of the add-on to one older than attribution is remembered as `attributionSaid` until the next `/subjects` read flips it (self-correcting; the sweep's and inventory's own reads are what flip it); and the `not_recorded` audit nit the review raised is pre-existing, not 7t-3's. **Also deferred from 7t-4/7t-5 (15 Nov 2026):** (a) a revocation blocked by a binding conflict shows `last_error` on Withdrawn access with a link to the system's page, not the "Decide it under Waiting on you" link, since that row carries no wait reason (the outbox API would need to send it); (b) `PLAN_STALE` settles through `markFailed` (no wait reason) with a code-prefixed sentence (`refusalReason`), so the pages say only "{system} refused it, so Syndra stopped sending it." and never its sentence, which is still not plain words; (c) owner questions still open: Q1 (`account_id` stays the uid), Q2 (legacy policy B), Q4 (`subject` and `conflicts` core), Q5 (only `unbound`), Q6 (the drain raises the finding too).
    - *Done (15 Nov 2026, branch `feat/addon-contract-7t6`):* one PR, not split into 7t-6a/b (both halves rewrite the same drain branch and the same RecordTargetBinding).
      **(1) The mirror is not an identity.** Migration **000098** drops `idx_target_account_bindings_username`, `_uid` and `_account_id`; only the primary key `(target, subject_id)` stays unique, and no lookup index replaces them (the only read by name or number is the clear below, on one target's few hundred rows, which the key's leading `target` already narrows). `db.RecordTargetBinding` clears another subject's row on the same account (`clearStale`: the same id where both rows carry one, the same name only where either has none; a same-name row on another id is another account whose mirrored name is stale and is kept), logs it (`[TARGETS]`) and audits each clear against the cleared subject (`binding.<target>.stale_cleared`, actor = the writer, details `username`, `account_id`, `now_bound_to`; worded in `audit-vocabulary.ts`), then upserts, all in one transaction. `ErrBindingConflict` is gone. **Down:** the three unique indexes come back after duplicates are resolved deterministically by the up's own rule (per key in turn, the row last seen kept, then `bound_at`, then `subject_id`; each cleared row audited by `system:migration`), chosen over refusing because a refusal blocks a rollback in the hour it is needed, over rows the add-on's next answer re-records.
      **(2) `target_binding_conflicts` retired.** Every reader and writer is gone: `db.RecordBindingConflict`, `StandingBindingConflicts`, `ResolveBindingConflict`, `BindingHolder` (`binding_conflicts.go` deleted), the drain's `recordConflict`/`bindingConflictReason` and its `ErrBindingConflict` branch (a successful apply now always settles `applied`; a mirror write failure is logged, as a transient one always was), `health.binding_conflicts`, `POST /targets/{target}/binding-conflicts/{id}/resolve` (handler, route, `useResolveBindingConflict`), the system page's "Two records disagree" notice and owner dialog, the connected-systems count, and the pending page's `outbox_id` match. The UI `MergeFindings` repair row ("Finish: stop tracking it") was checked: it is the merge finding's own `unbound` repair, never read the table, and is unchanged. Prod runs contract 1 and has the table (000039 is on main), so it is **kept** for one release, unread and unwritten, for a rollback to the contract-1 backend (migrations never run down there, 1.10), and dropped with `account_uid` (1.7b, which now names it); an open prod row is resolved in the contract-1 UI before the cutover (NEXT.md). Not migrated into findings: the sweep raises from the add-on's own `conflicts` whatever still stands and closes any `binding_conflict` the add-on does not report, so a converted row would be closed on the first read or duplicate the sweep's.
      **(3) Stats `peopleOn`:** kept on the mirror, documented (design §9.4b end): the collector has no `/subjects` read, and adding one would spend the fresh-read cooldown hourly or answer from a kept copy older than the mirror (a subject bound since would read unbound and its points be dropped for good). The misname is fixed at its cause instead: a reused number or name moves to its new holder on the add-on's next answer (`TestLiveAReusedNumberNamesItsNewHolder`; with the clear removed the record named both people).
      **(4) A revocation blocked by a binding conflict.** The drain settles a refusal or blocked re-plan carrying a binding's conflict as `wait_reason` **`binding_conflict`** (`markBindingConflict`; 000097's value, its meaning narrowed in 000099's comment: the old "applied but unattributed" cannot arise since 000098). Which conflicts are a binding's is the contract's (`bindingsConflict`, `addons.ConflictNameHeld`): `number_held`, `ambiguous` and `claimed_twice` always, so a person whose mirror row was just cleared still gets it; `name_held` when the mirror records a binding. `raiseConflict` returns that verdict. The UI: `decideAt(row, stands)` (`lib/wait-reason.ts`) gives "Decide it under Waiting on you" (`#waiting`) on Pending changes and now on Withdrawn access (`unconfirmed-revocations` already sends `wait_reason`, 7t-4) while a `binding_conflict` finding stands for that person there (fix round, below). `stoppedLine` words `binding_conflict` as the refusal it is.
      **(5) `PLAN_STALE`:** settles `wait_reason` **`plan_stale`** (migration **000099**, which holds nothing, like the old plain failure; down sets it back to NULL), `last_error` in plain words, no code, no system, with what happens next (fix round, below). Pages: "{system} refused it: their account there changed after this change was approved, so Syndra stopped sending it." Copy guide §16 row.
      Tests: `TestLiveAReusedNumberOrNameMovesTheMirrorAndIsAudited`, `TestLiveMigration98DownResolvesDuplicatesAndUpDropsTheIndexes` (duplicates on all three keys; the retired table's row untouched both ways), `TestLiveMigration99RunsDownAndUp`, `TestLiveAReusedNumberNamesItsNewHolder`, `TestTheMirrorIsNotAnIdentity`, `TestABindingWriteClearsTheStaleRowInItsTransaction`, `TestMigration99_AllowsPlanStaleAndDownClearsIt`, `TestARevocationBlockedOnANumberHeldConflictIsABindingConflictWithNoMirrorRow`, `TestANameHeldConflictIsABindingsOnlyWhenTheMirrorSaysSo`, `TestAStalePlanIsSettledAsStaleInPlainWords`; UI: pending "sends a binding conflict's row to Waiting on you by its wait reason alone", "says a stale plan in plain words, with no code", Withdrawn access "sends a revocation blocked by a binding conflict to Waiting on you", the copy-guide guard's two new cases. Opus review (15 Nov 2026) on `c7086b13`, fixed: the clear matched by name alone, so a renamed account's person (the add-on following them by number) lost their mirror row to whoever took the freed name, and dropped out of the sweep until their next apply; now the name is the key only where a side has no id. And the per-target advisory lock inverted `ResolveBindingCall`'s order (row `FOR UPDATE`, then this write): removed, since the race it guarded needs the add-on to give one account to two people. Left on purpose: a `name_held` conflict for a bound subject whose mirror row is missing settles `target_refused` (the sweep still raises the finding under Waiting on you; the row keeps the add-on's words and next step); `binding_conflict` rows written under 000097's old meaning are not rewritten (000097 never reached production, and no page could tell them from the new by anything but wording); a write that reports no id keeps the subject's recorded id, which another row could also hold (contract 2 always reports one). Mutation-checked (each red): the name always the key (live: B's row on another id is cleared), the stale clear matching nothing (live: A's row stays, and the stats gap names both people), the audit loop skipped (live), the down's name pass partitioned per subject (live: the unique index refuses to build), `bindingsConflict` ignoring the reason, the conflict settle as `target_refused`, the stale settle as a plain failure, `decideAt` returning null, `stoppedLine` without its `binding_conflict` arm, the audit action left unworded (`audit-vocabulary-coverage`, whose joined-action scan now covers the `binding` family). `syndra_test`: found at 94 (clean), every table fingerprinted (count and md5 of rows) before and after, identical, left at 94.
    - *Fix round (16 Nov 2026, independent Opus review of `cb3bf35f`, "mergeable"):*
      **(1) The rollback depends on the indexes.** The contract-1 backend's only guard against two people on one account is the unique violation, and the forward deploy after a rollback re-runs 000088, whose unique index fails on duplicates written meanwhile (dirty, backend down). `scripts/rollback-mirror-indexes.sh` runs 000098's down SQL byte for byte (`TestTheRollbackScriptIsMigration98Down` holds them equal), dry run by default (duplicates per key, the indexes present, a rehearsal rolled back), `--apply` in one transaction; idempotent and audited (`TestLiveTheRollbackScriptIsIdempotentAndAudited`, the SQL run twice on the real engine over duplicates). DEPLOY.md, Rollback: it MUST run before a rollback to a pre-098 backend (copied to `/usr/local/bin` first, since the rollback checks out a commit without it) and again before any forward deploy that re-runs 000088; the version reset there now reads `WHERE version >= 88` (it said `= 88`, which matches nothing once 89+ have run). 000098's up comment, NEXT.md and the scripts README say the same. The shell wrapper is not run here (no Docker on this Mac); its SQL is.
      **(2) The link checks that something still waits.** `decideAt(row, stands)` links only while a `binding_conflict` finding stands for that person on that system; `useStandingBindingConflicts` reads each system's findings once per page (one `useQueries` over `mergeFindingsQuery`, the same key as `useMergeFindings`), and while they are read the seat says "Looking up what to do next." (back in the copy guide). Tests: the pending page's standing/decided/looking cases (its mock records that the hook is called once for the list, with the binding conflicts' systems), Withdrawn access's standing and decided cases, `useStandingBindingConflicts.test.tsx` (two systems, five rows, two reads).
      **(3) `peopleOn` naming two people: not changed, by evidence.** `named_subjects` has one reader, the member view's `namesOther`, which leaves out any record naming anyone but the viewer. Naming both A and B already leaves it out of every member's view (theirs included) and counts it; that is the closed direction. "Credit neither" would empty the list and show the record to every member: open. Tests pin it (`TestANameTwoRecordsCarryNamesBoth`, `TestARecordNamingTwoPeopleIsLeftOutForEveryMember`); design §9.4b now says this instead of "fixed".
      **(4) A late adoption settle.** `ResolveBindingCall` treats another subject's copy on the recorded account, written or seen at or after the call, as "decided since": half is not run, so that copy is neither cleared nor joined by a second subject (`TestLiveALateAdoptionSettleNeverClearsANewerHolder`, which also shows an older copy is cleared). Read, not locked, so no second lock order.
      **(5)** Waiting on you's lede no longer names the retired disagreement; copy guide row and guard.
      **(6) A stale plan's next step is what the code does.** The person's access still says the change, so the sweep's next pass queues it again (fast-forward, `merge.Convergeable`) unless the target changed the same field (a finding); a withdrawal then sends on its own (`db.sendsOnItsOwn`), anything else waits in Pending changes. `last_error` is `planStaleWithdrawal` or `planStaleChange` by `intent.WithdrawsOnly`; the page's step is `planStaleStep` by the row's `sends_on_its_own`. "Give it again from their page" and "Make the change again" are gone.
      Mutation-checked (each red): `decideAt` ignoring `stands` (pending, Withdrawn access, the guard), `planStaleStep` ignoring `sends_on_its_own`, the drain's stale sentence ignoring `WithdrawsOnly`, `names` crediting neither on two matches, `namesOther` sparing the viewer's own name, `ResolveBindingCall` without the newer-holder check (live), and the rollback script's SQL differing from the down by a byte.
  - [x] **7t-7 Three more ways to settle a binding conflict** (owner ruling on Q5, 17 Nov 2026: Stop tracking by default, plus adopt when adoptable, keep-for-one on `claimed_twice`, and Check again). *Done (17 Nov 2026, branch `feat/addon-contract-7t7`):*
    - **Adopt the holder ("It's theirs: adopt it").** Add-on: a recorded binding's own `name_held`/`number_held` conflict (`BindingConflict.recorded`, set in `boundAccount`) is `adoptable` by `account.adopt`'s own rule, judged in `attributeOver` over the same read (the holder bound to nobody, the add-on's own account known and not it); its `next` then says "Adopt {name} if it is theirs. If it is not, …" (`words`; the non-adoptable words unchanged). `adoptAccount` moves a bound subject's binding to the holder named (the one adoption, no second path), judged again over the one read it writes from (the subject's verdict must still be that conflict, with that holder, adoptable), and refuses a stale one with 409 in words ("… can no longer be adopted for this person: TrueNAS changed since it was checked. Check again …"); a fresh binding, nothing on the account touched. New optional param `account_id` (registry, `policy.go`): the account the operator was shown, an `If-Match`-style precondition (RFC 9110 §13.1.1), refused with 409 when the name now belongs to another account; a non-string is 400. Torn reads stay 503 `TARGET_CHANGED` (resolveLive). Contract: `binding_conflict.json` `adoptable`, AUTHORING (the one create, the account operations), CHANGELOG; no fixture changed. Backend: resolution `adopted`, accepted by `resolutionFits` only when the stored report is `adoptable` with a holder and the reason is `name_held` or `number_held`; dispatched by `adoptOnTarget` (`account.adopt` through `svcDispatchOperation`, params `username` + `account_id`), this side's half `recordAdoptedBinding` with the merge base forgotten first; a refusal is 409 `ADOPTION_REFUSED` in the add-on's words, no answer `ADOPTION_NOT_CONFIRMED`; both give the reservation back.
    - **Keep it for one of two ("Keep it for {person}").** Resolution `kept`, accepted only for `claimed_twice` with `bound_to` naming the other person: `account.release` of the OTHER subject through `releaseOnTarget` (never this one), their mirror row forgotten and their standing finding settled `unbound`; this row stays decided until the add-on reports one binding. Keeping it for the other person is this person's `unbound`. Either answer reserves the other person's row with the same answer from its side before the add-on is called (`reservePair`), so the two rows can never release both people; an opposite answer already there refuses with the usual 409 naming who and why.
    - **Check again.** The Check now pass (`useReconcileTarget`, `POST /targets/{t}/reconcile`), no new reader; not a resolution, no reason needed. The finding closes itself when the add-on stops reporting it (the sweep's `ClearMergeFindingsOutside`). Inside the add-on's cooldown the page shows the shared `cooldownLine` and keeps the button off until its count ends; "Checked just now: {system} still reports this." only once a fresh pass refreshed the row (`last_seen_at` moved); a stopped pass in its words with a next step.
    - **Migration 000100** (reversible): the resolution check allows `adopted` and `kept` (a row closes carrying its decision). Down withdraws a standing one and reads a settled one as `agreed`, the decision column keeping what was chosen.
    - **Audit:** every resolution `merge_finding.decided` (actor, person, finding, target, outcome, resolution, reason; also on a half-done repair), every check `target.checked` (actor, outcome); UI audit vocabulary rows. No plan-gate surface: both operations are confirmed one-shots, as from the inventory.
    - **UI:** the decision form shows one choice at a time (the `Radio` primitive), safest first and preselected (`conflictChoices`): Stop tracking, adopt (only when adoptable), Check again; for `claimed_twice` both people by name, "Keep it for {other}" (default) and "Keep it for {this}"; for `ambiguous` Check again first. The sentence beside the one accent button says what happens on the system and in Syndra; ids stay in Advanced's Technical detail. A decided adopted/kept row says who and "closes when Syndra next checks {system}". Copy guide §16 rows and `bindingConflictCopyGuide.test.ts` (also the add-on's adoptable and stale sentences against the Go source).
    - **Tests:** add-on `adopt_move_test.go` (`TestAdoptingTheHolderMovesTheBindingAndTouchesNoAccount`, `TestAStaleAdoptableIsRefusedWhenItRuns` ×3, `TestAMoveIsJudgedAgainOverTheReadItIsWrittenFrom`, `TestTheHolderIsNeverAdoptableWhenItIsTheAddOnsOwn`, `TestAnAccountIDThatIsNotAStringIsABadRequest`), the identity table's adopt and `/subjects` cases, `TestAReusedUsernameIsNotThisSubjectsAccount` (adoptable, and not when another person holds it); handlers `merge_findings_conflict_actions_test.go` (the gate table, the one adoption, a refused adoption, kept releases the other never this one, Stop tracking holds the other row as kept, the other row's opposite answer refuses before the target, the check is audited); db `TestMigration100_AllowsEveryDecisionAndDownMapsTheNewOnes`, live `TestLiveMigration100RunsDownAndUp` (and the binding-conflict live tests re-run under 000100); UI `MergeFindingsConflictAnswers.test.tsx` (11). `syndra_test` found at `94|f`, migrated back to 94 after, fingerprint (schema, every table's rows, the target rows) identical. Mutation-checked (each red): the move's re-check, the shown-id precondition, adoptable ignoring bound / self / self-unknown, the move case removed, adoptable never set; the gate's adoptable / reason / claimed_twice checks, kept releasing or forgetting this person, no pair reservation, the adoption not recorded or sent without `account_id`, either audit missing, a refused adoption keeping its reservation; the UI's adoptable, ambiguous order, keep without the other person, keep-for-this sending `unbound`, Check again needing a reason, the cooldown not waited, "still reports" without a fresh sighting, the decided row's tail.
    - **Known limit:** a `kept`/`adopted` whose backend half fails after the add-on acted answers 202 `REPAIR_NEEDED` and can be finished by choosing it again through the API; the page has no Finish control for it (only `unbound`'s), and the sweep closes the row anyway. A `kept` half that never finishes leaves the other person's mirror row, which the next sweep reads as gone (`deleted_upstream`, whose own Stop managing it settles it).
    - *Review round (18 Nov 2026, Opus on #197 `fc761242`; the move never binds a person to another person's account held):* **(1)** the merge-base clear is in the one shared step, `recordAdoptedBinding`: whenever the person's recorded account changes (`sameAccount`: by id when both sides have one, else by name; an unreadable copy counts as moved), the old account's base and landed record go first, on every path (the conflict's resolution, the inventory's adopt, a `/calls` settle); removed from the resolution path. **(2)** `resolutionFits` requires the holder's `account_id` for `adopted`; the add-on refuses a MOVE without `account_id` (409, "Moving this person to {name} needs the account you were shown …"); the inventory sends the `account_id` its row showed (`adoptRequest.AccountID`, `adoptParams`, UI `useAdoptAccount`); `binding_conflict.json` already required `account_id` for every reason but `ambiguous` (its `else` branch), so no schema or fixture changed. **(3)** a decided `adopted`/`kept` row has a Finish control (`finishLabel`, `FINISH_AGAIN_NOTE`) sending the same choice on the repair route, which re-runs the call (an adoption's repeat answers its account id) and this side's half, so Syndra's copy ends right. **(4)** `account.release` takes an optional `account_id` precondition (registry, `policy.go`, AUTHORING MUST; TrueNAS `bindingNames`: the binding's own real uid, needing no NAS, or for a contract-1 binding the account the module finds; a binding that names none has nothing to drop): 409 and nothing released when the binding names another account. Syndra sends it from `kept` (the account both claimed), Stop tracking (the claimed account, else the backend's copy, `releasedAccount`), and the system page's release (`boundAccountID`); a refusal is 409 `RELEASE_REFUSED` in the add-on's words and gives every reservation back. **(5)** the reference serves `account.adopt` and `account.release` with their preconditions (`binding`); kit checks `adopt.shown`, `adopt.not-shown`, `release.not-shown` (`preconditions`, `-mutate` only, after the operations step, each write after a plan names the kit's account; a 422 "cannot adopt yet" skips; a release that ignores its precondition is bound back at once, else the kit stops writing), failed by the reference defects `adopt-refuses-repeat`, `adopt-ignores-account-id`, `release-ignores-account-id`; the TrueNAS kit run passes all three; `defects.go`'s old "Release the binding, then adopt" wording is gone. **(6)** the failure path that gives the other row's reservation back is tested (`TestAStaleKeepIsRefusedAndGivesBothReservationsBack`). **(7)** copy: adopting says what they get (everything already on it, and its password; that Syndra now manages it; and for `name_held` that their old account, if still there under another name, stays unmanaged) — "home folder" is the storage kind's word, kept out of the generic page by the lint rule; keeping it says their next change sets up an account or, if their name is taken, stops and asks in Pending changes (the drain settles that create's `name_held` as a refusal there); Check again says "Checked. Updating…" until the row is read again; the cooldown is still shown only after a press, since no read this page holds carries it (the inventory's refusal is an error, and reach is a different read). **(8)** a holder two people's conflicts name is adoptable for neither (`sharedHolder`), with words saying why and what to do. Tests: add-on `TestAMoveWithoutTheShownAccountIsRefused`, `TestAHolderTwoConflictsNameIsAdoptableForNeither`, `TestAReleaseForAnAccountTheBindingNoLongerNamesIsRefused`, `TestAReleasePreconditionOnALegacyBindingAsksTheModule`, the identity table's move (refused by name alone, made with the id); backend `merge_findings_fix_round_test.go` (all three adoption paths moved / same / first, the gate, every release's account, the stale keep, the system page's release); UI the Finish control, the Updating line, adopt's name_held sentence, the inventory's account id; kit defect table and the TrueNAS kit run. Mutation-checked (each red): the base clear removed or ignoring the id; the gate without the shown account; the inventory (backend and UI) sending none; the add-on moving without it; release ignoring its precondition, or the module for a legacy binding; kept / Stop tracking / claimed / system page sending no or the wrong account; a refused release read as unconfirmed; the pair's reservation kept on failure; the shared holder ignored; Finish missing or sending another choice; the Updating line; the reference adopt's precondition (clean kit run); the kit's release and adopt.not-shown checks not judged. Survived and left: disabling only adopt.shown's outcome assertion (its status check still fails the defect; no defect answers 200 with another id). No migration or db change, so no live run this round; `syndra_test` untouched.
  - [x] **7t-8 Three low findings from the 7t-7 check, fixed at the root** (Opus on #197). *Done (18 Nov 2026, branch `feat/addon-contract-7t8`):*
    - **Contract-1 release needs no NAS.** `releaseAccount` checks `account_id` only when the binding's own uid is a real account id (`ownsUID`); a legacy binding (uid may be a record key) is released unchecked and the reply says so ("released without checking which account it named"). `bindingNames` (the live read, which also refused a name recreated with a new uid) is deleted. AUTHORING says the check MUST need no read. Tests: `TestAReleaseOfALegacyBindingNeverNeedsTheNAS` (NAS unreachable; name recreated with a new uid). Mutation: legacy branch refusing instead of releasing, red.
    - **System page release sends no mirror id.** That page lists people the add-on's `/subjects` attributes no account to, so the only id Syndra holds is its mirror's, which only an apply or adopt refreshes; sending it could repeat a 409 that "Check again" never cleared. `handleReleaseBinding` now sends no `account_id` (the add-on releases what it binds; the param is optional). Chosen over refreshing the mirror in Check now: a person with no `/subjects` row has nothing to refresh from, so only dropping the precondition removes the loop, and the findings' Stop tracking still sends the add-on's own id. Test `TestTheSystemPagesReleaseSendsNoAccountFromTheMirror`; mutation (sending `boundAccountID` again), red.
    - **Keep it for X checks X.** `callTarget` for `kept` (first run and Finish alike) refuses with 409 `RELEASE_REFUSED` when Syndra's own row for the person it is kept for is gone, in words with "Check again"; reservations given back, nothing released. Only presence is judged (a mirror id can lag, and a refusal Check again cannot clear is the wedge fixed above). Test `TestKeepingItForSomeoneNoLongerTrackedIsRefused` (first run and Finish); mutation (check disabled), red.
    - *Round 2 (18 Nov 2026, Opus on `26d44d26`):* **(1)** a system page left open could release a person given an account since (an adoption under another name updates the row in place, `bound_at` kept), dropping the new binding and stranding its account. `db.TargetBinding.Version()` is a strong validator (RFC 9110 §8.8.3: a hash of `bound_at`, name and account id; not `last_seen_at`, which a sweep moves). `drift.StaleBinding.Version` carries it to the page; `useReleaseBinding` sends `version` back (If-Match, §13.1.1); `handleReleaseBinding` refuses with 409 `RELEASE_REFUSED` "This person's account changed since this page was read. Check again." and releases nothing when Syndra's own record differs (no NAS read; no record means already released; no version sent means no check). An id from `/subjects` is never sent. Tests `TestAReleaseFromAStalePageIsRefusedAndAFreshOneReleases` (fresh; name only; id only; re-bound; sweep-only touch; already released; no version), the drift stale row's version, UI `lets a stale binding go` sends the version it read. Mutation (each red): check disabled; name left out of the version; id left out; UI sending none. **(2)** A Finish on a keep refused because the kept-for person is gone now says "Nothing more to do here: Syndra closes this when it next checks" (a decided row refuses any other choice, so "choose from what it shows" was untrue); the first press keeps "Check again". Mutation (Finish wording branch off), red.

- [x] 7s What has been used, polished (A-23, settled by the owner 17 Nov 2026: Q1 faint baseline and a tick, Q2 one fold closed, Q3 the stat face, Q4 one meter, O1 no switch, O2 one link to Insights, O3/O5 not now, R1 the home month under its meter, R2 retention in Advanced). Built from `design_handoff_syndra/reconcile-2026-10/from-design/A-23-what-has-been-used.md` §3–§8 and its six boards (committed with the build). No backend change.
  *Done (17 Nov 2026, slice 7s):* `lib/usage.ts`: `plot` has a fourth day, `before` (`"kept"` before `kept_from`, `"began"` before the record), never hatched and never a zero, and returns `recordFrom`; `beforeRecord`, `change`, `seriesName`; `Limits` moved here. `components/usage/Band.tsx` (was `Spark.tsx`): 40px, width measured (`ResizeObserver`, 320 fallback), columns for a counter (45% of a step, a real 0 a 1.5px stub, the edge columns at the band's edges), line and area for a level, the faint pre-record baseline, the 6px tick, the limit line and its word, the read column on `--tint-3`, one focusable image read by hover, focus and ← →, a day that gained a value tinting in on a later read (`stillMotion()` still). `MetricBlock.tsx`: one `Row` (the 200 · band · 240 ledger from `@tablet`, the card's own width, `--container-tablet` in `globals.css`; stacked name, figure, band under it), the figure in the stat face, the label on its own line or in the name seat, one `Axis` with one `Crossfade` readout per section; `Gaps`, `Collected`, the meter and `limits` removed. `UsageCard.tsx`: one `Fold` per card (gap lines merged across metrics, the gap before the record never listed, the dropped lines per metric in `AdvancedOnly`), one `End` row (the step; on the totals card `ArrowLink` "Open Usage" and Last counted from the newest `collected.at`; Counting began; a person's Kept for only), `ViewUsage` leaving out `meteredMetric`, and `HomeMonth`, drawn in `MyStorage` `PlaceBlock` and `PersonStorage` `HomeUse` under the `Meter`. `kinds/limits.ts` `meteredMetric` over `storageLimitField`. `TargetStats` is the card; `TargetOverview` wraps `StatsRetention` in `AdvancedOnly`. `InsightsUsage` names each system's first section `usage-{target}` and glides there on landing (`insightsUsageHref`). Copy guide §15 and `usageCopyGuide.test.ts`: the §6 rows; "of {limit}", "no limit", the unread-limit sentence and the card-wide dropped line retired with the usage card's meter.
  **Verified in code (the spec's three open points):** (1) 7r marks the buffer gap only in the add-on (`statsMeta.Began`, reason `reasonBegan`, free text); the contract gives a gap no kind, so the UI cannot read it "by its reason code". `beforeRecord` finds it by shape, as the contract's buffer rule makes it (AUTHORING, the buffer: a gap from the day `since` falls in to the first day held, and Syndra's first `since` is now − `stats_buffer`): the earliest gap, whole metric, starting before the window's first day, ending by today, nothing held before its end. Known ceiling (in the code): on a level, a whole-metric outage crossing the window's first day that is the earliest gap the 12-month read returns would read as before the record. A gap kind in the contract would make it exact (owner). (2) Offline: the add-on keeps collecting on its own clock; on Syndra's next read from its watermark a counter's days are back-filled from the NAS audit log (purged days a gap), and a level's missed days come back as a gap ("The NAS could not be read that day", `putLevels`), since a level cannot be read after the fact. Until then the days after `counted_through` are drawn not counted yet, as the board does. (3) Insights, Usage had only the tab (`?tab=usage`); a `#usage-{target}` anchor per system is added, with the landing glide.
  **Decisions:** the totals step says "Who used each one is in Insights." and not the board's "each share": "share" is the storage kind's word, which the lint guard keeps out of `components/usage` (owner to confirm). A counter's missing final day stays a real 0, the backend's sparse-days contract (`useStats.ts`): only gaps, days before the record and days not yet counted are never filled. The figure is a block with a real space between value and words (the board's refined CSS), not the spec table's inline-flex.
  Tests: `UsageCard.test.tsx` (every row mounts its band; one grid row and the stat face; the label in the name seat; one axis and one readout per section, hover and ← →; edge columns inside the band; change up, down, unchanged; one fold, closed, merged; hatch and break; the nine states; the end row per card; Home folder left out on a storage view; `HomeMonth`'s limit line and word, none at 0 or unread; the Insights link and Last counted on the totals card only; the dropped lines in Advanced only; tint-in on a later read and none under reduced motion), `usage.test.ts` (`before`, `beforeRecord` including an in-window outage on a quiet counter, `change`, `seriesName`), `MyStorage.test.tsx` and `PersonStorage.test.tsx` (the month under the meter), `TargetOverview.test.tsx` (retention in Advanced only, last card), `InsightsUsage.test.tsx` (the anchor), `Meter.test.tsx` (no statistic draws a meter), `chartTokens.test.ts` (the band reads the hatch token). Mutation-checked (each red): the band dropped from a row, the grid columns, the stat face, the home month removed from each storage card, `ViewUsage` not leaving the home metric out, the link on every card, the dropped lines outside `AdvancedOnly`, the retention card outside `AdvancedOnly`, `beforeRecord` without the window guard, tint-in under reduced motion, a §15 sentence changed, and a day before the record drawn as a gap.
  *Fix round (18 Nov 2026, after the Opus review of PR 196):*
  - **Every gap says its kind** (the owner's ruling of 18 Nov 2026, "label each gap", which replaces both the UI's shape heuristic and a Syndra-side `record_from`, built and removed in this round). Contract 2 (unshipped): `stats.json` gap `kind` required, enum `began` / `unread` / `dropped` / `unobserved`; fixtures `stats.json` (`unread`) and `stats_storage.json` (`unobserved`), and their copies in AUTHORING; AUTHORING's MUST with the rule for each kind; `openapi.yaml`; CHANGELOG; design.md §9. TrueNAS: `gapKinds` pairs each fixed reason with one kind (`reasonBuffer` and `reasonPurged` dropped, `reasonNAS`, `reasonDark`, `reasonBehind` and `reasonOverfull` unread, `reasonAudited` unobserved), `gapOf` makes every fixed-reason gap, the buffer gap is `began` exactly when `statsMeta.Began` (7r) says so, and a gap stored before kinds existed takes its reason's kind when served. Reference: `unread` (print queue, xl-1's log) and `dropped` (the buffer; it reads its log back over the whole buffer, so it never has a `began` window). Kit (`stats.gaps`): every gap's kind is the enum's, and no point of the metric starts before a `began` gap's end; defect `stats-gaps-all-began` fails it. Backend: `addons.StatGap.Kind` (decoded by exact name), a gap with no kind or an unknown one dropped as `bad_value`; migration 000101 adds `addon_metric_gaps.kind` (NOT NULL DEFAULT 'unread', CHECK the enum) and gives every stored gap the kind its reason was written for (the TrueNAS and reference sentences; anything else `unread`); `db.StatsGaps` reads each metric's whole-metric `began` gap however old; the view names it `began` (`statsMetricView.Began`) and lists gaps with their `kind`. UI: `beforeRecord` deleted; `plot` draws a day before the view's `began`, or inside a `began` gap on the series, as before the record (faint baseline and tick, never hatched, never zero-filled); every other kind is hatched and listed in the fold with its reason.
  - **The home month is said once**: `drawsHomeMonth(view, operator)` (`kinds/storage/limits.ts`, with `drawsHome` moved there) is the one answer the storage cards and the usage card ask (`meteredMetric(view, operator)`), so where the member's card does not draw it (unread, places unreadable, opens nothing, no home place) or the person page's does not (no account, no answer, no home drawn) the usage card keeps "Home folder used".
  - **Insights only where there is something there**: the totals card's step and link appear only when a metric is per person; else "Nothing for you to do."
  - **Copy**: the totals step is the kind's (`usageStep`; storage supplies "Nothing for you to do. Who used each share is in Insights.", the owner's ruling of 18 Nov 2026, from `kinds/storage/captions.ts`, the one place storage words may live), else "Nothing for you to do. Who used what is in Insights."; the fold's note is "A striped day was not counted. It is not a zero."; guide §15 rows and `usageCopyGuide.test.ts`.
  - **Fidelity**: the read day is a `div` behind the band on `--radius-inner` (`rounded-inner`); with a limit the line sits 16px down and its word above it, moved to the line's left where a point over the limit would rise into it; the home month's axis is `pt-0.5`.
  - Tests: `UsageCard.test.tsx` (each kind; a year-old system with a later outage; a short buffer; a new member's person card; storage's "each share" and no "share" on any other kind; no link without a per-person metric; the read day's radius; the limit word at 95% full and over the limit), `usage.test.ts` (kinds in `plot`), `MyStorage.test.tsx` and `PersonStorage.test.tsx` (the month said once, per state), backend `TestAGapIsKeptWithItsKindAndOnlyWithOne`, `TestTheDayCountingBeganOutlivesTheWindow`, live `TestLiveAGapKeepsItsKind`, `TestLiveTheBeganGapIsReadHoweverOld`, `TestLiveMigration101KindsEveryStoredGap`, `TestMigration101IsExpandOnly`, TrueNAS `TestEveryGapGoesOutWithItsKind` and the schema refusals, kit `TestEveryDefectFailsTheCheckThatCatchesIt`. Mutation-checked (each red): `plot` ignoring the kind; the shape heuristic returned (a whole-metric gap crossing the window's start read as began); the fold listing a `began` gap; the link without the per-person check; `meteredMetric` ignoring `drawsHomeMonth`; no kind step for storage; the limit word back on the line; the view without `began`; `keepGaps` taking any kind; `StatsGaps` without the old began gap; the TrueNAS legacy kind fill; the kit's `began` check.
  *Rebase (18 Nov 2026):* merged `feat/addon-contract` at `443f8bf2` (7t-7, whose migration is 000100); the gap kind is now **000101** (`000101_stats_gap_kind`), with its tests, design.md §9 and NEXT.md's cutover list. **`syndra_test` left changed by the fix round's one full live-package run** (no dump was taken before it; schema, constraints, `schema_migrations` at `94|f` and every statistics table were left exactly as found, by fingerprint): rows other packages' live tests write and do not clean up, or rewrite: `audit_logs` 5,108 → 5,262 rows (+154), `plans` 1,838 → 1,894 (+56), `plan_subjects` 1,907 → 1,965 (+58), `desired_state_snapshots` 498 → 516 (+18), `sign_in_daily` 2 → 1 (−1), and `drift_items`, `onboarding_triggers` and `webhook_events` 1 row each with its content rewritten. Left as they are. This round's live run (000101's tests only) was bracketed by a `pg_dump` and a drop-and-restore: every table's rows and the columns identical by fingerprint, `94|f`; the restore re-deparsed three CHECK constraints (`ck_access_requests_pending_unresolved`, `ck_access_requests_status_enum`, `ck_claim_profiles_format_type`) into equivalent text, as PostgreSQL does on any restore.
  *Round 3 (18 Nov 2026, Opus on `7e1f624c`):* **(1)** the read day's tint takes no pointer (`pointer-events-none`) and sits behind the band (the band is `relative`, after it), so the readout no longer loops and the marks paint over the tint. **(2)** A gap the add-on sent is never lost: `keepGaps` keeps one with no kind, an unknown kind or no reason as `unread` (its reason as sent, empty when none) and still counts a `bad_value`; 000101 also loosens the reason's check to allow an empty one (down gives such a gap "No reason given"); the fold says "no reason given" (copy guide §15). **(3)** TrueNAS splits the buffer window at `Began`: `began` from `from` to `Began`, `dropped` from `Began` to `HeldFrom`, so an add-on older than its buffer keeps the record's start. **(4)** Reasons are a type (`gapReason`); a gap is made only by `gapOf` (kind from `gapKinds`) or `beganGap`; `TestEveryGapReasonHasAKind` parses the package for every `gapReason` constant and for any other `StatGap` literal. **(5)** `meteredMetric` also asks that the metric be counted with one series, so where the month cannot draw the usage card keeps it. **(6)** The limit word checks both ends and is left out where both collide (the meter says "full"); the fold's note is "A striped day was not counted. Neither it nor a day not counted yet is a zero." Live run (000101's tests only) bracketed by `pg_dump` and drop-and-restore: every table, the columns and the constraints identical by fingerprint, `94|f`; `TestLiveEachDropIsCountedByReason` now expects the reason-less gap kept (CI runs it). Mutation-checked (each red): the tint without `pointer-events-none`, the band not `relative`, `keepGaps` dropping again, no split at `Began`, a `gapKinds` row removed, a `StatGap` literal outside the makers, `meteredMetric` without the counted-and-one-series check.
  *Round 4 (18 Nov 2026, Opus on `cbf3f25d`):* the source-parsing test is gone; the type system holds it. `gapReason` is a struct, `{text, kind}`, each fixed reason written positionally (`gapReason{"…", kindUnread}`), so a reason without its kind does not compile; `StatGap`'s reason and kind are one unexported value (`why`), set only by `gapOf` (and `beganGap`, through it), read through `Reason()` and `Kind()`; the wire form is `statGapWire`, and `MarshalJSON` refuses a gap with no kind (one made as a bare literal), so it can never be served. A gap stored before kinds existed takes its sentence's kind on decode (`gapReasons`), else `unread`. `TestEveryGapReasonHasAKind` is now small: every declared reason has a contract kind, and a bare `StatGap` is refused on the wire. Mutation-checked: a reason written without its kind fails to compile; a keyed one without it fails the test; the marshal guard removed fails it. Left as noted (owner's low item): on the person page the band's limit is the storage read's (`observedLimits`, the home place's `quota_bytes`) and the "Up to N" line the roles'; when current usage cannot be read the band has no limit line at all, so the two never show different numbers, and they can differ only while the NAS has not yet taken a new limit, which the band then shows truthfully as the limit in force.
  *Slice 7s-2 (18 Nov 2026, owner, after staging):* (1) the card listed its metrics alphabetically by label (the store reads `ORDER BY metric`, then `statsUsageFor` re-sorted by label), which gave Home folder used, Storage sessions, Storage used; the board and A-23 give Storage sessions first. One rule, `addons.SortMetrics`: counters before levels, then the manifest order, then name; it feeds the TrueNAS card and the person and member cards (all one `statsUsageFor`). Insights keeps its own system-then-label order: it lists across systems and groups by unit. (2) The days before the record drew `--line` (6-8% alpha, invisible on real data); they now use `--line-strong`, the record's own baseline colour, so the 6px tick (in `--faint`) marks where the record starts. A 3:1 ratio is not met by `--line-strong` and is the hatch's job, so `chartTokens.test.ts` pins the baseline between `--line` and the hatch. Tests: `stats_order_test.go`, `UsageCard.test.tsx`, `chartTokens.test.ts`.

- [ ] 7u Density, alignment and motion (D-1, asked by the owner 18 Nov 2026 at 100% on staging: "why is the button on a separate line. the whole page looks zoomed in", "look for misaligned buttons due to dynamic text or spacing", "immersive consistent cohesive motion"). Built from `design_handoff_syndra/reconcile-2026-10/from-design/D-1-density-and-alignment.md` and its four boards `design/d1-density-1..4.dc.html` (committed with pass 1). One PR per pass to `feat/addon-contract`. Every taste call was first built on the designer's default, recommended, pending owner, so each flips in one token or one small change. **Decided by the owner, 18 Nov 2026:** "Yes, do recommended" for Q1–Q6 and Q8; Q7 flipped.
  - Q1 how much tighter: B (28 / 22 / 17). **Decided.** Flips in `@theme`'s `--text-*`, `--control-sm`, `--row-y`, `--card-head-y`, `--empty-y` (the boards' `opt-a` / `opt-c` blocks hold the other two steps).
  - Q2 a wide window: a 1280 frame, centred. **Decided.** Flips in `--page-x` (left against the rail: `26px`).
  - Q3 three body sizes: yes (pass 4). **Decided.**
  - Q4 Name: the row (frame 6). **Decided**; built in 7u-7.
  - Q5 silence for a healthy table row (frame 11). **Decided**; built in 7u-7.
  - Q6 Home: the two groups stacked (pass 3). **Decided.**
  - Q7 page change: **page transitions wanted (owner, 18 Nov 2026); spec written, D-1 §7a.** The page you leave fades on tint, the page you reach arrives as M1; React `<ViewTransition>` keyed on the pathname in AppShell. Two owner questions open (§9 Q7: when, with Next 16 recommended; fade alone recommended). Builds in the Next 16 slice unless the owner says now.
  - Q8 the sign-in door: exempt, written into FINAL §4 (motion pass). **Decided.**
  - [x] 7u-1 **Pass 1, tokens and shared parts** (every page tightens at once; no screen's structure is edited).
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-1`):* `globals.css` `@theme`: every §3 token (`--text-display` … `--text-fine` with line-heights and the title's weight, `--control-sm`/`-md`, `--page-max`, `--page-x`, `--page-top`, `--bar`, `--measure`, `--field-max`, `--region-gap`, `--region-head-gap`, `--card-x`, `--card-head-y`, `--row-y`, `--row-gap`, `--empty-y`, `--dot-indent`, the five seats); the phone block takes `--text-display: 28px`, `--text-page: 26px`. The six `type-*` roles read their steps; every `clamp` is gone (it saturated at 1080, so every desktop got the largest size); `type-section-title` has its line-height; `type-dialog-title` reads `--text-heading`. New utilities `measure`, `row-seat` (`data-seat="narrow|wide"`), `row-detail`, `row-top` (rule 4, `--row-control` is `--tap` until desktop). `AppShell`'s column is the `@container` `--page-x` reads in `cqw`; main, `TopBar` (`--bar`) and `SectionNav`'s band take `--page-x`. `PageHeader` meta and lede, `Region` (gaps, lede; `REGION_GAP` for `SectionPill`), `TargetOverview`'s two region grids, `EmptyState`, `Notice` (title `text-title`), `ActionOutcome`'s block title, `SettingRow` (label and action seats), `CardHeader` (tokens; new `caption`; `note` documented as a short fact), `CardRow` (tokens; new `action`, `seat`, `detail`, `top`), `CardColumns` (tokens; `seat` draws the empty heading cell), exported `ROW` (the list row's box), `PILL` (`text-small`, `desktop:min-h-(--control-*)`), `DENSE_FIELD`, `Chip`, `ArrowLink` (`data-arrow-link`, for the row guards). `--dot-indent`: `StatusSeat`'s dot column, `StandingLine`'s step through `StatusSeat`'s grid (`next`), `NextStep`, `ActionOutcome`, `RetryStatus`, `MemberAccess`, `ShapeChooser`. §5 #37: the sixteen list rows take `ROW` (the policies row splits it: floor and inset on the row, padding on its button, so its tint is the row's height).
    **Closes** §5 #30 (but `TargetOverview`'s reading, 18, which is pass 2's row), #31, #34, #37; the type half of #40 for Notice and ActionOutcome; and passes 2 and 3 (#1–#29, #32, #33, #36, #38, #39) are built on these parts.
    **Decisions:** `ROW` is the row's box (floor, inset, padding), not CardRow's whole line: the list rows stack on a phone (`flex-col`) and CardRow wraps, and one class carrying both alignments would let generated CSS order decide (§8 keeps every phone layout). A pill's height is a floor at the token (`desktop:min-h-(--control-*)`), not the board's fixed `height`: the same 30 / 36 for a one-line label, and a label that has to wrap grows its pill instead of spilling out of it. `--seat-number`, `--field-max`, `--text-figure` and `--text-fine` are declared and emitted, and no part reads them yet (pass 2 and pass 4 do). The `data-wide` opt-out for Insights' map is not built: nothing sets it; add it when the lane map needs the room. The motion section (§7, M1–M14 and the five motion tokens) is not one of the five passes; it is its own pass after pass 3.
    **Tests:** `design-system.test.ts` "density and alignment (D-1)" (every token at its value in `@theme`, the phone step; each display role reads its step and line-height, no clamp; `REGION_GAP` is `--region-gap`; the page frame is read by main, the top bar and the band, no `26px` left; no `min-h-[60px]` without `tablet:min-h-0`; no `pl-[14px]`/`pl-[15px]`), the type-floor test now resolves `var(--text-*)` at every declared value, the scroller test requires the `@container`; `RowSeat.test.tsx` (the seat is the last part before the detail, carries `data-action-seat`, `row-seat` and its width; the heading's empty cell and an empty seat take the table's width; no seat when none is asked; `row-top` never beside `items-center`; the seat in a disclosing row; CardRow is built on `ROW`; the caption is a `p` at the measure on its own line; SettingRow's seats; no control in a sentence across Notice, EmptyState, CardHeader and CardRow; StandingLine's step in StatusSeat's column); `Button.test.tsx` (accent and outline of one size, one height class from the token, no padding); `touch-targets.test.tsx` and `ActionOutcome.test.tsx` updated to the token. Mutation-checked (each red): the action outside the seat; `items-center` beside `row-top`; the detail without its marker; every seat at the default width; the heading cell missing; the caption drawn as the note; `ROW` without `tablet:min-h-0`; SettingRow's hand-made 200px seat; StandingLine's step outside the grid; `PILL` built from `py`, an outline variant with padding of its own; `REGION_GAP` 32; a clamp back in `type-page-title`; `--row-y` 14; the phone step removed; `pl-[15px]` back in NextStep; a list row typing its own box; the top bar's `26px`; the column without `@container`; a 12px title step; EmptyState's action in a `p`.
    *Fix round (18 Nov 2026, Opus on #200 `ac39590d`):* **(1) one column grid.** `row-cols` (`column-gap: var(--row-gap)`) is the table's grid: `CardColumns` and `ROW` carry it, and the hand-built table rows take it and state only `gap-y-*` (roles-that-wait, projects/[id], operations, bundles, InsightsUsage, RecordsList, ClaimsDelivery ×2, UnexplainedAccess ×2, ClaimHistory, BundleVersions, zitadel/grants, policies' row and button), so a heading sits over its cell; the fifteen `ROW` rows drop `tablet:gap-(--row-gap)`; `<CardRow className>` no longer states a gap (InsightsUsage, PeopleOnTarget). **(2) every phone layout kept (§8).** The phone block holds each density token a phone reads at its old value (`--text-title` 19, `--region-gap` 32, `--region-head-gap` 14, `--card-head-y` 16, `--row-y` 14, `--row-gap` 18, `--empty-y` 32), and the parts scope what a token cannot say to tablet and up (EmptyState `px-6`, Notice `py-4`, CardHeader's 11px wrap, a wrapped row part 18px down, Region's 6px). `SectionPill`'s `gap` is a CSS length and TargetOverview passes `var(--region-gap)`, so `REGION_GAP` (a JS mirror that would now disagree on a phone) is gone. **Known phone differences left:** body and control text follow the type scale at every width (§3; Q3 folds them in pass 4); `ActionOutcome`'s block title is the title step, 19 on a phone (it was 22, outside the scale, §5 #40); one row box means the six list rows that had `py-3` are 14px on a phone, `SettingRow` (now a CardRow) is 14px with an 18px gap, and RecordsList, zitadel/grants and the policies row have an 18px column gap where they had 16 or 12. **(3) `row-top`** is no longer a utility: its offset is a `@layer components` rule at zero specificity (`:where`), only from tablet, so a part's own `py-*` wins and a phone row is not padded; a `top` row is `items-center tablet:items-start`. **(4) one way to build a row.** `ActionSeat` is exported; `SettingRow` is a `CardRow` (`seat="default"`); `CardRowProps` is a union, so `onToggle` with `action` or `detail` does not compile. **(5) guards.** `test-utils/layout.ts` runs after every test in the suite (`test-setup.ts`), over whatever it rendered: an action (pill button, pill link, `ArrowLink`, now `data-arrow-link`) in a `<p>` or after text in one inline flow fails, a link in a lede, caption or meta line (`data-prose` on PageHeader's lede and meta, Region's lede, CardHeader's caption, the roles page's caption) passes; any `pl-*` other than the token on a line under a status dot fails. `PASS_2_SEATS` lists the five it finds that pass 2 seats (§5 #1, #8, and three found here: HomeMapping's home row, zitadel/users' intro row, the zitadel doors). Home's two links sit in a `div`, not a `p`; `VersionBand` (`pl-[18px]`, gap 10) and the connected systems' reading lines (`pl-4`) are at the token. The `{" "}<Button|ArrowLink>` source guard is in now, with a counted allowlist (the roles caption, AppTokenScreen's meta line, and the three pass-2 rows). The 48-character note test stays with pass 2. Mutation-checked (each red): a table row back at `gap-x-[18px]`; a gap on `<CardRow className>`; the phone `--row-y` removed; EmptyState's phone inset; the wrapped part's phone gap; `row-top` unscoped; `{" "}<ArrowLink>` in UpstreamShell; VersionBand back at `pl-[18px]`; PageHeader's lede without `data-prose`; Home's links back in a `p`; a `PASS_2_SEATS` entry removed; the guard's `p` rule and its inline-flow rule each off; SettingRow without its seat; `CardRowProps` letting `onToggle` take `action` (typecheck).
  - [x] 7u-2 Pass 2, seats: §5 #1–#29, #32, #35 (`action` / `detail`; long `note`s become `caption`; `PeopleOnTarget`'s data-driven seat; the Name and retention form rows; Accounts' Status column), and the three rows found by the prose guard.
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-2`):*
    - **Shared parts** (`Card.tsx`, `globals.css`, `SectionNav.tsx`):
      - `CardRow`'s line carries `data-card-row`, and a CardHeader `note` carries `data-card-note`.
      - `CardColumns` takes `seatLabel`, a heading for the seat ("Decision", "Outcome", "Needs you"; an sr-only "What happens next").
      - `row-seat` wraps and is never wider than its row (`max-width: 100%`, `min-width: min(token, 100%)`). A button's reason or three verbs never run off a phone card.
      - `row-detail:empty` takes no line.
      - A new token, `--seat-status` (240): the Status column on a system's accounts and a person's role rows (§5 #22, #39).
      - The section pill keeps an arrow seat on every segment, empty and hidden for the current one, so it holds its width as you scroll (#35).
      - `CopyableValue`'s button and the consent retry are `data-inline`: they belong to the words beside them.
      - Every hand-made number seat (`w-[110px]` with `text-right`) is `w-(--seat-number)`.
    - **Seats.** Every action in a row moved to the seat; sentences moved to the detail line or caption, word for word.
      - **targets:**
        - reach, with "Open Maintenance" in a narrow seat (#1, #30);
        - Check now as a `top` row (#2);
        - accounts no longer there (#3);
        - Not created by Syndra (#19);
        - Maintenance, where the reason form opens in the row's panel, so the verb, then Cancel, stays where it was pressed (#17);
        - findings, with Decide on the finding's line, Finish in the seat and the sentence as detail (#4, #12);
        - RetryWait (#11);
        - Name and what is kept as form rows (#13, #14; Q4 recommended, pending owner: Name stays its region);
        - HomeMapping's home row;
        - PeopleOnTarget: the seat is `wide` when the system's setup has a home limit field and `default` when it has none, held `wide` until the setup is read (#18). Status is its own column (#39; Q5: "Nothing for you to do." kept on every row);
        - mapping rows, with three verbs in the wide seat, "Nobody holds this role" as detail (never while holders load), and Technical detail under the row (#20);
        - version rows, Say what happened, and Publish (#21);
        - the long notes as captions (#32).
      - **Home:** the request rows' seat, with the held and rules lines as detail (#29, #36); Extend, whose result opens in the row's panel (#27); Open bundles (#10); Send it now (#27); Mark seen (#27); Open {system} (#10, #27); Review on Drift; the "and N more" rows; and the expiring note as caption.
      - **Usage:** the usage card's end row is a CardRow with Open Usage in its seat (#8). Insights' closing line stays a caption with its link (`data-prose`).
      - **Change history:** Pending changes in a narrow seat (#7).
      - **Connected systems:** what needs you is the row's seat under a "Needs you" seat heading.
      - **People:**
        - PersonAccess: Show the rule (#5), Decide on Drift (#6), and the role row's status seat with its default action seat (#22);
        - PersonSystems and PersonStorage;
        - AccountKind (#29);
        - PersonRequests (#29), with the holding note after the seat;
        - MemberAccess (#28), where "Asked to keep it" takes the button's place in the seat;
        - ServiceAccountPage (#28, #29);
        - RequestsScreen (#28, #29).
      - **Review:** Drift, with Adopt, Revoke and Owned elsewhere in the wide seat and the explanation as detail (#15); Holds due, with "Lift the hold" and an aria-label keeping its name (#25, the one forced word change); WithdrawnAccess (#29).
      - **Bundles:** the role rows (#24), the newer-version row (#16) and BundleVersions (#29).
      - **ShapeCard:** "In {where}." with its link in the seat (#9).
      - **Pages:**
        - policies, with Retire rule and its heading cell (#24);
        - a role's people (#23);
        - expiring access (#26);
        - roles that wait (#29);
        - pending's stopped rows (#29);
        - zitadel/users' intro row;
        - the zitadel doors, with the door and Syndra's page in one seat;
        - zitadel/projects and ClaimHistory, whose hand-made seats are now `ActionSeat`;
        - TrendsTab's note as caption.
      - **Copy guide** §17: "Lift the hold" and "Status".
    - **Guards:**
      - `test-utils/layout.ts`, after every test in the suite: an action in a CardRow outside its seat, a form or a dialog fails (a fold's own `aria-expanded` toggle excepted); a CardHeader `note` over 48 characters fails, pointing at `caption`. `PASS_2_SEATS` is gone, and the suite runs with no exceptions.
      - `design-system.test.ts`:
        - the `{" "}` source guard allows exactly the roles caption and AppTokenScreen's meta line;
        - no pixel width beside `justify-end` and no `w-[110px]` number seat outside `Card.tsx`;
        - `<CardRow className>` never states a gap (read at the tag's top level);
        - the seat wraps inside its row;
        - an empty detail takes no line;
        - `--seat-status`.
    - **Tests:** every moved row has a seat test, in its component's test file. Each asserts the action in `[data-action-seat]`, one `data-seat` on every row of the table including rows with no action, and `[data-row-detail]` where there is one. `RowSeat.test.tsx` covers the seat and note guards and `seatLabel`; `SectionNav.test.tsx` covers the arrow seat; `AccountsTable.test.tsx` covers the held wide seat.
    - **Mutation-checked** (each red):
      - the seat guard without its seat exemption, and the note limit raised;
      - copy controls counted as actions;
      - PeopleOnTarget's seat read from the rows, and its seat not held while loading;
      - Holds due without its aria-label;
      - `seatLabel` dropped;
      - `row-detail:empty` removed;
      - the arrow seat's width removed;
      - a hand-made pixel seat restored;
      - a gap on `<CardRow className>`.
    - **Opus review round** (before the PR), every blocking and medium finding fixed:
      - the Maintenance form moved into the row's panel; it now stays mounted, so the pressed button keeps its focus;
      - the seat can no longer overflow on a phone;
      - MemberAccess's answer is back in the seat;
      - the request note is in blocks and after the seat, and the consent retry is `data-inline`;
      - no `order-*` row hacks (Extend's result and Technical detail);
      - expiring's seat is right-aligned at every width;
      - PeopleOnTarget's seat is held while loading;
      - UsageCard's end row and the zitadel doors are CardRows;
      - BundleVersions uses the default seat;
      - "Nobody holds this role" is not shown while loading;
      - MergeFindings has no empty padded div;
      - Holds' retry is right-aligned;
      - fold toggles are exempt only from the seat guard, not from the prose guard.
    - **Staging round** (18 Nov 2026; lead in Chrome, 1481px, dark):
      - **(1) The reports card's titles went a word a line, and (2) long captions sat at the right of their titles.** One cause:
        - The detail line (and the caption) was `flex-basis: 100%` capped by `max-width: 60ch`. Flex wraps an item by its basis clamped to its max-width, so on a wide card a 60ch line fitted beside a short first line and sat there.
        - On a reading it squeezed the title's `flex-1` part to a word a line ("A / disk / reports / errors…"). On a CardHeader, the `flex-1` spacer pushed the caption to the right.
        - The 48-character note guard was right: both were already captions, not notes. The guard reads a note's rendered text, string or node alike.
      - **Fix (2):** `row-detail` is full width, with the measure as padding (`padding-inline-end: max(0px, calc(100% - var(--measure)))`), never a max-width. CardHeader's caption takes `row-detail`'s shape.
      - **Fix (1):** the connected systems' reading sentence, the one hand-made copy of the trap (`order-last w-full max-w-[80ch]`), is now the row's `detail`.
      - **No label seat was involved.** The narrow "label column" was the title squeezed by the detail beside it. The label seats (`--seat-label`) hold only short labels.
      - **(3)** The accounts table's Status column holds the state and its note only (`StandingLine step={false}`). The next step (`StandingStep`, the same words) is the row's detail line, with the home folder line, full width under the row.
      - **Guards:**
        - `design-system.test.ts`: the detail line is full width with the measure as padding and no max-width; the caption is `row-detail`; no class string anywhere builds a full line (`basis-full`, or `order-last` with `w-full`) capped by a `max-w-*` or `measure`.
        - `SystemHealth.test.tsx`: the reports card with a CRITICAL SMART alert and a degraded `raid-z1-slow` pool, the title in the row's flexing first part and never a fixed-width column, the sentence in `[data-row-detail]`.
        - `AccountsTable.test.tsx`: the step is in the detail line, not the status cell.
        - jsdom draws no layout, so the CSS rule is the guard that fails locally on this class of defect.
      - **Mutation-checked** (each red): the detail line back to a max-width; the caption back to `measure basis-full`; the systems reading's hand-made line; the step back in the status cell.
    - **Left, low:**
      - while Check now waits, the check's own row says nothing beside the disabled button, because the wait line is RetryWait's row under it;
      - ServiceAccountPage's sources keep their pre-existing `order-last` on a phone.
  - [x] 7u-3 Pass 3, Home (Q6).
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-3`):*
    - **Home (§5 #38; Q6, recommended, pending owner):**
      - Home's two groups, Time-sensitive and When you're ready, stack a region apart (`flex-col gap-(--region-gap)`). Before, they sat in two pinned columns, so a short group left its column empty beside a long one.
      - Each group's heading is a region's (`type-section-title`, no longer the card titles' size), and its note is `text-small`.
      - A group's blocks sit in a grid, `repeat(auto-fit, minmax(min(100%, 600px), 1fr))`, gap 16, under `--region-head-gap`: two across where each can be 600 wide (the 1800 window), one under the other where they cannot (1440).
      - `Block` no longer carries its own 14px. A block that has gone renders nothing, so it holds no cell and no gap.
      - **To flip Q6** back to two pinned columns: `desktop:grid-cols-2` on the stack, and `col-start` on the groups.
    - **Staging consistency (lead, 18 Nov):**
      - **(1) "Not created by Syndra" follows the rule "Accounts Syndra manages" set.**
        - The status column holds the state and its note: `StandingLine step={false}`, in a `--seat-status` seat.
        - The next step (`StandingStep`, the same words) is the row's detail line.
        - The account flexes, so the status starts at one x whatever the username's length.
        - "No longer needed" has no status column (its rows say when each account last signed in), so there is nothing to move.
        - Not created's adoption outcome panel is always passed (a fragment), so the outcome arriving never remounts the row.
      - **(2) Syndra's own row** keeps an empty seat, and its sentence stays on the detail line, now beside the other rows' next steps.
    - **Tests:**
      - `HomeColumns.test.tsx`: an empty group leaves and the other stands at full width; each group's heading is `type-section-title`; its blocks use the 600px auto-fit grid.
      - `TargetOverview.test.tsx`: Not created's status seat is `--seat-status`, with the next step in the detail line and not in the seat; Syndra's own sentence is in the detail line beside an empty seat.
    - **Mutation-checked** (each red): the stack back to two columns; the heading back to a card title; the blocks' minimum at 420; Not created's step back in the status seat.
    - **Staging round** (18 Nov 2026; lead, 1481px): Home's "Gaps" stretched to the height of "Roles most people hold" (about 370px, one row), in the makerspace's hand-built grid with no start alignment. When you're ready's short block sat beside a long one.
      - **One rule, one primitive.** `CardGrid` (`Card.tsx`): `grid items-start gap-4`, as many across as can each be `min` wide (`pair` 420, `wide` 600).
      - **Why top-aligned, rows first:** a card is as tall as what it says, never stretched, and the space under a short card is the page's.
      - **Why not a column flow:** it would fill that space, but it moves a card from one column to the other whenever a count changes (structure moving with data), reads down before across, and breaks a block's fold animation at a column break. Stacking would waste the 1800 frame.
      - **Applied** to Home's blocks (`wide`), the makerspace's Gaps and Roles (`pair`), the system page's reach and reports cards and its "What it can do" and "Check for differences" cards (`pair`), and Insights' trends (`pair`; the full-width chart is `col-span-full`).
      - **Left as they are:** a band of stat tiles (`StatCard`, now `data-stat-tile`) is equal tiles, not cards, and keeps its own grid.
      - **Guards:**
        - `test-utils/layout.ts`, after every test: a grid that splits into columns and holds two or more cards (directly, or each in one wrapper) outside `CardGrid` fails.
        - `design-system.test.ts`: `CardGrid` is `items-start`, and no hand-built column grid holds a `Card` or `…Card` part in source.
        - `RowSeat.test.tsx`: `CardGrid`'s classes, and the guard catching a hand grid while leaving stat tiles alone.
      - **Mutation-checked** (each red): the makerspace back to its hand grid (caught by the suite-wide guard over the Home tests); the system page's "What runs here" as a hand grid; `CardGrid` without `items-start`.
  - [x] 7u-4 Pass 4, three text sizes (Q3), with §5 #40's remaining raw display sizes and #41 `/storage`; then no `text-[NNpx]` in components.
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-4`):*
    - **The fold** (Q3, recommended, pending owner): a mechanical codemod over 163 files, keeping every variant prefix.
      - `text-[15px]`, `[15.5px]`, `[14.5px]` and `[14px]` become `text-body` (14, 1.5);
      - `[13.5px]` and `[13px]` become `text-small` (13, 1.45);
      - `[12.5px]` and `[12px]` become `text-fine` (12.5, 1.4).
      - The 15.5 row names are a row's first line, so they read the body step (§3).
      - `body` in `globals.css` reads `--text-body` and its line-height (was 14.5 / 1.55).
      - To flip Q3: change the three tokens' values, since every caller reads a step.
    - **§5 #40, the display sizes onto the scale:**
      - **To the figure:** the stat tile's and the usage card's figure (24) became `text-figure` (24, 600 from the token).
      - **To the heading:** Insights' answer title and `/storage`'s headline (24) became `text-heading` (22).
      - **To the title:**
        - AccessSource's expanded title (19), NotOpenYet's page line (19), and its other line (15, now `text-body`);
        - the makerspace's counts (20 and 18), with `font-normal`, so a count keeps its weight;
        - BundleVersions' version (17), also `font-normal`.
    - **§5 #41 `/storage`:** headline at `--text-heading` (22); its sentences (66, 70 and 62ch) at `measure`; its fields (460px) at `--field-max`.
    - **§6 test 6, every sentence at the measure:** the remaining hand-set caps (60 to 92ch, 15 files) became `measure`. Left alone:
      - the greeting's balance widths (22, 34ch);
      - CommandBlock's code;
      - a stat tile's own detail (42ch);
      - a button's reason (46ch).
    - **Left at their pixel sizes,** each marked `type-scale-exempt` beside it:
      - the avatar initials;
      - the dialog field and textarea (15px);
      - the sidebar's wordmark (18, the brand's, not a step).

      Decoration nobody reads (`aria-hidden`: the sort and section arrows, the rule chain's arrow) also keeps its size.
    - **Phone values (pass 1's note):** body and control text now follow the type scale at every width, as §3 sets it; that was the noted difference, and this fold is its resolution. Unchanged, and still noted:
      - ActionOutcome's block title is the title step (19 on a phone, was 22);
      - the six list rows that had `py-3` keep the one row box (14 on a phone).
    - **Guards** (`design-system.test.ts`):
      - no `text-[Npx]` anywhere in `src` but `aria-hidden` decoration and what is marked `type-scale-exempt` within the eight lines above;
      - `body` reads `--text-body`;
      - no `max-w-[Nch]` outside the four places named above.
    - **Tests updated:** `UsageCard.test.tsx` (the figure is `text-figure`) and `ViewSwitch.test.tsx` (`text-fine`).
    - **Mutation-checked** (each red):
      - `text-[14.5px]` back in PageHeader's meta;
      - the stat figure back to `text-[24px]`;
      - `body` back to 14.5 / 1.55;
      - `max-w-[60ch]` back in ErrorBoundary.
    - **Review round** (18 Nov 2026, lead on #203):
      - **(1)** TermDatesCard's and WelcomeClaimsCard's opening sentences: the divider and inset are on a full-width wrapper, and `measure` only on the `<p>`. The divider line was cut at 60ch.
      - **(2)** DegradedBanner: `measure` moved from the container holding the command block onto the `<p>`, so the command and its Copy keep their width.
      - **(3)** PersonAccess's cleanup hints: `max-w-[840px]` became `measure` (spec §3 row 78). ChoiceRow's 170px note is now `max-w-(--seat-label)`, the same width as a token.
      - **(4)** The measure guard exempts by line marker, not by file. The only exemption left is the greeting's balance widths (`measure-exempt` on the line above). The rest moved to `measure`: CommandBlock's caption and step, the stat tile's detail, and a button's reason.
      - **(5)** `error.tsx`: no cap on the column. The Notice sets its own measure, and the link is short.
      - **(6)** The type guard:
        - A marker covers only its own line or the line directly below it; the eight-line window is gone. Each avatar size and the dialog field and textarea carry their own.
        - It also catches an inline `fontSize` literal and Tailwind's own steps (`text-xs` … `text-9xl`).
        - Those steps are also cleared at the source: `--text-*: initial` in `@theme`, Tailwind v4's way to empty a namespace, so `text-sm` now renders nothing.
        - `global-error.tsx` renders after the root layout failed, so `globals.css` and its tokens may never have loaded, and it may import nothing from the app. Its sizes are the scale's steps written out once, in a `TYPE` map with each line marked: heading 22, body 14, small 13, fine 12.5. They were 20, 14.5, 13, 12.5 and 13.5.
        - The usage band's limit word (SVG `fontSize="12.5"`) is now `text-fine`.
      - **(7)** NotOpenYet: the dead ternary is gone.
      - **Mutation-checked** (each red):
        - a marker moved two lines away from its size;
        - `global-error` with a literal `fontSize`;
        - a `text-sm`;
        - `--text-*: initial` removed;
        - PersonAccess back at `max-w-[840px]`;
        - CommandBlock's caption back at `max-w-[80ch]`;
        - the greeting's marker removed;
        - an avatar size without its marker.
  - [x] 7u-5 Motion (§7: the five tokens, M1–M14, the eight motion tests).
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-motion`).* There is one motion vocabulary: `lib/motion` and the tokens. Nothing travels over 10px, no count ticks, there is no route transition, and every movement has a still form (`stillMotion()` in JS, the one `prefers-reduced-motion` block in CSS).
    - **Tokens** (`globals.css` `@theme`): `--duration-fold` 200ms, `--light-lead` 40ms, `--travel-arrive` 8px, `--travel-lift` 6px, `--travel-nudge` 4px. `still.ts` gains `FOLD_MS` beside `LIGHT_LEAD_MS`, and `design-system.test.ts` holds the mirrors.
    - **M1:**
      - The system page's region wrappers carry `data-arrive-through`.
      - The `page-head` rule passes the stagger through a marked wrapper, and on through a nested one (`--arrive-k`, `--arrive-j`), capped at five.
      - The section pill's sticky root takes no slot.
    - **M2:**
      - `Count` in `Badge.tsx` is a `Crossfade` keyed on the number. `CountChip`, `CardHeader`'s count and SectionNav's chip and pill counts read it.
      - The first read shows one layer, and a failed read shows a dash.
      - The rail's `settled` gate is reproduced by Crossfade's own first-mount rule, not shared, because the rail's flag lives in Sidebar.
    - **M3:**
      - `Notice` takes `open` and stays mounted through its close: it fades, then folds (`useExitPhase(TINT_MS + FOLD_MS)` and `Unfold`).
      - The region's count is read from the rows still drawn (MergeFindings `onDrawn`).
      - The all-clear (new `AllClear.tsx`) is `resolved`. It is `lift` only when the list emptied in front of the reader, and it blooms only when the rows that left are ones the reader resolved (a Set of resolved ids, not a clock).
      - The anchor finding fades and folds while the count holds.
    - **M4:**
      - Check now's sentence is in a `Crossfade`, and the row flashes once when the result changed (`useFlashOnChange`).
      - New stale rows arrive (`ArriveList`).
      - Both follow the last settled result, held in a ref, so a pending check (the mutation's `data` is undefined while it runs) neither flashes nor replays.
    - **M5:** "Not created by Syndra" and "No longer needed" use `useLeavingRows` / `LeavingRow`. Their counts come from the drawn rows, and the adoption outcome holds its row (`stay`).
    - **M6:**
      - `ActionOutcome`'s block form mounts in `UnfoldOnArrival`. Whether it sits inside an opening region is read once, at mount, so it never opens or announces twice.
      - A finding's Decide form opens by `Unfold`, only while the finding is undecided, and Decide/Cancel crossfade in the seat.
    - **M7:** the section pill keeps its arrow seats (pass 2), and its fill is the shared `GlideIndicator`. `glide-indicator.tsx` finds `aria-current="location"` and watches the group's segments of any tag.
    - **M8:** `Region` marks its heading `data-landing`, and `glideTo` flashes a target's landing row when it has one.
    - **M9:** already right in code. A sheet's settle-back was on settle's curve, and the dismissal stays on tint's (an exit; FINAL §4). The spec's line references pointed at the dismissal. A first build moved the dismissal and was reverted after review. `ModalMotion.test.tsx` holds both.
    - **M10:** the lane map's two items press (`control press-scale`).
    - **M11:**
      - The new tokens replace their literals: the fold `calc`, every light-lead 40ms (including the scrim that borrowed `--arrive-step`), and the keyframes' travel.
      - `motion-press` and `motion-settle` are gone; nothing used them.
    - **M12:** `Unfold` asks `stillMotion()` first. The loose timing constants are in `still.ts`: `FLASH_MS`, `LANDED_HOLD_MS`, `PRESS_FORGET_MS`, `SKELETON_DELAY_MS` and the `ARRIVE_*` values. A closing `Unfold` is `inert` and `aria-hidden`, so a cancelled form's button cannot be pressed while it folds.
    - **M13:** the policies page's one scroll is inside the rule dialog, whose panel scrolls itself, so it keeps `scrollIntoView`. `glideTo` is the page's glide, and M13 covers page jumps. There is no other jump on that page; the section chips and pill already glide.
    - **M14** (Q8, decided by the owner 18 Nov 2026): the door keeps its two loops and its own timings, named `--door-*`. `ceremony.ts` mirrors its two easings, with a test. FINAL §4 has the exemption (decided 18 Nov 2026).
    - **Guards** (`design-system.test.ts`):
      - **Literal motion timings:**
        - no literal `Nms`, `cubic-bezier(`, `duration:` / `delay:` number, or seconds value;
        - no numeric timer delay, in components, app and `lib/` outside `lib/motion`;
        - a timer that is not motion says so with `// not-motion: why` (four: a debounce, two clocks and a countdown);
        - the door is exempt only in `ceremony.ts` and by `// door-exempt:`.
      - **Reduced motion:**
        - every movement in script (`.animate(`, `requestAnimationFrame`, an inline transition or animation) asks `stillMotion()` or calls a primitive that does;
        - the reduced-motion block collapses durations, delays and iterations.
      - **Tokens and wrappers:**
        - the five tokens and their mirrors;
        - no `40ms` and no fold `calc` left;
        - `motion-press` and `motion-settle` gone;
        - the page-head wrapper rule.
    - **Tests** (§7's eight):
      - `design-system.test.ts` (1);
      - `Notice.test.tsx` (2);
      - `CountCrossfade.test.tsx` (3);
      - `MergeFindingsMotion` and `TargetOverviewMotion` (4, 6, the M3 order, M5, M6);
      - `SectionNav.test.tsx` (5);
      - `ArriveList.test.tsx`'s Back and view-switch cases (7);
      - `motion.test.tsx` (8);
      - plus `ModalMotion`, `UnfoldView`, `glideIndicator` and `ActionOutcome`.
    - **Tests changed:** `motionInTests()` added where a test meant motion to be on, now that `Unfold` honours `stillMotion()`. `{ hidden: true }` added where a test reads content that is closing, now `aria-hidden`.
    - **Mutation-checked** (each red):
      - a literal `40ms`, and the fold `calc` restored;
      - `--travel-lift` changed, `FOLD_MS` changed, and `--door-ease-exit` changed;
      - delays dropped from the reduced-motion block;
      - the wrapper's `animation: none` removed, the nested rule removed, and the pill's slot counted;
      - a raw `260ms` in Modal, the dismissal or settle-back swapped, a `not-motion` marker removed, a `door-exempt` marker removed, a WAAPI duration in ArrowLink, and a bare `requestAnimationFrame` in Badge;
      - the closing `inert` removed, and the indicator watching only buttons;
      - the Crossfade keyed on a constant, and the landing lookup removed;
      - the wrapper attribute, the count read from the data, the anchor hold, bloom always on, and lift dropped;
      - the flash class, the Crossfade id, and both tables' `LeavingRow`;
      - Notice not staying mounted, ActionOutcome's Unfold, and the Decide form's Unfold;
      - the pending-gap replay, the remount on settle, a Notice that does not open late, lift on a first read, and the form under a decided finding;
      - `glideTo` back in the rule dialog.
    - **Opus review before the PR:** two blocking findings (the dialog jump, and Check now flashing on every press) and seven medium ones, all fixed as above.
    - **Left as is:**
      - the unconfirmed changes' count still comes from the read, since `UnconfirmedChanges` has no `onDrawn` yet;
      - the pill's slot is selected as `.sticky:first-child`, a data attribute on SectionPill would be firmer;
      - the page-head stagger is held as CSS text, because jsdom computes no delays.
  - [ ] 7u-6 Pass 5, resync the design system (`ui/.design-sync`, BRIDGE steps 1 to 3), after the owner's OK.
    *Code side done, publishing waits on the owner (18 Nov 2026, branch `feat/addon-contract-d1-5`).* Steps 2 and 3 publish to the Syndra Design System artifact and copy it into the canvas. BRIDGE and the standing rule ("ask before any publish") put both after the owner's OK, and the owner is away, so neither ran and no board lost its override block. Pass 5 adds no motion: §7 is the next pass (7u-5).
    - **Cards.**
      - `CardGrid` and `ActionSeat` are design-system cards (`config.json` `componentSrcMap`), with previews: `CardGrid` Pair and Wide, `ActionSeat` OnAListRow.
      - CardRow's preview shows the row D-1 built:
        - ActionSeats: the table-level seat, an empty seat kept, the next step on the detail line, the caption, and the Status column at `--seat-status`;
        - TopAligned;
        - DisclosureWithControl, whose button is now in its seat.
      - CardHeader gains Caption.
      - CardColumns gains WithSeat (`seat` and `seatLabel`).
      - `moves.json`: CardRow's Phone line says its seat wraps under it, and CardGrid and ActionSeat have their lines. The preview blocks and `MOVES.md` are regenerated (`node .design-sync/moves.mjs`).
    - **Preview glue reads the type scale.** The 62 numeric font sizes in 41 previews became `var(--text-body | small | title)`, as pass 4 folded the app; the widths in the touched previews became the seat tokens.
    - **Step 1, run locally (nothing published):**
      - `bun .design-sync/build-css.mjs` and `resync.mjs` built `ds-bundle/`: 120 components, 120 previews, "bundle is complete".
      - The 46 cards it captured have no errors; the ten render warns are the dialogs, GoTo, KeysSheet, PushedStep, RehearsalDialog and TouchNav, already listed in `NOTES.md`.
      - The captures of CardRow ActionSeats, CardGrid Pair and CardColumns WithSeat read as the app does: seats right, detail lines under the row, cards top-aligned.
      - `ds-bundle/` stays local (ignored) until the owner's OK. Then BRIDGE step 2 (`ds_merge.py`, `token_check.py`, publish) and step 3 (the canvas copy, then drop the D-1 boards' override block).
    - **Guard** (`design-system.test.ts`):
      - every part `Card.tsx` exports is a `Card.tsx` card in `componentSrcMap`, with a preview;
      - no preview's glue sets a numeric `fontSize`;
      - CardRow's preview shows a seat with an action and a detail line; CardHeader's shows a caption; CardColumns' shows a `seatLabel`.
    - **Mutation-checked** (each red): `CardGrid` left out of `componentSrcMap`; a preview's `fontSize: 14.5`; CardRow's preview without its detail lines.
    - **Review round** (18 Nov 2026, lead on #204):
      - **(1)** The guard reads CardRow's preview within the ActionSeats story's own block. Before, `action=` in TopAligned or DisclosureWithControl could stand in for it.
      - **(2)** The guard also catches an arbitrary pixel text class (`text-[13px]`) in a preview. GoToPanel's idle line is now `text-small`.
      - **(3)** CardColumns' "Apps served" column is at `--seat-action`; it was 200.
      - **(4) Type errors that predate this pass, all fixed:**
        - `entry.tsx`'s `export *` met two `Seat`s: Card.tsx's width type (D-1 pass 1) and Choice.tsx's component. Card's type is now `SeatWidth` (Card, Home, PeopleOnTarget, TargetOverview).
        - ActionOutcome's previews now pass the required `word`.
        - The Notice and Unfold previews now pass their step (`next` or `nothingToDo`, NoticeNextStep).
        - PushedStep's change reads `said` (the field was renamed).
        - RoleWordsTable's rows have one row type.
        - **The previews now type-check in CI.** `.design-sync/tsconfig.previews.json` maps `ui` to `entry.tsx` and `@/*` to `src`, covering `entry.tsx` and every preview. `bun run typecheck`, which CI runs, now runs it after the app's (about 1.5s). Named `tsconfig.previews.json` so the resync's `tsconfig` setting cannot pick it up by accident.
      - **(5)** `NEXT.md`'s stray space.
      - **Mutation-checked** (each red):
        - ActionSeats with its actions removed, while the other stories keep theirs;
        - GoToPanel back at `text-[13px]`;
        - PushedStep back at `reason` (`bun run typecheck`).
  - [x] 7u-7 The owner's answers (18 Nov 2026): Q4 the row, Q5 silence in table rows, and two fixes.
    *Done (18 Nov 2026, branch `feat/addon-contract-d1-owner`):*
    - **Q4, the Name row** (part 2 frame 6). `TargetNameRow` (`targets/TargetName.tsx`) is the settings card's first row: Name, the name, what members see (the first half of `nameHint`), and Change name in the seat. `SetupStrip` (`ProjectSetup.tsx`) draws it in every state, so a system whose project is not chosen still has its name (the card holds the name row alone then, and for the visit the project was chosen in); the project's rows are `ProjectRows`, whose reads start only once there is a project. The Name region is gone from `TargetOverview`; nothing linked to `#name` (the section pill and nav never listed it). One write place: today's form, word for word, is `TargetNameDialog`, opened by Change name; it ends on its result with Close, as Choose folder's dialog does, and Enter in the field saves.
    - **Q5, silence** (frame 10 → 11; G17). `rowStep` (`lib/account-standing.ts`) is a table row's step: none for a healthy row whose step is "Nothing for you to do.", the standing's step for every other row. Both account tables (`PeopleOnTarget`, Not created by Syndra) draw a detail line only when there is a step (or a home folder line, or Syndra's own account line). `StandingStep` is folded into it. `standingWords` is unchanged, so the member's card still says "Nothing for you to do.". The copy guide's next-step rule names the exception and its limit.
    - **The Home folders row** (`kinds/storage/HomeLocation.tsx`) returned `null` until the settings were read, so the rows under it jumped on first paint. For a storage system it now holds its row while they are read: the label, a quiet "…" (`aria-busy`), and a skeleton the button's size in the seat. A system that is not storage still draws nothing (it does not offer the setting).
    - **The pill's glide** (`lib/motion/glide-indicator.tsx`, `globals.css` `indicator-glide`) is transform only. The indicator is translated to its place and drawn in three abutting parts: two caps, translated and never scaled, so the rounded ends stay round all through the glide, and a square-ended middle scaled along x. Abutting, not overlapping, so a translucent fill (`bg-tint-3`) stays one colour. Same tokens (settle, its ease); no class under reduced motion, so it still jumps.
    - **Guards and tests:** `next-step.test.tsx` (every standing, both audiences, with and without a step: only a healthy row with nothing to do is silent; a row that is not healthy always has a step, "Nothing for you to do." included); `AccountsTable.test.tsx` (a healthy row has no detail line; three rows that are not, or have a step, write it); `TargetName.test.tsx` (the row, its seat, no field on the page; the dialog's reason, the default-name button, Enter); `TargetOverview.test.tsx` (the name opens the settings card with and without a project, no `#name` region or link; the Home folders row held while read, and none for a system that is not storage); `design-system.test.ts` (`indicator-glide` transitions `transform` only, on settle's tokens); `glideIndicator.test.tsx` (a fill and an underline placed by translate, caps never scaled, the middle scaled along x); `GlideIndicator.test.tsx` and `PersonSystems.test.tsx` moved to the new shape.
    - **Mutation-checked** (each red): `rowStep` silent on any "Nothing for you to do." (the not-healthy rows fail); `rowStep` never silent; the Name row only with a project; the Home folders row's held state removed; held for every system; `width` back in the glide's transition; a cap scaled.
    - *Fix round (18 Nov 2026, review and staging on #206):*
      - **(1) The Home folders row never vanishes on a storage system.** A read that failed, or came back unread, keeps the row: "Could not be read", the operator's read-failure step, and Try again (`settings.refetch`) in the seat. **(2)** The held row is two lines (… and a skeleton line), as "Not chosen yet" is.
      - **(3) The glide's middle is tucked 0.5px under each cap** (`TUCK`), so two antialiased edges never leave a hairline mid-glide. Over a translucent fill the half pixel is doubled, far below what an abutting seam showed over an opaque one.
      - **(4) Healthy is usable and waiting on nobody** (`rowStep`): a usable account still waiting on the member, staff or the system keeps its step. **(5)** "Not created by Syndra" asserts its healthy row is silent and a waiting one is not.
      - **(6)** The non-storage test reads each state (loading, failed, unread, not offered) against a storage system in the same state, so it can fail.
      - **(7) The section pill's fill reads in light theme** (it predates this PR). It was `bg-surface-2` on the pill's `bg-surface-1`, both white in light. It is now FilterPills' selection, `bg-tint-3`, as `PILL_FILL` beside `PILL_SURFACE` (`SectionNav.tsx`), and the pre-measure fallback uses the same constant. Guard (`design-system.test.ts`): the two tokens differ, and their values differ in each theme.
      - **Mutation-checked** (each red): healthy as `usable` alone; "Not created by Syndra" never silent; the failure row gone on an error; `expected` dropped; the held row back to one line; `TUCK` 0; the fill back at `bg-surface-2` (red in light), and at the surface; the indicator off `PILL_FILL`.
    - **Recorded:** Q1–Q8 decided in D-1 §9 and above; Q7 is page transitions wanted, spec to follow (D-1 §7a); FINAL §4's door exemption is no longer pending, and its "never a route transition" says it waits for that spec. The TrueNAS account-deletion research is `research-account-delete.md` beside this file, tracked in `NEXT.md` 0k (owner: implement later; its slice numbers 7u-0..7u-5 predate D-1's 7u).
  - [x] 7u-8 A table's headings on its rows' columns (the owner, 19 Nov 2026: "the drift page has issues with the column alignment").
    *Done (19 Nov 2026, branch `fix/table-column-alignment`):*
    - **Root cause:** every table typed its CardColumns widths beside its rows', twice, and the copies drifted. On Drift a heading ("Why Syndra can't explain it") stood over a column the rows never filled (the reason is the row's detail line, D-1 §5 #15), and the rows' 3px edge put every cell 3px right of its heading. Requests drew the avatar as a part of its own, so every cell after it sat ~46px right of its heading. And `SortHead` reserved its invisible arrow's room after the label, so a right-aligned heading (Found, Revoked, Waiting, Attention) ended 16px short of the figures under it.
    - **One column list** (`Columns`, `Card.tsx`): each column's classes (width, alignment, the breakpoint it shows from), given to `CardColumns cols` and to every row (`CardRow cols`, or `Cells` for a row that is a link or a label). Each heading and each cell is drawn in its column's box (`data-col`), so a heading cannot sit anywhere but over its cells. Flex, not CSS grid: a row still wraps its seat under itself where it cannot fit, which a grid track cannot do. `CHECK_COL` (`SelectionBar.tsx`) is the checkbox's column; `WHO_COL`, `WHAT_COL` (`RowSeats.tsx`) and `NUMBER_COL` (`Card.tsx`; it was `FIGURE_COL` in `RowSeats.tsx` until 7y) the review queues' shared columns, and WhoSeat / WhatSeat no longer size themselves.
    - **Drift:** Why is no column of its own: its sort shares What's heading cell (one `Cells` child holding both buttons), because the reason is the row's detail line under What (D-1 §5 #15), and it still gathers equal reasons A to Z with their counts (`route-drift` "Sorted by why"; RULES: a reason is a gathering column). Found is right-aligned over right-aligned dates; the headings take the rows' 3px edge, unpainted; both sections ("In Zitadel, not given by Syndra" and "Given by Syndra, gone from Zitadel") share the one heading, and their compared rows (a role the comparison shows on one side only) are CardRows on the same columns, their sentence the detail line and "Check Pending changes" in the seat.
    - **Audited:** Unfinished revocations, Expiring access (also Ends right-aligned, the edge), Requests (the avatar in Who), a person's Requests, Pending changes, the TrueNAS accounts table, People (the Person column gives way to 180 for heading and rows alike) and Audit are on one list each. Holds due and Change history have no column headings. The other CardColumns tables (Projects, Roles, Apps, Bundles, Systems, Service accounts, Zitadel grants and projects, Roles that wait, Policies, Incoming events, Insights usage, claim and debug histories, records) still type their headings by hand: the guard holds their seats, and moving each onto `cols` is a follow-up.
    - **Guard** (`headsOffRows`, `test-utils/layout.ts`, in `layoutFindings`, so it reads every table the suite renders): every row after a CardColumns in its card has the same column boxes, class for class, and the same seat width (or none). `RowSeat.test.tsx` holds it to Drift's two faults and Requests' extra part; `SortHead.test.tsx` the hung arrow; `UnexplainedAccessSections.test.tsx` the three headings and the reason under none.
    - **Mutation-checked** (each red): the drift heading's seat dropped (75 failures); the drift rows off the checkbox column (14); the Requests avatar back as its own part (51); the People heading without its checkbox column and the Unfinished revocations heading without Where (28); `Cells` without its column's classes; either comparison in the guard removed; the arrow back in the flow.
    - *Review round (PR #216):*
      - **The headings wrap as their rows wrap.** CardColumns is `flex-wrap` (the seat's cell drops to a second line where a row's seat does), so at mid widths the headings no longer run past the card's edge while the rows break. A table whose rows do not wrap says so on its headings: Requests `desktop:flex-nowrap` (its rows shrink from desktop), People and Pending changes `tablet:flex-nowrap`. The guard compares each line's wrap at tablet and at desktop.
      - **The "why" sort is back** (`REASON_LABELS`, `driftReason`, the key, its runs and tests), in What's heading cell, as above.
      - **`Cells` keys each box by its part's own key**, so the checkbox column arriving in select mode adds a cell instead of remounting every cell after it. `EDGE` (`RowSeats.tsx`) is the one unpainted 3px edge for Drift and Expiring access.
      - **The TrueNAS accounts table fits its line at 1440** (part 2, frame 10). Fixed widths and the wide seat summed to 1134px in a 1094px row, so Put on hold / Revoke dropped under every row and STATUS sat 300px off its cells. Account and Status now flex from a basis, 1 to 1.3 as the board draws them; the board's bases (180, 240) less 40 each, because a system with a home folder limit has three verbs, the wide seat: 1034px at 1440. "Not created by Syndra" has no column headings and a default seat (672px), so its seat already sits on the first line; unchanged.
      - **Mutation-checked** (each red): CardColumns without `flex-wrap` (280 failures); Requests' headings without `desktop:flex-nowrap` (51); the guard's wrap comparison removed; `Cells` keyed by position; the accounts table's Status back at a fixed 240 (the 1094px sum); the "why" runs removed.
- [x] 7v Flaky conformance test (add-on track; noted 18 Nov 2026, not fixed). `addons/conformance` `TestAnUnansweredWithdrawThatBoundAnAccountIsOwnedUpTo` (`kit_test.go:1312`) failed once on PR #200's CI run, with no Go change in the PR (reported by the PR's review). Find the race or ordering it depends on (`go test -race -count=200 -run TestAnUnansweredWithdrawThatBoundAnAccountIsOwnedUpTo ./...` in `addons/conformance`), fix it at the root, and leave a guard that fails without the fix. Done 21 Nov 2026. The test cancelled the run when the withdraw arrived, before the subject had applied it. Run returns on the cancel, so its read of the subject could come first and miss the bound account. The test now cancels after the withdraw has taken effect and holds the reply back, so the order is forced. The fixed test is itself the guard, because its order is now forced: the base test fails every time with a sleep after cancel.
  - A second one, 19 Nov 2026 (PR #219, run 44599163490, no Go change in the PR): `addons/truenas` `TestTheKitSkipsALimitThisAddOnDoesNotReport` failed in the conformance kit with "MUST freshness.cooldown: a fresh read inside the cooldown answered current true". It is timing under CI load. Same item: fix it at the root with the first, with a guard that fails without the fix. Not fixed here.
- [ ] 7w The craft (A-24, the owner 18 Nov 2026: "yes to all A-24 recommendations, get building. be careful and elegant"). Built from `design_handoff_syndra/reconcile-2026-10/from-design/A-24-the-craft.md` and its boards `design/a24-the-craft-1..3.dc.html`, one PR per wave to `feat/addon-contract`. Every §12 answer is as recommended; Q12 (a year of days), Q16 (the name carries you) and Q19 (the term in review) are not now and wait in `NEXT.md`.
  - [x] 7w-1 The foundation (§11's twelve, §2 quiet and warm, §8 quiet and warm, Q14, Q15).
    *Done (18 Nov 2026, branch `feat/addon-contract-a24-w1`):*
    - **A place arrives once a session, and never by key** (Q14, S11 rewritten in FINAL §4). `notePage` (`lib/motion/arrive.tsx`) decides each visit as the top bar notes it: quiet when the path has arrived before in this tab's session (`syndra:arrived`, sessionStorage), when it was asked for by key (`visitByKey`: a captured click with `detail` 0, which is Enter on a link, and every row Go to opens), or when it is a Back. `quietVisit()` settles `ArriveList` and `usePageHead`, which marks the head `data-quiet`: `page-head` tints it in and nothing after it arrives.
    - **Back gives you your row** (§11.4, §2 warm). `ArriveList` remembers the row link opened (`rowOpened`, `lib/keys.ts`, by route in sessionStorage); on a Back, `handBackRow` focuses it without scrolling, makes it the roving stop and flashes it once (`.flash`). A visit that is not a Back forgets it (`forgetOpenedRow`), so an older visit's row never flashes.
    - **Go to appears at once and leaves at once** (§8, §11.8): `Modal`'s `still` (no `settle-scrim`, no `sheet-in`, no leaving copy), worn by Go to only.
    - **Escape in two stages in every search** (§11.5): `Input` with `type="search"` clears on the first Escape through the native value setter and an `input` event, so the caller's `onChange` is the only state, and stops it there; empty, Escape goes on (Go to closes, a list leaves Select). Every search field is now `type="search"` (fourteen more, and Go to's combobox); Go to's footer says "Esc clear, then close" while something is typed.
    - **Lately in Go to** (§8 warm, Q13). `keepLately` (`lib/go-to.ts`, localStorage `syndra:lately`, paths only, twelve kept) is fed by `GoToHost` on every path; `goTo` lists, with nothing typed, the first five that this session can open and that are not the page on screen, most recent first, above Pages, as the very rows the groups below would list (keys prefixed `lately:`). Typed, the groups are A to Z again.
    - **Reduced motion keeps tint** (§11.10, Q15; FINAL §4's reduced-motion list). The reduced block restores `motion-tint`, `hover-tint` and `link-hover` transitions (each its own list, so the focus ring still appears at once) and `tint-in`/`tint-out` animations on tint. `stillTint()` (`lib/motion/still.ts`) is false under reduced motion, and `Crossfade` asks it, so a status still crossfades.
    - **Figures** (§11.1, §11.2): one rule, `font-variant-numeric: tabular-nums` on `body`; the eighteen per-part `tabular-nums` are gone. `CrossfadeDigits` crossfades only the digits that changed, seated from the right; `Count` uses it.
    - **The rest of §11:** `scrollbar-gutter: stable` on `#app-scroll` (§11.3); `overscroll-contain` on Modal's panel, the touch navigation sheet and FilterPanel's sheet (§11.6); `caret-color: var(--accent)` in every field (§11.7); a second tooltip is instant (`useWarmTip`, `lib/motion/tip.ts`: once one is open or shut within 300ms, Term and WaitsMark open without their entrance; §11.11).
    - **A number opens its list** (§10 warm, §11.12), only where the list exists and has something in it: Trends' Drift waiting (Drift) and People who left and still hold access (People, departed), each when above zero, and every Held but not used role (People, narrowed to the role); a system overview's per-person figure (`metric.per_subject`) to Insights' Usage. Through `AdvancedLink`, so Basic switches view where the list is Advanced's.
    - **Device memory** is one helper, `lib/memory.ts` (`recall`, `remember`, `recallPaths`, each guarded); `place.ts`, `arrive.tsx`, `keys.ts` and `go-to.ts` all read it. Paths only, never read for access.
    - **§13 answers** written into A-24 §13: both faces carry `tnum`; FilterBar's Escape did nothing before this.
    - **Guards** (`src/__tests__/craft-foundation.test.tsx`, plus `trendsTermLink.test.tsx` and `UsageCard.test.tsx`): first visit arrives, later and by-key visits are quiet, pointer clicks are not; the quiet head tints and the CSS stages nothing after it; a quiet visit's list is settled; Back hands back the opened row (focus, current, flash) and never an older visit's; Go to has no scrim fade, no rise and no leaving copy; Escape clears then closes in Go to, and in a list's search stops at the field; every search field is `type="search"` (source scan); Lately's order, exclusion, cap, unique keys and memory; the reduced block keeps each tint utility; Crossfade fades under reduced motion; tabular-nums on the body and nowhere else; 47 to 46 fades only the 7; the gutter, the overscroll, the caret; the second tooltip is warm and a cold one is not; figure links only where a list exists.
    - **Mutation-checked** (each red): the arrived memory dropped; `detail` 0 not read; Go to's Modal not still; `clearOnEscape` off; Lately's `here` filter off; Crossfade back on `stillMotion`; the reduced block's `.tint-in` rule gone; `CrossfadeDigits` keyed by position from the left on the whole text; warmth never set; `handBackRow` not called; `forgetOpenedRow` not called; the `per_subject` check dropped; `Fact`'s link off; one search field's `type="search"` removed; the body's `tabular-nums` gone.
    - *Review round (Opus, before the PR):* **(1)** Modal's Escape listener now skips an Escape already spent (`defaultPrevented`): in the app the React root is the document, beside that listener, so the search's `stopPropagation` alone did not keep Go to from closing on its first Escape. Guard: a spent Escape dispatched on the document leaves Go to open (red without the check). **(2)** `pageSettled()` is unchanged from before (a quiet visit's Notices in the first render stay still, as on a Back). **(3)** `CrossfadeDigits`' live digits are plain inline text and only the leaving digit is laid over its seat (`absolute`, `aria-hidden`, `select-none`), so the figure copies and reads as one number; a grid seat per digit was a block per digit. Guard: the leaving layer's classes and no grid inside a figure (red with the overlay back in the flow). **(4)** A handed-back row is forgotten once handed back, so a reload does not flash it again. **(5)** Go to's Esc hint follows what Escape clears (any text, spaces included).
    - *Fix-and-refine round (PR #210, the lead's review, 18 Nov 2026):* **(1)** A system's usage figure opens `insightsUsageHref(target)` (its own anchor on Insights' Usage), derived in `UsageCard` from `target` as its End row already was; the prop and the hardcoded `/insights?tab=usage` are gone. **(2)** Only a plain primary click remembers a row (`ArriveList`): a modified or middle click opened it elsewhere, so a reload never flashes it. **(3)** Tabular figures are on figures only: one `[data-figure]` rule (the body's is gone, prose keeps proportional figures), carried by `Time`'s parts (now `<time dateTime>`), `Count`, every number seat with a value, `StatCard`, the stat face and `CrossfadeDigits`, and the eighteen former `tabular-nums` elements. **(4)** `CrossfadeDigits` is its plain text at rest and splits into digits only for the one tint a digit fades (one timer, the previous text, the old text overlaid from the right, `aria-hidden`, `select-none`); `Digit` is gone. **(5)** The row memory is place's: `keepPlace("row")`/`recallPlace("row")`, null forgets (`remember` removes on null). Back detection (`returnedTo`, `returnedToThisPage`) moved into `place.ts`, which `arrive.tsx` reads, so neither imports the other back; `keys.ts` is as it was. `flashOnce` (`lib/motion/flash.ts`) is the one wash, used by the hand-back and by glide's landing. **(6)** Sign-out forgets Lately (`forgetLately`, `AccountSheet`), so a shared computer shows the next person nothing. **(7)** Lately shows on the phone, above the touch sheet's own list, by the same `GoToPanel` rows. **(8)** Each tint utility names its duration list once (`--tint-durations`); the reduced block reads it back in one rule. **(9)** Tidies: `Fact`'s value is `string | number`; `GO_TO_ESC` is private; `place.ts` has no aliases. **(10)** One label per Go to row (`go-to.ts`): a top-level page is "Page" with no line under it, a page in a group has its group, a bundle says its roles ("2 roles"), an app with no project has no line.
      - **Guards** (each red when its rule is broken): a modified or middle click flashes nothing on return; a handed-back row does not flash on the reload after; `font-variant-numeric` is declared once, on `[data-figure]`, never on body, and no part writes `tabular-nums`; Time, a count and StatCard carry `data-figure`; the figure is plain text at rest, split only while fading, and plain again after one tint; sign-out removes `syndra:lately` (`AccountSheet.test.tsx`); the phone's Go to lists Lately and keeps its own list; no Go to row says "A page", a bundle says its roles; each tint utility's `--tint-durations` has one entry per property and ends on `--ring-fade`, and the reduced block reads it; the usage figure opens its system's anchor. `TechnicalDetail.test.tsx` now remounts after a real Back, not a mock, and `connected-systems.test.tsx`'s matchers allow the `<time>` element.
    - **Not built, and why:** the band's last counted day in full ink (§10 warm) — it already carries its dot; a list's search has nothing of its own to close, so its second Escape goes on to the page (Select, a dialog). A person's page and a member's page keep plain figures: Insights' Usage is Advanced's and operators' only.
  - [x] 7w-2 Finishing and trust (§4.1 warm and vivid on Home only, §5 the trail, §6 the wait, §7; plates prepared, hidden until the owner sees them, Q9).
    *Done (18 Nov 2026, branch `feat/addon-contract-a24-w2`), after tracing every surface a result touches (A-24 §13, answered there):*
    - **The change's trail** (§5, Q8). `ChangeTrail` and its pure `trailSteps` (`components/people/ChangeTrail.tsx`), drawn by `CheckTrail` with a new `label` (the list is named "On its way to Zitadel" and each step reads "Step n:", no pass or fail word). Under a give's result (`GrantDirectAccess`) and a direct revoke's (`RemovalDialog`). Each step settles only on what was observed: Recorded in Syndra (the act answered; its rows are written before it returns), Sent to Zitadel (the answer's `status` applied with nothing waiting, or a later read's standing `sent`/`held`, a give only), In Zitadel / No longer in Zitadel (only a Zitadel read made after the act: a fetch of the person landed after the answer, `dataUpdatedAt` ≥ it, the browser's clock both sides, carrying an `observation.read_at` newer than the one the person's access held when the trail appeared, the server's clock both sides; standing `held` / `not_held`; "Read back at HH:MM"). Open steps are rings and say what is known ("Not sent to Zitadel yet.", "Not in Zitadel yet." / "Not removed from Zitadel yet.", "Not read back from Zitadel yet."). Steps fill on tint (the dot's own transition). A give draws it only when it queued its own row (`outbox_id`: with none, the answer's "applied" means only that nothing waits); a revoke only when it queued one (`status`) and this role is not in `retained_roles`. **Not drawn** where a stage cannot be observed: a refused give or revoke; a revoke that queued nothing or that a bundle or rule still covers; request decisions (their answer has no `status`, only the absence of a wait reason, and the Requests screen holds no read of the person); bulk and plan applies (per-row effect, no read back); extend, acknowledge and lift (no Zitadel step at all); Send (counts). The row's own detail line keeps its standing words (`standingWords`), the same fact.
    - **A long wait says so once** (§6). `useLongWait` (`lib/useLongWait.ts`, `LONG_WAIT_MS` 10s). Check now on a system's page says "Still reading {system}." on tint past ten seconds, in its sentence's seat. The member's password set uses the same hook at its own 4s (`SLOW_ANSWER_MS`); its `useSlow` is gone. "You can leave this page…" is built nowhere: Check now's result does not survive leaving (§13).
    - **The ending of a queue** (§4.1 warm). Every queue already lifted and bloomed by `useQueueCleared`; Home's all-clear did neither. It now lifts when Home empties in front of the reader and blooms when their own act emptied it (`useQueueCleared(calm ? NO_WORK : WORK, ready, true)`, `ownActWithin` 30s). **Vivid on Home's all-clear only:** `EmptyState closes` (the sentence at `type-section-title`, a lime `close-line` hairline drawn by `hairline-draw` over one flash, then at rest; drawn at once under reduced motion), only by the reader's own hand and once a day on this device (`closesToday`, `syndra:home-closed`, the reader's own clock as the greeting's).
    - **The ledger line is not built.** No read gives one operator's acts today and how many are in Zitadel (§13); the smallest read is proposed there (`GET /me/today`) and waits for the owner's word.
    - **Plates** (§7, Q9). `Plate` (shut, open) in `components/states`, drawn in CSS from tokens (`@utility plate`), `aria-hidden`; `EmptyState plate` draws one in the right seat only when `PLATES_SHOWN`, which is false until the owner has seen them on staging (wave 4). Wired where the board draws them: Home's all-clear and Drift's clear queue (shut), People's first run (open); a search that missed has none.
    - **Guards** (`src/__tests__/craft-finishing.test.tsx`, plus `GrantDirectAccess.test.tsx`, `RemovalDialog.test.tsx`, `HaltedReconcile.test.tsx`, `HomeHandover.test.tsx`): the trail's truth table for both acts (only `held` settles In Zitadel, only `not_held` settles No longer, a give's `sent` never settles a revoke); a fetch after the act carrying an older Zitadel read settles nothing, a newer Zitadel read does, and with nothing read before nothing settles; a give with no row of its own and a revoke that kept this role draw none, a revoke that kept another still does; a switch of view is not an emptying; a settled long wait is never "still waiting" for a render; the dialogs draw the trail for a sent and a waiting act and none for a refused give or a revoke that queued nothing or was retained; a labelled CheckTrail reads Step n and an unlabelled one still Check n, passes; Check now says "Still reading" only after ten seconds and never that you can leave; Home's all-clear is still on arrival, lifts without bloom when a read empties it, blooms and closes when the reader did, and closes once a day; the close-line's keyframe and colour; plates hidden while `PLATES_SHOWN` is false and drawn from tokens.
    - **Mutation-checked** (each red): In Zitadel settled on `sent`; the read's age ignored; Sent always settled; a revoke settled by `sent`; a refused give given a trail; a trail with nothing queued; the Zitadel-read gate off; a give's trail without a row; no view scope; the trail hidden when any role was kept; the long wait one render late; Check now's long wait off; closing every time; bloom on any emptying; no lift; plates shown; the route read as checks.
    - *Review round (Opus, before the PR):* **(1)** The last step could settle on a Zitadel read older than the act, because a fetch made after it still carries the last Zitadel read (`observation.read_at`), and `standingOf` says `not_held` for anything that read did not see: a revoke still waiting could have read "No longer in Zitadel". Now only a Zitadel read newer than the one held when the trail appeared counts, and with nothing held then, none does. **(2)** Switching Advanced to Basic emptied Home's blocks and read as the reader emptying it (a lift, a bloom, the day's close). `useQueueCleared` takes a `scope` (Home passes its view); a change of scope takes what is there as seen. **(3)** A give with no row of its own answers "applied" (`actResult`), so the give's trail now needs `outbox_id`. **(4)** A revoke's trail hid when the cascade kept any other role; now only when it kept this one. **(5)** `useLongWait` said "still waiting" for one render after settling; it returns `pending && long`. **(6)** The landing bloom is gone: nothing re-reads the person while the sheet is open, so it could almost never play, and the step fills on tint without it.
    - **Found while tracing, fixed in the fix-and-refine round below.** The "what stays" line before a revoke is true on every path traced but changes a consent, so it waits for the owner (NEXT 0l).
    - *Fix-and-refine round (PR #211, the lead's review, 18 Nov 2026):*
      - **(1)** A Zitadel read that did not complete (`observation.current` false) settles nothing (`ChangeTrail`).
      - **(2)** Home's close is decided in render (`closesOnce`, keeping `{ day, at }`, the clearing's own time, so a second render or strict mode's answers the same): the title is drawn at its size from the first frame, never resized after paint.
      - **(3)** While its last step is open the trail reads the person again every 15s (`TRAIL_REREAD_MS`) through the page's own query (`refetch`), for at most 6 minutes after the act (`TRAIL_WATCH_MS`), and stops when the step settles or the sheet closes.
      - **(4)** One rule: `changeTrailFor(status, wait_reason)` (no trail without the act's own row, on a failure or a refusal), used by both dialogs; `withTrail` is gone.
      - **(5)** Backend: a direct revoke's `status` is this role's own revoke row's (`removeGrantPlanned`, `ownRow`), never the last row sent in a cascade; "pending" when its own row did not go. `TestHandleDeleteUserDirectGrant_StatusIsTheRolesOwnRow`; the kept-role test now expects "pending", not a sibling's "failed".
      - **(6)** The copy guide's wave 2 table follows the wave 1 rows.
      - **(7)** `StatusDot` carries `data-ring`, which the tests read instead of a class; the long-check test resets in `afterEach`; the Home tests clear device memory first and close again the next day (`vi.setSystemTime`).
      - **(8)** The trail is named "On its way to Zitadel".
      - **(9) Nothing says what doesn't happen.** A direct revoke Zitadel took says "Nothing else was supplying it, and Zitadel took the revocation. It has not been read back yet." (and "…The revocation is not in Zitadel yet." when it was not taken), never "the access is gone" (`removalOutcome`). A revoke on a system's page is **On its way** (queued, sends itself) while its lock is queued, never Revoked (`RevokeDialog`). Copy guide §18 rows for both. Traced: `removalOutcome` has one caller; the RevokeDialog's lede and What happens list are forecasts and stay; the backend's `revokeCopy` and disclosure already said the lock drains on its own; no audit word changes.
      - **Mutation-checked** (each red): `current` ignored; the close back in an effect; no re-read; no stop on settle; no watch cap; no stop on unmount; the rule's row check dropped; the backend back on the last row sent; "the access is gone" back; Revoked for a queued lock.
    - **Deferred, not built** (A-24 §5): the trail as a row's own detail line, until a role is held (the row keeps its standing words); a fourth step for a role that reaches a connected system, in that system's own word; the bloom on the last step when it lands on screen.
  - [x] 7w-3 Places that know (§3.1 the light and the term line, Q5 the mark, §9 the line that knows and access over time in Advanced, Q18 the storage sentence). *Access over time waited on a read (below); built as 7w-5.*
    *Done (19 Nov 2026, branch `feat/addon-contract-a24-w3`):*
    - **The room knows the hour** (§3.1, Q2). `hour-light` (globals.css) behind Home's head, placed by `dayPhase` (`lib/greeting.ts`), which `greeting` now reads too, so the word and the light share one clock and cannot disagree. Four places from tokens (`--hour-shape`, `--hour-reach`) and a weight per room (`--hour-weight`: 1 dark, 0.75 light). The one written exception to "colour is never decorative" (globals.css header, RULES, FINAL §4) with its bounds: Home's head only, still, the accent only, both rooms, off under forced colours, and no line in the head under 4.5:1 at the pool's strongest, over the canvas the head sits on (dark ink 13.73, muted 7.43, faint 5.29, accent text 4.79; light 12.08, 6.77, 4.75, 4.74; unlit 17.85, 8.88, 5.97, 6.23 and 15.36, 7.89, 5.28, 6.02).
    - **The term line** (§3.1, Q3). `TermLine` (`components/home/TermLine.tsx`): the term holding today (`termHolding`, `termName` in `lib/term-dates.ts`), from the terms setting Home already reads; days gone, today a violet point, an amber mark on each day a direct access ends, from Ending soon's own read (`expiring_grants`, 14 days, the makerspace's days). Its words are only the term's day and end; its name for a screen reader says the rest ("Monsoon term, day 110 of 153. Access ends for two people on 24 Nov and 27 Nov."). In an `Unfold`, so its read landing never jumps the head.
    - **The mark says the room** (Q5). `SyndraMark` `room` (`roomOf`): from the rail's own reading, operators only. Amber and breathing while `zitadel_reachable` is false; violet while any count the reading carries waits; as built otherwise, and with no real reading. Lime and dim are drawn and wait for a complete reading (review round below). The tokens it already has (`--mark-dot`, `--mark-glow`), set per `data-room` in globals.css.
    - **The line that knows** (§9, Q10). `personLine` (`components/people/personLine.ts`) in the person page's lede seat, where a sentence said the same thing for everyone: "Holds 2 roles across 2 projects." from the standings of Syndra's read of Zitadel (the answer each row's standing words come from), then the one thing worth knowing in My access's order: a held role ending within 14 days, a given role not in Zitadel, one on its way. A missing, failed (`current` false) or cut-short read counts nothing and says so. `headerLine` shares `countWord`, `inDays` and `ENDING_WITHIN_DAYS`.
    - **One sentence under the storage meter** (Q18). `roomLine` (`kinds/storage/captions.ts`): "Plenty of room." under half (`PLENTY_BELOW`), "{left} left." up to the meter's own nearly full, "Getting full: {left} left." past it, nothing at the limit (the card's Full line says it). From the same read as the meter; member's card only.
    - *Review round (Opus, before the PR):* **(1)** The mark's lime ("nothing waits") could show while Home's headline counted something: the rail's reading carries the queue counts (requests, ending soon, pending changes, drift, unfinished revocations, holds, retries, gave up, unconfirmed operations; now all counted, whichever view, `needsYou`) but not every one of Home's blocks (onboarding gaps, owner-less service accounts, merge findings, unvouched). So the mark now says **only what it can prove**: violet when a count waits, amber and breathing while Zitadel isn't answering, and otherwise as built. Lime and dim are drawn and wired (`roomOf` `complete`) and wait for a reading that carries every Home block (`HOME_COUNTS_IN_INDICATORS`, NEXT 0l). **(2)** The term line opened into Home on every visit (the clock and the terms land after the first render): it mounts only once both are known, in an Unfold already open, and Home's clock is set before paint. **(3)** The light reached 10px past a phone's 16px gutter (a sideways scroll): its inset is the phone gutter below the tablet step. **(4)** The contrast test measured over `--ground`; the head sits on `--canvas`. It now does, and checks `--accent-text` (the head's links on hover) too. **(5)** A tautological test (the greeting against itself) now pins the hours: 4 quiet, 5 and 11 morning, 12 and 16 afternoon, 17 and 21 evening, 22 quiet. **(6)** The term line's end is in its spoken name. Refinements: one clock for the rail and the light (`useDayPhase`, `lib/useDayPhase.ts`, asked each minute, re-rendering only on a change), so both turn at 22:00; the person line counts days with `daysUntil` as My access does, and says "hasn't answered yet" for a person never read; a one-day term is day 1 of 1; MyStorage's meter and its sentence share one condition. Kept: the date line's "First day of Monsoon term" on the two days it shows, beside the term line's count (A-18's own words).
    - *Fix-and-refine round (PR #212, the lead's review and staging in Chrome, 19 Nov 2026):*
      - **(1) The clock is the makerspace's.** `wallHour` (`lib/time.ts`, the configured zone's wall clock) feeds `isQuietHour` and `dayPhase`; `now.getHours()` (the browser's) is gone, per the standing rule of 14 Nov. The "which clock" item left NEXT. Guard: 20:00Z is evening in UTC and quiet hours in Kolkata (red on the old code on any machine's clock).
      - **(2) A role held twice never "ends".** The person line skips a role two sources give (`orderedSources(...).length > 1`, as `findMultiSource` decides): a direct grant's end is not a bundle's. Guard: direct grant plus bundle.
      - **(3) No jump on a cold load.** The term line's Unfold mounts with the clock, not with the terms read, so terms that land late unfold. Guard: terms arriving late open ("opening"), warm reads are there at once ("open").
      - **(4) The person line counts as the People row does** (`observedHolding`, its parameter widened to the two fields it reads). With no count it says `CHECKING` ("Checking what Zitadel shows.", exported) and the page's read warning owns the failure; a given role missing is "Not in Zitadel: {role}.", the standing's word. (The "hasn't answered yet" sentence is gone with it, so the copy guide carries CHECKING instead.)
      - **(5) One phase on Home.** The greeting's word (`PHASE_WORD`), the quiet-hours sentence and the light all read `useDayPhase`, so a Home left open across a boundary turns all three together. 00:30 in the makerspace is quiet hours, and quiet hours greet "Evening" (A-24 frame 4, greeting.ts since A-18); the copy guide's greeting row now says so, with the zone.
      - **(6) Dead rail states cut.** The rail's mark says only what its reading proves: `waiting` (amber, breathing) while Zitadel isn't answering, for an operator; nothing else. `needsYou`, `roomOf`, `HOME_COUNTS_IN_INDICATORS`, the clear and quiet CSS and the rail's clock are gone; NEXT 0l keeps the read that would bring lime and dim back.
      - **(7)** The storage sentence is "Plenty of room." or "{left} left.": the meter already says nearly full.
      - **(8)** The term line's words are plain text; only its track is a picture, named by the one sentence it adds, and only when something ends on it.
      - **(10) No edge to the light.** Its pool is masked to nothing on all four sides inside its own box (`mask-image`, two linear fades, `mask-composite: intersect`), so where a place puts it past the box, no line shows, in either room.
      - **(11) The term line sits with the head's lines.** Same row as Claims and Checked (`flex items-center gap-x-2`, a StatusDot first, so the dot and its gap are `--dot-indent`), text-small; the track is a 3px hairline from `--tint-3` with the days gone in `--line-strong`, today a dot on it and ending days in amber, visible in both rooms.
      - **(12) The header's actions in their seat (D-1), found on staging at 1481px.** `/users/<id>` put "Manage bundles" and "Give a role" at x≈686, right after "Holds no roles.": A-20 (`9384144f`) wrapped PageHeader in a root for the Notices and left the caller's `className` on the inner row, so the person page's `w-full flex-1` no longer sized the box beside the avatar, which shrank to its text. `className` sizes PageHeader's root again; the row fills it, the text block grows (`flex-auto`) and the actions end it, so their seat never depends on the text. The person page is the only caller with a `className`; a phone still stacks (the avatar row is a column, the actions wrap under the text). Guard (`PageHeader.test.tsx`): the root carries the caller's sizing, the row is a full-width flex with nothing narrowing it, the text block grows, the actions are last. Mutation-checked (each red): `className` back on the inner row; the text block not growing; the row `inline-flex`.
      - Test helper: `inZone` (`test-utils/zone.ts`) picks a zone and the tests put the file's back after.
      - **Mutation-checked** (each red): `getHours` back; the held-twice skip dropped; the terms gate back; `observedHolding` bypassed (records counted); "Not in Zitadel" reworded; the greeting from its own clock; the mark amber for a member; the mask removed; the track named with nothing on it.
    - **Access over time is not built.** No read gives one person's roles over a year: the audit read is capped at 200 a page, mixes actor and target, names bundle assignments by bundle not role, has no rule-given roles, and nothing marks Zitadel-held roles Syndra did not give; revoked direct grants are deleted rows. Proposed, read-only, waiting for the owner's word (NEXT 0l): `GET /api/v1/users/{id}/access-history?since=` (operator, Advanced), `[{project_id, role_key, given_at, ended_at, ended_how, source}]` from `audit_logs` (the `direct_grant.*` and `bundle.*` rows for the person) and `direct_role_grants`, bundle roles resolved at assignment.
    - **Not built, and why:** the held-twice popover sentence (§9 warm: which source decides) is a separate change to `AccessSource`; the term line's per-day hover (the board's "as the usage band does") waits for the band's readout to be shared; the mark on the phone's top bar stays as built (the rail is where the room is read).
    - **Guards** (`src/__tests__/craft-places.test.tsx`, `Sidebar.test.tsx`, `HomeHandover.test.tsx`, `PersonHeaderId.test.tsx`, `MyStorage.test.tsx`): the greeting and the light agree at every hour; four places, still, accent only, off under forced colours; the head's text at or above 4.5:1 lit, in both themes, and under a quarter strength on paper; Home lights its head for the hour and the lamp in quiet hours; the term line's marks, words and once-per-person count, nothing past the term, and none between terms on Home; the mark violet for any count, amber and breathing, never lime or dim from an incomplete reading, as built without a reading or for a member; `roomOf`'s full table and `needsYou`'s counts; the person line says less on a missing, failed or cut-short read, counts only standings, and its ladder; the room line's thresholds and its seat under the meter; the person page opens on the line.
    - **Mutation-checked** (each red): the light fixed to one place; forced colours left on; a stronger pool; full weight on paper; people counted per row; ends past the term; Zitadel said after the counts; retries not counted; lime from an incomplete reading; the term line opened into Home; no phone inset; a one-day term miscounted; its end not spoken; "hasn't answered yet" dropped; records in flight counted as held; a failed read counted; ends of roles not held; plenty up to 90%; the old lede back; the room line gone; the term line gone from Home.
  - [ ] 7w-4 Ceremony (§3.3 the lit card, §3.2 the first arrival, Q9 the plates), each judged on staging by the owner before merge.
    - *Built (wave 4, `feat/addon-contract-a24-w4`, for the owner to judge on staging before merge):*
    - **The first arrival of a sign-in** (§3.2, Q4). The session carries the instant it was signed in (`signedInAt`, stamped where each kind of sign-in becomes a session, the dev identity picker's included: round (10) below). `lib/motion/door.ts`: the shell notes the sign-in's first page on every page (`noteSignIn`, AppShell's layout effect), Home asks once as it mounts (`takeFirstArrival`), and the answer is yes only when Home is that page, the sign-in is new to this device (`firstTime`, localStorage, the instant only) and nobody has asked yet. Home then draws a doorway's arch (`hour-light[data-arch]`, carrying the current part of the day, `door-open` on settle's curve over `--duration-arrive`) where this hour's pool is, widening into it, and fades the pool up behind it (`[data-door]`, `fade`): round (9) below. It keeps the pool's mask so it has no edge either; its width, height and place are registered properties, so it widens rather than jumps; nothing transforms or travels; nothing waits for it (the words, lines and blocks paint as any first visit). Reduced motion: no arch, the pool at rest. The arch is 30% accent (the board's 44% would take accent text under 4.5:1 and the paper light over a quarter); the contrast test now holds it with the four parts of the day.
    - **The lit card** (§3.3, Q6). On My access, a project whose role was given or approved today (the makerspace's day, `isSameDay`; it was the browser's `toDateString`, fixed here, which also moves the line's "From today" rung onto the makerspace's day) and that Zitadel holds is lit on the first visit: `lit-card` (the same light at the same weight per room, a violet hairline on the top edge, clipped by the card, gone under forced colours), "From today" as its note, in its alphabetical place. Decided once per visit on that visit's own answers from Zitadel and the feed; only the starts Zitadel holds are spent then (`syndra:lit`, `<id hash>:<project>:<instant>`, newest 50). Never on a failed read, a role on its way or one not held; a device that can't remember lights nothing. Lit reach 22% (30% took dark accent text to 4.13:1 on the card's surface).
    - **Plates** (§7, Q9). `PLATES_SHOWN` is on: the shut door on Home's all-clear and Drift's clear queue, the open door on People's first run, none for a missed search. EmptyState's own comment now names the exception.
    - **One helper for once** (`lib/memory.ts`): `firstTime(kind, key, mark, keep)` remembers a mark and answers true only if the store kept it (read back), so a store that can't keep it never repeats a moment; `idHash` (FNV-1a, 32 bits) keeps an id as a hash.
    - **Docs:** FINAL §4 (the exception's three places, the first arrival, the lit card, plates), RULES (the exception, with the new contrast numbers), the copy guide (Wave 4 rows), NEXT 0l (wave 4 on staging; the access token in the client payload, found here, `needs-triage`).
    - **Guards** (`craft-ceremony.test.tsx`, `MemberAccessLit.test.tsx`, `HomeHandover.test.tsx` "the first arrival of a sign-in", `craft-places.test.tsx`, `craft-finishing.test.tsx`, `session.test.ts`). **Mutation-checked** (each red): `firstTime` without its read-back; the door's path check off; the arrival not spent when asked; Home's door ungated from the sign-in; the arch drawn always; the shell not noting the sign-in; the lit card not kept to once; its mark without the member; the raw id kept; the browser's day for today; the Zitadel read gate off; the feed gate off; the held check off; a waiting role counted; the arch shown under reduced motion; the doorway with its own mask; `door-open` with a transform; lit reach back at 30%; plates off; a plate for a missed search; the card without its note.
    - *Fix-and-refine round (PR #213, the lead's review, 19 Nov 2026). Checked locally against the built UI: a dev identity sign-in lands on Home with `signedInAt` in the session the page receives; the door then plays once, as the Home tests hold:*
      - **(1) Only fresh answers decide the lit card.** The gate is `isFetchedAfterMount` and no error on both the access read (with Zitadel read) and the feed, so a cached answer never spends a start and a failed one never decides. Guards: cached access, cached feed, a failed access read with data still in hand.
      - **(2)** FINAL §4: the mark and the mask sentences are back at the end of the hour's-light paragraph.
      - **(3, 4) Plates at true size.** 1px arch and floor, an 8px point at -4px, gone under forced colours. Open reads as open: the floor breaks at the doorway's jambs and the light inside spills through onto the ground in front (`.plate-floor::after`, the accent at 40%).
      - **(5)** One marker for a lit card: `isLit`, and the class (`data-lit` dropped).
      - **(6)** Only held starts are spent: the comment, FINAL and this record say so.
      - **(7)** `door.ts` is imported from its own module everywhere (`resetDoorForTests` is not in the motion index).
      - **(8)** The shell's guard is a loose match on `noteSignIn(session.signedInAt)`.
      - **(9) The arch rises where the pool is.** The pool's place and size are tokens (`--hour-x`, `--hour-y`, `--hour-w`, `--hour-h`, replacing `--hour-shape`); the arch carries the current part of the day (`data-hour={phase} data-arch`), stands at the pool's `--hour-x`, and `door-open` ends at the pool's own size and height (`--door-w/h/y` to `var(--hour-w/h/y)`), so in the evening it rises at the right and widens into the evening's pool.
      - **(10) The dev identity picker's sign-in opens the door too.** `signedInAt` is stamped in the session factories (`createSessionValue` and `createOidcSessionValue`), the one place each kind of sign-in becomes a session; the callback no longer sets it. A demo session carries it through `getSession` on a copy of the catalogue's user.
      - **(11)** `PLATES_SHOWN` stays until the owner decides Q9; then it is deleted, with the plates or with the constant, whichever he chooses.
      - **Mutation-checked** (each red): `isFetchedAfterMount` dropped on either read; the access error check dropped; the arch at a fixed place; `door-open` ending at a fixed size; the arch without the current part of the day; the open floor unbroken; the spill removed; the point back at 7px; the 1.5px arch; plates shown under forced colours; the demo stamp removed; the demo stamp not carried through `getSession`; the OIDC stamp removed; the shell guard's call removed.
    - *Owner on staging (19 Nov 2026): "i am not able to see the door animation at login. the page transitions are also happening only for the first time i open a page."*
      - **(A) Pages arrive on every visit (reverses Q14).** `notePage` keeps only the page and when it was shown; the per-session memory (`syndra:arrived`), `visitByKey`, the Enter-on-a-link listener, `quietVisit` and the page head's `data-quiet` are gone. Every forward navigation arrives as built (link, Go to, Enter); Back, Forward and a reload stay still (`returnedToThisPage`), as do a filter, a sort and the Basic/Advanced switch; the returned row's flash and focus are unchanged. FINAL §4 S11 rewritten; A-24 §12 Q14 notes the reversal. Guards (`craft-foundation.test.tsx`): a second visit arrives (header and list), a visit by key arrives, Back is still, no memory and no by-key mark in the source, no quiet rule in the CSS. Mutation-checked: the once-a-session code restored (three red); a per-session memory in the page head; Back not still.
      - **(B) You walk through the door.** The in-head arch (`data-arch`, `door-open`, `--door-w/h/y`, the pool's `data-door` fade) is gone, and the hour light is still again. In its place, `DoorThrough` (AppShell, outside `#app-root`): a full-window, `aria-hidden`, `pointer-events: none` layer in the sign-in door's own look (its room `--door-page`, its arch `--door-arch-*` with `--door-stroke` and `--door-arch-mask`, its violet `--door-fill`), one keyframe set (`door-through-room`, `-arch`, `-light`) over `--door-through` (900ms). 0 to 14%: the room whole, the arch and its light inside, widening. 14% to 28% (the tint before `--door-through-reveal`, 250ms): the room and the arch dissolve as the arch widens past you, and the light falls to 0.15 of `--door-through-light` (60%). From 28%: the page tints in (`#app-root[data-door]`) under a 9% glow that widens and dissolves by 900ms. Only opacity and size change. It plays on the sign-in's first page, whatever it is (`firstArrival`, asked once a page load by the shell, kept by `firstTime` to the sign-in's `signedInAt`); reduced motion or forced colours: no layer and no wait. Contrast: before the reveal the page is not shown; from it, the glow keeps every text token at 4.5:1 or more on canvas, surface and rail in both rooms (RULES). Checked in a local build (Chrome, every animation held at 0, 125, 250, 500 and 900ms, both themes; and at playbackRate 0.2, where the layer took itself away after 4.5s).
      - **Guards** (`AppShell.test.tsx` "walking through the door"; `craft-ceremony.test.tsx`): it opens on any first page over a page that is there and held for the reveal, outside `#app-root`; it leaves when its light ends; once a sign-in, again for the next; once a page load; nothing when the device can't remember; no layer under reduced motion, with the arrival spent; none without `signedInAt`; only opacity and scale; the stops on the tokens; a moment, not a haze (the room, arch and light whole until 14%, the light's heart at 50% or more, the room and arch gone at the reveal); the door's own tokens, no literal colour; no layer and no wait under reduced motion or forced colours; 4.5:1 from the reveal on. **Mutation-checked** (each red): the layer under reduced motion (JS and CSS); a second ask in a page load opening it; no device memory; the layer never leaving; the page not held for the reveal; the layer inside `#app-root`; the room held past the reveal; the light at 0.3 over the page; the page tinting in at once; the arch travelling; the light's heart back at 30%.
    - *Longer, so it reads as a door (lead on staging at 0.1×, 19 Nov 2026: plainly visible, but at real speed the bold part was a 250ms flash).* `--door-through-reveal` 550ms and `--door-through` 1300ms; the stops follow (33% the tint before the reveal, 42% the reveal). The arch and its light now grow in one stroke (a size only at 0% and 100%), so the growth never holds or pauses at a stop, and every part runs on `--ease-settle`. Every other rule as before: once a sign-in, clicks pass through, no layer under reduced motion or forced colours, 4.5:1 from the reveal (0.15 of the light, unchanged). Guards: the reveal at 500ms or more; the stops on the tokens; one growth segment per part; the settle curve on all three. **Mutation-checked** (each red): the reveal back at 250ms; a stop off its token; an intermediate size on the arch (a pause); the room on `linear`; the light at 0.3 after the reveal.
    - *Owner on the full-window door (19 Nov 2026): "this looks really unelegant. what happened to the light traveling left to right on the greeting."* The full-window door is gone (`DoorThrough`, its AppShell wiring, `--door-through*`, its keyframes, its tests and docs); the walk-through records above are history. Part (A), arriving on every visit, stays. The header door of `3f4fdc32` is back (`takeFirstArrival` on Home, `noteSignIn` in AppShell), now as the light travelling the greeting: `greeting-light`, a soft band inside the greeting's own box (`relative w-fit`), its gradient's position 0% to 100% in one stroke (reading direction), over `--door-light` (1200ms) on `--ease-settle`, handing over to the pool (`door-pool` on `[data-door]`) on the same stops from 65%. Strength `--door-light-reach` 30% at the room's weight, wide (60% of the greeting) and soft; its brightest composite keeps every token at 4.5:1 (dark accent text 4.53, light faint 4.66; RULES). Reduced motion or forced colours: no band, the pool at rest. Guards (`craft-ceremony.test.tsx` "the door's light travels the greeting", `HomeHandover.test.tsx` "the first arrival of a sign-in", `craft-places.test.tsx`): no full-window layer; reading direction in one stroke; only opacity and position; 1100 to 1300ms on the settle curve; the handover stops; reduced motion and forced colours; 4.5:1; once a sign-in, spent by the first page, again for the next, nothing when the device can't remember, none without `signedInAt`. **Mutation-checked** (each red): the band right to left; a full-window band; the band at 40% (contrast); a haze at 12%; the pool's handover off its stop; the band hurried to 500ms; a transform in its keyframes; the band under reduced motion; Home lighting it for any signed-in session (once); the greeting without its own box.
      - **The plates, drawn as the sign-in door** (owner, 19 Nov 2026: "make the drawings closer to how the login arch is"). The login arch's proportion and top (48 by 44, radius 21; a test holds both to `--door-arch-w/h/r` within a pixel), its violet hairline (`--door-ring-base` shut, `--door-stroke` open), its uprights dissolving (`--door-arch-mask`), its light inside (`--door-pool`, `--door-pool-edge`, new tokens that the sign-in screen's `.login-pool` now reads too: one light). Shut: dim, a lime point on an unbroken floor. Open: the login's bright pool, the floor broken at the jambs and the light spilling through, a violet point. 1px lines and an 8px point, both rooms, gone under forced colours. **Mutation-checked** (each red): a grey arch; off the login's proportion; the uprights not dissolving; the open door without the login's pool; the login's pool a second source again; the open floor unbroken; plates under forced colours; a 1.5px line.
    - *An even crossing (lead, 19 Nov 2026: the settle curve front-loaded the band into a ~300ms flick).* The travel is its own animation, `greeting-travel`, on `--ease-tint` (the existing symmetric ease-in-out) over three quarters of `--door-light` (900ms), then it rests at the end of the words; the light's opacity stays on settle and hands over to the pool as before. Guard: the crossing's curve and length, and no position in the light's keyframes. **Mutation-checked** (each red): the travel on settle; the travel over the whole 1200ms; the position back in the light's keyframes.
    - *Owner on the greeting band (19 Nov 2026): "not good. make it like it is on the design board. slower."* Built to A-24 part 1 frames 5 to 7 at twice the board's timing; the travelling band is gone. One element, the hour's light (`hour-light[data-door]`), and one keyframe set (`door-open`) on registered `--door-w/h/x/y/reach`: the board's arch (90 by 260px at 70px along the head's foot, 44%), its opened pool at 37% (34% by 150% at 22% and -10%, 32%; `--door-opened` 520ms of `--door-open` 1400ms), then the hour's own pool. The head (`.page-head[data-door]`) holds only the light at first; the greeting tints in over it (twice a tint, ending at frame 6), the date and term lines (`data-door-step="date"`) twice the light lead behind, the lines and Notices (`data-door-step="lines"`) from `--door-lines` (880ms), and the blocks arrive as on every page from `--door-blocks` (1160ms): every page-head delay now starts after `--arrive-after`, which the door sets on the head's siblings, and `usePageHead(after)` settles that much later (`DOOR_BLOCKS_MS`, held equal to the token). The pool's mask fades over the box's own overhang (24, 26 and 28px) instead of 18% and 22%, so the light is whole behind the words and the arch stands on a soft floor. The light is drawn on the box's `::before` in container units (`container-type: size`; the hour's tokens in cqw and cqh), because Chrome refuses an ellipse whose size mixes % and px, which is what the board's px arch interpolating to its % pool gives; and the door's light exists only under `(forced-colors: none) and (prefers-reduced-motion: no-preference)`, so reduced motion shows the pool, never the arch held still. Contrast is measured per beat at the light under it when it starts to show (RULES). Reduced motion and forced colours: frame 7 at once. Guards (`craft-ceremony.test.tsx` "the door opens on Home", `HomeHandover.test.tsx` "the first arrival of a sign-in", `craft-places.test.tsx`). **Mutation-checked** (each red): the greeting shown at once; the lines before the light has fallen (contrast); the board's own timing, not twice it; another arch; the head fading with its light; a transform in `door-open`; the beats not skipped under reduced motion; the blocks not held for the door; the head settling before them; Home lighting it for any signed-in session; the settle mirror apart from its token; the door's light outside its motion guard (held still under reduced motion); a size registered as a length-percentage again (the % and px mix Chrome refuses).
    - *Against the board at 0.05× (lead, 19 Nov 2026): the arch was a clipped blob, the blocks showed at frame 5, the head seemed to collapse.* (1) The arch stands a beat (`door-open` holds it 0% to 15%) so frame 5 is seen and not a flick of the settle curve; its foot is the head's foot (`calc(100cqh - 28px)`, above the box's bottom fade) and its height is `min(260px, (100cqh - 52px) / 0.7)`, so the part of the ellipse that shows (70%) always fits inside the box's fades and nothing cuts it. (2) The blocks are held on the door itself: `.page-head[data-door] ~ *` arrives from `--door-blocks` with the usual stagger, after and over the ordinary arrival's rules, never gated on the head settling; the head's settle timer runs at wall speed, so slowed playback had settled the head and dropped the hold. Home ends the door when the last block's `arrive` ends (its `data-door` comes off, so the door's rules never outlive it and an Unfold's own animation is never overridden after it), with `DOOR_BACKSTOP_MS` if that never comes. The `--arrive-after` offset and `usePageHead(after)` / `DOOR_BLOCKS_MS` are gone. (3) The head's beats are opacity only and rendered from the first paint; a guard holds that (no display, visibility, height or position in the door's head rules, no `door &&` in Home). A line whose own read lands late still opens as on every page; under the door that happens while the blocks are still held. (4) Frame 6 is the board's `lights[1]` (34% by 150% at 22% and -10%, 32%) in the box's units. Checked in a local build at 1440px against the board's frames 5 and 6. Contrast re-measured with the hold (RULES). **Mutation-checked** (each red): the blocks' hold gated on the head settling; no stagger; the door never ending (no `onAnimationEnd`); the door ending on any arrival, not the last block's; no backstop; the arch not held; its height uncapped (260px); its foot at the box's bottom; a head beat hidden with `display`.
    - *Seats for the late lines (lead, 19 Nov 2026: on staging the term line and "Checked, nothing waiting" arrived late and pushed the head down).* The term line's Unfold is open while the terms read is out (`termPending`: no data and no error), holding one line (`data-line-seat="term"`, the TermLine's own height); the term takes the seat when it lands, and the line folds with the existing fold only once the read says there is no term (or fails). The Checked line's Unfold is rendered from the first paint and open while the queue is read, holding its line with an invisible copy of its own shape (`data-line-seat="checked"`, `aria-hidden`); once read, it stays if it has something to say and folds if not, and an error folds it. On every visit, not only the door's. Guards (`HomeHandover.test.tsx`): the term seat open before the read and filled in place (no opening), folded when there is no term; the Checked seat open and hidden while loading, replaced by the line. **Mutation-checked** (each red): the term seat not held; the Checked line not held while loading; the seat not hidden from assistive technology.
    - *The hold on a box with no box (lead at 0.04x on c11ffa60: the blocks at about 30% at frame 5).* Home's blocks come in `ArriveList`, whose wrapper is `display: contents`, so the door's opacity on it painted nothing and its children showed through (and arrived on the list's own clock). The door now holds the wrapper still and holds its children (`&[data-door] ~ .contents > *`, the same stagger from `--door-blocks`, after the list's own rules so it wins); Home ends the door on the last box's arrival, through the wrapper. Guards: the children rule and its stagger, its place after `@utility arrive-list`, `ArriveList`'s wrapper being `contents`, and the door ending on the wrapper's last child, not the wrapper. **Mutation-checked** (each red): the hold left on the wrapper alone; the door ending on the wrapper.
    - *Decided before the first paint (lead, rAF on staging at 90fb75d5: the greeting tinted in, snapped to 0 when the door was set after hydration, then tinted in again).* The door is now the server's: the OIDC callback and the dev identity picker set `syndra_door=<signedInAt>` (`doorCookie`: 60s, Path=/, SameSite=Lax, not httpOnly, an instant only), `app/page.tsx` reads it (`doorFor`: only the sign-in's own instant) and renders `<Home door>`, and Home's light is in the server's HTML under the door (no `data-hour` until it mounts), so frame 5 is the first thing painted. The shell spends it on the first page that mounts (`spendDoor`: deletes the cookie, records `firstTime`), and Home's first client render checks `openedAlready` so a cached render (Back) never replays it. `takeFirstArrival`, `noteSignIn` and the per-load claim are gone. Noted, not handled: two tabs mounting at once on the same cookie could each open the door. Guards: `app/__tests__/landing-door.test.tsx` (door with its own cookie, none without or for another sign-in, the cookie's shape) and `HomeHandover.test.tsx` (the server's render of Home with the door carries `data-door` on the head and the light, without it neither; spent, a cached render stays shut; the next sign-in opens). Checked end to end on a local build: the dev sign-in sets the cookie and the first HTML of `/` carries `data-door` on the head and the light; without the cookie, none. **Mutation-checked** (each red): the page never deciding; any cookie opening it; Home ignoring the server's answer; a spent door replaying; the light left out of the server's HTML; the shell not spending it.
    - *The blocks popped in at 673ms (lead, rAF on staging at e1f5d5a4).* ArriveList settles its rows at 580ms after mount (`.arrive-list[data-settled] > *:not(.close-gap) { animation: none }`), a selector exactly as strong as the door's hold on them; the build orders utilities as it likes, so the list's rule won, the hold came off and the block appeared at once, its arrival already run unseen, and that arrival's end then ended the door. Every hold now binds more strongly than the parts' own rules (`~ :not(.contents)` for boxes beside the head, `~ .contents > :not(.close-gap)` and `:nth-child(k):not(.close-gap)` inside a list's wrapper), so the blocks are held at 0 until `--door-blocks` and arrive from there with the usual fade, rise and stagger, and the door ends on the arrival that was seen. Guard (`craft-ceremony.test.tsx`): the holds, read from the CSS, outrank every rule the list and Unfold utilities write and the ordinary arrival's, by specificity; the first block's delay is `--door-blocks` and `arrive` starts from opacity 0. Checked in a local build's real cascade, a list settling at 580ms: the first block 0 and 8px down every frame to 1140ms, 0.10 at 1173ms, 0.99 at 1429ms, the second 40ms behind. **Mutation-checked** (each red): the list hold back to `> *` (a tie); the beside-the-head hold back to `~ *`; the first block's delay at 0.
    - *After PR 218 (board fidelity) merged.* The late lines' seats follow the redrawn lines from one source: the term line's seat is `TermLineSeat`, the TermLine's own box and track classes with unseen words (one row on a desktop, the track's own row on a phone), and the Checked line's seat shares the line's box and its links' 24px reach (`CHECKED_LINE`, `CHECKED_LINK`: the tap's 44px with -10px either side on a phone, none on a desktop). Blocks that mount late (their read lands after the door began) wait only what is left of `--door-blocks` on the door's own animation clock (`doorWait`, `--door-wait` set by Home as the blocks change; `DOOR_BLOCKS_MS` held equal to the token), so every block arrives from 1160ms. Checked by rAF after a real dev sign-in in a local build: `data-door` from the first frame, the greeting 0.02 at 339ms and 0.99 at 546ms, a block mounted at 262ms held at 0 to 1160ms, 0.11 at 1206ms, 0.99 at 1464ms. Guards: each seat's size classes equal its loaded line's (`HomeHandover.test.tsx`), the late wait measured on the door's clock, the holds reading `--door-wait`. **Mutation-checked** (each red): the Checked seat at the old 44px tap; the term seat back to `min-h-6 pt-1`; the wait ignoring the door's clock; the first block's hold ignoring the wait.
    - *The door as a deletion (owner, 19 Nov 2026: "its horrible. all you have to do is to make the diffused purple blob go left to right gracefully. just like in the design board. this is so bad. be elegant do better.").* The choreography is gone: the arch and its stops, the staged greeting, date and lines (`data-door-step`), the held blocks and their waits (`--door-blocks`, `--door-wait`, `doorWait`), the door's end event and backstop, `--door-opened`, `--door-lines`, `--door-reach` and their guards and contrast table. What is left: one keyframe, `door-open`, on the hour light's `::before`, carrying the pool from `-8cqw` at 0.8 of its size to its resting place and size (`--door-x`, `--door-s`, registered), opacity rising over the first quarter, 1800ms on `--ease-tint`, in the pool's own light so never stronger than at rest; the server's cookie decision, `firstTime` and `openedAlready` stay. The door's CSS is 28 lines (it was 174) and Home's door code 6 (it was 25). The page arrives as on every first visit. The late lines' seats stay: they stop the head growing when the terms read or the queue read lands on any visit, door or not. Checked by rAF after a real dev sign-in in a local build: `--door-x` rose from -58px to 626px (the evening pool) with no step back and none over 10px a frame, the light 0.56 at 296ms and whole by about 450ms, the greeting at full opacity in every sample; set beside board part 1, frames 5 to 7, it is the same travel from the head's left into the hour's pool. Guards (`craft-ceremony.test.tsx`, `HomeHandover.test.tsx`, `landing-door.test.tsx`): the glide's two ends and its opacity rise, its registered values, its curve and length, light only, the pool at rest under reduced motion and forced colours, nothing else marked or staged, the server's decision, once.
    - *The crossing (coordinator, 19 Nov 2026, from staging at 23:14: in the quiet hour and the morning the pool rests at the left, so the glide travelled about 120px and read as no movement).* The door is now two layers of the hour's own light on the same element: `::after` crosses from `-10cqw` to `62cqw` at every hour (`door-cross`, opacity up over the first 20%, out over the last 35%), and `::before`, the resting pool, fades in over that same last 35% (`door-settle`), so the two opacities are p and 1 − p and never sum past one pool; `--door-s` is gone. Door CSS 26 lines. Checked by rAF after a real dev sign-in in a local build at 1440px with `data-hour` forced to morning, evening and quiet: in each the crossing centre went from −14.6% to 62.9% of the head, never backwards, at most 0.73% of the head a frame; the two opacities summed to at most 1.000; the greeting at full opacity in every sample. Guards mutation-checked (the settle window, the travel, the reduced-motion bound).
    - *Boundless (owner, 19 Nov 2026, on 7bfdddbe: "the glow moves smoothly and i love it. the bounding box is breaking the immersion though … it feels limiting").* The motion is unchanged. The light's box now meets the content column's own edges (`calc(-1 * var(--page-x))`, the same 26px up to a 1584px window) and the scroller's top, and its two layers run on `-80cqh` under the head, past the lowest reach of any pool or the crossing (135cqh), so both gradients fade out inside it; the linear edge masks are gone. Home's column is the stacking context (`isolate`) and the head sits under its siblings (`--layer-under`; popovers are portalled), so the light falls on the canvas below the head and behind the cards. Checked in a local build at 1440 and 390, both themes, mid-crossing and at rest (and the afternoon and quiet pools at 1440 dark), with the page's own content hidden: no step of 3 levels or more between any two neighbouring pixels anywhere in the scroller (the same scan finds a 554px straight edge when the layers are cut back to the old box), no sideways scroll. Text on the canvas under the head holds the pool's strongest-point numbers (RULES). Guards (`craft-places.test.tsx`, `HomeHandover.test.tsx`): no mask, the box at the column's edges, every gradient's lowest reach inside its layer, the head under its siblings in an isolated column.
    - *The room's light (owner, 20 Nov 2026: "be careful of the bounding boxes clipping the glow"; A-25 §2 rule 1, §3, §7.3).* The hour's and the door's light leave the page for the shell's column: `RoomLight` (`components/shell/RoomLight.tsx`) portals Home's light into `AppShell`'s `@container` column, now `relative isolate`, through `RoomContext`; outside the shell nothing is drawn. The layer is 480px at the column's top, behind the top bar and `#app-scroll`, so the bar's hairline, the scroller's box and its `scrollbar-gutter: stable` seat no longer end it, and it stays while the page scrolls (R3, recommended). Undoing R3 is not one line: the light would go back inside the page, with its own containing block and a stacking context there, and the scroller's top would end it again. Every light's falloff is the six eased stops (`--pool-stops`, once, in globals.css), the lit card's too; A-25's places and sizes; `--door-x` path, timing and strength unchanged; `--layer-under` and Home's `isolate` gone. Checked in a local build after a real dev sign-in: the layer is the column's (1188×480 at 1440, 390×480 at 390), not inside the scroller; the crossing from -10% to 62% of it, never backwards, the greeting whole; light on minus light off, the same strength each side of the bar's hairline (dark at 1440: 72 and 67, 57 and 54 levels; paper: 34 and 35; 390: 49 and 45), and no step of 6 levels or more anywhere in the light below the bar's own controls (largest 4 dark, 6 on paper, its gradient's own banding, 3 at 390). The gutter measured 0 (overlaid scrollbars); the layer spans the column, so a classic scrollbar's seat lies over it, not beside it. Guards: `HomeHandover.test.tsx` (the light is the column's child, never inside `#app-scroll`; none without the shell; AppShell hands the column, not the scroller) and `craft-places.test.tsx` (the six stops; every light gradient draws them, no `transparent N%`; no mask; every pool and the crossing spent inside the layer), each mutation-checked (light drawn in place, a fallback in place, the ref on the scroller, a ramp on the hour light, a ramp on the lit card, a changed stop, the crossing past the layer's end). Not run: the layout check's replay, which needs a tape recorded from `SYNDRA_STAGING_URL`, unset here.
    - *Review refinements on c2c594c1 (Opus review, nothing blocking).* The head's leftover `relative isolate` is gone. A classic scrollbar's track is transparent (`scrollbar-color: var(--faint) transparent` on `#app-scroll`, CSS Scrollbars Styling 1), so it never cuts the light. The contrast test now holds `--warn-text` and `--danger-text` under the light too, and found paper's `--warn-text` at 4.34:1 under the morning pool's strongest point: it is `#805000` now (4.83:1 there, 6.14 on the bare canvas; the design system's synced bundle still says `#8a5600` until the next resync from code). One source check of the shell (`design-system.test.ts`): the column is what `RoomContext` hands, never `#app-scroll`, and the top bar, `#app-scroll` and `main` carry no `bg-` class, since a ground on any of them would hide the light. Mutation-checked: `bg-canvas` on `#app-scroll`, on `main`, `bg-surface` on the bar, and the ref moved to the scroller each turn it red.
    - *Your place in the rail, the two repairs (A-25 moment 4, first in its build order; owner, 20 Nov 2026: "yes to recommended").* (2) The rail's current-place dot takes the tab bar's token, `bg-accent-text` (`Sidebar.tsx:172`): on its pill 4.93:1 in the dark room and 5.76:1 on paper, where `bg-accent` was 3.42 and 3.88 (WCAG 1.4.11; A-25's low-vision bar is 4.9). (1) Under forced colours the current place in the rail (`Sidebar.tsx:156`), the tab bar (`TouchNav.tsx:161`) and the phone's sheet (`TouchNav.tsx:581`) reads one rule, `nav-place` (globals.css), keyed on `aria-current="page"`: `forced-color-adjust: none`, `Highlight` behind, `HighlightText` for its words; inside, the dot and any badge give way to the box (children inherit the colour, no ground of their own); no app animation over it (`flash` would take the box away for 0.9s once the system's correction is off), and its focus ring in `CanvasText`, not `--focus-ring` (about 1.8:1 when the app's room differs from the system's) (Opus review of 5b127c3d). Before, the 15% fill kept its alpha when forced and vanished with its dot, leaving only the weight. Guards (`rail-place.test.ts`): the dot's token read from the rail, 3:1 on its pill and on the rail and 4.9 on its pill, both rooms; the rule's whole shape, through its children, `animation: none` and the focus ring; each current place carries `nav-place`. Mutation-checked: the dot back to `bg-accent`, the dark `--accent-text` lowered to violet-500, the hook removed from the rail, the tab bar and the sheet in turn, `forced-color-adjust` dropped, the children block, the animation line and the focus line each deleted: each red. The rule is checked as emitted by the build; not seen under emulated forced colours (the browser tool here cannot emulate them). Nothing else in the rail changes; the glide and its light are step 5.
    - *The member's room (A-25 moment 1, build order step 2; owner, 20 Nov 2026: "yes to recommended"). Branch `feat/addon-contract-a25-room`, on W4.* My access gets Home's light by Home's own path: `HourLight` (`components/shell/RoomLight.tsx`) is the one place the hour's light is drawn, the pool and the door's crossing, handed up through `RoomLight` to the shell's column; Home and My access each render `<HourLight phase door signedInAt />`, and Home's own door state moved into it (mounted with the page's first render, so it decides the door before the shell spends it). `app/page.tsx` decides the door for either page. My access passes no hour when Zitadel was not read (`observation.read_at` empty: the Notice and no rows), and a failed read is the page's error, with no head to light; while the read is out the room is lit. Moment 0's keyframes, tokens, still form and forced-colours rule as they are; nothing new in globals.css but the comment naming the second head. Contrast: the test now holds every colour class in both lit heads to the tested list at the pool's strongest (My access's second sentence is `text-faint`: 5.29 dark, 4.75 paper). Checked in a local build against a local stand-in backend (`rey_student`): the member's door crossed from -10% to 62% of the column, never backwards, greeting whole; a visit after it stays still; with Zitadel unread no light. Guards (`MemberAccessRoom.test.tsx`, `landing-door.test.tsx`, `craft-places.test.tsx`), mutation-checked: My access without its light (6 red), lit on a failed read (1), a door that ignores `openedAlready` (2), a second drawing of the light in My access (5), the member's page without the door (1), the faint sentence in a colour the test doesn't hold (1). Not run: the layout replay (no tape; `SYNDRA_STAGING_URL` unset here).
    - *The member's room, review round (Opus review of 28f67b87, nothing blocking).* The rule, as intended and now recorded in A-25 moment 1: the room is lit while the read is out, and the light goes if the read lands unread. My access unmounts `HourLight` then (`{!(access.data && unread) && <HourLight … />}`), so a later read brings the pool back still and the door never replays minutes later; a null `phase` means only "before the clock". An operator's own `/me` (My access) shows the still pool and no door: the same page gets the same room, the door being the landing page's, a deliberate consequence of A-25 moment 1 (coordinator, 20 Nov). Stale "Home" wording in `lib/motion/door.ts` and `lib/useDayPhase.ts` now says Home or My access. Tests that only echoed the code are gone (the `<HourLight …/>` regex, the `<h1>…text-faint` match; the head-colour sweep covers the latter); the one-drawer check reads any quoted `hour-light`. New: the read landing unread then read never replays the door; Home and My access each inside the real `AppShell`, which spends the door in its layout effect, cross on the first visit and rest on the second. Mutation-checked: the light kept mounted through an unread read (1 red), the door decided in a `useEffect` (2 red: both pages in the real shell), a second drawing written `{'hour-light'}` (6 red).
    - *The member's room, lit only after a good read (owner on staging, 20 Nov 2026: "the glide light plays out while the page is loading and abruptly stops when it switches to account not found. not elegant. this happens on every page visit.").* The rule "lit while the read is out" was wrong and is replaced: My access lights only once Zitadel's read has landed and is read (`lit={!unread}`; an error returns the page's error before the head); never while loading, on unread or on error, so nothing is cut off mid-animation. `HourLight` decides how it arrives once, the first time it is drawn: lit from the first render, at once (or crossing, if the door is due), as Home always is; lit later, it settles in (`data-settle`: `fade` on `--duration-settle`/`--ease-settle`), and crosses instead only if the door is due and the read landed within `--door-open` of the page's first appearance (`performance.now() <= DOOR_OPEN_MS`, 1800ms, the crossing's own length, held to the token by `design-system.test.ts`; a due door is only ever a full page load); drawn again after going, it settles in. Simplified after review: one `start` (due, lit) and three arrivals, a light that goes becoming `settle`. The door is still decided at the page's first render, before the shell spends it, so once a sign-in stays exact. "Every page visit", found by reproducing it in a local build against a stand-in backend answering 404 after 1.2s (the account-not-found case): it was not the door replaying (that crossed once, on the first visit only); it was the still pool, drawn the moment each visit's read began and held through React Query's retries (about 3.4s), then cut when the page switched to its not-found state. The cause was the rule itself, lighting before the read; fixed at the rule, not by fading the cut. After the fix, in the same local build: not found, nothing lit at any frame; a good read landing at 570ms, the door crossed in full (-10% to 62%); a revisit through the rail, the pool settled in over about 260ms, no door. Home is unaffected: it has no unread state of this kind, renders `HourLight` with `lit` at its default (true), and keeps its arrival (at once, or crossing). Guards (`MemberAccessRoom.test.tsx`, `craft-ceremony.test.tsx`, `design-system.test.ts`), mutation-checked: lighting while the read is out (5 red), crossing however late the read (1), the door crossing again after the light went (1), arriving without a settle (3; 2 after the simplification, its re-show now settling by the hide branch), the settle rule removed from globals.css (1), the door decided in an effect (2).
    - *The edge, light under your hand (A-25 moment 2, build order step 3; owner, 20 Nov 2026: "i really like the edge", then "yes to recommended": option A on outline, ghost and filled). Branch `feat/addon-contract-a25-edge`, on the room branch.* One utility, `rim` (globals.css, beside `hover-tint`): a `::after` one pixel wide on the wearer's own edge, masked to its ring (`padding: 1px`, `content-box exclude`), one pool at the pointer on the six eased stops (`--pool-stops`, now coloured by `--pool-c`, the accent unless a light says otherwise), opacity in on `--duration-tint` and out on `--duration-settle`, both `--ease-tint`, under `@media (hover: hover)`; the keyboard's answer is a focused button's whole edge, and a card's hairline beside its focused row; not drawn under forced colours. Kinds, by the build's table: `rim-card` (inset 0 inside the border, since a card clips at its padding edge; 50%, 320px), `rim-outline` (inset -1px, on its border; 46%, 84px), ghost (the default: inset 0; 46%, 84px), `rim-filled` (inset 1px; 34%; its own colour, `--rim-filled`, the button's ink). Worn by `Card` unless stale, and by `Button`'s accent, outline and ghost when enabled; never by danger, dangerConfirm, a disabled or pending button, a Notice or Not open yet (`data-no-hand`), a first-day card (its edges are its own), the rail or the column headings (neither is a `Card` or `Button`), or a dialog's consent (dialogs live outside the page). One listener, `HandHost` in the shell: one passive `pointermove` on the window, ignoring touch, remembering the pointer and once a frame measuring every wearer under it before writing any `--px`/`--py` (no layout in the handler; the write moves a gradient); a `focusin` tells a card where its focused row is. Rollout, staged as the spec asks and in one line: the shell's `main` carries `data-hand` on Home and People (`HAND_FIRST`), and the light is drawn only under it. Built hover, press and focus ring unchanged; no label's contrast moves. Ratios at each pool's strongest point (A-25 measured, then here): card 1.7-1.8 and 1.8-1.9 (1.88, 2.00; the strong hairline 1.51, 1.38), outline 2.2 and 2.1 (2.33, 2.23), ghost 1.6 (1.77, 1.81), filled 1.6 and 1.75 against its face (1.85, 2.07). Checked in a local build on People: the card's ring lit at the pointer's place, opacity 1 under the hand, `--px` and `--py` in the card's own box, nothing inside. Guards (`edge-light.test.tsx`, `craft-places.test.tsx`), mutation-checked: a host per card (2 red), listeners on each wearer (1), danger wearing it (1), disabled wearing it (1), the card's ring over its border (1), measuring on every move (1), no frame gate (1), reads and writes interleaved (1). Unverified: the frame cost of a card-sized masked gradient on People at 200 rows (A-25 §9).
    - *The edge, review round (Opus review of 0ee08f15: two cascade blockers the guards missed).* (1) `.rim` set `--pool-k` outside its drawn scope and came after `.lit-card` in the output, so a first-day card on My access took 50% where it is 22% x `--hour-weight`: the strength (and the filled button's `--pool-c`) now live only inside `:where([data-hand]) &:not(.lit-card, ...)`, and a test holds that nothing outside that scope declares `--pool-*` and that the lit card's strength is its own 22% x weight. (2) `--rim-k`, `--rim-r` and `--rim-inset` inherited, so a button inside a card took the card's 50% and 320px, the whole-edge look the owner did not choose: every wearer now declares its own (46%, 84px, 0px), and the kinds are compound (`&.rim-card`, `&.rim-outline`, `&.rim-filled` inside `rim`), decided by specificity, not output order. Also: the light redraws on a scroll (window, capture, passive) and a resize, at what `document.elementFromPoint` finds under the still pointer, holding only x and y; the listener no longer skips a Notice (the CSS draws nothing there); `--px` and `--py` are registered (`<length-percentage>`, not inherited, 50%) and inherited into the `::after` only, so a frame's write restyles the wearer alone; `AppShell` is tested to mark `main` on `/`, `/users` and `/users/...` and nowhere else; the tests that only restated source strings are gone. Mutation-checked: `--pool-k` back outside the scope (1 red), no defaults on the wearer (2), a kind as its own utility (3), no scroll redraw (2), the first hit reused (1), a listener left after unmount (2), the scope letting a Notice draw (the edge test file fails to load: its scope is no longer found), `--px` inherited (1), the scope on every page (1). Browser targets: `ui` sets no browserslist, so Next's default applies (chrome 64, edge 79, firefox 67, opera 51, safari 12), which predates `color-mix` (Chrome 111, Safari 16.2, Firefox 113); Tailwind v4, which the UI is built on, already requires Safari 16.4, Chrome 111 and Firefox 128, and main's `globals.css` used `color-mix` before this wave. Reported, nothing changed.
    - *The edge everywhere, and on every card (owner on staging, 20 Nov 2026, of Home's stat tiles: "these dont show the effect. and only home page shows the effect").* `HAND_FIRST` is gone: the shell's `main` carries `data-hand` on every route; what must not light opts out where it is drawn. `StatCard` drew its own frame, so it never wore the edge, and its link kept the old flat ring (`hover:shadow-[0_0_0_1px_var(--accent-line)]`), the uniform purple border on "Connected systems": both tiles, linked and still, now wear `CARD_EDGE` (`rim rim-card`, exported from `Card.tsx`, the one path for every card frame) and the ring is gone; its press tint and focus ring are as they were, and a focused tile lights its whole edge. Audit of card-like surfaces outside `Card`: `StatCard` (wears it), `BundleVersions`' through-bundles section (`card` utility; wears it), Insights' `AnswerPanel`, `Timeline` and `LaneMap` (card frames of their own; wear it); not cards, left as built: `ErrorBoundary`'s danger frame (an error, like a Notice), popovers and sheets (portalled), fields, choice rows and copyable values (controls), code and token blocks, warn boxes (`Withheld`, `Acknowledge`), lists inside a card or a dialog, the selection bar. Opted out: `RuleConsentFailed` (`data-no-hand`), a consent's body that sits on a page (the Zitadel users page) and holds a retry button; Notice and Not open yet were already. No other flat accent hover ring remains (`hover:shadow-[0_0_0_1px`, `hover:border-accent`: none). Visibility of a card's edge (50%, 320px), by arithmetic on the six stops over `--surface-1`: at the pointer 1.88:1 dark and 2.00:1 paper; 30px from it 1.71 and 1.83; 160px from it 1.19 and 1.25, about the built `--line` hairline (1.16 and 1.17), so on a big card the edge reads only within about 150px of the hand; strength unchanged, for the owner. Guards (`edge-light.test.tsx`, `AppShell.test.tsx`), mutation-checked: a tile without the edge (1 red), the flat ring back on a tile (2), `hover:border-accent` on a card frame (1), the bundles section without the edge (1), `data-hand` only on Home (1).
    - *The frame carries the edge (owner on staging, 20 Nov 2026, of a person's "Bundles · None assigned" strip: "this element is not responding to the hover. please look into other such ui elements as well").* The strip was a `panel` frame, which the last audit (it grepped for `card`) missed. Fixed at the root: the edge is one block in globals.css (`@layer components`) whose wearers are a button's `rim` and every frame utility, `card` and `panel`, so a frame cannot be drawn without it; `HandHost.WEARERS` is the same list, and a test derives the frame utilities from globals.css (a card's radius, a surface, a one-pixel hairline) and fails if one is not a wearer in each place. `CARD_EDGE` is gone: Insights' answer panel and timeline are now the `panel` and `card` frames themselves; the lane map, whose ground is its own (`--surface-0`), wears `rim rim-card`. A stale card is refused by the CSS (`[data-stale]`), as a lit card, `[data-no-hand]` and everything outside the page are. Newly lit this round: People, a person's Bundles strip (`PersonAccess.tsx:389`); Home, the calm line's panel (`Home.tsx:501`), lit because it is a card and nothing in it warns. No new opt-outs. A real-browser rule, `edge`, in the layout check's `detect.js`: inside `main[data-hand]`, anything drawn like a card (a frame radius or more, a one-pixel border, a surface token's ground, read from the tokens) that is neither a wearer nor inside `[data-no-hand]`, a dialog or a popover is a finding. Run: a tape recorded from staging (420 page runs), replayed into this branch's local build: 420 page runs, 0 findings, 0 reads not on the tape; with the Bundles strip typed by hand instead of `panel`, the rule reports it ("BUNDLES None assigned" is drawn like a card and does not wear the edge). Grep guard: a frame utility with `data-no-hand` needs a comment above it naming the light, the hand or the edge. Mutation-checked: `panel` dropped from the wearers' own values (4 red), from what is drawn (5), from the listener (1), a frame opted out with no word why (1).
    - *The edge, review round (Opus review of fb23e4d3, nothing blocking).* An opt-out on the frame itself did not hold (`:where([data-no-hand] *)` matched only what was inside one): the drawn rule's `:not()` is now `.lit-card, [data-stale], [data-no-hand], [data-no-hand] *, :has(> [aria-busy="true"])`, and a test reads that selector from the CSS and matches real components against it (a plain card and panel and a button lit; a card refusing it, one inside a refusal, a first-day card, a stale card and a placeholder not). The stale storage card on a person's page (`PersonStorage.tsx`) is `<Card stale>` now, not hand-written `!` classes, so it recedes, rings its dots and is refused the light. `ErrorBoundary` refuses it (`data-no-hand`): an error is like a Notice. Quiet frames, the coordinator's decision: a placeholder has nothing to act on yet, so a frame whose content is still a skeleton is refused by the rule itself (`:has(> [aria-busy="true"])`, `RowSkeleton` being `aria-busy`), one place for Home's loading card and every list's, and `SkeletonCardList` refuses it too; Home's all-clear panel keeps the light, since it is a card. `detect.js`'s `edge` rule: the roundest of the four corners, at least a card's or a panel's radius and less than a pill's (no pill is a card); a hairline on all four sides (over 0, at most 1.5px); a ground that is a surface token at any alpha (the computed colour painted onto one pixel and read back, since Tailwind's `bg-surface-1/60` computes to `oklab`, which a parse of `rgb()` missed), or a drawn gradient on a frame's shape. Tests anchor on the selector, not a comment, and `braced()` throws when nothing matches, so a `not.toMatch` cannot pass on an empty string. `AppShell` is rendered once (it does not ask the route). A-25 moment 2 now says what is built and where; "Home and People first" is gone. Replay of the staging tape (420 page runs) into this build: 0 findings, 0 reads not on the tape; the detector itself checked by re-typing the person page's Bundles strip by hand, opaque and at 60% (`bg-surface-1/60`): each reported. Mutation-checked: the frame's own opt-out removed (5 red), placeholders lit (5), stale lit (5), the storage card's hand-written recede back (1).
    - *The pool's colour default moved into the base layer (Opus review of the lamp, e50532e1: a cascade bug from the edge).* `:where(.hour-light, .lit-card, .rim, .card, .panel) { --pool-c; --pool-stops }` sat outside every layer, and an unlayered declaration beats every `@layer` one whatever its specificity, so a filled button's `--pool-c: var(--rim-filled)` (in `@layer components`) never applied and its edge lit in the accent on its accent fill. It is in `@layer base` now. Checked in a real browser on a local build: a filled button's computed `--pool-c` is `--rim-filled` (rgb 247 244 255), an outline button's and a card's the accent (rgb 127 90 240); the door, the hour's pool and the lit card set no colour of their own and read the same default as before. Guard (`edge-light.test.tsx`): every `--pool-*` default sits in a lower cascade layer than every override, and the default is in base; moving it back out of the layer turns it red.
    - *The lamp breathes (A-25 moment 3, build order step 4; owner, 20 Nov 2026: "rest recommended seem good": the breath, for operators). Branch `feat/addon-contract-a25-lamp`, on the edge.* `lampRoom` (`lib/queries/useIndicators.ts`) is the one answer: "watch" while the rail holds a live reading (not the placeholder's zeros, not a failing read), "waiting" while that reading says Zitadel isn't answering, nothing otherwise; operators only, since a member's lamp stays still until a reading of theirs is proven (the spec's accepted answer, and the cheaper: no new read). The rail passes it its own reading; the phone's bar asks the same read (`useLampRoom`), so both marks say the same. `SyndraMark`'s `Room` gains "watch". In globals.css, beside the mark: a lit mark's `::before` is a pool of the dot's own colour (`--pool-c: var(--mark-dot)`, 36%, the six eased stops, `.syndra-mark` added to the `--pool-stops` list), 0.85 mark-widths out (`inset: calc(var(--mark-size) * -0.85)`), under the dot, ring and arc (`isolation`, `z-index: -1`), which never move; at rest 0.72; one keyframe, `lamp` (opacity 0.28 to 1, scale 0.76 to 1), over `calc(var(--door-breath) / 2)` on `--ease-breathe`, alternating, while watching, and on `--duration-breathe` while waiting, amber because waiting's dot is `--warn`; only under `(forced-colors: none) and (prefers-reduced-motion: no-preference)`; gone under forced colours. A hidden tab: the browser does not run a composited animation it does not paint. FINAL §4 and RULES now name two loops, breathe and the lamp's; the design-system licence test admits exactly those two. The layout check gains `lamp`: a lit mark's glow box, read from its own computed inset, must sit inside every clip (the rail's scroller, the phone's bar, the window). Checked by replaying the staging tape into this build on Home and People at 390 and 1440, light and dark: 16 page runs, 0 findings; with the glow widened to the first draft's 1.3 widths, 12 findings (the rail's top and left at 1440, the window's left at 390). Guards (`lamp.test.tsx`, `Sidebar.test.tsx`), mutation-checked: a placeholder breathing (3 red), a failing read breathing (1), a member breathing (2), the glow not the dot's colour (1), the breath outside the motion guard (1), a glow under forced colours (1), the 1.3-width glow (1, and the browser rule above).
    - *The lamp, review round (Opus review of e50532e1).* With the edge's fix merged (the pool's colour default in the base layer), the lamp's glow takes the dot's colour: checked in a real browser, watching `--pool-c` is `--mark-dot` (rgb 155 123 255) and waiting `--warn` (rgb 245 165 36). Waiting breathes the dot's own `breathe` on `--duration-breathe`, so a change of room restarts it in step with the dot. Offline is no reading: `lampRoom` treats a paused poll (`fetchStatus: "paused"`) as none, and the rail's read now ends itself after two thirds of its poll interval (`AbortSignal.timeout`, 20s of 30s), so a hung GET cannot keep the lamp "live" (no test of the timeout itself: the query options are private to the module). The rail's wordmark and the phone's breadcrumb are `relative`, so the words paint over the glow. `useLampRoom` is gone: the phone's bar is `lampRoom(isOperator, useIndicatorsRead(isOperator))`, as the tab bar reads, and both TopBar test files stand the read in as the Sidebar tests do; the phone's test asserts `data-room="watch"` and a member's none. The hard-coded reach arithmetic is gone (the layout check measures it). The layout check's `lamp` rule also reports an operator's page with no lit mark, so a tape without a reading cannot pass in silence. Replayed on Home and People at 390 and 1440, light and dark: 16 page runs, 0 findings; with `lampRoom` returning nothing, the page reports "no lit lamp on an operator's page". Mutation-checked: offline breathing (1 red), waiting on `lamp` (1), the phone's mark unlit (2), the wordmark under the glow (1), the breadcrumb under it (2).
    - *Your place in the rail: the glide, with its light (A-25 moment 4, build order step 5; owner, 20 Nov 2026: "do glide with light", at the recommended 6%). Branch `feat/addon-contract-a25-rail`, on the lamp.* The pill, the bold violet word and the dot still carry the place; `nav-place` (forced colours) and the dot's repair are as they were. One path for the rail and the tab bar: `usePlace` and `PlaceMarks` (`components/shell/Place.tsx`). The pill is the glide indicator's own, with a new `kind="box"`: one element shaped as the active row is (its computed corners, the rail's 12px and the tab's 13px, never round caps), moved by transform and sized by width and height on `indicator-glide` (`--duration-settle`, `--ease-settle`); once it is measured the row gives up its own fill (before, it keeps it, so nothing is ever unmarked). The light: one pool under the pill, the rail's (or the bar's) own layer, under everything in it (the rail and the bar are `relative isolate`, `data-place-room`), the accent at `calc(6% * var(--hour-weight))` on the six eased stops (`.place-light` joins the `--pool-stops` list), as wide as the rail and up to 176px tall but never taller than twice the distance from the row's centre to the rail's nearer end, as tall as the bar and up to 160px wide but never wider than twice the distance to the bar's nearer side; it moves with the pill on its movement, and its size changes at once. It travels only when the place was reached by a press on one of the group's own links just before the path changed (decided once per path change, from that press); a first paint, Back and a key (Go to, Enter on a row) land it at once, and so does reduced motion. The tab bar does the same; the phone's navigation sheet opens fresh and only takes repair 1. A place folded away: a current row inside a fold that is moving or shut has no box (`measure` treats `.unfold:not([data-phase="open"])` as unplaced), so no pill and no light, and the row's own fill marks it while it is there; shut, an Advanced-only row is not rendered at all in Basic (corrected after review: the first build measured a folding row, and a stale pill sat on the row that took its seat after an Advanced to Basic switch). Under forced colours the pill and the light are not drawn (`.place-pill`, `.place-light`), and the place keeps the system's Highlight. Contrast: the bold violet word on its pill over the pool's centre, both rooms, at least 4.5:1 (A-25 measured 4.72 dark, 5.51 paper). The layout check gains `place`: the pool's box inside its room (the rail's whole scroll content, the bar). Checked in a local build: on a pointer press from Home to People the pill and its light glided 67 to 109px and 0 to 40.5px over about 200ms, with the light's height changing at once; Back landed it at once; the pill's corners 12px; the light 251 by 173px at Home's row (spent at the rail's top) and inside the rail. Replay of the staging tape on Home, People and Requests at 390 and 1440, light and dark: 24 page runs, 0 findings; with the light forced to 400px tall and 600px wide, the rail's top and the tab bar's left are reported. Guards (`rail-glide.test.tsx`), mutation-checked: travel always (2 red), travel never (1), round caps (1), the row keeping its fill (1), the light at 40% (2), a light under forced colours (1). The tab bar has its own (`TouchNavPlace.test.tsx`): one box shaped as its tab, the bar the light's room, and travel only after a tap on one of its tabs (Back and a key land it at once); mutation-checked, the tab keeping its fill (1 red) and the bar not hearing the tap (1). Light theme, looked at: at 1440 the pill's corners are 12px and its light a soft pool under it, edgeless; at 390 the tab's pill is 13px and its light 160 by 60px inside the 61px bar. A full replay of every page (420 runs, a staging tape) found nothing for `place`, `edge` or `lamp`; it found 19 `clipped` and `seat` findings on a TrueNAS page and two person pages at 390, 768 and 1024, which reproduce identically with the base's own code (the rail's files swapped back to `feat/addon-contract`'s), so they are not this change's.
    - *The rail's glide, review round (Opus review of c7385945).* Blocker: after an Advanced to Basic switch the current row folded away under you, the size observer measured it mid-fold and the pill stayed on the row that took its seat (the observer watched only rows that existed at mount). Now a row inside a fold that is not open is unplaced, the glide is measured again when the view flips (`usePlace(pathname, view)`), and a `MutationObserver` re-measures when a fold's phase changes or a row arrives. The layout check gains `pill`: the pill must sit on its group's visible current row within 0.5px, and a group without one has no pill. "Travel next" is a plain primary click on one of the group's links (`onClick`; not a key, `detail` 0; not a modifier or another button), and the glide is one-shot: after `SETTLE_MS` a later re-measure lands it at once. `kind="box"` is gone: `fill` takes the option's measured corners, so every pill is shaped as its option (round for a pill, 12px for a rail row). The server's HTML draws no pill and keeps the row's own fill. The current row's fill snaps (`[transition-property:color,transform]`), so it and the pill never show together at hydration. Mutation-checked: a folding row measured (1 red), a modifier click counting (1), a key counting (2), the glide never ending (1), the measured corners ignored (1), the server's row without its fill (1). Accepted as low and left: the light's height on a content change, and the tab's press shrink.
    - *The rail's glide, the replay with layout regressions merged (PR 227, d0ce637f).* A fresh tape from staging, replayed into this build at 390 to 1440 and at 860, 900, 950, 1060 and 1180, all rules: 0 findings once the `seat` rule counted every drawn part. It had left Home's `aria-hidden` avatar out of a new person's row, so a row a few pixels short of its seat at 860 (an avatar and a gap past the 499px it has) read as one that fits; the seat wraps there by design (D-1 §4 rule 9). `vis(c, true)` for a row's parts, as the lamp already counts its mark; mutation-checked (back to `vis(c)`: the five rows return). Page loads that ran out of time with eleven widths in parallel on the dev server were rerun in smaller batches, and each came back clean.
    - *The rail's glide, second review round (Opus check of f6b14a3a, nothing blocking; staging confirms the glide and the view-switch fix).* (1) The fold check counted any fold around the choice, so a Segmented in an opening `Unfold` (a shape's line) had no box, and nothing re-measured when the fold opened outside the group: only a fold inside the group hides the choice now. (2) The `MutationObserver` is exercised: Basic to Advanced on `/bundles`, the row arrives in an opening fold, and the pill is there once the fold opens, with no resize. (3) The current row's timing is one utility, `place-current` (colour on tint, transform on press, each with its own duration and curve, with its reduced-motion durations beside `.control`'s), used by the rail and the tab bar; the arbitrary `[transition-property:color,transform]` over control's ten-entry list had given the press the tint's 120ms. (4) The `seat` rule leaves out parts out of flow (`absolute`, `fixed`). (5) The corner is read from `borderTopLeftRadius`, falling back to `var(--radius-pill)` where none is read. (6) The pill rule's date (20 Nov), and the first-HTML test reads the markup as a document. Mutation-checked, each red: any fold counting (1), no mutation observer (1), no corner fallback (1), the press on tint (1), the reduced-motion rule gone (1), the tab bar and the rail back to the arbitrary class (1 each).
    - *The trail lights (A-25 moment 5, build order step 6; owner, 20 Nov 2026: the recommendation). Branch `feat/addon-contract-a25-trail`, on 71dd3107.*
      - **What it does.** Under a give's result, once Syndra has sent it, `ChangeTrail` passes `CheckTrail` a `light`. One pool rests on the first open (`ring`) step: a route's current step (`aria-current="step"`, on every route trail, lit or not), measured by the glide indicator's one hook.
      - **How it moves.** It travels there only when the steps change, which is only on a read, over `--duration-flash` on `--ease-tint`. Each dot fills `--light-lead` behind it (`light-lead`). While a step is open it breathes, gated on no-preference as the lamp's is.
      - **How it goes out.** When no step is open (home) it fades over flash on `--ease-flash`. It also goes out when the watch ends (`TRAIL_WATCH_MS`), though the step is still open.
      - **Where it never is.** Not on a removal's trail (act `revoke`), a refusal or a failure (no trail at all), a give that waits for someone to press Send (Syndra isn't moving it; decided in the build review, A-25 §4.5 updated), the route before the answer (`PendingTrail`), or the claims trails (`DebugMode`, `ClaimPreview`). It isn't drawn under forced colours. Still (reduced motion), it rests with no travel and no breath.
      - **Rule 1 in the real dialog** (a give answered locally on the replayed tape, read by `detect.js`'s new `trail` rule).
        - The pool is 280 wide from 20px left of the trail, which leaves 5px inside the panel's 24px margin.
        - It is no taller than its own block and that block's margins allow (`data-trail-room`, `mx-6 mb-1 mt-3`), whatever sits around it and however the panel has scrolled. Measured: 28px tall on the last step, which has its own row and 4px of margin under its dot.
        - Its side is the room's own margin less 4px (20 of `mx-6`'s 24), never the caller's number.
        - First round, sized to the neighbours: with the clamp removed, the rule reported "30.0px past the ground below it". The rule still reads the neighbours, which a block's own margins always clear. `trail-light.test.tsx` holds the geometry.
      - **Contrast.** Under the light's centre on the panel: faint 4.77:1 in the dark room and 4.96:1 on paper, ink 11.56 and 13.30, muted 6.53 and 7.20. A violet link would be 4.03 in the dark room, and a change's steps carry none (held).
      - **Mutation-checked**, each red:
        - the last open step, not the first (3);
        - lit ahead of a read (4);
        - no lead (1), and the lead not the token (1);
        - a removal lit (1), and a refusal with a trail (2);
        - the watch never ending it (1);
        - breath when still (1), travel when still (1), and breath ungated (2);
        - drawn under forced colours (1);
        - a check's trail lit (1), and debug mode passing a light, bare or valued (1 each);
        - the light at 40% (2);
        - the room clamp gone (3).
      - **Open for the owner.** On the common answer (Zitadel took it, waiting on the read back) the light rests on the last step at 280 by 28: the room under that step is its own row and the trail's 4px margin. A-25 drew 96 inside an outcome block with room under the trail. The owner is deciding whether the trail moves inside the outcome card, as the board draws it; the sizing is generic, so it takes the room it is given there.
    - *The trail lights, review round (Opus review of 47044c97, no blockers).*
      - (1) The pool no longer flashes going out: its breath runs whether or not it is going, and pauses where it is (`:not([data-going])`), so the fade starts from the strength it had.
      - (2) It is sized from its room's own margins, not its neighbours' (no sliver when the panel scrolls).
      - (3) A ring dot is an inset shadow in its tone's colour (`dot-ring`, the tone in `color`), which `motion-tint` carries into the fill. Checked in Chrome: swapping a ring dot to filled left no frame empty, where the 1.5px border it replaced left the dot empty for its first 55ms. Under forced colours the ring is a border. A stale card's dots ring the same way.
      - (4) `aria-current="step"` is the route's, lit or not, once it has reached a step past a settled one (follow-up: a route drawn before its answer marks none).
      - (5) One clock: the re-reads run only while the trail is watching.
      - (6) `PendingTrail` is held unlit.
      - (7) The side is from the room's margin.
      - (8) A queued give has no light until Syndra has sent it.
      - (9) The light is `data-indicator`, a safeguard so the glide's observer would leave it alone. The observer is set up before the light mounts, so it never sees it either way; the test holds the attribute only.
      - Mutation-checked, each red: the breath tied to going (1), the pause rule gone (1), the room's bottom (1) and top (1) not its own, the ring a border (1) and not a shadow (1), the current step only when lit (2), the re-reads ignoring the watch (1), the pending route lit (1), the side a fixed number (1), a queued give lit (2), the `data-indicator` attribute removed (1: the attribute's own check, not the observer); and the first round's guards again against the moved code: ahead of a read (3), a removal lit (1), breath when still (1), breath ungated (2).
    - *The trail lights, follow-ups (Opus verify of 70d67f04, nothing blocking).*
      - (1) Item 9's observer half could not fail: dropped, and the line above corrected.
      - (2) The ring's forced-colours border is held.
      - (3) One ring: the stale card's dots `@apply dot-ring`, and the compiled CSS carries its forced-colours border to them too.
      - (4) A route marks no current step until one before it is settled, so `PendingTrail` marks none.
      - (5) One ring style: the freshness line's `provisional` dot, hand-drawn as a 1px `border-warn`, is now `StatusDot` with `ring` and `tone="warn"` (the 1.5px ring every other ring dot draws). Its own handle (`data-freshness-dot`) passes through.
      - Mutation-checked, each red: the ring's forced-colours border gone (1), the stale dots' ring not applied (1), the pending route marking a step (1), the room's top not its own (1, now held by a block with no top margin), the provisional reading filled (1).
    - *The trail lights: the walk home, inside the result (owner on staging, 20 Nov 2026: "could barely see it"; the stand-in answers at once, so every step was settled before the result showed and the light flashed on the last step).* The owner chose both changes; A-25 §4.5 and the §7 audit row are updated.
      - **The walk home** (`CheckTrail.tsx` `TrailLight`): the light appears on the first step and walks down the steps already seen, one a flash on tint's curve, to the step reached, where it waits and breathes; home, it rests one breath on the last step and goes out over flash. The dots it passes are already filled (it walks over truth); `--light-lead` only matters to a dot that changes, which is a step settling while the light waits. Once per change; still, no walk (it rests on the step reached, or one breath on the last). It is placed before it travels, so it never slides in from the list's corner. Its position is its own (`data-step`), apart from the route's current step (`aria-current`), so the glide indicator no longer measures trails (`[aria-current="step"]` left its selector, and with it the `data-indicator` safeguard).
      - **Inside the result** (`ActionOutcome` `trail`, under what happened and above what to do next; `GrantDirectAccess` and `RemovalDialog` both): the card is the light's room (`data-trail-room`) and the pool is sized to its inner box. `detect.js`'s `trail` rule now holds the pool inside its card's inner box.
      - **Measured in Chrome** on a give answered locally on the replayed tape, at 1280 (dark and light) and 390 (light):
        - 280 by 96 on every step, 4px inside the card's side and 7px above its inner bottom; 0 `trail` findings. With the pool pushed 12px past the card's side, the rule reported "12.0px past its card's left".
        - The walk: step 1 to 908ms, step 2 to 1815ms, resting on step 3 from about 2.7s; the dots read filled, filled, ring throughout.
        - At the light's strongest (its centre, full breath), the ground goes from rgb(20, 22, 18) to rgb(48, 40, 76) in the dark room (1.33:1 against the unlit card) and from white to rgb(229, 223, 254) on paper (1.29:1). At the breath's low, 1.13:1 in both. Faint ink keeps 5.02:1 (dark) and 4.96:1 (paper) at the centre, on the card (`contrast.test.ts` now measures on `--surface-1`). 26% is kept.
      - **Mutation-checked**, each red:
        - the walk skipping a step (3), no walk with motion (4), and a walk when still (3);
        - dots rung while lit (1);
        - walking again when a step settles (4);
        - no rest at home (2), and never going out at home (2);
        - sliding in from the corner (1);
        - taller than its card (1), and past its side (1);
        - the card not its room (1), and the trail after the next step (1);
        - a removal lit (1), a queued give lit (2), and a refusal with a trail (2);
        - the light at 40% on the card (2).
    - *The trail lights, review round (Opus review of b68d206d, no outright blockers).*
      - (1) The trail is `aria-live="off"` inside its result's `status`, so a settling step does not re-read the whole card.
      - (2) No slide-in: the light travels only once it has left the first step (`!still && at > 0`). In Chrome, its transform held at one value for every frame of its first 300ms, without the travel.
      - (3) The pool lies under every line in the card: the card is `isolate` when it holds a trail, and the list is only `relative`. In Chrome, every visible line of the card hit-tests above the light.
      - (4) Once seen sent, a step stays sent (`gone`). The light is for a give that had already gone out when its result first appeared (`eligible`, latched at first render); a give queued for Send and sent later never walks (coordinator's decision; A-25 §4.5 updated).
      - (5) The contrast comment is corrected: on the light's centre a violet link would be 4.37:1 in the dark room. A lit result holds none, and a test holds that.
      - (6) Simpler walk: it reads its end through a ref, so it depends on its own steps only, and its rest is one timeout (`FLASH_MS + BREATHE_MS`, or `BREATHE_MS` when still).
      - (7) The README says the `trail` rule runs by hand.
      - Mutation-checked, each red (1 each): the trail read with its status, travel from the first frame, the list as the layer, the card not `isolate`, sent not latched, eligibility not latched, and the rest without the last move's flash.
    - *The trail lights: the lamp carried home (owner, 20 Nov 2026: "Carried", the recommendation, after "the bloom moves, the dots do not, it is not organic"). A-25 §4.5 as redrawn in round 4, and its prototype, are committed with it; §10's five questions are marked decided.*
      - **The lamp** (`CheckTrail.tsx` `TrailLamp`): a pool centred on the dot (7px across, the dot's centre down), four quarter-ellipses round that point on the six eased stops (`ellipse farthest-side`), reaching 240px over the sentence, 56px up and down, and the card's room less 4px on the left. It is measured once against the card's inner box at the walk's tightest steps, again only if the card or the list changes size, never per step. 26% × `--hour-weight`, as before.
      - **Carried** (`useTrailWalk`): one wrapper per move, each a `transform` of its own step's height on `--duration-flash` and `--ease-tint`, nested so the lamp is their sum and stays on the compositor (the standard way; the prototype's registered `@property` lengths are not used). The first move sets out at `ARRIVE_MS`, each next one a beat later (`BEAT_MS = FLASH_MS - SETTLE_MS`, 640ms, no new token). It swells in over arrive and goes out over flash; home, a beat after the last step; waiting, it breathes only once it has landed (Web Animations, so the breath ends from where it is).
      - **The line**: a 1px box from the first dot to the last, clipping a gradient (the accent at 60% at the lamp's centre, on the same stops) carried by the same moves. The dots stand in front of it.
      - **The dots**: a read dot answers once, `--light-lead` behind the lamp reaching it: `trail-answer`, the ember's light (`--healthy` at 44%, 17px out, `@keyframes answer`), under the words. A ring never answers; no dot's fill is ever touched.
      - **Still**: one place, no glow, no breath, gone one breath later at home. **Forced colours**: no lamp, no line, no glow. Kept: `aria-live="off"`, no slide-in, `isolate` on the card, the latches, eligibility, no link in a lit result.
      - **Measured in Chrome**, a real give on the replayed tape, sampled every frame (253 frames over 4.2s), at 1280 dark and 390 light:
        - Walk: no backward frame, at most 1.08px a frame (1.09 at 390), the line exactly on the lamp on every frame, 70px travelled.
        - The dots' glows begin at 465, 1098 and 1748ms, with the lamp 0.4, 3.5 and 3.7px from each dot; each is fullest about 250ms later and gone about 770ms after it began.
        - Every read dot's fill is one value on every frame (lime, rgb(163, 230, 53) dark and rgb(77, 124, 15) light).
        - The lamp is inside the card's inner box on every frame, and the `trail` rule reports 0. Gone at 3048ms.
        - Waiting on an open step, it breathes only after landing, and the ring never glows. The live read: the pool's strength moves at most 0.012 a frame across it (0.05 at most in the whole run), the dot fills 51ms after the read and its glow opens at 84ms, and the lamp is out 1.37s later.
        - Still: one place, no glow, gone at 2408ms.
        - At the lamp's strongest point (its dot) the strength is unchanged, so faint ink keeps 5.02:1 (dark) and 4.96:1 (paper), held by `contrast.test.ts`.
      - **Mutation-checked**, each red:
        - the lamp off the dot (2), and above it (3);
        - the moves not overlapping (1);
        - breathing while walking (2);
        - the glow with the light rather than behind it (1);
        - a ring answering (2);
        - a read dot dimmed until reached (1);
        - walking (2), answering (1) and gliding (1) when still;
        - reach past the card's foot (3) and side (2);
        - the line not carried with the lamp (1);
        - home going out a breath later rather than a beat (2);
        - walking again when the change comes home (1);
        - the glow drawn under forced colours (1);
        - a queued give lit (2), and a removal lit (1);
        - the lamp at 40% (2).
    - *The lamp carried home, review round (Opus review of c16db6fa, one blocker).*
      - (1) **The swell now runs on a real trail.** It read `[data-going] > &`, which only matched while the pool was the lamp's own child, and the measured moves wrap it. The pool now rests at 0.86 and is 1 under `[data-going]` at any depth, with `@starting-style` 0.86, on `--duration-arrive`. A test renders a 3-step trail and holds that the pool sits inside the moves and that the rule reads its state through a descendant selector. In Chrome, on a real 3-step give, two moves deep, the pool's computed scale is 0.86 at its first frame (390 light), then 0.94, 0.97, 0.99, and 1 by 390ms, never going back.
      - (2) **A late read walks, never jumps.** `reached` is the dot the lamp has come to, and each next beat is a beat after the last move set out. So a read that settles two steps sets the lamp out at once and the next move a beat later, each dot answering with it.
      - (3) **Smaller fixes.** `trail-answer` only on a lit trail; no `border-radius` on the glow (the `closest-side` pool is round already). The lamp and its line sit beside the list, in one `relative` box (`data-trail`), before it, so the list holds only its steps. In Chrome every visible line of the card still hits above the light, and every dot above the lit line.
      - (4) **Guards.** All four quarter rules are held (corner, reach, far side). `detect.js` reports a lamp missing a quarter, or one that measures nothing; in Chrome, a quarter taken out and every reach set to nought each gave their finding.
      - Kept as it is: the `answer` keyframe would replay if a trail's container went `display: none` and back, since an animation restarts when its box is made again. No container does that today.
      - Measured again in Chrome (253 frames, 1280 dark and 390 light):
        - Walk: 0 backward frames, 1.08px a frame at most, the line on the lamp, 70px travelled.
        - Glows begin at 462, 1096 and 1746ms, with the lamp 0.4, 3.5 and 3.7px from each dot.
        - Read dots keep one lime on every frame; the lamp is inside the card on every frame; gone at 3045ms.
        - Waiting: breath only after landing. Still: one place, gone at 2412ms.
      - Mutation-checked, each red: the swell back on a child selector (1), no resting scale (1), a quarter rule gone (1), a quarter on the wrong corner (1), a late read jumping (5), the lamp after its list (1), and the lamp back in the list (5).
    - *The ember (A-25 moment 7, build order step 8; owner, 20 Nov 2026: "yes to recommended", the ember last; "continue with ember", 21 Nov). Branch `feat/addon-contract-a25-ember`, on `feat/addon-contract`; built, awaiting the owner's look on staging.* The lime dot of a clear queue holds a low, edgeless glow, and when your own act cleared the queue it opens once. One utility, `dot-glow` (globals.css, where `bloom` was): a `::before` pool of the dot's own colour (`--pool-c: var(--dot-light, currentColor)`) at 44% on the six eased stops (`.dot-glow` joins the `--pool-stops` list), `inset: -17px` (20px from a 6px dot's centre, so it is spent inside any card), under the words (`z-index: -1`; `EmptyState`'s dot row is `isolate`). `StatusDot` gains `ember` (the resolved queue's dot only; `EmptyState resolved` passes it); `ember` holds the pool at opacity 0.8, still; with `glow` (the prop was `bloom`; the same per-page gates as before, now through `useOneOpening`: a clearing earns one opening, spent by the first mount of the all-clear (`EmptyState`'s `onGlow`) or by a read that stops answering, and no clock decides it, so a slow leave still opens and a failed read that recovers never reopens it; `useQueueCleared` and the findings' `useEmptied` both use it) the class `ember-open` plays `@keyframes ember` once (scale 0.2 to rest, opacity 1 to rest) over `--duration-flash` on `--ease-flash`, beside the built `lift-in`; no overshoot. Never on a ring (`ring` draws no glow), a search that missed or a read that failed (no `resolved`, no dot). Only under `(forced-colors: none) and (prefers-reduced-motion: no-preference)` does it open; reduced motion leaves the ember at rest; forced colours draw no light.
      - **It replaces `bloom`.** The `bloom` keyframe and utility (the flat disc with an edge) and their reduced-motion rule are gone. The other dots that bloomed (a copy's confirmation, an action's outcome, a refreshed read, the mark's welcome, a fold heading's and a storage row's dot) now open the same light once and are gone (`glow-once`, the trail's `answer` keyframe, none under reduced motion); the mark's takes the mark dot's colour (`--dot-light`), and the trail's read dots (`.trail-answer`, gone) are the same rule. Light is never in front of words: nothing isolates the dot, each line it sits in is `isolate`. A stale card's ring never wears it, and only a healthy dot wears light (rule 5). The prop is `glow`; the class is `dot-glow`.
      - **Copy.** Not changed. "That was the last one. Nothing needs you right now." is the board's warm line; Home says "Nothing needs you right now." only, and each queue's own all-clear line is the copy guide's. Putting the board's first sentence on Home is the owner's call (it would need the copy guide's sanction and a trace through every surface that says it).
      - **Layout check.** A new `ember` rule (`detect.js`): the glow's box, read from its computed inset, must sit inside the card's inner box, every clip and the window. A scroller clips only on the sides it cannot scroll past (`clipRect(el, true)`), and one `pastEdge` helper serves the lamp, place, trail and ember rules.
      - **Contrast.** The pool takes the hour's weight like every other pool (`--pool-k: calc(44% * var(--hour-weight))`, so 33% in the light room, dark unchanged). The dot against the paper bare is 4.2:1 or better in both rooms; over its own pool at rest the light room's dot is 3.19:1 on the canvas and 3.50 on white (it was 2.82 and 3.08 at 44%), dark 5.24 and 4.76.
      - **Guards** (`ember.test.tsx`, and the bloom tests moved to `ember-open` and `dot-glow`), mutation-checked, each red: the rest opacity dropped (2), always open (3), a ring glowing (1), the opening outside the motion guard (1), the old `bloom` keyframe back (1), a dot without `resolved` (1), the forced-colours rule removed (1).
    - [x] *The older lights (A-25 Q10, build order step 9, the last; decided by the owner on a side-by-side page, 21 Nov 2026). Branch `feat/addon-contract-a25-older-lights`, on `feat/addon-contract`. A-25 is complete.* The first build (both on the edgeless falloff) was judged and reworked: both stay as they were, and the change is small.
      - **`flash` is a tint, not a light** (owner: "okay with A"). `@keyframes flash` is `from { background-color: var(--accent-soft) }` with no `to`, so it ends on the element's own ground and nothing snaps at 900ms. `--duration-flash`, `--ease-flash`, once-only and the reduced-motion form are as before. No pool, no `--flash-k`, no pseudo-element. The utility's comment: "the row tints and returns; a tint has its thing's shape, a light has none" (RULES G32).
      - **"New since you looked" no longer flashes** (`CancelQueuedStep.tsx`): its resting `bg-accent-soft` and its words are the cue; a flash on that ground does nothing or fails contrast.
      - **The lit card keeps its edge-centred pool** (owner: "for lit card the 'before' one looks more visually appealing"): `ellipse 60% 90px at 18% 0`, cut by its card on purpose; the one written exception to "no box cuts a light" (RULES). The `older` layout-check rule, which only measured the redrawn lights, is dropped.
      - **Guard** (`flash-tint.test.ts`, mutation-checked: a `to` frame back, 1 red): the keyframe, no pool or pseudo-element, the consumer, and at the tint's peak every text token the consumers use at 4.5:1 or better and every dot at 3:1 or better on the canvas, a card and the rail, in both rooms. `cssTokens.ts` gained the shared `braced()` and `themes()`. `craft-ceremony.test.tsx`'s lit-card title no longer says "clipped by its bounds".
      - **Docs:** A-25 section 6 and its audit row, RULES G32 and the exception, NEXT 0n.
    - *Go to, as chosen (A-26 §0, round 4; the owner, 21 Nov 2026). Supersedes A-25 moment 6, which is not built on its own. Branch `feat/addon-contract-a26-goto`, on `feat/addon-contract`.*
      - **One list, one source.** `lib/go-to.ts`: `goToRows` (at rest the last place; `↓` five and "Needs you" last, only when something waits and the page is in the view's nav; typing best first by `rankName`, seven at most, each with its kind; in a scope that kind A to Z), `goToNamed` (under an answer, the places its question names, best first, three at most), `offeredScope` (two letters of a kind's word), `sigilScope` (`@` `#` `/`), `GO_TO_SCOPES`. Insights' question parser now reads names through the same `matchKey`/`rankName` (`components/insights/question.ts`), so Go to and a question rank a name one way.
      - **The pane.** `Modal` gains `glass`: the scrim is `go-to-scrim` (the lamp's `--light`, a registered colour; `data-scope` turns it), the panel `go-to-pane rim` (glass `--surface-2` at `--glass`, 78% dark and 84% paper, over `blur(36px) saturate(170%)`, radius 28, solid under `prefers-reduced-transparency` or without `backdrop-filter`). The light behind the pane is `goToLightBox`: on the lamp, or the window's middle when the lamp is within 280px of an edge, and never larger than twice the lamp's distance to the nearer edge, so it is spent inside the window. `GoTo.tsx`: the lamp (`SyndraMark`) first, the pill, the raw field, the offer, one listbox; the chosen row a point of the lamp's light and its name in ink, no fill. (The phone sheet's embedded `GoToPanel` was removed in the review round below: one Go to.)
      - **The state machine.** Tab (only while offered) or a tap on the offer or a sigil enters a scope; Backspace on an empty field leaves and gives the typing back (a sigil gives nothing back); Esc clears what is typed or chosen, then closes; `↵` opens the chosen row or follows the answer, and `⌘↵` from a row under an answer follows the answer. The dialog's focus trap now leaves a Tab a field already used alone, as it does an Escape (in Chrome, React's sync flush removed the offer before the trap ran, so the field was the last stop and the trap moved focus to the pill).
      - **The gesture,** only on entering a scope and never when still: the lamp's colour (120ms), the press (`scale(.98)`, 100ms in, 140ms back), the beam and the rim's wave (`--reach`), the halo's flare, the light behind swelling .6 to .85 and back, all Web Animations on token curves. Reduced motion: the colour is there at once, nothing animates.
      - **Ask (Advanced only).** `goToAsk.ts` reuses Insights' `suggest`, `buildModel`, `useAnswer` and `toParams`; `⌘↵` or `↵` follows to `/insights?…`. The reads are open to every operator (topology `withUserAuth`; `insights/who` and `whatif` `withOperatorAuth`; a person's access self-or-operator), but Basic has no Insights page to follow the answer to, so Basic is not asked. `useTopology(enabled)` reads only while a question is typed.
      - **The owner's two additions.** Roles take the accent turned by -100 (aqua), so no kind is Syndra's violet; People +44 and Pages -46 unchanged; a browser that cannot turn a colour falls back to the accent. The hand's edge (A-25 moment 2, the same rule): the pane wears `rim` (card strength) and the scrim carries `data-hand` for the one `HandHost`; the edge lights in `--light` (the scope's colour in a scope); the lamp-lit hairline moved to `::before`. The pill is `rim rim-filled` (as `Button`); rows `hover-tint`, with the chosen row's look winning; the offer a quiet hover under `(hover: hover)`.
      - **Layout check.** `detect.js` gains `goto` (the light behind's box inside the window); `check.mjs` opens Go to by `/` on Home at every width and view, reads only that rule, and reports a Go to that does not open.
      - **Measured in Chrome** on the replayed staging tape (1440 dark, 1440 light, 390 light, 1440 dark still): no page error; the field's box 390, 96, 660, 68 at 1440 and 8, 12, 374, 62 at 390 in every state, as the board; the press at its deepest 0.98 at about 114ms, beam and wave at 1, all back by 1.1s; still: 0 animations; `--light` hue about 333 in People; `goto` 0. The hand: the pane's edge 0 away, 1 under the pointer, in the scope's colour in a scope, 0 with the pointer away; hovering the third row kept the first chosen; the pill lit and unmoved.
      - **Guards** (`GoTo.test.tsx`, `go-to.test.ts`, `go-to-glass.test.ts`, `Modal.test.tsx`), mutation-checked, each red: no Tab offer (4), a sigil kept as typing (3), no give-back (1), Esc in one stage (2), the answer not followed (1), `⌘↵` from a row not following it (1), no names under an answer (1), more than three of them (1), the gesture when still (1), the gesture on leaving (1), no `data-hand` (1), the pill not a `rim` (1), hover moving the chosen row (1), Roles back to the accent (1), the paper's glass changed (1), the light box not bound by the window's foot (1), the trap taking a spent Tab (1).
      - **Layout replay** of a fresh staging tape (420 page runs): 0 findings for `goto`, `edge`, `lamp`, `place` and `pill`, and none at 1100 to 1440. Six `clipped`/`heading` findings on `/insights?tab=trends` at 390 (`TrendsTab.tsx`, untouched here) were reported by staging's own code while recording, so they come from the data, not this change; they are left for a trends fix.
      - **Differs from the board:** the answer is Insights' own panel (its eyebrow, title and first two lines), not the board's head and body, so a what-if keeps "a preview, nothing changes"; the board's "Read from Zitadel at 14:32." is not shown, since Insights' answers carry no read time (a new field, not invented here).
    - *Go to, review round (Opus review of PR #233: merge after fixes).*
      - **Fixes.**
        - (1) Entering a scope puts the caret back in the field: a tap on the offer had left the focus on a button that was gone, the Modal's recovery put it on the pill, a phone's keyboard closed and Space or Enter left the scope.
        - (2) Every key is the input method's while it composes (`isComposing`, or Safari's 229): Enter opens nothing and Esc clears nothing. The Modal leaves a composing Escape alone too.
        - (3) A paste that starts with a sigil (`@pri`) enters the scope and keeps `pri`.
        - (4) A question is asked once the typing has rested a settle (`useDebounce`, `SETTLE_MS`): no read per key, the last answer stays meanwhile. Basic and members make no Insights or topology read (a request spy holds it).
        - (5) One `role="status"` line outside the listbox says entering and leaving a scope, "Working it out.", the answer, a failed answer and "Nothing by that name". The listbox now holds options only; what came of the typing sits beside it (`go-to-say`).
        - (6) `hover-tint` is a utility and outranked the components-layer no-fill, so only rows that are not chosen wear it.
        - (7) A failed answer says what to do: "Try again" (refetches what failed) or "open Insights".
        - (8) The pane floats under a phone's notch: `--go-to-top` lives on the scrim (96px; on a phone `max(12px, env(safe-area-inset-top))`), its padding and the list's height read it, and the Modal's classes no longer set it.
      - **Refinements.**
        - `goToLightBox` and its resize effect moved from `Modal` to `GoToPane`, so Modal stays generic (it keeps only `glass`'s classes and `data-hand`).
        - One Go to: `GoToPanel`, `GO_TO_ESC`, and the grouped `goTo`, `narrow` and `flatItems` are gone. The phone nav sheet's field is a door: it leaves by history as any dismissal does, and once the sheet has left the page it asks the one Go to to open (`OPEN_GO_TO`), so Go to never opens over another dialog. The catalogue tests now read `goToRows`; the design-sync preview for `GoToPanel` is removed.
        - The chosen row's point is the row's own `::before`, so it moves with the row (no layout effect, no stale place after a resize or a growing answer).
        - The halo's flare is capped at 1.15 (`GO_TO_FLARE`): at 0.92 of the mark it reaches 33.8px of the field's 34 (29.7 of 31 on a phone). The beam's box is now the bar's (half the field each way, to the pane's right edge), so it is spent at the bar's edges and the pane no longer cuts it.
        - The redundant reduced-motion `transition: none` is gone (the one global rule collapses every transition). Inside a scope the rows no longer repeat their kind.
      - **Layout check.** `goto` now enters a scope by `@` after opening Go to. With the gesture's animations held at their furthest frame, and again once they are done, it measures the halo, the beam and the scope's light against the pane, and the light behind against the window.
      - **The rule, checked against the code it replaced:**
        - Recording a fresh tape from staging, which ran this PR's first build, it reported that build's beam: 4.8px past the pane's left, 6.8px past its top and 137px past its right, at every width in Advanced (48 findings).
        - Locally, with the halo widened and the scope's light raised by an injected style, it reported both.
        - On this build, measured in Chrome with the gesture held at its largest, everything is inside the pane: the halo 4.1px in from the left and 2.1px from the top, the beam exactly at the pane's edges, and the scope's light 6px from the top.
        - A real click on the offer left the focus in the field, and the next keys typed into the scope.
      - **Replay** of that tape into this build (420 page runs): 0 findings, apart from the six Trends findings at 390, which PR #234 fixes.
    - *Go to, second check (Opus, of the review round).*
      - (1) **An answer to the question no longer in the field is never followed.** While the typing has not settled (`stale`), the last answer stays where it was, quieted (`opacity-60`, `aria-disabled`), so nothing flickers per key. ↵, ⌘↵ and a press all go nowhere, and the status line says "Working it out." instead of its title.
      - (2) **Safari's composition Escape** (`isComposing` false, keyCode 229) is left to the input method by the Modal, as `isComposing` already was. Any other Escape still closes the dialog.
      - (3) **"Try again" asks again only for the answer's own failed reads:** the map, Insights', and a person's access. Nothing else in the app is reset.
      - (4) **Known iOS limitation, not fixed.** Go to opens after the tap that asked for it: after the phone sheet's history close, or after a state change from the top bar's button. iOS opens the keyboard only for a focus made inside the tap, so on an iPhone the field is focused but the keyboard waits for a tap on it. The usual trick is focusing a stand-in input inside the tap and handing the focus over. It would not survive the sheet's own focus restore on close, so it is not a small change.
      - **Mutation-checked, each red:**
        - the stale answer followed by Enter (1), said as the answer (1), not quieted (1), followed by a press (1);
        - Safari's 229 Escape closing the dialog (1), and Escape never closing (4);
        - Try again resetting every failed read (1).
      - **Mutation-checked, each red:**
        - offer tap dropping focus (1); IME guard gone (1), and 229 not read (1); the Modal closing on a composing Esc (2);
        - a pasted sigil's words lost (1); no debounce (1); Basic asking (2);
        - no status region (6); the status line inside the listbox (6); leaving not said (1);
        - the chosen row tinted (1); Try again doing nothing (1); no notch (1); the flare at 1.34 (1);
        - the kind repeated in a scope (1); the sheet never asking (1); the sheet asking before it has gone (1).
  - [x] 7w-5 Access over time (§9 vivid, Q11, read proposal Q-W3-1; the owner, 19 Nov 2026: "i dont see the access over time chart anywhere", which settles both as yes). Board part 3, frame 19 (`person-vivid`).
    *Done (branch `feat/addon-contract-access-history`):*
    - **The read.** `GET /api/v1/users/{id}/access-history?since=YYYY-MM-DD` (`handlers/access_history.go`, route beside `/access` in `router.go`), `withOperatorAuth`: never a member's, not even their own. `since` is a makerspace day from a year before today to today (`services.historyStart`, else 400 `VALIDATION_FAILED`, `details.since`). Answer: `{since, truncated, roles: [{project_id, project_name, project_name_resolved, role_key, standing, ends?, runs: [{from, to}]}]}`, `from` null where no record says when a run began, `to` null where the records give it today.
    - **Where each date comes from** (`db.AccessHistory`, one query, plus the ledger read `collectUserRoles` already makes): a direct grant held today, `direct_role_grants.created_at`; a bundle held today, `user_bundle_assignments.assigned_at` through its pinned version (`bundle_version_effective_roles`); a direct grant that ended, `direct_grant.removed` / `direct_grant.revoked_by_expiry` (they name `project/role`), dated from the latest approved `access` request for the role with no other ending between; a bundle that ended, `bundle.unassigned`, its roles from the version the person was last moved to (`bundle.holder_moved`) or else the one latest published at its `bundle.assigned` row, which is what `AssignBundleAndEnqueue` pins, and from that row unless moved. Overlapping or touching spans are one run (`foldRuns`).
    - **Held today is observed.** `standing` is `Standings.Of` from the same `personSnapshot` the person page reads; the runs only explain it. A role Zitadel shows with no open record gets an open run with no start.
    - **Never a guessed date.** `from` is null for: a rule's role (`mapping` reasons; no record dates a rule's giving, so even beside a dated grant the start is unknown); a role Zitadel shows that Syndra never gave; a bundle holder moved between versions (`assigned_at` is the first version's day); a grant given by hand and since ended (`direct_grant.upserted` names only the ledger row the removal deletes: NEXT 0l has the write-side follow-up); an ending whose start fell past the cap. Not drawn at all, since no record names the role: a role a rule stopped giving, a role a retired bundle carried, and `direct_grant.revoked` (a role Syndra never gave, taken back).
    - **Bounded.** A year at most; at most 500 spans (`AccessHistoryCap`, asked for 501 so `truncated` is exact), live bundles first, then the newest endings. No N+1: one spans query, the ledger, and the snapshot's fixed reads; every audit read in it, the lateral ones too, is by target, action and time, on `idx_audit_logs_target_action_created` (migration 000102, expand-only).
    - **The card** (`components/people/AccessOverTime.tsx`, words and places in `accessRuns.ts`), on the person page above the projects, `isOperator` and `AdvancedOnly` (not mounted, so not read, in Basic). This makerspace year on one axis (1 Jan to 31 Dec, `civilDay`, never the browser's zone), each term marked where it starts and today in violet; a line per role, A to Z by project then role name; ink held today (`holds(standing)`), grey ended or given and not held, the dashed amber run from today to its end when held and ending; a run that began before the year starts at the axis's edge, and one with no start on record from where the run before it ended (or the edge, when it is the first); its words say which. Each line is one sentence for a screen reader ("Laser cutting · Operator, given 3 Aug, held today, ends 12 Dec"); the visible words are the board's ("Since 14 Aug · ends 30 Nov", "2 Feb to 14 Aug", "Start not on record"). The caption says the dates are Syndra's records and today is read from Zitadel. While the read is out it holds a line per role held today, in the same type, so it lands without a jump. `useAccessHistory` beside the person reads; a look-again (`observed`) refreshes it with the access view.
    - **Guards.** `services/access_history_test.go` (a grant then a revoke, and given again; held today with no record; a bundle-given role; an expiry, by hand with no start; a rule's role beside a dated grant; touching spans; the cap; `since`'s bounds), `handlers/access_history_test.go` (a member refused for another and for themselves, the read never run; the shape, nulls present; 400 naming `since`, 500 on a failed read), `db/access_history_live_test.go` (the query against the schema in a rolled-back transaction: live bundle, ended bundle through the pinned version and never a later one, the request-dated ending, the undated expiry, the window, another person, the cap's order; run on `syndra_test` at 94 with migrations capped, dumped, fingerprinted, restored, fingerprint identical), `AccessOverTime.test.tsx` (sentences and order; ink, grey and dashed; the edge and its words; the makerspace's zone for a day and for the year asked; the held height; nothing and cut short; terms and today), `PersonAccessOverTime.test.tsx` (operator in Advanced; not in Basic and not read; never a member's), `invalidationMap.test.tsx`.
    - **Mutation-checked** (each red): the route on `withSelfOrOperatorAuth`; the 400 mapping; each `since` bound; the cap off by one, both ways; the rule's undated start (caught a wrong reason kind, `rule` for `mapping`); the observed-only run; the standing dropped; touching spans split; undated spans sorted last; a live grant undated; `ends` dropped; in SQL, the window, version resolution, the request's no-ending-between, the moved holder's start, the order, the person filter; in the UI, a UTC day, the browser's year, ink from the run not the standing, the dashed run for a role not held, "held today" for every standing, a guessed start, the before-year edge, the order, the held height, the cut-short caption, the operator gate and the Advanced gate.
    - *Review round (PR 217, the lead):*
      - **(1) A start no record gives never bridges a gap.** `foldRuns` joined a span with no start into whatever run came before it, so a grant Feb to Apr and a grant by hand that expired in Jul became one run, May and June drawn as held. Now a span is placed by its start, or with none by its end; one with no start joins only a run still open, and one that ends inside the run before it adds nothing. The card draws such a run from where the run before it ended, or from the edge when it is first (`placeRuns`). Guards: `TestFoldRuns_JoinsWhatTouches` (the Feb–Apr, Jul case; one ending inside; one before an open run), `AccessOverTime.test.tsx` (the later run starts at the earlier one's end, the first at 0%).
      - **(2) The cut-short words say what is true.** The cap leaves whole older spans off: "Only the latest 500 changes are drawn; earlier ones this year are left off." (and the `Truncated` comment).
      - **(3) The index.** Migration 000102 (`idx_audit_logs_target_action_created`, up and down; 102 is free on every open branch). Not run on any live database: `syndra_test` stays at 94, and the live test reads through the capped migrations.
      - **(4) Retired bundles.** `RetireBundleAndEnqueue` deletes every assignment with no row per holder, so those runs cannot be drawn: said in the spec and NEXT 0l. The no-op `retired_at IS NULL` join is gone.
      - **(5)** Pinning the version on `bundle.unassigned` is a write path: in NEXT 0l beside `given_at`, for the owner.
      - **(6–7) Words.** An ended run with no start: "Start not on record · ended 14 Aug". One name for a line, seen and heard: "{Role} in {Project}" (`lineName`).
      - **(8) The held height.** Ended roles can't be known before the read, so the skeleton holds the lines held today, one at the least; the comment says so.
      - **The board (part 3, frame 19), exactly:** the dates in `text-fine`; rows 9px above and below, the body 4px from the caption; each cell carries its own rule, so the divider breaks at the 16px column gap; every term's label; no tick. The chart is one image named by the board's sentence ("Four roles this year. Two are held today.", `chartSummary`), with the lines' sentences in a list beside it for a screen reader.
      - **Mutation-checked** (each red): an undated span joined to an ended run; undated spans sorted by start only; the contained case dropped (two overlapping runs); the later undated run drawn from the edge; "Start not on record · ended" back to "Ended"; the image's summary dropped or miscounted; the line name and the sentence worded apart.
  - [x] 7w-6 Built as drawn (the owner, 19 Nov 2026: "please check the boards and ensure things are actually built like they are drawn"). From the fidelity audit of A-24, D-1 part 1 and A-23 part 5 against staging (branch `fix/board-fidelity`). Type sizes stay D-1's (34 / 22 / 56): the A-24 boards were drawn before D-1. Out of scope here: the TrueNAS Accounts table (PR 216), plates, the door and Home's arrival (PR 213), the ledger line and access over time (built elsewhere), the rail mark's cut states.
    - **Go to (A-24 frame 22).** Pinned at a fixed top from the tablet step (`Modal pinned`: `tablet:items-start tablet:pt-24`, bounded at `100dvh-112px`), so it no longer jumps as the list changes. No "Go to" heading and no boxed field: the bare field row (`Input bare` in `field-row`, the row's rule under it, sticky over a long list), named by the dialog's `aria-label`. The ring would draw the box back, so the row's rule takes the focus ring's colour (`@utility field-row`), as the selection seat moves its ring. The phone's sheet keeps its boxed field.
    - **The term line (A-24 frames 2, 2p).** No dot of its own (review fix 11's dot is gone, as the board draws it); a grid, `auto 1fr auto` up to 760px from the tablet step, so the track is the row's free width (about 480px at 1440, as drawn); on a phone the words share the first row and the track takes the full width under them, so "Ends" never wraps alone. The board's hairline (1px `--line-strong`, the days gone in `--muted`), today 7px with its `--accent-soft` halo, words faint. Each amber mark reads its day under the pointer (a native `title`, "Access ends for two people on 24 Nov.", on a 10px reach). The Checked line's links keep their 44px reach without growing the row (`-my-2.5` below desktop), so the phone's head lines sit 24px apart, not 36.
    - **TrueNAS (D-1 part 1 frame 2).** The regions' container is a flex column: SectionPill gives its gap back with a negative margin, which a grid track can't take below 0, so the first region sat 40px low (79 under the read line, drawn 41). SectionPill's doc states the contract.
    - **The line that knows (A-24 frame 18).** `personLine` returns its ladder's tone with its words (`HeaderLine`): amber when it names an end or a role not in Zitadel, grey otherwise; drawn as a head line, its StatusDot then its words, text-small; the dot's seat is kept, unseen, while checking.
    - **A member Zitadel has no account for.** `GET /users/{id}/access` answers 404 only for `ErrUserNotInDirectory` (Zitadel has no account with the id, after the live lookup); any other failure is 500, never "no such person". My access keeps the greeting and says "Zitadel has no account for you, so there's no access to show." with "Ask makerspace staff to check your account."
    - **The usage card (A-23 part 5 frame 23).** No card for a person with no account and nothing of theirs counted (the board draws no empty form); with anything counted it stays, so it is never nowhere. A person's card keeps no empty action seat, so "Counting began" ends at the card's edge; a system's card keeps its seat.
    - **The trail (A-24 frame 14).** `PendingTrail` (`ChangeTrail.tsx`): while a give or a direct revoke is pending, the route under the forecast as the board draws it, each step its name as a label with no full stop beside an open ring (`ROUTE`: Recorded in Syndra · Sent to Zitadel · In Zitadel / No longer in Zitadel). The ring says "not yet seen", so no label claims anything; nothing settles before the answer, whose own `ChangeTrail` takes its place and settles each step only from what it observes. Not for cancelling a give that never went.
    - **Still different from the boards, and why:** Go to's rows keep the dialog's 24px inset where the board has 18; the term line's readout takes the term's day's seat, which is held as wide as its widest reading so the track never moves under the pointer, so with marks on it the track is about 50px shorter than drawn; the line that knows keeps the lede's seat under the meta row (the board's -8px pull is the old density); a plain holding's dot is grey, a tone the board doesn't draw.
    - **Guards:** `GoTo.test.tsx` (pinned top, never centred; no heading; the field row bare); `craft-places.test.tsx` (no dot, a grid with the track on its own full-width phone row, the readout by pointer, focus and arrows, the line's tones); `PersonHeaderId.test.tsx` (the dot's seat kept while checking; amber when it names an end); `HomeColumns.test.tsx` (the Checked links' 44px reach given back); `design-system.test.ts` (the regions a flex column with the pill first); `directory_test.go` (Zitadel's 404 is no such user, any other failure an error); `UsageCard.test.tsx` (no card with no account and nothing counted, the card with anything counted, the card with an account; no empty seat on a person's card); `MemberAccessStanding.test.tsx` (the 404 keeps the greeting and says it; other failures still "Couldn't load"); `craft-finishing.test.tsx` (PendingTrail is the route's names beside open rings), `GrantForecast.test.tsx` and `RemovalForecast.test.tsx` (a pending give and revoke render it; a cancel doesn't); `handlers/views_access_test.go` (404 only for no account; a failed read is 500).
    - **Mutation-checked** (each red): Go to unpinned; a heading back; the boxed field; the track off its phone row; a wrapping row; the marks' readout dropped; the Checked link at 44px; the regions a grid again; the empty card back; hidden when any metric is empty; the empty seat back; the line's dot untoned; the ladder's end grey; the 404 as a load failure; "Recorded" settled while pending; a Zitadel read settling a pending step; the give's pending trail gone; the sentinel unwrapped; every failure a 404.
    - *Review round (PR 218, the lead):*
      - **(Bug) A member Zitadel has no account for was still a 500 on real Zitadel.** `GetUser` never answers `nil, nil`: a missing user is `*zitadel.StatusError{Code: 404}`, which `FindUser` wrapped as a failed read, so `ErrUserNotInDirectory` fired only for the demo source and stubs. `FindUser` (`directory/zitadel.go`) now takes a 404 as "no such user" (ok false, no error); the dead `zu == nil` branch is gone. The test mock answered `nil, nil` and hid it: it now answers 404 as Zitadel does, with `FindUser_A404IsNoSuchUser` and `FindUser_OtherFailuresAreErrors`. Mutation-checked: a 404 not recognised; every StatusError taken as no such user.
      - **(1) The pending trail is the board's:** the route's names beside open rings, not "Not … yet" ("Not recorded in Syndra yet" was itself unobserved). `PendingTrail` maps `ROUTE` straight to rings; `trailSteps` lost its `recorded` flag.
      - **(2) The term line reads a mark on hover or focus, as the usage band does:** the track is one focusable image, ← and → move between marks, a pointer on a mark reads it, and the words go to the line's live readout in the term's day's seat (tinted mark, colour only). One `endsOn(days)` names the track and each reading.
      - **(3) Go to's focus is the house ring's 2px** (the row's rule and `inset 0 -1px` in `--focus-ring`), and the row's inset is the bare input's own, so the whole row takes the press.
      - **(4) One renderer:** `DotLine` (`StatusSeat.tsx`) draws My access's line and the line that knows (the dot's seat kept while there is no tone); My access's greeting is one `hello`.
      - **(5) Render tests, not source reads:** a pending give and a pending revoke render the ringed route (`GrantForecast`, `RemovalForecast`; none for a cancel); the Checked links' reach on the rendered Home (`HomeColumns`); the readout by pointer, focus and arrows (`craft-places`).
      - **Mutation-checked** (each red): pending steps settled; the labels as sentences; the revoke's pending trail gone; a trail on a cancel; the track not focusable; no hover reading; no arrow keys; Go to's focus at 1px; the inset back on the row; the Checked link at 44px; the dot's seat dropped while checking.
- [x] 7x The access token never reaches the page (security, 19 Nov 2026). `getSession()` returned the Zitadel access token on `SessionUser`, and `layout.tsx` (`AppShell`) and `page.tsx` (`Home`, `MemberAccess`) passed that session to client components, whose props are serialised into the RSC payload, so any script on the page could read the bearer the httpOnly cookie was meant to hide.
  *Done (branch `fix/session-token-client`):* `SessionUser` has no `accessToken`; `getAccessToken()` beside `getSession()` in `lib/session.ts` (one cookie read, `readSessionCookie`, with the same signature and expiry checks) is the only way to it, and `lib/session.ts` is `import "server-only"` (Next resolves the marker; vitest aliases it to Next's server-side no-op). The proxy route asks for the token only for an OIDC session and still refuses one without it; demo sessions keep the shared key. Cookie format, sign-in and sign-out unchanged. Checked the other server-to-client props (every server page and the layout): ids, route params, booleans, the view cookie, zone and locale, and demo identities' id, name and role; no other secret.
  - **Guards:** `session.test.ts` (the `SessionUser` type has no `accessToken`; an OIDC session's JSON never contains the token; `getAccessToken` is null with no, expired or demo session); `route.test.ts` "proxy route bearer" (an OIDC session sends `Bearer <its token>`, one without a token is refused before the backend, a demo session never sends a session token); `session-token-boundary.test.ts` (under `app/` and `components/`, only the proxy and callback routes name the token; under `app/`, `components/` and `lib/`, only the proxy calls `getAccessToken` and only `lib/session.ts` defines it); the demo case also checks the bearer is the shared key and `getAccessToken` is never called.
  - **Mutation-checked** (each red): the token back on `SessionUser` (type and runtime); the proxy sending the shared key for OIDC; the refusal dropped; a page importing `getAccessToken`; a `lib/` helper wrapping it; the proxy calling it for a demo session; the demo bearer not the shared key; a client component (`AppShell`) importing a value from `lib/session` fails `bun run build` on `server-only`.
  - Main has the same code: NEXT 0m.
- [x] 7y The layout plague (the owner, 19 Nov 2026: "the alignment issue with ui elements is present like a plague. fix it"). From a measured audit of staging: 648 page runs at 390, 768, 1024, 1100, 1280 and 1440, in both themes and both views. Its 15 worst defects come from 8 root causes. Fixed at the shared root, not page by page (branch `fix/layout-plague`).
  *Done (19 Nov 2026):*
  - **RC2, a leaving row could not be narrower than its columns.** `.leave-row > *` now has `min-width: 0` (`globals.css`). Before, a grid item's automatic minimum was its min-content, so People at 768 was a 602px row in a 462px card. Projects and Roles had the same fault.
  - **RC3, headings typed beside their rows.** `CardColumns` now requires `cols`, so a heading can no longer be typed without its column list. Every table is on one `Columns` list: Apps, Roles, Projects, Connected systems, Bundles, Roles that wait, Policies, a project's page, the event list, Service accounts, Zitadel grants and projects, ClaimHistory, DebugMode and SignInRow, BundleVersions, RecordsList, RoleWordsTable, Insights usage and ClaimsDelivery.
  - **RC4, the column budgets.** Each column is now a fixed D-1 seat (`NUMBER_COL`, `STATUS_COL` in `Card.tsx`), or a share with a minimum and no basis. Lines break on those sizes, so headings and rows break at the same widths. A row whose words fit keeps its seat on line 1, and a row that cannot fit wraps instead of clipping. Every link row is `tablet:flex-wrap`, and the `flex-nowrap` on People, Pending and Requests is gone.
    - Drift's Who and What (`RowSeats.tsx`, minimums 130 and 90) sum to 678px with the wide seat, and to 720px with Select's checkbox column. Both fit Drift's narrowest desktop row (728px at 1080).
    - TrueNAS accounts sum to 874px, one line from a 1220px window.
    - Pending's Change and Requests' columns had no minimum, or had one that could not fit.
    - Home's "New people": the name is the share and the words keep their width.
    - The wide seat stays 300: Drift's three verbs measure 291px.
  - **A seat that holds a button's reason** is exactly the table's seat width (`.row-seat:has([data-reason])`), and the reason wraps inside it. On Drift, the disabled Adopt's reason widened the seat to 522px and pushed the row's columns out from under their headings.
  - **RC1, a person's role row** (`PersonAccess.tsx`, `ROLE_COLS`) is a container query on the row (`@container`):
    - from 878px: name 230, source at least 160, status 240, then the seat;
    - below 878px: the name on its own line, then source and status on half a line each, then the seat.
  - **RC5, Drift's heading.** Why heads the headings' detail line (`CardColumns detail`), under the column headings and starting at the text's edge, where every row's reason is. A right-aligned heading's sort arrow hangs before the word (`SortHead`).
  - **RC6, Home's "Checked, nothing waiting".** Each link's 44px reach is now its own line until desktop, with no negative margin, and the separators are MetaRow's dots.
  - **RC7, a project card's header.** The tally and the sign-in line are in `MetaRow`, so a wrapped line never starts with "·".
  - **RC8, a heading over an avatar, a dot or a chip.** D-1 part 2 frame 10 draws Person over the avatar and Status over the dot, so a heading sits over its column's box. The browser check compares a left-aligned heading with the column's box, and a right-aligned one text to text. This is written into D-1 §4 rule 10.
  - **The layout check** (`ui/scripts/layout-check/`, `bun run layout:check -- --base …`): the audit's detector in Chrome through `playwright-core` 1.64.1, run against system Chrome. It exits 1 on any of these:
    - an overlap over 2px;
    - text clipped outside a truncate;
    - a heading more than 4px off its cells;
    - a seat off its first line when the row fits;
    - a page that scrolls sideways.

    `--record` and `--replay` check a local build against staging's data. It is the pre-merge check for UI PRs (CLAUDE.md, and the README beside it), and it is not in CI.
  - **Guards (jsdom):**
    - `unsizedColumns` (`test-utils/layout.ts`, run after every test): every heading column has a width or a minimum.
    - `headsOffRows` now stops at a `role="table"`.
    - `@ts-expect-error` on a `CardColumns` without `cols` (`RowSeat.test.tsx`).
    - `design-system.test.ts`: the seat's `--seat-w`, the reason cap, and `leave-row`'s `min-width`.
    - `PersonAccessLayout.test.tsx`: the role row's columns, and the header's MetaRow.
    - `HomeColumns.test.tsx`: no negative margin, no typed dot.
    - `UnexplainedAccessSections.test.tsx`: Why is the detail line.
    - `AccountsTable.test.tsx`: the minimums plus the wide seat fit in 934.
    - `SortHead.test.tsx`: the arrow sits before the word in a right-aligned column.
  - **Mutation-checked** (each one went red):
    - `cols` made optional (`tsc`);
    - Apps' Project back to `min-w-0 flex-1` (8 failures);
    - the role row's source back to `min-w-0 flex-1`;
    - a typed "·" back in the project header.
  - **Measured:** the layout check against this branch, run locally with staging's data replayed, gives 0 findings at all six widths in both views (see the PR for the page list). On staging's current build it gives 378. Still to do: the lead deploys the branch and re-runs the check against staging. `ready-for-human`
  - *Fix round (PR #219: Opus review, and the lead's staging run of 420 page runs with 0 findings):*
    - **(1) The pre-merge gate is the local replay.** CLAUDE.md names `layout:check --replay` against `bun next dev` as the gate, and the staging `--base` run as the check after deploying.
    - **(2) A tape never lands in the repo.** `--record`, `--replay` and `--tape` share one file, which defaults to `os.tmpdir()/syndra-layout.tape.json`. Recording to a path under the git root is refused with exit 3, and `*.tape.json` is in `.gitignore`.
    - **(3) A replay miss fails.** Every request the tape doesn't hold is listed as `tape missing …` and fails the run, because a 404 draws an error state that would otherwise pass.
    - **(4) SortHead's arrow selector** is one form: `in-[[data-col][class*=text-right]]`.
    - **(5) The class-echo SortHead assertions are deleted.** The browser check measures the arrow instead.
    - **(6) `NUMBER_COL`/`STATUS_COL` replace the hand-typed copies:**
      - roles that wait;
      - a project's page;
      - Projects' Roles;
      - Service accounts;
      - TrueNAS accounts;
      - ClaimHistory;
      - BundleVersions (both);
      - Requests' history date;
      - RecentChanges;
      - Systems' Status, now D-1's 240 seat: 858px, one line from a 1204px window.

      Kept on purpose: `TargetOverview`'s unmanaged standing seat stays fixed on a phone too (`w-(--seat-status) flex-none`, tested). The role row's `@container` widths can't take a tablet-prefixed constant.
    - **(7)** CardColumns' wrap comment now says what the code does. 7u-8's column list names `NUMBER_COL`.
    - **(8) A member's role row** (no seat) holds its three columns from 662px (`ROLE_COLS_NO_SEAT`).
    - **(9) Drift in Select mode, measured at 1080–1111:**
      - Before: the line was 760px in a 728px row, so the seat dropped under every row.
      - Now: Who and What are 130 and 90. That is 720px with the checkbox, and the seat is on line 1 at 1080 and at 1100.
    - **(10) Roles on a phone:** under a project's label the project cell is `""` (`:empty`) and `empty:hidden tablet:empty:block`, so it takes no line on a phone and keeps the column from tablet.
    - **(11) Home's 44px "Checked" line** is recorded in D-1 §9. Its comments cite the real criteria: the product follows 44px (`--tap`, WCAG 2.5.5 AAA and Apple's HIG), above 2.5.8's 24px minimum.
    - **Mutation-checked** (each red):
      - a member's row back on the operator columns;
      - the Roles placeholder back to an element;
      - the in-repo refusal off: a tape written in `ui/` (git shows it ignored);
      - a tape missing `GET /roles`: `tape missing …`, exit 1.
    - **Replay:** 420 page runs and 0 findings, against a fresh staging tape of 96 answers, with no request missing.
  - *Fix round 2 (Opus review of 3913392c):*
    - **(1) The in-repo refusal can't be talked round.** It refuses when:
      - the tape's folder resolves (`realpathSync.native`, which settles case and symlinked folders) inside any git work tree (`git -C <dir> rev-parse --is-inside-work-tree`), whichever checkout or worktree it is;
      - the folder doesn't exist (refused before the sweep);
      - the tape itself is a symlink (`lstat`).

      Each refusal exits 3. A tape is written 0600. Checked by hand:
      - refused: `./x.tape.json`, a case-changed path through `../../<worktree>`, the main checkout's root, a missing folder, a symlink;
      - accepted: a `/tmp/lc/..x/` folder.
    - **(2) A replay miss is its own exit, 4 ("tape incomplete").** Exit 1 stays for layout findings. CLAUDE.md and the README say that a branch adding a new read can't be recorded from staging. For those pages the gate is the staging run after deploying, and the PR says so.
    - **(3) The sort arrow is guarded directly** (`detect.js`): every `[role=columnheader] button > [aria-hidden]` must sit inside its `[data-col]`, pressed or not.
      - It found Drift's What at 1100: a label wrapping to fill a 121px column pushed the arrow 14px outside.
      - Fix: the label leaves the arrow its 16px (`max-w-[calc(100%-16px)]` on SortHead's button).
      - Mutation-checked: without the `in-[[data-col][class*=text-right]]` classes, Found and Attention were 14px outside, exit 1.
    - **(4) Systems: one line from 1080.** The system and its status are shares held to 130 each, with `NUMBER_COL` and the wide seat: 718px. D-1's fixed 240 status seat had made it 858px, so the seat dropped under every row from 1114 to 1203.
      - Measured: the seat is on the first line at 1080, 1100 and 1200.
      - The board (route-systems) draws Status 180 and a 250 seat; D-1's seats have no 250. At 1280 each share is 238px.
    - **(4, cont.) `NUMBER_COL`'s docstring** says what it is below tablet: no width of its own, a line of its own in a stacking row, and as wide as its words in a CardRow.
    - **Phone change, recorded.** Six cells that were fixed at every width are now fixed from tablet only, so on a phone they are as wide as their words:
      - a project's page's People;
      - TrueNAS accounts' Since;
      - ClaimHistory's date;
      - BundleVersions' holders' People and an includer's count;
      - Requests' history date.

      A number is the row's last part, so on a phone it now sits after its row's words, not in a 110px box. `TargetOverview`'s unmanaged standing seat keeps its fixed width on a phone, because its username column is laid out against it and a test holds that.
    - **(5) The review queues' budget** (`RowSeatsBudget.test.ts`, tokens read from `globals.css`):
      - Drift with the checkbox: 720px against the measured 728 at 1080; without it, 678.
      - Expiring access and Unfinished revocations at tablet, What at 90: 362px against 374 at 720, and with the checkbox 422 against 422 at 768.
      - Mutation-checked: What back at 120, both tests red.
    - **(6) Recording awaits every body** (`Promise.allSettled`) before its context closes.
    - **(7) README: `/healthz` isn't on the tape.** The server's zone and locale read never passes through the browser, so locally dates show "UTC", labelled.
    - **(8) Nits:**
      - HomeColumns' comment no longer describes the old 24px lines;
      - SortHead's comment is wrapped to 100 characters;
      - projects-columns' helper finds the Roles cell by `hidden desktop:block`;
      - CLAUDE.md's dev command clears `ZITADEL_DOMAIN=`.
- [x] 7z Steady controls (the owner on staging, 20 Nov 2026: "there are lines on the sides of the selected pill"; "the placement of the navigation buttons are changing depending on which one is selected. that is very jarring to have the button you just clicked move away from you. please do a more elegant fix"). Branch `fix/steady-controls`.
  - **(1) One element, no seams.** 7u's three-part fill (two caps, a middle scaled on x and tucked 0.5px under each) doubled the alpha of a translucent fill (`bg-tint-3`, `bg-accent-dense`, the section pill's fill) where the parts overlapped: two vertical lines inside each end. `GlideIndicator` is now one absolutely positioned `rounded-pill` element, placed by `translate()` and sized by `width`/`height`, never scaled. `TUCK` and the caps are gone. `indicator-glide` transitions `transform, width, height` on settle's tokens; never `left`/`top`. The element is out of flow, so a size change lays out itself alone (this supersedes 7u's "transform only"). Reduced motion: still no class, so it lands at once.
  - **(2) Labels that never change width.** `SteadyLabel` (`components/ui/SteadyLabel.tsx`) wears `label-steady` (`globals.css`): its `::after` carries `attr(data-label)` at semibold with `height: 0; visibility: hidden; overflow: hidden`, so each label is always as wide as its chosen self. Worn by every option of FilterPills and Tabs, the two whose chosen option turns semibold. Tabs takes `count` on an option and reserves label and count together; `bundles/TabLabel.tsx` is deleted (BundlePage and RetiredBundle pass `count`). Segmented and the section pill are semibold throughout, so they reserve nothing; their guard is one weight for every option.
  - **(3) A choice's own control never pushes its row.** Insights Map's first row (`data-toolbar`) is the question, the lens pills and the view switch, whatever the lens. `LensChoice` is a line of its own beneath it, right-aligned under the pills from tablet up and stacked on a phone, inside `Unfold` (its gap folded with it), so the only movement is the content below easing down by one line. Each line says its words ("How far ahead", "Which group", "Team name"); a team's name unfolds on a line beneath the group, never beside it, so the select never moves either.
  - **Other rows checked** (a choice that inserts siblings beside itself): Storage's home folder limit (`HomeLimit.tsx`, `HomeMapping.tsx`), where "Up to a size" brings a 48px GB field beside a 37px Segmented and the `items-center` row re-centred the choice ~5px: the field is now `dense`, never taller than the Segmented. Ruled out, the control first in a start-aligned row and its neighbour no taller: Trends' period line (`TrendsTab.tsx`), a new bundle's "Start from" hint (`bundles/page.tsx`), a claim's fixed value (`ShapeCard.tsx`, dense Input and a small Segmented the same height). Expiring access's search and Select appear with rows, not with the window choice, after the pills (`FilterBar` slots): data, not choice, left as is.
  - **Guards** (each mutation-checked red, then restored): `glideIndicator.test.tsx` (a fill is exactly one element, no children, `rounded-pill`, translate with no scale, width and height set; the underline too) — red with a child part; `GlideIndicator.test.tsx` (every option of Segmented, FilterPills and Tabs has a `.label-steady` whose `data-label` is its text; a tab's count is inside its reserved words) — red without SteadyLabel in Select, and with Tabs' count left out of `text`; `design-system.test.ts` (`label-steady` is a heightless, hidden, semibold `attr(data-label)` copy; `indicator-glide` is `transform, width, height`) — red with `height: 0` removed, and with the transition back to `transform` alone; `MapTabSwitch.test.tsx` (for Ending soon, A group and a team, the first row holds Lens and View and not the choice, which is in the next line) — red with LensChoice back in the row; `HomeLimit.test.tsx` (the GB field is dense) — red without `dense`. `SectionNav.test.tsx` reads the fill as the one element.
  - **Real-browser check, by hand** (Chrome via `playwright-core`, the built CSS, four FilterPills options chosen in turn): every left edge moved 0.00px; without the steady label, 2.55px. The tape replay was not run: no tape exists locally and recording needs staging.
  - *Fix round (20 Nov 2026, review of #223):*
    - **(1) Swapping one lens for another moves nothing below.** `Unfold` eases opening and closing, not a swap, so a swap snapped the height (about 12px on desktop: 36px pills against a 48px Select; about 26px on a phone). The group Select and the team Input are `dense`, and every `Line` is `flex-col items-start` on a phone and, from tablet, a row at `min-h-(--control-md)`. Measured in Chrome (built CSS, server-rendered LensChoice): Ending soon and A group are both 36px at 1280, 44px at 900 and 68.8px at 390; the swap moves 0px.
    - **(2) The playbook.** CRAFT 1.7 names its one exception: an out-of-flow indicator with no children may transition its own width and height (one seamless element; only it re-lays out; MUI's Tabs indicator does the same). D-1's Tabs/Segmented/FilterPills motion row says the same, and that labels hold their chosen width.
    - **(3) Lean.** `SteadyLabel` is off Segmented and the section pill, where every option is already semibold.
    - **(4) Guards.** `design-system.test.ts` also holds `label-steady` to `display: inline-flex` (what lets `height: 0` take no height). `detect.js` gains a read-only `steady` finding: an unchosen option's `label-steady` narrower than its words at the chosen option's weight (measured in a detached copy, 0.5px of rounding), or reserving words other than its own (README table, `check.mjs` header).
    - **Fix-round guards, each mutation-checked red then restored:** `display: inline` on `label-steady` (design-system); Segmented with only the chosen option semibold (`GlideIndicator.test.tsx`, Segmented labels are one weight); the current section `font-bold` (`SectionNav.test.tsx`, every section at one weight); FilterPills without SteadyLabel (the guard narrowed to FilterPills and Tabs); the group Select, then the team Input, not dense (`MapTabSwitch.test.tsx`). `detect.js`'s `steady` run in Chrome on built FilterPills: no findings healthy; with the reserve removed (`content: none`), three (e.g. "People" 41.4px, 43.1px once chosen).
    - **CI (af6bd9b5): the design-sync preview.** `.design-sync/previews/Tabs.tsx` still passed an element as a tab label, which `bun run typecheck` (its second pass, `tsc -p .design-sync/tsconfig.previews.json`) rejects; `bun run build` type-checks `src` only, and typecheck had not been run locally. The preview passes `count: 7`, and `config.json`'s hand-inlined Tabs props (`dtsPropsFor`) take `label: string; count?: number`. The CI ui job's steps (install, test, lint, typecheck, build) were then run locally in order, all green.
- [x] 7za Layout regressions that waited in the data (20 Nov 2026: the layout check against staging, which runs d121f973, reported 19 findings, all light, Basic and Advanced, on `/system/targets/truenas` and two person pages at 768).
  - **Bisect.** A fresh staging tape (96 answers) replayed into a production build of each merge since PR 219: 59f34743 (PR 219 itself), 2a3e88d0 (223), 59848f77 (225) and d121f973 (226) report the same findings, number for number ("5px, needs 11px", "17px", "495px of 663px"). The code at the check's own merge reports them on today's data, so no merge since introduced them. So the variable is staging's data, which is restored nightly from production. Every finding needs a shape that 420 clean runs never met: an unassigned NAS account with a button in its seat, the first two people with a network storage account, folders and a hold row, and a reading with a note. The old data is gone, so which of these arrived after PR 219's run is inferred, not measured.
  - **Three row parts that stated no floor**, so flex could not move the seat to the next line:
    - *A setting row (label, value, seat):* the label was 170 from `tablet:`. At 768 the rail leaves a card 447px wide, so 170, the 200px seat and two gaps left the value 5px. `LabelRow` (`Card.tsx`) is now the one way to draw a setting's row. Its label (`row-label`, globals.css) sits beside the value only where the row holds `--seat-label` + `--seat-value` (160, new) + the seat it is given (`--seat-action`, or `--seat-action-wide`) + two `--row-gap`s. Narrower, it sits on its own line, so the seat stays on the value's line (D-1 §4). It is read from the tokens, and a container query can't read a custom property, so the switch is the label's flex basis: `max(--seat-label, (fits - 100%) * 999)`, held to the line by `max-width: 100%`. `100%` is the row's width. The same switch drops `row-top`'s padding when stacked (`--part-pad`). The value has a floor (`min-w-[min(var(--seat-value),100%)]`), so a long verb wraps the seat instead of crushing the value. On a phone an empty seat takes no line. Used by `PersonSystems.Row` (every person's system and storage card), the service account page and `AccountKind` (a wide seat, which squeezed its value the same way at 504–600px rows).
    - *Guards:* `design-system.test.ts` forbids `tablet:w-(--seat-label)` anywhere and holds `row-label`'s rule to the tokens. `LabelRow.test.tsx` checks that the label is a part of the row, that it reads a wide seat, that the value has its floor, and that an empty seat is hidden on a phone. Mutation-checked, each red (1 failure each): a hand-typed `tablet:` label, the value's floor removed, the label switch removed, the wide seat not passed, a hard-coded 566px threshold, the wide seat ignored in CSS, the empty seat drawn on a phone.
    - *In-between widths (860, 900, 950, 1060, 1180, beside the six), on the person, service account and TrueNAS pages:* two more starved shares, already on staging and fixed here. On a service account's Holds, the source column had no minimum beside a 260 name and the seat: 7px at 860, 47px at 900, and its "Direct" chip ran out. It is now held to 160, the source's minimum on a person's page. A system's settings row (`targets/SettingRow`) had a 240px value basis: at 950 "Mappings" dropped under a value that fitted (567px of 589). Its value is now a share held to `--seat-value`. The row itself stays as it is (below).
    - *Later, one path:* `targets/SettingRow.tsx:22` (a system's settings, a fixed 170 label, `flex-none`) is left as it is. Merging the setting-row shapes into `LabelRow` is a later "one path" change. `ready-for-agent`
    - *A reading (`TargetOverview` `Reading`):* `min-w-0` with nothing to truncate reported 0 to the line break, so at 390 "Answering" was cut to 65px of 82. It is now a share held to `min-w-[min(120px,100%)]`, so the seat wraps under it whatever the words, and its label wraps anywhere (a long hostname).
    - *An unassigned account:* a 200px basis made the line 688px, so at 1024 (663px) the seat dropped under a three-letter name that fitted. It is now a share held to a minimum, `min-w-[min(120px,100%)]`, as on "Accounts Syndra manages" (7y).
  - **Guard: not shipped. Why the check missed it, and why no committed tape.** These were not merge regressions: PR 219's gate and every PR after it replayed staging's data as it stood, and those shapes weren't there. A committed tape would carry them into CI. One was built from a staging tape: ring names, made-up ids, and a check that no word of any name survived. Replayed, it reproduced all 19 findings on d121f973 and 0 on the fix. Review withdrew it before commit. To keep lists in order and joins intact it kept each name's alphabetical rank, the length of each part of a name, digits and timestamps. With the member roster, that re-identifies people, and their roles and holds, in a public repo. A tape made safe enough to publish has to be written, not derived: ring-name people built to the API's types, with these shapes (an unassigned account with a seat button, a person with storage folders and a hold row, a reading with a note). That is a separate piece of work. `ready-for-human`: the owner decides whether it is worth one. Until then the gate stays manual, and the README now says a pass holds only for the data recorded.
  - **Measured:** a fresh staging tape (96 answers) replayed into a production build of this branch gives 420 page runs, 0 findings and 0 reads missing. Recording it from staging (d121f973) gave the 19.
- [x] **Trends at 390, 21 Nov 2026 (layout check on staging, 6 findings in the number columns).** Ending soon gave a project's name 66px on a phone (`1fr` beside two 110px seats). Now the name's track is `minmax(0,1fr)` and wraps, and the numbers sit in `--seat-number`, which a phone narrows to 4.75rem, the width "30 DAYS ↑" needs (globals.css, the phone block); from tablet it is the 110px seat as before. The heading row and the data rows share one grid (`ENDING_ROW`), and a test holds them equal. A fact's number no longer shrinks (`shrink-0`). Replay of a fresh staging tape: 420 page runs, 0 findings (6 before).

## 8. Reachability (owner, 21 Nov 2026)

Verbatim: "reachability to add-on and how to handle it should be standard part
of contract 2". Design §13. Survey: `research/reachability-survey.md` (21 Nov 2026), its
§7 ranks the 15 inconsistencies this slice closes. All on `feat/addon-contract`;
nothing merges to main alone. Each PR: elegant, standard, careful; one source,
one path; mutation-check every guard; Opus review after.

- [x] 8a The contract says it (files and guide only, no code).
  - `schema/common/stale.json`: `reason` is `cached | cooldown | unreachable`; `draining` and `never_read` removed, fixtures updated.
  - `registry/errors.json`: `REACH_NEVER_READ` removed; `NO_CURRENT_READ` means "the add-on holds nothing to serve"; `TARGET_UNREACHABLE` says it is the add-on's wire code only.
  - `addons/AUTHORING.md`: a section "When Syndra cannot reach you" (what Syndra does, that nothing is asked of the add-on but to come back with its mirror and call records); the two no-copy codes, with the 429 `FRESH_READ_COOLDOWN` and `STORE_UNREADABLE` exceptions; the `-stop-app`/`-start-app`/`-stop-addon`/`-start-addon` hooks; `health.reachable` as the kit now checks it.
  - `CHANGELOG.md` entry; design §4's `stale` line and §6's health paragraph point at §13.
  - *Built (21 Nov 2026):* as listed. The kit's hooks are written `-stop-app` and so on, the guide's one-dash style for every kit flag (Go's `flag` takes both). One code line beyond the files: the TrueNAS `/reach` answers `NO_CURRENT_READ` where it answered `REACH_NEVER_READ`, because `TestEveryCodeTheAddOnWritesIsInTheRegistry` refuses a code the registry dropped; the rest of 8g is untouched. Both validating ends refuse `stale.reason` `never_read` and `draining` (`TestSchemasRefuseWhatTheyShould`, `TestV2SchemasRefuseWhatTheyShould`); mutation-checked by restoring the old enum, red on both. The guide describes the outage hooks and checks 8f builds.
- [x] 8b Transport and monitor (backend; design §13.1, §13.2, §13.5).
  - `credentials.go`: `net.Dialer{Timeout: 3s}`, `TLSHandshakeTimeout: 5s`, constants in `deps.go`. Test: a blackholed address is `unreached` inside 4 s.
  - The dial resolves the registered host as an absolute name (root dot appended in `DialContext` unless an IP literal); URL, `Host` and SNI unchanged. Test: the resolver is asked for `name.`, an IP literal is untouched, the signed request is byte-identical.
  - `addons.Unread(reply) -> {reason, cause}` is the one mapping (`SubjectsResult.Halt()` calls it): the table of §13.1, one test row per cause, `wrong_host` from `pinMismatchError`. `target_unreachable` no longer produced.
  - The breaker remembers the cause that opened it; `ErrCircuitOpen` carries it and the time left.
  - Monitor: `State(target)` with `answering | app_unreachable | addon_unreachable | checking`, `cause`, `since` (absent when not observed), `checked_at`, `retry_after`; first probe decides from `checking`; three misses, one success; probes in parallel, 5 s each; one log line per change with `error.type`.
  - `main.go`: one `ProbeAll` before the listener serves.
  - Readers moved onto it: `propagation.addonReachable`, `describeTargetFor` (`view.Reachable`, `health: ""`, the per-load `Health` read), `statusOf`.
  - Drain: a row answered with a reachability cause spends no attempt; a shape refusal still does. Test: ten passes against a closed port leave `attempts` at 0 and the row queued.
  - *Built (21 Nov 2026):* as listed. `addons/unread.go` holds `Why`, `Unread` and `CircuitOpenError`; the breaker keeps the whole `Why` that opened it, so an app outage that opened it stays `app_unreachable`. Three calls the table did not name: Syndra holding no key (the secret did not load) is `wrong_host`, whose next step is the secret (fix round 1: now `no_key`, below); a feature the manifest does not declare, or a shape refusal, is `addon_outdated`; Syndra's own refusals before sending (no call record, no fingerprint, fresh out of scope) and any answered 4xx are `read_refused`, so they still spend attempts. A 2xx cut off is `timeout`, one that does not read (or is oversized) is `fault`. The monitor keeps A-01's pips (`recent`, `missed_since`, `last_answered_at`, `lifecycle`, `started_at`) beside §13.2's five fields, and is still read as `addons.HealthOf`; the roster's `status` is its state; the member view's `health` is its state, `reachable` and `reachable_since` gone (`since`). `target_unreachable` is no longer written for an add-on; Zitadel's sweep still writes it. `ProbeAll` is parallel with 5 s each; `main` runs it before `net.Listen`, then every 30 s. Tests (each mutation-checked red): `unread_test.go`, `health_monitor_test.go`, `TestAnOutageSpendsNoAttempt`, `TestTheStartupProbeRunsBeforeTheListenerServes`. Seams left: 8c reads `Unread`/`Why` and `HealthOf(...).State` to answer no copy undialed; 8d reads `ApplyResponse.Why`-style classes for `writeOutcome`; 8e redraws the UI's readers of the old words (`targets/page.tsx`, `TargetReading.tsx`, `TargetOverview.tsx`, `MyStorage.tsx`, `state.ts`, `drain-outcome.ts`, `format.ts`).
  - *Fix round 1 (21 Nov 2026, Opus review of PR 252):* every send where nothing went out (an outage, a lifecycle refusal) takes one path, `db.DeferPropagation`: pending, no attempt, `not_before` = now + max(Retry-After, the probe interval), so `ClaimDueRetries` sends it by itself and `retryingRow` (no longer `attempts > 0`) counts it; `ReleasePropagation` is gone. `TARGET_CHANGED` is `read_refused` (spends an attempt, backs off) and never counts toward the breaker. A secret Syndra cannot load is `no_key` (§13.1 row). A failed call record or body build is `read_refused`. Any coded reply is the add-on explaining itself, so only an uncoded 5xx is `fault`; the app codes are read before 429. Only the monitor's probe feeds it: `addons.Health` is the live read for `?fresh=true`, every page poll reads `LastHealth` (Kubernetes: only the periodic probe counts toward `failureThreshold`). A `checking` target halts the drain as `addon_unreachable`. Tests (each mutation-checked red): `TestUnreadNamesWhoIsDownAndWhy` (4 rows), `TestAClaimThatFailsIsNotAnOutage`, `TestTargetChangedNeverOpensTheBreaker`, `TestOnlyTheProbeMovesTheMonitor`, `TestThePagePollNeverReadsTheAddon`, `TestTLSNamesTheHostWithoutTheDot`, `TestARowNothingWentOutForIsDueAgainBySelf`, `TestACheckingTargetHaltsAsAddonUnreachable`. Live, passed (21 Nov 2026, on a throwaway copy of `syndra_test`): `TestDeferPropagation_SpendsNoAttemptAndIsTheBackgroundDrains`, `TestAnOutageLeavesTheRowToTheBackgroundDrainLive`.
  - *Fix round 2 (21 Nov 2026, Opus review of PR 252):* a deferred row (attempts 0, `not_before` set) shows its next try, try 1 (`RowRetry` no longer needs an attempt; with the background drain off it still says nothing), so `Delivery.Unsent` no longer calls it abandoned. `Retry-After` is capped at an hour where the transport reads it (`maxRetryAfter`, past the 30 min backoff cap; RFC 9110 §10.2.3 sets no bound), so no defer wraps `time.Duration` or overflows `make_interval`. The runner audits a deferred row it sends as an automatic try. An answering add-on in maintenance (`draining`, `read_only`) halts both passes on that switch (`haltReason`, one rule): nothing is sent or spent, the halt counts what waits, and the send's outcome names the switch and the step back (Make active under Maintenance). "Try again" on the health page reads live (`?fresh=true`), and a live reply says so (`live`); `LastHealth` of an unregistered target keeps its `not registered` error. Not built: routing `Delivery`'s `ScheduleRetry` through the defer for an unreachable add-on, because a delivery holds only Zitadel rows (`EnqueueParams` has no target) and `DrainBatch` halts only for Zitadel. Tests (each run red against the unfixed code): `TestDeliverChange_ADeferredRowShowsItsNextTryAndIsNotUnsent`, `TestRetryAfterIsReadAsDelaySeconds` (extreme rows), `TestDrainRevocations_AuditsADeferredRowItSends`, `TestAddonPassesHaltOnTheMaintenanceSwitch`, `TestLastHealthOfAnUnregisteredTargetSaysItIsNotRegistered`, `TestALiveHealthReadSaysItIsLive`, UI `Try again reads the health live` and `a system in maintenance is named by its switch`. Live, passed (21 Nov 2026, on a throwaway copy of `syndra_test`): `TestDeferPropagation_TheLongestRetryAfterSettles`.
  - *Fix round 3 (21 Nov 2026, Opus review of PR 252):* an answered `SetLifecycle` writes the new lifecycle into the monitor (`setLifecycle`, the one write beside the probe), so a Send right after Make active is not halted, and the Maintenance card shows the new state, without waiting for the next probe. `haltReason` reads a lifecycle it does not know as `read_only`, as the add-on does. The step back from maintenance says revocations go on their own when the background drain is on, and "send again" only of the rest. Tests (each run red against the unfixed code): `TestAnAnsweredLifecycleSwitchMovesTheMonitorAtOnce`, `TestAnUnknownLifecycleHaltsAsReadOnly`, UI `the step back from maintenance leaves revocations to the background drain`.
- [x] 8c One reply for every read (backend; design §13.3).
  - `read: {state, taken_at, reason, cause, detail, retry_after}` written by one function; every route of survey §3 rows 1–5, 8–13, 15–16 answers through it. No copy is 503 `TARGET_NOT_READ` + `Retry-After` in the existing envelope with `details: {reason, cause}`, and no data member.
  - Gone: `read`/`readable`/`usage_read`-as-state booleans, the 503 inventory body, `read_at`, `last_known`, `lastStorage`, `lastMappableValues`, and the codes `ADDON_UNREACHABLE`, `VALUES_NEVER_READ`, `TARGET_CAPABILITIES_UNKNOWN`.
  - `drift.Inventory` reads bindings before it can return; Syndra's own records stand outside `read`.
  - While the monitor holds `addon_unreachable`, a read answers no copy without dialing.
  - `ResolvesValue` returns the unread reason; mappings and allowances refuse to save unchecked (`errMappingTargetSilent` becomes the one path).
  - Usage cards: "not counted since {watermark}" takes its why from the monitor; nothing new is stored.
  - `reachability_test.go`: every such route (read from the router; a route without a row fails) against a stub add-on in each state of §13.1; asserts status, code, class and `noFalseZero`. Mutation-check: restore one `omitempty`-less `time.Time`, red.
  - *Built (21 Nov 2026):* `addons/read_state.go` holds `ReadState`, `ReadOf` (the one writer), `HealthState.Down`, `NotRead` and the held read: `Addon.read` answers the monitor's why undialed while it holds `addon_unreachable`; the probe alone still dials. A reply whose schema dates nothing (values, settings, `health.get`) is now, dated when Syndra asked. Every row of survey §3 answers through it: no copy is `writeNotRead` (`handlers/not_read.go`, §13.1's operator sentences as drafts for 8e's copy pass). `lastStorage`, `last_known`, `lastMappableValues`, `rememberStorage` and the usage card's recalled copy are deleted; `ResolvesValue` reads through `ReadValues` and fails closed, and `errMappingTargetSilent` is gone (one path, `notChecked`, "Not saved. Syndra couldn't check this value on {system}. Try again when it answers."). Usage carries the monitor's `why` while down. Tests (each mutation-checked red): `reachability_test.go` (13 routes × 13 ways, routes read from the router; a zero date, a route without a row, a held add-on dialed, a value saved unchecked each turn it red), `read_state_test.go`, `not_read_test.go`. Departures, for the owner: (1) the health route, `/me/targets` and `/users/{id}/targets` answer 200 with `read: none` beside Syndra's own records (the monitor and the log anchor stay visible during an outage; `TestACompromisedLogIsReportedEvenWhenTheTargetIsDown`), every other route 503; inventory and homes are 503, so their bindings and limits are not shown while the add-on is down, and `drift.Inventory` does not read bindings on no copy; (2) a cooldown with nothing kept is 503 `TARGET_NOT_READ` with reason `cooldown` on the inventory (was 429), the dormant list keeps 429 (owner Q1); (3) reconcile and rehearse rows drive the pipe through a stand-in for the service's Postgres first step; their kept answers are tested in drift and services.
  - *Fix round 1 (21 Nov 2026):* the dormant list inside a cooldown is 503 `TARGET_NOT_READ` `cooldown` like every read; 429 stays only for the dormant removal (`ReadCooledDown` gone). A dormant read cut at the add-on's cap says so in its own sentence. An allowance on a system with no manifest takes the mapping's path (503 "Not saved…", was 400). One fallback, `addons.WhyNoManifest`, which also names an open breaker. The add-on's `detail` goes first only for `app_unreachable` and `read_refused`; the product is the accepted manifest's. `storage` is set only when `read` is not none. `ResolvesValue` fails closed when the role map can't be read. The table adds four ways (held by the monitor, a 429 with nothing kept, `no_key`, an open breaker) and refuses a `0`, `[]` or `""` in a none `read`; services tests pin the rehearsal's and the dormant list's `NotRead`. Each mutation-checked red.
- [x] 8d Writes say one of two things (backend; design §13.4).
  - `writeOutcome`: `unreached` is 503 `NOTHING_SENT` + `Retry-After`; `indeterminate` is 202 `unconfirmed`. Adopt, release, lifecycle, settings, member password, member actions and the revocation's rotation answer through it. Gone: `TARGET_UNCONFIRMED`, `PREVIEW_UNANSWERED`, the `writeUpstream` 502 on these paths, the merged adopt/release sentence.
  - An interactive write to an `addon_unreachable` system is refused without dialing.
  - Revocation: `recorded_at` and per-system `landed_at` on the reply and the unfinished-revocations read; landed only on the add-on's `succeeded`. Audit carries both times.
  - Tests: each path against unreached and indeterminate; a revocation with the add-on stopped is recorded, queued, never terminal, and lands on the first pass after it returns.
  - *Built (21 Nov 2026):* `handlers/write_outcome.go` holds `writeOutcome` (503 `NOTHING_SENT` + `Retry-After`, the monitor's next probe when the add-on gave none, `details: {reason, cause}`; 202 `{status: "unconfirmed"}`), `outcomeAnswer` for a reply that carries one inside it, and `whySentence`, §13.1's drafts (a member reads one sentence for both outages). `addons.Held` is the one undialed gate: `addonop.Dispatch` (before the record and the member's rate count), `operate` (setting, lifecycle, before the row) and `Preview`; app down is not held. `addonop.Result.Why`, `ReadReply.Why`, `addons.HeldError` (read by `Unread`) and `addons.NotLandedError` (lifecycle) carry the why. Adopt, release (inventory and findings), setting, lifecycle, the dormant removal (whole request when held; per account `nothing_sent`/`unconfirmed`), password, physical action and its preview, and the revocation's rotation answer through it. Gone: `TARGET_UNREACHABLE` (member actions, password, setting), `TARGET_UNCONFIRMED`, `PREVIEW_UNANSWERED`, `RELEASE_NOT_CONFIRMED`/`ADOPTION_NOT_CONFIRMED` 502s, the merged sentences, `partialRevokeCopy`; a dispatch Syndra refused before sending is 500, never 502. Revocation reply: `status: recorded`, `recorded_at`, `landed_at: null`, the "still works" sentence, `rotation` when it did not land; the list carries `recorded_at`; `revocation.landed` audits both times on the add-on's succeeded; a queued add-on revocation no longer escalates on age (`WaitingAge` is Zitadel's rows). Tests (each mutation-checked red): `write_outcome_test.go` (seven paths × unreached/indeterminate, the preview, the dormant gate, member vs operator words), `held_test.go`, `TestAHeldSystemIsRefusedUndialed`, `TestARevocationWaitsForItsSystemAndLandsWhenItAnswers`, the escalation case. Left: the dormant re-read's failure carries no why until 8c's read reply does; the `Escalated` SQL filter is live-DB only.
  - *Fix round 1 (21 Nov 2026):* only a system held unreachable now is exempt from age escalation (owner 28.3): `CountUnconfirmedRevocations(ctx, held)` filters `p.target <> ALL($1)`, held from `addons.HeldTargets` (the monitor's `Held`). `revocation.landed` is written by `db.MarkPropagationApplied` in the transaction that settles the row, for exactly the list's rows (`withdrawal`, shared with its predicate): Zitadel revokes and add-on `replace` rows too. The revoke reply's `recorded_at` is the outbox row's, as the list and audit carry (a retry no longer answers the zero time; the allowance read returns `created_at` too). A dispatch Syndra refused before sending is 500 under its own code (`ACTION_NOT_SENT` for a physical action, `CREDENTIAL_NOT_SET` for the password, never 502); an operation's bad params are 400; Syndra's own record or encode failure in `operate` is `addons.ErrOwnFault`, 500 (`LIFECYCLE_NOT_SET`, `SETTING_NOT_SET`), never `NOTHING_SENT`. `addon_outdated` carries no `Retry-After`. Live tests to run: `TestLiveAnAddonRevocationEscalatesUnlessItsSystemIsHeld`, `TestLiveALandedRevocationIsAuditedWithBothTimes`, the allowance retry's `created_at`. Fold with 8c: `Held`/`HeldError`/`HeldTargets` onto `HealthState.Down()`/`heldError`, `whySentence` with `notReadSentence`/`addonSentence`.
  - *Fix round 2 (21 Nov 2026, merged with 8c):* folded onto one path. `addons.Held` is `HealthState.Down` filtered to `addon_unreachable` (`HeldTargets` on it); `Addon.read` refuses through it with `HeldError`, read in `unsent`; 8c's `heldError` is gone. `ReadReply.Why` is the one why; `notRead` adds only the decode fault. One sentence, `whySentence(system, product, why, member)` (`handlers/not_read.go`), replaces `notReadSentence`/`addonSentence`; `nothingSent` wraps it. `TARGET_NOT_READ` and `NOTHING_SENT` share `unavailable` and `writeAnswer` (an outdated add-on now gets no `Retry-After` on a read either). Fixes: a call sent and its outcome not recorded is 202 `unconfirmed` on every dispatching path (`dispatched`: adopt, release, the finding's adopt and release, the dormant purge, password, member action, rotation), never `ADOPTION_FAILED`/`RELEASE_FAILED` 500; a member action refused for its own request is 4xx (`OPERATION_UNAVAILABLE` 409, `CONFIRMATION_REQUIRED` 422, `NOT_SENT_AGAIN` 409, `NOT_AN_ACTION` 400); `RecordSystemConvergence` and `UnfinishedConvergence` return the row's `created_at`, `PropagationRecordedAt` gone; the dormant removal's failed re-check carries its `NotRead` why, and any other failure is 500 `DORMANT_ERROR`; `ResolvesValue` with no manifest is `NotRead{WhyNoManifest}` (503), and Syndra's role map failing to read is `ErrRolesUnread`, 500 `FIELDS_NOT_READ` "Not saved. Syndra couldn't read its record of {system}'s fields. Try again; if it lasts, read Syndra's log." The source-grep escalation test is deleted (the live tests cover it). Tests, each run red first: `TestASentCallWithItsOutcomeUnrecordedIsUnconfirmed`, `TestAnActionRefusedForTheRequestIsTheCallers`, `TestTheRecordedTimeComesWithTheRecord`, `TestADormantRemovalsFailedReCheckCarriesItsWhy`, `TestAMappingWhoseFieldRecordWontReadSaysSoPlainly`, `TestResolvesValueFailsClosedWithoutTheRoleMap`, `TestWhyNoManifestNamesAnOpenBreaker`. Live tests to run: `TestLiveAnAddonRevocationEscalatesUnlessItsSystemIsHeld`, `TestLiveALandedRevocationIsAuditedWithBothTimes`, and the `db` live tests that call `RecordSystemConvergence` (`retired_field_live_test.go`, `merge_bases_live_test.go`).
  - *Fix round 3 (22 Nov 2026):* `dispatched` keeps `unreached` (nothing went out; the row stays `dispatching`, listed unresolved and settled like any other), and the dormant purge, the rotation and the finding adopt are tested through it. A member's unavailable action is one code, `ACTION_UNAVAILABLE`. `whySentence` follows §13.1 row by row (`TestEveryReasonSaysItsSection13Sentence`): `NO_CURRENT_READ` and `TARGET_UNREADABLE` without the add-on's words get "{product} didn't give its add-on an answer it could use" (`Why.Unusable`, not on the wire); a member's `read_refused` or `addon_outdated` is "can't do this right now. Ask makerspace staff." and only an outage says "Try again when {system} answers". A dormant re-check cut at the cap says 8c's truncated sentence.
- [x] 8e The UI reads one state (design §13.6; copy through `plain-language-copy`).
  - Copy: `audit-vocabulary.ts` words `revocation.landed` per `details.target`, naming the system it landed on (PR 253 review).
  - Blocking adoption from a kept copy now depends on the UI reading `read.state` (8c fix round 1).
  - `lib/queries/read.ts`: `Read<T>`, `Why`, `readOf`; every add-on hook returns it. `readInventory` deleted.
  - `ReadFreshness`, `ListStates`, `SectionNav`/`FilterBar` counts and `Card` take a `Read`; `none` draws the sentence and its next step, no count, no empty state, no date.
  - Consumers, each from the survey's §5 table: `TargetOverview` (Accounts chip, inventory card, header read line), `PeopleOnTarget`, the roster rows, `MappingManagement`, `AddMappingDialog`, `HomeLocation`, `PersonActivity`, `DormantAccounts`, `ConvergeEntitlements`, `PersonStorage`/`PersonSystems`, `MyStorage`/`state.ts` (`none` before `needs-password`), Home's unreconciled list (`whyNotChecked` per class).
  - Writes: one toast/inline pair for `NOTHING_SENT` and `unconfirmed`; Unfinished revocations and Withdrawn access show recorded, not yet applied, the age, and that the account still works.
  - Copy guide: a section for the sentences of §13.1 (operator, member), each ending in a next step or "Nothing for you to do"; guard test.
  - Guards, mutation-checked: an add-on hook not returning `Read` fails; a fixture of each no-copy reply rendered on every consumer shows no "0", no empty state, no date (`getByText` over the three).
  - "Try again" on the health page says what it is doing: a waiting state while it reads, the 429 cooldown, and a failure of Syndra's own told apart from the add-on's (from 8b fix round 3).
  - A deferred row sent again is audited as try 1 each time it is re-deferred; say which try it is (from 8b fix round 3).
  - Layout check: these pages make no new read, so the recorded tape replays; say so in the PR.
  - *Built (22 Nov 2026):* `lib/queries/read.ts` holds `Read<T>`, `Why`, `readOf` (the one builder; a reading with no moment, Go's zero time included, is `none`), `notReadOf` (a 503 `TARGET_NOT_READ`), `readFrom` (the one fetch: every add-on hook's `queryFn`, a none is data, any other failure Syndra's own error) and `ofRead` for counts. Composite replies are split at the hook: `useTargetHealth` is the monitor and log anchor beside `addon: Read`; each `/me/targets` and `/users/{id}/targets` view carries `read: Read<{storage, standing, reads}>` and each member read its own `Read`. `readInventory`, `observed`, `stamped`, `pageLoad`, `useReadAgainSince`, `last_known` and every `readable`/`halted`/`read_at` branch are gone. `ReadFreshness` takes `read` (none is `NotReadLine`: the sentence and its step, Read again where offered, no date); `ListStates` takes `read` (pending is the skeleton, none draws nothing); counts come through `ofRead`. §13.1's sentences are `whyLine` (`components/results/copy.ts`), copy guide section 19, held to the table at both ends (`whyCopyGuide.test.ts`, `TestTheCopyGuideSaysEverySection13Sentence`); a member reads "{system} can't be read right now. Nothing for you to do." for both outages. Writes: `nothingSentOutcome` (also a 503 `TARGET_NOT_READ` refusing a save), `unconfirmedOutcome`, `isNothingSent`/`notDoneWord`; the revoke reply says Recorded with the backend's sentence and the credential half's own step; Withdrawn access says recorded, not yet applied, and that the account still works. `revocation.landed` is worded by `details.target`. The member state asks none before the password step; the roster, header and member views read the four monitor states. Folded in: one status region per `ReadAgainProvider`; the press scale and the cursor honour `aria-disabled`; `keepFocus` stops a form submit; the focus flag clears on a none; one wait wording (`cooldownLine`); Try again on the health page says Reading, a 429's wait, and Syndra's own failure apart from the add-on's; a re-deferred row is audited `not_sent` with `tries_spent` and no try number (`TestDrainRevocations_AReDeferredRowSaysNoTryWasSpent`). Guards (each run red once): `useTargetInventory.test.tsx` (every add-on hook answers a `Read`), `NotReadSystemPage`, `NotReadMappingsAndLists`, `NotReadMember`, `whyCopyGuide`, `audit-landed`, `RevokeDialog`, `WithdrawnAccess`. Layout: the tape is staging's pre-8b reads, so it replays the pages in their not-read state only; the gate for the new reads is the staging run after deploy (8g).
  - *Fix round 1 (Opus review of PR 256).* A re-deferred retry (`propagation.retried`, outcome `not_sent`) is worded by its outcome: nothing went out, Syndra will try again. Health "Try again" is said by its Notice's live region alone, and a failure there no longer repeats the Notice. A member's kept storage copy is dated and recedes. A dormant "Read again" told only to wait keeps the rows read moments ago and counts down; a real no-copy answer still takes them. A replaced credential says "Nothing for you to do." once. Withdrawn access carries §13.1's sentence while the monitor holds the system down (`downWhy`, copy guide row). The reconcile pass's `read` is built once in `useReconcileTarget` (`withRead`). A kept roster count says when it was read. The backend dates the activity log's "no account here" (`Activity`, 422 answered now, dated when Syndra asked), so the UI never dates by the browser clock. One `notRead` helper replaces the `state === "none"` checks; a counting Read again clears the page's region when it goes.
- [x] 8f The kit takes things down (kit + reference add-on; design §13.7).
  - `-stop-app`, `-start-app`, `-stop-addon`, `-start-addon` (a command each; a missing hook reports "not checked"; `-require-outage` makes that a failure, and CI passes it).
  - Reference add-on: `REFERENCE_APP_DOWN_FILE` switches `Config.Down` at run time (test binary only).
  - Checks `outage.health`, `outage.kept`, `outage.none`, `outage.write`, `outage.restart`; `freshness.unreachable` is no longer skipped.
  - `noFalseZero` walker on every reply of a run. Mutation-check against the reference: an empty `subjects: []` in a 503, a zero `taken_at`, `current: false` without `stale`, each red.
  - *Built (21 Nov 2026):* as listed, in `addons/conformance/outage.go`. The hooks are `Config.StopApp` and the rest, `func(context.Context) error`, which the binary builds from each command with `sh -c`; the self-tests pass closures, so they stay in-process. `-require-outage` requires the app pair only: the reference keeps no disk, so CI cannot pass `outage.restart`, and it reports "not checked" there; its self-test restarts an in-process reference keeping its state (pass) and a new one (fail). The walker's check is `no-false-zero`; "nothing to serve" is the codes `TARGET_UNREACHABLE`, `NO_CURRENT_READ`, `TARGET_UNREADABLE`, `FRESH_READ_COOLDOWN`, `STORE_UNREADABLE`. `freshness.unreachable` is judged on the fresh read during the outage, and fails a 503 when the read just before had a value. The reference: `down()` is the one switch (`/subjects` without freshness, `/plan` and `/apply` answer 503 `TARGET_UNREACHABLE`, the apply unrecorded; `/health` `reachable: !down`, and `last_read_at` from its last good read); `/values` and the member operations still answer (a `ponytail:` note). In `-mutate` the outage runs after the calls step, the apply sent by `toOwned` right after its plan, the app stopped in between. Self-tests: one bad reply per check (`TestEachOutageCheckFailsOnABadReply`, 9 cases), mutation-checked by disabling each assertion (8 red) and the reference's apply check (red on the clean run). CI's end-to-end runs pass `-require-outage` with `touch`/`rm` hooks; run locally, all six exit 0.
  - *Fix round 1 (Opus review of PR 251).* Once a stop is tried its start always runs: the app's in a `defer` (`appBack`, with `context.WithoutCancel`), the add-on's straight after, failed, hung or interrupted (the binary's first SIGINT already cancels the run's context and lets it put things back). Each hook gets 2 minutes (no other kit flag takes a timeout, so a constant, not a flag), runs in its own process group killed whole (so a terminal's Ctrl-C reaches the kit, not the start hook), and says "kit: running -stop-app …" on stderr. A run that cannot make a fresh read (no readable `target_reads`, a cooldown longer than the kit waits) skips `freshness.unreachable` and the fresh-read parts only; the other outage checks run on plain reads, or fail under `-require-outage`. A plan naming someone else's account halts before the stop. The reference's `reachable` and `last_read_at` come from one helper, `reach`, on every read (`/subjects`, the fresh read, `/plan`, the stats read). AUTHORING: both hooks must be safe to run twice. Tests, each red on the unfixed code: `TestAFailedStopStillStarts` (failed and hung stops), `TestNoFreshReadSkipsOnlyFreshnessUnreachable`, `TestAHaltBeforeTheStopLeavesTheAppUp`, `TestReachableIsTheLastReadsResult`, `TestAHangingHookTimesOut`; `TestARestartThatRedatesTheCopyFails` is red with the restart's date check disabled.
  - Left for 8c: the backend table test (every route of §13.3 and §13.4 against a stub add-on: closed port, blackhole, wrong key, hang, uncoded 500, each coded 503), which the kit cannot be. Left for 8g: the TrueNAS add-on run with the app hooks against a stub NAS.
- [ ] 8g TrueNAS add-on conforms, then staging. `ready-for-human` (staging deploy needs the owner's OK: same real NAS; the add-on there is `read_only`)
  - [x] `addons/truenas/reach.go` (the rename is done by 8a): a not-sent failure is labelled `NO_CURRENT_READ` instead of `TARGET_UNREACHABLE`, because `handleReach` wraps every failure in `errNeverRead` (`:209`); a failed groups read with no copy answers `TARGET_UNREADABLE` (`:531`); kit green with the app hooks against a stub NAS.
    - *Built (22 Nov 2026):* one answer for a read with nothing kept, `writeNotRead` (`fresh.go`): 503 `TARGET_UNREACHABLE` when nothing was sent, `NO_CURRENT_READ` otherwise; `/subjects`, `/reach`, `/homes` and the read operations answer through it (`errNeverRead` gone; a read operation's dead socket no longer reads "account could not be found"). `/values` keeps `TARGET_UNREADABLE` (the guide). A write whose lookup died on its socket sent nothing (`readBeforeWrite`, `nas.go`): the apply and the write operations answer `TARGET_UNREACHABLE`, not `APPLY_FAILED`/`OPERATION_FAILED`, which is what the kit's `outage.write` found. The stub NAS is the fake behind the add-on's dialer seam, off while a file exists (`stubNAS`, `conformance_test.go`); every kit run there gives the app hooks and `-require-outage`, the start hook waiting out the 15 s reconnect floor, and CI runs them in addon-conformance beside the reference. Guards, each run red once: `TestEveryReadWithNothingToServeAnswersOneOfTwoCodes`, `TestAWriteWhoseLookupDiedSentNothing`, the kit's `outage.write`.
  - [x] Fixed (22 Nov 2026), found on the staging run: pointed at an unroutable NAS, the monitor went `addon_unreachable timeout`, not `app_unreachable`. The add-on's `/health` pinged the NAS unbounded (the client's 45 s handshake, 10 s ping), past the backend's 5 s probe, and the dial held `NAS.mu`, so the next `/health` waited behind it too. Now every NAS call `/health` makes ends at `healthReachWithin` (2 s, `server.go`) and answers `reachable: false`; `session()` never holds the lock across the wire (`offLock`, `nas.go`). The kit's `outage.hang` (`-hang-app`) asks `/health` within 3 s, twice, with the app hanging; the reference hangs while its down file holds `hang`, the stub NAS likewise, and CI gives both the hook. Guards, each run red once: `TestHealthAnswersInTimeWhenTheNASHangs`, `TestOutageHangFailsAHealthThatWaitsOnTheApp`, `outage.hang` against the unfixed add-on, `TestAHealthSlowedByTheAppIsTheAppWithinTheBound` (backend, by shrinking `probeTimeout`).
    - *Fix round 1 (22 Nov 2026):* the session's calls go one at a time, so `/health`'s ping can queue behind an `audit.query` page or a `get_quota` while the NAS is busy and fine; when the 2 s run out, an answer within `recentReadCounts` (10 s) counts as reachable, and a NAS that hangs still reads false once its last answer is older. That is now the contract (§13.2, the guide, the changelog): `reachable` is true when a call to the app succeeded now or, when it gets no answer in time, within the last 10 s; a refusal is false at once. The kit's `outage.hang` waits `hangSettle` (12 s) after hanging the app before it asks; the reference counts its last read the same way. `session()` re-checks that a proven client was not dropped meanwhile. The changelog says `-require-outage` now requires `-hang-app`. Backend (task #89): from `checking` an outage takes the same three misses as from `answering`, so a restart whose first probe finds the add-on not yet listening stays `checking`, and one answer still decides; the drain still halts `checking` as `addon_unreachable`. Guards, each run red once: `TestHealthCountsARecentAnswerWhileTheSessionIsBusy`, `TestAProvenSessionDroppedMeanwhileIsNotReturned`, the kit's "reachable on a recent answer" case (red without the wait), `TestFromCheckingOneAnswerDecidesAndThreeMissesMakeAnOutage`; `TestHealthAnswersInTimeWhenTheNASHangs` fails on its own deadline, not the test timeout.
  - [x] Fixed (22 Nov 2026), found on the staging rerun: with the NAS blackholed, `health.get` (the system health route) answered `addon_unreachable`. Its first NAS read dialled with no bound of the add-on's own (the client sets none for the connect and 45 s for the handshake), and a call on a live session waited 30 s, the backend's whole `callTimeout`. Now a session is had within `nasDialTimeout` (8 s, `dialBounded`, `nas.go`; the purge's own dial too), a call ends at `callTimeoutSeconds` (15 s), and a call that waited out its timeout arms the 15 s reconnect floor (`lost`), so the next calls answer `ErrNotSent` at once and a read of four calls waits once. `/health`'s ping goes through the same door at 5 s. `health.get` with every source unanswered answers as `/subjects` does (`writeNotRead`: `TARGET_UNREACHABLE` when nothing was sent, `NO_CURRENT_READ` otherwise), not 503 `OPERATION_FAILED`. The kit's `outage.hang` now also asks each declared read operation within 25 s (`readWithin`, under the backend's 30 s) for its kept copy or a no-copy code; the reference's `health.get` and `holdings.list` refuse while the app is down or hangs, and the stub NAS's calls wait out their timeout as the client's do. Backend unchanged: its per-call timeout is 30 s (`callTimeout`, `deps.go`), one attempt for a POST. Guards, each run red once: `TestHealthGetAnswersInTimeWhenTheNASHangs` (red without the dial bound, without the floor, without the code), `TestACallAfterAFailedDialFailsFast`, `TestNASBoundsFitUnderTheBackendsCallTimeout`, the kit's "a read that waits on the app" case, `outage.hang` against the TrueNAS add-on answering `OPERATION_FAILED`.
    - *Fix round 1 (22 Nov 2026):* a busy NAS is not an outage; only silence is. A call that runs out its time is followed by a `core.ping` on the same session: an answer keeps the session and arms nothing (`errBusy`), none arms the floor (`lost`, which now drops and stamps in one critical section, only while that session is still the one, and closes it). One deadline per request from its start (`requestWithin`, 24 s, `server.withBudget`), carried by the context through the wait for the session, the dial, the idle proof, the wait in line for the wire (`NAS.line`, a channel now) and each call, so their sum cannot outlast it. The collector's calls get their own 60 s (`collectCallTimeout`, `withCallLimit` on its context). `rpc.Ping` dropped; the seam guard lets `Ping` and `ping` send only the literal `core.ping`. Test speed: the kit's `readWithin` and the add-on's ping bound and reconnect floor are shortenable in tests. Guards, each run red once: `TestABusyNASIsNotLost`, `TestASilentNASIsLost`, `TestTheCollectorsCallsGetTheirOwnLimit`, `TestAWholeRequestStaysInsideItsBudget`, `TestNASBoundsFitUnderTheBackendsCallTimeout`, `TestLostForgetsOnlyTheSilentSession`, `TestAfterASilentWriteTheNextIsNotSent`, the seam guard's literal `core.ping`.
    - *Fix round 2 (22 Nov 2026):* a busy NAS or a busy add-on queue is never an outage; only silence is. The collector pages `audit.query` to `statsCallTarget` (5 s a call, at an assumed `statsAuditRowsPerSecond` of 40: 200 rows a page, still 50 000 a run; the rate is UNVERIFIED, measure one page on staging read-only), so `/health`'s ping waits one short call and each page keeps `LastRead` recent; `collectCallTimeout` (60 s) is the ceiling only. New wire code `TARGET_BUSY` (503, `Retry-After` 5 s, `writeBusy`, `fresh.go`): a read or a write that sent nothing, when it gave up in line (`lineUp`, the session wait), had less than the whole dial or ping bound left (not tried: no breaker count, no floor, the session kept), or a read that ran out on a NAS that answers `core.ping` (`ErrBusy`, `nas.go`); a kept copy then says `stale.reason: busy` (`staleSince`). Only a dial that really failed arms the floor. `seconds()` honours an ended context. Backend: `Unread` maps `TARGET_BUSY` to `busy` (not `Unreachable()`), the breaker skips it, `Halt` says `read_stale`, `ReadOf` keeps `busy`, and `whySentence`/`whyLine` say "{product} is busy right now. Try again in {n} s." (copy guide §19, three rows). Registry, `stale.json`, guide, changelog, §13.1/§13.2 updated; the kit takes `TARGET_BUSY` as a no-copy code. Guards, each run red once: `TestHealthStaysReachableThroughALongCollectorRun`, `TestABusyNASAnswersTargetBusy`, `TestACopyServedWhileBusySaysBusy`, `TestADialWithTooLittleTimeLeftIsNotTried`, `TestAPingWithTooLittleTimeLeftIsNotAsked`, `TestAnEndedContextSendsNothing`, `TestADialEndsAtTheRequestsDeadline`, `TestTheWaitForTheSessionEndsWithTheContext`, the kit's "a read with no copy answered busy", `TestTargetBusyNeverOpensTheBreaker`, `TestUnreadNamesWhoIsDownAndWhy`, `TestReadOf`, `TestHaltSaysWhatTheReplySaysAheadOfTheCooldown`, `TestTheCopyGuideSaysEverySection13Sentence`.
    - *Fix round 3 (22 Nov 2026):* a busy apply is deferred (`deferUnsent`, no attempt spent) and its `last_error` says "{system} was busy; nothing was sent. Syndra sends it again by itself." (`unreachedWhy`). The drain keeps, in memory, when each target first answered busy, cleared by any other answer; busy without a break past `busyHaltAfter` (30 min) defers that row and halts the pass as `busy` with `busy_since`, which the send outcome ("{system} has been busy since {time}; nothing was sent.") and the target page (`/targets` `busy_since`, `propagation.BusyHalted`) say. `/health` never answers `TARGET_BUSY` (registry, `unread.go`, §13.1, guide, changelog; the kit's `outage.hang` checks it); `outage.none` refuses `TARGET_BUSY` with the app stopped; `stale.json` lists it as a no-copy code. Guards, each run red once: `TestABusyApplySpendsNoAttemptAndIsDeferred`, `TestABusyTargetHaltsAfterHalfAnHourAndClearsOnAnotherAnswer`, the UI's "a system busy past the limit", the kit's "a health that answered busy" and "a stopped app with no copy answered busy", `TestAWriteSentOnABusyNASIsNeverTargetBusy`.
  - [ ] Staging, by hand: stop the add-on container and expect `no_answer` on the roster inside 90 s (not `wrong_host`: this is the proof of the absolute-name rule, which is inferred from Docker's resolver), no page with a 0, an empty state or a date; start it and expect `answering` on the next probe. Then point the add-on at a dead NAS address and expect `app_unreachable` with kept copies dated.
  - [ ] Layout check against staging in both outage states.
- [ ] 8h (owner's, after 8g) Open questions of the 21 Nov grill round, recorded here with their answers; any "no" reopens the slice it names. `ready-for-human`
