# The version check: an add-on checks what it depends on

Status: design, 23 Nov 2026. Not built. Slices are `tasks.md` §9, `needs-triage`.
Extends `design.md` §13. Source facts: `research-core-get-methods.md`.

Owner, 23 Nov 2026: "make it robust, elegant and be careful". When the TrueNAS
add-on meets a TrueNAS it was not tested on, it checks its own dependencies.
It neither trusts nor distrusts the version number. Then it switches each
Syndra function on or off, and says honestly why.

## 0. The shape in one paragraph

The add-on ships what TrueNAS's own method schemas looked like on the version
it was tested on. On every connection it asks the NAS for the same schemas
and compares only the parts Syndra uses. Each TrueNAS function comes out
**unchanged**, **changed**, **missing** or **unread**. Every reply is decoded
strictly, so a field Syndra relies on that is absent fails the read instead
of becoming a zero. A read through a changed function is shown but marked
`unchecked`, so nothing plans from it or counts it. A write through a changed
or missing function is held. On a new release line, writes through unchanged
functions wait for one deliberate "Trust TrueNAS X" from staff. This is a
consumer-driven contract (Fowler, *Consumer-Driven Contracts*): the consumer
states what it relies on and the provider is verified against that, at run
time, with a tolerant reader (Fowler, *Tolerant Reader*) for everything else.

## 1. The fact this rests on

`core.get_methods` on TrueNAS 25.10, read from source at `TS-25.10.8`
(`truenas/middleware@29ed02b8`), full citations in
`research-core-get-methods.md`:

- Each entry is a dict. `accepts` is a list with one JSON Schema per call
  argument, in order. `returns` is a list with exactly one schema. `job` says
  the call answers a job id instead of a result. Source:
  `src/middlewared/middlewared/service/core_service.py` L247–L380.
- The schemas are pydantic's, with `$ref`/`$defs` inlined, plus four
  non-standard additions the source marks FIXME "should be removed
  eventually": `_attrs_order_`, `_name_`, `_required_`, `title` overwritten
  with the property name, and `items` wrapped in a one-element list. Source:
  `src/middlewared/middlewared/api/base/jsonschema.py` L10–L62.
- The entry also carries the docstring (`description`, `cli_description`,
  `examples`). A docs-only patch changes a naive hash.
- The reply is filtered by the caller's credential. A method the key may not
  call is absent. `core.get_methods` itself needs no role
  (`authorization_required=False`).
- **Patch releases change schemas.** 25.10.0 to 25.10.8 moves the current API
  through five model versions; `user`, `sharing.smb` and `audit` models
  changed. 25.10.5 to 25.10.8 share one. So a version number proves nothing
  either way.
- On the 26 line the method stays public; entry keys `item_method` and
  `require_websocket` go. `accepts`, `returns` and `job` stay.

Not verified from source, and settled by slice 9a's capture: the exact JSON
for our methods, which of them our key sees, the reply's size and time.

## 2. What the owner's direction becomes

Accepted as given, with five refinements. Each is a pushback with its reason.

1. **Compare what Syndra uses, not the whole schema.** A whole-schema hash
   flags `sharing.smb.query` as changed the day TrueNAS adds a share option
   Syndra never reads (it did, inside 25.10). That holds functions for
   nothing, and a check that cries wolf gets trusted through. "Changed" means
   the relied-on part changed (§4).
2. **Keep the schemas, not only a hash.** A hash can say "changed". It cannot
   say "`smb` was a boolean and is now a string", which is the honest
   warning. The baseline is the recorded schemas themselves; the hash is only
   the name of a check (§3, §4.4).
3. **The baseline is the capture, with no derived file.** The same code
   projects the shipped capture and the live answer. A derived fingerprint
   file would be a second copy that drifts the day a reply struct gains a
   field.
4. **A read probes itself, every time.** The strict decode sits at the one
   place every reply passes (§5). The "probe once" pass is then only the
   existing readers run once, so the page can speak before anyone asks.
5. **A patch on a tested line with nothing changed asks nobody.** Holding
   writes also holds revocations. Asking staff on every patch teaches them to
   press Trust without reading. The number is still not trusted: it never
   unlocks anything by itself, it only decides whether a person is asked
   (§7). Owner question 1.

## 3. The module and its seam

One deep module, `addons/truenas/compat`, pure: no network, no store, no
clock. Its whole interface:

```go
// What Syndra relies on, per TrueNAS method. One table, in the add-on.
type Dep struct {
    Write   bool           // a write: cannot be probed
    Replies []any          // zero values of every Go type a call site decodes into
    Sends   []string       // for a write: JSON Pointers (RFC 6901) into accepts that Syndra sends
}

func Check(baselines []Baseline, deps map[string]Dep, version string,
    methods map[string]json.RawMessage) Report   // methods nil: nothing was read

func (r Report) Verdict(method string) Verdict   // unchanged | changed | missing | unread, with Detail
func (r Report) ID() string                      // names this exact set of relied-on shapes
func (r Report) TestedLine() bool                // the release line is one a baseline was recorded on

func Require(raw json.RawMessage, into any) error  // the strict decode (§5)
```

Hidden behind it: parsing the capture, canonical form, projection, the diff
and its wording. Two real callers feed `Check` the same type (the embedded
capture and the live reply), so there is no interface and no fake: tests pass
JSON. The deletion test holds: without it, canonical form and "what do we
rely on" spread over `nas.go`, `server.go`, the collector and the tests.

**Where it runs.** `NAS.ensureProbed` (`nas.go`), where `loadMethods` runs
today: once per connection, after `system.version`. A reconnect may land on
an upgraded NAS, and TrueNAS cannot change version without dropping the
session, so per connection is exactly "on version change". The report is
kept in memory. If `core.get_methods` fails on a reconnect and the version
string is the one last checked, the last report stands; a version string
names one build.

**Concurrency.** Nothing new. The check's one call goes through `NAS.call`,
so it waits its turn on `line` like any read. `n.mu` is never held across
the wire; the finished `Report` is swapped in under `n.mu` as one immutable
pointer, and readers (`unavailable`, `/health`, the fresher) copy the pointer
and read without a lock. The probe pass (§5.2) runs on its own goroutine,
one call at a time through the same line, and writes its results only if the
report it started under is still the current one.

**What goes.** `loadMethods`, `MethodPresent` ("unknown is present"),
`MajorSupported`, `majorOf`'s gate use, `supportedMajs` and
`TRUENAS_SUPPORTED_MAJORS`. That variable let a deployment claim 25.04 and
26.04 were tested when only 25.10.7 ever was. Which lines are tested is now
read from the captures that ship.

**The one decision.** `server.unavailable(operation)` stays the single
source for the manifest and for the route's gate (§6). `NAS.sendWrite`
repeats the verdict for its own method after `ensureProbed`, so a write
admitted under the old version on a session that reconnected onto a new one
is not sent (`ErrNotSent`).

## 4. The baseline and the comparison

### 4.1 Where the baseline lives

`addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json`, embedded with
`go:embed`. It is the NAS's own `core.get_methods` entries, as answered, for
the methods in the dependency table and no others, plus the version string.
One file per tested version; several may ship. **Privacy:** method schemas
and docstrings only. No account, share, path, host or key is in a schema;
the recorder writes entries for listed methods only and refuses a reply that
is not a map of entries.

**The one command** after testing a new version:
`scripts/record-truenas-fixtures.sh`. It already records
`truenas_observed.json` read-only from a real NAS with the add-on's own key;
it gains one more read and writes the baseline file beside it. Commit both.
**Without a NAS there is nothing to run:** the projection is computed from
the committed capture by the same code at test time and at start, so adding
a field to a reply struct needs no re-record.

### 4.2 What Syndra relies on

- **Returns:** every JSON path a registered reply type reads. Derived by
  reflection from the Go struct's `json` tags: a struct descends `properties`,
  a slice descends `items`, and an `anyOf` is followed down each branch that
  fits. At each path the projection keeps `type`, `enum`, `format`, `const`
  and, for an object Syndra reads as a whole, nothing more.
- **Accepts**, for a write: the number and order of arguments; the subschema
  at each pointer in `Sends`; and, for every object Syndra sends into, its
  `required` list and `additionalProperties`. A new required field Syndra
  does not send is a change. A new optional one is not.
- **`job`**: whether the call answers a job.
- Rule that makes this complete: **Syndra sends every field whose value it
  cares about.** A field left to TrueNAS's default is not seen by the check.
  That limit is stated, tested by `write_rules`, and is one reason a new
  line still needs a person (§7).

### 4.3 Canonical form

Applied to both sides before comparing:

1. Drop annotations that carry no behaviour: `description`, `title`,
   `examples`, `deprecated`, `$comment` (JSON Schema 2020-12, Validation §9),
   and TrueNAS's `_attrs_order_`, `_name_`, `_required_`. `default` is kept:
   pydantic applies it.
2. `items` given as a one-element list becomes that element.
3. `required`, `enum` and a list-valued `type` are sets: sorted. `anyOf` and
   `oneOf` branches are sorted by their own canonical bytes.
4. Serialise with RFC 8785 (JSON Canonicalization Scheme): sorted keys,
   fixed number and string forms.

### 4.4 Verdicts

Per method, against each baseline; the best match wins.

| Verdict | Means |
|---|---|
| `unchanged` | The canonical projection equals a baseline's, byte for byte. |
| `changed` | It differs. `Detail` names each path with "was / now". |
| `missing` | The NAS lists no such method for Syndra's key: removed, renamed, or the key's role no longer allows it. Either way Syndra cannot call it. |
| `unread` | `core.get_methods` gave no answer, or the entry lacks `accepts`, `returns` or `job`. Nothing is concluded. |

A doc-text change, a new unrelated field, a reordered `required` and the
removal of the FIXME keys are all `unchanged`. `Report.ID()` is SHA-256 over
the canonical projections of every dependency, sorted by method. It does not
include the version string.

## 5. Reads: fail closed, then probe

### 5.1 One strict decode

`NAS.roundTrip` decodes every reply in one place, and `decodeAnswer` the
health sources. Both call `compat.Require`, which replaces plain
`json.Unmarshal` and closes the `ponytail:` note at `operations.go:821`.

**"Relies on" means:** every field with a `json` tag in the type the call
site decodes into. One source, the Go struct. For each such field, at every
depth and in every row:

- the key is present;
- its JSON kind fits the Go type;
- `null` is accepted only by a pointer, slice, map or `json.RawMessage`.

A field TrueNAS may leave out is tagged `nas:"optional"`, in the struct,
where the reader sees it. A failure is the existing "an answer the add-on
could not use" (`TARGET_UNREADABLE` / `NO_CURRENT_READ`; §13.1's `unusable`),
with the add-on's sentence naming the method and field. Never a zero, never
a kept copy relabelled as an outage. Inline anonymous reply structs become
named types so the dependency table can list them; `read_shape_test.go`
already walks every call site and gains the assertion that each site's type
is registered.

### 5.2 The probe pass

When a report's ID is one this process has not probed, the add-on runs its
own existing readers once, in the background, discarding the data: accounts
and groups, shares and their audit settings, datasets, one quota table, the
ACL walk's first level, the four health sources, and the oldest audit row
(`auditOldest`). No new call shapes: what is probed is what is used. The
seam in §5.1 records per method `passed`, `failed` (with the field) or
`no_rows` (an empty list proves nothing about its rows; the strict decode
still guards the first real row).

Cost: about a dozen reads, once per new shape, each bound by
`statsCallTarget` (5 s) through `withCallLimit`, one at a time on the line.
No request waits on it, so `withBudget` is untouched, and `/health`'s ping
queues behind at most one short call (§13.2). A slow or interrupted pass
leaves the rest `pending` and resumes after `probeCooldown`.

Writes are not probed. A write cannot be tried without making it.

## 6. What each operation does

One table, in `server.unavailable`, from the report, the probes and the trust.

| The operation's functions | A read | A write (apply, a write operation, a setting TrueNAS holds) |
|---|---|---|
| any `missing` | withdrawn: `available: false`, reason names the function | withdrawn, same |
| any `changed` or `unread`, or a probe `failed` | stays declared; its reply is `unchecked` (§6.1); a failed decode answers `unusable` | held: `TARGET_VERSION_UNSUPPORTED` |
| all `unchanged`, probes not `failed`, line tested | normal | normal |
| all `unchanged`, probes not `failed`, line new | normal | held until trusted (§7) |

A write also depends on the reads it makes first. `methodsFor` grows to name
every method of every operation, the apply and the collector; a test fails
when a call site uses a method its operation does not list.

Reads stay declared through an unread version, as PR 264 decided. Only
`missing` withdraws a read: there is nothing to call.

### 6.1 `unchecked`: one flag, on the path that exists

A read reply already says whether it can be concluded from: `current`, and
`stale` when it cannot (`common/stale.json`). The flag is one more reason
there, **`stale.reason: "unchecked"`**, with `current: false` and the
add-on's sentence in `note`. `taken_at` stays the real moment of the read.

Why not a new sibling field: every consumer already refuses to conclude from
a reply that is not current. Drift halts (`drift/addon.go`, `!read.Current`),
adoption waits, the plan is not made. A new field would be safe only once
every consumer learned it; this is safe on the day it ships.

- **Stamped once**, where `Current: true` is stamped today
  (`fresher.read`, `fresh.go`): `NAS.call` notes each `changed` or `unread`
  method it served on the request's context, and the fresher reads the note.
  `/plan` and the kept copy carry it the same way.
- **Usage history.** `collectStatsOnce` treats an unchecked read as it
  treats a failed one (`failedRun`): the watermark holds and nothing is
  written. Sessions are re-read from the audit log once the check passes;
  a level that could not be sampled becomes a gap, as today. No change to
  `stats.json`.
- **Backend.** `ReadOf`, the one writer of `read`, maps it to a new state:
  `read: {state: "unchecked", taken_at, reason: "unchecked", detail}`. The
  data is shown with its warning. `Halt()` halts on it with the existing
  `read_stale` reason and the detail, so no migration.

## 7. "Trust TrueNAS X"

- **What it is.** The add-on's setting `trusted_version`, written through
  the settings path that exists: `PUT /settings/trusted_version` with
  `call_id` and `actor`, deduplicated by `once`, stored by `store.PutSetting`
  inside `server.mutate`. So it survives a restart and is a record in the
  add-on's tamper-evident mutation log, which Syndra anchors.
- **Its value** is `"<release line> <report ID>"`, for example
  `26.04 sha256:9f2c…`. The setting's `values` offers exactly one: the one
  observed now. Anything else is refused (`NOT_OFFERED`), so nobody can
  trust a version the add-on is not looking at. Empty withdraws it.
- **What it unlocks.** Only the last row of §6: writes whose functions are
  all `unchanged` with no failed probe. It never unlocks `changed`,
  `missing` or `unread`. Those need a tested add-on release; there is no
  override, because the person clicking cannot see what the maintainer
  could not.
- **When it lapses.** Trust holds while the line and the ID are the ones
  trusted. A patch that changes nothing Syndra uses keeps it. Any change to
  what Syndra relies on, on the NAS side or by an add-on update that adds a
  dependency, makes a new ID, and the old trust simply no longer matches.
  Nothing is deleted, so the log keeps who trusted what.
- **Revoking.** "Stop trusting TrueNAS X" writes the empty value. Writes are
  held from the next call. A downgrade to the tested version needs no trust
  and ignores the stored one.
- **Who.** Any operator, through `PUT /api/v1/targets/{target}/settings/{name}`
  (`withOperatorAuth`), the gate every system setting has. The backend adds
  what that handler lacks today: an audit event, `target.version_trusted`
  and `target.version_trust_withdrawn`, with actor, system, version and ID.
- **Not gated by itself.** It changes only the add-on, so neither the
  version hold nor `read_only` refuses it (as `/lifecycle`).
- **One write place:** the version notice on the system's page. Everywhere
  else reads it and links there.

## 8. How the result reaches the backend

One path each. No back-compat inside contract 2 (owner, 13 Nov): the old
fields go in the same change, both containers deploy together.

| Surface | Change |
|---|---|
| Manifest | unchanged in shape. `available` and `unavailable_reason` now come from §6. |
| `health.json` | `version_tested` and `version_note` are **removed**. `version_check` is added and required. |
| `common/stale.json` | `reason` gains `unchecked`. `current`'s description gains "and whose shape the add-on has checked". |
| `errors.json` | `TARGET_VERSION_UNSUPPORTED` means: "An operation that depends on an app function that is missing, changed, unread, or not yet trusted on this app version. Nothing was sent." `details: {functions: [...], trust: bool}`. A missing method no longer answers `OPERATION_FAILED`, which can mean "may have reached the app". |
| Settings | `trusted_version`; new code `NOT_OFFERED` (422) for a value the add-on does not offer. `NOT_A_HOME_PLACE` stays. |
| Kit, `AUTHORING.md` ch. 2 step 2, reference add-on | the gate rule is rewritten to this; the reference answers `state: "tested"`. |

```json
"version_check": {
  "state": "tested | new | trusted | unread",
  "version": "TrueNAS-26.04.0",
  "tested": ["TrueNAS-25.10.7"],
  "id": "sha256:9f2c…",
  "note": "the add-on's own sentence",
  "trust": {"setting": "trusted_version", "value": "26.04 sha256:9f2c…", "by": "…", "at": "…"},
  "functions": [
    {"name": "user.update", "effect": "write", "verdict": "changed", "detail": "smb: was boolean, now string"},
    {"name": "user.query", "effect": "read", "verdict": "unchanged", "probe": "passed"}
  ]
}
```

`state`: `tested` is every function unchanged on a tested line. `unread` is
no version or no method list yet. `trusted` is a matching trust. `new` is
everything else. `trust` is present only when pressing it would unlock
something (`value`, no `by`) or when it is given (`by`, `at`). The names are
app-neutral: a UniFi or inventory add-on fills the same object, and the UI
needs no TrueNAS knowledge to draw the button.

**Held writes do not burn retries.** A write held for the version is §13.4's
`unreached`: nothing went out. The drain reads `version_check` from the
monitor's last health answer and claims no row whose operation is held, as
it claims none for an unreachable system. A row answered
`TARGET_VERSION_UNSUPPORTED` spends no attempt. A revocation therefore
waits and lands by itself after Trust; it never becomes "out of retries".

## 9. Failure modes

| Case | What happens |
|---|---|
| `core.get_methods` fails | Same version string as the last report: that report stands. Otherwise every function is `unread`: reads `unchecked`, writes held, `state: unread`. Tried again after `probeCooldown`. The old "unknown is present" is gone: it concluded safety from a read that did not happen. |
| NAS down during the check | No version, no report, `state: unread`. Reads stay declared and answer the outage (§13.1). Writes could not go anyway. The check runs on the first connection. |
| Version flip mid-session | TrueNAS restarts to change version, so the session dies. The new connection re-checks before its first call, and `sendWrite` re-reads the verdict, so a write queued across the flip is not sent on the old verdict. A write already on the wire is the existing in-doubt case. |
| Downgrade | Compared like anything else. Back on the tested version: `tested`, no trust needed. On an older untested one: `new`, with its own verdicts. |
| A 25.10.x patch | Not "new" by number, not "same" by number. Compared. Nothing relied-on changed: `tested`, nobody is asked. Something did: those functions are `changed` and held; the rest carry on. |
| Probe slow or cut off | Each call is bound to 5 s and runs outside any request. Unfinished probes read `pending`; `pending` does not block a read, and blocks Trust from being offered until the pass ends. |
| A reply's shape breaks with the schema unchanged | The strict decode fails that read, every time, with the field named. The probe marks it `failed`; writes depending on it are held. |
| Add-on updated, NAS not | A new dependency changes the ID. On a tested line with a matching baseline nothing is asked. On a trusted new line, trust is asked for again. |
| The baseline file is unreadable | The add-on refuses to start. It is embedded and tested, so this is a build fault, not a deployment state. |

## 10. Copy (for the copy guide §19; final words go through it)

`{product}` TrueNAS, `{version}` the short version ("26.04.0"), `{system}`
the system's name, `{n}`/`{m}` counts.

| Reader, where | Says |
|---|---|
| Operator, system page, `tested` | nothing. No notice at rest. |
| Operator, `new`, trust offered | {product} {version} is new to Syndra. {n} functions checked fine. Changes to {system} wait until you trust this version. Read the list, then press Trust {product} {version}. |
| Operator, `new`, some held | {product} {version} is new to Syndra: {n} functions checked fine, {m} held back: user.update (changed), audit.query (missing). Held functions stay off until the add-on is updated. Update the add-on. |
| Operator, `new` on a tested line (a patch changed something) | {product} {version} changed {m} functions Syndra uses: user.update (changed). They stay off until the add-on is updated. Everything else works. Update the add-on. |
| Operator, `trusted` | {product} {version}, trusted by {who} on {date}. Nothing for you to do. |
| Operator, `unread` | Syndra's add-on hasn't been able to check {product} yet. Changes wait. It checks again by itself. |
| Operator, the list, per function | Checked fine · Changed: {detail} · Missing · Not checked yet |
| Operator, confirm Trust, title | Trust {product} {version}? |
| the same, body | Syndra checked the {n} functions it uses, and they look the same as on {tested version}. It can't test a change without making one. After this, Syndra makes changes on {system} again, starting with the {k} that are waiting. You can stop trusting it at any time. |
| the same, buttons | Trust {product} {version} · Not yet |
| Operator, result | Trusted. Syndra is sending the {k} waiting changes. |
| Operator, withdraw | Stop trusting {product} {version} · Syndra stops making changes on {system} until you trust it again. |
| Operator, a read marked `unchecked` | {product} changed how it answers this. Syndra shows it but doesn't act on it or count it. Update the add-on. |
| Operator, a write held for trust | Nothing went out. {product} {version} isn't trusted yet. Trust it on {system}'s page; Syndra sends this by itself after that. |
| Operator, a write held, changed or missing | Nothing went out. {product} {version} changed something this needs, so it stays off until the add-on is updated. Update the add-on. |
| Member, a read marked `unchecked` | {system} was updated and Syndra is still checking it. This may be incomplete. Nothing for you to do. |
| Member, a write held | Nothing went out. {system} was updated and makerspace staff need to check it first. Ask makerspace staff. |
| Usage, not counted | Not counted since {date}: {product} changed how it reports this. Update the add-on. |
| Audit | Trusted {product} {version} on {system} · Stopped trusting {product} {version} on {system} |

Function names (`user.update`) are TrueNAS's own words, shown to operators
only, in `Mono`. A member never sees one.

## 11. Slices

In build order. Each is small and has a test that fails without it. All on
`feat/addon-contract`; none merges to main alone.

- **9a Capture.** Owner-gated, read-only: run the recorder against the
  tested NAS, commit the baseline file, and settle the research's
  unverified points (the JSON as answered, which methods the key sees, size
  and time of the reply). *Test:* the committed file parses and holds an
  entry with `accepts`, `returns`, `job` for every method in the table.
- **9b Strict decode.** `compat.Require`; `roundTrip` and `decodeAnswer` use
  it; reply types named and registered; `nas:"optional"` where
  `truenas_observed.json` shows a null or absent key. *Test:* a recorded
  reply with one relied-on key renamed fails the read with that key's name;
  today it decodes to zero. *Mutation:* swap `Require` back to
  `json.Unmarshal`, red.
- **9c `compat.Check`.** Pure: canonical form, projection, verdicts, ID.
  *Tests,* each from the capture with one edit: a docstring edit is
  `unchanged`; a new unrelated property is `unchanged`; the FIXME keys
  removed is `unchanged`; a relied-on type changed is `changed` with its
  path; a new required argument is `changed`; an entry removed is `missing`;
  no methods is all `unread`.
- **9d Contract.** Schemas, registry, fixtures, kit and guide of §8, alone,
  as slice 1a did. *Test:* the schema tests reject a health reply with
  `version_tested` and one without `version_check`.
- **9e The add-on decides.** `ensureProbed` runs the check; `unavailable`
  and `sendWrite` read it; the probe pass; `/health` serves `version_check`;
  the old gate and `TRUENAS_SUPPORTED_MAJORS` deleted. *Tests:* against a
  fake NAS serving an edited capture, a changed `user.update` holds
  `password.set` and leaves reads declared; a missing `audit.query`
  withdraws `activity.get`; a failing `core.get_methods` on a new version
  holds every write; a reconnect onto a new version does not send a queued
  write.
- **9f `unchecked`.** The fresher's stamp, `/plan`, the collector.
  *Tests:* a read through a changed method answers `current: false`,
  `stale.reason: unchecked`; a collector run over it writes no point and
  holds the watermark.
- **9g Trust.** The `trusted_version` setting. *Tests:* a new line with
  everything unchanged holds an apply until the setting is written, then
  sends it; a value other than the offered one is `NOT_OFFERED`; trust does
  not unlock a `changed` function; it survives a restart; a new ID ends it.
- **9h Backend.** `healthWire`, `ReadOf`'s `unchecked` state, the drain's
  hold with no attempt spent, the audit events. *Tests:* the reachability
  table gains an `unchecked` stub on every route of §13.3 (no plan, no
  count, data shown); a revocation queued while held spends no attempt and
  lands on the first pass after trust.
- **9i UI and copy.** Board first (designer), then the notice, the list,
  the Trust dialog, the `unchecked` reading through `readOf`, and the §19
  rows with their guard test. Layout check: the health read changes shape,
  so the gate is the staging run after deploy; say so in the PR.
- **9j Staging.** Owner-gated. The add-on there is `read_only`, so: the
  check reads `tested` on the real NAS; a fake-version run (an edited
  baseline in a test build) shows `new`, the list and the held writes.

## 12. Questions for the owner

1. **A patch on a tested line with nothing changed: ask staff or not?**
   Recommended: not. Otherwise every patch holds revocations until someone
   clicks, and the click becomes a habit.
2. **Revocations on a new, untrusted line: held like every write?**
   Recommended: yes. A revocation is `user.update`; sending it blind is the
   risk this exists to stop. The notice counts what is waiting, and Trust is
   one press.
3. **Who may Trust: any operator, or admins only?** Recommended: any
   operator, audited. It is the same people who can already send every
   write.
4. **9a reads the real NAS once (read-only).** Needs your yes, as every NAS
   touch does.
