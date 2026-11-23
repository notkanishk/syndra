# The version check: an add-on checks what it depends on

Status: design, round 4, 24 Nov 2026: the owner's rulings, three Opus reviews
and the owner's addition (§12, his yes 24 Nov 2026, settled for 9j) folded in.
9b and 9c are built. Slices
are `tasks.md` §9. Extends `design.md` §13. Source facts:
`research-core-get-methods.md`.

Owner, 23 Nov 2026: "make it robust, elegant and be careful". When the TrueNAS
add-on meets a TrueNAS it was not tested on, it checks its own dependencies.
It neither trusts nor distrusts the version number, and it asks nobody to.

## 0. The shape in one paragraph

The add-on ships what TrueNAS's own method schemas looked like on the version
it was tested on. On every connection it asks the NAS for the same schemas
and compares only the parts Syndra uses. Each TrueNAS function comes out
**unchanged**, **changed** or **missing**. One rule then applies to every
version, patch or major:

- **Unchanged:** the function works, writes included.
- **Changed:** that function only is held. A read through it is shown but
  marked `unchecked`, so nothing plans from it or records it. A write through
  it waits for an add-on update.
- **Missing:** withdrawn.

Until the check has answered, the report is **unread** and the add-on is
simply not ready: every call through the NAS answers as a busy line does
today (§6.2). No new word, and nothing is concluded.

Every reply is also decoded strictly, so a field Syndra relies on that is
absent fails the read instead of becoming a zero. This is a consumer-driven
contract (Fowler, *Consumer-Driven Contracts*): the consumer states what it
relies on and the provider is verified against that, at run time, with a
tolerant reader (Fowler, *Tolerant Reader*) for everything else.

**There is no "Trust" step** (owner, 23 Nov 2026: if the check passes, why
trust?). A removal through an unchanged function goes ahead, so nobody keeps
access waiting on a button. The net for "same shape, different behaviour" is
the read-back: apply reads the account back after its writes
(`addons/truenas/apply.go:1166`, `readBack`). §12 (the owner's addition)
makes that read-back hold a function that twice did not do what it was
asked.

## 1. The fact this rests on

`core.get_methods` on TrueNAS 25.10, read from source at `TS-25.10.8`
(`truenas/middleware@29ed02b8`), full citations in
`research-core-get-methods.md`:

- Each entry is a dict. `accepts` is a list with one JSON Schema per call
  argument, in order. `returns` is a list with exactly one schema. `job` says
  the call answers a job id instead of a result. Source:
  `src/middlewared/middlewared/service/core_service.py` L247–L380.
- The call takes `service: str | None`, "Filters the result for a single
  service" (`api/v25_10_5/core.py:43-49`).
- The schemas are pydantic's, with `$ref`/`$defs` inlined, plus non-standard
  additions the source marks FIXME "should be removed eventually":
  `_attrs_order_`, `_name_`, `_required_`, `title` overwritten with the
  property name, and `items` wrapped in a one-element list. Source:
  `src/middlewared/middlewared/api/base/jsonschema.py` L10–L62.
- A CRUD `query` returns one schema of `list[Item] | Item | int`
  (`api/base/model.py:307-314`): an `anyOf` of three branches. A nullable
  object is an `anyOf` of the object and `null`.
- The entry also carries the docstring. A docs-only patch changes a naive
  hash.
- **The reply is filtered by the caller's credential.** A method the key may
  not call is absent. `core.get_methods` itself needs no role.
- **Patch releases change schemas.** 25.10.0 to 25.10.8 moves the current API
  through five model versions; `user`, `sharing.smb` and `audit` models
  changed. 25.10.5 to 25.10.8 share one. So a version number proves nothing
  either way.
- On the 26 line the method stays public; `accepts`, `returns` and `job`
  stay.

Not verified from source, and settled by slice 9a's capture: the exact JSON
for our methods, the reply's size and time (whole and per service).

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
6. **One code for "the add-on is behind its app":**
   `TARGET_VERSION_UNSUPPORTED` (§7). A held write and a reply that failed
   its decode need identical handling, so they are one registry entry, one
   `classifyStatus` branch and one breaker entry.
7. **`user.delete` is never in the shared report** (§3.1). The add-on's own
   key cannot see it, by design.

## 3. The module and its seam

One deep module, `addons/truenas/compat`, pure: no network, no store, no
clock. Its whole interface:

```go
// What Syndra relies on, per TrueNAS method. One table, in the add-on.
type Dep struct {
    Replies []any     // zero values of every Go type a call site decodes into
    Sends   []string  // JSON Pointers (RFC 6901) into accepts that Syndra sends
}

func Check(baselines []Baseline, deps map[string]Dep,
    methods map[string]json.RawMessage) Report   // methods nil: nothing was read

func (r Report) Verdict(method string) Verdict   // unchanged | changed | missing | unread, with Detail

func Require(raw json.RawMessage, into any) error  // the strict decode (§5)
```

`unread` is the whole report or none of it: `methods` was nil. An entry that
is listed but lacks `accepts`, `returns` or `job` is `changed`, because it is
not what the baseline holds.

Whether a method is a read or a write is not in `Dep`: `readMethods`
(`nas.go:617`) already says, and stays the one source.

Hidden behind the interface: parsing the capture, canonical form,
projection, the diff and its wording. Two real callers feed `Check` the same
type (the embedded capture and the live reply), so there is no interface and
no fake: tests pass JSON.

**Where it runs.** `NAS.ensureProbed` (`nas.go:449`), where `loadMethods`
runs today (`nas.go:479`): once per connection, after `system.version`.
TrueNAS cannot change version without dropping the session, so per
connection is exactly "on version change". The finished `Report` is swapped
in under `n.mu` as one immutable pointer; readers copy the pointer and read
without a lock. No new goroutine.

**A new connection starts unread.** The line that resets `probed` on
reconnect (`nas.go:341`) also sets the report to `unread`, under the same
lock. Without that, a second caller passes `ensureProbed` (the flag is set
before the probe runs), takes the line (`lineUp`, `nas.go:252`) ahead of the
probing caller's `system.version`, and has its write judged by the old
version's report. With it, that caller answers busy and sends nothing
(§6.2). The window is the length of the check's own calls.

**What goes.** `loadMethods` and its call (`nas.go:479`); `MethodPresent`
("unknown is present") and its call (`server.go:179`); `MajorSupported` and
its four calls (`server.go:175`, `server.go:315`, `apply.go:638`,
`main.go:329`); `majorOf`; `supportedMajs` (`nas.go:185`) and
`TRUENAS_SUPPORTED_MAJORS` (`main.go:165`, README). That variable let a
deployment claim 25.04 and 26.04 were tested when only 25.10.7 ever was.

**The one decision.** `server.unavailable(operation)` (`server.go:165`)
stays the single source for the manifest and for the route's gate (§6).
`NAS.sendWrite` (`nas.go:612`) repeats the verdict for its own method after
`ensureProbed`, because the session may have reconnected since the gate
(`ErrNotSent`).

### 3.1 The purge checks `user.delete` on its own session

The add-on's key holds `ACCOUNT_WRITE` and `SYSTEM_AUDIT_READ` "and
deliberately not deletion" (`operations.go:556`), and the methods reply
lists only what the key may call (§1). In the shared report `user.delete`
would read `missing` on every version and `account.purge` would be withdrawn
for ever. So:

- `user.delete` has its own one-entry table, `elevatedDeps`. It is not in
  `deps`, not in the shared report and not in `version_check`.
- `account.purge` is declared available from its pre-reads alone.
- At purge time, after `operatedAccount` and on the elevated session it has
  just opened (`operations.go:597`), the add-on calls `core.get_methods`
  with `service: "user"` and runs the same `Check` with `elevatedDeps`.
  `unchanged` sends `user.delete` (`operations.go:605`). Anything else sends
  nothing and answers `TARGET_VERSION_UNSUPPORTED`; `missing` here says the
  pasted key may not delete.
- The check and the delete share one connection, and a connection cannot
  change version. So the purge has no flip window and needs no re-read. Its
  pre-reads' verdicts are read again from the shared report after
  `operatedAccount`.

No elevated key is stored, cached or reused: the operator pastes it per
purge, as today.

**How its baseline is captured: the owner pastes the elevated key once,
during 9a** (recommended). The recorder asks for it with a silent prompt,
opens one more session, makes one call (`core.get_methods`,
`service: "user"`), keeps the `user.delete` entry and closes. The key is in
no file, no shell history and no log. Why this one:

- It is the same source and the same code as every other entry: the NAS's
  own answer, projected by the same `Check`. One source, one path.
- It fits the owner's rule for that key: pasted per action, never stored.
- The alternative, the NAS's API docs bundle, is a second source in a second
  format that would need its own parser, and the docs page on this NAS says
  v25.10.5 while the NAS reports 25.10.7. That label is probably the API
  model version (`api/v25_10_5`, shared by 25.10.5 to 25.10.8, §1), but
  "probably" is not a baseline. If the owner pastes no key, 9a records no
  `user.delete` entry, the purge-time check answers `changed` and the purge
  stays held: fail closed, and visible in 9a's notes.

## 4. The baseline and the comparison

### 4.1 Where the baseline lives

`addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json`, embedded with
`go:embed`: `{version, methods: {name: {accepts, returns, job}}}`, as the NAS
answered, for the methods in `deps` and `elevatedDeps` and no others. One
file per tested version; several may ship.

**The one command** after testing a new version:
`scripts/record-truenas-fixtures.sh --methods`. A new mode of the existing
recorder that makes two reads (`system.version`, `core.get_methods`) and
writes the baseline file only. Without a NAS there is nothing to run: the
projection is computed from the committed capture at test time and at start.

The mode is built so the capture cannot leak or damage anything:

- **It refuses `--write` outright** when given with it. No account is
  created on the way to a schema.
- **It reads no member data.** The mode skips every other probe, so audit
  events, share names and paths, quota rows and ACLs are never fetched, not
  even into memory.
- **It keeps only listed methods and only `accepts`, `returns`, `job`.**
  `description`, `cli_description`, `examples` and every other entry key are
  dropped. A reply that is not a map of entries is refused.
- **It does not touch `truenas_observed.json`.** A read-only run rebuilds
  that file from the previous recording (`record-truenas.py:130-134`); run
  from a temp directory there is no previous recording, so `write_rules`
  would come back empty and gut `truenas_rules_test.go`.
- **It prints no address.** Today's first line prints `TRUENAS_URL`
  (`record-truenas-fixtures.sh:68`). The mode prints the version, each
  method name seen, and the size and time of each reply. The run log is
  never committed and never pasted into `tasks.md`.
- **The container is pinned.** The image by digest and `websockets` by
  version and hash (today: `python:3.12-alpine` and an unpinned
  `pip install`, `record-truenas-fixtures.sh:74`), because the key is in the
  container's environment.

**Where it runs: on the staging host, never on the Mac.** The recorder runs
its probe in a container (`record-truenas-fixtures.sh:65`), and the owner
forbids local Docker. So for 9a: copy the slice's `scripts/` to a temp
directory on the staging host over ssh, run `--methods` there with
`TRUENAS_URL` and the key read from the staging add-on's own env file, copy
back **the baseline file only**, delete the temp directory. The host is the
operator's shell configuration, never written in the repo. Nothing is
deployed, the staging stack is not touched, and the staging add-on stays
`read_only`.

### 4.2 What Syndra relies on

- **Returns:** every JSON path a registered reply type reads, derived by
  reflection from the Go struct's `json` tags. The Go type picks the way
  down at each step:
  - a struct descends `properties`; a slice descends `items`;
  - **at an `anyOf` or `oneOf`, it descends every branch that can hold the
    step**: a slice takes the `array` branches, a struct the `object`
    branches, a scalar the branches of its own type. `null` branches and
    branches of another kind are not descended. So `[]user` reads
    `user.query` through the list branch of `list | item | int`, and a
    pointer to a struct reads through `object | null`.
  - A path is present when at least one branch holds it. A relied-on path no
    branch holds fails the test: the struct names something TrueNAS never
    declared.

  At each path the projection keeps `enum`, `format`, `const` and the `type`
  of **every** branch at that node, descended or not, so a field that
  becomes nullable, or a query that loses its list form, is seen.
- **Accepts**, for a write: the number and order of arguments; the subschema
  at each pointer in `Sends`; and, for every object Syndra sends into, its
  `required` list and `additionalProperties`. A new required field Syndra
  does not send is a change. A new optional one is not.
- **Defaults**, for every write: the `default` of every argument and
  argument property Syndra does not send, its `required` list, and an unsent
  argument's `_required_: true` (TrueNAS marks a required argument only
  there). Where a schema declares a default, an unsent value takes it, so a
  changed default changes what the write does. An update declares none
  (fields left out keep their value, RFC 7396 §2), so a default appearing
  there means updates begin resetting what Syndra omits. Examples: `group.delete`'s `delete_users`, `user.delete`'s
  `delete_group`, `user.create`'s unsent fields. The subschema kept at a sent
  pointer drops its `default`: what Syndra sends is never defaulted.
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
| `changed` | It differs, or the entry lacks `accepts`, `returns` or `job`. `Detail` names each path with "was / now". |
| `missing` | The NAS lists no such method for the key: removed, renamed, or the key's role no longer allows it. Syndra cannot call it. |
| `unread` | No method list was read. Every function at once; nothing is concluded (§6.2). |

A doc-text change, a new unrelated field, a reordered `required` and the
removal of the FIXME keys are all `unchanged`.

## 5. Reads fail closed: one strict decode

`NAS.roundTrip` decodes every reply in one place (`nas.go:662`), and
`decodeAnswer` the health sources. Both call `compat.Require`, which
replaces plain `json.Unmarshal` and closes the `ponytail:` note at
`operations.go:821`.

**"Relies on" means** every field with a `json` tag in the type the call
site decodes into. One source, the Go struct. For each such field, at every
depth and in every row:

- the key is present, unless the field is tagged `nas:"optional"`;
- its JSON kind fits the Go type;
- `null` is accepted only by a pointer, slice, map or `json.RawMessage`.

**Nullability comes from the baseline schema, not from one recorded reply.**
A test walks every registered reply type against the baseline `returns`
schema, by §4.2's descent: where a path has a `null` branch, the Go field
must be able to hold it, or the test fails. A reply recorded on one day
shows only what was null that day. Pydantic's `required` list describes what
a caller must send, so it is not used for replies.

**A decode failure is not a TrueNAS outage.** Today's "an answer the add-on
could not use" is `TARGET_UNREADABLE`, which the backend reads as
`app_unreachable` (`backend/internal/addons/unread.go:137`) with the words
"Check TrueNAS, then read again" (`handlers/not_read.go:125`). That next
step is wrong here: TrueNAS is fine and reading again changes nothing. So a
failed `Require` answers `TARGET_VERSION_UNSUPPORTED` (§7) with
`details: {functions: [f], field, verdict}`. `TARGET_UNREADABLE` keeps its
meaning for an answer that is no answer (a bare `null`, not JSON).

**Only before anything was sent.** The code says "nothing was sent", and the
backend sends such a row again by itself. So it is answered only for a
decode that fails before the call's first write: a read, or a write's
pre-read. A decode that fails on the read-back after a write keeps today's
outcome, `applied` and `Unverified` (`apply.go:1166`): the write happened
and is never sent twice.

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
| any `changed` | stays declared; its reply is `unchecked` (§6.1) | held: `TARGET_VERSION_UNSUPPORTED`, nothing sent |
| any `missing` | withdrawn: `available: false`, reason names the function | withdrawn, same |
| `unread` | stays declared; answers busy (§6.2) | stays declared; answers busy, nothing sent |

A write also depends on the reads it makes first. `methodsFor`
(`server.go:191`) grows to name every method of every operation, the apply
and the collector; a test fails when a call site uses a method its operation
does not list.

**The purge** (`account.purge`, irreversible) follows the same rule through
§3.1: an operator already types the elevated key and confirms each time.
Allowed when `user.delete` (on the elevated session) and its pre-reads are
unchanged, held when any changed.

**A revocation through a changed function stays held.** Sending
`user.update` in a shape TrueNAS no longer documents is the risk this exists
to stop. The notice counts the departed members still waiting to lose
access, prominently (§9). The count is the system's unsent removal rows in
the outbox while its pass is halted for the hold (§7); nothing new is
stored.

### 6.1 `unchecked`: one flag, on the path that exists

A read reply already says whether it can be concluded from: `current`, and
`stale` when it cannot (`common/stale.json`). The flag is one more reason
there, **`stale.reason: "unchecked"`**, with `current: false` and the
add-on's sentence in `note`. `taken_at` stays the real moment of the read.
It is set for a `changed` function only.

Why not a new sibling field: every consumer already refuses to conclude from
a reply that is not current. A new field would be safe only once every
consumer learned it; this is safe on the day it ships.

**This is one new word in `design.md` §13.1's vocabulary**, and §13.1 and
§13.3 are updated with it: `unchecked` joins the kept-copy reasons.

- **Stamped once**, where `Current: true` is stamped today (`fresher.read`,
  `fresh.go:229`): `NAS.call` notes each `changed` method it served on the
  request's context, and the fresher reads the note. `/plan` and the kept
  copy carry it the same way.
- **Usage history.** `collectStatsOnce` treats an unchecked read as a failed
  one (`failedRun`, `stats_collect.go:107`): the watermark holds and nothing
  is written. No change to `stats.json`.
- **Backend: no new read state.** `ReadState.None()` is true for anything
  but `now` and `kept` (`read_state.go:38`), so a third state would hide the
  data at every site that asks it, and the UI draws its dated card for
  `kept` only. `ReadOf` maps the reply to
  `read: {state: "kept", taken_at, reason: "unchecked", detail}`. The data
  is shown with its warning, as every kept copy is.
- **Drift** halts as `addon_outdated`, the reason that already exists with
  the right next step ("Update the add-on"). `SubjectsResult.Halt()`
  (`read.go:486`) is the one mapping and gains the case; drift's
  `!read.Current` branch (`services/drift/addon.go:250`) asks it. No
  migration.

### 6.2 `unread` is "not ready", and that already has a path

`unread` is not `changed`, and telling staff to update the add-on would be
wrong: there is nothing for them to do. It is the add-on not having finished
asking, which is exactly what a busy line is. So while the report is unread,
`NAS.roundTrip` answers every method but the check's own (`system.version`,
`auth.me`, `core.get_methods`) with the error `lineUp` already gives
(`ErrNotSent` and `ErrBusy`, `nas.go:252`), before anything is sent. Its
`Retry-After` is the time left until the check's next try (§8's backoff), at
least 1 s, so "Try again in {n} s" stays true as the wait doubles.

Everything after that exists and is unchanged: a read serves its kept copy
with `stale.reason: busy` or answers `TARGET_BUSY`; a write answers
`TARGET_BUSY`, is `unreached`, spends no attempt and opens no breaker; the
sentence is "{product} is busy right now. Try again in {n} s."; busy without
a break for over 30 minutes halts the send with its own notice (`design.md`
§13.1). No
code, reason, state or sentence is added for `unread`. `health` alone names
it, as `version_check.state: "unread"`, for the system page (§9).

With no session at all the NAS is down, and the outage answers as it does
today.

## 7. How the result reaches the backend

One path each. No back-compat inside contract 2 (owner, 13 Nov): the old
fields go in the same change, both containers deploy together.

| Surface | Change |
|---|---|
| Manifest | unchanged in shape. `available` and `unavailable_reason` now come from §6. |
| `health.json` | `version_tested` and `version_note` are **removed**. `version_check` is added and required. |
| `common/stale.json` | `reason` gains `unchecked`. `current`'s description gains "and whose shape the add-on has checked". |
| `errors.json` | `TARGET_VERSION_UNSUPPORTED` (`errors.json:259`): "The add-on is behind its app: a function this needs is missing or changed on this app version, or answered in a form the add-on does not know. Nothing was sent." `details: {functions: [...], field?, verdict}`. `retry` moves from `never` (`errors.json:259-265`, `addons/truenas/errors.go:88`) to `retry_after`, and its status from 422 to 503 with `Retry-After` (RFC 9110 §15.6.4, §10.2.3), as `TARGET_BUSY`: the two places that write it are `operations.go:155` and `apply.go:641`. A missing method no longer answers `OPERATION_FAILED`, which can mean "may have reached the app". |
| Kit, `AUTHORING.md` ch. 2 step 2, reference add-on | the gate rule is rewritten to this; the reference answers `state: "unchanged"`. The guide's three mentions of the code already say 503 (round 3); the kit's reads-only fixture (`kit_test.go:1923-1931`) still sends it as 422 and moves in 9c. New kit check: no write the manifest declares `available` appears in any `holds`. |
| `design.md` §13.1, §13.3 | one table row and the kept-copy reason `unchecked` (done with this revision). |

```json
"version_check": {
  "state": "unchanged | changed | unread",
  "version": "TrueNAS-26.04.0",
  "note": "the add-on's own sentence",
  "functions": [
    {"name": "user.update", "effect": "write", "verdict": "changed",
     "detail": "smb: was boolean, now string", "holds": ["apply", "password.set"]},
    {"name": "user.query", "effect": "read", "verdict": "unchanged"}
  ]
}
```

`state`: `unchanged` is every function unchanged, on any version. `changed`
is at least one function changed or missing. `unread` is no method list yet,
with `functions` empty. Whether the version is one a baseline was recorded
on is not said: it changes nothing anyone does. `effect` comes from
`readMethods`. `holds` names the writes held or withdrawn because of that
function (operation ids, and `apply`); it comes from `server.unavailable`,
so health and the manifest cannot disagree. The names are app-neutral: a
UniFi or inventory add-on fills the same object.

**The code is `unreached`: it spends no attempt and opens no breaker.**
Today it is `retry: never`, so `classifyStatus` falls to its `default`,
`OutcomeRejected` (`transport.go:611`), which is terminal
(`transport.go:62`): a queued revocation would be refused for good. The
backend changes in five places, each with a test:

- `classifyStatus`: the code joins the `OutcomeUnreached` list at
  `transport.go:606`, above the 5xx branch, with `shape` false. It must not
  be a `ShapeRefusal`: that re-reads the manifest (`pipe.go:117`) and the
  breaker counts it.
- `Unread`: a case before the catch-all at `unread.go:143` maps the code to
  the existing reason `addon_outdated`, with `details.functions` as the
  detail and the reply's `Retry-After`. Left alone it is `read_refused`.
- The breaker's not-counted list (`transport.go:744`) gains the code. It is
  the add-on answering on purpose, and five held writes would otherwise take
  the system away from everybody.
- The add-on pass's no-attempt case (`propagation/addon.go:282-285`) takes
  the code beside `Why.Unreachable()`. `addon_outdated` is neither busy nor
  `Unreachable()` (`unread.go:81-82`), so left alone the row falls to the
  case below (`addon.go:287`), spends an attempt, and ends "out of retries".
  With it the row goes through `deferUnsent`, the one path for "nothing went
  out": pending again, no attempt spent, due after `Retry-After`. The case
  asks for the code, not the reason, so a shape refusal behaves as today.
- `haltReason` (`propagation/drain.go:304`) reads `version_check` from the
  monitor's last health answer and halts the pass, as `addon_outdated`, for
  a system whose `holds` name `apply`. The rows wait unsent, as they do for
  a system in maintenance.

The add-on changes in the two places the table names: the registry entry
with its Go twin, and the status at `operations.go:155` and `apply.go:641`.

A row meets the code past the pass halt in one case: **a write's pre-read
fails its decode while the schema is unchanged**, so nothing is in `holds`.
The fourth place is what makes it wait like the rest instead of being
refused for good.

`Retry-After` is the cap of the check's backoff (§8): a version hold (§7) ends on a
new connection or a new add-on, not sooner. A misbehaviour hold (§12) does not.

`addon_outdated`'s sentence (`not_read.go:138`) branches on the detail. With
functions named: "{product} answers {functions} differently from what
{system}'s add-on expects. Nothing was read. Update the add-on." Without:
today's "{system}'s add-on doesn't offer this. Update the add-on." A member
reads neither; `not_read.go:92` already gives them the plain sentence.

So a held revocation waits and lands by itself after the add-on update. It
never becomes "out of retries".

## 8. Failure modes

| Case | What happens |
|---|---|
| `core.get_methods` fails | The report stays `unread` and everything answers busy (§6.2). The backoff below asks again. |
| Retrying the check | `ensureProbed` has one retry branch for an incomplete probe, replacing the `auth.me` early return (`nas.go:451-458`), which today returns before anything else is retried. Connected, and the self-account or the report still unknown: the next call after the wait runs whichever is missing, once. So the check retries while `selfName` is empty, and the two never double the traffic. The wait is a capped exponential backoff, `probeCooldown` (30 s, `nas.go:90`) doubling to 10 min, reset by a new connection. |
| The check's own cost | It asks per service, with the `service` filter (§1), one short call for each service in the table. One unfiltered call rebuilds every schema on the NAS while holding the shared line for up to its bound. Each call runs on a context detached from the request (`context.WithoutCancel`) with its own bound, `methodsCallLimit`, set from 9a's measurement. A first request's `withBudget` cannot cut it short. Any service failing leaves the whole report `unread`. 9a measures both forms; the single call replaces this only if it is the faster in total. |
| Add-on restart | The report is in memory only, so a restart checks again on the first connection (`main.go:325`, before any request). **Not persisted**, decided: a stored verdict would be a second copy that also depends on the key's roles and the add-on's own dependency table. (§12's holds are evidence, not verdicts, and are stored.) |
| NAS down during the check | No version, no report, `state: unread`. Reads answer the outage (`design.md` §13.1). Writes could not go anyway. |
| Version flip mid-session | TrueNAS restarts to change version, so the session dies. The new connection starts `unread` (§3), so no call is judged by the old report, and `sendWrite` reads the verdict after `ensureProbed`. The purge checks on the connection it sends on (§3.1). A write already on the wire is the existing in-doubt case. |
| A patch, a new line, a downgrade | Compared like anything else. Nothing relied-on changed: everything works, nobody is asked. Something did: those functions are held; the rest carry on. |
| A reply's shape breaks with the schema unchanged | The strict decode fails that read, every time, with the field named (§5). A write whose pre-read fails is not sent and waits without spending an attempt (§7). |
| Same shape, different behaviour | Not visible to any schema check. Apply's read-back sees it. Today it shows as drift and is queued again, which can loop; §12 holds the function instead. |
| Add-on updated, NAS not | The new dependency is compared like the rest. |
| The baseline file is unreadable | The add-on refuses to start. It is embedded and tested, so this is a build fault. |

## 9. Copy (for the copy guide §19; final words go through it)

`{product}` TrueNAS, `{version}` the short version ("26.04.0"), `{system}`
the system's name, `{n}`/`{m}`/`{k}` counts.

| Reader, where | Says |
|---|---|
| Operator, system page, `unchanged` | nothing. No notice at rest, on any version. |
| Operator, `changed` | {product} {version} changed {m} functions Syndra uses: user.update (changed), audit.query (missing). They stay off; everything else works. Update the add-on. |
| the same, when removals wait | {k} people who left still have access to {system}. Syndra removes it by itself once the add-on is updated. |
| Operator, `unread` | Syndra's add-on hasn't finished checking {product} yet. Changes wait, and it checks again by itself. Nothing for you to do. |
| the same, once the send has halted as busy (30 minutes) | Syndra's add-on still can't check {product}. Read the add-on's log. |
| Operator, the list, per function | Checked fine · Changed: {detail} · Missing |
| Operator, a read marked `unchecked` | {product} changed how it answers this. Syndra shows it but doesn't act on it or count it. Update the add-on. |
| Operator, a read that failed its decode | {product} answers {function} differently from what {system}'s add-on expects. Nothing was read. Update the add-on. |
| Operator, a write held (the row's `last_error`) | {system} changed something this needs; nothing was sent. Update the add-on. Syndra sends it again by itself. With the background drain off, or a row that waits for you (`role_waits`, `manual`), the last sentence is: Then press Send on Pending changes. (One ending with busy's, `sendsAgain`.) |
| Member, a read marked `unchecked` | {system} was updated, so this may be incomplete. Makerspace staff know. Nothing for you to do. |
| Member, a write held | Nothing went out. {system} was updated and makerspace staff need to update Syndra first. Ask makerspace staff. |
| Usage, not counted | Not counted since {date}: {product} changed how it reports this. Update the add-on. |

A call that met `unread` reads the existing busy sentence; it has no row
here.

The removals line is drawn first and as the notice's strongest line: it is
the one state here where someone holds access they should not.

Function names (`user.update`) are TrueNAS's own words, shown to operators
only, in `Mono`. A member never sees one.

## 10. Slices

In build order. Each is small and has a test that fails without it. All on
`feat/addon-contract`; none merges to main alone. Round 2 swapped the old 9d
and 9e: the strict decode's `details.verdict` needs the report.

- **9a Capture.** Owner-approved, read-only, on the staging host (§4.1),
  with the recorder's `--methods` mode. The owner pastes the elevated key
  once for the `user.delete` entry (§3.1). Notes record: which methods each
  key lists, and the **size and time of `core.get_methods`**, whole and per
  service. *Tests:* the committed file parses and holds `accepts`,
  `returns`, `job` for every method in `deps` and `elevatedDeps`; a walk of
  the file finds only schema keywords, property names and types (no entry
  key but those three, no method outside the tables); the recorder given
  `--methods --write` exits non-zero before connecting.
- **9b `compat.Check`.** Pure: canonical form, projection, verdicts.
  *Tests,* each from the capture with one edit: a docstring edit, a new
  unrelated property and the FIXME keys removed are `unchanged`; a relied-on
  type changed is `changed` with its path; a field made nullable is
  `changed`; a new required argument is `changed`; a changed default on an
  unsent `user.create` property is `changed`; **`job` flipped is
  `changed`**; an entry without `returns` is `changed`; an entry removed is
  `missing`; no methods is all `unread`. **`[]user` resolves every path of
  `user.query` through the `anyOf`, and the capture with its list branch
  removed is `changed`.** `user.delete` checked with `elevatedDeps` is
  `unchanged`, and is absent from a report built from `deps`.
- **9c Contract.** Schemas, registry, fixtures, kit and guide of §7, and
  its four backend places and two add-on places, with
  the least code that keeps both sides valid: the add-on emits
  `version_check` (from the old gate, until 9d) and `healthWire` reads it.
  *Tests:* the schema tests reject a health reply without `version_check`
  (one still on `version_tested` has none); a 503 `TARGET_VERSION_UNSUPPORTED` is
  `unreached`, not `indeterminate`, and not a shape refusal (the manifest is
  not read again); five such answers leave the breaker closed; `Unread`
  answers `addon_outdated` with the functions, never `read_refused` or
  `app_unreachable`; **a revocation row answered with the code is pending
  again with its attempt count unchanged** (*mutation:* drop the code from
  `addon.go:282`, red); the kit's reads-only fixture sends the code as 503; the kit fails an add-on that
  declares a write available and names it in `holds`.
- **9d The add-on decides.** The check in `ensureProbed`, per service, on
  its own bound, with the one retry branch and its backoff; the report
  `unread` at reconnect; busy while unread; `unavailable` and `sendWrite`
  read the report; the purge's check on its elevated session; `/health`
  serves the real `version_check`; `details` required for this code in
  `error.json` (9c left it optional; `AUTHORING.md` already says MUST);
  `Retry-After` on this code set to the check's next try, replacing
  `writeError`'s 30 s default; the old gate and
  `TRUENAS_SUPPORTED_MAJORS` deleted. *Tests,* against a fake NAS serving an
  edited capture: a changed `user.update` holds `password.set` and apply and
  leaves reads declared; everything unchanged on an unknown version sends an
  apply; a missing `audit.query` withdraws `activity.get`; a failing
  `core.get_methods` answers every call busy and sends no write, and the
  call after the wait checks again and releases them; **with `auth.me` and
  `core.get_methods` both failing, the next due call asks each exactly once,
  and the report is read while `selfName` is still empty**; the wait doubles
  to its cap; a slow `core.get_methods` outlives the request's budget and
  still lands its report; **after a reconnect onto a changed version, a
  second writer that takes the line while the fake NAS withholds
  `system.version` sends nothing**; a purge with `user.delete` changed on
  the elevated session sends no `user.delete`, and one with it unchanged
  sends it though the ordinary key does not list it.
- **9e Strict decode.** `compat.Require`; `roundTrip` and `decodeAnswer` use
  it; reply types named and registered. *Tests:* a recorded reply with one
  relied-on key renamed fails the read as `TARGET_VERSION_UNSUPPORTED`
  naming the function, the key and the verdict (today it decodes to zero);
  **an apply whose pre-read fails its decode sends no write and answers the
  code; one whose read-back fails it answers `applied`, `Unverified`, never
  the code**; a reply type whose field cannot hold a `null` the baseline
  schema allows fails the nullability test. *Mutation:* swap `Require` back
  to `json.Unmarshal`, red.
- **9f `unchecked`.** The fresher's stamp, `/plan`, the collector.
  *Tests:* a read through a changed method answers `current: false`,
  `stale.reason: unchecked`; a collector run over it writes no point and
  holds the watermark.
- **9g Backend.** `ReadOf`'s `kept` with reason `unchecked`, `Halt()`'s
  `addon_outdated`, `haltReason`'s hold, the sentence's branch, the
  waiting-removals count. *Tests:* the reachability table gains an
  `unchecked` stub on every route of `design.md` §13.3 (data shown, dated, no plan, no
  count); a revocation queued while held spends no attempt across passes and
  lands on the first pass after the hold ends; the count equals the held
  removal rows.
- **9h UI and copy.** Board first (designer), then the notice with its
  removals line, the function list, the `unchecked` reading through
  `readOf`, and the §19 rows with their guard test. Layout check: the health
  read changes shape, so the gate is the staging run after deploy; say so in
  the PR.
- **9i Staging.** Owner-gated. The add-on there is `read_only`, so: the
  check reads `unchanged` on the real NAS; a test build with an edited
  baseline shows `changed`, the list and the held writes.
- **9j Held on misbehaviour** (§12). The owner's addition; his yes, 24 Nov
  2026. Built last and alone: nothing in 9a to 9i depends on it.

## 11. Decided

By the owner, 23 Nov 2026:

1. A patch where nothing Syndra uses changed needs no staff step.
2. No "Trust". One rule for every version (§0). Removed with it: the
   `trusted_version` setting, the `NOT_OFFERED` code, both audit events, the
   Trust dialog and the report ID.
3. A purge is allowed when `user.delete` and its pre-reads are unchanged.
4. A revocation through a changed function stays held, and the notice counts
   who is waiting.
5. 9a may read the real NAS once, read-only, with no Docker on the Mac.

In round 2, from the second Opus review:

6. One code, `TARGET_VERSION_UNSUPPORTED`. `TARGET_SHAPE_CHANGED` is not
   added.
7. `unread` is the busy path (§6.2), not `changed`'s message.
8. Removed: `Dep.Write`; `Report.Tested()` and `Check`'s `version`
   argument; the `tested`/`checked` split and its notice; `unread` as a
   per-function verdict; the fixed 30 s retry.

In round 3, from the third Opus review:

9. Removed: the per-version report restore (§8; the backoff covers it); the
   refused-write signal and its closed-schema test (§12.1); the add-on
   version in a hold's key (§12.3).
10. Try again sends its one row through the add-on pass, not `DrainOne`
    (§12.4).

By the owner, 24 Nov 2026:

11. Yes to §12 (slice 9j).
12. A closed-field mismatch is answered by one second try in the same call
    before anything is held (§12.1). Never the purge.
13. Try again stays, for after a hold.

In round 4, from the Opus check of §12:

14. Try again names one apply by its `call_id`, and only that apply's
    closed-field read-back clears the hold. Removed: the "no row waiting,
    the next write takes the slot" fallback (§12.4).
15. The backend picks the trial row, not the operator. A narrowed hold's
    trial is that account's own row, queued by `convergeOn` (§12.4).
16. `functions[].observed` is a list of records, not a boolean (§12.5).

## 12. Held on misbehaviour (slice 9j; the owner's addition, his yes 24 Nov 2026)

Owner, 23 Nov 2026. A schema check cannot see a function that keeps its
shape and stops doing its job. Today apply's read-back sees it, reports it,
drift queues the change again, and the loop never ends.

Settled 24 Nov 2026, by the owner: yes to this section; one mismatch is
answered by a second try before anything is held (§12.1); Try again stays,
for after a hold, bound to one call (§12.4).

**Rule.** A function that misbehaves in use is held through the same
verdict, withdrawal and notice as a changed one. There is no second
mechanism: `Check` takes one more input and everything after it is §6 and
§7 unchanged.

```go
type Observed struct{ Method, Detail string }   // a function seen misbehaving twice (§12.1)

func Check(baselines []Baseline, deps map[string]Dep,
    methods map[string]json.RawMessage, observed []Observed) Report
```

A function `unchanged` by its schema and named in `observed` comes out
`changed`, with the hold's sentence as its `Detail`. Only a function's own
record is passed in. A narrowed record (§12.4) is one account's, so it is
not a verdict on the function. Its `functions` entry lists the stored
records under `observed` (§12.5), which is what the page needs to offer
**Try again**.

### 12.1 What counts

One signal: **a write that did not take, twice** (the second try, below).
The write reported success and the read-back that follows
(`apply.go:1166`, and the create's at `apply.go:1345`) shows a closed field
Syndra just wrote holding another value: `locked: true` asked,
`locked: false` read. It is a completed exchange in which TrueNAS said yes
and did otherwise.

Only closed fields are compared: booleans, enums and consts, such as
`locked`, `smb` and `password_disabled`. `home` and `groups` are not:
TrueNAS normalises and resolves them on write (`apply.go:1158-1160`), so a
different value there is not a fault. A password is never read, so never
compared.

**Never counts:** a refusal of any kind, busy, unreachable, a timeout, rate
limiting, `TARGET_CHANGED`, a missing path, and a read-back that failed or
was `Unverified`.

A refusal is left out on purpose. TrueNAS has rules across fields that no
schema shows (`smb: true` is refused unless the same call sets a password,
`apply.go:968-972`), and a staff edit between the pre-read and the write trips
the same refusal, so a refused value the schema allows proves nothing. And a
refusal does not loop: it is terminal today, `target_refused`
(`propagation/addon.go:241-263`), and already waits on staff.

The reason names what was asked and what was read: "user.update: asked
`locked: true`, read `locked: false`". Closed fields only, so it can never
hold a secret or free text.

#### The second try (owner, 24 Nov 2026)

One mismatch holds nothing. The add-on sends the closed fields again at
once and reads back again. Only a second mismatch is the signal above.

- **Where.** `converge`, in one helper that takes the place of the bare
  read-back at `apply.go:1166`: `confirm` reads the account back, compares
  `closed(update)` with what it read and, on a difference, sends
  `user.update` once more and reads back once more. It returns the account
  as last read and the difference still left, if any. `converge` is its only
  caller, so nothing but an apply's update can reach it.
- **What is resent.** `closed(update)`: the closed fields of the first
  write, with the same values, and nothing else. One projection serves the
  comparison and the resend, so the two cannot disagree on what a closed
  field is. It is idempotent (RFC 9110 §9.2.2): each field is an absolute
  value of the same desired state, so sending it twice leaves what sending
  it once should have. `groups` and the home fields are never resent:
  `home_create` is not idempotent (`home.go:123-133`, a second one nests the
  home a level down).
- **Same call.** The same `/apply` call, the same `caller` and so the same
  `call_id`, through `s.send` (`apply.go:1527`) on the same NAS session. The
  backend gets one answer: no new row, no attempt spent. The resend is its
  own `mutate` (`apply.go:1498`), so the mutation log holds a second
  `user.update` intent and result under that `call_id`. Nothing new records
  it.
- **Bound.** One resend. No wait, no loop. Both extra calls run on the
  request's own `ctx`, whose deadline `withBudget` set when the request
  began (`server.go:126`; `requestWithin`, `nas.go:36`). No new context, no
  timer. If the budget ends first, the resend or its read-back fails as any
  call does. That is not a second mismatch, so nothing is held: the outcome
  is `applied` and `Unverified`, as a failed read-back is today
  (`apply.go:1167-1191`), and the next plan reads the account fresh.
- **Never resent.** `user.create`: a second create is a second account or a
  refusal, never the same state. A mismatch on its read-back
  (`apply.go:1345`) holds `user.create` on the first observation. And
  `user.delete`: `account.purge` sends it on its own session in its own
  `mutate` (`operations.go:604-605`) and never enters `converge`. It writes
  no closed field and has no read-back to compare.

| The second read | Outcome | `detail`, then `consequence` |
|---|---|---|
| matches | `applied`, fingerprint from the second read. Nothing is held. | {what changed} {product} didn't make this change the first time. It took on the second try. Nothing for you to do. · {what the account holds} |
| differs again | `blocked`. The function is held (§12.2). | {product} accepted this change twice and didn't make it: asked `locked: true`, read `locked: false`. · {what the account holds} Syndra has stopped using user.update so it can't repeat this. Check the account in {product}, then press Try again on the system's page. |
| never came (the budget ended, or a call failed) | `applied`, `Unverified`. Nothing is held. | {what changed} {product} accepted this change and didn't show it. There wasn't time to send it again, so it isn't confirmed. Syndra's next check reads the account fresh. Nothing for you to do. |

A row's `last_error` is the add-on's two sentences joined, as for every
refusal (`addonWords`, `propagation/addon.go:497`): the second line above is
the held row's, whole.

### 12.2 Effect

- **The function is held on this TrueNAS version only.** Through §6: writes needing it answer `TARGET_VERSION_UNSUPPORTED`,
  the pass halts on `holds` (§7), other rows wait without spending an
  attempt, and waiting removals are counted in the notice's first line (§9).
- **The tripping row stays visible and is not sent again by itself.** Its
  subject answers `blocked` (`apply.go:194`), with the reason as its detail
  and, as its consequence, what the account now holds and the next step
  (§12.1's table). While the hold stands, a later
  row for the same account waits like every other.
- **Staff are told three ways,** from one emitter, on the monitor's first
  health answer that shows the hold: a Home attention item, the system
  page's notice (§9's `changed` notice, with the reason), and an audit event
  (`addon.function_held`: system, function, the TrueNAS version, the reason,
  the row).

### 12.3 Persistence

**Stored, in the add-on's own store** (`store.go`, a `holds` bucket beside
`bindings`). A function's hold is keyed by the function. A narrowed one
(§12.4) is keyed by function and subject, so a second account's fault is a
second record: it never overwrites the first, and the two never flip. Each
record holds the TrueNAS version, the reason, the account, the row's
`call_id`, when, and the trial while one is armed (§12.4).

It must persist: it is evidence of something that happened, and a restart
cannot derive it again without making the same bad write. It lives in the
add-on because the add-on decides (`server.unavailable` is the one gate, and
it also guards operations that queue no row); the backend learns of it
through `health`, like every other verdict. §8's "not persisted" stands for
verdicts, which a connection re-derives.

**The add-on's own version is not in the key.** It has none today:
`Dockerfile:23` builds with no version flag, and `ContractVersion`
(`server.go:674`) is the contract's. Stamping the commit in would not help:
every deploy rebuilds the image (`docker compose build`, `deploy.yml:159`),
so every unrelated merge would clear every hold and cost one more bad write
to hold it again. A new build is not evidence that TrueNAS behaves. Try
again is, and after an add-on update it is one press.

### 12.4 Clearing

- **By itself**, on connect: a hold whose TrueNAS version is no longer the
  current one is deleted. A new version is new evidence either way.
- **Try again**, by staff, on one observed record. It is justified where
  Trust was not: a real failure was observed, and this asks TrueNAS the
  same question once more instead of asking a person to vouch.

**Try again is one call.** It names one apply by its `call_id`, and the hold
is cleared only by that apply's own closed-field read-back (§12.1). Nothing
else can clear it. A member's `password.set` writes `user.update` and reads
nothing back (`operations.go:343-386`), so "the next write through the
function" would let a write nobody checked clear a hold. There is no
fallback for "no row waiting": step 1 always has a row.

1. **The backend picks the row.** One function in `services`, beside
   `convergeOn`, under the drain lock `DrainAddon` takes
   (`propagation/addon.go:49`).
   - *A function's record:* the first waiting row the pass itself would
     send (removals first, then intent order: `revocationFirst`,
     `db/propagations.go:290`), for an account no record of this
     function names.
   - *A narrowed record, or a function's record with no such row:* the
     record's own account. Its rows end terminal at the plan's `blocked`
     (`propagation/addon.go:606-621`) and the sweep queues nothing for a
     `blocked` plan (`drift/addon.go:576`), so there is none to find. The backend queues one with
     `convergeOn` (`services/lifecycle_trigger.go:148`), the call a role
     change already makes: it resolves the person's set now, so the trial
     sends what they should hold today, and once it is applied an older
     waiting row of theirs is overtaken at the claim (`terminateSuperseded`,
     `db/propagations.go:250`). `convergeOn` returns the row's id, which it
     drops today. No second way to make a row.

   **Why the backend picks, not the operator.** A picker asks staff which
   waiting change writes a closed field through this function, which they
   cannot know, and adds a second control beside one button. The rule is
   the pass's own order, so there is no second ordering: a removal goes
   first because its wait costs most (someone holds access they should not),
   and it is the one row the backend knows writes a closed field and nothing
   else (`locked`, the lifecycle switch alone: `sendsOnItsOwn`,
   `db/propagations.go:311`). And it answers the case a picker cannot:
   nothing waiting.
2. **It arms the record** with `function.retry {function, call_id}`: the
   row's id, which is the `call_id` its apply will carry
   (`propagation/addon.go:204`), with the row's subject as the operation's
   subject. Declared as `account.release` is, in the three places that
   declare one: the add-on's manifest (`capabilities.go:194`:
   `Scope: "admin"`, `Confirm: true`), the backend's policy
   (`addons/policy.go:163`; `function` and `call_id` required strings) and
   `registry/operations.json`. Signed like every operation, refused without
   the confirmation, audited as `addon.function_retried` (system, function,
   TrueNAS version, the row, who). It is local: `methodsFor` names no
   method for it, so nothing about the NAS withdraws it
   (`server.go:165-171`). It writes the record through `s.mutate`, as the
   release does (`operations.go:483`), so the add-on's own log has the
   arming or it did not happen. It answers the record keyed by the function
   and that subject, else the function's; with neither (a schema change, or
   no hold) it refuses and arms nothing.
3. **It sends that one row** through the add-on pass: `DrainAddon`'s lock,
   then `claimOne(ctx, target, id)` (`db.ClaimPropagationByID` already
   takes the target, `db/propagations.go:449`; `DrainOne` passes Zitadel's,
   `drain.go:364`), then `dispatchEntitlement` (`propagation/addon.go:109`).
   No batch. The one-row pass applies every halt but 9g's `holds`. The
   scheduled pass stays halted on `holds` until the function's record is
   gone.
4. **The add-on decides**, in `confirm`, for the armed call alone.

**The arm** is one field on the record: `trial {call_id, subject}`. A new
Try again overwrites it. While it is set:

- **Admission** is in `server.send` (`apply.go:1527`), the one function
  every NAS write passes with its caller: a write through a held function
  goes out only when `caller.callID` is the trial's. `NAS.sendWrite` cannot
  ask this, having no call. The check runs inside the `mutate`, before
  `sendWrite`, so a write it stops is in the log as `not_sent`. Every other
  call still answers as §12.2 says.
- **The trial's subject plans and applies as any account does.** Without
  this the pass's re-read before the send (`fingerprintFor`,
  `propagation/addon.go:573`) meets `blocked` and ends the row before it is
  sent.
- **It is spent** in `applyNow` (`apply.go:637`), the one place, two ways.
  By the trial's own apply, once it answers anything but "nothing was
  sent": `ErrNotSent` and `errTornRead` keep it (`apply.go:656-665`),
  because the backend leaves that row queued and it is still the trial. And
  by any apply for the trial's subject under another `call_id`, before that
  apply runs, so it is answered as at rest and never reaches `send`. That
  second way is what bounds a trial row that never arrives (overtaken at the
  claim): the subject is `blocked` again after one row, instead of planning
  as `apply` for the sweep to queue again and again. Applies run one at a
  time per subject (`once`, `calls.go:162`), so the two cannot interleave.

| The trial's apply | The record |
|---|---|
| its closed fields read back as asked (first or second try), and its account is the record's own | deleted (`addon.function_cleared`) |
| the same, another account (a function's record only) | narrowed: replaced by one keyed by the function and the record's account (§12.3) |
| they differ twice | kept, rewritten from this call: its reason, account and row |
| it wrote no closed field through the function (nothing to change, or other functions only) | kept; arm spent; nothing was tested |
| nothing was sent (busy, unreachable, `TARGET_CHANGED`) | kept; arm kept |
| any other ending (refused, stale plan, in doubt) | kept; arm spent |

- **A fault that is one account's own.** A narrowed record means the
  function works and not for that account. Its writes through the function
  answer `blocked` without being sent, others flow, the pass is not halted
  (its `functions` entry stays `unchanged`, `holds` empty), and the notice
  names the account. It clears the same two ways, and its trial is that
  account's own row (step 1). Without narrowing, drift would queue that
  account again, re-hold the function, and staff would press Try again for
  ever.

Only an observed record can be tried again. A schema change has no button:
the add-on does not know the new shape, and one more try would be sending
it blind.

### 12.5 Contract and copy

`version_check.functions[]` gains optional `observed`: one
`{subject, detail, narrowed}` per stored record. It is a list because two
accounts can each hold a narrowed record on one function (§12.3). The
function's `verdict` is `changed` only while a record that is not narrowed
stands. `registry/operations.json` gains `function.retry`. Three audit
events. No new error code, reason or read state.

| Reader, where | Says |
|---|---|
| Operator, Home | {product} didn't do what Syndra asked on {system}. {k} people who left still have access. |
| Operator, system page | {product} accepted a change and didn't make it: {reason}. Syndra has stopped using {function} so it can't repeat this. Other changes that need it wait. **Try again** sends one change to see if it works now. |
| the same, narrowed | {function} works, but not for {account}: {reason}. Check that account in {product}, then **Try again**. |
| Operator, the confirmation | Send one change to see if {function} works now? Syndra sends one waiting change and nothing else. · narrowed: Syndra sends {account}'s change and nothing else. |
| Operator, after Try again, it took | {function} works again. Syndra is sending what waited. Nothing for you to do. |
| the same, narrowed by it | as "the same, narrowed" above. |
| Operator, after Try again, it didn't | Still the same: {reason}. Nothing else was sent. Check {product}, or update the add-on. |
| Operator, after Try again, nothing was tested | That change didn't need {function}, so nothing was tested. Press **Try again** to send the next one. |
| the same, narrowed, nothing to change | {account} is already as Syndra wants it, so nothing was sent or tested. Press **Try again** when Syndra next has a change for it. |
| Operator, after Try again, nothing was sent | copy guide §19's sentence for why, then: Nothing was tested. Press **Try again** when {system} answers. |
| Operator, a change that took on the second try, and the held row | §12.1's table. |
| Member | as §9's "a write held". |

### 12.6 Slice 9j

One PR, after 9i, in the order of `tasks.md` 9j. Add-on: `closed` and
`confirm`, the store bucket, `observed` into `Check`, `function.retry`, the
admission in `send`. Backend: the policy entry, the pick, `convergeOn`'s
id, the one-row add-on pass, the three events, the Home item. UI: the
reason, the button with its confirmation, the narrowed line; board first.

*Tests,* against the fake NAS. Each fails without its step.

- *The second try.* A write whose first read-back shows `locked: false`
  and whose second shows `true` answers `applied` with the second-try
  sentence, holds nothing, and leaves two `user.update` records under one
  `call_id`. Two mismatches hold `user.update`, answer that subject
  `blocked`, and a later pass sends nothing for it. The resend's params are
  the closed fields only: an update that also carried `groups` and a new
  home resends neither. A purge whose `user.delete` fails sends exactly one
  `user.delete`. A `user.create` mismatch holds at once with one
  `user.create` sent. With the budget spent before the resend, the outcome
  is `applied` and `Unverified`, nothing is held and no call outlives the
  deadline.
- *What counts.* A read-back with `home` normalised or `groups` in another
  order holds nothing and resends nothing. A refusal of any kind, busy,
  unreachable, a timeout and an `Unverified` read-back hold nothing.
- *The hold.* A second row needing the function waits with its attempt
  count unchanged. The hold survives a restart. It is gone after a connect
  on another TrueNAS version. The event and the Home item are written once
  per hold, not once per probe.
- *The call binding.* With `user.update` held and row A armed, a
  `password.set` and another account's apply send nothing, and the hold
  and the arm stand. Row A then clears it. `function.retry` for a
  schema-changed function, or with no hold, is refused and arms nothing;
  without the confirmation it is refused by the backend.
- *The pick.* With a removal and an older ordinary row waiting, the removal
  is the trial. **Try again sends its one row while the pass is halted on
  `holds`, and sends no second row.**
- *The endings.* The trial mismatching twice holds again with the new
  reason. The trial taking for another account narrows the hold, and that
  account's next write is not sent. A second account narrowed after it
  leaves the first record as it was. A busy trial keeps the arm, and the
  same row sent again is the trial. A trial that changes nothing spends the
  arm and leaves the record.
- *The narrowed trial.* With a narrowed record and no row for that account,
  Try again queues one through `convergeOn`, arms its id, and the row is
  sent, not ended at the plan's `blocked`; its read-back matching deletes
  the record. With the function's record and nothing else waiting, the
  trial is the record's own account's row and its match deletes the record.
  An armed subject whose trial row never arrives is `blocked` again after
  the first other apply for it, and the sweep queues nothing more.

*Mutation:* compare every written field, and the `home` case goes red;
resend the whole update, and the home case goes red; admit by function
instead of by `call_id`, and the `password.set` case goes red; let the plan
stay `blocked` for an armed subject, and the narrowed trial goes red.

**Not in scope: telling staff outside the app.** There is no email or push
today, only the dead-man URL. A possible later slice: a notification URL
the backend calls when a function is held.

## 13. Still the owner's

1. **9a:** paste the elevated key once (§3.1), and say when.
2. **9i**, the staging run.
3. Inside §12, three points this round settled by the code, for him to
   overrule:
   - **The second try resends the closed fields, not the whole write.** His
     ruling says "the same write". The whole update is not idempotent when
     it carries a new home (`home.go:123-133`), so the resend is the closed
     fields of that write with the same values (§12.1).
   - **A `user.create` mismatch holds on the first observation.** A create
     cannot be sent twice (§12.1).
   - **A narrowed hold stays when its trial finds nothing to change.** If
     staff set the field by hand in TrueNAS, the trial sends nothing, and
     only a closed-field read-back clears a hold (his ruling). It then
     clears the next time Syndra has a change for that account. The other
     way: treat "already as asked" as cleared.
4. One candidate left out: a reply that fails its strict decode with the
   schema unchanged (§5) is also a function misbehaving, and holding it
   would put its waiting removals in the count.
