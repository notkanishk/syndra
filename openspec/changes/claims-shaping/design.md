# Claims shaping: design and API contract

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-14-claims-shaping-rescoped.md` ("For code"), with the owner's rulings of 3 Nov. Templates follow `docs/research/2026-11-03-app-claims.md`. Decision: ADR-003 (one shape per app, picked by `application.client_id`); ADR-002 (one project per token) is unchanged.

## The model

**The shaper is `internal/claims`, and nothing else shapes.** The data plane, the preview, the draft preview, the debug-mode checks and `/simulate` all call `claims.Shape(profile, facts)` with one profile.

```
Profile { claims: [Claim], roles: [RoleMap], include_new: bool, lands: both|access_token|userinfo }
Claim   { path: [string], kind, source?, value?, roles?, format? }
RoleMap { key, as?, included }
```

- **Which profile.** The app's own latest version. With none, its project's default (stored flat in `claim_profiles`, edited on the project's page only), else `claims.Builtin()`: `roles`, every role in Syndra's words, a list, in both places. Never nothing.
- **Kinds.** `roles` (the roles this app sees, as `array`, `csv` or `space_delimited`; templates pin `array`), `attribute` (`given_name`, `family_name`, `name`, `username`, `email`, `is_service_account`, `title`, `team`, `role_count`, `compiled_at`, `user_id`, `project_id`), `static` (typed JSON: text, number, yes/no or list), `if_role` (`true` if they hold any of `roles`, else `false`), `bundles` (bundle names), `role_ends` (`{word: "YYYY-MM-DD"}`, roles with no end left out).
- **Roles.** First filter, then rename, then dedupe. Mapped roles come in the map's order, then roles added later, which are kept only when `include_new` is set. Every roles row is always sent, `[]` when nothing survives, nested rows included. `if_role` reads the real holding, before any filter.
- **Nesting.** `path` builds objects. Rows under one parent merge into it. A dot inside a segment stays a dot, so `syndra.laser.roles` is one key. Validation refuses a repeated path, a leaf that is another row's parent ("`custom_claims` is already an object here, holding `groups`.") and a top-level key Zitadel never lets an action set (`sub`, `aud`, `iss`, `exp`, `iat`, `nbf`, `jti`, `client_id`, `urn:zitadel:iam*`; claims-delivery-integrity Z-g). The token is written in the editor's order (A-16): the roles claim's key first, then A to Z, and A to Z inside an object, so the list and the token read across. Apps don't read key order.
- **Lands.** `access_token` answers only `function/preaccesstoken`, and `userinfo` answers only `function/preuserinfo` (the ID token gets it when "User Info inside ID Token" is on). An unnamed function is answered.
- **Diff and Breaking.** `Diff(old, new)` gives `add | remove | rename | retype | move | lands | include_new`. A role-level change carries `role` plus `from`/`to` words. A claim-level change carries `path` (and `new_path` for a move). `ShapeOnly` drops the role-level ones; that gives "Changed since the template". `Breaking(template, changes)`:
  - NetBird, GitLab and both Supabase templates: any role renamed or left out, plus any claim removed, moved, retyped or landing elsewhere.
  - Home Assistant: the words `admins` and `users` only, plus the claim itself.
  - Custom and Sign-in only: never breaking.
- **Whose rule.** The breaking rule is the app's, not the draft's. It comes from the first app-specific template among the new shape's, the saved shape's, and the one the app's name suggests. So going back to a Custom version on NetBird still warns. An app with no shape of its own is on a default that was never made for it, so its first save is never breaking.
- **Templates** (`claims.Templates()`, in chooser order). All are version 1:

| id | sends | lands | reads | roles added later |
|---|---|---|---|---|
| `netbird` | `groups` | both | access token | left out |
| `gitlab` | `groups` | userinfo (ID token + userinfo) | userinfo | included |
| `supabase-oidc` | `custom_claims.groups` | userinfo | ID token | included |
| `supabase-keycloak` | `groups` | userinfo | userinfo | included |
| `home-assistant` | `groups` | userinfo | ID token | included |
| `sign-in-only` | nothing | both | — | — |
| `custom` | `roles` | both | — | included |

`reads` feeds check 1. The research didn't say how GitLab, Supabase and Home Assistant treat roles added later; those defaults are this slice's choice (below).

## The data plane

`POST /api/action/inject` reads `application.client_id` (Zitadel v4+, already parsed). One Redis read-through per project (`claim_shapes:<project>`, 5 min TTL, dropped on every save) holds `{default, default_source, by_client: {client_id: {application_id, version, template_id, profile}}}`. Each saved version carries its app's client id, so a token never waits on Zitadel to find its app.

- Client id absent (Zitadel before v4) or unknown, or the app has no shape of its own: the project's default. This is logged once per (project, client).
- Lands is checked before any facts are read: the other function gets `append_claims: []`.
- The one-project rule (ADR-002) and the degraded paths are unchanged. A failed shape read serves `Builtin()` uncached.
- Debug mode is asked once, after the answer is written.

## Storage

Migration `000071_app_claim_shapes`:
- `app_claim_shape_versions` (application_id, version) holds the immutable versions: `project_id`, `client_id`, `profile` (JSONB), `template_id`, `template_version`, `copied_from_application_id`, `copied_from_version`, `restored_from`, `note`, `saved_by`, `saved_at`. The latest version applies. "Go back" saves the old profile as the next version, with `restored_from`.
- `app_claim_shape_seed` holds one row, `cutoff`, set to the migration's time.

**The seed is lazy, because apps live in Zitadel, not in the database.** `services.SeedClaimShapes` runs at start and every hour, and `GET …/claim-shape` runs it for its own app. It gives every app Zitadel created before the cutoff, with no shape yet, a Custom shape that reproduces Actions v1 exactly, in both places, as version 1 "As Actions v1 sent it":
- `groups`: every role key, flat;
- `custom_claims: { groups, given_name, family_name, name }`.

An app with no creation date counts as existing. An old `app_claim_overrides` row rides along in that app's seed if its keys fit; its note then reads "As Actions v1 sent it, with its own claims from before". Apps made after the cutoff start on their project's default. The seed is idempotent: it writes only into an app with no versions.

**Facts** gain `given_name`, `family_name` and `username` from Zitadel's profile, `bundles` (the names of bundles giving a role in this project), `role_ends` (only when every source of a role ends; a rule-derived role ends with its source, one-path W2a) and `is_service_account` (a machine user or an operator's mark). The cache compiler persists them.

## API

Every route is operator-only. Mutations decode strictly.

### Templates and attributes
- `GET /api/v1/claim-templates` returns `{templates: [{id, version, name, description?, profile, reads?, breaking_rule?}]}`.
- `GET /api/v1/claim-attributes` returns `{attributes: [...], formats: [...]}`.

### The app's shape
`GET /api/v1/applications/{id}/claim-shape` returns:
```
{ application, client_id,
  source: "app" | "project_default" | "builtin",
  profile,                       // what applies now
  version: ClaimShapeVersion | null,
  project_default, project_default_source,
  project_roles: [{key, label, group?, description}],
  template: {id, version, name, changes: [Change], updated: bool} | null,
  suggested_template?: "gitlab" }
```
Reading an app that qualifies for the seed seeds it first.

`PUT /api/v1/applications/{id}/claim-shape` saves:
```
{ profile, template_id?, template_version?, copied_from?: {application_id, version},
  note?, acknowledge_breaking?: bool }
```
- On success: `200 {version: ClaimShapeVersion, changes: [Change], breaking: [Change]}`, the cache is dropped, and the audit records `claim_shape.saved`.
- A change the app will act on, sent without `acknowledge_breaking: true`: `409 {error: "BREAKING_UNACKNOWLEDGED", breaking, reach: {by_role: {key: n}, people} | null}`. The counts are observed from Zitadel; they are null when Zitadel has never been read.
- A shape that can't be saved: `400 VALIDATION_FAILED` with `details.row`, the claim row's index, or `-1` for the shape as a whole. This covers a role key that isn't in the app's project, including an `if_role` row whose role didn't copy.
- Lost a race with another save: `409 CONFLICT`. An unknown app: `404`.

### Preview: the explain step
`POST /api/v1/applications/{id}/claim-shape/preview` takes `{user_ids: [1..6], draft?: Profile, template_id?}` and returns:
```
{ people: [{user_id, now: Explanation, draft?: Explanation}],
  changes, breaking, reach?, left_out: [row index] }
```
`left_out` lists the rows the draft preview had to skip: copied `if_role` rows that have no role yet.

`live` (5 Nov) maps a person to `{at, function, served, expected, match}`: their newest sign-in debug mode captured for the app, against `now` (never a draft). Absent for someone never captured. A person Zitadel lists no project for gets `token: {}` and check 5 `none` with `unlisted: true`, as the door answers `no_grants`. Check 4 carries `client_id_missing: [function]` (amber) for an app with its own shape when the newest call at a step it lands in named no app; check 6 carries the instance-wide list.

An `Explanation` is `{user_id, checks: [5 × {state, facts}], token, would_receive?, read_at}`. The states are `pass | none | warn | fail | stale`:
1. **Zitadel's settings for the app.** Facts: `project_role_assertion`, `project_role_check`, `access_token_type` (`JWT`/`BEARER`), `id_token_userinfo_assertion`, `lands`, `reads`, and `reached{access_token, id_token, userinfo}`.
   - `fail` when the project's roles aren't asserted, or when the template's `reads` isn't reached.
   - On Sign-in only: `warn` while the role check is off.
   - `stale`, with `read_at`, when the facts are Zitadel's last answer.
   - `warn` with `unread` when Zitadel has never answered.
   - The facts come from two read-only GETs (`TokenSettings`), with the last answer kept in memory.
2. **Roles held** (`roles`). `none` when there are none.
3. **Claims built** (`compiled_at`, from the stored cache entry the token path serves).
   - `warn` when nothing is stored yet; `none` when there's nothing to build; `fail` on a read error.
4. **Which shape applies** (`source`, `version`, `template_id`, `draft`, `keys`). `warn` with `suggested_template` when the app is on a default that doesn't send what its name's template sends.
5. **The token** (`lands`).
   - `none` for Sign-in only.
   - `fail` when check 3 isn't `pass`. Then `token` is what the token path really sends (degraded: `{}`, or the minimal-safe set) and `would_receive` is the shaped token.
   - `fail` when check 1 fails.

### Drafts
`POST /api/v1/applications/{id}/claim-shape/draft-from` takes `{template_id}` or `{application_id}`, exactly one. It returns:
```
{ profile, template_id, template_version, copied_from?, not_copied: [{role_key, name}],
  missing_roles: [{row, role_key, name}] }
```
Nothing is written.
- **From a template:** the template's rows, lands and roles added later. Every project role is listed with the inclusion and word the app has now, because role words are always the operator's.
- **From another app** (any project, and it must have a shape of its own): everything copies. Role words and inclusions copy only for keys this project has; the rest are listed in `not_copied`. An `if_role` row loses the roles that aren't here, and `missing_roles` flags it, which stops the save. Saving with `copied_from` records it.

### History
- `GET /api/v1/applications/{id}/claim-shape/versions` returns `{versions: [ClaimShapeVersion + {changes}]}`, newest first. `changes` is the diff from the version before; version 1's is empty.
- `POST /api/v1/applications/{id}/claim-shape/versions/{n}/restore` takes `{acknowledge_breaking?}` and saves version n as new, through the same checks. The audit records `claim_shape.restored`.

### The project default
- `GET /api/v1/projects/{id}/claim-shape` returns `{project_id, project_name, configured, default: {claim_name, format_type, attribute_claims, static_claims}, profile, applications: [{application_id, name, source, version?, template_id?}]}`.
- `PUT /api/v1/projects/{id}/claim-profile` takes the flat `default` and is its one write place.
- The per-app override endpoints (`PUT|DELETE /applications/{id}/claim-profile`) are gone.

### The apps index
`GET /api/v1/applications` gives each app `claims: {source, keys, lands, template_id?, template_changed?, copied_from_application_id?, reaches?}`: its own shape's top-level keys, read once per project. `reaches` is whether the claims reach where the template says the app reads, from check 1's last answer in memory; absent when Zitadel's settings for that app were never read here. Shapes that can't be read leave `claims` out.

### Debug mode
- `PUT /api/v1/applications/{id}/debug` takes `{user_id?}` (the caller's own by default, never everyone). It returns `{ends_at, switched, session}`.
  - While it's on, the same PUT moves the capture to another account and keeps `ends_at`.
  - It is refused for an app with no OIDC client, and for a person Zitadel doesn't know.
- `DELETE …/debug` turns it off and returns `{on, session}`.
- `GET …/debug` returns `{on, session: {application_id, user_id, started_by, started_at, ends_at, ended_at?, ended_by?} | null}`. `ended_by` is `"system"` when it ended by itself.
- `GET …/debug/sign-ins?after=<seq>` returns `{on, session, sign_ins: [{seq, at, function, client_id, user_id, grants, request, response, duration_ms, checks}]}`, newest first. Poll it with the newest `seq`.
- **Audit:** `claim_debug.started` (actor; the target is the captured account; the end is `started + 15 min`), `claim_debug.switched`, and `claim_debug.ended` (actor `system` when it ended by itself). Captured sign-ins aren't audited.
- **Server-side:** the 15 minutes is enforced in `claimdebug.Recorder.Wants`, and a timer records the end. Filtering on (client id, user id) happens before anything is held. Capture is a non-blocking send after the response is written; the checks are computed off the latency path. The ring holds 200 per app for 1 hour, in memory only. Request headers (where `ZITADEL-Signature` travels) are never captured; the body is kept, emails included.
- **One replica:** sessions and captures live in one process. If the API ever runs more than one replica, pin debug mode to one or move it to Redis.

## Choices this slice made where the answer was silent
- Roles added later: NetBird leaves them out (research). GitLab, both Supabase templates and Home Assistant include them, because an extra group is harmless to their exact-match lists.
- The breaking rule follows the app, not the template the save names, and a first shape on the default is never breaking.
- Claim-level removals, moves, retypes and lands changes break for every template that names an app, Home Assistant included.
- `include_new` changes are a Diff kind of their own (`include_new`). The answer listed six kinds; this is the seventh, for "roles added later".
- `bundles` lists only bundles that give a role in this project, so a token doesn't reveal membership elsewhere.
- A role a rule gives has no end, even when its source role ends.
- Check 1 fails when the project's roles aren't asserted. Syndra can't see whether an app asks for role scopes, so the UI's words offer that way out.
- The project default stays flat (one roles key plus attribute and static maps). A richer project default would need its own storage; it waits until the project page asks for one.

## Not built here
- Test people (`GET/PUT /applications/{id}/test-people`) and "What the app reads" (A-11 §3, §6). Both are A-11 carry-overs, outside this slice's list.
- The last refused sign-in per client (A-11 check 1's "Last refused at 10:12"). The answer says to drop that line rather than invent it.
- "NetBird clients is told something different" (A-14 §8), the cross-app comparison in check 4.
- Console deep links: the UI builds them from `/zitadel/health`'s domain and the project and app ids.
- Which Supabase provider the deployment uses. An operator has to confirm it.
