## Tasks

### Backend — lenient decoder

- [x] Add `decodeJSONLenient(io.Reader, any) error` to `backend/internal/handlers/contracts.go` alongside `decodeJSONStrict`. Preserves the trailing-token guard but allows unknown fields.

### Backend — reshape `HandleActionInject` to v2

- [x] Replace `ActionRequest` / `ActionResponse` with `ActionV2Request` / `ActionV2Response` in `backend/internal/handlers/action.go`.
- [x] `ActionV2Request` accepts Zitadel's function payload: `{function, user{id}, user_grants[{projectId, roles}]}` via `decodeJSONLenient`.
- [x] Multi-grant dedup by `projectId` preserving first-seen order.
- [x] Project-selection logic: 0 projects → empty `append_claims`; 1 project → flat claim keys; 2+ projects → namespaced `mkauth.<projectID>.<key>`.
- [x] `degradedResponse(ctx, projectID, namespace)` wraps `fail_closed` / `minimal_safe` output in `append_claims` envelope; DB outage defaults to fail_closed.
- [x] Per-project degradation does not suppress sibling projects in a multi-grant response.
- [x] Introduce helpers `dedupProjectIDs`, `claimsForProject`, `claimsToEnvelope`.

### Backend — HMAC signature middleware

- [x] Read `github.com/zitadel/zitadel-go/v3/pkg/actions/signing.go` to confirm header name, hash-input composition, encoding, and tolerance. Record findings in `design.md §Ship-blocking verifications`.
- [x] Create `backend/internal/handlers/zitadel_action_auth.go` implementing `withZitadelActionSignature(secretEnvVar, next)` using stdlib `crypto/hmac`, `crypto/sha256`, `encoding/hex`.
- [x] Parse `ZITADEL-Signature: t=<unix>,v1=<hex>[,v1=<hex>...]` — accept multiple `v1=` pairs (key rotation); any match passes.
- [x] Enforce 300 s tolerance (matching Zitadel's DefaultTolerance) in both directions.
- [x] Dev-mode pass-through when env var is empty — log warning, do not verify.
- [x] Rewind `r.Body` so the downstream decoder works.
- [x] Wire into `backend/internal/handlers/router.go`: `POST /api/action/inject` wrapped `withCORS(withZitadelActionSignature("ZITADEL_ACTION_SIGNING_KEY", HandleActionInject))`.

### Backend — tests

- [x] Rewrite the 5 existing tests in `backend/internal/handlers/action_test.go` to the v2 request/response shape.
- [x] Add multi-project namespacing, multi-project partial-degrade, empty-grants short-circuit, duplicate-project dedup, missing `user.id`, invalid JSON, and unknown-fields-accepted tests.
- [x] Update the legacy `TestHandleActionInjectRejectsMissingFields` in `contracts_test.go` to the v2 shape (assert `user.id` detail).
- [x] New `backend/internal/handlers/action_v2_contract_test.go` with a verbatim canonical Zitadel v2 function payload round-trip.
- [x] New `backend/internal/handlers/zitadel_action_auth_test.go`: valid signature passes, body rewound for downstream decoder, invalid signature 401s, missing header 401s, stale timestamp 401s, dev-mode pass-through passes, rotated-key acceptance.
- [x] **CHK**: `cd backend && go test ./... && go vet ./...` — all green (113 handlers tests).

### Deployment artifacts

- [x] Create `zitadel/actions/README.md` — explains v1 vs v2 and file purposes.
- [x] Create `zitadel/actions/targets.json` — declarative target + executions with `interruptOnError: false` and `preaccesstoken` + `preuserinfo` triggers.
- [x] Create `zitadel/actions/register.sh` — idempotent installer: look up by name, upsert target, capture `signingKey` once to `.action-signing-key` (mode 0600), bind/unbind executions. `--remove` path deletes executions only.
- [x] Create `zitadel/actions/SIGNING_KEY.md` — credential lifecycle, rotation procedure, storage.
- [x] Create `zitadel/actions/.gitignore` — excludes `.action-signing-key`.
- [x] Create `scripts/smoke-test-action-v2.sh` — canonical v2 body, HMAC-signed when `ZITADEL_ACTION_SIGNING_KEY` is set, asserts envelope shape (200, `append_claims` is array, each entry has `{key,value}`).
- [x] Add `ZITADEL_ACTION_SIGNING_KEY` to `.env.example` with dev/prod note.
- [x] Thread `ZITADEL_ACTION_SIGNING_KEY` through `docker-compose.yml` backend env.
- [x] Add `zitadel-actions-register`, `zitadel-actions-remove`, `zitadel-actions-verify` to root `Makefile`.
- [x] **CHK**: `bash -n` passes on `register.sh` and `smoke-test-action-v2.sh`.

### OpenSpec

- [x] Create `openspec/changes/zitadel-actions-v2-deployment/{proposal,design,tasks,IMPLEMENTATION,DEPLOY}.md`.
- [x] Create `openspec/changes/zitadel-actions-v2-deployment/specs/application-claims/spec.md` with MODIFIED requirements capturing: (a) v1 → v2 envelope wording correction, (b) Actions v2 target deployment completed, (c) Status flip to Complete.
- [x] Add the change to `openspec/INDEX.md` Change Log table under Phase 5.
- [x] Tick Phase 5 > Operations > Actions v2 Deployment box in `openspec/changes/mkauth-core-architecture/ROADMAP.md`.
- [x] Flip Status header of `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` from Partial to Integrated; remove the trailing "Deferred to Phase 5" paragraph; replace v1 wording in §Implementation with the v2 envelope.
- [x] Flip `application-claims` row in `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` from Partial → Integrated; bump `Last updated` to 2026-04-23.

### Codebase memory

- [ ] Run `mcp__codebase-memory-mcp__detect_changes` then re-index the backend module so the graph reflects the reshaped handler, new middleware, and new files.

### Final verification

- [ ] `cd backend && go test ./... && go vet ./...` — all green.
- [ ] `cd ui && bun run lint && bun run test && bun run build` — all green (no UI changes, but confirm no regression).
- [ ] `openspec validate zitadel-actions-v2-deployment --strict` — passes.
- [ ] Manual: start `go run ./backend/cmd/api`, run `scripts/smoke-test-action-v2.sh` — receives 200 with `append_claims` array (dev mode, no signing key).

### Rotate-command follow-up (2026-04-24)

Zitadel does not expire the Action target signing key (`CreateTargetRequest` has no expiration field; `UpdateTargetRequest.expiration_signing_key` is a rotation trigger, currently `"0s"` only). Rotation is a MkAuth policy choice — not scheduled, runs on demand.

- [x] Create `zitadel/actions/rotate.sh`: M2M token resolution, target lookup by name, `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`, capture new key, back up previous key to `.action-signing-key.previous`, write new key, print operator env-swap + restart guidance.
- [x] Add `zitadel-actions-rotate-key` to `Makefile` `.PHONY` and as a target wrapping the script.
- [x] Extend `zitadel/actions/.gitignore` to exclude `.action-signing-key.previous` alongside `.action-signing-key`.
- [x] `SIGNING_KEY.md`: add "Zitadel does not expire the signing key" subsection with proto citation; rewrite the Rotation section to point at `make zitadel-actions-rotate-key`; keep raw curl as a deep-dive fallback; describe `.action-signing-key.previous` as audit-only (backend reads a single env var, not this file).
- [x] `DEPLOY.md`: rotation bullet in operator warnings now points at the make target; add an explicit "do not schedule" bullet.
- [x] Amend living `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` §Actions v2 Target Deployment with a paragraph recording the no-expiry fact and the on-demand-only rotation posture.
- [x] `IMPLEMENTATION.md`: new "Rotate-command follow-up (2026-04-24)" subsection enumerating what was built, what was deliberately scoped out, and why.
- [x] `bash -n zitadel/actions/rotate.sh` — syntax clean.
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — still passes after the addendum.
- [ ] Operator manual validation against live Zitadel: run `make zitadel-actions-register` to bootstrap, then `make zitadel-actions-rotate-key` and verify new key in both response and `.action-signing-key`, old key in `.action-signing-key.previous`. Swap env var, restart backend, confirm `make zitadel-actions-verify` passes.

### Rotation-status panel (2026-04-24)

Observability for the rotate command: operators need to see key age at a glance without putting a cryptographic mutation behind a single click. Scope: read-only endpoint + read-only UI panel.

- [x] `backend/internal/handlers/rotation_status.go`: new handler `HandleActionRotationStatus`. Reads `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` (RFC3339 UTC) and `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` (default 90) from env. Emits `{last_rotated_at, age_days, threshold_days, status, rotate_command}`. Status ladder: ok (< threshold), warn (≥ threshold), stale (≥ 2×threshold), unknown (env unset/malformed).
- [x] `backend/internal/handlers/rotation_status_test.go`: 11 tests — boundary pairs for classify, negative age clamp, unset env, malformed env, fresh/ok, past-threshold warn, way-past stale, custom threshold, invalid threshold fallback, zero/negative threshold fallback, non-GET 405.
- [x] `backend/internal/handlers/router.go`: wire `GET /api/v1/zitadel/action-rotation-status` with `withOperatorAuth`.
- [x] `zitadel/actions/rotate.sh`: after successful rotation, compute `$(date -u +%Y-%m-%dT%H:%M:%SZ)`, write to `.action-signing-key.rotated_at`, and print the `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=…` env line alongside the new key line in the operator-guidance block.
- [x] `zitadel/actions/.gitignore`: add `.action-signing-key.rotated_at`.
- [x] `.env.example`: add `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` and `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` (commented, with defaults documented).
- [x] `docker-compose.yml`: thread both env vars through the backend service.
- [x] `ui/src/app/zitadel/page.tsx`: new `RotationStatusSection` card between Health and Projects. Badge (ok/warn/stale/unknown), last-rotated timestamp, age, threshold, contextual guidance text, read-only copyable `make zitadel-actions-rotate-key` snippet. Explicit no-click-rotate design with rationale commented inline.
- [x] `SIGNING_KEY.md`: new "Rotation observability (the Status panel)" section documenting the endpoint, status ladder, and the two env vars.
- [x] `DEPLOY.md`: Step 2 seeds `ROTATED_AT` on first install with `date -u` fallback; two new troubleshooting rows for `unknown` and `warn`/`stale` states.
- [x] `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md`: new requirement "Rotation status MUST be observable to operators" with scenarios for each status state and the no-click-rotate safety property.
- [x] `cd backend && go test ./internal/handlers/ -run TestHandleActionRotationStatus` — 11 pass.
- [x] `cd ui && bun run lint && bun run build` — clean (zitadel page grew from 4.19 → 4.84 kB).
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — passes after the addendum.
- [x] `mcp__codebase-memory-mcp__detect_changes` + `index_repository` — picks up new handler and UI section.

### Rotation-status review follow-up (2026-04-24)

Review surfaced P2 (status didn't consider whether the signing key was actually installed) and P3 (future-dated timestamps silently clamped to `ok`, plus a stale `restWebhook` reference in the canonical living spec).

- [x] Add `KeyInstalled bool` to `ActionRotationStatus`; highest-precedence `disabled` status when `ZITADEL_ACTION_SIGNING_KEY` is unset. Response omits `last_rotated_at` and `age_days` in the disabled branch.
- [x] Explicit `t.After(now)` check in the handler for future-dated `ROTATED_AT` — return `unknown` with a log line including the hours-ahead delta.
- [x] Drop the negative-clamp from `ageInDays`; function is now a pure signed-delta helper. Handler owns the "is this age meaningful?" question.
- [x] Rotation-status tests: new `TestHandleActionRotationStatus_KeyUnset_Disabled` and `TestHandleActionRotationStatus_FutureRotatedAt_Unknown`; existing tests wrapped in a new `withKeyInstalled(t)` helper so they don't collapse into `disabled`; updated `TestAgeInDays_ReturnsSignedDelta` to document the non-clamped semantics. 13 rotation tests (up from 11).
- [x] `ui/src/app/zitadel/page.tsx`: `RotationStatus` interface gains `key_installed: boolean`; union includes `"disabled"`. Badge renders `disabled` destructive; contextual text explains verification is off and points at `ZITADEL_ACTION_SIGNING_KEY`.
- [x] Living `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md`: rollback scenario now says `restCall.interruptOnError: false` (was `restWebhook.…`). Status ladder section expanded to describe precedence (disabled → unknown → ok/warn/stale), and two new scenarios cover future-timestamp and key-unset cases.
- [x] `openspec/changes/zitadel-actions-v2-deployment/specs/application-claims/spec.md`: same `restWebhook` → `restCall` correction at line 59 of the MODIFIED delta.
- [x] `SIGNING_KEY.md` status ladder documents `disabled` as highest precedence + `key_installed` field.
- [x] `go test ./internal/handlers/ -run TestHandleActionRotationStatus` — 10 pass; full backend suite 223 pass; `go vet` clean.
- [x] `bun run lint && bun run build` — clean (`/zitadel` at 5.00 kB).
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — still passes.
- [x] `mcp__codebase-memory-mcp__detect_changes` + `index_repository`.

### Env-fragment follow-up (2026-04-24)

P1 review flagged the operator-guidance heredoc as a copy-paste hazard. Bash's unquoted heredoc *does* expand `$(cat …)` substitution (the claim was a false positive), but the underlying paste-flow brittleness is legitimate. Eliminated paste entirely.

- [x] `zitadel/actions/rotate.sh` now writes `.action-env.fragment` (mode 0600 via the already-active `umask 077`) with `ZITADEL_ACTION_SIGNING_KEY=…` and `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=…` lines.
- [x] Operator guidance rewritten to use `cat "$FRAGMENT_FILE" >> .env` (or `install -m 0600` for systemd) — no copy-paste from the terminal at all.
- [x] `zitadel/actions/.gitignore` excludes `.action-env.fragment`.
- [x] `SIGNING_KEY.md` rotation section updated with the redirect-based flow and a note that the fragment should be deleted after use.
- [x] `DEPLOY.md` Step 2: fresh-install inline still uses `printf + date -u` (no rotate has run yet so no fragment exists); subsequent rotations are the redirect flow.
- [x] `bash -n zitadel/actions/rotate.sh` — clean. Fragment mode + content validated with an isolated mock (`umask 077`; resulting file is 0600 with two literal `KEY=…` lines).

### Doc-drift cleanup (2026-04-24)

P3 review flagged two stale references to the superseded paste flow that the fragment-file change missed. Grep-audited the rest of the tree and caught two more.

- [x] `zitadel/actions/SIGNING_KEY.md` "Rotation observability" env-var description rewritten around the fragment redirect.
- [x] `openspec/changes/zitadel-actions-v2-deployment/DEPLOY.md` Troubleshooting table: `unknown` and `warn`/`stale` rows now reference the fragment; added a third row for the `disabled` status (P2 follow-up).
- [x] `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` env-var description updated.
- [x] `.env.example` `ROTATED_AT` header updated.
- [x] Grep audit confirms remaining "paste" references are contrastive ("never by copy-paste") or historical review notes in this change-dir.

### `.env` auto-load (2026-04-24)

Operator-asked: scripts didn't read `.env` from the repo root, forcing a `set -a && . .env` pre-step that nobody remembers on first incident.

- [x] `zitadel/actions/register.sh`: inline loader after `set -euo pipefail`, before the `${VAR:?…}` checks. Resolves `_ENV_FILE` from `SCRIPT_DIR/../..`, parses KEY=VALUE lines with regex allowlist, strips optional surrounding quotes, skips blanks/comments, respects CLI-override invariant via `[[ -z "${!_k+x}" ]]`. Silent when `.env` is absent.
- [x] `zitadel/actions/rotate.sh`: same loader block.
- [x] `scripts/smoke-test-action-v2.sh`: same loader block, `_ENV_FILE` resolved from `SCRIPT_DIR/..` (one level up, not two).
- [x] All three scripts: required-env error messages appended with "(set in .env or export)".
- [x] Loader behaviour validated with a mock `.env` exercising plain values, double/single-quoted values, literal `${KEY}` values, leading-whitespace tolerance, invalid-no-equals lines, `#` comments. CLI-override invariant verified (parent-shell `KEY=cli_override` wins over `.env` `KEY=value_a`). Missing-file path is silent (loader returns; downstream `${…:?}` still triggers the clear "required" error).
- [x] `zitadel/actions/SIGNING_KEY.md` Lifecycle section notes the auto-load + CLI-override invariant.
- [x] `openspec/changes/zitadel-actions-v2-deployment/DEPLOY.md` Prerequisites + Step 1 rewritten around the `.env`-first flow; inline `KEY=val make …` shown as the override path.
- [x] `openspec/changes/zitadel-actions-v2-deployment/IMPLEMENTATION.md` new "`.env` auto-load (2026-04-24)" subsection recording the decision, the CLI-override invariant, and the inline-vs-shared-helper tradeoff.
- [x] `bash -n` clean on all three scripts.
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — passes.
- [x] `mcp__codebase-memory-mcp__detect_changes` + `index_repository` refresh.

### M2M token CLI (2026-04-24)

Review found the machine-key mint path in register.sh/rotate.sh was shelling out to a non-existent helper (`go run ./backend/cmd/test -action=mint-m2m-token` — backend/cmd/test is the DB/Redis regression harness, not a token helper). Operators relying on `ZITADEL_MACHINE_KEY_PATH` hit a silent failure with stderr swallowed.

- [x] `backend/internal/zitadel/token.go`: new exported `MintM2MToken(ctx, domain, keyPath) (string, error)` — one-shot JWT-profile grant; wraps LoadServiceAccountKey + newTokenManager.Token.
- [x] `backend/cmd/mkauth-token/main.go`: new CLI that reads ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH from env, calls MintM2MToken, prints Bearer token to stdout. Clear errors on every failure class.
- [x] `zitadel/actions/register.sh`: machine-key branch now runs `(cd backend && go run ./cmd/mkauth-token)` so module resolution works. No stderr silencing — operator sees the real error from the helper.
- [x] `zitadel/actions/rotate.sh`: same fix.
- [x] 3 new unit tests in `backend/internal/zitadel/mint_m2m_token_test.go` covering empty domain / empty keyPath / nonexistent file rejection paths.
- [x] `DEPLOY.md` Prerequisites note that the machine-key path requires Go toolchain on the host (because it invokes `go run ./cmd/mkauth-token`).
- [x] `bash -n` clean on both scripts; `go build ./cmd/mkauth-token` clean; `go test ./...` 226 pass (up from 223); `go vet ./...` clean.
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — passes.
- [x] `mcp__codebase-memory-mcp__detect_changes` + `index_repository` refresh.
- [x] **Relative-path follow-up:** resolve `ZITADEL_MACHINE_KEY_PATH` to an absolute path (anchored to `REPO_ROOT`) before the `cd backend`. `.env.example` documents relative paths as resolving against the repo root / docker-compose directory; `cd backend` had silently reinterpreted them against `backend/` and would have broken the documented `./zitadel-machine-key.json` style path. POSIX `case` covers absolute, `./...`, bare, `../...`, and `~/...` inputs. Verified with a 6-case bash harness. Both scripts updated identically.

### Service-account permissions doc + visible HTTP errors (2026-04-24)

Operator hit `HTTP 403` on first `make zitadel-actions-register` run with an ORG_OWNER-only service user. Actions v2 target management is instance-scoped; the org roles `.env.example` lists don't cover it.

- [x] `DEPLOY.md`: new **Service-account permissions** subsection under Prerequisites — explains the instance-vs-org scope mismatch, tabulates the exact permissions per script call (`action.target.read`, `action.target.write`, `action.execution.write`, plus optional `action.target.delete` only for the full-removal path), and lists three narrow-to-broad assignment paths (custom instance role → prebuilt action-scoped role → `IAM_OWNER` fallback). Includes duration guidance (needed only during register + rotate; can be assigned permanently / per-run / to a separate M2M key). _(Content subsequently relocated to `zitadel/actions/PERMISSIONS.md` — see follow-up below.)_
- [x] `DEPLOY.md` troubleshooting table: new row for `HTTP 403` pointing directly at the permissions section.
- [x] `.env.example`: note added to the `ZITADEL_MACHINE_KEY_PATH` block clarifying the ORG roles it recommends are insufficient for Actions v2 scripts, with a pointer to `DEPLOY.md § Service-account permissions`.
- [x] `zitadel/actions/register.sh` + `rotate.sh`: new `zitadel_api METHOD PATH [BODY]` helper replacing every `curl -fsS` call. On HTTP error prints method + path + status + Zitadel's JSON error body to stderr; 401/403 include an inline IAM_OWNER hint. Exit codes preserved.
- [x] Helper verified against httpbin `/status/200` and `/status/403`: 200 path silent on stderr, 403 path renders the full diagnostic.
- [x] `bash -n` clean on both scripts.
- [x] `openspec validate zitadel-actions-v2-deployment --strict` — passes.
- [x] `mcp__codebase-memory-mcp__index_repository` — refresh.
- [x] **Doc relocation (review follow-up):** moved the permission matrix out of the change-scoped `DEPLOY.md` into a durable `zitadel/actions/PERMISSIONS.md` so the `.env.example` + script + troubleshooting pointers survive OpenSpec archive. DEPLOY.md section reduced to a short pointer; README Contents table + troubleshooting row + `.env.example` note all repointed at the living-tree doc.
- [x] **Helper-hint alignment (review follow-up):** rewrote the 401/403 hint in both scripts' `zitadel_api` helpers to list the minimum permissions (`action.target.read` / `action.target.write` / `action.execution.write` / optional `action.target.delete`) and the three assignment paths narrow-first (custom role → prebuilt → IAM_OWNER), replacing the earlier "requires IAM_OWNER" line that would have led operators to over-grant.
