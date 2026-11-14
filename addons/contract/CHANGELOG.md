# Contract changelog

One entry per major or feature: what it is, the schema files it touches, and the
fixture that changed. This file lives above both majors because it spans them.

## `result` is core; the per-read keys and feature `results` go (slice 7h, 14 Nov 2026)

A change to major 2 before its release, not a new major: production runs
contract 1, and no contract-2 add-on has shipped. Owner ruling (14 Nov
2026): TrueNAS is the only add-on in production and it ships with the
backend, so nothing needs a contract-2 add-on without `results`. Every
`effect: read` operation now answers one `result`, valid against the schema
`registry/operations.json` names for it, and under no other key; Syndra
reads `result` alone (`answer` in `backend/internal/addons/results.go`), and
a reply without it is no reading (indeterminate), as a result missing a
required key already was. A physical operation needs `calls` alone.

- Schema files: `operation_response.json` loses `activity`, `storage` and
  `health` and their `$defs` (`ActivityReport`, `StorageStatus`,
  `TargetHealth`); `result` loses `x-feature: results`.
  `profiles/storage/status.json`'s description drops "feature results".
- Registries: `operations.json` loses `pocket` (from `activity.get`,
  `health.get`, `storage.status`) and its `$comment` the rules about it;
  `features.json` loses `results`. A manifest still listing `results` is
  an unknown feature: skipped, never refused.
- Fixtures: `operation_response_activity.json`,
  `operation_response_storage.json` and `operation_response_health.json`
  are deleted; `manifest.json`, `manifest_truenas.json` and
  `manifest_inventory.json` no longer list `results`.
- The kit's `results` check is the core check `operations.result`: every
  read answers a `result`, whatever the manifest declares. The guide's
  section is "Read operations and `result`".
- Deploy and rollback: unchanged. The backend and the TrueNAS add-on ship
  and roll back together; the rollback target is production's contract-1
  pair, which never read or wrote a contract-2 read reply.

## Feature `stats` served by the add-ons, in makerspace days (slice 6d-1, 12 Nov 2026)

Additive within major 2, but for two tightenings of the schema: `against`
is allowed only on a `per_subject` gauge with no dimensions, and
`stats_zone` is required with the feature. Both are harmless, because no
add-on served `stats` before. The TrueNAS add-on and the reference declare
`stats` and answer `GET /stats`; Syndra does not collect it yet (6d-2): a
backend before then ignores `metrics` and `stats_zone` and never calls
`/stats`, so the add-on may deploy first. Every bucket is a calendar day in
the manifest's `stats_zone` (owner ruling O1), not a UTC day.

- Schema files: `manifest.json` gains `stats_zone` (an IANA zone,
  `x-feature: stats`, required with the feature) and allows `against` only
  on a `per_subject` gauge with no dimensions (ruling O3); `stats.json`'s
  descriptions say a bucket starts at a midnight in `stats_zone` and ends at
  the next, a day's midnight being its first instant (where a DST change
  skips 00:00, the instant the clocks resume).
- Registries: `features.json` lists `stats_zone` under `stats`.
- Fixtures: `stats.json` in `+05:30` midnights, points in keyset order,
  `next_cursor` the last key; new `stats_storage.json` (TrueNAS
  `storage.sessions`); `manifest.json` splits the storage gauge into
  `storage.home_bytes` (against `home_quota`) and `storage.used_bytes` (per
  share) and carries `stats_zone`; `manifest_truenas.json` declares `stats`,
  three metrics, `stats_zone` and `limits.stats_buffer: P90D`.
- The guide's statistics section moved from the appendix into chapter 4 as
  "Statistics: `GET /stats` (feature `stats`)"; the mapping sketches are
  A.2. Its totals rule is per instrument (ruling O2): a counter's people sum
  to no more than the total, each of a gauge's is no more than it.
- Deploy and rollback: the TrueNAS add-on's buffer is a new bbolt bucket a
  previous binary never opens; rolling the add-on back loses nothing but
  the collection it would have made. Set `STATS_COLLECT_EVERY=0` for a kit
  run.

## Feature `calls` served, `effect: physical`, outcome `no_change` (slice 6c, 12 Nov 2026)

Additive within major 2. Syndra now offers a `physical` operation only from
an add-on declaring `calls` (and `results`, for its record), previews it
(`preview: true`, no `call_id`, nothing recorded), dispatches the confirm
once with a fresh `call_id`, never resends it, refuses the next while one is
unsettled (`CALL_UNSETTLED`), and settles a lost answer, any write's, through
`GET /calls/{call_id}`; a physical call in doubt past
`constants.in_doubt_bound` becomes Waiting on you. Syndra reads a 2xx
operation reply's own `outcome`: `succeeded`, `no_change` (from a `calls`
add-on only, recorded as `no_change`), and anything else or nothing as
possibly acted. The TrueNAS add-on declares `calls` (it has no physical
operation; it answers `GET /calls` and refuses a preview).

- Schema files: `common/outcome.json` (`oneOf`, `no_change` under
  `x-feature: calls`, listed in `features.json` as `outcome.no_change`),
  `error.json` (extension members `param`, `unit`, `count`; which codes
  require them), `operation_response.json` (what a preview answers).
- Registries: `operations.json` gives every id `secret_result` (all false)
  and registers `item.checkout` (member, confirmed, `item_id` with
  `format: target_id`, `effect: physical`, result `common/records.json`);
  `errors.json` registers `UNKNOWN_RESOURCE` (404), `NOT_ENTITLED` (403),
  `LIMIT_REACHED` (409) and `RESOURCE_TAKEN` (409), each `retry: never`.
- Fixtures: new `operation_request_preview.json`,
  `operation_response_no_change.json`, `call_in_doubt.json`,
  `call_unknown.json`, `error_call_unsettled.json`,
  `error_unknown_resource.json`, `error_not_entitled.json`,
  `error_limit_reached.json`, `error_resource_taken.json`;
  `manifest_inventory.json` declares `calls` and `item.checkout`;
  `manifest_truenas.json` declares `calls`.
- Decided while building: "held by someone else" is `RESOURCE_TAKEN` (409,
  `param`; RFC 9110 §15.5.10), a refusal, never an outcome (O1);
  `item.return` waits for the inventory add-on (O2); `GET /calls` keeps the
  shipped `{call_id, state, reply}` (B1); a preview answers the outcome the
  confirm would (B2).
- The guide's appendix A.1 moved into chapter 7 as "Physical operations and
  `GET /calls/{call_id}`"; A.2-A.4 are A.1-A.3.
- Deploy and rollback: the backend and both add-ons ship together. An add-on
  declaring `calls` beside a backend before 6c has its `no_change` read as
  succeeded (that backend reads status alone), and its `GET /calls` unused;
  migration 000092 only widens the record's status CHECK, and its down folds
  `no_change` into `succeeded`, which is what that backend called it.

## Feature `freshness` served (slice 6b, 11 Nov 2026)

Additive within major 2; no schema shape changed. Syndra now sends `fresh`
(`?fresh=true` on `GET /subjects`, `/values/{field}`, `/reach`, `/homes`;
`"fresh": true` in an `effect: read` operation's body) to an add-on
declaring `freshness`, and only there: the drift sweep's `/subjects` read
always, the dormant accounts list and its removal always, and a person's
"Read again". Never on `/plan`, `/apply` or `/stats` (the backend refuses
it before anything leaves). A `FRESH_READ_COOLDOWN` is never a breaker
failure. The TrueNAS add-on declares `freshness`.

- Decided while building: a fresh read joining one already under way is
  answered `cooldown` (B1); `health.target_reads` never decreases while the
  add-on runs and may start again at 0 (B2, `v2/schema/health.json`'s
  description); with nothing to serve, `TARGET_UNREACHABLE` when no request
  reached the app and `NO_CURRENT_READ` when it was asked and failed (B3, the
  guide's "Nothing to serve" rule).
- The guide's appendix A.1 moved into chapter 4 as "Fresh reads"; A.2-A.5
  are A.1-A.4.
- Fixtures: new `subjects_cooldown.json` (a cooldown answer),
  `error_fresh_read_cooldown.json` (429), `error_target_unreachable.json`
  (503), `operation_request_fresh.json` (a read asking for a fresh one, as
  the backend sends it); `manifest_truenas.json` declares `freshness` with
  `limits.fresh_read_cooldown: PT30S`.
- Deploy and rollback: the backend and the TrueNAS add-on ship and roll back
  together. A TrueNAS add-on declaring `freshness` beside a backend that
  never sends `fresh` answers every `/subjects` from its copy, and the drift
  sweep then skips every pass (safe, and silent).

## Feature `results` served (slice 6a, 11 Nov 2026)

Additive within major 2. Syndra now reads `result` from an add-on declaring
`results`, through one read path for every `effect: read` operation (never
dispatched, logged or confirmed), and still reads the typed pockets from one
that does not, until major 3. The TrueNAS add-on declares `results`.
(Superseded before release by slice 7h, above: `result` is core and the
pockets are gone.)

- `holdings.list` added (`v2/registry/operations.json`): scope `member`,
  `effect: read`, no parameters, result `common/records.json`.
- `storage.status`'s result is the storage profile,
  `v2/schema/profiles/storage/status.json` (new): `common/status.json` plus
  `places`, `places_readable`, `needs_password`, `needs_new_password` and
  `quota_read` (the pocket's `usage_readable`, named as in `homes.json`;
  false on a partial read too).
  Every valid reply is a valid common status. `username` and `smb_enabled`
  are not in it: Syndra has the username from its binding. An amendment
  before release: no add-on declared `results` until this entry.
- Activity events gain an optional `resource` (`v2/schema/common/activity.json`),
  the place on the target an event happened, in the vocabulary of
  `uncovered`: the pocket's `share`, without the product word.
- Registry `pocket` (`storage`, `activity`, `health`): the key each read was
  answered under before `results`, read only from an add-on without it and
  removed at major 3. A read with no pocket needs `results`; the
  `results` feature's meaning says so (`v2/registry/features.json`).
- `operation_response.json`'s `result` description names kind profiles.
- Fixtures: new `operation_response_records.json`,
  `operation_response_activity_result.json`,
  `operation_response_health_result.json` (the TrueNAS add-on's three read
  replies and the inventory's) and `manifest_inventory.json` (kind
  `inventory`, `holdings.list`); `operation_response_result.json` is the
  storage profile as the TrueNAS add-on sends it; `activity.json`'s event
  carries `resource` instead of `x-truenas-share`; `manifest_truenas.json`
  declares `results`.

## Major 2, rules the author's guide review found (slice 5, 11 Nov 2026)

Backward compatible within major 2 (owner decision, as above): each rule was
already what Syndra did, and is now written down and checked.

- `TARGET_UNREACHABLE` covers writes (`registry/errors.json`): a write that
  sent the app nothing answers 503 `TARGET_UNREACHABLE` with `Retry-After`, and
  Syndra keeps the change queued; a write that may have reached the app stays
  `APPLY_FAILED`/`OPERATION_FAILED` with a 5xx. The TrueNAS add-on answered
  every outage `APPLY_FAILED` 503, which sent each queued change to a person.
- A `limit` sent as `[]` sets nothing; `state` and `observed` hold exactly one
  value for a limit (`common/field.json`). The reference add-on cleared it.
- An `effect: read` operation ignores `call_id` (`openapi.yaml`,
  `identity/operation.json`): Syndra sends one `call_id` for every read.
- `health.log_head` is never empty (`health.json`, `minLength: 1`): the digest
  of the empty log until the first record. Syndra refuses an empty head at
  first sight; both add-ons reported `""` on a fresh log.
- Fixtures: `subjects.json` holds one `home_quota` value; `subjects_stale.json`
  drops it (absent is unknown); the plan and apply pair name one account;
  `group` values are sorted as Syndra sends them; `operation_request.json` is
  the `account.adopt` call `operation_response.json` answers, with nothing
  Syndra does not send.

## Major 2: `product` is a display name (slice 3, 11 Nov 2026)

Backward compatible within major 2: the shape is unchanged, and no end keys
anything on the value. The manifest's `product` is what people call the
product, and Syndra shows it as a system's name until an operator names the
system (addon-contract design §6.1). `v2/schema/manifest.json` (and `health.json`) now say so;
the TrueNAS add-on reports `"TrueNAS"` (it reported `"truenas_scale"`), in its
manifest and its health. Fixtures `manifest.json`, `manifest_truenas.json`,
`health.json`.

## Major 2, amended before release (slice 2, 11 Nov 2026)

v2 has not been released to anything but the TrueNAS add-on, which moves with
it, so these amend major 2 rather than add a feature (owner decision):

- `storage.status`'s `storage` pocket requires `standing` (`common/standing.json`):
  the subject's own account standing, so a member's page says it without a full
  `/subjects` read. `v2/schema/operation_response.json` (`StorageStatus`);
  fixture `operation_response_storage.json`.
- An apply's `observed` carries a managed `limit` field as the effective limit
  the target holds after the write, as `/subjects` `state` reports it, and
  leaves it out when it could not be read. Shape unchanged (`FieldValues`);
  `v2/schema/apply_response.json` says so.

## Major 2 on the wire (slice 1)

The TrueNAS add-on and the backend speak contract 2, deployed together in one
deploy whose order is in `.github/workflows/deploy.yml` and `_staging.yml`
(stop the backend, stop the add-on, copy its `state.db`, start both). `v1/` is
deleted: the backend supports one major (decision 2). The add-on's store is
written additively (bindings carry `account_id` beside `uid`; a recorded call
carries its identity digest beside what contract 1 wrote), so rolling back both
containers restores nothing. The backend still reads a contract-1 refusal's
`error` key, for classification only, until the next major (design §11).

No schema, fixture or registry file changed in slice 1.

## Major 2 (files in `v2/`, slice 1a, schemas only)

Written before anything sent it; slice 1 put it on the wire (above). Migration:
the add-on and the backend move together in one deploy (design §2, decision 1).

What changes the meaning of something that exists (the reason this is a major):
every write carries `contract_version` and `call_id` in the body and is
identified by an RFC 8785 digest; the manifest's `type`/`lifecycle`/`single`
give way to `role`; `uid` becomes `account_id` (subjects, apply, conflict, homes, the adopt reply); `read_at` becomes `taken_at`;
`usage_read` becomes `quota_read`; `password_set` leaves the wire; every
non-2xx is RFC 9457 `application/problem+json`; `CALL_ID_REUSED` answers 422; a blocked or conflicting apply is a problem+json (`APPLY_BLOCKED` 422, `BINDING_CONFLICT` 409) with the outcome as extension members; a preview carries no `call_id`;
`acknowledge_scope` and `SCOPE_ACKNOWLEDGEMENT_REQUIRED` leave the add-on; the
query string joins the MAC.

| File | What it is | Fixture |
|---|---|---|
| `v2/schema/manifest.json` | `kind`, `features[]`, `role` per field (`if`/`then` on `limit`), `metrics[]`, `limits`, `x-` rules | `manifest.json`, `manifest_truenas.json` |
| `v2/schema/health.json` | `credential_scopes`, `credential_expiry`/`credential_expires_at`, `target_reads` (freshness) | `health.json` |
| `v2/schema/subjects.json` | `taken_at`, `current`, `truncated`, `stale`; `account_id`; `standing` | `subjects.json`, `subjects_stale.json` |
| `v2/schema/values.json` | unchanged shape; exempt from `taken_at`/`current` | `values.json` |
| `v2/schema/plan_request.json`, `plan_response.json` | no `acknowledge_scope`; reply gains `stale` | `plan_request.json`, `plan_response.json` |
| `v2/schema/apply_request.json`, `apply_ok.json` (the /apply 200 body), `apply_response.json` (also plan outcomes) | `account_id` + `standing` together or neither; conflict carries `account_id` | `apply_request.json`, `apply_response.json` |
| `v2/schema/operation_request.json`, `operation_response.json` | `preview` (calls), `fresh` (freshness), `result` (every read's answer, slice 7h) | `operation_request.json`, `operation_response*.json` |
| `v2/schema/lifecycle_request.json`, `lifecycle_response.json` | `call_id` on the request | `lifecycle_request.json`, `lifecycle_response.json` |
| `v2/schema/settings.json`, `setting.json`, `setting_put_request.json` | `contract_version` and `call_id` in the body | `settings.json`, `setting.json`, `setting_put_request.json` |
| `v2/schema/profiles/storage/reach.json`, `homes.json` | `read_at` to `taken_at`; `current`, `stale`; `quota_read` | `reach.json`, `homes.json` |
| `v2/schema/error.json` | RFC 9457 body: `type`, `title`, `status`, `detail`, `code`, `retry` | `error.json`, `error_apply_blocked.json`, `error_binding_conflict.json` |
| `v2/schema/call.json` | `GET /calls/{call_id}` (feature `calls`) | `call.json` |
| `v2/schema/stats.json` | `GET /stats` (feature `stats`) | `stats.json` |
| `v2/schema/common/field.json` | `Role`, `FieldValues` (no null), `Unit`, `Duration`, `AccountId` | n/a |
| `v2/schema/common/stale.json` | `stale` and the `current` iff `stale` invariant | `subjects_stale.json` |
| `v2/schema/common/standing.json` | `standing` | `subjects.json` |
| `v2/schema/common/effect.json`, `outcome.json` | as contract 1 | n/a |
| `v2/schema/common/records.json`, `activity.json`, `status.json` | the three `result` shapes | `records.json`, `activity.json`, `status.json` |
| `v2/schema/identity/{apply,operation,lifecycle,setting}.json` | the request-identity objects of design §5 | n/a |
| `v2/schema/identity/vector.json` | a JCS golden vector | `identity_{apply,operation,lifecycle,setting}.json` |
| `v2/registry/roles.json`, `kinds.json`, `units.json`, `constants.json` | closed vocabularies and Syndra's own constants | n/a |
| `v2/registry/features.json` | every key a feature introduces, listed under it | n/a |
| `v2/registry/errors.json` | problem+json codes with titles; adds `UNKNOWN_REQUEST_FIELD`, `CALL_IN_PROGRESS`, `CALL_UNSETTLED`, `UNKNOWN_METRIC`, `FRESH_READ_COOLDOWN`, `TARGET_UNREACHABLE`, `PLAN_TOO_LARGE`, `UNKNOWN_ROUTE` (added in slice 4: every non-2xx is a problem with a registered code, so the 404 an undeclared route answers needs one); `CALL_ID_REUSED` to 422; adds `APPLY_BLOCKED`, `BINDING_CONFLICT`; drops `SCOPE_ACKNOWLEDGEMENT_REQUIRED` | `error.json` |
| `v2/registry/operations.json` | adds `meaning`, `effect`, `result` per operation | `operation_response_result.json` |
| `v2/openapi.yaml` | fourteen routes, `x-feature` gates, problem+json errors, the query in the signature | n/a |
| `transport_derivation.json` | gains `raw_query_signatures` (introduced by major 2): four vectors; the existing keys are untouched | n/a |

Contract 1 files moved, unchanged, to `v1/` in slice 1a, and were deleted by
slice 1.
