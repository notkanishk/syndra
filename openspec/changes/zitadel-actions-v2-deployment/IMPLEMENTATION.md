# Implementation Record: Zitadel Actions v2 Deployment

> **Status:** Code complete, pending staging smoke-test | [Proposal](proposal.md) | [Design](design.md) | [Tasks](tasks.md) | [DEPLOY](DEPLOY.md)

## What landed

### Backend

* `backend/internal/handlers/contracts.go` — new `decodeJSONLenient` helper that permits unknown fields while keeping the trailing-token guard.
* `backend/internal/handlers/action.go` — fully reshaped to the Zitadel Actions v2 contract:
  * Types: `ActionV2Request`, `ActionV2UserRef`, `ActionV2UserGrantRef`, `ActionV2Response`, `ActionV2Claim`.
  * `HandleActionInject` now reads `{function, user.id, user_grants[{projectId, roles}]}` via the lenient decoder and responds with `{append_claims:[{key,value}], append_log_claims?}`.
  * Helpers: `dedupProjectIDs`, `claimsForProject`, `claimsToEnvelope`.
  * Project selection: 0 → empty claims; 1 → flat keys; 2+ → `mkauth.<projectID>.<key>` namespaced keys.
  * `degradedResponse` preserves per-project `fail_closed` / `minimal_safe` semantics; returns empty envelope on DB outage.
* `backend/internal/handlers/zitadel_action_auth.go` — new HMAC-SHA256 middleware `withZitadelActionSignature`. Algorithm mirrors `github.com/zitadel/zitadel-go/v3/pkg/actions/signing.go` (confirmed 2026-04-23): header `ZITADEL-Signature` with `t=<unix>,v1=<hex>` pairs; hash input `<unix>.<body>`; 300 s tolerance; multiple `v1=` entries accepted for key rotation.
* `backend/internal/handlers/router.go:110` — `/api/action/inject` wrapped `withCORS(withZitadelActionSignature("ZITADEL_ACTION_SIGNING_KEY", HandleActionInject))`.

### Backend tests (113 total in `handlers` package, up from ~100)

* `action_test.go` — 12 v2-shaped tests: cache-miss fail_closed / minimal_safe, malformed cache, DB outage, cache-hit single-project flat keys, multi-project namespaced keys, multi-project partial-degrade isolation, empty grants short-circuit, duplicate-grant dedup, missing `user.id`, invalid JSON, unknown-fields-accepted.
* `action_v2_contract_test.go` — canonical Zitadel v2 function payload (all documented fields populated) round-trips to the v2 envelope with namespaced claims.
* `zitadel_action_auth_test.go` — 7 middleware tests: valid sig, body rewind, invalid sig 401, missing header 401, stale timestamp 401, dev-mode pass-through, rotated-key acceptance.
* `contracts_test.go` — legacy `TestHandleActionInjectRejectsMissingFields` updated to assert the v2 `user.id` detail.

### Deployment artifacts

* `zitadel/actions/` — new directory with `README.md`, `targets.json`, `register.sh` (executable, `bash -n` clean), `SIGNING_KEY.md`, `.gitignore` (excludes `.action-signing-key`).
* `scripts/smoke-test-action-v2.sh` — executable, uses python3 for HMAC when `ZITADEL_ACTION_SIGNING_KEY` is set; otherwise assumes dev pass-through. Asserts envelope shape with `jq`.
* `Makefile` — new targets `zitadel-actions-register`, `zitadel-actions-remove`, `zitadel-actions-verify`.
* `.env.example` — new commented block documenting `ZITADEL_ACTION_SIGNING_KEY`.
* `docker-compose.yml` — backend service now consumes `ZITADEL_ACTION_SIGNING_KEY` env.

#### API-correctness review pass (2026-04-23)

A follow-up review flagged that the first draft of the deployment artifacts
was speaking the wrong Zitadel API dialect. Fixed in this change:

* **Target type:** `restWebhook` → `restCall`. Webhook targets only inspect
  the HTTP status code and discard the response body — which is where the
  claim envelope lives. A webhook-typed target would have made the whole
  deployment a functional no-op. (`zitadel/actions/targets.json`)
* **Target body layout:** `name`, `endpoint`, and `timeout` moved to the
  top level of the Target object (per `target.proto`); `interruptOnError`
  stays inside the `restCall` submessage.
* **API base path:** `/resources/v3beta/actions/*` → `/v2/actions/*`.
* **Target search:** `POST /v2/actions/targets/_search` → `POST /v2/actions/targets/search`
  (no leading underscore) with body `{filters:[{target_name_filter:{name}}],
  pagination}`.
* **Target update verb:** `PATCH /v2/actions/targets/{id}` →
  `POST /v2/actions/targets/{id}`.
* **SetExecution `targets` payload:** `[{target: "<id>"}]` → `["<id>"]`
  (per-proto `array<string>`).
* **Unbind:** `DELETE /v2/actions/executions` with body → `PUT
  /v2/actions/executions` with `targets: []`.
* **Target-search filter field (P1 follow-up):** `target_name_filter.name` →
  `target_name_filter.target_name` with explicit
  `method: "TEXT_FILTER_METHOD_EQUALS"`. The inner field is `target_name`
  per `TargetNameFilter` in `action/v2/query.proto`. The earlier wrong
  field name would have silently matched nothing, causing every re-run to
  fall into the create path instead of updating in place — duplicate
  targets on the second invocation.
* **Proto source citations (v2 vs v2beta):** earlier revisions of this doc
  cited `proto/zitadel/action/v2beta/*.proto`. The stable location is
  `proto/zitadel/action/v2/*.proto`; both directories exist in the Zitadel
  repo and the stable v2 protos have identical message shapes (Target with
  name/endpoint/timeout/payload_type at top level, TargetNameFilter using
  `target_name`, Condition oneof with function/event/request/response,
  Execution.targets as `repeated string`). The wire API path is and has
  been the stable `/v2/actions/*` — no code path change, docs now cite the
  stable source of truth.
* **Explicit `payloadType`:** added `"payloadType":"PAYLOAD_TYPE_JSON"` to
  `targets.json` at the top level of the target body. The v2 Target proto
  documents this field alongside name/endpoint/timeout; setting it
  explicitly immunizes the manifest against a future default change.

#### Rotate-command follow-up (2026-04-24)

Research pass before deciding whether to build rotation automation surfaced
a decisive fact: **Zitadel does not expire the Action target signing key.**
Verified against `CreateTargetRequest` in `proto/zitadel/action/v2/target.proto`
(no expiration field) and `UpdateTargetRequest.expiration_signing_key` which
is a rotation trigger (currently `"0s"` only) and not a TTL. The first key
works indefinitely; rotation is a MkAuth *policy* choice, not a Zitadel
requirement.

Given that premise the following were considered and explicitly scoped out:

* In-process scheduler goroutine in the backend (no existing precedent;
  backend is pure request-response).
* Dual-key acceptance middleware (`ZITADEL_ACTION_SIGNING_KEY_PREVIOUS`
  alongside current). Deferred until Zitadel enables longer
  `expiration_signing_key` graceful periods — at which point dual-key buys
  zero-downtime rotation; today it just adds a lever no real flow needs.
* New `action_signing_keys` Postgres table + PG-LISTEN hot reload.
* Dedicated task-runner container (sync/ pattern extended for scheduled ops).

What shipped instead:

* **`zitadel/actions/rotate.sh`** — one-shot rotate. Resolves M2M token via
  the same `ZITADEL_M2M_TOKEN` / `ZITADEL_MACHINE_KEY_PATH` paths as
  `register.sh`; searches the target by name using the proven
  `target_name_filter.target_name` + `TEXT_FILTER_METHOD_EQUALS` body;
  calls `POST /v2/actions/targets/{id}` with
  `{"expirationSigningKey":"0s"}`; captures the new `signingKey` from the
  response; copies the current `.action-signing-key` to
  `.action-signing-key.previous` (mode 0600) and overwrites the active key
  file; prints the operator env-swap + restart instructions and the
  verification-gap disclaimer.
* **`make zitadel-actions-rotate-key`** — `.PHONY` + target that invokes
  the script.
* **`zitadel/actions/.gitignore`** — now excludes both `.action-signing-key`
  and `.action-signing-key.previous`.
* **Docs** — `SIGNING_KEY.md` gained a "Zitadel does not expire the signing
  key" subsection (with proto citation) explaining why rotation is not
  scheduled; the rotation procedure now points at the make target, with
  the raw curl retained as a deep-dive fallback. `DEPLOY.md` operator
  warnings updated accordingly. Living `application-claims/spec.md` got a
  one-paragraph statement of the no-expiry fact and the on-demand-only
  rotation posture.

#### Rotation-status panel (2026-04-24)

Follow-up to the rotate-command work: operators still had no visibility
into *how old* the current signing key was, so a 90-day-policy shop would
have to remember manually. Built observability without adding a
click-to-rotate footgun.

* **Backend** — new handler `backend/internal/handlers/rotation_status.go`
  exposes `GET /api/v1/zitadel/action-rotation-status` (gated by
  `withOperatorAuth`). Reads `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`
  (RFC3339 UTC) and `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS`
  (default 90) from the environment, emits
  `{last_rotated_at, age_days, threshold_days, status, rotate_command}`.
  Status semantics: `ok` (age < threshold), `warn` (threshold ≤ age <
  2×threshold), `stale` (age ≥ 2×threshold), `unknown` (env unset or
  malformed). 11 new unit tests covering every boundary + the invalid/
  zero/negative threshold fallbacks + the 405 on non-GET.
* **rotate.sh** — captures a fresh `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`
  timestamp (RFC3339 UTC) on every rotation, mirrors it to a local
  `.action-signing-key.rotated_at` sidecar (gitignored), and writes a
  ready-to-append env fragment at `zitadel/actions/.action-env.fragment`
  (mode 0600) containing both the new `ZITADEL_ACTION_SIGNING_KEY` and
  `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` lines. Operators apply the
  fragment with `cat … >> .env` (or `install -m 0600` for systemd
  `EnvironmentFile` deploys) — no copy-paste from the terminal. After
  applying and restarting, the `/zitadel` panel flips to `ok` with age 0.
  *(The first cut of this work emitted the two lines in a terminal
  guidance block and asked operators to paste them; the "Env-fragment
  follow-up (2026-04-24)" subsection below records why that was replaced
  with the redirect flow.)*
* **Compose / env** — `.env.example` documents both new env vars;
  `docker-compose.yml` threads both through the backend service.
* **UI** — new `RotationStatusSection` card on `/zitadel` between Health
  and Projects. Shows a colored badge (ok / warn / stale / unknown), last
  rotated timestamp, age in days, configured threshold, contextual
  guidance text, and a read-only `<code>make zitadel-actions-rotate-key</code>`
  snippet with a "Copy" button. Explicit design choice: **no button that
  triggers rotation.** Rationale captured inline: rotation is a
  cryptographic mutation whose failure modes are easier to debug with the
  operator running the command themselves in a terminal with full context.
* **Docs** — `SIGNING_KEY.md` gained a "Rotation observability (the
  Status panel)" section. `DEPLOY.md` Step 2 now seeds `ROTATED_AT` on
  first install (using `date -u` as the fallback when the `.rotated_at`
  file doesn't yet exist) and includes two new troubleshooting rows.
* **Living spec** — `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md`
  gained a "Rotation status MUST be observable to operators" requirement
  with scenarios covering each status state and the no-click-rotate
  safety property.

#### Rotation-status review follow-up (2026-04-24)

Review pass flagged two correctness gaps in the first-cut rotation status
and one stale reference in the living spec:

* **P2: status ignored whether a key was actually installed.** The first
  draft reported `ok`/`warn`/`stale` purely from `ROTATED_AT`, so a backend
  running with `ZITADEL_ACTION_SIGNING_KEY` unset (dev-mode pass-through —
  signature verification disabled) would still render a green badge.
  That's the exact scenario the panel was supposed to surface.
  Fix: added a new highest-precedence `disabled` status plus a
  `key_installed` boolean to the response. When the key is unset, the
  handler returns `disabled` unconditionally and omits `last_rotated_at` /
  `age_days`. UI renders `disabled` as a destructive badge with
  explanatory text pointing at the missing env var — not as a benign
  missing-config state.
* **P3: future-dated timestamps were silently clamped to age 0.**
  `ageInDays` used to clamp negative durations to 0, so a typo or clock
  skew putting `ROTATED_AT` in the future would report `ok` forever. Fix:
  removed the clamp (function is now a pure subtraction), and the handler
  explicitly checks `t.After(now)` before computing age, returning
  `unknown` with a log line that includes the hours-in-the-future delta.
* **P3: stale `restWebhook` reference in the canonical living spec.**
  `application-claims/spec.md` still had "`restWebhook.interruptOnError:
  false` MUST be preserved" in the rollback scenario from before the
  target type was corrected. Flipped to `restCall.interruptOnError`.
  Also caught one more stale occurrence at line 59 of the change-dir's
  MODIFIED spec delta and corrected that too.

Two new tests (disabled-when-key-unset, unknown-when-timestamp-in-future)
plus rewrote the existing tests to set `ZITADEL_ACTION_SIGNING_KEY` via
`withKeyInstalled(t)` so they don't accidentally collapse into
`disabled`. Full rotation test count: 13 (up from 11). Full backend
suite still 223 passing.

#### Env-fragment follow-up (2026-04-24)

P1 review on `rotate.sh` flagged that the operator-guidance heredoc could
strand unevaluated `$(cat …)` substitution in `.env` if copied
literally. The specific claim was a false positive — Bash's unquoted
heredoc *does* perform command substitution, so the resolved key text
reaches the terminal — but the underlying concern about copy-paste
brittleness (terminal wrap on a long key, stderr interleaving, someone
copying from script source instead of output) is legitimate. Resolved by
eliminating the paste flow entirely.

* `rotate.sh` now writes `zitadel/actions/.action-env.fragment` (mode
  0600, via the `umask 077` already in effect for the key writes)
  containing the two env lines ready to append. Operator guidance was
  rewritten around `cat "$FRAGMENT_FILE" >> .env` (or an `install -m 0600`
  step for systemd EnvironmentFile deploys). No copy-paste surface.
* `zitadel/actions/.gitignore` now also excludes `.action-env.fragment`.
* `SIGNING_KEY.md` and `DEPLOY.md` updated to describe the redirect
  flow. Fresh-install step still uses an inline `printf + date -u` so the
  ROTATED_AT timestamp is seeded without needing a rotation first.
* Verified empirically: the fragment file gets mode 0600 under the
  active umask, and contents are `KEY=<bytes>\nROTATED_AT=<ts>\n` with no
  interpolation surprises.

#### Doc-drift cleanup (2026-04-24)

Follow-on P3 review caught four stale references to the superseded paste
flow that the env-fragment change failed to migrate at the same time:

* `zitadel/actions/SIGNING_KEY.md` "Rotation observability" subsection —
  the env-var description still said "set it by pasting the line
  `rotate.sh` prints".
* `openspec/changes/zitadel-actions-v2-deployment/DEPLOY.md`
  Troubleshooting table — two rows (`unknown`, `warn`/`stale`) still told
  operators to paste lines that `rotate.sh` no longer emits. Added a
  third row for the `disabled` status at the same time.
* `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md`
  — the living-spec env-var description mirrored the stale SIGNING_KEY.md
  wording.
* `.env.example` — the commented `ROTATED_AT` header said "paste the line
  it prints".

All four now describe the `cat .action-env.fragment >> .env` redirect
flow (with the `install -m 0600` fallback for systemd) and explicitly
contrast against copy-paste so future readers don't re-introduce the old
guidance.

#### `.env` auto-load (2026-04-24)

Operator question surfaced a real gap: `register.sh`, `rotate.sh`, and
`scripts/smoke-test-action-v2.sh` all relied on the process environment
for `ZITADEL_DOMAIN`, `MKAUTH_EXTERNAL_URL`, `ZITADEL_M2M_TOKEN` /
`ZITADEL_MACHINE_KEY_PATH`, etc. — with no layer sourcing `.env`. So
`make zitadel-actions-register` would fail `${ZITADEL_DOMAIN:?required}`
even with a fully-populated `.env` at the repo root, forcing operators
to remember `set -a && . .env && set +a && make …` as a workaround.

Fixed by inlining a small `.env` loader at the top of each script (after
`set -euo pipefail`, before the first `${VAR:?…}` check):

* Resolves the repo root relative to the script's own location
  (`zitadel/actions/*.sh` walks up two levels; `scripts/*.sh` walks up
  one).
* Silent when `.env` is absent (CI, bare clones, container builds don't
  produce spurious errors).
* Line-by-line parse via regex: `^[[:space:]]*KEY=VALUE$` with leading
  whitespace tolerated, blank lines and `#` comments skipped. A single
  layer of surrounding `"…"` or `'…'` is stripped from the value. `${VAR}`
  inside a value is kept literal — the loader doesn't re-implement shell
  expansion.
* **CLI override invariant preserved**: `[[ -z "${!key+x}" ]]` before
  export means an already-set env var is never overwritten by `.env`. A
  one-off `ZITADEL_DOMAIN=other.example.com make zitadel-actions-register`
  works as expected.
* Required-env error messages sharpened: the `${…:?}` suffix now reads
  "set in .env or export" so operators who still hit the error learn
  immediately where the value is supposed to live.

Empirically verified against a mock `.env` exercising every parsing
branch (plain, double-quoted, single-quoted, literal `${…}`, leading
whitespace, invalid-no-equals, `#` comment) plus the CLI-override and
missing-file paths.

Kept inline in all three scripts rather than extracted to a shared
helper: three copies of ~15 lines, vs a shared file forcing
`scripts/smoke-test-action-v2.sh` to cross-directory source something in
`zitadel/actions/`. The coupling would have been worse than the
duplication for a loader with no state, no branches, no imports.

Also rejected:
* Makefile-level `include .env; export` — fragile with quoted values
  (Make keeps the quotes in the exported value), and would only cover
  make-invoked runs. Inline script loader covers both `make …` and
  `bash zitadel/actions/rotate.sh` with one mechanism.
* `.env.local` / multi-file override support — MkAuth doesn't use these;
  adding now is speculative.

#### M2M token CLI (2026-04-24)

Review pass found that both `register.sh` and `rotate.sh` had a
`ZITADEL_MACHINE_KEY_PATH` branch that shelled out to
`go run ./backend/cmd/test -action=mint-m2m-token`. That command doesn't
exist in the form the scripts assumed — `backend/cmd/test/main.go` is the
cache/DB regression harness that connects to Postgres and ignores any
`-action` flag. So the advertised one-command machine-key flow was
actually broken: operators who filled in `.env` with
`ZITADEL_MACHINE_KEY_PATH` hit `fork/exec` succeeding, the test harness
failing on missing `DB_DSN`, stderr silenced via `2>/dev/null`, and the
scripts then reporting "could not mint M2M token — provide
ZITADEL_M2M_TOKEN directly."

Fixed by shipping a real helper:

* New exported `MintM2MToken(ctx, domain, keyPath)` in
  `backend/internal/zitadel/token.go` that reuses the existing
  `LoadServiceAccountKey` + `newTokenManager` path to do a one-shot JWT
  profile grant (RFC 7523) against the Zitadel token endpoint. No caching,
  no DB/Redis side effects — safe to call from a CLI context.
* New `backend/cmd/mkauth-token/main.go` — thin CLI that reads
  `ZITADEL_DOMAIN` + `ZITADEL_MACHINE_KEY_PATH` from the env (auto-loaded
  from `.env` via the script loader already in place), calls
  `MintM2MToken`, prints the Bearer token to stdout. Clear stderr
  messages + non-zero exit on every error class (missing env, key load
  fail, assertion build, token exchange).
* `register.sh` / `rotate.sh` now `cd backend && go run ./cmd/mkauth-token`
  — module resolution works because the `go run` is inside the module
  root. Stderr from the helper flows through to the operator verbatim
  instead of being silenced, so a bad key file produces "key file is
  empty" or "parse private key: ..." where before it produced a generic
  "could not mint" error.
* 3 new unit tests covering the rejection paths (`empty domain`, `empty
  keyPath`, `nonexistent key file`). Full backend suite: 226 passing (up
  from 223).

No cross-module coupling beyond the existing `mkauth/internal/zitadel`
import the backend already uses. Operators who can't install Go on the
mint host can still use `ZITADEL_M2M_TOKEN` — the preference order
preserves that.

##### Relative-path follow-up (same day)

Review caught a P1 regression immediately after the CLI landed: the
`cd backend && go run ./cmd/mkauth-token` pattern broke relative
`ZITADEL_MACHINE_KEY_PATH` values. `.env.example` documents these as
resolving against the repo root (the docker-compose directory), so a
value like `./zitadel-machine-key.json` should find the file at
`<repo>/zitadel-machine-key.json` — but after `cd backend`, Go's
`os.ReadFile` inside `mkauth-token` resolved it against `<repo>/backend/`
instead, silently looking for a file in the wrong directory.

Fix: resolve `ZITADEL_MACHINE_KEY_PATH` to an absolute path (anchored to
the already-computed `REPO_ROOT`) *before* the `cd backend`, and export
the absolute value so the child process sees it. Portable POSIX shell
`case` handles the three real shapes operators use:

* `/abs/path` → passed through unchanged.
* `~/rel/path` → expanded against `$HOME` (bash won't expand a tilde
  loaded from `.env` because that expansion only happens at parse time
  for unquoted tokens).
* everything else → prefixed with `REPO_ROOT/`.

Verified with a 6-case bash harness covering absolute, `./...`, bare
filename, `../...`, tilde, and the exact docs-example value
`./zitadel-machine-key.json`. All resolve to paths under the repo root
(or `$HOME` for the tilde case) regardless of which directory the
operator invoked `make` from.

Same fix applied to both `register.sh` and `rotate.sh`; both still pass
`bash -n`.
* **Signing-key rotation:** `SIGNING_KEY.md` updated to reflect in-place
  rotation via `UpdateTarget {"expirationSigningKey":"0s"}` instead of the
  "recreate target" workaround the first draft assumed.

All corrections verified against:

* `zitadel/zitadel:proto/zitadel/action/v2/target.proto`, `execution.proto`,
  and `query.proto` (raw GitHub fetch, 2026-04-24). The stable v2 directory
  supersedes the older `v2beta/` sibling that an earlier verification pass
  had cited; all message shapes match between the two, but the stable v2 is
  the authoritative source. Wire path is `/v2/actions/*`.
* `zitadel.com/docs/guides/integrate/actions/testing-function` (working
  walkthrough).
* `zitadel.com/docs/apis/resources/action_service_v2/*` endpoint pages.

`jq`-rendered `targets.json` → `{name, endpoint, timeout, restCall:{interruptOnError}}`
plus an `executions` list of `{condition, targets:["<id>"]}` bodies — matches
the documented shapes exactly.

### OpenSpec

* This change directory with `proposal.md`, `design.md`, `tasks.md`, this file, `DEPLOY.md`, and `specs/application-claims/spec.md` MODIFIED delta.
* `openspec/INDEX.md` — added to Change Log as Phase 5 Complete.
* `openspec/changes/mkauth-core-architecture/ROADMAP.md` — Phase 5 > Operations > Actions v2 Deployment ticked.
* `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` — Status flipped to Integrated; §Implementation wording corrected from v1 (`SetCustomClaims`) to v2 envelope (`append_claims[]`); trailing deferral paragraph removed.
* `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — `application-claims` row flipped Partial → Integrated; `Last updated` bumped to 2026-04-23.

## What did NOT change

* Cache compilation (`backend/internal/cache/*`) — unaffected; still writes `mapping:<user>:<project>` keys.
* Zitadel M2M client (`backend/internal/zitadel/*`) — unaffected.
* Sync service — unaffected.
* UI — unaffected (diagnostic UI already exercises the M2M path).

## Gaps carried forward

* **Staging smoke-test against a live Zitadel instance** — deferred until operator has credentials. Deployment sequence captured in [DEPLOY.md](DEPLOY.md); `make zitadel-actions-verify` will validate once the target is live.
* **Payload size ceiling** — `withMaxBody` is still 1 MB. If grant-heavy users trigger 413s at the middleware, raise to 4 MB. Not observed yet, so left alone.
* **Rotation without outage** — Zitadel does not publish a rotation API at target level. `SIGNING_KEY.md` documents the recreate-target procedure; the `interruptOnError: false` setting keeps users unblocked during the brief gap.
* **v1 collision detection** — no programmatic check that an operator hasn't also wired a v1 Action to the same trigger on a self-hosted Zitadel. Operator guidance in `DEPLOY.md`.

## Verification performed

* `cd backend && go test ./internal/handlers -v` — 113 tests pass (including 19 new).
* `cd backend && go build ./...` — clean.
* `bash -n zitadel/actions/register.sh` — clean.
* `bash -n scripts/smoke-test-action-v2.sh` — clean.
* `jq -e . zitadel/actions/targets.json` — JSON valid.
* Dry-run render of `targets.json` through the `register.sh` jq pipeline
  produces `{name, endpoint, timeout, restCall:{interruptOnError:false}}` at
  top level — matches the Zitadel v2beta Target proto exactly.
* `github.com/zitadel/zitadel-go/main/pkg/actions/signing.go` raw-fetched and re-read on 2026-04-23 before writing the middleware; algorithm matches.
* `zitadel/zitadel:proto/zitadel/action/v2/{target,execution,query}.proto`
  raw-fetched on 2026-04-24. Confirms (a) name/endpoint/timeout/payload_type
  at top level of Target, (b) `rest_call.interrupt_on_error` field 1,
  (c) `Condition.function = FunctionExecution{name}` oneof field 3,
  (d) `targets` = `repeated string` (array of ID strings),
  (e) `TargetNameFilter.target_name` (not `name`) with
  `zitadel.filter.v2.TextFilterMethod` enum.

#### Service-account permissions doc + visible HTTP errors (2026-04-24)

Operator hit `HTTP 403` on a first `make zitadel-actions-register` run
because their service user was scoped only to `ORG_OWNER` — the org
roles the backend's normal user/grant CRUD uses don't cover Actions v2
target management, which is instance-scoped.

Documentation:

* `DEPLOY.md` — new **Service-account permissions** subsection under
  Prerequisites. Explains why ORG_OWNER doesn't work, lists the exact
  `action.target.read` + `action.target.write` + `action.execution.write`
  (+ optional `action.target.delete`) permissions per script call,
  and gives three narrow-to-broad assignment paths (custom instance
  role → prebuilt action-scoped role → `IAM_OWNER` fallback). Also
  spells out that the permissions are only needed during register and
  rotate — steady state calls don't use them, so the role can be kept
  permanently, assigned-and-revoked per run, or scoped to a separate
  M2M key.
* `DEPLOY.md` troubleshooting table — new row pointing operators who
  hit 403 directly at the permissions section.
* `.env.example` — note added to the `ZITADEL_MACHINE_KEY_PATH` block
  clarifying that the ORG-level roles it lists are insufficient for the
  Actions v2 scripts, with a pointer to the DEPLOY.md section.

Script hardening (landed alongside the doc):

* New `zitadel_api METHOD PATH [BODY]` helper in both `register.sh` and
  `rotate.sh`, replacing every `curl -fsS` call. On HTTP error the
  helper prints the method + path + status + Zitadel's own JSON error
  body to stderr, so operators see "permission denied:
  action.target.write" instead of a bare `curl: (22)`. 401/403 responses
  include an inline hint pointing at the Default Settings →
  Administrators assignment. Verified against httpbin with 200 and 403
  fixtures; the 200 path is silent on stderr, the 403 path renders the
  full diagnostic.

##### Doc relocation + least-privilege alignment (same day)

Review caught two issues with the first cut:

* **Least-privilege conflict.** The runtime 401/403 hint in
  `zitadel_api` said *"requires IAM_OWNER on the service user"* while
  the new doc listed IAM_OWNER as the fallback and custom
  instance-scoped roles as the preferred path. An operator following
  the runtime hint would over-grant. Fixed by rewriting both scripts'
  hint blocks to: (1) name the minimum permissions
  (`action.target.read`, `action.target.write`,
  `action.execution.write`, optional `action.target.delete`), (2)
  enumerate the three assignment paths narrow-first (custom role →
  prebuilt → IAM_OWNER), and (3) point at the canonical doc.
* **Pointer rot from archive workflow.** The first version put the
  canonical content in `openspec/changes/zitadel-actions-v2-deployment/DEPLOY.md`
  and pointed `.env.example` at that path. When this change is
  archived, the path becomes `openspec/changes/archive/...` and every
  long-lived pointer rots. Fixed by extracting the canonical content to
  a new durable file:

  * **`zitadel/actions/PERMISSIONS.md`** (new) — full matrix, assignment
    paths, duration guidance, separate-M2M-key options. Lives under the
    operator tree so it survives any OpenSpec reorganization.
  * `DEPLOY.md` Prerequisites subsection reduced to a short pointer at
    the durable doc.
  * `DEPLOY.md` troubleshooting row, `.env.example` note, and
    `zitadel/actions/README.md` Contents table all repoint at
    `zitadel/actions/PERMISSIONS.md`.

  Any future archive of this change will leave `PERMISSIONS.md` and the
  other living-tree pointers untouched.
