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

- [ ] 1a.1 Manifest v2 schema and fixture: `contract_version: 2`, `kind` from
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
- [ ] 1a.2 Every other major-2 schema and fixture: the per-message reply
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

- [ ] 1.0 (found by slice 0) The backend's `Homes.Read` and `Reach.Read`
  (`addons/read.go`) are its own "the read succeeded" flags but share the
  wire's JSON key `read`, so a tolerant decode would let an add-on claim a
  read. Split the wire type (decoded from the add-on) from the view type
  (sent to the UI) for every read reply, so a backend-only field can never
  be set by an add-on; the v2 schemas then stay the only description of
  what the add-on sends #ready-for-agent

- [ ] 1.0a (found by slice 0 review) The add-on emits `[]`, not `null`, for an
  empty list field (wire change): `entitlementState` (`apply.go`) builds the
  group list with `append([]string(nil), ...)`, so an account with no groups
  is `"group": null` in `/subjects` `state` and in apply `observed`. Contract 1
  pins that (the reply schemas allow null for list fields; `desired` stays
  arrays only). `services/merge` canonical must also treat a null list as `[]`
  (today `sameValue(null, [])` is false; harmless now because the add-on's plan
  compares groups as sets) #ready-for-agent

- [ ] 1.2 TrueNAS add-on declares it (`capabilities.go`): `kind: storage`,
  features `operations, settings, storage.reach, storage.homes`, the five
  fields by role as in design §3; `ContractVersion = 2` #ready-for-agent
- [ ] 1.3 Backend manifest decode becomes tolerant **everywhere**, inside
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
- [ ] 1.4 Every write carries `contract_version` and `call_id` in the body:
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
- [ ] 1.4a The raw query joins the MAC: `ts.method.target.body` with
  `target` = escaped path as sent + `"?" + RawQuery` when non-empty, on
  both ends (`backend/internal/addons/transport.go:441-450`,
  `addons/truenas/transport.go:173`), the path signed **as sent, escaped**
  (both ends sign the decoded `URL.Path` today, `transport.go:317`, add-on
  `:104`); `transport_derivation.json` gains four vectors (no query, whose
  MAC equals major 1's; a base64url `cursor`; a value with a reserved
  character; an escaped path); tests: each vector reproduces, a reordered
  or altered query → 401. Decision 24 #ready-for-agent
- [ ] 1.5 Error envelope: every add-on refusal is `application/problem+json`
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
- [ ] 1.6 `standing` on every subject (`common/standing.json`) and on the
  apply outcome; TrueNAS: no password → `member` with the existing
  `smbWaiting` sentence as `note` and the storage page's next step as `next`;
  `password_set` removed from the wire #ready-for-agent
- [ ] 1.7 `account_id` (string) replaces `uid` on `subjects[]`, the apply
  reply and the binding conflict (decision 26); migration adds
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
- [ ] 1.9 Code for every message that moved in 1a: `PLAN_TOO_LARGE` (422)
  replaces `SCOPE_ACKNOWLEDGEMENT_REQUIRED` in the add-on (the backend
  chunks under `limits.subjects_per_plan` and owns the acknowledgement);
  `reach.read_at` → `taken_at`, `homes` gains `taken_at` + `current` and
  keeps `quota_read` as its completeness flag, on both ends; when the
  TrueNAS add-on answers a read from its mirror because the live read
  failed (`server.go:321-349`, its only mirror path today) the reason is
  `stale.reason: unreachable`, not `cached`, which is reserved for a
  deliberate mirror answer under `freshness` (6.2); the drift sweep already
  treats `current: false` as a skipped pass #ready-for-agent
- [ ] 1.10 Deploy. **The order is written into `deploy.yml`'s production
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
  window. Rollback = both previous tags through the same workflow; **no
  store restore** (the store is additive, 1.7; the dated copy is a fallback
  only if a v1 binary cannot open the file); the re-runnable backfill
  (1.7) covers rows v1 writes meanwhile #ready-for-human

## 2. The core forgets the names (backend only)

- [ ] 2.1 `services/entitlement.go`: `LifecycleState{Account, Channels map[string]bool, NotAMember, Reason}`;
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
- [ ] 2.2 Migration: the `000087` CHECK on `desired_state_snapshots.withdraws_only`
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
- [ ] 2.3 Mapping validation by role (`mappings.go`): `account`/`channel`
  not bindable; `limit` parses as a non-negative integer; `flag` accepts only
  `"true"`; `mapping_plan.go:174` wording from `label` #ready-for-agent
- [ ] 2.4 `handlers/target_homes.go`, `my_storage.go`, `person_storage.go`,
  `services/role_members.go:306`, `drift/addon.go:723`: read the field by
  role (`limit` with `unit: bytes`, the `flag`, the `membership`); the
  storage kind's view (`handlers/kinds/storage.go`) owns `smb://`/UNC from
  `connection.protocol`; `read.go:755-781` deleted #ready-for-agent
- [ ] 2.5 Two guard tests. (a) `backend/internal/addons/no_target_words_test.go`:
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

## 3. The UI reads roles (UI only)

- [ ] 3.1 `ui/src/lib/fields.ts`: `fieldsByRole(schema)`, `accountField`,
  `limitFields`, `formatLimit(value, unit)`; the manifest's schema reaches
  the UI through the existing targets endpoint unchanged #ready-for-agent
- [ ] 3.2 `lib/role-mapping-plan.ts`, `targets/HomeLimit.tsx`,
  `AddMappingDialog.tsx`, `MappingManagement.tsx`, `PeopleOnTarget.tsx`,
  `ui/Withheld.tsx`, `review/HoldsDueForReview.tsx`, `people/PersonStorage.tsx`,
  `queries/useMyStorage.ts`: by role, copy from `label`/`unit`; the
  `enabled`/`smb_enabled` sentences in `AddMappingDialog.tsx:403-416` become
  "the account" and "its {channel label}" #ready-for-agent
- [ ] 3.3 `storage/MyStorage.tsx` becomes the storage kind's module
  (`components/kinds/storage/`), chosen by `kind`; a kind with no module
  (`access`, `inventory`, `machine` today) renders the core member card
  (account standing + `next`) #ready-for-agent
- [ ] 3.4 Lint guard: the five field literals and `smb` are allowed only
  under `components/kinds/storage/` and tests #ready-for-agent
- [ ] 3.5 Copy guard run (`plain-language-copy`): every new sentence passes;
  `standing.next` always ends in a next step or "nothing for you to do" #ready-for-agent

## 4. The conformance kit and the reference add-on

Decisions 5 and 8 accepted 10 Nov 2026. **Core only, built before slice 1
deploys** (design §12): the kit is written against the slice-0 and slice-1a
schemas and run against the v2 TrueNAS add-on on staging as the gate for
1.10. The feature halves (reference and checks) are slice 6 (6.0, 6.11).

- [ ] 4.1 `addons/reference`: an in-memory target with accounts, one field of
  each role, one member operation (`password.set`), lifecycle, settings;
  ~400 lines; the derived-key transport (with the raw query in the MAC)
  from `addons/truenas/transport.go` copied, not imported #ready-for-agent
- [ ] 4.2 `addons/conformance`: the binary in design §7, every check in the
  table, `pass|fail|skip` with the MUST it enforces, JSON and text report,
  non-zero on a MUST failure, `--mutate` gated, reads only without it #ready-for-agent
- [ ] 4.3 CI: the kit runs against the reference add-on on every push
  (`ci.yml` job `addon-conformance`); the job is required for `ci-ok` #ready-for-agent
- [ ] 4.4 The kit's own tests: each check is watched to fail against a
  deliberately broken reference (a flag per defect: ignores fingerprint,
  creates on withdraw-only, echoes the secret, answers a replay after the
  fingerprint check, remembers a refusal, omits `truncated`, says
  `current: true` from its mirror, accepts an altered query, treats
  max(`"0"`, 5 GiB) as 5 GiB, lets a withdraw-only apply touch a membership) #ready-for-agent
- [ ] 4.5 Reads-only run against production TrueNAS from the app host;
  report attached to this change #ready-for-human
- [ ] 4.6 `--mutate` run against staging TrueNAS with a throwaway subject,
  against the **v2** add-on before 1.10; report attached; the kit's last
  line names the one disabled, labelled account it leaves (removal disables,
  never deletes) #ready-for-human

## 5. The guide

- [ ] 5.1 `addons/AUTHORING.md` to the outline in design §8, every rule
  BCP 14 (RFC 2119 + RFC 8174), every core route with its fixture inline, the reference add-on
  walked file by file; §1 opens with the boundary rule (the add-on alone
  holds the app's address and credential and alone talks to it; Syndra
  talks only to add-ons) and §4 carries the freshness contract and the
  `stale`-not-error rule #ready-for-agent
- [ ] 5.2 Mapping sketches for UniFi Access, Ruby and the inventory system as
  the guide's §12, from the two research notes and design §10; each names
  its unverified items as such (the inventory one names its API as unknown
  and lists the four questions in design §10) #ready-for-agent
- [ ] 5.2a The guide's statistics chapter: `metrics[]`, the collector is the
  add-on's (schedule, backoff, the app's rate limits), the buffer and its
  bounds, a point is the whole UTC day from the app's log (not a delta,
  never an explicit zero), the per-metric watermark and when not to move it,
  a gap is never a zero, no local name for an unbound account, the total
  beside every per-subject point, `against`; and the
  operations chapter's `result`, `effect`, and the six `physical` rules
  with the inventory checkout as the worked case #ready-for-agent
- [ ] 5.3 A fresh agent with only `AUTHORING.md`, `addons/contract/` and the
  kit builds an add-on for a toy target and passes the kit; what it had to
  ask is written back into the guide #ready-for-agent
- [ ] 5.4 `openspec/NEXT.md` and `architecture-deepening/tasks.md` item 6
  point here; `addon-platform/design.md` §5 and §16 gain a one-line pointer
  to this contract as the current statement #ready-for-agent

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
