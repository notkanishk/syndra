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
- [x] 4.8 Drop `overrides`, `emitted_keys` and `conflicts` from `GET /projects/{id}/claim-shape` (the UI slice replaced the app's token screen)
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
- [x] 7.1 UI slice: the claims part per A-14 on this API (§8 below)
- [ ] 7.2 [ready-for-agent] Test people (`GET/PUT /applications/{id}/test-people`) and "What the app reads" (A-11 §3, §6)
- [ ] 7.3 [needs-info] Which Supabase provider the deployment uses (research unverified)
- [ ] 7.4 [needs-triage] Confirm on prod that Zitadel sends `application.client_id` in both functions (debug mode's first capture shows it); without it every app gets its project's default

## 8. The UI (A-14 §1–§8)
- [x] 8.1 The page: header (client id in Advanced), "What <app> is told" beside the sticky preview, Debug mode, History; on a phone the shape comes first. The old per-app override screen (`TokenFormatEditor`, `TokenPreview`) is gone
- [x] 8.2 Design-system parts in `components/ui`, registered in `.design-sync` (entry + componentSrcMap): `CheckTrail`, `TokenBlock` (+ raw mode, and `Ticks` for backticked words), `ClaimRow`, `RoleWordsTable`, `SignInRow`
- [x] 8.3 The preview: the five checks read off the explain step's facts (`components/apps/claim-words.ts`), check 5's token, a draft's − and + lines (marks plus screen-reader words, never colour alone), check 1's Zitadel console links from `/zitadel/health`'s domain
- [x] 8.4 Templates: the chooser on an app with no shape (Suggested preselected, the draft previews at once), the template sheet (sections, Changed since, Start again), Start from … makes a draft
- [x] 8.5 Copy another app's shape: the same chooser in the offer and in a sheet; what copies, "Not in this project, so not copied", and a copied if-role row with no role stops Save with its danger line
- [x] 8.6 Roles table with was-lines and shared-word notes; one ClaimRow form for every kind ending "Sent as …" (nested path, if_role through the role picker in single-project mode, bundles, role_ends, attributes behind More claims, typed fixed values); text formats only in Advanced or when already saved, with the warning; Lands in (Segmented in Advanced) and its check-1 line; Roles added later in Advanced
- [x] 8.7 Breaking saves: the before-line, the save summary (What changes, What <app> will do) and the I-understand tick that sends `acknowledge_breaking`; History with Go back (asks for the tick on a 409)
- [x] 8.8 Debug mode: one account at a time (you by default, or someone else by search), the 15-minute end shown and enforced by the server, the live list whose new rows wait while the pointer, focus or an opened sign-in is in it, one sign-in opened in place (request, answer, checks, labelled rebuild, no signatures), the ended state
- [x] 8.9 Apps index: the Claims column (real key, where it lands, amber "Doesn't reach it"), the sub-line naming the template or default, one warn line above the table. Backend: `GET /applications` gains `claims` per app (`attachClaimLines`); its verdict is check 1's last answer, never a fresh Zitadel call per app
- [x] 8.10 The project default moves to the project's page (`ProjectClaimDefault`), with the apps it reaches
- [ ] 8.11 [ready-for-agent] Test people in the debug sheet and the preview (needs 7.2); "Last on 2 Nov, for 15 minutes, by …" on the off row (needs the last session kept past its hour)

## 9. The page, calm by default (A-15, 3 Nov; Q-15 after the owner saw A-14 live)
Layout only: every A-14 behaviour and the API stand (no API change).
- [x] 9.1 The header has no lede (the summary does its job; argued in the copy guard's `LEDELESS`); the preview has no lede. The eight sentences A-15 §1 cuts are gone, `TEMPLATE_CAPTION.custom` with them
- [x] 9.2 The summary under the card title, both views: one line per top-level claim in the token's order, key in mono beside its words (`summaryLines` in `lib/claims.ts`: renamed, left-out, objects, fixed, if-role, attributes; a text-format roles row keeps its warning here); then "In {lands}. {template line}" with See the template / Copy another app's shape. When the suggested template isn't the one the app is on, the button reads "See the {suggested} template". Check 1 failing on reach says so in danger text under it. A new app's summary replaces its lede and "Sends now", with "{project}'s default →"
- [x] 9.3 RoleWordsTable: quiet word fields (transparent at rest, line on hover, ring on focus), `changed` keeps the border while a row differs from what's saved; rows `py-2`; "N of M included" only
- [x] 9.4 Claim rows (Advanced only, label in one constant `CLAIM_ROWS`, owner may rename to 4b): a 28px chevron fold, closed on every arrival, note "{n} rows · keys · {lands}" (`foldNote`), holding every ClaimRow form, Add a claim and Lands in. It opens itself while a row stops Save (a local problem or 400 `details.row`). Basic renders no claim row, Add a claim, Lands in or Roles added later; there a row that stops Save is named in the before-line, "Fix `x` in Advanced to save."
- [x] 9.5 Save unfolds with the first edit (`Unfold open={draft.committed}`), Discard folds it away; "Nothing to save yet." is gone
- [x] 9.6 The preview: "Preview for [person]" title line with Checked and Check again at its end (you first in the picker); one verdict line (`verdictOf`, crossfading in its seat) with Show/Hide the checks; the trail unfolds on demand (whole) or by itself on a fail or warn (compact); the token sits under the trail, with one faint line (Zitadel adds…, the draft's one difference, or what the app misses on failure) and Copy preview in a right seat. Check 5 reads "Syndra answers with `…`, below." (", but only for userinfo." on failure; "would answer" on a draft)
- [x] 9.7 One tools card: Sign-ins (30 bars, 178×20, the count), Debug mode (StatusSeat "Off" and its note; on and ended as built, live list inside the card), History (the version in use in one line; Show all N unfolds the table from two versions)
- [x] 9.8 Tests: Basic has no claim rows; the fold is closed in Advanced and opens itself on a blocked row; checks collapse when all pass, open compact on a failure; Save only after the first edit; history shows one version until there are two; summary words, fold note and verdict phrases (`lib/__tests__/claims.test.ts`)
- [ ] 9.9 [needs-info] Loading draws the summary's seat as two Skeleton lines: the page still loads as one card skeleton, since the summary has nothing to stand on before the shape is read
- [ ] 9.10 [needs-triage] The owner's call on the fold's name: 4a "Claim rows" is built; 4b "Custom claims" is one constant away
