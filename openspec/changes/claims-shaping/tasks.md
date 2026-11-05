# Tasks: claims shaping (A-14)

Contract: `design.md`. Tags per `docs/agents/triage-labels.md`.

## 1. Shaper (`internal/claims`)
- [x] 1.1 The per-app effective profile: the app's own, then the project default, then `Builtin()`; `Shape(profile, facts)` takes exactly one profile
- [x] 1.2 `RoleMap{key, as, included}` and `include_new`: filter, then rename, then dedupe, in a stable order
- [x] 1.3 Ordered claim rows, kinds `roles | attribute | static | if_role | bundles | role_ends`; the token is written in the editor's order (A-16): the roles claim's key first, then A to Z, A to Z inside an object (`ordered`, `RolesKey`, `Object`)
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
  - Checked 5 Nov 2026, NOT done: nothing writes the table, but `ListAppClaimOverrides` (db/claim_profiles.go) is read by `seedApp` (services/claim_shape.go), which runs at start and hourly. Remove that reader, then drop with a guard refusing while an override's app has no shape version. See NEXT.md 0c.
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
- [x] 7.2a Test people, rescoped (owner, 5 Nov: "pick any person and see exactly what their token would carry"): the preview's person picker already does it (`ClaimPreview`, A to Z, service accounts apart) through the one shaper, so no saved list and no `test-people` endpoints. What was missing was parity at the door: the preview shaped a token for someone Zitadel lists no project for, where the door answers `no_grants` and nothing. `explain` now applies the same gate (`zitadelLists`: an observed grant in the project, roles or none) and check 5 says so (`unlisted`). The property test now runs the door (`Door.Answer` on a decoded payload, 1,600 cases, ~270 with no project listed), not `Plane.Answer`
- [ ] 7.2b [needs-triage] "What the app reads" (A-11 §6), and a saved test-people list only if the picker proves short
- [ ] 7.3 [needs-info] Which Supabase provider the deployment uses (research unverified)
- [x] 7.4a The page says it when Zitadel doesn't send `application.client_id`: the newest call per step across every app (`claim_delivery_last`, `ClientID == "-"`, within a day) is the instance's answer, since Zitadel's version decides it (`clientIDMissing`). An app with its own shape whose steps are affected gets an amber check 4: "Zitadel doesn't say which app is signing in, so {app} gets {project}'s default, not this shape", and the fix (Zitadel 4.0 or later)
- [ ] 7.4b [ready-for-human] Confirm on prod: the code path is in place, but whether prod sends the field is UNVERIFIED here (no prod read). Open any app's claims page after a sign-in: check 4 turns amber if it doesn't
- [x] 7.5 The live check (owner, 5 Nov): the preview response carries `live`, per person, their newest sign-in debug mode captured for the app against the saved shape's preview (`services.CompareServed`: the same `Envelope`, nothing where the shape doesn't land at that step, values compared as JSON, the claims' order kept). The preview says "carried exactly this", or "something different" with what was sent; with nothing seen, how to see one. Debug captures only: the correlator's relayed calls (`claims:calls:*`) carry outcomes, not payloads
- [x] 7.6 The project-not-listed notice (`listingNotice`), under the header: the project's roles aren't asserted and no call was served in a day (so the app asks for no role scopes either), or the newest call was refused as cross-project (ADR-002: the extra `…:aud` scope)

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
- [ ] 8.11 [needs-triage] "Last on 2 Nov, for 15 minutes, by …" on the off row (needs the last session kept past its hour). Test people in the preview: done by 7.2a; the debug sheet's "Someone else" already picks any person

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
- [x] 9.10 The fold's name: moot. A-16 removes the Claim rows fold; each line opens its own editor

## 10. The claims editor, simple for the common case (A-16, 4 Nov; Q-16 after the owner used A-15 on prod)
The owner accepted every recommendation. Every A-14/A-15 behaviour and the API stand; the shaper's key order and one validation are the only backend changes.
- [x] 10.1 One list of claims under the card title (`claimLines` in `lib/claims.ts`): key in mono (132px seat), its words, one faint line. The roles claim's line first (`rolesIndex`, the shaper's `RolesKey`: first top-level roles row, else the first nested one), the rest A to Z (`compareKeys`, byte for byte the shaper's `keyLess`), an object's keys A to Z. A top-level row is its own line, the rows inside one object one line. "New in this draft." on a line the saved shape lacks; "Its `groups` is the same list of roles." on an object holding a second roles row; a text-format roles claim keeps its warning on its line. With no roles claim the first line is "No roles. {app} gets none from Syndra.". The summary block, the Claim rows fold, `CLAIM_ROWS`, `foldNote`, `summaryLines`, `rolesWords` and `TEMPLATE_CAPTION` are gone, and `ClaimRow` with them
- [x] 10.2 Each line is a button with `aria-expanded` and the grouped list's 28px chevron in its 44px seat; one line open at a time, closed on arrival, opening by Unfold. Basic opens only the roles line; the others keep their seat. The order is settled while a line is open (a name being typed keeps its field) and settles when it closes. A line holding the row that stops Save opens by itself once, in Advanced
- [x] 10.3 The roles line: faint state (`rolesState`: All N / Both / Its one role, in Syndra's or the app's words; up to two changes named, A to Z by name; else counts). Opened: Claim name (Advanced, mono dense, the template's "Set in" line via `NAME_SET_IN`), Sent as (Advanced, `SENT_AS_WORDS` + Segmented, the text warning), `RoleWordsTable dense`, Roles added later (Advanced), Remove {key} (danger sm). Basic: the table only (owner Q5: Basic can't rename the roles claim). Folded on every arrival (Q7)
- [x] 10.4 A claim's editor (Advanced): Claim name, Carries (dense Select), its kind's row (If they hold: Chips + Change roles outline sm; Value: type + dense input; Sent as), Inside (Nothing: top level · each object · A new object), the faint literal, the error line, Remove (danger sm). An object's editor: its name, one compact row per key A to Z (name, Carries, Remove danger sm), the object's literal, Add a claim inside (outline sm) and Remove {object} (danger sm, "…and the N claims inside it")
- [x] 10.5 Add a claim (Q1, option a): outline sm opening an `AnchoredPanel` listbox (QuestionSeat's option rows), a Modal sm sheet on a phone. `ADD_MENU` is the one list. A finished entry appends a working row, opens no editor and arrives as a FormPart; "Yes if they hold a role" opens the role picker first, then lands `is_<role>` opened; "A fixed value" lands opened, its name focused, and Save stays off until it has a name and a value ("Name it and give it a value." / "Give it a value."). The roles entry is `groups` by default (Q6), muted "Sent as `…`" while a top-level roles claim exists
- [x] 10.6 Zitadel's keys (Q4 and "For code" 6, decided from the Z-g source reading in `claims-delivery-integrity/assessment.md`): the menu greys `email`, `name` and `preferred_username` at the top level ("Zitadel sends `email`"; choosable inside an object). `given_name`/`family_name` stay addable: with the `profile` scope Zitadel keeps its own in userinfo and the ID token, but the access token carries an appended one, and Supabase reads them inside `custom_claims`. A top-level key Zitadel never lets an action set (`sub`, `aud`, `iss`, `exp`, `iat`, `nbf`, `jti`, `client_id`, `urn:zitadel:iam*`) is refused by `Validate` (`ZitadelOwns`) and by `rowProblems`, naming the row
- [x] 10.7 The token in the list's order (Q2): `claims.Shape` sorts a copy of the rows (`ordered`), so the Actions v2 answer and every preview share it; `CanaryExpected` follows
- [x] 10.8 Controls (Q3; `one-control-surface.test.ts` "keeps ghost for the quieter half of a pair", pinned on `components/apps/`): See the template, Copy another app's shape, Show the checks, Show all N, See the whole template, Change roles and Add a claim are outline sm; Remove is danger sm; Discard the draft stays ghost beside Save. `Input`/`Select` gain `dense` (`DENSE_FIELD`: 34px with a mouse, the 44px floor below desktop); `RoleWordsTable` gains `dense` (one line a role, 190px name seat, 70px Included seat). Lands in is a settings row after Add a claim, its ID token option's `title` carrying A-15's sentence (`ID_TOKEN_TITLE`); the template line and its two buttons sit at the card's foot
- [x] 10.9 Tests: roles first and A to Z (UI lines, `compareKeys`, the shaper's `TestShape_Order`); the roles state phrasings; Basic opens only the roles, with no name, Sent as, Add a claim or Lands in; one line open at a time; the menu adds a finished row with no editor, greys Zitadel's keys and the taken roles claim, opens the picker first for if-role, and lands a fixed value opened and blocking Save; the reserved keys in `Validate` and `rowProblems`; the controls' parts
- [ ] 10.10 [ready-for-agent] Q3's sweep across the rest of Syndra: the lone ghost buttons outside `components/apps/` (insights, storage, review, system, settings, targets, the bundles RolePicker); widen `SWEPT` as each area follows. PaginationRow keeps its own controls
- [ ] 10.11 [ready-for-human] Resync and publish the design-system bundle (`ui/.design-sync`): `RoleWordsTable dense`, `Input`/`Select dense`, `ClaimRow` removed
- [ ] 10.12 [needs-triage] Motion left plain: a removed claim leaves at once rather than as a LeavingRow (A-16 §6); arrival is a FormPart
