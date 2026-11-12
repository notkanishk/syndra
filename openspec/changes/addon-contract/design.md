# Design: the add-on contract, declared once

Evidence: the wire inventory of 10 Nov 2026 ([research/contract-1-inventory.md](research/contract-1-inventory.md)), `addons/README.md`
and `addons/contract/README.md`, `addons/truenas/{capabilities,plan,apply}.go`,
`backend/internal/addons/*`, `services/entitlement.go`, `addon-platform`
§4–§18 and tasks §37, `addon-transport-derived-keys`, and the two target
studies: [UniFi Access](research/unifi-access-api.md), [Trotec Ruby](../../../docs/research/2026-11-05-trotec-ruby.md).

Not re-litigated: the two planes (entitlement = level-triggered desired state,
operation = one-shot event); the manifest is a ceiling and the backend's policy
is the grant; plan-then-apply with the fingerprint re-verified at the add-on
immediately before the write; read-back observed state as the merge base;
withdraw-only rows are the switch alone and never create; the derived-key
transport and its vector; strict request decoding, tolerant reply decoding;
no queue in the add-on; the backend decides, the add-on translates.

## 0. The shape in one paragraph

The contract is a set of files in `addons/contract/`, not a type in anybody's
compiler: one JSON Schema per message, an OpenAPI document naming the routes, a
registry of error codes and one of operation ids, a golden fixture per message,
and the transport vector that already exists. The two Go ends keep their own
hand-written types and prove, from their own test suites, that what they send
validates and what they read is what the fixture carries. The manifest grows
from "what the fields are called" to "what the fields *are*" (a `role`), what
the add-on can do beyond the core (`features`), and what kind of system it
fronts (`kind`), so the backend and the UI derive behaviour from declared
meaning and a new kind of target is a new profile beside the core, not a change
inside it. Statistics travel the same way: the manifest declares metric
descriptors, Syndra pulls bucketed points on a schedule and keeps them, and
per-person records (what I did, what I hold) are read live and never copied
(§9). A conformance binary drives any add-on black-box and says whether it may
be connected. An author's guide is written for an agent with no other context.

**The boundary rule, stated once and MUST everywhere** (owner, 10 Nov 2026:
"it's the add-on's job always to ever interact with the external app or
service, never Syndra directly"). Syndra never calls a target. The add-on is
the only component that holds the target's address and credentials and the
only one that talks to it, on its own schedule, within the target's rate
limits and availability. Syndra talks only to add-ons, and only to ask:
state, a plan, an apply, a read, a collection of what the add-on has
gathered. Today's code already holds this (the backend has no target URL,
credential or client; only the add-on container holds `TRUENAS_URL` and
`TRUENAS_API_KEY`); the contract now says it, the guide's §1 opens with it,
and a backend guard test *guards* it (§11: an allowlist of the HTTP
clients `backend/internal` may construct, each with its reason; a source
scan cannot see where an operator-set URL points, so "guarded", not
"proven"), because the kit, standing outside Syndra, cannot see it at all. Every design choice below that
could have put Syndra in contact with an app (push statistics, a forced
re-read, a webhook) is resolved on this rule.

## 1. Source of truth: schema files, two validated ends, no shared module

**Decision.** `addons/contract/` is the contract:

```
addons/contract/
  openapi.yaml                 every route, method, status, body ref, feature gate
  schema/
    manifest.json  health.json  subjects.json  values.json  error.json
    plan_request.json  plan_response.json
    apply_request.json apply_response.json
    operation_request.json operation_response.json
    lifecycle_request.json lifecycle_response.json
    settings.json  setting_put_request.json  setting.json
    stats.json  call.json                      (features stats, calls; slice 6)
    profiles/storage/reach.json  profiles/storage/homes.json
    common/field.json  common/standing.json  common/effect.json  common/outcome.json
    common/stale.json  common/records.json  common/activity.json  common/status.json
  registry/
    errors.json        code → http status, retry class, meaning
    operations.json    id → meaning, params schema, scope ceiling, secret params, effect, result
    features.json      feature → what it adds (routes, fields), deprecated → replacement
    roles.json         the field-role vocabulary and what each implies
    kinds.json         the closed kind list, each with its credential-scope denylist
    units.json         the closed unit list and how each is formatted
    constants.json     contract and Syndra constants an add-on never declares: max_reply_bytes (1 MiB),
                       call memory (≥ P30D), in_doubt bound (PT1H), series cap (500), cooldown default (PT30S)
  fixtures/            one golden document per message (the four today move here)
  transport_derivation.json
  README.md            what each artifact pins and why
```

Each end asserts, in its own suite, for every message it **sends**: marshal
the Go value built from the fixture, normalise, byte-equal the fixture, and
validate against the schema. For every message it **reads**: decode the
fixture and check every field it uses landed. The schema validation uses
`github.com/santhosh-tekuri/jsonschema/v6` in test scope on both sides (one
dependency; the only JSON Schema 2020-12 implementation in Go that is
complete and maintained).

**One directory per major while two coexist** (decided 10 Nov 2026, building
slice 1a). From slice 1a until slice 1 ships, `addons/contract/v1/` holds the
pinned contract 1 (schema, fixtures, registry, openapi; slice 0's files, which
today's code is still tested against) and `addons/contract/v2/` holds major 2.
Files that do not change with the major (`transport_derivation.json`, the
cross-binary vectors) stay at the top level; the raw-query vectors 1a adds are
tagged with the major that introduces them. When slice 1 ships, `v1/` is
deleted in the same PR (decision 2: one supported major) and `v2/` is the
contract until a major 3 adds `v3/` beside it the same way. Versioned
directories, as protobuf packages and versioned API specs do, rather than a
version field inside one file set: a test then loads exactly one major and
cannot validate a v1 fixture against a v2 schema by accident.

**Rejected: a shared Go module (`addons/contract` as a package).** The owner
widened the scope to add-ons built by other people and other agents; a Go
package is a contract only for Go. It would also undo a standing rule
(`addons/README.md`, "the two ends never share a Go module: a shared constant
would hide exactly the version skew the contract version exists to surface")
for the one benefit, less typing, that a fixture already pays for. A
`replace ../contract` directive would make both ends build against the working
tree and hide skew exactly where it starts.

**Rejected: Go types as truth with schemas emitted.** Truth would live in one
compiler, the add-on author would read Go to learn JSON, and emitted schemas
cannot carry what the manifest needs most (`role`, feature gates, `x-`
extension rules) without struct-tag conventions nobody else can read.

**Rejected: fixtures only.** A fixture pins one instance. It cannot say a
field is optional, an enum is closed, or a value is a cursor. Eight unpinned
messages are the symptom; "fixture per message" is necessary and not
sufficient.

**Code generation** is deliberately later. Generating Go from the schema
(`oapi-codegen`) or TypeScript (`openapi-typescript`) is a one-way street the
schema makes possible; it is adopted the day a third Go add-on exists or the
UI's hand-written types drift once. The schema is the truth either way.

## 2. Versioning: an equality-checked major, additive features, nothing in between

**Decision.** `contract_version` stays an integer, checked by equality at
registration, and is the **major**. This change is major 2, and it is designed
so that 3 is years away: every change after it is additive and travels as a
**feature**.

```json
"contract_version": 2,
"features": ["operations", "settings", "subjects.paged", "storage.reach", "storage.homes"]
```

Rules, stated once in `registry/features.json` and the guide:

1. **A reply may always carry more, and never less.** The backend decodes
   every reply tolerantly, the manifest included (today it is strict, which
   makes every new manifest field a breaking change on the one message meant
   to describe change). Unknown features are ignored and logged once at
   registration. Two guards keep "tolerant" from meaning "silent":
   *(a) typos fail the kit, never the reader.* The backend is tolerant
   **everywhere**, inside field, operation and metric objects too, and
   shows every unknown key (top-level or nested) on the target page under
   "What the add-on also says"; otherwise the first key a future feature
   adds inside a field object would break registration on every older
   backend, the failure this rule exists to remove. The **schema** is
   strict: field, operation and metric objects are
   `unevaluatedProperties: false` (not `additionalProperties`, which cannot
   see keys an `if`/`then` branch declares) with `patternProperties:
   {"^x-": {}}`, and role-conditional keys are required through `if`/`then`
   (`role: limit` ⇒ `unit`, `combine`, `unlimited`), so a misspelled
   `unlimted` fails the kit and is not a quota of zero. A key a feature
   introduces is listed in `features.json` under that feature, so an older
   kit answers `skip`, never `fail`. The validator runs with format
   assertion on (`AssertFormat` in santhosh-tekuri v6; in 2020-12 `format`
   is annotation-only by default, so `date-time` on `taken_at` would never
   be checked). *(b) absence is never a value.* The fields each reply
   carries are listed per message (§4 Freshness); a safety-relevant one
   (`current`, `truncated`, `standing.usable`, `taken_at` where the message
   lists it; `outcome` on plan and apply) is decoded as a pointer or
   presence-checked, and a missing one makes the read unusable
   (`current: false`, `truncated: true`, "could not look"), never Go's zero
   value, which would read an add-on that forgot `truncated` as "complete
   list" and every absent subject as "no account".
2. **A request may carry a new field only under a feature.** The add-on
   decodes strictly, so the backend sends a field or calls a route only to an
   add-on whose manifest declared the feature that introduced it. That is the
   whole of capability negotiation: the manifest says, the backend adapts, the
   add-on is never surprised.
3. **Add-on ahead of the backend:** it declares features the backend does not
   know; nothing is sent for them; its surface shows what the backend can use.
   Nothing breaks.
4. **Backend ahead of the add-on:** it does not declare a feature the backend
   would like; the backend does without and the target page says "this add-on
   does not do X" (not "X failed"). Nothing breaks.
5. **A major changes the meaning of something that exists.** It is rare, it
   is announced in `CHANGELOG.md` with the migration, and the backend supports
   exactly one major. Add-ons on the old major are refused at registration
   with both numbers, as today.
6. **Deprecation, in one line.** A feature or field is marked `deprecated`
   in the registry with what replaces it and is removed only at the next
   major. No logging machinery for it until there is a second add-on to
   warn. A field is never *re-purposed*; a new meaning is a new name, and
   the backend enforces it: a manifest that changes the `role` of an
   existing field name, or names a different `account` field, is refused
   at registration with the reason, the last good role map is kept, and
   the target page shows why (a silent role change would change
   withdraw-only and revocation semantics and orphan every deny allowance
   that names the field). The same rule holds for a metric: a change of
   `instrument` or `unit` under an existing name is refused (§9.6).

**Rejected: semver string with minor negotiation.** A minor the backend would
have to range-check is a compatibility matrix with the number hidden inside
it; the features list says the same thing per capability instead of per
release, which is what the backend actually needs to know.

**Rejected: the backend carrying two majors through a transition.** Two
decode paths for every message, for a deployment that ships both ends from
one Compose file. If an external add-on ever needs the window, that is the
day to build it; see owner decision 2.

**Why this change is a major and not a long minor.** Three request-leg
changes cannot be gated (every write gains `call_id` and `contract_version`,
the manifest's `type`/`lifecycle` give way to `role`), and the tolerant
manifest decode is itself a behaviour change at the backend. One coordinated
deploy of two containers is cheaper than carrying both shapes.

## 3. Field semantics: a `role` per field, behaviour derived from it

**Decision.** Every entry in `entitlement_schema` declares a `role` from a
closed vocabulary. `type`, `lifecycle` and `single` are removed; the role
implies them. `label`, `per_person` stay; `unit`, `combine`, `unlimited` are
added where a role needs them.

| role | what it is | wire value | who sets it | mapping-bindable | combine over several roles |
|---|---|---|---|---|---|
| `account` | **the** on/off of the subject's account on the target. Exactly one per schema. | `true`/`false` | resolver | no | n/a |
| `channel` | a way of using the account, off whenever the account is off (TrueNAS `smb_enabled`) | `true`/`false` | resolver (= account, unless an allowance denies it) | no | n/a |
| `membership` | a set of target-defined values a role confers (TrueNAS `group`, UniFi access policy, Ruby group or role) | `["a","b"]`, sorted, deduped | mappings + allowances | yes | union |
| `flag` | one thing a role gives or does not (TrueNAS `home`, Ruby `materials`) | `["true"]` or `[]` | mappings + allowances | yes | any |
| `limit` | a numeric ceiling with a `unit`; `unlimited` names the value meaning no ceiling (TrueNAS `home_quota`, `bytes`, `"0"`) | `["0","5368709120"]` | mappings + allowances | yes | `combine: max`, and **`unlimited` dominates**: max(`"0"`, 5 GiB) is unlimited, not 5 GiB. Combined in **one** place, the add-on (as `apply.go:270-282` does today), which reports the effective limit in observed `state`; every meter and every page reads the observed limit (holding is observed) |

The five are working names (owner decision 3). Whatever the final names,
`feature` is reserved: the manifest already has a `features` list and a role
of that name would make "the add-on declares a feature" mean two things.

TrueNAS, declared this way:

```json
"kind": "storage",
"entitlement_schema": [
  {"name": "group",      "role": "membership", "label": "Group"},
  {"name": "home",       "role": "flag",       "label": "Gives a home folder"},
  {"name": "home_quota", "role": "limit",      "label": "Home folder limit", "unit": "bytes", "combine": "max", "unlimited": "0", "per_person": true},
  {"name": "enabled",    "role": "account"},
  {"name": "smb_enabled","role": "channel",    "label": "SMB"}
]
```

UniFi Access: `{"name":"policy","role":"membership","label":"Access policy"}`,
`{"name":"enabled","role":"account"}`. Ruby: `group` and `role` as
`membership`, `materials` as `flag`, `active` as `account`. None of the three
needs a word in the core.

**What the core derives from roles, and stops hardcoding:**

- `EntitlementSet.Lifecycle` becomes `{Account bool, Channels map[string]bool, NotAMember, Reason}`,
  computed by `resolveLifecycle` over the fields whose role is `account` or
  `channel`. "Disabling the account takes SMB with it" becomes "every channel
  is false when the account is false" with no field named.
- `Desired()` writes every `account` and `channel` field always; a
  withdraw-only set is those fields and nothing else (§37.6 unchanged, now
  generic). `WithdrawsOnly()` is true when the account ends false for a reason
  no policy change confers back: `NotAMember`, or an allowance denying the
  `account` field. The `000087` CHECK ("the switch alone, off") becomes
  target-agnostic with no new column, because only `account` and `channel`
  are JSON booleans on the wire and every other role is an array:
  `CHECK (NOT withdraws_only OR (state_json <> '{}'::jsonb AND NOT jsonb_path_exists(state_json, 'strict $.* ? (@.type() != "boolean" || @ == true)')))`.
  Verified on Postgres 16 (10 Nov 2026): it accepts only a non-empty object
  whose every value is JSON `false`, and refuses `{"enabled":false,"group":["admins"]}`,
  a `true` switch, the string `"false"` and `{}`. The earlier form
  (`'$.* ? (@ != false)'`, lax mode) accepted the groups row: a string or
  array compared with a boolean is *unknown* and the filter never matches.
  (A CHECK cannot read another table, so "reads the registry" was never
  buildable.)
- **The role map is persisted at registration** (`targets.field_roles`
  JSONB, refreshed on every manifest refresh) and every lifecycle reader
  reads roles from it, never from the live manifest. A revocation
  (`handlers/revocation.go:99`) names `enabled` as a constant today and so
  works with the add-on unreachable; by role it must keep working after a
  backend restart while the add-on is down, which a cached manifest cannot
  promise and a column can. Revocation never depends on reachability.
- `IsLifecycleField`, `FieldEnabled`, `FieldSMBEnabled`, `fieldHomeQuota`,
  `FIELD_GROUP/HOME/HOME_QUOTA` are deleted, and every caller moves to the
  role map: `services/entitlement.go`, `handlers/revocation.go:99`,
  `handlers/merge_findings.go:105,574`, `handlers/mappings.go:325`,
  `services/role_members.go:306,358`, `drift/addon.go:723`,
  `handlers/{target_homes,my_storage,person_storage}.go`. Mapping validation
  refuses `account`/`channel`; a `limit` value must parse as a non-negative
  integer; a `flag` value must be `"true"`.
- The UI asks `fieldsByRole(schema)` and renders by role: a `membership`
  picker, a `flag` toggle labelled by `label`, a `limit` input in `unit`, the
  account state from the `account` field. The home-folder page is the storage
  kind's module (§4), not a generic surface that happens to know `home_quota`.

**Two rules made explicit per field, both MUSTs on the add-on:**

- **Absent means unmanaged.** A field not present in `desired` is not touched.
  Present and empty means "make it empty". A withdraw-only instruction
  therefore cannot give (§37.6) on any target.
- **Unlisted values are preserved.** On a `membership` field, a value the
  target holds that is not in the *managed set* is never removed by
  convergence. The managed set is what `GET /values/{field}` lists, and a
  `membership` field MUST be enumerable (SCIM 2.0, RFC 7644 §3.4.2: a
  provisioner manages only what it can list). A non-enumerable membership is
  refused at the mapping (`ResolvesValue`), not at registration: the manifest
  does not say it, `/values` does, and a managed set that lived only in
  Syndra's mappings would be a second place for one fact, orphaned when a
  mapping is deleted. Where the target's API replaces a *subject's*
  whole set (UniFi `PUT /users/:id/access_policies`), the add-on reads,
  merges, then writes; the race with a console edit between the read and
  the write (no ETag) is accepted and documented, and the next drift sweep
  finds what it lost. Where the API replaces a *shared* set (Ruby
  `ReplaceUsersInGroup` writes a group's member list, not a user's groups),
  the add-on MUST serialise read-merge-write per shared set and preserve
  unlisted *members* as well as unlisted values; two applies for two
  subjects must not lose each other's update. This is one rule instead of a
  `replace|merge` knob: Syndra owns what it can enumerate or has named, and
  nothing else, which is also what the drift sweep already assumes: it
  compares the whole list, so `state` and `observed` show only the managed
  set (decided at 5.3).

**Rejected: keeping `type` and adding semantics beside it.** `type` was already
false on the wire: `home` is declared `bool` and travels as `["true"]`,
`home_quota` is declared `int` and travels as `["0","5368709120"]`. The role
states the wire shape truthfully and the `type` had no other reader.

**Rejected: an open-ended `semantics: [...]` tag list.** A field with two roles
has two behaviours and nobody can say which wins. One role per field; a field
that needs two is two fields.

**Rejected: renaming the TrueNAS fields to generic names.** Names are the
add-on's and appear in stored snapshots and bindings. Only meaning moves.

**Two things that are deliberately not roles.** *Schedules and time
windows:* UniFi's schedule lives inside a policy, which is a `membership`
value, and time-bounding is Syndra's own (allowance bounds, role expiry); no
target gets a schedule field. *Profile attributes* (display name, email kept
in sync): identity travels in the plan's subject (`subject`, `email`,
`display`) and the add-on writes it at create and on rename, never as an
entitlement field; the day a target needs an attribute Syndra must converge,
that is a new role and a core change, by design rare.

## 4. Endpoint taxonomy: a core, features, and kind profiles

| Route | Standing | Declared by | Notes |
|---|---|---|---|
| `GET /capabilities` | **core** | — | the manifest |
| `GET /health` | **core** | — | own view + target view + log anchor |
| `GET /subjects` | **core** | — | one reply; `?cursor=` under `subjects.paged`, a feature deferred until a target needs it (§11) |
| `GET /values/{field}` | **core** | — | may answer `enumerable: false` |
| `POST /plan` | **core** | — | mutates nothing |
| `POST /apply` | **core** | — | one subject, plan id + fingerprint + call id |
| `POST /lifecycle` | **core** | — | the operator's stop switch is not optional |
| `POST /operations/{name}` | feature `operations` | manifest `operations[]` | per-op availability as today |
| `GET /settings`, `PUT /settings/{name}` | feature `settings` | — | generic name/label/value/values |
| `GET /calls/{call_id}` | feature `calls` | — | the recorded outcome of a write: `recorded` (with the reply, redacted if `secret_result`) · `in_doubt` · `unknown`; how a `physical` operation is settled (§10); the feature also gates the `preview` request field |
| `GET /stats?metric=…&since=…` | feature `stats` | manifest `metrics[]` | daily points, one call per metric with that metric's watermark (§9) |
| `GET /reach`, `GET /homes` | kind profile `storage` | `kind: storage` + features `storage.reach`, `storage.homes` | storage-shaped replies; `/homes` is the storage kind's *current* usage query, its history is §9 |

**Core** is what every add-on implements and the conformance kit refuses to
pass without. **Features** are additive capabilities with generic shapes.
**Kinds** are a closed vocabulary every add-on declares: `storage`, `access`,
`inventory`, `machine` (a tool with a job log: the laser). The kind is what
keys the credential-scope denylist (`access`: `edit:space`) and the UI
module, so it is never absent: an add-on for a kind not in the list is
refused at registration, and adding a word to the list is a registry entry.
**Kind profiles** are optional per kind and are where a variety of system
gets shapes of its own: `profiles/storage/` holds `reach.json` and
`homes.json` (places, shares, home folders, SMB addresses, usage). `access`,
`inventory` and `machine` ship with no profile (owner decisions 7, 19): the
kind is declared, the denylist fires, and the core routes carry everything
until a member surface needs doors, items or materials. The backend's
generic surfaces (mappings, drift, plans, pending changes, people on a
target, holds due for review) read **only core**; a kind's UI module reads
its profile. Adding a profile is: a directory in the contract, a UI module
keyed on `kind`, and nothing in `internal/services`.

`connection` moves under the kind: `{"protocol": "smb", "host": "nas.example.org"}`
is a storage fact and the `smb://` and UNC builders at `addons/read.go:772`
move into the storage kind's view code, keyed on `protocol`. The core keeps
no address builder.

**Operations.** An operation id is contract vocabulary: `registry/operations.json`
names every id the backend will ever have a policy for, its meaning, its
parameter schema, its scope ceiling and its secret parameters. The backend's
`operationPolicy` is asserted equal to the registry by test (one source; the
policy table is the registry's compiled form, not a second opinion). A new
target-specific operation (UniFi PIN assignment, Ruby password reset) is a
registry entry plus a policy entry, as today; a manifest declaring an id the
registry lacks is dropped and logged, as today. The manifest's `operations[]`
gains an optional `params` JSON Schema that is a **UI hint only** (labels,
placeholders, an enum for a picker); the backend validates every parameter
against the policy's spec alone, as `ValidateParams` does today, because
"intersecting" two schemas is undefined and the policy wins everywhere.
`password.set` keeps its name; a PIN or a card is a different credential and
gets its own id when an add-on needs one (owner decision 6).

Two additions to the registry entry, both needed by the fourth target (§10)
and both **additive, under features**, so that major 2 ships without them
and nothing becomes a major 3 later (owner decisions 18, 20):

- **`result`: the reply shape, declared per operation** (feature `results`).
  Today `OperationResult` carries typed pockets (`Activity *ActivityReport`,
  `Health`), so the core compiles a product's report format. An add-on
  declaring `results` answers one `result` object validated against the JSON
  Schema the registry names for that id, and the backend reads `result` only
  from such an add-on; the pockets stay readable until the next major and
  TrueNAS moves off them in slice 6. Three shapes are generic and have a
  generic renderer: `common/records.json` (a list of things the subject
  holds or did: `{id, label, category?, since?, due?, standing?, x-*}`),
  `common/activity.json` (what `activity.get` answers today, with
  `uncovered[]` and `truncated`, the share word gone: an event's place is
  its generic `resource`), and `common/status.json` (`usable`, `standing`, a
  list of `{label, used, limit, unit}`; `storage.status` answers the storage
  profile `profiles/storage/status.json`, this shape plus places and password
  state, decided in slice 6a). A new operation picks one of these, a kind
  profile of one, or declares its own; its own is shown raw until a kind
  module reads it.
- **`effect`: what the operation changes** (registry only; the backend
  reads it from the registry, so no feature). `read` (writes nothing),
  `account` (the subject's account or entitlements), `credential` (a secret
  the member holds), `physical` (something in the world: stock leaves a
  shelf).

  **`effect: read` has one call path and no log.** Every read operation
  goes through the one generic read path that `storage.status` already
  uses (`addons.MyStorage`, "never dispatched", `policy.go:140-144`,
  generalised): never dispatched, never written to the operations log,
  never confirmed. A member reads for themself through `/me/targets`; an
  admin reads for any subject through `/users/{id}/targets`, where the
  subject is named by the route and the admin is the actor (the A-10 rule
  `storage.status` has today). Without this, `holdings.list` as a
  member-scope operation would either refuse an admin or write an
  operations-log row per page view, which is the view log decision 14
  rejected.

  **`secret_result: true`** in the registry marks an operation whose reply
  carries a secret (a future `pin.assign` returning the generated PIN,
  research §3). The add-on records such a reply **redacted**: a replay and
  `GET /calls` answer `recorded` without it, so no secret sits in the
  add-on's store for 30 days (§8's "never store a declared secret" now
  covers replies).

  The `physical` rules, in the code's own words (`unreached` means never
  sent; "sent, outcome unknown" is `indeterminate`):
  1. `confirm: true` always, and the confirmation names the thing: the
     operation is first sent with `preview: true` (a request field gated by
     feature `calls`, so a strict add-on without it is never sent one),
     which returns the would-be `result` (the item's label, the due date)
     with no effect, the same shape as plan-then-apply; the member confirms
     that. **A preview is never recorded and carries no `call_id`; the
     confirm carries a fresh `call_id`.** (Were a preview remembered under
     an id the confirm then reused, the add-on would replay the preview and
     tell the member the item was checked out when nothing happened.)
  2. At-most-once. An `indeterminate` first attempt is never re-executed by
     anyone: the backend does not resend (operations already have
     `addonAttempts = 1`), a member's retry mints a new `call_id` and is
     refused with 409 `CALL_UNSETTLED` while the earlier intent is
     unsettled, and the add-on MUST NOT execute a physical call whose
     mutation-log intent has no recorded result.
  3. Settlement has a path: `GET /calls/{call_id}` (feature `calls`,
     required with any `physical` operation) answers `recorded` with the
     original reply, `in_doubt` (intent logged, no result, the add-on is
     still finding out), or `unknown` (**no effect recorded**: the request
     never arrived or was refused before any write, so it is safe to
     retry). The member's page polls it through the backend, which settles
     its record on `recorded`, or on `unknown` for a physical call sent
     within `constants.call_memory` (the operator's list asks the same
     question first, for any write whose answer was lost but an adoption or
     release, and leaves any other `unknown` to a person: decided while
     building 6c), and the member sees "we are checking whether it went
     through", then the outcome;
     `in_doubt` for longer than `constants.in_doubt_bound` (`PT1H`) becomes
     a `ready-for-human` item naming the desk.
  4. Natural idempotency, MUST on the add-on: checking out an item this
     subject already holds is `no_change` with the record; one held by
     someone else is `conflict`; returning an item already returned is
     `no_change`.
  5. Physical operations are serialised per subject in the add-on, so two
     concurrent checkouts cannot both pass the limit check.
  6. The add-on keeps call records for **at least `constants.call_memory`
     (`P30D`)** (today's `idempotencyTTL`, now a contract MUST), because a
     third-party add-on with an hour's memory would re-execute on day two.
  There is no `destructive` effect, because there is no destructive
  operation (removal disables, never deletes).

**Dangerous capabilities are impossible, not discouraged.** Three layers, all
existing or cheap: (1) the registry has no `door.unlock`, so no policy, so no
dispatch path, so the backend cannot be asked; (2) the guide MUST: the add-on's
target credential is issued without the scope that actuates the world (UniFi:
never `edit:space`), so a compromised add-on cannot unlock either;
(3) `health.credential_scopes[]` reports the scopes the credential holds, so
the targets page shows it and the kit warns on the denylist keyed on the
declared `kind` (`access`: `edit:space`; `inventory`: whatever the product
names for deleting items, editing stock or writing off losses). The kit
cannot prove a negative about routes an add-on might serve, and the scope
list is self-reported by the least trusted component, so it is **advisory**:
the proof is the operator issuing the credential without the scope, which
the guide's §2 makes the first thing an author does.

**What each read reply carries, as the wire really is.** "Every read
carries `taken_at` and `current`" was false: `/values` carries neither
(`capabilities.go:209`), `/reach` carries `read_at` + `current`
(`reach.go:382-383`), `/homes` carries only `usage_read` (`home.go:235-238`),
health has `snapshot_taken_at` and `last_read_at`. Major 2 settles one
concept under one name, listed per message in `schema/` and nowhere else:

| Reply | `taken_at` | `current` | `truncated` | `stale` |
|---|---|---|---|---|
| subjects | yes | yes | yes | when `current: false` |
| plan | yes | yes (today's meaning: computed against a live read, `plan.go:50-56`) | **yes** (the drain refuses a truncated plan, `propagation/addon.go:388-391`) | when `current: false` (answered from the mirror, §11) |
| reach (storage) | yes (renamed from `read_at`) | yes | — | when `current: false` |
| homes (storage) | yes | yes | — | when `current: false`; plus `quota_read` (today's `usage_read`), a **completeness** flag: the quota table could not be read and the homes are listed live, which is a partial read, not a stale one |
| status, activity, records (`effect: read` operations) | yes | yes | activity: yes | when `current: false` |
| values | **exempt**: an enumeration of legal values is not an observation | — | — | — |
| stats | yes (when the collector last read the app) | **—** (the watermark says it) | — | — |
| health | `snapshot_taken_at`, `last_read_at`, as today | — | — | — |

The kit's "required reply fields" check reads this table, not "any read".

**`current`, defined once:** "this answer comes from a read of the app
that began after this request arrived". A single-flighted read counts.
Today's `SubjectsResponse.Current` already means this ("came from the
target just now"; "the drift sweep consumes only current reads",
`server.go:281-285`), and that meaning is kept, because an add-on that
answered `current: true` from a mirror would make the drift sweep compare
against a mirror, the defect that comment exists to prevent. On every reply
the table marks with a `stale` column, whenever `current` is `false` the
reply carries `stale`, and the invariant is
**`current == false` ⇔ `stale` present**:
`stale: {reason: cached | cooldown | unreachable | draining | never_read, retry_after: 20, note: "The laser PC has been off since 18:00"}`
(`retry_after` in integer seconds, mirroring the `Retry-After` header;
every *declared* duration in a manifest or registry is ISO 8601, `PT30S`,
`P90D`). `stale` is a core reply object (a reply may always carry more),
so an add-on without `freshness` still says `cached` when it answers from
its mirror; the request side is the feature.

**Freshness: Syndra says whether it needs a fresh read, the add-on decides
how to give one** (owner, 10 Nov 2026: "Syndra should be able to force a
refresh, a fresh read"; feature `freshness`, owner decision 21). The reads
in scope are `GET /subjects`, `GET /values/{field}`, the storage profile's
reads, and operations with `effect: read`; each may carry `fresh: true` (a
query parameter on GET, a body field on operations; one boolean, because
the only value ever sent would have been `max_age=0`). Without `fresh`, an
add-on declaring the feature MAY answer from its mirror with
`current: false, stale.reason: cached`; with it, the add-on reads the app
now, or answers the last known value with `stale.reason: cooldown` and
`retry_after`. Who sends it: **the drift sweep's `/subjects` read always
sends `fresh: true`** and is the only scheduled sender (it needs `current`,
and a cooldown answer is a skipped pass, exactly as `current: false` is
today); display reads send it only when a person presses "Refresh"; every
other display read accepts `cached` and shows `taken_at` ("as of 08:00").
`/plan` and `/apply` are **not** in scope and are never cooled down, and
that includes the planner's one-subject re-plan, which *is* a `/plan` call
(`propagation/addon.go:363`, `planOne`): `/plan` keeps today's meaning,
computed against a live read, `current: false` only when the mirror had to
answer, and the drain refuses a non-current fingerprint as it does now, so
nothing about the planner changes and N departures never cost N × 30 s;
`/stats` is not in scope because when the app is read for statistics is the
add-on's (§9.1). A read answers with an error only when the add-on has
nothing to serve at all: `FRESH_READ_COOLDOWN` (429, `Retry-After`),
`TARGET_UNREACHABLE` (503, `Retry-After`) when no request reached the app,
or `NO_CURRENT_READ` (503) when the app was asked and failed (B3), all
registered, and the backend
reads `TARGET_UNREACHABLE` as `unreached`, never as a lifecycle refusal:
with the envelope in place a lifecycle refusal is detected by the code
`LIFECYCLE_REFUSAL` alone, no longer by "503 with `Retry-After`"
(`transport.go:342,401`). The add-on enforces the bounds because it knows
its app: single-flight coalescing of concurrent fresh reads of the same
thing; a per-subject and a per-target cooldown (`limits.fresh_read_cooldown`,
default `constants.cooldown_default = PT30S`, declared so the UI can say
"refreshed 10 s ago, again in 20 s"); the target-wide read (`/subjects`) is
the rate-limited one, and the drift sweep's `fresh` read (every
`DRIFT_RECONCILIATION_INTERVAL_HOURS`, 6 h by default) is far outside any
sane cooldown. A fresh read that joins one already under way arrived after
it began, so it is answered as `cooldown`, never `current` (decided while
building 6b, B1); one that joins a read that fails is answered as that
failure is, and never reads the app again. A reply made of several reads,
answered later from the add-on's copy, carries the copy's own reason,
dated by its oldest part, never a part's label from when the copy was made:
a copy says `cooldown` only to a fresh read the cooldown refused. Answered
`cooldown`, the sweep waits once, `min(retry_after or the declared
cooldown, the declared cooldown, 60 s)`, the ceiling twice
`constants.cooldown_default`; "Check now" runs the same pass inside a
person's request and never waits (`ReconcileAddonNow`). An add-on's
`taken_at` more than two minutes ahead of the backend's clock is read as
now, logged once per target (decided while building 6b, review round 2),
and one dated the zero time is not a reading: every reply shape, the typed
readers' and every `results` shape's, through one clamp (round 3). A kept
copy is dated by its age on the add-on's own clock, as an HTTP cache does (RFC
9111 §4.2.3; 7c): age_value = the reply's `Date` less `taken_at`, corrected_age
= age_value + response_delay, and the copy dates request_time - age_value on
this clock, so a skew cancels. The RFC's apparent_age (response_time - `Date`)
is left out on purpose: it compares two clocks. A `Date` more than a second
before `taken_at` is another clock's and ignored; without a usable `Date` the
clamp above applies as before, and an add-on MUST send one (kit check `date`).
One function, `ahead`, dates every taken_at: the typed readers', every result
shape's, a physical call's reply and its `/calls` record, `/stats`, and the
health timestamps. For the kit to prove any of this black-box, an add-on
declaring `freshness` MUST carry `health.target_reads`, every read it has
made of its app since it started, never decreasing while it runs and free to
start again at 0, like an OpenMetrics counter (`log_records` counts
mutations and cannot serve); it is required with the feature, not in the
core (B2). Syndra
cannot use `fresh` to dictate *how* the app is read, and it is not a bulk
mechanism.

**The query string is signed, as sent.** Today the MAC covers
`ts.method.path.body` (`transport.go:441-450`) and not the query, which
`?metric=&since=` (§9) and a future `?cursor=` would leave outside it.
Major 2 signs the **raw query verbatim**: the input becomes
`ts.method.target.body` where `target` is the escaped path exactly as sent
plus `"?" + RawQuery` when the raw query is non-empty: the query as
RFC 9421's `@query` has it (as sent: not sorted, not re-encoded), except
that an empty query adds nothing (`@query` would add `?`), so every
query-less MAC is byte-identical to major 1's. Two things change for the
path: both ends sign the *decoded* `URL.Path` today (`backend
transport.go:317`, `addons/truenas/transport.go:104`), and major 2 signs
the escaped path as sent, which is a change with a vector of its own.
No canonicalisation: two containers on one host have no intermediary that
rewrites a query, and a canonical form (SigV4's seven rules on sorting,
repeated keys, empty values, `+`, the unreserved set, uppercase hex, decode
before encode) is exactly the kind of text a Python add-on built on
`urlencode` fails on its first `cursor`. A reordered query is a modified
query and verifies as nothing. `transport_derivation.json` gains four
vectors: no query, a base64url `cursor`, a value with a reserved
character, and an escaped path. About five lines per end, and both ends ship together in this
major anyway. The reasoning this replaces was wrong in kind: the channel is
TLS 1.3 pinned to an Ed25519 key derived from the same secret
(`derive.go:87-120`), so no on-path peer can read or alter headers, query,
body or responses, and the only party that could alter the query already
holds the HMAC key; what the MAC gives is caller authentication and a
*complete* binding of the request, and a forged `cursor` (a skipped page,
absent subjects read as "no account") or a forward `since` (statistics lost
for good) is not "one bounded extra read". Responses need no signature:
the pinned TLS already means only the secret holder can answer. What was
missing is **content** validation at Syndra (a per-person point for a
subject not bound to that target, §9.6), not transport.

## 5. Identity binding and idempotency

**Binding.** The backend records, per subject and target, an `account_id`
(string, opaque, stable on the target: TrueNAS uid, UniFi user `id`, Ruby
user `id`) and a `display` name (`username`). Rules, MUST for every add-on:

- A subject is found by its recorded `account_id` first. An **unbound**
  account that matches by the target's own external-reference field (UniFi
  `employee_number`, which the add-on writes as the Syndra subject id at
  create) or by derived name is adopted **only** when the add-on's mutation
  log holds a create intent for that subject with no recorded id (the crash
  window below); otherwise it is a `conflict` for an operator, who adopts
  through the confirmed `account.adopt` operation. Anyone with console access
  can type an `employee_number`, and a wrong adoption hands over someone
  else's data, which is exactly why `account.adopt` is confirmed. Name is
  display, never identity: a rename on the target is recognised by id,
  reported, never treated as a missing account.
- Derivation of a new name is the add-on's (the storage derivation in
  `addon-platform` §11 is the TrueNAS one); it happens once, at create, and
  the resulting binding is authoritative. A derived name that collides with
  an unbound account is a `conflict` on the outcome, never an adoption.
- `ApplyResponse.UID int64` becomes `account_id string`; TrueNAS formats its
  uid. (The name: SCIM's `externalId` is the *client's* id, which here is
  the Syndra subject id UniFi stores as `employee_number`; the target's own
  id is SCIM's `id`. The column is `account_uid` already, so the wire field
  and the new column are `account_id`, owner decision 26.) The column is
  `account_uid BIGINT` in **two** tables (`000032:38`, `000039:22`), but
  only `target_account_bindings` has the partial unique index (`000032:62`);
  `target_binding_conflicts` keeps resolved history and is unique only on
  `(target, username) WHERE resolved_at IS NULL` (`000039:52-54`), so a
  unique `(target, account_id)` there would make a second conflict on the
  same account fail to insert after the first is resolved. So: `account_id
  TEXT` beside the old column in both tables; the partial unique index
  **on the bindings table only**; backfilled by a **re-runnable** statement
  (so rows v1 writes after a rollback are filled on the next forward
  deploy), both written for one release, `account_uid` dropped a release
  later by its own task.

**Idempotency: body `call_id` on every write, inside the signature.** Apply,
every operation, lifecycle and settings carry `contract_version` and
`call_id` (lifecycle already carries `contract_version`, `lifecycle.go:158`;
the backend already sends `Idempotency-Key` on lifecycle through `operate`,
`pipe.go:79-81`, and the add-on ignores it, so "nothing for lifecycle" was
half true). The `Idempotency-Key` header is removed from the pipe. The
add-on dedupes all four kinds through one store and answers a replay from
the first call's recorded reply. **Request identity, per kind**, is the
SHA-256 of the RFC 8785 (JCS) canonical JSON of:

| Write | Identity object | Deliberately excluded |
|---|---|---|
| apply | `{subject, desired}` | `plan_id`, `fingerprint` (below); `email` (resolved live at dispatch, `propagation/addon.go:178`) |
| operation | `{name, subject, params − secret_params}` | every `secret_params` value, so the store never holds even a hash of a member's password; a same-id call with a different password is a replay of the first. `preview` is not in it: a preview is never recorded (§4) |
| lifecycle | `{state, reason}` | — |
| setting | `{name, value}` | — |

Why `plan_id` and `fingerprint` are excluded: a system row re-plans before
every dispatch to get a fresh fingerprint (`propagation/addon.go:358-398`)
and re-sends an indeterminate row under the same outbox call id
(`:296-306`, `:188`); if the first attempt applied, the re-plan sees the
converged state, the fingerprint differs, and a whole-body rule would
refuse, which becomes `target_refused` and under §37.7b makes the person's
next withdrawal wait for an operator. Four rules, all MUST:

1. Same `call_id`, same identity → the recorded reply, **answered before
   the fingerprint check** (`apply.go:347-351` already does it in that
   order).
2. **Only a 2xx reply is recorded.** A refusal (a 503 `LIFECYCLE_REFUSAL`,
   a 422) is not, so a requeued row with the same `call_id` is re-evaluated
   and not answered with the old refusal until it is exhausted (today a
   TrueNAS comment, `apply.go:409`; now contract).
3. Same `call_id`, different identity → **422 `CALL_ID_REUSED`**, following
   draft-ietf-httpapi-idempotency-key-header, where a key reused with a
   different payload is 422 and 409 is reserved for a request still in
   flight (today `CALL_ID_REUSED` means "used for a different kind",
   `store.go:131`, and answers 409; major 2 widens the meaning and corrects
   the status).
4. Same `call_id` while the first is still in flight → **409
   `CALL_IN_PROGRESS`** with `Retry-After`, classed `unreached` by the
   backend. **"In flight" means a request with that `call_id` currently
   executing in this process**, held as an in-memory lease taken before
   the mutation-log intent and released when the reply is written or the
   handler returns; it does not survive a restart and it is never inferred
   from "an intent with no result" (that reading would answer every
   post-crash re-send of an indeterminate apply with 409 for ever, and the
   row would requeue for ever). After a restart, an apply relies on
   lookup-before-create (below) and a physical operation on
   `CALL_UNSETTLED` (§4).
5. **An entry remembered before major 2 carries no identity digest and is
   treated as matching**, exactly as `store.go:126-132` already treats an
   entry with no `Kind`: outbox rows that were indeterminate when the
   backend was stopped for the deploy are re-driven by v2 under the same
   `call_id`, and without this rule each would get 422 `CALL_ID_REUSED`
   and become `target_refused`.

The header was chosen (pipe.go) because a strict add-on refuses a new body
field; major 2 removes that reason. The argument for the body is not an
on-path threat (the pinned TLS, §4, leaves none) but completeness: the MAC
should bind everything the add-on acts on, and the replay token is
something it acts on.

**A target with no idempotent create** (all three studied: TrueNAS
`user.create`, UniFi `POST /users`, Ruby `CreateUser`). MUST: the add-on
looks up before it creates (by the rules above), writes the mutation-log
intent before the create, records the returned id under the `call_id` before
it answers, and on replay answers from that record. The remaining window, a
crash between create and record, is closed by the next apply's lookup: an
unbound account matching by external reference or derived name is adopted
**because the log holds the intent**, and that is the only adoption without
a human; with no intent, or with two candidates, it is a `conflict`. The
guide says this in those words and the kit replays an apply and asserts one
account.

"The log holds the intent", made exact for TrueNAS (5.3d,
`leftByCrashedCreate`): the account holding the derived name is unbound (by
name and by uid), not the add-on's own, inside the add-on's uid range, and
the log holds a `user.create` intent for **this** subject asking for **that
uid** (a create's records and every binding write carry `account_id`; a
name is display, never identity) whose latest result is `unknown`, with no
binding of that uid by any subject, and none of this subject, logged after
it, and no other subject's create of the uid that did not plainly fail (two
candidates are a conflict). The log is read across subjects because a uid is
not unique on trust: an orphan group's gid is reused, and a restored store
rolls the high-water mark back. Plan predicts it (`Adopts …`); apply checks
what could block, then binds it after the fingerprint check through
`mutate`, converges and answers as the create would have. That logged bind
is the trace that settles the doubt: a later create, even after a release,
finds a binding of the uid after the intent and does not adopt again.

## 6. States beyond on and off: `standing`

Every subject in `GET /subjects` and every apply outcome carries:

```json
"standing": {"usable": false, "waiting_on": "member", "note": "No password set yet", "next": "Set a storage password on your page"}
```

`waiting_on` ∈ `member | operator | target | none`. `standing` is present
**if and only if an account exists** on the target: a withdraw-only
`no_change` for a subject with no account carries none, and the required-
field check (§2) reads it that way. It is observed, not desired: the add-on
reports why a converged account cannot yet be used.
TrueNAS: no password → `member`. UniFi: no credential → `member` ("enrol a
card at the front desk"), status `PENDING` → `target`. Ruby: nothing per
subject; "the PC is off" is `health.reachable: false`, target-level. The
member page, the people-on-target list and the holds review show
`note` + `next` from the add-on, with no product word in the core. This
replaces `TargetAccount.PasswordSet` and the `smbWaiting` wording in
`apply.go:110`, which stay inside the TrueNAS add-on as the source of the
sentence.

Health keeps the target-level states: `reachable`, `lifecycle`
(`active|draining|read_only`), `version_tested`, `circuit_open`, plus
`credential_scopes[]` (§4) and `credential_expiry` generalised from
`key_expires_at`/`key_expiry` (`set|none|unrecorded`). `last_read_at`,
`checked_at` and `log_records` are pinned with their types (the inventory
found `uint64` vs `int64` and a field with no counterpart).

**Rejected: an enum of product states in the core** (`password_unset`,
`credential_unenrolled`, `pending_sso`). Each is one product's word, the next
target adds one, and the core would be learning each product, which §0 of
`addons/README.md` forbids. The add-on knows the word; the core knows who is
waited on.

### 6.1 What a system is called, and where a member meets it (3.10)

**The name is a registration attribute, shaped once by the backend.**
`targets.name` (migration 000091, nullable, `targets_name_check`: never blank,
at most 60 characters) is set on the system's own page under System, its one
write place (`PUT /api/v1/targets/{target}/name`, strict JSON, audited as
`target.named` with the old and new name in the same transaction). Every
roster carries `name = coalesce(targets.name, manifest.product, the id
title-cased)`, computed by `handlers.systemName` alone: `GET /targets` (with
`name_from`: `operator`, `product` or `id`, so the Name setting words its hint
for the case it is in), `GET /me/systems`, `GET /me/targets` and a person's
targets. Every surface draws a system by that `name`; the UI's `targetLabel`
is only the id title-cased for a row drawn before a roster lands, and its
product map is gone. Every helper that words a system (`describeAction`,
`describeSends`, `systemsOf`, `describeDrain`, `outcomeFromDrain`) takes the
caller's `nameOf` as a required argument, so there is one path for names (the
audit CSV export included). Two laser cutters from one image share a
manifest, so both default to its `product` until an operator names one.
**Rejected:** a manifest field (wrong owner: two deployments of one image
share it), a name per kind (cannot tell two apart), and the UI map (product
names in core code). The contract's `product` is therefore a display name
("What people call the product", `v2/schema/manifest.json`, CHANGELOG): the
TrueNAS add-on reports `"TrueNAS"` (it reported `"truenas_scale"`), from one
constant its fixture test pins. No name is seeded by migration. The 60-char
limit is the CHECK's; the backend's refusal and the Name field's `maxLength`
say the same number, held by one parity test (`name-limit.test.ts`, which
reads the migration and the Go refusal, the way the proxy test reads the
router) rather than an API field for a number that does not change.

**Where a member meets a system.** By kind, decided by the accepted manifest
(owner, 11 Nov 2026): kind `storage` folds into Network storage (`/storage`
lists kind `storage` only); any other kind gets its own row in the member rail
and its own page, `/systems/{target}` (a storage system there is sent to
`/storage`, one address per page). A registration with **no accepted manifest
is operators-only**: no member row, no member card, decided server-side
(`handlers.memberMeets`), so neither member read ever carries one. Rows are
registrations, never entitlements: a member none of whose roles reaches the
system has the row, and the page says so with a next step. The rail's seat is
a static *family* leaf in `MEMBER_NAV` (`basic-advanced-ia` §1), so
reachability stays derived from the static tree and the Edge middleware needs
no data.

**The rows' one source** is `GET /api/v1/me/systems` (`withUserAuth`, on the
proxy's member allowlist): `[{target, name, kind}]` from the registry, the
accepted manifest's kind and `targets.name` only, never an add-on call, the
systems a member meets, sorted by name (`textsort`, the UI's collation). The
member rail, the touch bar, Go to, the `/systems/{target}` gate and an
operator's account sheet all read it (`useMySystems`); `memberSystems` is the
one rule for which of them get a row. `GET /me/targets` (live add-on calls)
stays the cards' read. The touch bar holds an empty seat of its own height
while the read is in flight, so it never draws three tabs and then four; a
`/systems/{target}` page names its crumb (`useCrumb`) only once the read
lands, never an id first, and never a system the member does not meet. An
operator's account sheet lists their own non-storage systems after Network
storage, so their own laser account has a way in.

**The person page** (operators): Network storage (the storage kind's module,
`components/kinds/storage/PersonStorage`), then **Connected systems**, one
kind-agnostic card per other system, not read included: the account, the
add-on's `standing`, the roles that reach it and any hold.

## 7. The conformance kit

`addons/conformance`: a Go module, one binary.

```
syndra-addon-conform --url https://addon.example.org:8443 --target truenas --secret-file ./secret \
                     [--subject conformance-<run>] [--email conformance-<run>@example.org] [--mutate]
```

It speaks the real transport (derived keys from `transport_derivation.json`),
embeds the contract it was built from (`contract_version`, the schemas, the
registries, the fixtures; `addons/contract` is an embed-only module that
exposes those files and no Go type, so §1's rejection of a shared types
module stands, and only the kit imports it; there is no override flag), and reports every check as `pass | fail | skip`
with the MUST it enforces. Non-zero exit on any MUST failure. Without
`--mutate` it runs reads only; with it, it creates and converges one
throwaway subject, always named `conformance-...`, refusing to touch a subject
that already exists (the `record-truenas-fixtures.sh` rule; the plan is read by
the presence of `account_id`, `standing` or `conflict`, never by their value)
or to write at all once a read has failed a MUST, and at the end leaves **one disabled,
labelled account** on the target and says so in its last line: removal
disables and never deletes, so "as it found it" is not a promise the kit can
keep, and it does not pretend to.

| Check | What it proves |
|---|---|
| manifest | validates with format assertion on (typos in field, operation and metric objects fail: `unevaluatedProperties: false`, §2); major equals the kit's; exactly one `account` role; every op id is in the registry; a feature the kit does not know is `skip`, never `fail`; `kind` is one of the closed list |
| health | validates; `log_head`/`log_records` present and typed; `target_reads` when `freshness` |
| required reply fields | for each reply, the fields the §4 table lists for it are present (`taken_at`, `current`, `truncated` where listed; `standing.usable` on every subject that has an account; `outcome` on plan and apply); an absent one is a `fail` (absence is never a value, §2) |
| subjects | validates; every `state` key is a schema field; `current`, `truncated`, `taken_at` present; `current: false` ⇔ `stale` present; cursor checks `skip` unless `subjects.paged` |
| values | every schema field answers; `account`/`channel` answer `{false,true}`; unknown field → 404 `UNKNOWN_FIELD` |
| strict requests | unknown field → 400 `UNKNOWN_REQUEST_FIELD` (its own code, because `BAD_REQUEST` is also a missing `call_id` or `subject`, `apply.go:331-332`, which must stay `rejected`); wrong or **absent** `contract_version` → 422 `CONTRACT_VERSION_MISMATCH`, both numbers in the body |
| error envelope | every non-2xx is `application/problem+json` and validates against `error.json` (RFC 9457: `type`, `title`, `status`, `detail`, plus the `code` and `retry` extensions) with a registered code and the registry's status. Blocked (422 `APPLY_BLOCKED`) and binding-conflict (409 `BINDING_CONFLICT`) apply outcomes travel as problem extensions (RFC 9457 §3.2): `effect`, `consequence`, `username`, `account_id`, `standing`, `fingerprint`, `conflict` ride beside `code`, so there is no non-2xx that is not a problem. |
| plan | shape; fingerprint per subject; effect in the closed set; above `limits.subjects_per_plan` → 422 `PLAN_TOO_LARGE` with the limit (the backend chunks and acknowledges, §11; the add-on only refuses) |
| plan = apply | the plan's effect and consequence for the kit subject equal the apply's |
| fingerprint | apply without one → `FINGERPRINT_REQUIRED`; with a wrong one → `PLAN_STALE`, nothing written (`log_records` unchanged) |
| replay | same `call_id`, same identity → byte-identical reply, `log_records` unchanged, **even with a different `plan_id` and `fingerprint`** and, for `password.set`, a different password (the replay is answered before the fingerprint check and secret params are outside the identity, §5); same `call_id`, different identity (another subject, another `desired`) → 422 `CALL_ID_REUSED`; a refused call (`read_only` → 503) then the same `call_id` after `active` → executed, not the old refusal (only 2xx is recorded); a replay after 29 days still answers (skipped unless `--clock` is given) |
| read-back | `observed` of the apply equals the subject's `state` on the next `/subjects` |
| withdraw-only | `desired = {account: false}` for a subject with no account → `no_change`, no account created; for an **existing** account → the account off and every membership unchanged |
| channel follows account | `desired = {account: false, channel: true}` → observed channel false |
| absent is unmanaged | apply with the account field alone leaves every membership as read before |
| unlisted hidden | a membership in `state` or `observed` holds only the managed set (no value `/values` leaves out); that such a value survives an apply is invisible from outside, so it is the add-on's own test (5.3) |
| limit combine | two mappings, `"0"` and `"5368709120"` → observed limit is `unlimited` (unlimited dominates, §3) |
| lifecycle | `read_only` → apply answers `LIFECYCLE_REFUSAL` (503, `Retry-After`), reads still answer; no reason → `REASON_REQUIRED`; unknown → `UNKNOWN_LIFECYCLE_STATE`; back to `active` |
| results (feature `results`) | every available `effect: read` operation answers one `result` valid against the schema `registry/operations.json` names for it (which holds `current: false` ⇔ `stale`), or an object for an id that declares its own; without the feature, a read with no registry `pocket` (`holdings.list`) declared available is a `fail`, and otherwise `skip` |
| operations | unknown → 404; missing required param → 400; a canary secret value never appears in any later byte of any reply, health, error, replay or `/calls` answer (`secret_result` redaction included); a `physical` operation refuses without `confirm`, answers `preview: true` with no effect and no record (`log_records` unchanged; a preview sends nothing to `/calls`, and `/calls` on a never-used id → `unknown`), and `GET /calls/{call_id}` answers `recorded` after the confirm; a second call while the first is `in_doubt` → 409 `CALL_UNSETTLED` |
| calls (feature `calls`, built in slice 6c) | `calls.unknown` (a never-used id → `unknown`, valid against `call.json`); `calls.recorded` (the `password.set` call's id → `recorded`, its reply equal by JSON value to the first; the canary scan covers it); `operations.physical`, with `-item` naming an item the run checks out to its subject and leaves so: a physical op declared without `calls` or without `confirm` fails, the preview's `result` validates and writes nothing, the confirm with a fresh id → `succeeded`, `log_records` +1, `/calls` → `recorded`, the same item under a new id → `no_change` with `log_records` unchanged, and the replay byte for byte. `calls.unsettled` and `calls.serialised` always skip: a black-box kit cannot lose a reply or know the limit, and the reference add-on's own tests hold both (decided while building 6c, as `freshness.unreachable`) |
| transport | unsigned → 401; stale timestamp → 401; wrong key → 401 without saying which half; a signed request with its query reordered or altered → 401; the served key equals the derived one; the three query vectors in `transport_derivation.json` reproduce |
| freshness (feature `freshness`) | four checks, each `skip` without the feature, measured by `health.target_reads` against a baseline of two back-to-back `/health` reads: `freshness.cached` (a read without `fresh` answered from the mirror says `stale.reason: cached` and does not advance the count); `freshness.cooldown` (`fresh: true` advances it, by what one read of the app costs, and a second at once answers `stale.reason: cooldown` with `retry_after` or 429 `FRESH_READ_COOLDOWN` with `Retry-After`, advancing it by nothing; `/plan` straight after is `current`); `freshness.coalesce` (two concurrent `fresh: true` reads advance it by what **one** fresh read alone does, and one of them is current); `freshness.unreachable` always skips (a black-box kit cannot take the app down; the add-on's own test holds the last known value answered as `stale.reason: unreachable`, not an error). The cooldown is read from `limits.fresh_read_cooldown`; one longer than the kit's `FreshWait` (35 s) is a skip with the reason. Every reply in scope holds `current == false` ⇔ `stale` present |
| dangerous scopes | warn when `health.credential_scopes` intersects the declared kind's denylist (advisory: self-reported) |

Two things the kit does not claim: that the add-on never follows a 3xx
(a client property of the backend's transport, tested there) and that
Syndra talks to nothing but the add-on (a backend guard test, §11).

**Versioned with the contract.** The kit's module is tagged with the
contract major and minor-less feature set it knows; `--url` against an add-on
declaring a newer feature prints it as `skip: unknown feature`, never `fail`.
The kit is the one place the schema validator must exist; the two ends'
suites also use it because a fixture that validates in CI is cheaper than one
that validates on the day somebody runs the kit.

**Self-test.** `addons/reference` is a complete add-on over an in-memory
target (accounts, one `membership` field, one `flag`, one `limit`, one
`account`, one `channel`, one member operation). CI runs the kit against it
on every push, which is what keeps the kit honest when the contract moves.
The reference add-on is also the guide's worked example, so the example is
executable and conformant by construction.

**Against the real NAS**, the kit runs on staging with `--mutate` under an
operator's hand (`ready-for-human`); against production, reads only.

## 8. The author's guide: `addons/AUTHORING.md`

Written for an agent with no other context; every rule uses the BCP 14
keywords (RFC 2119 + RFC 8174: only UPPERCASE carries meaning). Outline:

1. **What an add-on is and is not.** The only thing that talks to the app:
   it alone holds the app's address and credential, reads the app on its own
   schedule, and Syndra talks only to it (the boundary rule, §0). Translates
   desired state; decides nothing; least trusted; one container, own
   network, no published port.
2. **Before you write a line.** Record what the target answers
   (`<product>_observed.json`), version-gate on what you recorded, pick the
   stable id, decide what the credential may do and issue it without the rest.
3. **The manifest.** `kind`, `features`, `entitlement_schema` with roles
   (the table in §3), `operations`, `connection`, `limits`, `x-`.
4. **The core routes**, one section each, with the fixture inline and the
   MUSTs beneath: strict decode; `contract_version` on every request;
   `call_id` dedupe on every write; absent = unmanaged; unlisted preserved;
   withdraw-only never creates; `account` false ⇒ every `channel` false;
   fingerprint verified immediately before the write; the replay answered
   before it, only a 2xx remembered, secrets outside the identity and
   redacted from a remembered reply; read back after write; the fields the
   §4 table lists, honest and never absent; `current` means "read after
   this request arrived" and nothing else; `fresh` honoured within your
   cooldown, the last known answer with `stale` rather than an error;
   `standing` on every subject that has an account; the query signed
   exactly as sent.
5. **Plan and apply agree.** One pure `decide`; plan serialises it, apply
   executes it (one-path W1f); the kit checks it.
6. **Errors.** The envelope, the registry, the retry classes; never echo a
   request body, never a secret, field paths not prose.
7. **Operations.** Registry first, policy second, manifest third; `scope`,
   `confirm`, `secret_params`, `secret_result`, `effect`; member scope
   binds the subject; `effect: read` is served through the one read path,
   never dispatched or logged.
8. **Security rules.** Never store or log a declared secret anywhere
   (tables, log, snapshot, error, panic, request-logging middleware); the
   target credential lacks every destructive and world-actuating scope;
   no destructive default (removal disables, never deletes); pinned transport,
   https only, no redirects; the mutation log `0600`, fsync before the write,
   chained, head on `/health`.
9. **Lifecycle and degradation.** `active|draining|read_only`; reads never
   gated; "could not look" is never "nothing".
10. **Kinds.** Which word to declare (the closed list), what a profile
    adds, how the UI module is found; `storage` as the shipped example.
11. **Worked example: the reference add-on**, file by file, ~400 lines.
12. **Mapping sketches.** TrueNAS (shipped), UniFi Access (`kind: access`;
    policy as membership, `employee_number` written at create and adopted
    only under a logged intent, `standing` for no credential, `PENDING`
    observed as `account: true` with `standing.waiting_on: target` so the
    apply never loops on a permanent difference, merge on `PUT` with the
    console race named, never `edit:space`; the research note's §7 row
    mapping `enabled` as "withdraw-only, reactivation a deliberate act" is
    superseded by §3's resolver-driven `account`), Ruby (`kind: machine`; `active`
    as account, roles and groups as membership, `materials` as flag,
    `ReplaceUsersInGroup` serialised per group, health from the version
    probe, read-only on an unrecorded version), the inventory system
    (`kind: inventory`; §10, with its unverified items listed).
13. **Validating before you ask to be connected.** Run the kit; attach its
    report; the operator adds the target to `ADDON_TARGETS`, a Compose
    service behind a profile, and a secret; the backend refuses a major
    mismatch and drops unknown operation ids, and the targets page says so.
14. **Evolving.** Features, deprecation, `x-`, what needs a registry entry.

## 9. Statistics: metrics are pulled and kept, records are read live

Owner, 10 Nov 2026: "a proper channel for the app → add-on → Syndra for
statistics reporting, be it overall usage, per user usage, or any other
statistical data." And: "TrueNAS already pushes how much storage a user has
written." Checked in code: nothing is pushed. Syndra pulls that number on
demand by two paths, `storage.status` (a person's per-share usage, for their
own page, `addons/read.go` `MyStorage`) and `GET /homes` (home-folder usage
per account against its quota, for the operator view, `ReadHomes`), and
keeps no history: every number is read live and forgotten. That is the right
behaviour for "how much now" and no behaviour at all for "how much over
time", which is what statistics are.

**Decision in one paragraph.** One feature, `stats`, adds one read route,
`GET /stats`. Syndra pulls it on a schedule with per-metric watermarks and
upserts what comes back into one table; what can come back is declared in
the manifest as metric descriptors, exactly as fields are declared with
roles, so the backend stores and the UI renders any add-on's statistics
with no product code. Every bucket is a makerspace day in this major: a
calendar day in the add-on's declared `stats_zone`, which must be Syndra's
one zone (`stats.hourly` is a later feature, owner decision 12, amended 12
Nov 2026): daily is what a makerspace reads,
and it removes a rollup job, a second row shape and two double-counting
hazards.
Discrete records (what I did, what I hold) are not statistics: they stay
live per-person reads through operations with a declared `result` shape
(§4), and Syndra never stores them. The storage usage numbers are the first
metrics on the channel, and the two live reads stay as the "current" query
beside the history.

### 9.1 Direction: the add-on collects from the app, Syndra collects from the add-on

**Recommend: two collectors, one boundary.** The add-on owns all contact
with the app, including its timing: it reads usage from the app on its own
schedule (per target, as its product allows: TrueNAS `audit.query` hourly,
Ruby's job history when the PC answers, UniFi `system/logs` by `since`),
respects the app's rate limits and availability, derives daily points, and
writes them to its own durable buffer with a **watermark per metric**: the
instant through which that metric is final, *including absence*. The add-on
MUST NOT advance a metric's watermark past a window it may still fill (the
laser PC is off: `laser.job_seconds` stays at the last day it could read,
while `storage.used_bytes` on another source moves on). Syndra calls
`GET /stats?metric=<name>&since=<that metric's watermark>` **once per
declared metric** (≤ 50) per target on its own schedule (hourly, through
the existing pipe and breaker, by its own runner `ADDON-STATS` beside the
add-on reconcile, never a step inside it: a slow `/stats` must not delay
convergence, and a failed collection is not a failed reconcile) and takes
everything since. A coded `/stats` answer (`NO_CURRENT_READ` before the
add-on's first read of its app) is the add-on answering on purpose: it
neither opens nor clears the breaker; a transport failure, a timeout or an
uncoded 5xx counts as on any route, and the open breaker is still consulted
first (6d-2 fix round). One call per metric, not one `since` for all: with a
single oldest mark, a laser PC off for a term would make every hourly
collection re-send up to 90 days of every other metric (per-subject ×
per-share `storage.used_bytes` is ~10⁴ points a day, hundreds of MiB an
hour through 1 MiB pages, all skipped on arrival). Syndra never decides how
or when the app is read; the add-on never calls Syndra. Where a target pushes to the add-on
(UniFi webhooks), the add-on folds that into the same buffer and Syndra
still pulls. Every rule the platform already has keeps holding: no inbound
route on the backend, no second credential, no add-on-side queue (a buffer
of observations is a store, not a queue of work), the add-on's network has
still nothing listening but the add-on.

**The buffer's bounds**, declared in the manifest and MUST on the add-on:
`limits.stats_buffer` (default `P90D`, ceiling `P400D`) is how long a point
stays in the buffer whether or not Syndra has collected it; a size ceiling
of the add-on's choosing (the reference uses 1 M points) drops oldest first.
Syndra's first collection of a metric uses `since = now − stats_buffer`.
If Syndra does not collect for longer than the buffer (a long outage, a
target parked for a term), the window that fell out is reported on the next
collection as a `gap` with the reason "the add-on no longer holds this
window", from `since` to the buffer's oldest point, so Syndra draws a gap
and never a zero, and the add-on continues from what it still has. The
add-on's buffer is also what answers a collection while the app is off: the
points are already there, the app's absence is a gap, and nothing waits.

**Rejected: add-on push** (`POST /targets/{id}/stats` on the backend). It
needs an inbound route on the backend reachable from every add-on network, a
credential per add-on that can write to Syndra, a retry queue in the add-on
(which the README forbids for the reason that two queues disagree about what
is pending), and it makes "the add-on is the least trusted component" false
in the one direction that matters. Push buys lower latency, which statistics
do not need.

### 9.2 The data model, against OpenTelemetry and OpenMetrics

The two industry references agree on the vocabulary; this contract adopts
the vocabulary and refuses the machinery.

| Concept | Adopted as | Why / why not |
|---|---|---|
| metric name | `name`, lowercase, dot-namespaced, `[a-z0-9_.]`, the first segment is the kind or product (`storage.used_bytes`, `laser.job_seconds`, `inventory.checkouts`) | OTel naming (dot namespaces; OpenMetrics forbids dots), without a `_total` suffix: the `instrument` says it |
| unit | `unit` from a closed registry `registry/units.json`: `count`, `items`, `bytes`, `seconds`, `grams` | OTel has a unit field; this one is a closed plain-word list rather than UCUM (`By`, `s`, `g`), because the UI must format it (`seconds` renders as minutes or hours; there is no `minutes`). A new unit is one registry entry and one formatter case |
| instrument | `instrument: counter | gauge` (the word `kind` is the manifest's system kind and is not reused) | the two OTel instrument families every target has a use for |
| histogram, summary | **not adopted** | no target reports a distribution; an average is two counters (`sum` + `count`) and a maximum is a gauge |
| attributes / labels | `dimensions`, declared per metric in the manifest, ≤ 4 per metric, values are target vocabulary strings naming **resources, never people** | OTel attributes, OpenMetrics labels; declared, not free-form, so cardinality is bounded by contract. A home folder is its own metric (`storage.home_bytes`), never a dimension value (ruling O3, 12 Nov 2026) |
| temporality | **not OTel delta.** A point is the **complete total for its bucket**; equal keys replace | see below |
| timestamps | RFC 3339 `start`/`end` with the zone's offset: `start` a midnight in the manifest's `stats_zone` (an IANA zone, the add-on's `TZ`), `end` the next midnight (a calendar day: 23 or 25 hours across a DST change). A day's midnight is its first instant: 00:00, or, where a DST change skips 00:00, the instant the clocks resume (`2026-09-06T01:00:00-03:00` in America/Santiago) | OTel points carry instants (`start_time_unix_nano`/`time_unix_nano`), so a makerspace day is as expressible as a UTC one; RFC 3339 §5.6 and ISO 8601-1 carry a local midnight with its offset. One zone for every day Syndra counts (the 5 Nov one-zone rule, `repoguard/one_zone_test.go`): no "UTC day" anywhere (ruling O1, 12 Nov 2026) |
| resource / scope / instrumentation library | **not adopted** | the target and the add-on are the resource; `x-` keys carry anything else |
| exemplars, text exposition, push gateway, remote-write | **not adopted** | nothing scrapes this except Syndra |

**A point is the whole bucket, not a delta.** OTel's delta points are
additive: several partial deltas for one interval sum. An author who knows
OTel would send partials and Syndra would undercount, so this contract does
not claim delta temporality. A counter point is "in `[start, end)` this much
happened, in total"; a gauge point is "the last sample seen inside
`[start, end)`", one definition. Keyed by
`(metric, dimensions, subject, start)` (`end` is always `start + P1D`), and
that key is what makes ingestion idempotent: Syndra upserts by key and
never adds, so re-reading a window overwrites equal values with equal
values; and for the provisional window (after the metric's watermark)
Syndra **replaces** what it holds with what the reply carries, so a
provisional 3 that is truly 0 (a job cancelled, a log corrected) is
corrected by its absence. Inside an observed (non-gap) window, an **absent
counter point is zero and an absent gauge point is "no new sample"**; the
add-on SHOULD NOT send explicit zeros for a counter (300 members × 20
shares × 90 days would be half a million rows of nothing), while a gauge
may legitimately read 0. Why from the
target's log and not a cumulative counter: Prometheus scrapes cumulative
counters because a missed scrape then loses resolution, not totals, which
assumes a process that keeps a counter from boot and is scraped often; an
add-on derives every number from the target's own log (TrueNAS `audit.query`,
UniFi `system/logs`, Ruby's job history), it restarts, and the target is
sometimes off. **Rejected: cumulative** because the add-on would need a
durable running total per series across restarts, a second system of record
for a number the target already logs.

### 9.3 The wire

Manifest (under feature `stats`):

```json
"metrics": [
  {"name": "storage.home_bytes", "instrument": "gauge", "unit": "bytes", "label": "Home folder used",
   "per_subject": true, "against": "home_quota"},
  {"name": "storage.used_bytes", "instrument": "gauge", "unit": "bytes", "label": "Storage used",
   "per_subject": true, "dimensions": [{"name": "share", "label": "Share"}]},
  {"name": "storage.sessions", "instrument": "counter", "unit": "count", "label": "Storage sessions",
   "per_subject": true, "dimensions": [{"name": "share", "label": "Share"}]},
  {"name": "laser.job_seconds", "instrument": "counter", "unit": "seconds", "label": "Laser time",
   "per_subject": true, "dimensions": [{"name": "machine", "label": "Machine"}]}
],
"stats_zone": "Asia/Kolkata"
```

`per_subject` on a metric means "reported per person"; the field key
`per_person` keeps its meaning, "an operator may override for one person";
one word is not reused for two things. `against` names a `limit` field of
the same unit; the UI then renders the gauge as a meter against the
subject's **observed** limit (§3) with no product code, which is the
home-folder view today, generalised. `against` is allowed only on a
`per_subject` gauge with no dimensions (the schema says so): it means "this
value is a fraction of that limit", which holds for a person's home folder
and not for every other share, so a dimension-scoped meter would be a
predicate over product vocabulary the core would have to evaluate. The home
folder is its own metric (ruling O3, 12 Nov 2026). `stats_zone`, required
with the feature, is the IANA zone the add-on counts days in; Syndra does
not collect statistics counted in another zone than its own `clock.Name`
(ruling O1). The manifest's `limits` holds only
what the add-on knows: `subjects_per_plan`, `fresh_read_cooldown`,
`stats_buffer`. Syndra's own caps (`series_per_metric = 500`,
`max_reply_bytes = 1 MiB`) live in `registry/constants.json` and are never
declared by the least trusted component.

Request: `GET /stats?metric=laser.job_seconds&since=2026-11-07T00:00:00%2B05:30[&cursor=…]`,
one metric per call. No `fresh`: when the app is read for statistics is
the add-on's (§9.1), and "refresh the history" has no reader. Reply:

```json
{
  "contract_version": 2,
  "metric": "laser.job_seconds",
  "taken_at": "2026-11-10T07:40:00Z",
  "watermark": "2026-11-08T00:00:00+05:30",
  "points": [
    {"start": "2026-11-07T00:00:00+05:30", "end": "2026-11-08T00:00:00+05:30",
     "subject": "sub-7f3a", "dimensions": {"machine": "speedy-400"}, "value": 2520},
    {"start": "2026-11-07T00:00:00+05:30", "end": "2026-11-08T00:00:00+05:30",
     "dimensions": {"machine": "speedy-400"}, "value": 3660}
  ],
  "unbound": {"accounts": 2},
  "gaps": [{"start": "2026-11-08T00:00:00+05:30", "end": "2026-11-10T00:00:00+05:30",
            "dimensions": {"machine": "speedy-400"}, "reason": "The laser PC was off"}],
  "next_cursor": null
}
```

Rules, MUST on the add-on:

- `since` is honoured: no point with `end <= since`. A point whose `end` is
  at or before the metric's watermark is **final** and is never re-sent with
  another value; a point after it is provisional, may be re-sent, and the
  reply's provisional window replaces Syndra's (§9.2). A metric's watermark
  never moves backwards and never past a window the add-on may still fill
  (§9.1).
- A gap is one window of one metric, with optional `dimensions` (a gap per
  share that auditing does not cover is one gap per `share` value), keyed
  by `(metric, dimensions, start)`; `start`/`end` are the point vocabulary
  (one vocabulary, and `from`/`to` are SQL reserved words). A gap ending
  after the watermark is provisional: Syndra replaces every provisional gap
  of that metric on each collection, so when the PC comes back and the
  window fills, the gap is retracted and the points arrive under a
  watermark that had not passed them. A gap at or before the watermark is
  final: the add-on has decided it will never know.
- A point without `subject` is the target-wide (or per-dimension) total and
  includes everyone, bound or not; on a `per_subject` metric it is
  **required for every bucket that has any per-subject point** (so the
  per-person rows can be deleted at their retention and the total remains).
  A point with `subject` is on
  a `per_subject` metric and carries the Syndra subject id, never a local
  name.
- A window the add-on could not observe is a `gap` with a `reason` in the
  member's words, never a zero. "The PC was off" and "nobody used it" are
  different sentences (the README's honest-degradation rule, applied to
  numbers).
- Paged with a **keyset** cursor (`next_cursor` encodes the last
  `(start, subject, dimensions)` sent; no snapshot needed, because
  ingestion is upsert-by-key and the watermark is written last); a cursor
  the add-on already gave in the same collection (A, B, A) stops it,
  keeping the watermark;
  `taken_at` and `watermark` from the first page. `taken_at` is when the
  add-on's collector last read the app, not when it answered Syndra. No
  `current`: the watermark says what is final.
- A reply page fits the contract's published reply ceiling,
  `constants.max_reply_bytes = 1 MiB` (today the backend's private
  `maxResponseBytes`, `transport.go:76`, now in `registry/constants.json`
  so an author can size pages under it; the same ceiling bounds a
  `/subjects` reply).

### 9.4 What can be reported, and how it meets the reads that exist

| | What | Channel | Kept by Syndra |
|---|---|---|---|
| (a) target-wide over time | door unlocks per day, laser minutes, bytes stored | `stats`, points without `subject` | yes |
| (b) per person | a member's laser minutes this month, their storage used, their items on loan (count) | `stats`, `per_subject` points | yes, under §9.7 |
| (c) per resource | per door, per machine, per share, per item category | `stats`, a `dimension` | yes |
| (d) discrete events | who did what, when | `activity.get` (operation, `common/activity.json`), read live with `since` | **no** |
| (e) current holdings | what I hold right now, with ids and due dates | `holdings.list` (operation, `common/records.json`), read live | **no** |
| (f) current status | can I use it now, how much room is left | `storage.status` (operation, `common/status.json`), `GET /homes` (storage profile), read live | **no** |

**Two shapes, one principle:** Syndra keeps aggregates and never copies
records. The add-on derives the aggregates from the same log it answers
events from, so an operator who sees "14 sessions on Tuesday" can open the
events for Tuesday, live, if the add-on still has them; Syndra does not
become a second copy of the audit log.

**Rejected: one channel carrying events and metrics.** Events have identity
and are personal data at the finest grain; retaining them in Syndra would
make it the log of record for three products, which is the wholesale copy
§9.7 forbids. Keeping them live also keeps their retention the target's.

**The storage numbers, reconciled.** `storage.status.shares[].used_bytes`
is the measurement the channel's `storage.used_bytes` gauge reports once a
day per share, and `/homes` `used_bytes` the one `storage.home_bytes`
reports per home folder, against `home_quota` (two metrics since ruling O3,
12 Nov 2026: they answer two questions and may overlap). Decision: the live
reads **stay**, as the storage profile's *current* query, served by the
add-on from its own most recent read of the NAS (its mirror) with `taken_at`
and `current`, re-read from the NAS when `fresh: true` asks and the cooldown
allows (§4); the channel is the *history*; the UI shows the current number
with its `taken_at` and, beside it, the trend from the channel, never a
stored sample as "now". History and trends are new: today nobody can say
whether a member's usage grew. `storage.sessions` from the audit log is the
second TrueNAS metric and reports a `gap` for every share auditing does not
cover (`uncovered` today, as a gap reason).

### 9.5 Identity

A per-person point carries the Syndra subject id, resolved by the add-on
through the binding it recorded at apply (the same `boundAccount` lookup
operations use), never the target's local name. Usage by an account the
add-on cannot bind (an unmanaged local account, a visitor, a shared desk
login) counts in the target-wide and per-dimension totals, where the
resource's truth belongs, and is **never** reported per person; the reply's
`unbound.accounts` says how many such accounts contributed, and the target
page says "2 accounts Syndra does not manage also used the laser; they are
in the totals". No local name crosses the wire for an unbound account.

### 9.6 Storage, retention, cardinality, gaps, idempotency

- **Tables** (one additive migration, 000093, which also adds the
  per-target retention override as a nullable column on `targets`,
  `stats_subject_retention_days`, so it is not a fifth migration):
  `addon_metric_points(target_id, metric,
  subject TEXT NOT NULL DEFAULT '' /* '' = target-wide */, dims JSONB sorted,
  bucket_start DATE, value NUMERIC, observed_at)` with the plain primary key
  `(target_id, metric, subject, dims, bucket_start)` (a PRIMARY KEY cannot
  contain `COALESCE`; `bucket_end` is derived, `start + 1 day`);
  `addon_metric_gaps(target_id, metric, dims JSONB NOT NULL DEFAULT '{}' sorted, gap_start, gap_end, reason)`
  with the primary key `(target_id, metric, dims, gap_start)` (a PK column
  cannot be NULL, so a gap without dimensions is `{}`);
  `addon_metrics(target_id, metric, descriptor JSONB, watermark,
  last_collected_at, unbound_accounts, dropped JSONB)`, the stored meaning
  of each metric name (written at registration, never deleted, so a name
  never comes back meaning something else) beside its watermark and what
  its last collection dropped, by reason (ruling B3; points and gaps
  reference it, so a descriptor before any point is a constraint).
  `bucket_start`, `gap_start` and `gap_end` are makerspace dates.
- **Ingestion is one transaction per metric per collection**: the stored
  points and gaps of that metric with `end > stored watermark` (the
  provisional window) are **deleted and replaced** by the reply's, points
  at or before it are upserted by key (equal keys replace, nothing adds),
  and the metric's watermark is written **last, after the final page**; a
  failed page leaves the watermark where it was and the next collection
  re-reads. `since` is that metric's stored watermark; a metric's first
  collection uses `now − limits.stats_buffer`. Re-reading any window twice
  is a no-op, for points and for gaps. The stored watermark is the reply's
  floored to its makerspace midnight, capped at the start of today (today
  is still being counted, whatever the add-on says) and never moved back
  (6d-2).
- **Content validation, because the add-on is least trusted**: a point with
  a `subject` that has **no binding on this target** is dropped and counted
  (an add-on cannot write usage onto an arbitrary member's page); a point
  with `subject` on a metric not `per_subject` is dropped. A text **names a
  bound person** when it holds a bound subject's id, account name or
  `account_id` (non-empty) as a whole token, case-insensitively: one RE2
  pattern per collection, `(?i)(?:^|[^\pL\pN])(?:n1|n2|…)(?:[^\pL\pN]|$)`
  with each name `QuoteMeta`d (a pattern matching nothing when no binding
  has a name). A token boundary is the text's start or end or any
  character that is neither a Unicode letter nor a digit, so
  `home-JOHN_CENA`, `home_ana` and `john_cena's` name them while `makerlab`,
  `proj-30512` and `ME3051` do not name `lab` or `3051`; not `\b`, which in
  RE2 is ASCII-only and counts `_` as a word character. One rule for
  dimension values and gap reasons (display names wait for a cheap source,
  ruling B7): a point or gap whose **dimension** names a person is dropped;
  a gap whose **reason** does is kept, its reason replaced by the fixed
  "The add-on's reason named a person." (the gap is still true); both
  counted `person_in_dimension`. So is a point dropped that is not one
  makerspace day, or has no value or a negative one. Each is counted by
  reason on the metric's row.
- **A metric is never re-purposed**: a manifest that changes an existing
  metric's `instrument` or `unit` is refused at registration with the
  reason; a new meaning is a new name (§2 rule 6). Bucket width is not
  declared in this major, so it cannot change.
- **Retention** (owner decision 13): target-wide and per-dimension daily
  points 5 years; per-subject daily points 400 days (one academic year with
  a margin for "same time last year"), then deleted, not anonymised: the
  total they would anonymise into is required to exist as its own point
  (§9.3). A point older than its retention arriving late is dropped at
  ingestion. Gaps follow the points they cover: kept with the totals, five
  years. Retention of N days counts today: a per-person point is kept from
  today − (N − 1), so 400 days are 400 days and 30 are the same days as "the
  last 30 days". The daily retention (at start-up, then every 24 hours, so a
  shortened setting takes effect within a day) runs whether or not anything is collected
  (`STATS_COLLECTION_ENABLED` gates collection only, and a backend with no
  add-on registered still prunes what it holds). An operator may shorten the per-person retention of one target
  (1 to 400 days, the column's CHECK the ceiling's one place) and never the
  totals' (ruling O4, 12 Nov 2026: 400 days is the privacy ceiling on
  personal data, five years the history of a resource that names nobody,
  and totals erased are unrecoverable). One SQL definition
  (`statsSubjectKeptFrom`) serves the prune, ingestion and every per-person
  read: a person's windows start at `max(window, kept_from)` and are returned
  as `window_30d_from` / `window_12m_from`, which the page names its windows
  from; Insights' spread is clamped to `GREATEST(from, kept_from)` in its
  own query and returns that start, so a day the prune has yet to delete is
  never counted. No rollup job exists because every point is
  already a day.
- **Cardinality**: `constants.series_per_metric` (500) caps distinct
  `dimensions` sets per metric (subject does not count; it is bounded by
  members).
  Beyond the cap Syndra keeps the series it already knows, drops the rest,
  counts them (`dropped.series`), and the target page says "the add-on reports
  more {label} series than Syndra keeps". ≤ 50 metrics per manifest, ≤ 4
  dimensions per metric; a metric with an unknown unit or an undeclared
  dimension is dropped at registration and logged, never partially stored.
- **Backfill and gaps**: when a target comes back (the laser PC is on
  again), the add-on reads the target's log for the window it missed and
  writes those buckets to its buffer; because that metric's watermark had
  not passed them (§9.1), Syndra's next `since` still reaches them, the
  provisional gap is retracted, and the trend fills in. That is the second
  reason for points-from-the-log over cumulative scrapes: an offline window
  is recovered exactly as far as the target logged it. What the target did
  not log becomes a final gap once the add-on advances the watermark past
  it, stored, and drawn as a gap. A gap and a zero never render the same.
  Clock skew between the ends is already bounded at ±2 minutes by
  `signatureTolerance` (`transport.go:49`), which is far inside a day.

### 9.7 Privacy and access

Per-person usage is personal data and is treated as the most sensitive thing
this contract carries.

- **A member sees their own**, through `/me/targets`, as the storage page
  works today. Nothing else about the channel is visible to a member.
- **Admins see everything, directly** (owner decision 14, verbatim: "there's
  no need to log; make the usage and statistics available and visible to
  admins"). An operator with the admin role sees target-wide, per-resource
  and per-subject usage on the pages where it belongs: the person's page
  carries "What they have used" and "What they hold" with the page, as it
  carries the storage status today (`GET /users/{id}/targets`); the target
  overview and insights carry the totals and the per-dimension series. No
  expand-to-reveal step, no separate endpoint, no view audit. The
  operations log still records what an admin *does* (an operation they
  dispatch), as everywhere; reading is not an action.
- **No small-group suppression.** Every page that shows an aggregate is an
  admin page, and admins see per-subject series on the same pages, so
  hiding a total with three people in it would hide nothing from anyone
  who can see it; the threshold stays where it already lives, in
  `services/insights.go`, and is not extended here.
- **No ranking of people** is a UI choice, not a privacy control: insights
  shows distributions ("most members use under 2 GB") and the person page
  shows one person, because a leaderboard of members is not a view this
  makerspace wants on a wall, and an admin who needs one person's number
  opens that person. The copy guard keeps the words that would introduce
  one out of Syndra's own sentences.
- **No raw logs copied wholesale**: events and holdings are live reads (§9.4);
  Syndra stores points, gaps and watermarks.
- **Retention is a ceiling, not a target**: the operator may shorten the
  per-person retention per target on its System page (one write place,
  `PUT /targets/{target}/stats-retention`); the ceiling in §9.6 cannot be
  raised without a change here, and the totals' five years have no setting
  (ruling O4).
- **Declared or refused** (§9.6 content validation): a `subject` with no
  binding on this target, a `subject` on a metric not `per_subject`, a
  dimension value that names a person (§9.6, whole token): dropped,
  counted, logged, and the kit warns; a gap reason that names one is
  withheld, its gap kept and counted. Dimensions are resources, never
  people. A person's page carries only the gaps whose dimensions are `{}`
  or a subset of one of that person's own series (every key=value of the
  gap in that series: `{"share":"makers"}` is on their
  `{"share":"makers","protocol":"smb"}`), never a gap on a resource only
  somebody else used.
- **Add-on text is quoted, never trusted**: `label`, `reason`, `note` and
  raw `x-` content are rendered as escaped, length-capped quoted text; the
  copy guard runs on Syndra's own sentences around them, not on what an
  add-on may say at runtime. No real names in fixtures (WWE ring names,
  `sub-…` ids, `example.org`).

### 9.8 Where it surfaces

All of it is rendered from declared descriptors; a kind module is needed only
where a product needs a view the descriptors cannot express, and none of the
four targets does.

- **Member page** (`/me/targets`): per target declaring `stats`, a card
  "What I have used": each `per_subject` counter summed over the last 30
  days and the last 12 months with a sparkline; each `per_subject` gauge as
  its latest value with its day, as a meter when `against` is declared
  (storage used against the observed home limit); gaps footnoted ("no data
  9–10 Nov: the laser PC was off"). Beside it, "What I hold" from
  `holdings.list` and "Now" from `storage.status`, live with `taken_at`.
  Every card ends with a next step or "nothing for you to do".
- **Person page** (admin): the same card for that person, with the page,
  beside their storage status (§9.7).
- **Target overview** (admin): target-wide series over 30 days, broken down
  by each dimension; gaps hatched; the unbound-accounts line; "This add-on
  does not report statistics" when the feature is absent (not "no data").
- **Insights** (admin, Advanced view): every target's metrics grouped by
  unit, distributions over members for `per_subject` metrics; one person's
  numbers are on that person's page, not here (§9.7, a UI choice). Where
  this page sits in the navigation contract is a design-canvas question
  (`basic-advanced-ia`), not this change's.
- **Windows**: 30 days and 12 months, in makerspace days, the same day axis
  as every other count Syndra shows (sign-ins, the audit day counts); a term
  boundary when the directory defines one (not in this change).

### 9.9 Conformance kit and guide additions

Kit checks under `stats` (skipped when the feature is absent):

| Check | What it proves |
|---|---|
| descriptors | every metric validates (typos fail); unit in the registry; dimensions ≤ 4; `against` names a `limit` field with the same unit, only on a `per_subject` gauge with no dimensions; `per_subject` is boolean; `stats_zone` an IANA zone; `stats_buffer` ≤ `P400D` |
| points | every point in a `?metric=` reply belongs to that metric and names only declared dimensions; `start` a midnight in `stats_zone`, `end` the next; `subject` only on `per_subject` metrics; `value` numeric |
| totals | on a `per_subject` metric every bucket with a per-subject point has a subject-less total; every value ≥ 0; **counter**: the sum of the per-subject points ≤ the total; **gauge**: each per-subject point ≤ its bucket's total (ruling O2, 12 Nov 2026: OTel's counter is a monotonic sum, every event someone's, so the sum rule is an identity; the contract's gauge covers OTel's Gauge, where sums are not meaningful, and the additive UpDownCounter, so only "a part is never more than the whole" holds for both) |
| since | no point with `end <= since`; the watermark a midnight in `stats_zone`, ≤ the start of `taken_at`'s day; a second collection never moves it backwards; an unknown `metric` → 404 `UNKNOWN_METRIC` |
| idempotent | the same `(metric, since)` twice → the same points at or before the watermark, and the same final gaps |
| gaps | every gap has `start < end`, a `reason`, and only declared dimensions; a gap at or before the watermark is unchanged on the next collection |
| no people in dimensions | no dimension value equals the username or `account_id` of the account the kit created (`stats.no-people`, a MUST); none equals those of any other account `/subjects` lists (`stats.no-people.ids`, warn: a service account may share a resource's name) |
| unbound | with `--mutate`, usage by an account the kit created *outside* the binding (where the target allows it) appears in the total and in `unbound.accounts`, never as a per-subject point |
| buffer | `since` older than the buffer → a gap from `since` to the oldest point with the buffer reason, and points still answer |
| page size | no reply page exceeds `constants.max_reply_bytes`; a keyset cursor continues it with no duplicate and no omission across two pages |

Guide: §1 opens with the boundary rule; §3 gains `metrics[]` and
`limits.{stats_buffer, fresh_read_cooldown}`; §4 gains `/stats` with the
fixture inline, the MUSTs of §9.3, and the freshness contract for the reads
in its scope; §7 gains `result`, `effect` and the physical rules; §8 gains
"no local name on the wire for an unbound account", "a gap is never a zero"
and "the collector is yours: its schedule, its backoff, its respect for the
app's limits"; the reference add-on reports one counter and one gauge with
one dimension, buffers them with a watermark it holds back while its
in-memory target is marked off, honours `fresh`, counts `target_reads`, and
answers `holdings.list`.

## 10. The fourth target: the inventory system, as a test of generality

Owner, 10 Nov 2026: "one other add-on that I'll possibly add later is for our
inventory management system so that people can directly check items out and
have an overview of what all they have taken, maybe on their account." No
research exists; its API is treated as unknown. It is used here as a design
test, because it is the first target where the member *acts* through Syndra
and the action moves a thing in the world.

**The three halves, mapped:**

| Need | Mechanism | Core change |
|---|---|---|
| who may borrow at all | `{"name": "borrower", "role": "account"}` | none |
| which categories | `{"name": "categories", "role": "membership", "label": "May borrow from"}`, values from `GET /values/categories` | none |
| how many at once | `{"name": "max_items", "role": "limit", "unit": "items", "combine": "max", "unlimited": "0", "per_person": true}` | none |
| check an item out / in | operations `item.checkout` and `item.return`: scope `member`, `effect: physical`, `confirm: true`, params `[{name: item_id, type: string, required: true, format: target_id}]`, `result: common/records.json` (the record created or closed) | registry + policy entries, as for any operation |
| what I have taken | operation `holdings.list`: scope `member`, `effect: read`, no params, `result: common/records.json`; served through the one read path (§4), so the member reads it on `/me/targets` and an admin on the person page, with no operations-log row either way | registry + policy entries |
| overdue | `standing: {usable: true, waiting_on: "member", note: "1 item overdue", next: "Return it at the desk"}` on the subject | none |
| how much the lending is used | metrics `inventory.checkouts` (counter, `count`, dim `category`, `per_subject`) and `inventory.items_on_loan` (gauge, `items`, dim `category`, `per_subject`, `against: max_items`) | none |

**Why check-out is an operation and not desired state.** "Rene Dupree holds
the multimeter" has a steady state, which is the README's test for a field,
but the state is not Syndra's to desire: a member decides to take an item at
a shelf, the desk can lend and take back without Syndra, and converging
"holdings" would mean Syndra undoing a return the desk recorded. Holdings are
*observed* (read live, `records`), the act of taking is an *event*
(operation), and only the entitlement to take at all is desired state. One
line, three mechanisms that already exist.

**The gaps the contract had, closed in this change:**

1. *Operation replies had no generic shape*: the core carried typed pockets
   for one product's reports. → `result` by registry schema, three common
   shapes (§4). Without it `holdings.list` would have been a fourth pocket.
2. *A member-initiated operation that creates a record and names a target
   object.* Items are thousands, so `GET /values/item` would answer
   `enumerable: false`; the UI needs an input (typed or scanned), not a
   picker. → `params[].format: target_id` renders an input with a scanner
   affordance; the add-on answers the registered `UNKNOWN_RESOURCE` (404,
   `retry: never`, `detail` names the param), `NOT_ENTITLED` (403, the
   category is not in the subject's `membership`) or `LIMIT_REACHED` (409,
   the `limit` with its `unit` and the current count) so the UI can say
   which without product words.
3. *Nothing said what an operation does to the world, and an
   indeterminate outcome had no way to settle.* → `effect`, and the six
   `physical` rules (§4): `preview: true` (unrecorded, no `call_id`) gives
   the member the item's label before they confirm with a fresh `call_id`;
   at-most-once with nobody re-executing and `CALL_UNSETTLED` for a retry
   in the meantime; settlement through `GET /calls/{call_id}` (`recorded` ·
   `in_doubt` · `unknown`), with the member shown "we are checking whether
   it went through" and then the outcome, and a human named only after
   `constants.in_doubt_bound`; natural idempotency on the item; per-subject
   serialisation; `constants.call_memory` of call records.
4. *A target that has no entitlement concept at all* (a plain lending ledger
   with items and loans and no borrower profile). **Not covered.** An
   add-on holding `borrower`, `categories` and `max_items` in its own store
   would make "observed state" the add-on's own claim and every page would
   say "on the target" meaning "in the add-on". The inventory API is
   unresearched; if it turns out to have no borrower profile, that is a
   design question for its own change, declared in the manifest so surfaces
   can say where the state lives, not a pattern this contract permits by
   default. (The reference add-on's in-memory *target* is a target, not an
   add-on-held store; the earlier draft conflated the two.)
5. *Overdue as `standing`*: fits as is. One `waiting_on` per subject; when
   two things wait on the member (overdue item and unset password), the
   add-on writes one `note` naming both and one `next`.

**Extra safety where an action touches physical stock:**

- The add-on's credential can lend and return; it is issued without the
  scope that deletes items, edits stock levels or writes off losses, on the
  §4 rule (the kit's denylist for kind `inventory`: whatever the product
  names those scopes).
- `item.checkout` is refused by the add-on whenever `health.lifecycle` is
  not `active` or the target is not `reachable` (never a queued checkout:
  the member is standing at the shelf now, or not).
- The limit and the category are checked before the call, so a refusal is
  Syndra's words and no half-record exists on the target.
- A return never depends on Syndra: the desk records it in the inventory
  system and Syndra observes it on the next read.
- The operations log records `item_id` and the outcome; the audit of who
  checked out what is Syndra's own (it made the call), not a copied log.

**Mapping sketch for the guide** (§8 item 12 gains it): `kind: inventory`
declared, no profile (owner decision 19); features `operations, results,
calls, stats`; the schema and
operations above; `account_id` = the product's member id, `display` = its
member name; binding by the product's external-reference field if it has one,
else by email at adoption; `standing` from overdue; the first `GET /subjects`
lists today's borrowers as unmanaged, as every target does. Unverified until
researched: whether the product has a borrower profile, whether its loan
records carry an id, whether a checkout can be made on behalf of a member by
an API credential, and whether returns are events or state.

## 11. Extensibility, scalability, refinement

**Extension points, each with one owner:**

| Point | Lives in | Changing it needs |
|---|---|---|
| a new field on a target | the add-on's manifest | nothing elsewhere; the role says what to do |
| a new field role | `registry/roles.json` + resolver + UI | a feature (`roles.<name>`) and a core change: rare by design |
| a new optional route or request field | `registry/features.json` + schema | a feature; add-ons opt in |
| a new operation | `registry/operations.json` + backend policy | two entries, as today; its reply is one of the three common `result` shapes or its own |
| a new metric | the add-on's manifest `metrics[]` | nothing elsewhere; stored and rendered from the descriptor |
| a new unit | `registry/units.json` + one formatter case in the UI | one entry, one case |
| a new per-person record list (holdings, bookings, jobs in a queue) | a read operation with `result: common/records.json` | registry + policy; the renderer exists |
| a new effect class | `registry/operations.json` vocabulary + the backend's retry rule | a core change: rare by design (`physical` was the last) |
| a new kind word | `registry/kinds.json` (+ its denylist) | one registry entry, which is a contract release compiled into the next backend build (no core *code*); a profile and a UI module only when a surface needs them |
| vendor data | `x-<product>` keys at the top of the manifest and inside `state`, `standing`, `health`, `result`, points | nothing; the core ignores, the kind module may read |

`x-` rule: any key beginning `x-` is ignored by the core, validated only as
JSON, never used for a decision, and shown raw (quoted, escaped,
length-capped) on the add-on's target page under "What the add-on also
says".

**The core keeps no product payload type.** After slice 6 the backend
compiles no struct shaped like one target's report: `OperationResult`'s
typed pockets are read only from an add-on without `results` and go at the
next major, the `smb://` builder is in the storage kind's view, and the
product words are in `addons/truenas` only. Two guard tests keep it so: the
literal guard of slice 2, scoped to `services/`, `handlers/`, `addons/` and
`drift/` at manifest-lookup sites (`"group"` and `"enabled"` are legitimate
in `zitadel/client.go`, `models/models.go`, `services/roles.go` and
`claim_probe.go`, so a repo-wide ban is unbuildable), and the **boundary
guard**: an HTTP client (`http.Client`, `http.NewRequest`, `net.Dial`) is
constructed in `backend/internal` only in an explicit allowlist of files
(no line numbers; they go stale), each with a one-line reason:
`addons/transport.go` (the pipe), `addons/credentials.go` (the pinned-TLS
client, `newAddonClient`), `zitadel/client.go`, `zitadel/token.go` and
`zitadel/deps.go` (the Zitadel client), `auth/jwt.go` (JWKS),
`handlers/deps.go` (the probe client), `handlers/claim_probe.go` (the claim
probe) and `handlers/claim_watch.go` (the dead-man ping to an operator-set
URL, through `http.DefaultClient`). There is no Google HTTP client in
`backend/internal`. This *guards* "Syndra talks to nothing but add-ons"; it
does not prove it, because a source scan cannot see where an operator-set
URL points, and the kit cannot see Syndra's side at all.

**The fifth-app test.** Re-read against TrueNAS, UniFi Access, Ruby, the
inventory system and statistics, the contract needs a core change for
exactly two things: a new field role and a new effect class, and both are
rare by design. Tried on two apps nobody has researched: a 3D-printer queue
(`kind: machine`; an account; printers as `membership`; a `limit` in
`seconds`; a counter `printing.grams` with a `material` dimension,
`per_subject`, in `grams`, which is in the unit registry for exactly this
reason; "my jobs" as `records`; `job.cancel` as a member operation with
`effect: account`) and a room-booking system (a kind word, `booking`, is
the one registry entry it needs; an account; rooms as `membership`; a
`limit` in `count`; "my bookings" as `records` with `since`/`due`;
`booking.cancel` with `effect: account`; `standing` for a no-show hold).
Both map with no core change; a loan period in `days` would be the next
unit entry. The one thing neither can express is a member *creating*
desired state (booking a room is an operation that creates a record, as
checking an item out is), which is by design: the member acts, the add-on
translates, Syndra desires only entitlements.

**Scalability.**

- *Many subjects:* **deferred** (owner decision 27). Roughly 300 members
  fit well inside `max_reply_bytes`, and a snapshot cursor needs mirror
  generations in the TrueNAS add-on for no present reader. When a target
  needs it, `subjects.paged` adds `?cursor=` and `next_cursor` as a
  **snapshot cursor** that pins a generation of the add-on's mirror (so a
  mirror that changes between pages cannot lose or duplicate a subject), an
  expired cursor answers 410 `CURSOR_EXPIRED` and the backend restarts the
  scan, an interrupted page set is `truncated`, and the backend treats a
  page set as one read (`current` and `taken_at` from the first page).
  Until then every add-on answers everything with `truncated` as today.
  `/stats` pages from the start, with a keyset cursor (§9.3), which needs
  no snapshot.
- *Many subjects per plan:* `limits.subjects_per_plan` in the manifest
  replaces the add-on's private cap and has one owner: the backend chunks
  below it and the acknowledgement rule is the backend's; the add-on only
  refuses a plan above it (`PLAN_TOO_LARGE`), and
  `SCOPE_ACKNOWLEDGEMENT_REQUIRED` leaves the add-on.
- *Many targets:* nothing changes; registry, breaker and pipe are per target
  already. A 429 with `Retry-After` is `unreached` as today; no
  `requests_per_minute` is declared, the 429 already says it.
- *Slow targets:* `/plan` may answer from the mirror (`current: false`) as
  today; `/apply` may not. Every read in the freshness scope may, and says
  so with `taken_at` and `stale` (§4).
- *Many points:* the add-on's buffer is bounded (§9.1), Syndra's table is
  keyed by day (§9.6), collection is per metric and paged under
  `max_reply_bytes`; a target that reports more series than the cap is
  truncated and says so, never slow.
- *Shape and version refusals are never the target's fault:* a 422
  `CONTRACT_VERSION_MISMATCH`, a 400 `UNKNOWN_REQUEST_FIELD` (the strict
  decoder's own code; a plain `BAD_REQUEST`, which is also a missing
  `call_id` or `subject`, stays `rejected`, or a malformed row would
  requeue for ever), or a 404 on a route the backend called under a feature
  the add-on no longer serves (there is no `UNKNOWN_FEATURE` code: an
  undeclared feature's route simply answers 404) is classified `unreached`
  and triggers a manifest refresh, never `target_refused`. The backend
  classifies by `code`, and **falls back to the v1 `error` key for
  classification only** until the next major, because a v1 add-on writes
  `{"error": "<CODE>"}` (`server.go:396-410`) with no `code` and no
  problem+json, and without the fallback a v1 400 would fall to `rejected`
  and a v1 503 lifecycle refusal to `indeterminate`. This protects the
  **new** backend from a v1 add-on (the rollback shape) and from an add-on
  rollback while the backend still caches a manifest declaring features the
  add-on no longer has; it cannot protect the *old* backend from a v2
  add-on during the deploy window, because the old backend runs v1
  `classifyStatus`, which is why the deploy procedure (§12, 1.10) stops the
  backend first.

**Refinement.** The contract has a `CHANGELOG.md`; every entry names the
feature or major, the schema files touched, and the fixture that changed.
A deprecated feature is listed with its replacement and survives one major.
Nothing is ever re-purposed.

## 12. Migration: seven slices, each shippable and reversible

| # | Slice | Ships | Reverts by |
|---|---|---|---|
| 0 | **Pin today.** Schemas for all 12 messages at contract 1 exactly as they are, the v1 error body included; a fixture for every message that has none today; two-ended assertions; validator dep; `errors.json` and `operations.json` written from the code. No wire change. | files, tests | deleting files |
| 1a | **The contract-2 schemas, alone.** Every schema, fixture and registry of major 2 (manifest v2 with roles, kinds, features; the per-message reply fields of the §4 table; the RFC 9457 envelope; the request-identity objects; the slice-6 feature schemas `stats.json`, `call.json`, `common/{stale,records,activity,status}.json`) and the `CHANGELOG.md` entry. No code. | files | deleting files |
| 1 | **Contract 2 on the wire.** `features`, `kind` (closed list), `role` in the manifest; tolerant manifest decode, role changes refused (§2); `call_id` + `contract_version` on every write, header removed, request identity and the four rules of §5; the raw query signed; `standing`; `account_id` in both tables, the unique index on bindings only; the problem+json envelope, lifecycle refusal by code, shape refusals and feature-route 404s as `unreached`; `targets.field_roles` persisted; the add-on's store written **additively** (bindings carry both `uid` and `account_id`, call entries with and without a digest both readable), so v1 opens a v2 store. The deploy order lives in `deploy.yml` and `_staging.yml` **in this PR** (a push runs the workflow file at the pushed commit): `compose stop backend` (the propagation runner is in-process in `backend`, so it stops with it; never the GitHub self-hosted runner, which runs the job), `compose stop truenas-addon`, copy `state.db` out of `truenas_addon_state` as a fallback (never `mutations.log`, whose record format is fixed and field-tagged, `mutationlog.go:49-80`, so v1 reads a log v2 extended), `up -d --build`. The kit (slice 4) runs against the v2 add-on **before** this merges. | backend + add-on | redeploying both at the previous tag, **no store restore** (the store is additive; the copy is a fallback only if v1 cannot open the file, which would rewind bindings and make a window-created account `target_refused`); the re-runnable backfill covers rows v1 wrote meanwhile |
| 2 | **The core forgets the names.** Every caller in §3 derives from `targets.field_roles`; the `smb://` builder moves to the storage kind's view; the `000087` CHECK becomes the verified target-agnostic form; the scoped literal guard and the allowlisted boundary guard. One migration (the CHECK swap; `field_roles` itself lands in slice 1). | backend | revert of a backend-only PR; the CHECK swap is reversible because the new form accepts every row the old one did |
| 3 | **The UI reads roles.** `fieldsByRole`, the storage kind module, copy from `label`/`unit`/`note`; the forty literals go; a lint guard. | ui | revert of a UI-only PR |
| 4 | **The kit and the reference add-on, core only.** Built against the slice-0 and slice-1a schemas; CI runs the kit against the reference; it runs against the v2 TrueNAS add-on on staging before 1.10 ships. The feature checks and the reference's feature halves are slice-6 work. | new modules | removing them |
| 5 | **The guide.** `addons/AUTHORING.md`, README route tables corrected, mapping sketches (UniFi, Ruby, inventory). | docs | n/a |
| 6 | **Statistics, records, freshness, calls: four features, all additive.** `results` (§4: `result` by registry schema, the three common shapes, `activity.get` and `storage.status` moved onto them, the one read path for `effect: read`), `freshness` (§4: `fresh`, `cached`, the cooldown, `health.target_reads`, the drift sweep's subjects read sending `fresh`), `calls` (§4: `GET /calls/{call_id}`, `preview`, `CALL_UNSETTLED`, `secret_result`, with `effect` in the registry), `stats` (§9: the TrueNAS collector and buffer, `storage.used_bytes` and `storage.sessions`, per-metric collection, tables, retention, content validation, the member card, the person page's card, the target overview, insights); the reference add-on's feature halves and the kit's feature checks. The TrueNAS add-on declares each feature only when its half is built. | backend + add-on + ui, each behind its feature | the add-on not declaring the feature; the tables stay empty |

Slice 0 is the one that must land first and alone: a schema written after the
wire changes pins what the code happened to do. Slice 1a (schemas only)
follows, then slice 4 is built against 0 and 1a, so that 1 is deployed only
after the kit has passed against the v2 add-on. Slices 2 and 3 are
independent of each other and both depend on 1. Slice 6 depends on 1
(features) and on 3 for its UI (`fieldsByRole` gives `against` its limit);
its four features are additive and nothing in 0–5 depends on them, so major
2 ships before they are built and none of them is ever a major 3.
`subjects.paged` is deferred past all seven until a target needs it
(decision 27).

## Owner decisions

Each is genuinely the owner's; the recommendation is mine. Decisions 1–10
were answered "all yes" on 10 Nov 2026 (7 re-read under 23). Decisions
11–23, as *revised* after the first Opus review of 10 Nov 2026, were
answered "all yes" the same day, with 14 decided by the owner in his own
words and 3 made final. The second Opus review (10 Nov 2026) refined the
mechanics under 18, 21 and 22 without changing what was decided (each says
so inline) and raised 24–27, which the owner accepted as recommended the
same day ("do what fable recommends"). Nothing is open.

1. **Major 2 now, in one coordinated deploy of both containers.** Recommend
   yes. The alternative, stretching 1 with reply-side additions only, cannot
   unify idempotency or replace `type`/`lifecycle` with `role`.
   **Accepted 10 Nov 2026.**
2. **The backend supports one major, never two.** Recommend one. Build the
   window the day an add-on the owner does not deploy needs it.
   **Accepted 10 Nov 2026.**
3. **The role vocabulary and its names:** `account`, `channel`, `membership`,
   `flag`, `limit`. Recommend as proposed; names are the only part worth
   bikeshedding and they appear in every manifest forever.
   **Accepted in principle 10 Nov 2026; the final names wait on a short
   naming discussion and these five stay as the working names until then.
   `feature` is excluded from any candidate list: the manifest's `features`
   list already owns the word (§3).** The owner has raised `toggle` for
   `flag`. Recommend keeping `flag`: `toggle` is a UI word that implies the
   member flips it, and `grant` (the other candidate) collides with Zitadel
   grants everywhere in this codebase. One simplification is on the table
   if the owner wants four roles instead of five: a `flag` is a
   `membership` with the single value `"true"`, and the UI could render a
   one-value membership as a switch; I would not take it, because `any`
   versus `union` is a role's worth of behaviour and the TrueNAS `home`
   field's label ("Gives a home folder") reads as a flag, not a set.
   **Final 10 Nov 2026: `account`, `channel`, `membership`, `flag`,
   `limit`.**
4. **`santhosh-tekuri/jsonschema/v6` in test scope of backend, add-on and
   kit.** Recommend yes; one dependency, the only complete Go 2020-12 validator.
   **Accepted 10 Nov 2026.**
5. **`addons/reference` as a maintained module** (the kit's self-test and
   the guide's worked example). Recommend yes; without it the kit tests
   itself against TrueNAS only, which is one target and needs a NAS.
   **Accepted 10 Nov 2026.**
6. **`password.set` stays the member credential operation; a PIN or card
   gets its own id when a target needs one.** Recommend keep; renaming
   touches the member page, the policy and the operations log for no reader.
   **Accepted 10 Nov 2026.**
7. **UniFi and Ruby are core-only kinds for now**; a profile is written
   when a member surface needs doors or materials. Recommend yes; a profile
   with no UI module is a schema nobody reads. **Accepted 10 Nov 2026**,
   with one correction under decision 23: "core-only" means *declared kind,
   no profile*, not `kind` absent, because an absent kind would have let
   the `edit:space` denylist never fire for the one target that can unlock
   doors.
8. **The kit's `--mutate` run against staging TrueNAS** with a throwaway
   subject, operator's hand. Recommend yes, once per contract change.
   **Accepted 10 Nov 2026.**
9. **Unlisted values preserved as a MUST with no knob** (§3). Recommend the
   MUST; a `replace` mode is how a hand-assigned door policy disappears.
   **Accepted 10 Nov 2026.**
10. **`uid` → `account_id` (text) on bindings and the apply reply,** with a
    two-release column swap. Recommend yes; UniFi and Ruby ids are UUID
    strings and an int column would force the add-on to invent a number.
    **Accepted 10 Nov 2026.**

Statistics, records and the fourth target (10 Nov 2026):

11. **Two collectors, one boundary** (§9.1), *revised*: the add-on reads
    the app on its own schedule into its own bounded buffer with a
    **watermark per metric** that it never advances past a window it may
    still fill; Syndra collects from the add-on per metric, since that
    metric's mark, and commits each watermark last; the add-on never calls
    Syndra. Recommend yes;
    the push alternative needs an inbound route, a write credential and a
    queue, and breaks the boundary rule in the one direction that matters;
    a single watermark would have lost every backfilled window.
    **Accepted 10 Nov 2026.**
12. **Daily buckets, whole-bucket totals, counters and gauges, no
    histograms** (§9.2), *revised*: every bucket is a UTC day in this major
    (`stats.hourly` is a later feature); a point is the complete total for
    its bucket and equal keys replace (not OTel delta, which is additive);
    `instrument: counter | gauge`; declared dimensions ≤ 4 naming resources
    only; a closed unit registry (`count, items, bytes, seconds, grams`).
    Recommend yes; daily is what a makerspace reads and removes the rollup
    job and two double-counting hazards; cumulative counters need a running
    total the add-on would have to keep across restarts; no target has a
    distribution to report. **Accepted 10 Nov 2026.**
    *Amended 12 Nov 2026 (ruling O1, slice 6d):* the bucket is a
    **makerspace day**, not a UTC one: a calendar day in the add-on's
    declared `stats_zone`, which must equal Syndra's one zone (`clock.Name`);
    points and gaps start at midnights there, RFC 3339 with the offset. "UTC"
    above was a bulk acceptance never weighed against the 5 Nov one-zone
    rule, and a UTC exception (`clock.UTCDay`) would re-admit by name the
    bug that rule exists for; the OTel data model carries instants, so a
    local day is as standard as a UTC one. No "UTC day" anywhere.
13. **Retention** (§9.6), *revised*: no rollup; target-wide and per-dimension
    daily points 5 years; per-subject daily points 400 days, then deleted
    (the total is required to exist on its own); late points past retention
    dropped. Operators may shorten per target, never lengthen. Recommend
    these numbers; 400 days is "the same week last year" and no more.
    **Accepted 10 Nov 2026.**
    *Amended 12 Nov 2026 (ruling O4, slice 6d):* the setting shortens the
    per-person retention only (`stats_subject_retention_days`, 1 to 400);
    totals and gaps keep five years with no override.
14. **Who sees per-person usage** (§9.7): a member their own; admins
    everything, target-wide, per-resource and per-subject, directly on the
    person page, the target overview and insights, with no view audit, no
    expand step, no separate endpoint and no small-group suppression; no
    ranking of people as a UI choice. **Decided by the owner 10 Nov 2026,
    verbatim: "for 14 there's no need to log; make the usage and statistics
    available and visible to admins."** My earlier recommendation (audited
    reveal, k=5) is withdrawn; the owner's rule is simpler and the pages
    that show aggregates are admin pages anyway.
15. **Unbound accounts** (§9.5): counted in the totals, reported as a count,
    never named on the wire, never a per-subject point. Recommend yes; a
    local name for an unmanaged account is personal data Syndra has no
    binding for. **Accepted 10 Nov 2026.**
16. **Events stay live reads, never stored** (§9.4): `activity.get` keeps
    its shape under `common/activity.json`; Syndra keeps points, gaps and
    watermarks. Recommend yes; retaining events makes Syndra the log of
    record for three products. **Accepted 10 Nov 2026.**
17. **Holdings are records, not statistics** (§9.4, §10): `holdings.list`
    as a member read operation with `result: common/records.json`, read live
    by the member and, on the person page, by an admin; the count is a
    gauge on the channel. Recommend yes; a list of things with ids and due
    dates is observed state, not a number over time. **Accepted 10 Nov 2026.**
18. **`effect` in the registry; `physical` at-most-once with a settlement
    path** (§4, §10), *revised*: `preview: true` before confirm, nobody
    re-executes an indeterminate call, `GET /calls/{call_id}` (feature
    `calls`) answers `recorded | in_doubt | unknown`, natural idempotency on
    the item, per-subject serialisation, call memory ≥ 30 days, refused
    unless the target is reachable and `active`. Recommend yes; a retried
    checkout is two items off the shelf, and an unsettleable one is a
    member at a shelf with nobody to ask. **Accepted 10 Nov 2026.**
    Refined after the second review, same decision: `preview` is gated by
    `calls`, unrecorded and without a `call_id`; the confirm carries a
    fresh one; the retry-meanwhile code is `CALL_UNSETTLED`; the `in_doubt`
    bound is `constants.in_doubt_bound`; `unknown` means "no effect
    recorded"; `secret_result` redacts a remembered reply.
19. **The inventory system declares `kind: inventory` with no profile**, as
    decision 7 corrected; a profile when a member surface needs item detail;
    an inventory product with no borrower profile at all is a question for
    its own change, not a pattern this contract permits. Recommend yes; its
    API is unresearched and the core already carries everything the sketch
    needs. **Accepted 10 Nov 2026.**
20. **`result` by registry schema, under feature `results`**, *revised*:
    additive, so major 2 ships without it; the typed pockets are read only
    from an add-on without the feature and go at the next major; `records`,
    `activity` and `status` as the common shapes. Recommend yes; it is the
    last product type the core compiles, and as a feature it can wait for
    the inventory target without ever forcing a major 3. **Accepted 10 Nov 2026.**
21. **Freshness as `fresh: true`, under feature `freshness`** (§4),
    *revised*: on `GET /subjects`, `GET /values/{field}`, the storage
    profile's reads and `effect: read` operations; never on `/plan`
    (the planner's re-plan included), `/apply` or `/stats`; `current` is
    defined once ("a read that began after this request arrived"), a
    mirror answer is `current: false, stale.reason: cached`, and
    `current == false` ⇔ `stale` on every reply the §4 table marks; the
    drift sweep's `/subjects` read is the one scheduled sender (a cooldown
    answer is a skipped pass), a person's "Refresh" the other; one name,
    `taken_at`, with `reach.read_at` renamed *to* it, `homes` gaining it
    (its `usage_read` becomes the completeness flag `quota_read`, per the
    §4 table) and the reply fields listed per message; `limits.fresh_read_cooldown`
    declared; `health.target_reads` required with the feature so the kit
    can prove it. Recommend yes; Syndra states the need, the add-on knows
    its app, and the planner is never cooled down. **Accepted 10 Nov 2026**
    (refined after the second and third reviews without changing what was
    decided).
22. **Sign the query string in major 2** (§4). Recommend yes; the pinned
    TLS already makes on-path tampering moot, so this is completeness, not
    a threat response. **Accepted 10 Nov 2026** as a canonical (sorted,
    re-encoded) form; the second review showed that form was miscited and
    underspecified, and decision 24 replaces the *form* with the raw query
    as sent. What was accepted, that the query is inside the MAC, stands.
23. **A closed kind vocabulary, declared by every add-on** (§4):
    `storage`, `access`, `inventory`, `machine`; the credential-scope
    denylist and the UI module key on it; a profile is optional per kind;
    a new word is a registry entry. Recommend yes; with `kind` absent the
    `edit:space` denylist would never have fired for UniFi.
    **Accepted 10 Nov 2026.**

Raised by the second review (10 Nov 2026), open:

24. **The query is signed raw, as sent** (§4): `ts.method.target.body`
    with `target` = the escaped path as sent + `"?" + RawQuery` when
    non-empty, RFC 9421's `@query`; no sorting, no re-encoding; three
    vectors. Recommend yes; two containers on one host have no intermediary
    that rewrites a query, and SigV4's seven canonicalisation rules are
    exactly what a Python add-on built on `urlencode` gets wrong on its
    first `cursor`. Replaces the form in 22. **Accepted 10 Nov 2026.**
25. **The error envelope is RFC 9457 `application/problem+json`** (`type`
    as a `urn:syndra:addon:error:<CODE>`, `title`, `status`, `detail`, with
    `code` and `retry` as extensions), on every non-2xx. Recommend yes;
    major 2 is the cheap moment, every HTTP client library already parses
    it, and the add-on's `writeError` changes four field names. **Accepted 10 Nov 2026.**
26. **The target's id is `account_id`, not `external_id`,** on the wire and
    in the new column (§5). Recommend yes; SCIM's `externalId` is the
    *client's* id (here the Syndra subject id, which UniFi stores as
    `employee_number`), the target's id is SCIM's `id`, and the existing
    column is already `account_uid`. **Accepted 10 Nov 2026.**
27. **`subjects.paged` is deferred out of slice 1** (§11) and built when a
    target needs it. Recommend yes; ~300 members fit well inside 1 MiB,
    and a snapshot cursor means mirror generations in the TrueNAS add-on
    for no reader today. `/stats` pages from the start with a keyset
    cursor, which needs no snapshot. **Accepted 10 Nov 2026.**
