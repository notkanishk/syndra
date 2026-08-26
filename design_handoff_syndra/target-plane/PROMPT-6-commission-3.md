# Commission 3 — the mapping screen, two contradictions, and the member's side

Paste into the **same conversation** as commissions 1 and 2 if it is still open.
If it is not, paste `PROMPT-1-design-system.md` first, then this.

---

The zip landed and verified: twenty figures, ids matching the inventory, boards
opening standalone. I have read all of it against the code. Three things to draw,
and first the answers you asked for.

## Your thirteen open questions — ten are settled by the code

You guessed well. Two of your guesses were wrong in ways that change a drawing.

1. **Maintenance during an outage — keep it live, but your reason is wrong.** It
   is not a Syndra-side record edit; it is `POST {addon}/lifecycle` over the
   network, and it can be refused. It survives an outage for a better reason:
   that call is *deliberately exempt from the breaker*, because letting a refusal
   count towards opening the circuit would conflate "we told it to stop" with "it
   stopped answering". **So it needs a failure path, which no figure has.** See
   commission B below.
2. **Two ages — correct.** `snapshot_taken_at` and `read_at` are separate fields
   from separate reads. Two strips is honest.
3. **Both findings arrive with the health read — correct.** `log_anchor` and
   `binding_conflicts[]` are fields on the health payload. Moving them into
   region 1 changes nothing about what the page fetches.
4. **Census line — keep the count.** Your own argument holds.
5. **The dashes collide.** In this codebase dashed already means *produced by an
   automatic rule* — a dashed chip on a role, a dashed edge in the access map. It
   is a provenance idiom. "Off right now, with a reason" must use the established
   pattern instead: the disabled control at reduced alpha with its reason as body
   text in place.
6. **The pool warning is already the target's own.** `pool.query` returns
   `warning` alongside `healthy` and `status`, and Syndra passes it through.
   Render that flag. Amber computed from `free/size` would be Syndra publishing a
   conclusion the target declined to publish — the one thing the two-card split
   exists to prevent.
8. **Deciding twice is refused, not replaced.** This is the one that matters —
   commission B.
9. **The half-finished unbind is the same call and is idempotent — correct**, as
   drawn.
11. **A sweep distinguishes them.** The result carries `current: boolean`;
    `current: false` is a sweep that did not read the target. Your copy is safe.
12. **A finding always names a subject.** Required and validated non-empty. The
    roster row's finding count always has somewhere to point.

Still open and not for you to solve: what routes sit behind *apply it again* and
*stop wanting it* (7); which index readings can co-occur (10).

**And one thing not to build.** Section 6 proposes a freshness strip. It already
exists — `ReadFreshness`, with the tone dot, the age sentence, the truncation
clause and the stale-only *Read again*. You considered "the amber banner inside
the unmanaged inventory, which is where this behaviour currently lives"; there is
a shared component above that banner, and you had no way to know. Draw the strip
as you drew it — the drawing is right — but it is an existing component, not a
new one. The other five stand: two are new, two are one-line extensions of
`Badge` and the status-dot map, one is touch-only.

---

# A · The mapping screen

**This is the blocker.** Your redesign removes *What roles reach here* and
*Published versions* from the target page and sends them to a screen that does
not exist. Nothing can be removed from the deployed page until it does. So this
is drawn first, and the target page waits for it.

`/system/targets/{target}/mappings` · Advanced › System › ‹target› › Mappings

Reached from the census line you drew in region 2, and from a role. **Not a new
rail row** — a rail row per add-on already exists and a second one for its
mappings would be structure competing with itself. Your words; I agree.

## What it holds

**A mapping** ties a role to what it reaches on the add-on:
`project_id` · `role_key` · `field` · `value`, plus who created it and who last
changed it. It is unique on (target, project, role, field). One row per mapping.

Two facts belong on every row: **how many people hold that role today**, and that
editing the row moves access for all of them.

**Fields are not free text.** `group` is a real field. `enabled` and
`smb_enabled` are *derived* — they follow from entitlement and are refused as
mapping fields at three separate layers of the backend. A field picker that
offers them would be offering something the API refuses; the screen should say
why they are absent rather than silently omitting them.

**A value must name something that exists on the target.** The backend checks it
and **fails open on everything except a definite no** — a target that could not
be read, a field the add-on cannot enumerate, an unregistered add-on all pass,
because refusing a mapping edit while a NAS reboots would make an outage look
like a validation failure. The only refusal is *the add-on answered and does not
recognise that value*, and its operator action is a correction, not a retry.

**Published versions.** A snapshot of the whole mapping set with a mandatory
note, and a rollback per version. The thing a table row cannot say, and the
reason this screen exists at all: **the working copy can differ from the newest
published version.** "Current version 4" can mean version 4 plus three
unpublished edits, and rolling back from there undoes work listed nowhere.

## What every change goes through

Edit, delete and rollback all **rehearse before they land** — the plan is what
gets approved, not the form. Per the build notes, the blast radius is **a backend
refusal producing a scope step**, not a checkbox drawn upfront: the rehearsal is
sent without acknowledgement, the backend refuses anything above the cohort
limit, and *that* refusal produces the step carrying the number. The threshold
lives in one place and the operator meets the ceremony only when it is warranted.

## Draw

1. The screen with two mappings, one version published, working copy clean.
2. **The working copy ahead of the published version** — three unpublished edits
   on top of v4. This is the figure the screen exists for.
3. An edit rehearsed, with the scope step the refusal produced.
4. A value the add-on does not recognise — a correction, not a retry.
5. Empty: no mapping reaches this target, so no role grants anything on it. On
   the live deployment this is the real state, and it is not an error.
6. The census line and its route in, as it appears on the target page — so I can
   see both ends of the handoff you designed.

---

# B · The two places the code contradicted the drawing

Two small figures. Both are states the boards do not have, and I cannot build
region 1 or the band correctly without them.

## B1 · A decision somebody else already took

Your decided-and-waiting row says *"deciding again replaces the queued work; it
does not stack."* The backend does the opposite, deliberately:

```sql
WHERE id = $1::uuid AND resolved_at IS NULL AND decision IS NULL
```

A second decision matches nothing and comes back **409 `ALREADY_DECIDED`**, and
the guard's comment says why: *"a second request could replace a decision whose
work was already queued, and for `unbound` that meant releasing the account on
the target while a re-provision sat in the outbox. One writer wins and the loser
is told so. Fail-closed rather than last-write-wins, because the two answers here
are opposites."*

So the sentence inverts. Two operators can open the same finding; the second one
to press loses, and the screen has to say so well. Today it renders the API's own
sentence — a UUID, a snake_case resolution and a subject id — which is the raw
material, not the copy.

**Draw:** the refusal in place, naming who decided and as what, and saying what
happens next. It is not an error the operator caused and it must not read as one.
The finding is not gone; it is decided, and their view was stale.

## B2 · Maintenance refused during an outage

The control stays live during an outage — you were right — but it is a network
call and it can be refused. No figure shows that.

**Draw:** the maintenance strip after a lifecycle change that did not land, in
the band, on a target that is not answering. It must be distinguishable from the
state having changed, and it must not read as the operator having done something
wrong. Say what is still true: the state Syndra last knew about, and that reads
keep working in all three states regardless.

---

# C · The member's side of read-only

Open question 13, and you were right to raise it. An operator sets read-only, and
the member's storage page says nothing at all — `MyStorage` carries no lifecycle
field, in neither the backend nor the UI. It is new work in both, so it needs the
drawing before it needs the code.

`/storage` · Member › Network storage

The member's page has three states already designed and built: **no entitlement**
· **entitled, no account yet** ("recorded, not created yet, nothing needed from
you" — the ordinary experience of every new member until a drain is resumed) ·
**account present**, with the credential form and the connection instructions.

Draining and read-only cut across all three. What a member needs to know:

- their access is **unchanged** — this is not a withdrawal;
- something they *do* on this page may not take effect yet, in particular
  **setting a password**, which is a write;
- roughly what to expect, without a promise Syndra cannot keep about when.

The distinction the copy has to carry: **the file server is fine and Syndra has
paused writing to it on purpose.** A member who reads it as an outage will go and
try to mount the share, which will work, and then conclude the page is wrong.

**Draw:** the account-present state under draining, and under read-only, and the
entitled-but-no-account state under read-only — which is the cruellest
combination, because the thing they are waiting for is exactly the thing that
cannot happen.

---

## Hand it back the same way

The zip layout from the last commission worked. Same again:
`design/*.dc.html` with `support.js` beside them, a prose reply, and a `FIGURES.md`
inventory of what actually exists on a board. Continue the id sequence — `M1…`
for the mapping screen, `B1`/`B2`, `C1…` — so nothing collides with `T1–T5` and
`S1–S15`.

Still no build plan, and still no "X is canonical".
