# Contract changelog

One entry per major or feature: what it is, the schema files it touches, and the
fixture that changed. This file lives above both majors because it spans them.

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
| `v2/schema/operation_request.json`, `operation_response.json` | `preview` (calls), `fresh` (freshness), `result` (results); legacy pockets kept | `operation_request.json`, `operation_response*.json` |
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
| `v2/schema/common/records.json`, `activity.json`, `status.json` | the three `result` shapes (feature `results`) | `records.json`, `activity.json`, `status.json` |
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
