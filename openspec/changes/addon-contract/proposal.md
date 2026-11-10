# The add-on contract, declared once

Architecture-deepening item 6, widened by the owner on 10 Nov 2026 to every
add-on there will be, not only TrueNAS. Design: [design.md](design.md). Build:
[tasks.md](tasks.md). Research: [research/](research/).

## Why

The backend and the TrueNAS add-on agree about twelve routes, and nothing but
four fixtures and two people's memory makes them agree. The inventory
([research/contract-1-inventory.md](research/contract-1-inventory.md), 10 Nov)
found the shape of the next defect already in place:

- **Four of twelve messages are pinned.** The manifest, health, subjects,
  values, reach, settings, homes, lifecycle and the plan reply have no fixture.
  The defect class this project keeps meeting, two internally consistent
  definitions of one thing, is open on eight messages.
- **Field meaning is hardcoded on three floors.** `group`, `home`, `home_quota`,
  `enabled`, `smb_enabled` appear by name in `services/entitlement.go`,
  `handlers/{my_storage,person_storage,target_homes,mappings}.go`,
  `drift/addon.go` and in forty places in `ui/src`. `smb_enabled` is a
  *lifecycle field of the core resolver*: the core knows what SMB is, which
  `addons/README.md` names as the exact thing that must never happen.
- **Idempotency travels two ways.** Body `call_id` for apply and operations,
  `Idempotency-Key` header for settings, nothing for lifecycle. The header is
  outside the signature.
- **The error vocabulary is defined on one side.** Twenty-five add-on error
  codes; the backend matches one (`PLAN_STALE`).
- **The version is a single integer checked by equality**, so every new
  request field is a breaking change and there is no way to say "this add-on
  can page its subjects" without bumping everybody.

Two more targets are researched and both break the TrueNAS assumptions: UniFi
Access (door policies replaced whole by `PUT`, physical NFC enrolment, a
"provisioned but cannot enter yet" state, endpoints that unlock doors) and
Trotec Ruby (an `active` switch, roles and groups, a Windows PC that must be
on). A third party, or another agent, building either against today's
contract would have to read the TrueNAS source to learn it.

Widened again by the owner on 10 Nov 2026, three ways. **Statistics:** "a
proper channel for the app → add-on → Syndra for statistics reporting, be it
overall usage, per user usage, or any other statistical data." Today the
only usage numbers (a member's storage used per share, home folders against
their quota) are read live on demand and forgotten; nothing has a history.
**A fourth target:** the makerspace's inventory system, "so that people can
directly check items out and have an overview of what all they have taken",
which is the first target where a member acts through Syndra and the action
moves a thing in the world. **The boundary rule, stated:** "it's the add-on's
job always to ever interact with the external app or service, never Syndra
directly", with "Syndra should be able to force a refresh" designed inside
it. The code already holds the rule; the contract did not say it.

## What Changes

- **One source of truth in `addons/contract/`:** a JSON Schema 2020-12 file per
  message, an OpenAPI 3.1 document naming every route, a registry of error
  codes and of operation ids, and a golden fixture for every message. Both
  ends' Go types are asserted against the schema and the fixture from their
  own suites; neither end imports the other and no Go module is shared.
- **Contract major 2.** The integer stays and stays equality-checked; it is
  the major. Additive evolution moves to a `features` list in the manifest:
  the backend sends a field or calls a route only to an add-on that declared
  the feature that introduced it, so request decoding stays strict and the
  contract still grows without a bump.
- **The manifest declares what a field is, not only what it is called.** Every
  entitlement field carries a `role` from a closed vocabulary: `account`,
  `channel`, `membership`, `flag`, `limit`. The resolver, the drift sweep, the
  mapping validator and the UI derive their behaviour from the role. The core
  stops naming `enabled`, `smb_enabled`, `group`, `home` or `home_quota`
  anywhere; `WithdrawsOnly` and "the channel goes off with the account" become
  rules over roles and work unchanged for UniFi's `status` and Ruby's `active`.
- **Kinds.** A manifest names the kind of system it fronts (`storage` today).
  Core routes serve every kind; a kind profile adds the routes and shapes only
  that kind needs (`reach`, `homes` for storage). A new kind adds a profile and
  a UI module and changes nothing in the core.
- **One idempotency mechanism:** body `call_id`, on every write, inside the
  signature. The header goes.
- **A generic pending-human state** on every subject (`standing`): usable, or
  waiting on the member, an operator, or the target, with the next step in
  words. Replaces the TrueNAS-shaped `password_set`.
- **A conformance kit** (`addons/conformance`): one binary that drives any
  add-on black-box over the real transport and reports MUST failures. Its
  self-test target is a reference add-on with an in-memory target, which is
  also the author's worked example.
- **An author's guide** (`addons/AUTHORING.md`) complete enough that an agent
  with no other context builds an add-on that passes the kit.
- **The boundary rule as a MUST.** Syndra never calls a target; the add-on
  alone holds the target's address and credential and alone talks to it, on
  its own schedule. Syndra talks only to add-ons.
- **Freshness** (feature `freshness`). `current` means one thing, "read
  from the app after this request arrived"; a mirror answer says
  `current: false, stale.reason: cached`. The drift sweep's subjects read
  and a person's "Refresh" send `fresh: true`; the add-on decides
  how to meet it, with single-flight and a declared cooldown, and answers
  the last known value with a `stale` reason instead of an error. Never on
  a plan, an apply or the statistics collection; never a bulk mechanism.
  Which reply carries which of `taken_at`/`current`/`truncated` is listed
  per message, as the wire really is.
- **A statistics channel** (feature `stats`): the manifest declares metric
  descriptors (name, unit, counter or gauge, dimensions, per-subject, a
  limit it is measured against); the add-on collects from its app on its
  own schedule into a bounded buffer with a watermark per metric; Syndra
  collects from the add-on, upserts whole-day totals idempotently, expires,
  and renders any add-on's statistics with no product code. The storage
  usage numbers are the first metrics; the live reads stay as "now".
  A member sees their own; admins see everything, target-wide, per-resource
  and per-person, directly on the person page, the target overview and
  insights (owner, 10 Nov 2026: no view logging); no ranking of people, as
  a UI choice; unbound accounts counted but never named; a point for a
  subject not bound to the target is dropped.
- **Records, not statistics,** for what a member did or holds (feature
  `results`): operation replies carry a `result` validated by the
  registry's schema (three common shapes: `records`, `activity`, `status`),
  read live and never stored. Every operation declares an `effect`; a
  `physical` one (checking an item out) is previewed and confirmed by name,
  never re-executed after an indeterminate outcome, and settled through
  `GET /calls/{call_id}` (feature `calls`). Read operations go through
  the one read path and are never logged.
- **Request identity is specified per write** (apply, operation without
  its secret params, lifecycle, setting) as a JCS digest; only a 2xx is
  remembered; a reused key with another payload is 422, one still in
  flight is 409, per the IETF idempotency-key draft.
- **The query string is signed as sent** (`ts.method.target.body`, RFC
  9421's `@query`), so `since` and a future `cursor` are inside the MAC
  like everything else the add-on acts on. Errors are RFC 9457
  `application/problem+json`. The target's id is `account_id`.
- **A closed kind vocabulary**, declared by every add-on (`storage`,
  `access`, `inventory`, `machine`), so the credential-scope denylist keys
  on it and never fails to fire; profiles stay optional.
- **The inventory system as the fourth target**, mapped with no core change
  (design §10): borrowing as `account`, categories as `membership`, items at
  once as a `limit` in `items`, check-out as a member operation, holdings as
  records, overdue as `standing`.

## Impact

- `addons/contract/` (schemas, fixtures, registries), `addons/truenas/`
  (manifest v2, `call_id` everywhere, `standing`, error envelope),
  `addons/conformance/` and `addons/reference/` (new modules), `addons/AUTHORING.md`.
- Backend: `internal/addons/*` (manifest v2, features, lifecycle-by-role,
  error registry), `internal/services/entitlement.go` (no field
  names), `handlers/{my_storage,person_storage,target_homes,mappings,mapping_plan}.go`,
  `services/{role_members,drift/addon}.go`. Guard test: no target field name
  outside `addons/truenas` and `addons/contract`.
- UI: `lib/role-mapping-plan.ts`, `targets/HomeLimit.tsx`, `AddMappingDialog`,
  `MappingManagement`, `PeopleOnTarget`, `storage/MyStorage`, `ui/Withheld`,
  `review/HoldsDueForReview`, `people/PersonStorage`, `queries/useMyStorage`:
  read fields by role from the manifest the backend already exposes.
- **BREAKING for add-ons:** contract 1 add-ons are refused at registration
  once the backend speaks 2. TrueNAS is the only add-on and ships in the same
  deploy; the deploy workflows themselves gain the order (stop `backend`,
  which carries the in-process propagation runner, stop `truenas-addon`,
  copy `state.db` as a fallback, then `up`); the add-on's store is written
  additively so rollback restores nothing; the new backend classes a shape
  or version refusal as `unreached`, never `target_refused`, and reads a
  v1 `error` body for classification, so no departure waits for an operator
  on rollback. The
  stored snapshots do not change (field names are unchanged,
  `desired_state_snapshots` stays readable).
- Slice 6, four additive features: `internal/addons/stats.go` (collection,
  content validation, one-transaction upsert), three additive tables and a
  nightly retention job (no rollup: every bucket is a UTC day),
  `/me/targets` carrying the actor's points, `GET /users/{id}/targets`
  carrying that person's for admins, target and insights endpoints for the
  totals and per-dimension series; `GET /calls/{call_id}`;
  UI cards rendered from descriptors; the TrueNAS add-on's collector and
  buffer.
- Four migrations, all additive except one constraint swap, plus one drop
  a release later: `account_id` beside `account_uid` in both binding
  tables, the unique index on bindings only (decision 10, slice 1);
  `targets.field_roles` (slice 1); the `000087` CHECK replaced by its
  verified target-agnostic form (slice 2; the new form accepts every row
  the old one did); the three statistics tables with the per-target
  retention column (slice 6); `account_uid` dropped (1.7b).
- One transport change, inside this major: the MAC input gains the raw
  query string as sent. The key derivation and its vector do not move.
  No Docker on the Mac.
- Out of scope: building the UniFi, Ruby or inventory add-on; code
  generation; a plugin SDK or dynamic loading; changing the key derivation;
  response signatures; a second supported major at the backend; hourly
  buckets, histograms or any export of the statistics.
