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
  - *Done (13 Nov 2026, slice 7g):* the end-to-end step also starts a second
    reference with every feature (`-features results,freshness,calls,stats`)
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
- [ ] 4.6 `--mutate` run against staging TrueNAS with a throwaway subject,
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
      - A gap whose reason names a person is kept with the fixed reason "The add-on's reason named a person." and counted `person_in_dimension`; one whose dimensions name a person is dropped, as its series is (`TestNoValueOrReasonNamesABoundPerson`).
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
  - Insights: fewer than 5 people shows the count and no spread or bar (MIN_SPREAD_MEMBERS; the backend has no floor for this read, insights.go's 5 is the single-route holder count).
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
      mislead. Only collisions are asked: usually no extra call. A plan
      that fails proves nothing, so the point stays a local name. A
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
