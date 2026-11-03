# Tasks: claims shaping (A-14)

Contract: `design.md`. Tags per `docs/agents/triage-labels.md`.

## 1. Shaper (`internal/claims`)
- [x] 1.1 The per-app effective profile: the app's own, then the project default, then `Builtin()`; `Shape(profile, facts)` takes exactly one profile
- [x] 1.2 `RoleMap{key, as, included}` and `include_new`: filter, then rename, then dedupe, in a stable order
- [x] 1.3 Ordered claim rows, kinds `roles | attribute | static | if_role | bundles | role_ends`; the token keeps row order (`Object`)
- [x] 1.4 Nested `path`: shared parents merge; a repeated path or a leaf-that-is-a-parent is refused; a dot stays one key
- [x] 1.5 Roles rows always sent (`[]`), nested included; `if_role` always `true`/`false`
- [x] 1.6 `Lands` (`both | access_token | userinfo`) and `LandsIn(function)`
- [x] 1.7 `Diff` and `ShapeOnly`; `Breaking(template, changes)` per template
- [x] 1.8 `templates.go`: NetBird, GitLab, Supabase (custom OIDC, Keycloak), Home Assistant, Sign-in only, Custom, per the research; `ActionsV1()` for the seed; tests pin every template's token, lands and rule

## 2. Facts and the cache compiler
- [x] 2.1 Facts gain `given_name`, `family_name`, `username`, `bundles`, `role_ends` and `is_service_account`; `Attributes()` lists them
- [x] 2.2 The compiler persists them: Zitadel's first and last name, bundle names per project, an end only when every source of the role ends, and the service-account rule (a failed read of the marks keeps the old entry on warm)

## 3. Data plane
- [x] 3.1 `application.client_id` maps to the app's shape through the project's Redis read-through (`claim_shapes:<project>`); absent or unknown, the project default, logged once per client
- [x] 3.2 Lands is checked against `req.Function` before any facts are read
- [x] 3.3 The contract tests cover per-app shaping, lands gating and debug capture (`action_v2_contract_test.go`)

## 4. Storage and API
- [x] 4.1 Migration 000071: `app_claim_shape_versions` (immutable, with the template, copied_from and restored_from) and the seed cutoff
- [x] 4.2 GET/PUT `…/claim-shape`; PUT needs `acknowledge_breaking` for a change the app acts on (409 with breaking and reach)
- [x] 4.3 `…/claim-shape/preview`: the explain step (five checks) for now and a draft, through the same `Shape`
- [x] 4.4 `…/claim-shape/draft-from` from a template or any app; `not_copied` and `missing_roles`
- [x] 4.5 `…/versions` and `…/versions/{n}/restore`
- [x] 4.6 Zitadel `TokenSettings` (accessTokenType, idTokenUserinfoAssertion, projectRoleAssertion, projectRoleCheck): read-only, faked in tests, with the last answer kept for a degraded check 1
- [x] 4.7 The project default stays at `/projects/{id}/claim-profile`; the per-app override endpoints are removed
- [ ] 4.8 [ready-for-agent] Drop `overrides`, `emitted_keys` and `conflicts` from `GET /projects/{id}/claim-shape` once the UI slice replaces the app's token screen
- [ ] 4.9 [ready-for-human] Drop `app_claim_overrides` once every environment has been seeded (check that `app_claim_shape_versions` has a version 1 for each app)
- [ ] 4.10 [ready-for-human] Run `TestClaimShapeVersions_Live` against a `_test` database (it is skipped without `SYNDRA_TEST_DATABASE_URL`)

## 5. The seed
- [x] 5.1 Lazy, because apps live in Zitadel: at start, hourly, and on reading an app. It covers apps Zitadel created before the cutoff, with no shape yet, and gives them a Custom shape reproducing Actions v1, as version 1 "As Actions v1 sent it"
- [x] 5.2 An old `app_claim_overrides` row is folded into that app's seed
- [ ] 5.3 [ready-for-human] Before turning Actions v1 off, check that it and the v2 seed don't both write `groups` and `custom_claims` into one token in a way Zitadel refuses (both run until v1 is removed)

## 6. Debug mode
- [x] 6.1 `internal/claimdebug`: one account per app, 15 minutes enforced server-side, a ring of 200 per app kept 1 hour, in memory; filtered before anything is held; a non-blocking capture after the response
- [x] 6.2 PUT (on, or switch keeping the end), DELETE and GET `…/debug`; GET `…/debug/sign-ins?after=`
- [x] 6.3 Audit: started, switched, ended (by hand or "system")
- [ ] 6.4 [needs-triage] More than one API replica splits captures: pin debug mode to one replica, or move it to Redis, if the API is scaled out

## 7. Not in this slice
- [ ] 7.1 [ready-for-agent] UI slice: the claims part per A-14 on this API (ClaimRow, RoleWordsTable, SignInRow, CheckTrail, TokenBlock)
- [ ] 7.2 [ready-for-agent] Test people (`GET/PUT /applications/{id}/test-people`) and "What the app reads" (A-11 §3, §6)
- [ ] 7.3 [needs-info] Which Supabase provider the deployment uses (research unverified)
- [ ] 7.4 [needs-triage] Confirm on prod that Zitadel sends `application.client_id` in both functions (debug mode's first capture shows it); without it every app gets its project's default
