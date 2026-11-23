# The version check: an add-on checks what it depends on

Status: design, revised 23 Nov 2026 after the owner's rulings and the Opus
review. Not built. Slices are `tasks.md` §9. Extends `design.md` §13.
Source facts: `research-core-get-methods.md`.

Owner, 23 Nov 2026: "make it robust, elegant and be careful". When the TrueNAS
add-on meets a TrueNAS it was not tested on, it checks its own dependencies.
It neither trusts nor distrusts the version number, and it asks nobody to.

## 0. The shape in one paragraph

The add-on ships what TrueNAS's own method schemas looked like on the version
it was tested on. On every connection it asks the NAS for the same schemas
and compares only the parts Syndra uses. Each TrueNAS function comes out
**unchanged**, **changed**, **missing** or **unread**. One rule then applies
to every version, patch or major:

- **Unchanged:** the function works, writes included.
- **Changed** (or unread): that function only is held. A read through it is
  shown but marked `unchecked`, so nothing plans from it or records it. A
  write through it waits for an add-on update.
- **Missing:** withdrawn.

Every reply is also decoded strictly, so a field Syndra relies on that is
absent fails the read instead of becoming a zero. This is a consumer-driven
contract (Fowler, *Consumer-Driven Contracts*): the consumer states what it
relies on and the provider is verified against that, at run time, with a
tolerant reader (Fowler, *Tolerant Reader*) for everything else.

**There is no "Trust" step** (owner, 23 Nov 2026: if the check passes, why
trust?). A removal through an unchanged function goes ahead, so nobody keeps
access waiting on a button. The net for "same shape, different behaviour"
already exists: apply reads the account back after its writes
(`addons/truenas/apply.go:1165`, `readBack`) and reports what it read, and
drift's next pass shows any difference. A check that passed plus a read-back
is more than a person pressing a button could know.

## 1. The fact this rests on

`core.get_methods` on TrueNAS 25.10, read from source at `TS-25.10.8`
(`truenas/middleware@29ed02b8`), full citations in
`research-core-get-methods.md`:

- Each entry is a dict. `accepts` is a list with one JSON Schema per call
  argument, in order. `returns` is a list with exactly one schema. `job` says
  the call answers a job id instead of a result. Source:
  `src/middlewared/middlewared/service/core_service.py` L247–L380.
- The schemas are pydantic's, with `$ref`/`$defs` inlined, plus non-standard
  additions the source marks FIXME "should be removed eventually":
  `_attrs_order_`, `_name_`, `_required_`, `title` overwritten with the
  property name, and `items` wrapped in a one-element list. Source:
  `src/middlewared/middlewared/api/base/jsonschema.py` L10–L62.
- The entry also carries the docstring. A docs-only patch changes a naive
  hash.
- The reply is filtered by the caller's credential. A method the key may not
  call is absent. `core.get_methods` itself needs no role.
- **Patch releases change schemas.** 25.10.0 to 25.10.8 moves the current API
  through five model versions; `user`, `sharing.smb` and `audit` models
  changed. 25.10.5 to 25.10.8 share one. So a version number proves nothing
  either way.
- On the 26 line the method stays public; `accepts`, `returns` and `job`
  stay.

Not verified from source, and settled by slice 9a's capture: the exact JSON
for our methods, which of them each key sees, the reply's size and time.

## 2. Decisions

1. **Compare what Syndra uses, not the whole schema.** A whole-schema hash
   flags `sharing.smb.query` the day TrueNAS adds a share option Syndra never
   reads (it did, inside 25.10). "Changed" means the relied-on part changed
   (§4).
2. **Keep the schemas, not a hash.** A hash cannot say "`smb` was a boolean
   and is now a string", which is the honest warning.
3. **The baseline is the capture, with no derived file.** The same code
   projects the shipped capture and the live answer.
4. **No probe pass.** The strict decode sits where every reply passes (§5).
   The drift sweep, the collector and the health monitor run every reader
   within minutes, and a write's own pre-reads guard it. A separate pass
   would add a goroutine, a `pending` state and resume logic to learn the
   same thing a few minutes sooner.
5. **One rule, no person in the loop** (§0). The one case that needs a
   person is updating the add-on, and the notice says so.

## 3. The module and its seam

One deep module, `addons/truenas/compat`, pure: no network, no store, no
clock. Its whole interface:

```go
// What Syndra relies on, per TrueNAS method. One table, in the add-on.
type Dep struct {
    Write   bool      // a write
    Create  bool      // a create: defaults of unsent properties count (§4.2)
    Replies []any     // zero values of every Go type a call site decodes into
    Sends   []string  // for a write: JSON Pointers (RFC 6901) into accepts that Syndra sends
}

func Check(baselines []Baseline, deps map[string]Dep, version string,
    methods map[string]json.RawMessage) Report   // methods nil: nothing was read

func (r Report) Verdict(method string) Verdict   // unchanged | changed | missing | unread, with Detail
func (r Report) Tested() bool                    // version is one a baseline was recorded on

func Require(raw json.RawMessage, into any) error  // the strict decode (§5)
```

Hidden behind it: parsing the capture, canonical form, projection, the diff
and its wording. Two real callers feed `Check` the same type (the embedded
capture and the live reply), so there is no interface and no fake: tests pass
JSON.

**Where it runs.** `NAS.ensureProbed` (`nas.go`), where `loadMethods` runs
today (`nas.go:479`): once per connection, after `system.version`. TrueNAS
cannot change version without dropping the session, so per connection is
exactly "on version change". The finished `Report` is swapped in under `n.mu`
as one immutable pointer; readers copy the pointer and read without a lock.
No new goroutine.

**What goes.** `loadMethods` and its call (`nas.go:479`); `MethodPresent`
("unknown is present") and its call (`server.go:179`); `MajorSupported` and
its four calls (`server.go:175`, `server.go:315`, `apply.go:638`,
`main.go:329`); `majorOf`; `supportedMajs` (`nas.go:185`) and
`TRUENAS_SUPPORTED_MAJORS` (`main.go:165`, README). That variable let a
deployment claim 25.04 and 26.04 were tested when only 25.10.7 ever was.

**The one decision.** `server.unavailable(operation)` stays the single
source for the manifest and for the route's gate (§6). Two places repeat the
verdict at the last moment, because the session may have reconnected onto a
new version since the gate:

- `NAS.sendWrite`, for its own method after `ensureProbed` (`ErrNotSent`).
- `account.purge`, which does not use `sendWrite`: it sends `user.delete`
  with `callOnce` on its own elevated session (`operations.go:605`). It reads
  the verdicts of `user.delete` and its pre-reads again after
  `operatedAccount` and before the call.

The methods reply is filtered by credential (§1), so the add-on's ordinary
key may not list `user.delete` at all. 9a settles it. If it does not, the
purge checks `user.delete` from `core.get_methods` on the elevated session
it has just opened, with the same `Check`.

## 4. The baseline and the comparison

### 4.1 Where the baseline lives

`addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json`, embedded with
`go:embed`: the NAS's own `core.get_methods` entries, as answered, for the
methods in the dependency table and no others, plus the version string. One
file per tested version; several may ship. **Privacy:** method schemas and
docstrings only. The recorder writes entries for listed methods only and
refuses a reply that is not a map of entries.

**The one command** after testing a new version:
`scripts/record-truenas-fixtures.sh`. It already records
`truenas_observed.json` read-only with the add-on's own key; it gains one
more read, writes the baseline file beside it, and prints the size and time
of the `core.get_methods` reply. Without a NAS there is nothing to run: the
projection is computed from the committed capture at test time and at start.

**Where it runs: on the staging host, never on the Mac.** The recorder runs
its probe in a container (`record-truenas-fixtures.sh:65`), and the owner
forbids local Docker. So for 9a: copy the slice's `scripts/` to a temp
directory on the staging host over ssh, run the recorder there without
`--write`, with `TRUENAS_URL` and the key read from the staging add-on's own
env file, copy the two output files back, delete the temp directory. The
host is the operator's shell configuration, never written in the repo.
Nothing is deployed, the staging stack is not touched, and the staging
add-on stays `read_only`. The recorder opens one extra session with the
add-on's key for a few reads.

### 4.2 What Syndra relies on

- **Returns:** every JSON path a registered reply type reads, derived by
  reflection from the Go struct's `json` tags: a struct descends
  `properties`, a slice descends `items`. At each path the projection keeps
  `enum`, `format`, `const` and the `type` of **every** `anyOf` branch, so a
  field that becomes nullable is seen. A relied-on path the baseline does
  not have fails the test: the struct names something TrueNAS never
  declared.
- **Accepts**, for a write: the number and order of arguments; the subschema
  at each pointer in `Sends`; and, for every object Syndra sends into, its
  `required` list and `additionalProperties`. A new required field Syndra
  does not send is a change. A new optional one is not.
- **Defaults**, for a create (`user.create`, `group.create`): the `default`
  of every property Syndra does not send. TrueNAS fills those in, so a
  changed default changes the account Syndra makes.
- **`job`**: whether the call answers a job.

### 4.3 Canonical form

Applied to both sides before comparing:

1. Drop annotations that carry no behaviour: `description`, `title`,
   `examples`, `deprecated`, `$comment` (JSON Schema 2020-12, Validation §9),
   and TrueNAS's `_attrs_order_`, `_name_`, `_required_`.
2. `items` given as a one-element list becomes that element.
3. `required`, `enum` and a list-valued `type` are sets: sorted. `anyOf` and
   `oneOf` branches are sorted by their own canonical bytes.
4. Serialise with `json.Marshal` of the normalised tree. Go's encoder writes
   map keys sorted, and both sides pass through the same code, so equal
   trees give equal bytes. No canonicalisation library.

### 4.4 Verdicts

Per method, against each baseline; `unchanged` if it equals any.

| Verdict | Means |
|---|---|
| `unchanged` | The canonical projection equals a baseline's, byte for byte. |
| `changed` | It differs. `Detail` names each path with "was / now". |
| `missing` | The NAS lists no such method for the key: removed, renamed, or the key's role no longer allows it. Syndra cannot call it. |
| `unread` | `core.get_methods` gave no answer, or the entry lacks `accepts`, `returns` or `job`. Nothing is concluded; treated as `changed` until read (§9). |

A doc-text change, a new unrelated field, a reordered `required` and the
removal of the FIXME keys are all `unchanged`.

## 5. Reads fail closed: one strict decode

`NAS.roundTrip` decodes every reply in one place, and `decodeAnswer` the
health sources. Both call `compat.Require`, which replaces plain
`json.Unmarshal` and closes the `ponytail:` note at `operations.go:821`.

**"Relies on" means** every field with a `json` tag in the type the call
site decodes into. One source, the Go struct. For each such field, at every
depth and in every row:

- the key is present, unless the field is tagged `nas:"optional"`;
- its JSON kind fits the Go type;
- `null` is accepted only by a pointer, slice, map or `json.RawMessage`.

**Nullability comes from the baseline schema, not from one recorded reply.**
A test walks every registered reply type against the baseline `returns`
schema: where a path has an `anyOf` branch of type `null`, the Go field must
be able to hold it, or the test fails. A reply recorded on one day shows
only what was null that day. Pydantic's `required` list describes what a
caller must send, so it is not used for replies.

**A decode failure is not a TrueNAS outage.** Today's "an answer the add-on
could not use" is `TARGET_UNREADABLE`, which the backend reads as
`app_unreachable` (`backend/internal/addons/unread.go:137`) with the words
"Check TrueNAS, then read again" (`handlers/not_read.go:125`). That next
step is wrong here: TrueNAS is fine and reading again changes nothing. So a
failed `Require` answers a new code, **`TARGET_SHAPE_CHANGED`**, with
`details: {function, field, verdict}` (the method's verdict from the
report). The backend maps it to the existing reason `addon_outdated`, whose
next step is already "Update the add-on" (`not_read.go:138`) and whose drift
halt already exists (`read.go`, `Halt()`). It is the add-on answering on
purpose, so the breaker does not count it. `TARGET_UNREADABLE` keeps its
meaning for an answer that is no answer (a bare `null`, not JSON). §13.1's
vocabulary gains no word; one code moves to a reason that exists.

Inline anonymous reply structs become named types so the dependency table
can list them; `read_shape_test.go` already walks every call site and gains
the assertion that each site's type is registered.

Writes are not probed. A write cannot be tried without making it; its
pre-reads and the read-back are its guard.

## 6. What each operation does

One table, in `server.unavailable`, from the report alone.

| The operation's functions | A read | A write (apply, a write operation, a setting TrueNAS holds) |
|---|---|---|
| all `unchanged` | normal | normal, on any version |
| any `changed` or `unread` | stays declared; its reply is `unchecked` (§6.1) | held: `TARGET_VERSION_UNSUPPORTED`, nothing sent |
| any `missing` | withdrawn: `available: false`, reason names the function | withdrawn, same |

A write also depends on the reads it makes first. `methodsFor` grows to name
every method of every operation, the apply and the collector; a test fails
when a call site uses a method its operation does not list.

**The purge** (`account.purge`, irreversible) follows the same rule and
needs nothing extra: an operator already types the elevated key and confirms
each time. Allowed when `user.delete` and its pre-reads are unchanged, held
when any changed, re-checked after `operatedAccount` (§3).

**A revocation through a changed function stays held.** Sending
`user.update` in a shape TrueNAS no longer documents is the risk this exists
to stop. The notice counts the departed members still waiting to lose
access, prominently (§10). The count is the system's unsent removal rows in
the outbox while its health holds `apply`; nothing new is stored.

### 6.1 `unchecked`: one flag, on the path that exists

A read reply already says whether it can be concluded from: `current`, and
`stale` when it cannot (`common/stale.json`). The flag is one more reason
there, **`stale.reason: "unchecked"`**, with `current: false` and the
add-on's sentence in `note`. `taken_at` stays the real moment of the read.

Why not a new sibling field: every consumer already refuses to conclude from
a reply that is not current. A new field would be safe only once every
consumer learned it; this is safe on the day it ships.

- **Stamped once**, where `Current: true` is stamped today (`fresher.read`,
  `fresh.go:229`): `NAS.call` notes each `changed` or `unread` method it
  served on the request's context, and the fresher reads the note. `/plan`
  and the kept copy carry it the same way.
- **Usage history.** `collectStatsOnce` treats an unchecked read as a failed
  one (`failedRun`, `stats_collect.go:107`): the watermark holds and nothing
  is written. No change to `stats.json`.
- **Backend: no new read state.** `ReadState.None()` is true for anything
  but `now` and `kept` (`read_state.go:38`), so a third state would hide the
  data at every site that asks it, and the UI draws its dated card for
  `kept` only. `ReadOf` maps the reply to
  `read: {state: "kept", taken_at, reason: "unchecked", detail}`. The data
  is shown with its warning, as every kept copy is.
- **Drift** halts where it halts on any reply that is not current: its own
  `!read.Current` branch (`services/drift/addon.go:250`), as `read_stale`.
  For `unchecked` the detail is the add-on's note and the next step is
  "Update the add-on", the same one a failed decode gives (§5). No
  migration.

## 7. How the result reaches the backend

One path each. No back-compat inside contract 2 (owner, 13 Nov): the old
fields go in the same change, both containers deploy together.

| Surface | Change |
|---|---|
| Manifest | unchanged in shape. `available` and `unavailable_reason` now come from §6. |
| `health.json` | `version_tested` and `version_note` are **removed**. `version_check` is added and required. |
| `common/stale.json` | `reason` gains `unchecked`. `current`'s description gains "and whose shape the add-on has checked". |
| `errors.json` | `TARGET_VERSION_UNSUPPORTED`: "An operation that depends on an app function that is missing, changed or unread on this app version. Nothing was sent." `details: {functions: [...]}`. Its `retry` moves from `never` (`errors.json:264`, `addons/truenas/errors.go:88`) to `retry_after`. New: `TARGET_SHAPE_CHANGED` (§5). A missing method no longer answers `OPERATION_FAILED`, which can mean "may have reached the app". |
| Kit, `AUTHORING.md` ch. 2 step 2, reference add-on | the gate rule is rewritten to this; the reference answers `state: "tested"`. New kit check: no write the manifest declares `available` appears in any `holds`. |

```json
"version_check": {
  "state": "tested | checked | changed | unread",
  "version": "TrueNAS-26.04.0",
  "note": "the add-on's own sentence",
  "functions": [
    {"name": "user.update", "effect": "write", "verdict": "changed",
     "detail": "smb: was boolean, now string", "holds": ["apply", "password.set"]},
    {"name": "user.query", "effect": "read", "verdict": "unchanged"}
  ]
}
```

`state`: `tested` is every function unchanged on a version a baseline was
recorded on. `checked` is every function unchanged on any other version.
`changed` is at least one function changed or missing. `unread` is no
version or no method list yet. `holds` names the writes held or withdrawn
because of that function (operation ids, and `apply`); it comes from
`server.unavailable`, so health and the manifest cannot disagree. The names
are app-neutral: a UniFi or inventory add-on fills the same object.

**A held write is `unreached`: it spends no attempt and opens no breaker.**
Today the code is `retry: never`, so `classifyStatus` falls to
`OutcomeRejected` (`transport.go:598`, its `default`), and a queued
revocation would be refused for good. Three changes, each with a test:

- `classifyStatus` answers `OutcomeUnreached` for
  `TARGET_VERSION_UNSUPPORTED`.
- The breaker's not-counted list (`transport.go:744`) gains the code. As
  `unreached` it would otherwise reach `b.failures++`, and five held writes
  would take the system away from everybody.
- The drain reads `version_check` from the monitor's last health answer and
  claims no row of a system whose `holds` name `apply`, as it claims none
  for an unreachable system (§13.4). A row answered with the code anyway
  spends no attempt.

So a held revocation waits and lands by itself after the add-on update. It
never becomes "out of retries".

## 8. Failure modes

| Case | What happens |
|---|---|
| `core.get_methods` fails | If this process already checked this exact version string, that report stands. Otherwise every function is `unread`: reads `unchecked`, writes held, `state: unread`. **Retry:** `ensureProbed` gains the branch it already has for `auth.me` (`nas.go:451`): connected, probed, report unread, and `probeCooldown` (30 s) since the last try, so the next call runs the check again. |
| Add-on restart | The report is in memory only, so a restart checks again on the first connection (`main.go:325`, before any request). **Not persisted**, decided: a stored verdict would be a second copy that also depends on the key's roles and the add-on's own dependency table, and its only gain is the 30 s between a failed check and its retry, during which a held write waits without cost. Revisit only if 9a measures the call as slow or unreliable. |
| The check's own time | The call runs on a context detached from the request (`context.WithoutCancel`) with its own bound, `methodsCallLimit`, set from 9a's measurement. A first request's `withBudget` cannot cut it short, so a large reply cannot fail the check for ever. The request that waited may answer `TARGET_BUSY` once. |
| NAS down during the check | No version, no report, `state: unread`. Reads answer the outage (§13.1). Writes could not go anyway. |
| Version flip mid-session | TrueNAS restarts to change version, so the session dies. The new connection re-checks before its first call, and `sendWrite` and the purge re-read the verdict (§3). A write already on the wire is the existing in-doubt case. |
| A patch, a new line, a downgrade | Compared like anything else. Nothing relied-on changed: everything works, nobody is asked. Something did: those functions are held; the rest carry on. |
| A reply's shape breaks with the schema unchanged | The strict decode fails that read, every time, with the field named (§5). A write whose pre-read fails is not sent. |
| Same shape, different behaviour | Not visible to any schema check. Apply's read-back reports what TrueNAS stored, and drift's next pass shows the difference (§0). |
| Add-on updated, NAS not | The new dependency is compared like the rest. |
| The baseline file is unreadable | The add-on refuses to start. It is embedded and tested, so this is a build fault. |

## 9. Copy (for the copy guide §19; final words go through it)

`{product}` TrueNAS, `{version}` the short version ("26.04.0"), `{system}`
the system's name, `{n}`/`{m}`/`{k}` counts.

| Reader, where | Says |
|---|---|
| Operator, system page, `tested` | nothing. No notice at rest. |
| Operator, `checked` | {product} {version} detected: {n} functions checked fine. Nothing for you to do. |
| Operator, `changed` | {product} {version} changed {m} functions Syndra uses: user.update (changed), audit.query (missing). They stay off; everything else works. Update the add-on. |
| the same, when removals wait | {k} people who left still have access to {system}. Syndra removes it by itself once the add-on is updated. |
| Operator, `unread` | Syndra's add-on hasn't been able to check {product} yet. Changes wait, and it checks again by itself. Nothing for you to do. |
| Operator, the list, per function | Checked fine · Changed: {detail} · Missing · Not checked yet |
| Operator, a read marked `unchecked` | {product} changed how it answers this. Syndra shows it but doesn't act on it or count it. Update the add-on. |
| Operator, a read that failed its decode | {product} answers {function} differently from what Syndra's add-on expects. Nothing was read. Update the add-on. |
| Operator, a write held | Nothing went out. {product} {version} changed something this needs. Update the add-on; Syndra sends this by itself after that. |
| Member, a read marked `unchecked` | {system} was updated, so this may be incomplete. Makerspace staff know. Nothing for you to do. |
| Member, a write held | Nothing went out. {system} was updated and makerspace staff need to update Syndra first. Ask makerspace staff. |
| Usage, not counted | Not counted since {date}: {product} changed how it reports this. Update the add-on. |

The removals line is drawn first and as the notice's strongest line: it is
the one state here where someone holds access they should not.

Function names (`user.update`) are TrueNAS's own words, shown to operators
only, in `Mono`. A member never sees one.

## 10. Slices

In build order. Each is small and has a test that fails without it. All on
`feat/addon-contract`; none merges to main alone.

- **9a Capture.** Owner-approved, read-only, on the staging host (§4.1): the
  baseline file, and the research's unverified points settled (the JSON as
  answered, which methods each key sees, and the **size and time of the
  `core.get_methods` reply**, written into this slice's notes).
  *Test:* the committed file parses and holds an entry with `accepts`,
  `returns`, `job` for every method in the table.
- **9b `compat.Check`.** Pure: canonical form, projection, verdicts.
  *Tests,* each from the capture with one edit: a docstring edit, a new
  unrelated property and the FIXME keys removed are `unchanged`; a relied-on
  type changed is `changed` with its path; a field made nullable is
  `changed`; a new required argument is `changed`; a changed default on an
  unsent `user.create` property is `changed`; an entry removed is `missing`;
  no methods is all `unread`.
- **9c Contract.** Schemas, registry, fixtures, kit and guide of §7, with
  the least code that keeps both sides valid: the add-on emits
  `version_check` (from the old gate, until 9e) and `healthWire` reads it.
  *Tests:* the schema tests reject a health reply with `version_tested` and
  one without `version_check`; `classifyStatus` answers `unreached` for
  `TARGET_VERSION_UNSUPPORTED`; five such answers leave the breaker closed;
  `TARGET_SHAPE_CHANGED` reads as `addon_outdated`, not `app_unreachable`,
  and is not counted by the breaker; the kit fails an add-on that declares
  a write available and names it in `holds`.
- **9d Strict decode.** `compat.Require`; `roundTrip` and `decodeAnswer` use
  it; reply types named and registered. *Tests:* a recorded reply with one
  relied-on key renamed fails the read as `TARGET_SHAPE_CHANGED` naming the
  key (today it decodes to zero); a reply type whose field cannot hold a
  `null` the baseline schema allows fails the nullability test.
  *Mutation:* swap `Require` back to `json.Unmarshal`, red.
- **9e The add-on decides.** The check in `ensureProbed` with its own bound
  and its retry branch; `unavailable`, `sendWrite` and the purge read it;
  `/health` serves the real `version_check`; the old gate and
  `TRUENAS_SUPPORTED_MAJORS` deleted. *Tests,* against a fake NAS serving an
  edited capture: a changed `user.update` holds `password.set` and apply and
  leaves reads declared; everything unchanged on an unknown version sends an
  apply; a missing `audit.query` withdraws `activity.get`; a failing
  `core.get_methods` holds every write, and the call after `probeCooldown`
  checks again and releases them; a slow `core.get_methods` outlives the
  request's budget and still lands its report; a reconnect onto a changed
  version does not send a queued write; a purge whose session reconnected
  onto a changed `user.delete` after `operatedAccount` sends no
  `user.delete`.
- **9f `unchecked`.** The fresher's stamp, `/plan`, the collector.
  *Tests:* a read through a changed method answers `current: false`,
  `stale.reason: unchecked`; a collector run over it writes no point and
  holds the watermark.
- **9g Backend.** `ReadOf`'s `kept` with reason `unchecked`, drift's halt
  with its next step, the drain's hold, the waiting-removals count.
  *Tests:* the reachability table gains an `unchecked` stub on every route
  of §13.3 (data shown, dated, no plan, no count); a revocation queued while
  held spends no attempt across passes and lands on the first pass after the
  hold ends; the count equals the held removal rows.
- **9h UI and copy.** Board first (designer), then the notice with its
  removals line, the function list, the `unchecked` reading through
  `readOf`, and the §19 rows with their guard test. Layout check: the health
  read changes shape, so the gate is the staging run after deploy; say so in
  the PR.
- **9i Staging.** Owner-gated. The add-on there is `read_only`, so: the
  check reads `tested` on the real NAS; a test build with an edited baseline
  shows `changed`, the list and the held writes.

## 11. Decided by the owner, 23 Nov 2026

1. A patch where nothing Syndra uses changed needs no staff step.
2. No "Trust". One rule for every version (§0). Removed with it: the
   `trusted_version` setting, the `NOT_OFFERED` code, both audit events, the
   Trust dialog and the report ID.
3. A purge is allowed when `user.delete` and its pre-reads are unchanged.
4. A revocation through a changed function stays held, and the notice counts
   who is waiting.
5. 9a may read the real NAS once, read-only, with no Docker on the Mac.
